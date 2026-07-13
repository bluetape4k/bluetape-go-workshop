package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/transactional-outbox-publisher/internal/orderoutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func TestLoadConfig(t *testing.T) {
	fixedNow := time.Date(2026, 7, 14, 20, 30, 0, 0, time.FixedZone("KST", 9*60*60))
	base := map[string]string{
		"DATABASE_URL": "postgres://workshop",
		"REDIS_ADDR":   "127.0.0.1:6379",
	}

	config, err := loadConfig(mapGetenv(base), func() time.Time { return fixedNow })
	if err != nil {
		t.Fatalf("loadConfig(defaults) error = %v", err)
	}
	if config.databaseURL != base["DATABASE_URL"] || config.redisAddr != base["REDIS_ADDR"] {
		t.Fatalf("endpoints = (%q, %q), want configured endpoints", config.databaseURL, config.redisAddr)
	}
	if config.redisStream != "" {
		t.Fatalf("redis stream = %q, want provider default", config.redisStream)
	}
	if config.orderID != "order-1001" || config.customerID != "customer-42" || config.commandID != "command-1001" {
		t.Fatalf("default IDs = (%q, %q, %q)", config.orderID, config.customerID, config.commandID)
	}
	if config.totalCents != 3700 || !config.createdAt.Equal(fixedNow.UTC()) {
		t.Fatalf("default order = (%d, %s), want (3700, %s)", config.totalCents, config.createdAt, fixedNow.UTC())
	}

	custom := cloneEnv(base)
	custom["REDIS_STREAM"] = " workshop:orders "
	custom["ORDER_ID"] = " 주문-42 "
	custom["CUSTOMER_ID"] = " 고객-7 "
	custom["COMMAND_ID"] = " 명령-9 "
	custom["ORDER_CREATED_AT"] = "2026-07-14T07:08:09+09:00"
	config, err = loadConfig(mapGetenv(custom), func() time.Time { return fixedNow })
	if err != nil {
		t.Fatalf("loadConfig(custom) error = %v", err)
	}
	if config.redisStream != "workshop:orders" || config.orderID != "주문-42" || config.customerID != "고객-7" || config.commandID != "명령-9" {
		t.Fatalf("custom config = %#v", config)
	}
	wantCreatedAt, _ := time.Parse(time.RFC3339, custom["ORDER_CREATED_AT"])
	if !config.createdAt.Equal(wantCreatedAt) {
		t.Fatalf("created at = %s, want %s", config.createdAt, wantCreatedAt)
	}

	tests := []struct {
		name    string
		key     string
		value   string
		wantVar string
	}{
		{name: "missing database", key: "DATABASE_URL", wantVar: "DATABASE_URL"},
		{name: "blank redis address", key: "REDIS_ADDR", value: " \t ", wantVar: "REDIS_ADDR"},
		{name: "blank stream", key: "REDIS_STREAM", value: "  ", wantVar: "REDIS_STREAM"},
		{name: "invalid stream UTF-8", key: "REDIS_STREAM", value: string([]byte{0xff}), wantVar: "REDIS_STREAM"},
		{name: "oversized stream bytes", key: "REDIS_STREAM", value: strings.Repeat("가", 86), wantVar: "REDIS_STREAM"},
		{name: "blank order ID", key: "ORDER_ID", value: " ", wantVar: "ORDER_ID"},
		{name: "invalid customer ID", key: "CUSTOMER_ID", value: string([]byte{0xff}), wantVar: "CUSTOMER_ID"},
		{name: "oversized command ID", key: "COMMAND_ID", value: strings.Repeat("명", 129), wantVar: "COMMAND_ID"},
		{name: "invalid created at", key: "ORDER_CREATED_AT", value: "private-invalid-time", wantVar: "ORDER_CREATED_AT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := cloneEnv(base)
			if tt.value == "" {
				delete(env, tt.key)
			} else {
				env[tt.key] = tt.value
			}
			_, err := loadConfig(mapGetenv(env), func() time.Time { return fixedNow })
			if err == nil {
				t.Fatal("loadConfig() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Fatalf("loadConfig() error = %q, want variable %s", err, tt.wantVar)
			}
			if strings.TrimSpace(tt.value) != "" && strings.Contains(err.Error(), tt.value) {
				t.Fatalf("loadConfig() error echoed configured value: %q", err)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	postgresURL, redisAddr := startRuntimeFixtures(ctx, t)
	db, err := sql.Open("pgx", postgresURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	config := appConfig{
		redisStream: "workshop:execute", orderID: "order-execute",
		customerID: "customer-42", commandID: "command-execute",
		totalCents: 3700, createdAt: now, now: func() time.Time { return now },
	}
	var output bytes.Buffer
	if err := execute(ctx, db, client, config, &output); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	assertRunOutput(t, output.Bytes(), config)
	if length, err := client.XLen(ctx, config.redisStream).Result(); err != nil || length != 1 {
		t.Fatalf("stream length = (%d, %v), want (1, nil)", length, err)
	}
}

func TestRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	postgresURL, redisAddr := startRuntimeFixtures(ctx, t)
	now := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
	config := appConfig{
		databaseURL: postgresURL, redisAddr: redisAddr,
		redisStream: "workshop:run", orderID: "order-run",
		customerID: "customer-42", commandID: "command-run",
		totalCents: 3700, createdAt: now, now: func() time.Time { return now },
	}
	var output bytes.Buffer
	if err := run(ctx, config, &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	assertRunOutput(t, output.Bytes(), config)

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })
	if length, err := client.XLen(ctx, config.redisStream).Result(); err != nil || length != 1 {
		t.Fatalf("stream length after run = (%d, %v), want (1, nil)", length, err)
	}
}

func TestRunWithDependenciesSuppressesSuccessWhenCloseFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	postgresURL, redisAddr := startRuntimeFixtures(ctx, t)
	db, err := sql.Open("pgx", postgresURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })

	postgresErr := errors.New("postgres close")
	redisErr := errors.New("redis close")
	postgresCalls := 0
	redisCalls := 0
	deps := &dependencies{
		db: db, redisClient: client,
		closePostgres: func() error { postgresCalls++; return postgresErr },
		closeRedis:    func() error { redisCalls++; return redisErr },
	}
	now := time.Date(2026, 7, 14, 14, 0, 0, 0, time.UTC)
	config := appConfig{
		redisStream: "workshop:close-failure", orderID: "order-close-failure",
		customerID: "customer-42", commandID: "command-close-failure",
		totalCents: 3700, createdAt: now, now: func() time.Time { return now },
	}
	var output bytes.Buffer
	err = runWithDependencies(ctx, deps, config, &output)
	if !errors.Is(err, postgresErr) || !errors.Is(err, redisErr) {
		t.Fatalf("runWithDependencies() error = %v, want both close errors", err)
	}
	if postgresCalls != 1 || redisCalls != 1 {
		t.Fatalf("close calls = (%d, %d), want (1, 1)", postgresCalls, redisCalls)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want empty on close failure", output.String())
	}
}

func TestExecuteRejectsStalePendingRecord(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	postgresURL, redisAddr := startRuntimeFixtures(ctx, t)
	db, err := sql.Open("pgx", postgresURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
	store, err := sqloutbox.NewStore(sqloutbox.Options{
		Table: "transactional_outbox_records", Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("sqloutbox.NewStore() error = %v", err)
	}
	service, err := orderoutbox.NewService(store, orderoutbox.Config{
		Author: "stale-seed", Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("orderoutbox.NewService() error = %v", err)
	}
	if err := service.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}
	if _, err := service.Place(ctx, db, orderoutbox.PlaceOrderCommand{
		OrderID: "order-aaa-stale", CustomerID: "customer-42",
		CommandID: "command-stale", TotalCents: 3700, CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed Place() error = %v", err)
	}

	config := appConfig{
		redisStream: "workshop:stale-pending", orderID: "order-zzz-current",
		customerID: "customer-42", commandID: "command-current",
		totalCents: 3700, createdAt: now.Add(time.Second), now: func() time.Time { return now.Add(time.Second) },
	}
	var output bytes.Buffer
	err = execute(ctx, db, client, config, &output)
	if err == nil || !strings.Contains(err.Error(), "stable identity mismatch") {
		t.Fatalf("execute() error = %v, want stable identity mismatch", err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want no false success", output.String())
	}
}

func TestDependenciesClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	postgresURL, redisAddr := startRuntimeFixtures(ctx, t)
	deps, err := openDependencies(ctx, appConfig{databaseURL: postgresURL, redisAddr: redisAddr})
	if err != nil {
		t.Fatalf("openDependencies() error = %v", err)
	}
	if err := deps.db.PingContext(ctx); err != nil {
		t.Fatalf("postgres before close: %v", err)
	}
	if err := deps.redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis before close: %v", err)
	}
	if err := deps.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := deps.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := deps.db.PingContext(ctx); err == nil {
		t.Fatal("postgres Ping after Close() error = nil")
	}
	if err := deps.redisClient.Ping(ctx).Err(); err == nil {
		t.Fatal("redis Ping after Close() error = nil")
	}
}

func TestDependenciesCloseJoinsErrorsOnce(t *testing.T) {
	postgresErr := errors.New("postgres close")
	redisErr := errors.New("redis close")
	postgresCalls := 0
	redisCalls := 0
	deps := &dependencies{
		closePostgres: func() error { postgresCalls++; return postgresErr },
		closeRedis:    func() error { redisCalls++; return redisErr },
	}
	err := deps.Close()
	if !errors.Is(err, postgresErr) || !errors.Is(err, redisErr) {
		t.Fatalf("Close() error = %v, want both close errors", err)
	}
	_ = deps.Close()
	if postgresCalls != 1 || redisCalls != 1 {
		t.Fatalf("close calls = (%d, %d), want (1, 1)", postgresCalls, redisCalls)
	}
}

func mapGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func cloneEnv(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func startRuntimeFixtures(ctx context.Context, t *testing.T) (string, string) {
	t.Helper()
	postgresURL := postgrestestcontainer.Start(ctx, t)
	redisAddr := redistestcontainer.Start(ctx, t)
	return postgresURL, redisAddr
}

func assertRunOutput(t *testing.T, data []byte, config appConfig) {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("output is not newline-terminated: %q", data)
	}
	var got runOutput
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got.Order.OrderID != config.orderID || got.Order.CustomerID != config.customerID || got.Order.TotalCents != config.totalCents {
		t.Fatalf("output order = %#v, want configured order", got.Order)
	}
	if got.Relay != (sqloutbox.RelayResult{Claimed: 1, Published: 1}) {
		t.Fatalf("output relay = %#v, want claimed/published 1", got.Relay)
	}
	wantStream := config.redisStream
	if wantStream == "" {
		wantStream = "audit:sqloutbox"
	}
	if got.Stream != wantStream || got.EventID != config.commandID || got.IdempotencyKey != config.commandID {
		t.Fatalf("output transport = (%q, %q, %q), want (%q, %q, %q)", got.Stream, got.EventID, got.IdempotencyKey, wantStream, config.commandID, config.commandID)
	}
}
