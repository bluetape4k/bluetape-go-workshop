// Package main 은 S3 Floci 저장소 미리보기를 출력한다.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/s3-floci-storage/internal/receiptstore"
)

func main() {
	preview, err := receiptstore.NewPreview("tenant-receipts")
	if err != nil {
		log.Fatalf("build preview: %v", err)
	}

	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode preview: %v", err)
	}
	fmt.Println(string(encoded))
}
