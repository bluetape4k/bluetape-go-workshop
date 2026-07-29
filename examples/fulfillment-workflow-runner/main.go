// Package main 은 fulfillment workflow runner 예제를 실행한다.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/fulfillment-workflow-runner/internal/fulfillment"
)

func main() {
	httpAddr := env("HTTP_ADDR", ":8084")

	server, err := fulfillment.NewServer(fulfillment.Options{})
	if err != nil {
		log.Fatalf("create fulfillment workflow server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("fulfillment workflow runner example listening on %s", httpAddr)
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
