// Package main prints the local SQS worker preview.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bluetape4k/bluetape-go-workshop/examples/sqs-floci-worker/internal/taskqueue"
)

func main() {
	preview := taskqueue.NewPreview("fulfillment-tasks")
	payload, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal preview: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(payload))
}
