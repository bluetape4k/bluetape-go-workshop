// Package main 은 안전한 압축 해제 업로드 예제 서비스를 실행한다.
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

	"github.com/bluetape4k/bluetape-go-workshop/examples/safe-decompression-upload/internal/uploadguard"
	"github.com/gin-gonic/gin"
)

const (
	defaultHTTPAddr               = "127.0.0.1:8103"
	defaultCompressedLimitBytes   = int64(32 << 10)
	defaultDecompressedLimitBytes = int64(256 << 10)
	defaultReadHeaderTimeout      = 500 * time.Millisecond
	defaultReadTimeout            = 5 * time.Second
	defaultWriteTimeout           = 10 * time.Second
	defaultIdleTimeout            = 30 * time.Second
)

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)

	service, err := uploadguard.NewService(uploadguard.Config{
		CompressedBodyLimitBytes:   int64FromEnv("COMPRESSED_BODY_LIMIT_BYTES", defaultCompressedLimitBytes),
		DecompressedBodyLimitBytes: int64FromEnv("DECOMPRESSED_BODY_LIMIT_BYTES", defaultDecompressedLimitBytes),
	})
	if err != nil {
		log.Fatalf("create upload guard service: %v", err)
	}
	server := newHTTPServer(addr, uploadguard.NewRouter(service))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("safe decompression upload example listening on http://%s", addr)
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
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
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

func int64FromEnv(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}
