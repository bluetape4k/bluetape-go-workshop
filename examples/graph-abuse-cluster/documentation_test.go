package main

import (
	"os"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-abuse-cluster/internal/abusecluster"
)

func TestDocumentationParity(t *testing.T) {
	t.Helper()

	english := readDocumentation(t, "README.md")
	korean := readDocumentation(t, "README.ko.md")
	rootEnglish := readDocumentation(t, "../../README.md")
	rootKorean := readDocumentation(t, "../../README.ko.md")

	sharedMarkers := []string{
		"../../docs/images/readme-diagrams/graph-abuse-cluster-architecture.png",
		"../../docs/images/readme-diagrams/graph-abuse-cluster-sequence.png",
		"NEO4J_URI=bolt://127.0.0.1:7687",
		"NEO4J_AUTH=none",
		"-p 127.0.0.1:7687:7687",
		"neo4j:5.26.0",
		"`payment_token` | 5",
		"`device` | 3",
		"`ip` | 1",
		"256",
		"1024",
		"graph-abuse-cluster-v1",
		"cluster:usr-001",
		"cluster:usr-004",
		`"risk_score": 4`,
		`"isolated_users": [`,
		"go run ./examples/graph-abuse-cluster",
		"go test -count=1 ./examples/graph-abuse-cluster/...",
		"go test -p 1 -race -count=1 ./examples/graph-abuse-cluster/...",
		"graph/neo4j",
		"Neo4j Testcontainers",
		"Memgraph",
	}

	for _, marker := range sharedMarkers {
		marker := marker
		t.Run(marker, func(t *testing.T) {
			for locale, document := range map[string]string{"en": english, "ko": korean} {
				if !strings.Contains(document, marker) {
					t.Errorf("%s README missing %q", locale, marker)
				}
			}
			if strings.Count(english, marker) != strings.Count(korean, marker) {
				t.Errorf("marker %q count differs: en=%d ko=%d", marker, strings.Count(english, marker), strings.Count(korean, marker))
			}
		})
	}

	for _, document := range []string{english, korean} {
		if strings.Index(document, "graph-abuse-cluster-architecture.png") > strings.Index(document, "graph-abuse-cluster-sequence.png") {
			t.Error("architecture diagram must precede sequence diagram")
		}
		if strings.Count(document, "graph-abuse-cluster-v1") < 1 {
			t.Error("fixed namespace concurrency boundary is missing")
		}
	}

	fixture, err := abusecluster.DefaultFixture()
	if err != nil {
		t.Fatalf("build documented fixture: %v", err)
	}
	report, err := abusecluster.Analyze(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("analyze documented fixture: %v", err)
	}
	wantJSON, err := abusecluster.EncodeReport(report)
	if err != nil {
		t.Fatalf("encode documented report: %v", err)
	}
	for locale, document := range map[string]string{"en": english, "ko": korean} {
		if got := fencedBlock(t, document, "json"); got != string(wantJSON) {
			t.Errorf("%s documented JSON differs from EncodeReport:\ngot:\n%swant:\n%s", locale, got, wantJSON)
		}
	}

	rootMarkers := []string{
		"examples/graph-abuse-cluster",
		"`graph`, `graph/neo4j`, Neo4j Testcontainers",
		"NEO4J_URI=bolt://127.0.0.1:7687 go run ./examples/graph-abuse-cluster",
	}
	for _, marker := range rootMarkers {
		if !strings.Contains(rootEnglish, marker) || !strings.Contains(rootKorean, marker) {
			t.Errorf("root README pair missing %q", marker)
		}
	}
	assertMarkerOrder(t, rootEnglish,
		"| [`examples/audited-order-workflow-outbox`]",
		"| [`examples/graph-abuse-cluster`]",
		"| [`examples/order-intake-cleanup`]",
	)
	assertMarkerOrder(t, rootKorean,
		"| [`examples/audited-order-workflow-outbox`]",
		"| [`examples/graph-abuse-cluster`]",
		"| [`examples/order-intake-cleanup`]",
	)
	assertMarkerOrder(t, rootEnglish,
		"## Run the Audited Order Workflow Outbox Example",
		"## Run the Graph Abuse Cluster Example",
		"## Run the S3 Floci Storage Example",
	)
	assertMarkerOrder(t, rootKorean,
		"## Audited Order Workflow Outbox 예제 실행",
		"## Graph Abuse Cluster 예제 실행",
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
