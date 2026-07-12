package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAddressPolicy(t *testing.T) {
	for _, value := range []string{"", "127.0.0.1:9090", "[::1]:9090"} {
		address, err := resolveHTTPAddr(value, false)
		if err != nil {
			t.Fatalf("resolveHTTPAddr(%q): %v", value, err)
		}
		if value == "" && address != "127.0.0.1:8080" {
			t.Fatalf("default address = %q", address)
		}
	}
	for _, value := range []string{":8080", "0.0.0.0:8080", "[::]:8080", "192.168.1.10:8080", "203.0.113.10:8080"} {
		if _, err := resolveHTTPAddr(value, false); err == nil {
			t.Fatalf("resolveHTTPAddr(%q) accepted without opt-in", value)
		}
		if got, err := resolveHTTPAddr(value, true); err != nil || got != value {
			t.Fatalf("resolveHTTPAddr(%q, opt-in) = %q, %v", value, got, err)
		}
	}
}

func TestHTTPServerDefaults(t *testing.T) {
	server := NewHTTPServer("127.0.0.1:8080", http.NotFoundHandler())
	if server.Addr != "127.0.0.1:8080" || server.ReadHeaderTimeout != 2*time.Second || server.ReadTimeout != 5*time.Second || server.WriteTimeout != 5*time.Second || server.IdleTimeout != 30*time.Second {
		t.Fatalf("server = %+v", server)
	}
}

func TestRunServerEarlyListenResults(t *testing.T) {
	for name, listenErr := range map[string]error{"closed": http.ErrServerClosed, "failure": errors.New("listen failed")} {
		t.Run(name, func(t *testing.T) {
			server := newFakeHTTPServer(listenErr)
			err := RunServer(context.Background(), server, log.New(io.Discard, "", 0))
			if name == "closed" && err != nil {
				t.Fatalf("err = %v", err)
			}
			if name == "failure" && (err == nil || !strings.Contains(err.Error(), "listen")) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestRunServerGracefulCancellation(t *testing.T) {
	server := newFakeHTTPServer(nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunServer(ctx, server, log.New(io.Discard, "", 0)) }()
	<-server.started
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if server.shutdownCalls != 1 || server.closeCalls != 0 {
		t.Fatalf("shutdown=%d close=%d", server.shutdownCalls, server.closeCalls)
	}
}

func TestRunServerForcesCloseAndJoinsErrors(t *testing.T) {
	server := newFakeHTTPServer(nil)
	server.shutdownErr = context.DeadlineExceeded
	server.closeErr = errors.New("close failed")
	server.afterStopErr = errors.New("listen after close failed")
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- RunServer(ctx, server, log.New(&logs, "", 0)) }()
	<-server.started
	cancel()
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "shutdown") || !strings.Contains(err.Error(), "close") || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("err = %v", err)
	}
	if server.closeCalls != 1 || !server.sawFiveSecondDeadline {
		t.Fatalf("close=%d deadline=%v", server.closeCalls, server.sawFiveSecondDeadline)
	}
	if strings.Contains(logs.String(), "close failed") || strings.Contains(logs.String(), "listen after close failed") {
		t.Fatalf("unsafe lifecycle logs = %s", logs.String())
	}
	if !strings.Contains(logs.String(), "event=forced_close") {
		t.Fatalf("forced-close event missing: %s", logs.String())
	}
}

type fakeHTTPServer struct {
	started               chan struct{}
	stop                  chan struct{}
	listenErr             error
	afterStopErr          error
	shutdownErr           error
	closeErr              error
	shutdownCalls         int
	closeCalls            int
	sawFiveSecondDeadline bool
	once                  sync.Once
}

func newFakeHTTPServer(listenErr error) *fakeHTTPServer {
	return &fakeHTTPServer{started: make(chan struct{}), stop: make(chan struct{}), listenErr: listenErr}
}

func (s *fakeHTTPServer) ListenAndServe() error {
	close(s.started)
	if s.listenErr != nil {
		return s.listenErr
	}
	<-s.stop
	if s.afterStopErr != nil {
		return s.afterStopErr
	}
	return http.ErrServerClosed
}

func (s *fakeHTTPServer) Shutdown(ctx context.Context) error {
	s.shutdownCalls++
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		s.sawFiveSecondDeadline = remaining > 4*time.Second && remaining <= 5*time.Second
	}
	if s.shutdownErr == nil {
		s.once.Do(func() { close(s.stop) })
	}
	return s.shutdownErr
}

func (s *fakeHTTPServer) Close() error {
	s.closeCalls++
	s.once.Do(func() { close(s.stop) })
	return s.closeErr
}
