// Package main 은 DynamoDB conditional repository preview를 출력한다.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/dynamodb-conditional-repository/internal/catalogrepo"
)

func main() {
	preview, err := catalogrepo.NewPreview("catalog_items", catalogrepo.SampleItems())
	if err != nil {
		log.Fatalf("build preview: %v", err)
	}

	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode preview: %v", err)
	}
	fmt.Println(string(encoded))
}
