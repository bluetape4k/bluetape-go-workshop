// Package main 은 chunked CSV import checkpoint 예제를 실행한다.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bluetape4k/bluetape-go-workshop/examples/chunked-csv-import-checkpoint/internal/csvimport"
)

func main() {
	result, err := csvimport.RunDemo(context.Background(), csvPath())
	if err != nil {
		log.Fatalf("run chunked csv import checkpoint demo: %v", err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("encode result: %v", err)
	}
	fmt.Println(string(encoded))
}

func csvPath() string {
	if path := os.Getenv("CUSTOMER_CSV"); path != "" {
		return path
	}
	for _, candidate := range []string{
		filepath.Join("examples", "chunked-csv-import-checkpoint", "testdata", "customers.csv"),
		filepath.Join("testdata", "customers.csv"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("examples", "chunked-csv-import-checkpoint", "testdata", "customers.csv")
}
