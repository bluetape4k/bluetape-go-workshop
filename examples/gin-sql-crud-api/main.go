// Package main 은 Gin SQL CRUD API 예제 서비스를 실행한다.
package main

import (
	"context"
	"database/sql"
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

	"github.com/bluetape4k/bluetape-go-workshop/examples/gin-sql-crud-api/internal/ordersapi"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultHTTPAddr = "127.0.0.1:8097"

func main() {
	addr, err := resolveHTTPAddr(os.Getenv("HTTP_ADDR"))
	if err != nil {
		log.Fatalf("invalid HTTP_ADDR: %v", err)
	}

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		preview, err := ordersapi.NewPreview()
		if err != nil {
			log.Fatalf("build preview: %v", err)
		}
		encoded, err := json.MarshalIndent(preview, "", "  ")
		if err != nil {
			log.Fatalf("encode preview: %v", err)
		}
		fmt.Println(string(encoded))
		fmt.Fprintln(os.Stderr, "set DATABASE_URL to run the HTTP service against PostgreSQL")
		return
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close postgres: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}
	if err := ordersapi.Migrate(ctx, db); err != nil {
		log.Fatalf("migrate orders schema: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	api, err := ordersapi.NewServer(ordersapi.Options{DB: db})
	if err != nil {
		log.Fatalf("create orders API: %v", err)
	}
	server := newHTTPServer(addr, api)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Gin SQL CRUD API example listening on http://%s", addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
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
