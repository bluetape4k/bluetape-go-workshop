// Package main 은 Japanese search preparation preview를 출력한다.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/japanese-search-preparation/internal/catalogprep"
)

func main() {
	preview, err := catalogprep.NewPreview()
	if err != nil {
		log.Fatalf("build Japanese search preview: %v", err)
	}
	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode Japanese search preview: %v", err)
	}
	fmt.Println(string(encoded))
}
