// Package main 은 compensation workflow 예제를 실행한다.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/compensation-workflow/internal/compensation"
)

func main() {
	httpAddr := env("HTTP_ADDR", ":8087")

	server, err := compensation.NewServer(compensation.Options{})
	if err != nil {
		log.Fatalf("create compensation workflow server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("compensation workflow example listening on %s", httpAddr)
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
