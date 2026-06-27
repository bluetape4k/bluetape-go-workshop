// Package main runs the shared Redis Bloom admission example service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/shared-redis-bloom-admission/internal/redisadmission"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	defaultHTTPAddr  = "127.0.0.1:8102"
	defaultRedisAddr = "127.0.0.1:6379"
)

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}
	redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if redisAddr == "" {
		redisAddr = defaultRedisAddr
	}
	gin.SetMode(gin.ReleaseMode)

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() { _ = redisClient.Close() }()

	service, err := redisadmission.NewService(context.Background(), redisClient, redisadmission.Config{
		Namespace:                envOrDefault("BLOOM_NAMESPACE", "shared-redis-bloom-admission:webhooks"),
		InstanceID:               envOrDefault("INSTANCE_ID", "api-instance-local"),
		ExpectedInsertions:       uint64FromEnv("BLOOM_EXPECTED_INSERTIONS", 10_000),
		FalsePositiveProbability: floatFromEnv("BLOOM_FALSE_POSITIVE_PROBABILITY", 0.01),
	})
	if err != nil {
		log.Fatalf("create shared redis bloom admission service: %v", err)
	}
	server := newHTTPServer(addr, redisadmission.NewRouter(service))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("shared Redis Bloom admission example listening on http://%s", addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func resolveHTTPAddr(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultHTTPAddr
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("port is required")
	}
	if host == "localhost" {
		return value, nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", err
	}
	if !addr.IsLoopback() {
		return "", fmt.Errorf("HTTP_ADDR must bind to loopback, got %q", value)
	}
	return value, nil
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func uint64FromEnv(name string, fallback uint64) uint64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return fallback
	}
	return parsed
}

func floatFromEnv(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 || parsed >= 1 {
		return fallback
	}
	return parsed
}
