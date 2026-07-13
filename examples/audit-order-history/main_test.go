package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/audit-order-history/internal/orderhistory"
)

func TestRunPrintsDeterministicJSON(t *testing.T) {
	t.Parallel()

	var first bytes.Buffer
	if err := run(&first); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	var second bytes.Buffer
	if err := run(&second); err != nil {
		t.Fatalf("run() second error = %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("run output differs:\n%s\n%s", first.String(), second.String())
	}
	want, err := os.ReadFile("testdata/preview.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), want) {
		t.Fatalf("run output differs from golden:\n%s", first.String())
	}
	if first.Len() == 0 || first.Bytes()[first.Len()-1] != '\n' {
		t.Fatalf("run output must end with newline: %q", first.String())
	}
	var preview orderhistory.Preview
	if err := json.Unmarshal(first.Bytes(), &preview); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, first.String())
	}
	if preview.Current.Status != orderhistory.StatusShipped || len(preview.History) != 3 || len(preview.RecentHistory) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
}

func TestRunPropagatesWriterFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("write failed")
	err := run(errorWriter{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("run(errorWriter) error = %v, want %v", err, wantErr)
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }
