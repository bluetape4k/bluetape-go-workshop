// Package main 은 다국어 접수 가능성 미리보기를 출력한다.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/bluetape4k/bluetape-go-workshop/examples/multilingual-intake-feasibility/internal/intake"
)

func main() {
	preview, err := intake.NewPreview()
	if err != nil {
		log.Fatalf("build multilingual intake preview: %v", err)
	}
	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		log.Fatalf("encode multilingual intake preview: %v", err)
	}
	fmt.Println(string(encoded))
}
