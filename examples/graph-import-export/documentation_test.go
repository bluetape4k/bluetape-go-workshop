package main

import (
	"os"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-import-export/internal/graphimport"
)

func TestDocumentationParity(t *testing.T) {
	english := readDocumentation(t, "README.md")
	korean := readDocumentation(t, "README.ko.md")
	rootEnglish := readDocumentation(t, "../../README.md")
	rootKorean := readDocumentation(t, "../../README.ko.md")

	for _, marker := range []string{
		"v0.10.0",
		"#52",
		"risk-partner-acme-v1",
		"acme-payments",
		"NDJSON",
		"GraphML",
		"64 KiB",
		"16 KiB",
		"64",
		"directed",
		"graph/graphio/graphml",
		"go run ./examples/graph-import-export",
		"go test -count=1 ./examples/graph-import-export/...",
		"go test -race -count=1 ./examples/graph-import-export/...",
	} {
		if !strings.Contains(english, marker) || !strings.Contains(korean, marker) {
			t.Errorf("README pair missing %q", marker)
		}
	}
	if !strings.Contains(english, "normalized snapshot") {
		t.Error("English README missing normalized snapshot terminology")
	}
	if !strings.Contains(korean, "정규화된 그래프 기준 데이터") {
		t.Error("Korean README missing localized normalized snapshot terminology")
	}

	demo, err := graphimport.RunDemo(t.Context())
	if err != nil {
		t.Fatalf("RunDemo() error = %v", err)
	}
	encoded, err := graphimport.EncodeDemo(demo)
	if err != nil {
		t.Fatalf("EncodeDemo() error = %v", err)
	}
	for _, marker := range []string{`"equivalent": true`, `"format": "ndjson"`, `"format": "graphml"`, `"directed": true`, `"risk_score": 72`, `"confidence": 0.98`} {
		if !strings.Contains(string(encoded), marker) {
			t.Errorf("demo output missing %q", marker)
		}
	}
	for _, document := range []string{english, korean} {
		for _, marker := range []string{"\"equivalent\": true", "\"directed\": true", "\"vertices\": 4", "\"edges\": 3", "\"id\": \"account-a\"", "\"from\": \"account-a\"", "\"to\": \"device-01\""} {
			if !strings.Contains(document, marker) {
				t.Errorf("README missing output marker %q", marker)
			}
		}
	}

	for _, marker := range []string{
		"examples/graph-import-export",
		"graph/graphio",
		"graph/graphio/graphml",
		"go run ./examples/graph-import-export",
	} {
		if !strings.Contains(rootEnglish, marker) || !strings.Contains(rootKorean, marker) {
			t.Errorf("root README pair missing %q", marker)
		}
	}
	assertMarkerOrder(t, rootEnglish,
		"| [`examples/graph-recommendation`]",
		"| [`examples/graph-import-export`]",
		"| [`examples/order-intake-cleanup`]")
	assertMarkerOrder(t, rootKorean,
		"| [`examples/graph-recommendation`]",
		"| [`examples/graph-import-export`]")
	assertMarkerOrder(t, rootEnglish,
		"## Run the Graph Recommendation Example",
		"## Run the Graph Import/Export Example",
		"## Run the S3 Floci Storage Example")
	assertMarkerOrder(t, rootKorean,
		"## Graph Recommendation 예제 실행",
		"## Graph Import/Export 예제 실행",
		"## S3 Floci Storage 예제 실행")
}

func readDocumentation(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertMarkerOrder(t *testing.T, document string, markers ...string) {
	t.Helper()
	previous := -1
	for _, marker := range markers {
		index := strings.Index(document, marker)
		if index < 0 {
			t.Errorf("missing ordered marker %q", marker)
			return
		}
		if index <= previous {
			t.Errorf("marker %q is out of order", marker)
			return
		}
		previous = index
	}
}
