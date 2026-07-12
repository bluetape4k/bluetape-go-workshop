package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/multilingual-language-routing/internal/routing"
)

func TestRunIsDeterministic(t *testing.T) {
	var first, second, stderr bytes.Buffer
	if code := run(nil, &first, &stderr); code != 0 {
		t.Fatalf("first run code=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("first run stderr=%q", stderr.String())
	}
	stderr.Reset()
	if code := run(nil, &second, &stderr); code != 0 {
		t.Fatalf("second run code=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("second run stderr=%q", stderr.String())
	}
	if first.String() != second.String() {
		t.Fatal("stdout differs")
	}
	if !strings.HasPrefix(first.String(), "{\n  \"") || !strings.HasSuffix(first.String(), "\n}\n") {
		t.Fatalf("stdout is not indented JSON: %q", first.String())
	}
}

func TestRunPreloadChangesOnlyLifecycleMetadata(t *testing.T) {
	var lazyOut, preloadOut, stderr bytes.Buffer
	if code := run(nil, &lazyOut, &stderr); code != 0 {
		t.Fatalf("lazy run code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"--preload"}, &preloadOut, &stderr); code != 0 {
		t.Fatalf("preloaded run code=%d stderr=%q", code, stderr.String())
	}
	var lazy, preloaded routing.Preview
	if err := json.Unmarshal(lazyOut.Bytes(), &lazy); err != nil {
		t.Fatalf("decode lazy preview: %v", err)
	}
	if err := json.Unmarshal(preloadOut.Bytes(), &preloaded); err != nil {
		t.Fatalf("decode preloaded preview: %v", err)
	}
	if !reflect.DeepEqual(lazy.Decisions, preloaded.Decisions) {
		t.Fatal("decisions differ")
	}
	if !reflect.DeepEqual(lazy.LowConfidenceFallback.Decision, preloaded.LowConfidenceFallback.Decision) {
		t.Fatal("low-confidence decisions differ")
	}
	if lazy.ModelLoading != "lazy" || preloaded.ModelLoading != "preloaded" {
		t.Fatalf("model loading = %q/%q", lazy.ModelLoading, preloaded.ModelLoading)
	}
	if lazy.Config.PreloadModels || !preloaded.Config.PreloadModels {
		t.Fatalf("preload config = %v/%v", lazy.Config.PreloadModels, preloaded.Config.PreloadModels)
	}
}

func TestRunFlagErrorsAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--unknown"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("unknown flag code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--preload") {
		t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunQuotesUnexpectedArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"bad\n\x1b[31m"}, &stdout, &stderr); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	want := "unexpected argument: \"bad\\n\\x1b[31m\"\n"
	if stderr.String() != want || strings.Contains(stderr.String(), "\x1b") {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunReturnsOneWhenEncodingFails(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(nil, failingWriter{}, &stderr); code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "encode multilingual language routing preview") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
