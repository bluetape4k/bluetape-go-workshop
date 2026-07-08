// Package main runs the Gin text search service example.
package main

import (
	"context"
	"encoding/json"
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

	"github.com/bluetape4k/bluetape-go-workshop/examples/gin-text-search-service/internal/searchapi"
	"github.com/gin-gonic/gin"
)

const defaultHTTPAddr = "127.0.0.1:8098"

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}
	if os.Getenv("SERVE_HTTP") == "" {
		preview, err := searchapi.NewPreview()
		if err != nil {
			log.Fatalf("build preview: %v", err)
		}
		encoded, err := json.MarshalIndent(preview, "", "  ")
		if err != nil {
			log.Fatalf("encode preview: %v", err)
		}
		fmt.Println(string(encoded))
		fmt.Fprintln(os.Stderr, "set SERVE_HTTP=1 to run the Gin service")
		return
	}

	gin.SetMode(gin.ReleaseMode)
	service, err := searchapi.NewService(searchapi.DefaultPolicy())
	if err != nil {
		log.Fatalf("create search service: %v", err)
	}
	api, err := searchapi.NewServer(service)
	if err != nil {
		log.Fatalf("create search API: %v", err)
	}
	server := newHTTPServer(addr, api)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Gin text search service example listening on http://%s", addr)
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
