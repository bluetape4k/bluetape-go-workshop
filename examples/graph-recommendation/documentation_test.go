package main

import (
	"os"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-recommendation/internal/recommendation"
)

func TestDocumentationParity(t *testing.T) {
	english := readDocumentation(t, "README.md")
	korean := readDocumentation(t, "README.ko.md")
	rootEnglish := readDocumentation(t, "../../README.md")
	rootKorean := readDocumentation(t, "../../README.ko.md")

	for _, marker := range []string{
		"v0.10.0",
		"graph-recommendation-v1",
		"graph.Path",
		"PURCHASED",
		"FOLLOWS",
		"product_id",
		"score",
		"go run ./examples/graph-recommendation",
		"go test -count=1 ./examples/graph-recommendation/...",
		"go test -race -count=1 ./examples/graph-recommendation/...",
	} {
		if !strings.Contains(english, marker) || !strings.Contains(korean, marker) {
			t.Errorf("README pair missing %q", marker)
		}
	}

	fixture, err := recommendation.DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	graphValue, err := recommendation.NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	report, err := graphValue.Recommend(t.Context(), "alice", 10)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	wantJSON, err := recommendation.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport() error = %v", err)
	}
	for locale, document := range map[string]string{"en": english, "ko": korean} {
		if got := fencedBlock(t, document, "json"); got != string(wantJSON) {
			t.Errorf("%s documented JSON differs from EncodeReport():\ngot:\n%swant:\n%s", locale, got, wantJSON)
		}
	}

	for _, marker := range []string{
		"examples/graph-recommendation",
		"Graph Recommendation",
		"graph-recommendation-v1",
	} {
		if !strings.Contains(rootEnglish, marker) || !strings.Contains(rootKorean, marker) {
			t.Errorf("root README pair missing %q", marker)
		}
	}
	assertMarkerOrder(t, rootEnglish,
		"| [`examples/graph-abuse-cluster`]",
		"| [`examples/graph-recommendation`]",
		"| [`examples/order-intake-cleanup`]",
	)
	assertMarkerOrder(t, rootKorean,
		"| [`examples/graph-abuse-cluster`]",
		"| [`examples/graph-recommendation`]",
		"| [`examples/order-intake-cleanup`]",
	)
	assertMarkerOrder(t, rootEnglish,
		"## Run the Graph Abuse Cluster Example",
		"## Run the Graph Recommendation Example",
		"## Run the S3 Floci Storage Example",
	)
	assertMarkerOrder(t, rootKorean,
		"## Graph Abuse Cluster 예제 실행",
		"## Graph Recommendation 예제 실행",
		"## S3 Floci Storage 예제 실행",
	)
}

func readDocumentation(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func fencedBlock(t *testing.T, document, language string) string {
	t.Helper()
	startMarker := "```" + language + "\n"
	start := strings.Index(document, startMarker)
	if start < 0 {
		t.Fatalf("missing %s fenced block", language)
	}
	start += len(startMarker)
	end := strings.Index(document[start:], "```\n")
	if end < 0 {
		t.Fatalf("unterminated %s fenced block", language)
	}
	return document[start : start+end]
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
