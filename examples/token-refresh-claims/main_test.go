package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerSetsTimeouts(t *testing.T) {
	t.Parallel()

	handler := http.NewServeMux()
	server := newHTTPServer("127.0.0.1:8097", handler)
	if server.Addr != "127.0.0.1:8097" {
		t.Fatalf("Addr = %q, want 127.0.0.1:8097", server.Addr)
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
		{name: "loopback ipv4", value: "127.0.0.1:8098", want: "127.0.0.1:8098"},
		{name: "localhost", value: "localhost:8098", want: "localhost:8098"},
		{name: "loopback ipv6", value: "[::1]:8098", want: "[::1]:8098"},
		{name: "wildcard host", value: ":8098", wantErr: true},
		{name: "any ipv4", value: "0.0.0.0:8098", wantErr: true},
		{name: "non loopback", value: "192.168.1.10:8098", wantErr: true},
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
