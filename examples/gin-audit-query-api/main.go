// Package main runs the Gin audit query API example.
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

	"github.com/bluetape4k/bluetape-go-workshop/examples/gin-audit-query-api/internal/auditquery"
	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/gin-gonic/gin"
)

const (
	defaultHTTPAddr = "127.0.0.1:8080"
	shutdownTimeout = 5 * time.Second
)

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
	repository := audit.NewMemoryRepository()
	if err := auditquery.SeedRepository(context.Background(), repository); err != nil {
		logger.Fatalf("fixture construction failed: %v", err)
	}
	service, err := auditquery.NewService(repository, auditquery.DefaultServiceConfig())
	if err != nil {
		logger.Fatalf("service construction failed: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)
	engine, err := auditquery.NewEngine(service, auditquery.DefaultHTTPConfig(), logger)
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
		return "", errors.New("invalid HTTP_ADDR port")
	}
	loopback := strings.EqualFold(host, "localhost")
	if !loopback && host != "" {
		address, parseErr := netip.ParseAddr(host)
		if parseErr == nil {
			loopback = address.IsLoopback()
		} else if !allowUnauthenticatedRemote {
			return "", errors.New("HTTP_ADDR host must be a loopback IP")
		}
	}
	if !loopback && !allowUnauthenticatedRemote {
		return "", errors.New("non-loopback HTTP_ADDR requires ALLOW_UNAUTHENTICATED_REMOTE=1")
	}
	return value, nil
}

func NewHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

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
