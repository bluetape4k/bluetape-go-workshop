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
	got, err := resolveHTTPAddr("127.0.0.1:18100")
	if err != nil {
		t.Fatalf("resolveHTTPAddr() error = %v", err)
	}
	if got != "127.0.0.1:18100" {
		t.Fatalf("addr = %q", got)
	}
}

func TestResolveHTTPAddrRejectsNonLoopback(t *testing.T) {
	if _, err := resolveHTTPAddr("0.0.0.0:8100"); err == nil {
		t.Fatal("resolveHTTPAddr() error is nil, want rejection")
	}
}

func TestNewHTTPServerUsesBoundedTimeouts(t *testing.T) {
	server := newHTTPServer("127.0.0.1:18100", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
