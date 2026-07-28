// Package main 은 주문 이행 통합 HTTP 예제를 시작한다.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/bluetape4k/bluetape-go-workshop/examples/order-fulfillment-integration/internal/orderfulfillment"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8088"
	}

	server, err := orderfulfillment.NewServer(orderfulfillment.Options{})
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	log.Printf("order fulfillment integration API listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
