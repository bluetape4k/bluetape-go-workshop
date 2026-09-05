package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-import-export/internal/graphimport"
)

func TestRunPrintsDocumentedDemo(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	demo, err := graphimport.RunDemo(context.Background())
	if err != nil {
		t.Fatalf("RunDemo() error = %v", err)
	}
	want, err := graphimport.EncodeDemo(demo)
	if err != nil {
		t.Fatalf("EncodeDemo() error = %v", err)
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

func TestRunRejectsNilInputs(t *testing.T) {
	var nilContext context.Context
	if err := run(nilContext, &bytes.Buffer{}); !errors.Is(err, graphimport.ErrInvalidContext) {
		t.Fatalf("nil context error = %v, want ErrInvalidContext", err)
	}
	if err := run(context.Background(), nil); !errors.Is(err, graphimport.ErrInvalidInput) {
		t.Fatalf("nil writer error = %v, want ErrInvalidInput", err)
	}
}
