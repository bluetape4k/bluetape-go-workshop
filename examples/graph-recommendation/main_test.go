package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-recommendation/internal/recommendation"
)

func TestRunPrintsDocumentedReport(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	fixture, err := recommendation.DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	graphValue, err := recommendation.NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	report, err := graphValue.Recommend(context.Background(), "alice", 10)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	want, err := recommendation.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport() error = %v", err)
	}
	if output.String() != string(want) {
		t.Fatalf("run() output = %s, want %s", output.String(), want)
	}
}

func TestRunHonorsCanceledContextBeforeWriting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if err := run(ctx, &output); !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want context.Canceled", err)
	}
	if output.Len() != 0 {
		t.Fatalf("canceled run wrote %q", output.String())
	}
}
