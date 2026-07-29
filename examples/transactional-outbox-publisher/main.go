// Package main 은 transactional outbox commit 과 Redis Streams relay 를 한 번 실행한다.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go-workshop/examples/transactional-outbox-publisher/internal/orderoutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

const (
	defaultOrderID    = "order-1001"
	defaultCustomerID = "customer-42"
	defaultCommandID  = "command-1001"
	defaultTotalCents = int64(3700)
	maxConfigIDRunes  = 128
	maxStreamBytes    = 256
	readinessTimeout  = 5 * time.Second
	executionTimeout  = 30 * time.Second
)

type appConfig struct {
	databaseURL string
	redisAddr   string
	redisStream string
	orderID     string
	customerID  string
	commandID   string
	totalCents  int64
	createdAt   time.Time
	now         func() time.Time
}

type runOutput struct {
	Order          orderoutbox.Order     `json:"order"`
	Relay          sqloutbox.RelayResult `json:"relay"`
	Stream         string                `json:"stream"`
	EventID        string                `json:"event_id"`
	IdempotencyKey string                `json:"idempotency_key"`
}

type dependencies struct {
	db            *sql.DB
	redisClient   *redis.Client
	closePostgres func() error
	closeRedis    func() error
	closeOnce     sync.Once
	closeErr      error
}

func main() {
	config, err := loadConfig(os.Getenv, time.Now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration: %v\n", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, config, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "transactional outbox example: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string, clock func() time.Time) (appConfig, error) {
	if getenv == nil {
		return appConfig{}, errors.New("environment reader is required")
	}
	if clock == nil {
		clock = time.Now
	}
	now := func() time.Time { return clock().UTC() }

	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return appConfig{}, errors.New("DATABASE_URL is required")
	}
	redisAddr := strings.TrimSpace(getenv("REDIS_ADDR"))
	if redisAddr == "" {
		return appConfig{}, errors.New("REDIS_ADDR is required")
	}
	redisStream, err := configuredStream(getenv("REDIS_STREAM"))
	if err != nil {
		return appConfig{}, err
	}
	orderID, err := configuredID("ORDER_ID", getenv("ORDER_ID"), defaultOrderID)
	if err != nil {
		return appConfig{}, err
	}
	customerID, err := configuredID("CUSTOMER_ID", getenv("CUSTOMER_ID"), defaultCustomerID)
	if err != nil {
		return appConfig{}, err
	}
	commandID, err := configuredID("COMMAND_ID", getenv("COMMAND_ID"), defaultCommandID)
	if err != nil {
		return appConfig{}, err
	}
	createdAt := now()
	if raw := getenv("ORDER_CREATED_AT"); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(raw))
		if parseErr != nil || parsed.IsZero() {
			return appConfig{}, errors.New("ORDER_CREATED_AT must use RFC3339")
		}
		createdAt = parsed.UTC()
	}
	if createdAt.IsZero() {
		return appConfig{}, errors.New("ORDER_CREATED_AT clock produced zero time")
	}

	return appConfig{
		databaseURL: databaseURL, redisAddr: redisAddr, redisStream: redisStream,
		orderID: orderID, customerID: customerID, commandID: commandID,
		totalCents: defaultTotalCents, createdAt: createdAt, now: now,
	}, nil
}

func configuredID(variable, raw, fallback string) (string, error) {
	if raw == "" {
		return fallback, nil
	}
	value := strings.TrimSpace(raw)
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%s must be valid UTF-8", variable)
	}
	if runes := utf8.RuneCountInString(value); runes == 0 || runes > maxConfigIDRunes {
		return "", fmt.Errorf("%s must contain 1..%d runes", variable, maxConfigIDRunes)
	}
	return value, nil
}

func configuredStream(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	stream := strings.TrimSpace(raw)
	if !utf8.ValidString(stream) {
		return "", errors.New("REDIS_STREAM must be valid UTF-8")
	}
	if stream == "" || len(stream) > maxStreamBytes {
		return "", fmt.Errorf("REDIS_STREAM must contain 1..%d bytes", maxStreamBytes)
	}
	return stream, nil
}

func openDependencies(ctx context.Context, config appConfig) (*dependencies, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := sql.Open("pgx", config.databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL client: %w", err)
	}
	deps := &dependencies{db: db, closePostgres: db.Close}
	client := redis.NewClient(&redis.Options{Addr: config.redisAddr})
	deps.redisClient = client
	deps.closeRedis = client.Close

	pingCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	err = db.PingContext(pingCtx)
	cancel()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("verify PostgreSQL readiness: %w", err), deps.Close())
	}
	pingCtx, cancel = context.WithTimeout(ctx, readinessTimeout)
	err = client.Ping(pingCtx).Err()
	cancel()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("verify Redis readiness: %w", err), deps.Close())
	}
	return deps, nil
}

func (d *dependencies) Close() error {
	if d == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		var postgresErr, redisErr error
		if d.closePostgres != nil {
			postgresErr = d.closePostgres()
		}
		if d.closeRedis != nil {
			redisErr = d.closeRedis()
		}
		d.closeErr = errors.Join(postgresErr, redisErr)
	})
	return d.closeErr
}

func run(ctx context.Context, config appConfig, output io.Writer) error {
	deps, err := openDependencies(ctx, config)
	if err != nil {
		return err
	}
	return runWithDependencies(ctx, deps, config, output)
}

func runWithDependencies(ctx context.Context, deps *dependencies, config appConfig, output io.Writer) error {
	if deps == nil {
		return errors.New("run dependencies are required")
	}
	var buffered bytes.Buffer
	var executeErr error
	if output == nil {
		executeErr = errors.New("run output is required")
	} else {
		executeErr = execute(ctx, deps.db, deps.redisClient, config, &buffered)
	}
	if err := errors.Join(executeErr, deps.Close()); err != nil {
		return err
	}
	if _, err := io.Copy(output, &buffered); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func execute(ctx context.Context, db *sql.DB, redisClient *redis.Client, config appConfig, output io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil || redisClient == nil || output == nil {
		return errors.New("execute dependencies are required")
	}
	if config.now == nil {
		config.now = func() time.Time { return time.Now().UTC() }
	}
	operationCtx, cancel := context.WithTimeout(ctx, executionTimeout)
	defer cancel()

	store, err := sqloutbox.NewStore(sqloutbox.Options{
		Table: "transactional_outbox_records",
		Now:   func() time.Time { return config.now().UTC() },
	})
	if err != nil {
		return fmt.Errorf("construct SQL outbox store: %w", err)
	}
	service, err := orderoutbox.NewService(store, orderoutbox.Config{
		Author: "transactional-outbox-publisher",
		Now:    func() time.Time { return config.now().UTC() },
	})
	if err != nil {
		return fmt.Errorf("construct order outbox service: %w", err)
	}
	if err := service.CreateSchema(operationCtx, db); err != nil {
		return fmt.Errorf("create schemas: %w", err)
	}
	order, err := service.Place(operationCtx, db, orderoutbox.PlaceOrderCommand{
		OrderID: config.orderID, CustomerID: config.customerID,
		CommandID: config.commandID, TotalCents: config.totalCents,
		CreatedAt: config.createdAt,
	})
	if err != nil {
		return fmt.Errorf("place order: %w", err)
	}
	publisher, err := redisstreams.New(redisstreams.Options{Client: redisClient, Stream: config.redisStream})
	if err != nil {
		return fmt.Errorf("construct Redis Streams publisher: %w", err)
	}
	relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
		ClaimLimit: 1, MaxAttempts: 3,
		RetryDelay: 250 * time.Millisecond, IdleDelay: 50 * time.Millisecond,
		Now: func() time.Time { return config.now().UTC() },
	})
	if err != nil {
		return fmt.Errorf("construct SQL outbox relay: %w", err)
	}
	result, err := relay.RunOnce(operationCtx, db)
	if err != nil {
		return fmt.Errorf("publish outbox batch: %w", err)
	}
	wantResult := sqloutbox.RelayResult{Claimed: 1, Published: 1}
	if result != wantResult {
		return fmt.Errorf("publish outbox batch: result %#v, want %#v", result, wantResult)
	}
	messages, err := redisClient.XRevRangeN(operationCtx, publisher.Stream(), "+", "-", 1).Result()
	if err != nil {
		return fmt.Errorf("verify Redis stream: %w", err)
	}
	if len(messages) != 1 {
		return fmt.Errorf("verify Redis stream: messages = %d, want 1", len(messages))
	}
	eventID := fmt.Sprint(messages[0].Values["event_id"])
	idempotencyKey := fmt.Sprint(messages[0].Values["idempotency_key"])
	if eventID != config.commandID || idempotencyKey != config.commandID {
		return errors.New("verify Redis stream: stable identity mismatch")
	}
	if err := json.NewEncoder(output).Encode(runOutput{
		Order: order, Relay: result, Stream: publisher.Stream(),
		EventID: eventID, IdempotencyKey: idempotencyKey,
	}); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}
