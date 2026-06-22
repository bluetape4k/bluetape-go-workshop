package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerSetsTimeouts(t *testing.T) {
	t.Parallel()

	handler := http.NewServeMux()
	server := newHTTPServer("127.0.0.1:8095", handler)
	if server.Addr != "127.0.0.1:8095" {
		t.Fatalf("Addr = %q, want 127.0.0.1:8095", server.Addr)
	}
	if server.Handler != handler {
		t.Fatalf("handler not installed")
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("timeouts must be set: %#v", server)
	}
	if server.ReadHeaderTimeout > time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, expected bounded timeout", server.ReadHeaderTimeout)
	}
}

func TestResolveHTTPAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "default", want: defaultHTTPAddr},
		{name: "loopback ipv4", value: "127.0.0.1:8096", want: "127.0.0.1:8096"},
		{name: "localhost", value: "localhost:8096", want: "localhost:8096"},
		{name: "loopback ipv6", value: "[::1]:8096", want: "[::1]:8096"},
		{name: "wildcard host", value: ":8096", wantErr: true},
		{name: "any ipv4", value: "0.0.0.0:8096", wantErr: true},
		{name: "non loopback", value: "192.168.1.10:8096", wantErr: true},
		{name: "invalid", value: "not an addr", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveHTTPAddr(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveHTTPAddr(%q) error = nil, want error", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveHTTPAddr(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("resolveHTTPAddr(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestLeaderModeFromEnv(t *testing.T) {
	t.Parallel()

	if !leaderHeldFromMode("") {
		t.Fatalf("empty leader mode should default to held")
	}
	if !leaderHeldFromMode("held") {
		t.Fatalf("held leader mode should be held")
	}
	if leaderHeldFromMode("missing") {
		t.Fatalf("missing leader mode should not be held")
	}
}
