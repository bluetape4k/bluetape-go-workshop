// Package main 은 확률적 중복 제거 입장 예제 서비스를 실행한다.
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

	"github.com/bluetape4k/bluetape-go-workshop/examples/probabilistic-dedupe-admission/internal/dedupe"
	"github.com/gin-gonic/gin"
)

const defaultHTTPAddr = "127.0.0.1:8099"

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)

	service, err := dedupe.NewService(dedupe.Config{})
	if err != nil {
		log.Fatalf("create probabilistic dedupe service: %v", err)
	}
	server := newHTTPServer(addr, dedupe.NewRouter(service))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("probabilistic dedupe admission example listening on http://%s", addr)
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
