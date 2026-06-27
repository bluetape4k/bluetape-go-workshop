// Package main prints the SQL order repository preview.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/sql-order-repository/internal/orderrepo"
)

func main() {
	preview, err := orderrepo.NewPreview(orderrepo.Order{
		ID:         "order-1001",
		CustomerID: "customer-42",
		Status:     orderrepo.StatusPending,
		TotalCents: 2599,
		CreatedAt:  time.Date(2026, 6, 28, 9, 30, 0, 0, time.UTC),
	})
	if err != nil {
		log.Fatalf("build preview: %v", err)
	}

	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode preview: %v", err)
	}
	fmt.Println(string(encoded))
}
