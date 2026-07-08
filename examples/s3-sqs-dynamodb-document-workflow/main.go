// Package main prints the S3-SQS-DynamoDB document workflow preview.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow"
)

func main() {
	preview, err := documentworkflow.NewPreview("documents", "document-events", "document_processing")
	if err != nil {
		log.Fatalf("build preview: %v", err)
	}

	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode preview: %v", err)
	}
	fmt.Println(string(encoded))
}
