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
	got, err := resolveHTTPAddr("127.0.0.1:18103")
	if err != nil {
		t.Fatalf("resolveHTTPAddr() error = %v", err)
	}
	if got != "127.0.0.1:18103" {
		t.Fatalf("addr = %q", got)
	}
}

func TestResolveHTTPAddrRejectsNonLoopback(t *testing.T) {
	if _, err := resolveHTTPAddr("0.0.0.0:8103"); err == nil {
		t.Fatal("resolveHTTPAddr() error is nil, want rejection")
	}
}

func TestNewHTTPServerUsesBoundedTimeouts(t *testing.T) {
	server := newHTTPServer("127.0.0.1:18103", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	if server.ReadHeaderTimeout != defaultReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != defaultReadTimeout {
		t.Fatalf("ReadTimeout = %s", server.ReadTimeout)
	}
	if server.WriteTimeout != defaultWriteTimeout {
		t.Fatalf("WriteTimeout = %s", server.WriteTimeout)
	}
	if server.IdleTimeout != defaultIdleTimeout {
		t.Fatalf("IdleTimeout = %s", server.IdleTimeout)
	}
	if server.IdleTimeout != 30*time.Second {
		t.Fatalf("IdleTimeout = %s", server.IdleTimeout)
	}
}

func TestInt64FromEnvUsesFallbackForInvalidValues(t *testing.T) {
	t.Setenv("LIMIT_TEST", "-1")
	if got := int64FromEnv("LIMIT_TEST", 42); got != 42 {
		t.Fatalf("int64FromEnv() = %d, want 42", got)
	}
	t.Setenv("LIMIT_TEST", "128")
	if got := int64FromEnv("LIMIT_TEST", 42); got != 128 {
		t.Fatalf("int64FromEnv() = %d, want 128", got)
	}
}
