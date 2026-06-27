// Package main prints the DynamoDB batch write materializer preview.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/dynamodb-batchwrite-materializer/internal/docmaterializer"
)

func main() {
	preview, err := docmaterializer.NewPreview("document-index", docmaterializer.SampleEvents())
	if err != nil {
		log.Fatalf("build preview: %v", err)
	}

	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode preview: %v", err)
	}
	fmt.Println(string(encoded))
}
