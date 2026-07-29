// Package main 은 distributed JWT key rotation 예제 서비스를 실행한다.
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
	"strings"
	"syscall"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/distributed-jwt-key-rotation/internal/distributedjwt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const defaultHTTPAddr = "127.0.0.1:8099"

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	namespace := env("JWT_NAMESPACE", "workshop-auth")
	nodeID := env("NODE_ID", "api-1")

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() { _ = client.Close() }()

	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelSetup()
	if err := client.Ping(setupCtx).Err(); err != nil {
		log.Fatalf("ping redis at %s: %v", redisAddr, err)
	}

	gin.SetMode(gin.ReleaseMode)
	service, err := distributedjwt.NewService(setupCtx, client,
		distributedjwt.WithNamespace(namespace),
		distributedjwt.WithNodeID(nodeID),
	)
	if err != nil {
		log.Fatalf("create distributed JWT service: %v", err)
	}
	server := newHTTPServer(addr, distributedjwt.NewRouter(service))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("distributed JWT example listening on http://%s, redis=%s, namespace=%s, node=%s", addr, redisAddr, namespace, nodeID)
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

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
