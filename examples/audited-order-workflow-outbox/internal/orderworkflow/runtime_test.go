package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestRuntimeLoopbackAddressValidation(t *testing.T) {
	valid := []string{"127.0.0.1:8080", "127.8.9.10:1", "[::1]:8080"}
	for _, address := range valid {
		if err := ValidateLoopbackAddress(address); err != nil {
			t.Fatalf("ValidateLoopbackAddress(%q) error = %v", address, err)
		}
	}
	invalid := []string{"", ":8080", "0.0.0.0:8080", "[::]:8080", "localhost:8080", "example.com:8080", "192.0.2.1:8080", "127.0.0.1", "[fe80::1%lo0]:8080", "127.0.0.1:notaport"}
	for _, address := range invalid {
		if err := ValidateLoopbackAddress(address); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("ValidateLoopbackAddress(%q) error = %v", address, err)
		}
	}
}

func TestRuntimeRedisStreamValidation(t *testing.T) {
	if got, err := ValidateRedisStream(""); err != nil || got != "audit:sqloutbox" {
		t.Fatalf("default stream = (%q, %v)", got, err)
	}
	if got, err := ValidateRedisStream(" workshop:orders "); err != nil || got != "workshop:orders" {
		t.Fatalf("custom stream = (%q, %v)", got, err)
	}
	for _, value := range []string{" ", string([]byte{0xff}), strings.Repeat("a", 257)} {
		if _, err := ValidateRedisStream(value); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("ValidateRedisStream(%q) error = %v", value, err)
		}
	}
}

func TestRuntimeBoundedResources(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ConfigureDatabase(db)
	stats := db.Stats()
	if stats.MaxOpenConnections != 8 {
		t.Fatalf("MaxOpenConnections = %d", stats.MaxOpenConnections)
	}
	redisOptions := NewRedisOptions("127.0.0.1:6379")
	if redisOptions.PoolSize != 8 || redisOptions.MinIdleConns != 1 || redisOptions.PoolTimeout != 2*time.Second {
		t.Fatalf("redis options = %+v", redisOptions)
	}
	server := NewHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 2*time.Second || server.ReadTimeout != 5*time.Second || server.WriteTimeout != 5*time.Second ||
		server.IdleTimeout != 30*time.Second || server.MaxHeaderBytes != 16<<10 {
		t.Fatalf("http server = %+v", server)
	}
	options := DefaultRelayOptions()
	if options.ClaimLimit != 16 || options.MaxAttempts != 3 || options.RetryDelay != 250*time.Millisecond || options.IdleDelay != 50*time.Millisecond {
		t.Fatalf("relay options = %+v", options)
	}
}

func TestRuntimeHealthSeparatesRedisDegradation(t *testing.T) {
	delivery := &stubDeliveryReader{status: DeliveryStatus{Pending: 2}}
	health, err := NewRuntimeHealth(
		func(context.Context) error { return nil },
		func(context.Context) error { return errors.New("redis-secret") },
		delivery,
		func() time.Time { return testWorkflowNow },
	)
	if err != nil {
		t.Fatal(err)
	}
	health.SetRelayRunning(true)
	ready, err := health.Readiness(context.Background())
	if err != nil || !ready.DatabaseReady || !ready.RelayRunning || ready.RedisReady {
		t.Fatalf("Readiness() = (%+v, %v)", ready, err)
	}
	status, err := health.Status(context.Background())
	if err != nil || status.RedisState != "unavailable" || status.RelayState != "running" || status.Delivery.Pending != 2 {
		t.Fatalf("Status() = (%+v, %v)", status, err)
	}
}

func TestRuntimeLifecycleExpectedCancellationAndUnexpectedRelayExit(t *testing.T) {
	t.Run("expected cancellation joins", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("pgx", "postgres://unused")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		health := mustRuntimeHealth(t)
		runner := relayRunnerFunc(func(ctx context.Context, _ sqlkit.Session) error {
			<-ctx.Done()
			return ctx.Err()
		})
		server := NewHTTPServer(listener.Addr().String(), http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(204) }))
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- RunLifecycle(ctx, server, listener, runner, db, health, 250*time.Millisecond) }()
		waitForHTTPServer(t, listener.Addr().String())
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("RunLifecycle() error = %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("lifecycle did not join")
		}
		if health.RelayRunning() {
			t.Fatal("relay remains marked running")
		}
	})

	t.Run("unexpected relay exit shuts down HTTP with redacted error", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("pgx", "postgres://unused")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		health := mustRuntimeHealth(t)
		release := make(chan struct{})
		runner := relayRunnerFunc(func(context.Context, sqlkit.Session) error {
			<-release
			return errors.New("provider-secret-marker")
		})
		server := NewHTTPServer(listener.Addr().String(), http.NotFoundHandler())
		done := make(chan error, 1)
		go func() {
			done <- RunLifecycle(context.Background(), server, listener, runner, db, health, 250*time.Millisecond)
		}()
		waitForHTTPServer(t, listener.Addr().String())
		close(release)
		select {
		case err := <-done:
			if err == nil || strings.Contains(err.Error(), "provider-secret-marker") || !strings.Contains(err.Error(), "relay") {
				t.Fatalf("RunLifecycle() error = %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("unexpected relay exit did not stop lifecycle")
		}
		if health.RelayRunning() {
			t.Fatal("relay remains marked running")
		}
	})

	t.Run("relay that ignores cancellation cannot exceed shutdown deadline", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("pgx", "postgres://unused")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		health := mustRuntimeHealth(t)
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		runner := relayRunnerFunc(func(context.Context, sqlkit.Session) error {
			<-release
			return errors.New("provider-secret-marker")
		})
		server := NewHTTPServer(listener.Addr().String(), http.NotFoundHandler())
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- RunLifecycle(ctx, server, listener, runner, db, health, 100*time.Millisecond) }()
		waitForHTTPServer(t, listener.Addr().String())
		started := time.Now()
		cancel()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "relay_shutdown") || strings.Contains(err.Error(), "provider-secret-marker") {
				t.Fatalf("RunLifecycle() error = %v", err)
			}
			if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
				t.Fatalf("shutdown elapsed = %v", elapsed)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("relay join exceeded shutdown deadline")
		}
		if health.RelayRunning() {
			t.Fatal("relay remains marked running after deadline")
		}
	})
}

type stubDeliveryReader struct {
	status DeliveryStatus
	err    error
}

func (s *stubDeliveryReader) DeliveryStatus(context.Context, time.Time) (DeliveryStatus, error) {
	return s.status, s.err
}

type relayRunnerFunc func(context.Context, sqlkit.Session) error

func (f relayRunnerFunc) Run(ctx context.Context, db sqlkit.Session) error { return f(ctx, db) }

func mustRuntimeHealth(t *testing.T) *RuntimeHealth {
	t.Helper()
	health, err := NewRuntimeHealth(func(context.Context) error { return nil }, func(context.Context) error { return nil }, &stubDeliveryReader{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return health
}

func waitForHTTPServer(t *testing.T, address string) {
	t.Helper()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get("http://" + address)
		if err == nil {
			_ = response.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("HTTP server did not start")
}
