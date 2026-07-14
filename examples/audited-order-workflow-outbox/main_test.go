package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://user:secret@database.example/workshop",
		"REDIS_ADDR":   "redis.example:6379",
	}
	config, err := loadConfig(mapEnvironment(base))
	if err != nil {
		t.Fatal(err)
	}
	if config.httpAddress != "127.0.0.1:8080" || config.redisStream != "audit:sqloutbox" {
		t.Fatalf("defaults = %+v", config)
	}
	custom := cloneEnvironment(base)
	custom["HTTP_ADDR"] = "[::1]:9090"
	custom["REDIS_STREAM"] = " workshop:orders "
	config, err = loadConfig(mapEnvironment(custom))
	if err != nil || config.httpAddress != "[::1]:9090" || config.redisStream != "workshop:orders" {
		t.Fatalf("custom = (%+v, %v)", config, err)
	}

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "missing database", key: "DATABASE_URL"},
		{name: "missing redis", key: "REDIS_ADDR"},
		{name: "wildcard HTTP", key: "HTTP_ADDR", value: "0.0.0.0:8080"},
		{name: "hostname HTTP", key: "HTTP_ADDR", value: "localhost:8080"},
		{name: "blank stream", key: "REDIS_STREAM", value: " "},
		{name: "oversized stream", key: "REDIS_STREAM", value: strings.Repeat("a", 257)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environment := cloneEnvironment(base)
			if tt.value == "" {
				delete(environment, tt.key)
			} else {
				environment[tt.key] = tt.value
			}
			_, err := loadConfig(mapEnvironment(environment))
			if err == nil {
				t.Fatal("loadConfig() error = nil")
			}
			if strings.TrimSpace(tt.value) != "" && strings.Contains(err.Error(), tt.value) {
				t.Fatalf("configuration value leaked: %v", err)
			}
		})
	}
}

func TestRunRedactsStartupFailure(t *testing.T) {
	secret := "postgres://user:top-secret@127.0.0.1:1/workshop"
	config := appConfig{
		databaseURL:  secret,
		redisAddress: "127.0.0.1:1",
		redisStream:  "audit:sqloutbox",
		httpAddress:  "127.0.0.1:0",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := run(ctx, config, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || strings.Contains(err.Error(), "top-secret") || !strings.Contains(err.Error(), "database") {
		t.Fatalf("run() error = %v", err)
	}
}

func mapEnvironment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func cloneEnvironment(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
