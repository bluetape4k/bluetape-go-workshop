// Package main 은 운영 리포트 정책 예제를 실행한다.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/operations-report-policy/internal/operations"
)

func main() {
	httpAddr := env("HTTP_ADDR", ":8085")

	server, err := operations.NewServer(operations.Options{})
	if err != nil {
		log.Fatalf("create operations report policy server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("operations report policy example listening on %s", httpAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
