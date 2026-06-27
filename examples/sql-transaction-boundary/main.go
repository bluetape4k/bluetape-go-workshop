// Package main prints the SQL transaction boundary preview.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/sql-transaction-boundary/internal/orderplacement"
)

func main() {
	preview, err := orderplacement.NewPreview()
	if err != nil {
		log.Fatalf("build preview: %v", err)
	}

	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode preview: %v", err)
	}
	fmt.Println(string(encoded))
}
