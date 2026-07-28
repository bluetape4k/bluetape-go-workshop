// Package main 은 결제 승인 상태 예제를 실행한다.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/payment-authorization-state/internal/paymentauth"
)

func main() {
	amountCents, err := envInt("PAYMENT_AMOUNT_CENTS", 12_900)
	if err != nil {
		log.Fatalf("parse PAYMENT_AMOUNT_CENTS: %v", err)
	}

	server, err := paymentauth.NewServer(paymentauth.Options{
		PaymentID:   env("PAYMENT_ID", "pay-1001"),
		AmountCents: amountCents,
	})
	if err != nil {
		log.Fatalf("create payment authorization state server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              env("HTTP_ADDR", ":8086"),
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("payment authorization state example listening on %s", httpServer.Addr)
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

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
