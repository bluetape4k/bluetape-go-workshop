// Package main runs the order lifecycle state API example.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/order-lifecycle-state-api/internal/orderstate"
)

func main() {
	httpAddr := env("HTTP_ADDR", ":8083")
	orderID := env("ORDER_ID", "order-1001")
	totalCents := envInt("ORDER_TOTAL_CENTS", 12_900)

	server, err := orderstate.NewServer(orderstate.Options{
		OrderID:    orderID,
		TotalCents: totalCents,
	})
	if err != nil {
		log.Fatalf("create order lifecycle server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("order lifecycle state API example listening on %s, order=%s total_cents=%d", httpAddr, orderID, totalCents)
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

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
