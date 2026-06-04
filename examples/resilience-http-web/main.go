// Package main runs the resilience HTTP web example.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/resilience-http-web/internal/resilienceweb"
)

func main() {
	catalogURL := env("CATALOG_URL", "http://localhost:9090")
	httpAddr := env("HTTP_ADDR", ":8081")

	server, err := resilienceweb.NewServer(resilienceweb.Options{CatalogURL: catalogURL})
	if err != nil {
		log.Fatalf("create resilience server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("resilience HTTP web example listening on %s, catalog=%s", httpAddr, catalogURL)
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
