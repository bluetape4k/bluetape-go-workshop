// Package main 은 SQL 트랜잭션 경계 미리보기를 출력한다.
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
