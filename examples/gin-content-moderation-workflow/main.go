// Package main 은 Gin content moderation workflow 예제를 실행한다.
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

	"github.com/bluetape4k/bluetape-go-workshop/examples/gin-content-moderation-workflow/internal/moderationapi"
	"github.com/gin-gonic/gin"
)

const (
	defaultHTTPAddr = "127.0.0.1:8080"
	shutdownTimeout = 5 * time.Second
)

// HTTPServer 는 RunServer가 사용하는 좁은 server lifecycle 계약이다.
type HTTPServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	address, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"), os.Getenv("ALLOW_UNAUTHENTICATED_REMOTE") == "1")
	if err != nil {
		logger.Fatalf("configuration failed: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)
	config := moderationapi.DefaultConfig()
	service, err := moderationapi.NewService(config.Service)
	if err != nil {
		logger.Fatalf("service construction failed: %v", err)
	}
	engine, err := moderationapi.NewEngine(service, config.HTTP, logger)
	if err != nil {
		logger.Fatalf("HTTP construction failed: %v", err)
	}
	server := NewHTTPServer(address, engine)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := RunServer(ctx, server, logger); err != nil {
		logger.Fatalf("server lifecycle failed: %v", err)
	}
}

func resolveHTTPAddr(value string, allowUnauthenticatedRemote bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultHTTPAddr
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return "", fmt.Errorf("invalid HTTP_ADDR: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid HTTP_ADDR port")
	}
	loopback := strings.EqualFold(host, "localhost")
	if !loopback && host != "" {
		address, parseErr := netip.ParseAddr(host)
		if parseErr == nil {
			loopback = address.IsLoopback()
		} else if !allowUnauthenticatedRemote {
			return "", fmt.Errorf("HTTP_ADDR host must be a loopback IP")
		}
	}
	if !loopback && !allowUnauthenticatedRemote {
		return "", fmt.Errorf("non-loopback HTTP_ADDR requires ALLOW_UNAUTHENTICATED_REMOTE=1")
	}
	return value, nil
}

// NewHTTPServer 는 예제의 고정 network timeout을 적용한다.
func NewHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

// RunServer 는 listen goroutine 하나를 소유하고 제한된 shutdown 뒤에 합류한다.
func RunServer(ctx context.Context, server HTTPServer, logger *log.Logger) error {
	if ctx == nil || server == nil || logger == nil {
		return errors.New("invalid server lifecycle dependency")
	}
	listenResult := make(chan error, 1)
	go func() {
		listenResult <- server.ListenAndServe()
	}()
	logger.Printf("event=server_started")

	select {
	case err := <-listenResult:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	logger.Printf("event=shutdown_started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	shutdownErr := server.Shutdown(shutdownCtx)
	cancel()
	var results []error
	if shutdownErr != nil {
		results = append(results, fmt.Errorf("shutdown: %w", shutdownErr))
		logger.Printf("event=shutdown_failed")
		logger.Printf("event=forced_close")
		if closeErr := server.Close(); closeErr != nil {
			results = append(results, fmt.Errorf("close: %w", closeErr))
			logger.Printf("event=close_failed")
		}
	}
	listenErr := <-listenResult
	if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
		results = append(results, fmt.Errorf("listen: %w", listenErr))
	}
	if len(results) == 0 {
		logger.Printf("event=shutdown_completed")
	}
	return errors.Join(results...)
}
