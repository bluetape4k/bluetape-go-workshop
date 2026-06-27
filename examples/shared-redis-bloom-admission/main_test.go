package main

import (
	"net/http"
	"testing"
	"time"
)

func TestResolveHTTPAddrDefaultsToLoopback(t *testing.T) {
	got, err := resolveHTTPAddr("")
	if err != nil {
		t.Fatalf("resolveHTTPAddr() error = %v", err)
	}
	if got != defaultHTTPAddr {
		t.Fatalf("addr = %q, want %q", got, defaultHTTPAddr)
	}
}

func TestResolveHTTPAddrAllowsLoopbackOverride(t *testing.T) {
	got, err := resolveHTTPAddr("127.0.0.1:18102")
	if err != nil {
		t.Fatalf("resolveHTTPAddr() error = %v", err)
	}
	if got != "127.0.0.1:18102" {
		t.Fatalf("addr = %q", got)
	}
}

func TestResolveHTTPAddrRejectsNonLoopback(t *testing.T) {
	if _, err := resolveHTTPAddr("0.0.0.0:8102"); err == nil {
		t.Fatal("resolveHTTPAddr() error is nil, want rejection")
	}
}

func TestNewHTTPServerUsesBoundedTimeouts(t *testing.T) {
	server := newHTTPServer("127.0.0.1:18102", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	if server.ReadHeaderTimeout != 500*time.Millisecond {
		t.Fatalf("ReadHeaderTimeout = %s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 5*time.Second {
		t.Fatalf("ReadTimeout = %s", server.ReadTimeout)
	}
	if server.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %s", server.WriteTimeout)
	}
	if server.IdleTimeout != 30*time.Second {
		t.Fatalf("IdleTimeout = %s", server.IdleTimeout)
	}
}

func TestEnvParsingUsesFallbacksForInvalidValues(t *testing.T) {
	t.Setenv("UINT_TEST", "0")
	t.Setenv("FLOAT_TEST", "2")

	if got := uint64FromEnv("UINT_TEST", 42); got != 42 {
		t.Fatalf("uint64FromEnv() = %d, want 42", got)
	}
	if got := floatFromEnv("FLOAT_TEST", 0.01); got != 0.01 {
		t.Fatalf("floatFromEnv() = %v, want 0.01", got)
	}
}
