// Package main은 감사 가능한 주문 워크플로와 감독되는 SQL outbox 릴레이를 실행한다.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/audited-order-workflow-outbox/internal/orderworkflow"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

type appConfig struct {
	databaseURL  string
	redisAddress string
	redisStream  string
	httpAddress  string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	config, err := loadConfig(os.Getenv)
	if err != nil {
		logger.Error("application failed", "stage", "configuration", "class", "invalid")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, config, logger); err != nil {
		logger.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string) (appConfig, error) {
	if getenv == nil {
		return appConfig{}, stageError("configuration", "invalid")
	}
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	redisAddress := strings.TrimSpace(getenv("REDIS_ADDR"))
	if databaseURL == "" || redisAddress == "" {
		return appConfig{}, stageError("configuration", "invalid")
	}
	httpAddress := strings.TrimSpace(getenv("HTTP_ADDR"))
	if httpAddress == "" {
		httpAddress = "127.0.0.1:8080"
	}
	if err := orderworkflow.ValidateLoopbackAddress(httpAddress); err != nil {
		return appConfig{}, stageError("configuration", "invalid")
	}
	redisStream, err := orderworkflow.ValidateRedisStream(getenv("REDIS_STREAM"))
	if err != nil {
		return appConfig{}, stageError("configuration", "invalid")
	}
	return appConfig{
		databaseURL: databaseURL, redisAddress: redisAddress,
		redisStream: redisStream, httpAddress: httpAddress,
	}, nil
}

func run(ctx context.Context, config appConfig, logger *slog.Logger) (result error) {
	if logger == nil {
		return stageError("configuration", "invalid")
	}
	ctx = normalizeMainContext(ctx)
	db, err := sql.Open("pgx", config.databaseURL)
	if err != nil {
		return stageError("database_open", "unavailable")
	}
	orderworkflow.ConfigureDatabase(db)
	var redisClient *redis.Client
	defer func() {
		var redisCloseErr, databaseCloseErr error
		if redisClient != nil {
			redisCloseErr = redisClient.Close()
		}
		databaseCloseErr = db.Close()
		if redisCloseErr != nil {
			result = errors.Join(result, stageError("redis_close", "failed"))
		}
		if databaseCloseErr != nil {
			result = errors.Join(result, stageError("database_close", "failed"))
		}
	}()

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err = db.PingContext(pingCtx)
	cancel()
	if err != nil {
		return stageError("database_ping", "unavailable")
	}
	redisClient = redis.NewClient(orderworkflow.NewRedisOptions(config.redisAddress))
	pingCtx, cancel = context.WithTimeout(ctx, 2*time.Second)
	err = redisClient.Ping(pingCtx).Err()
	cancel()
	if err != nil {
		return stageError("redis_ping", "unavailable")
	}

	now := func() time.Time { return time.Now().UTC() }
	outbox, err := sqloutbox.NewStore(sqloutbox.Options{Table: "audited_order_workflow_outbox_records", Now: now})
	if err != nil {
		return stageError("outbox", "invalid")
	}
	history, err := orderworkflow.NewHistoryStore(db)
	if err != nil {
		return stageError("history", "invalid")
	}
	operationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = orderworkflow.CreateSchema(operationCtx, db, outbox)
	cancel()
	if err != nil {
		return stageError("schema", "incompatible")
	}
	service, err := orderworkflow.NewService(db, history, outbox, orderworkflow.Config{
		Author: "audited-order-workflow-outbox", Now: now,
	})
	if err != nil {
		return stageError("service", "invalid")
	}
	publisher, err := redisstreams.New(redisstreams.Options{Client: redisClient, Stream: config.redisStream})
	if err != nil {
		return stageError("publisher", "invalid")
	}
	relayOptions := orderworkflow.DefaultRelayOptions()
	relayOptions.Now = now
	relay, err := sqloutbox.NewRelay(outbox, publisher, relayOptions)
	if err != nil {
		return stageError("relay", "invalid")
	}
	health, err := orderworkflow.NewRuntimeHealth(
		func(ctx context.Context) error { return db.PingContext(ctx) },
		func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
		history,
		now,
	)
	if err != nil {
		return stageError("health", "invalid")
	}
	engine, err := orderworkflow.NewEngine(service, history, health, orderworkflow.DefaultHTTPConfig(), logger)
	if err != nil {
		return stageError("http", "invalid")
	}
	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "tcp", config.httpAddress)
	if err != nil {
		return stageError("http_listen", "unavailable")
	}
	server := orderworkflow.NewHTTPServer(config.httpAddress, engine)
	if err := orderworkflow.RunLifecycle(ctx, server, listener, relay, db, health, orderworkflow.DefaultShutdownLimit()); err != nil {
		return stageError("lifecycle", "failed")
	}
	return nil
}

func stageError(stage string, class string) error {
	return errors.New(stage + ": " + class)
}

func normalizeMainContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
