package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/sqlkit"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisStream   = "audit:sqloutbox"
	maximumStreamBytes   = 256
	databasePoolSize     = 8
	redisPoolSize        = 8
	defaultShutdownLimit = 5 * time.Second
)

var errDependencyUnavailable = errors.New("orderworkflow: dependency unavailable")

// Probe checks one runtime dependency without exposing its endpoint or error.
type Probe func(context.Context) error

// DeliveryStatusReader supplies the redacted SQL outbox status snapshot.
type DeliveryStatusReader interface {
	DeliveryStatus(context.Context, time.Time) (DeliveryStatus, error)
}

// RuntimeHealth tracks relay ownership and separates Redis degradation from SQL readiness.
type RuntimeHealth struct {
	databaseProbe Probe
	redisProbe    Probe
	delivery      DeliveryStatusReader
	now           func() time.Time
	relayRunning  atomic.Bool
}

// NewRuntimeHealth constructs the application health model from bounded probes.
func NewRuntimeHealth(databaseProbe Probe, redisProbe Probe, delivery DeliveryStatusReader, now func() time.Time) (*RuntimeHealth, error) {
	if databaseProbe == nil || redisProbe == nil || delivery == nil || isNilInterface(delivery) || now == nil {
		return nil, ErrInvalidConfig
	}
	return &RuntimeHealth{databaseProbe: databaseProbe, redisProbe: redisProbe, delivery: delivery, now: now}, nil
}

// SetRelayRunning records the supervised relay lifecycle state.
func (h *RuntimeHealth) SetRelayRunning(running bool) {
	if h != nil {
		h.relayRunning.Store(running)
	}
}

// RelayRunning reports whether the supervised relay is expected to serve work.
func (h *RuntimeHealth) RelayRunning() bool {
	return h != nil && h.relayRunning.Load()
}

// Readiness reports SQL and relay readiness while treating Redis as degradable.
func (h *RuntimeHealth) Readiness(ctx context.Context) (Readiness, error) {
	if h == nil || h.databaseProbe == nil || h.redisProbe == nil {
		return Readiness{}, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	readiness := Readiness{RelayRunning: h.relayRunning.Load()}
	if err := h.databaseProbe(ctx); err != nil {
		return readiness, errDependencyUnavailable
	}
	readiness.DatabaseReady = true
	readiness.RedisReady = h.redisProbe(ctx) == nil
	return readiness, nil
}

// Status returns redacted Redis, relay, and aggregate delivery state.
func (h *RuntimeHealth) Status(ctx context.Context) (DeliverySnapshot, error) {
	if h == nil || h.redisProbe == nil || h.delivery == nil || h.now == nil {
		return DeliverySnapshot{}, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	status, err := h.delivery.DeliveryStatus(ctx, h.now())
	if err != nil {
		return DeliverySnapshot{}, errDependencyUnavailable
	}
	redisState := "available"
	if h.redisProbe(ctx) != nil {
		redisState = "unavailable"
	}
	relayState := "stopped"
	if h.relayRunning.Load() {
		relayState = "running"
	}
	return DeliverySnapshot{RedisState: redisState, RelayState: relayState, Delivery: status}, nil
}

// ConfigureDatabase applies the workshop's bounded PostgreSQL pool settings.
func ConfigureDatabase(db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(databasePoolSize)
	db.SetMaxIdleConns(databasePoolSize)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
}

// NewRedisOptions returns bounded Redis client settings for address.
func NewRedisOptions(address string) *redis.Options {
	return &redis.Options{
		Addr: address, PoolSize: redisPoolSize, MinIdleConns: 1,
		PoolTimeout: 2 * time.Second,
	}
}

// NewHTTPServer constructs a server with bounded headers, I/O, and idle time.
func NewHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: address, Handler: handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

// DefaultRelayOptions returns conservative teaching values for continuous delivery.
func DefaultRelayOptions() sqloutbox.RelayOptions {
	return sqloutbox.RelayOptions{
		ClaimLimit: 16, MaxAttempts: 3,
		RetryDelay: 250 * time.Millisecond, IdleDelay: 50 * time.Millisecond,
	}
}

// ValidateLoopbackAddress accepts only an IP-literal loopback host and valid port.
func ValidateLoopbackAddress(address string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || host == "" || port == "" || strings.Contains(host, "%") {
		return ErrInvalidConfig
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || !ip.IsLoopback() {
		return ErrInvalidConfig
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return ErrInvalidConfig
	}
	return nil
}

// ValidateRedisStream normalizes a bounded Redis stream name or returns the default.
func ValidateRedisStream(raw string) (string, error) {
	if raw == "" {
		return defaultRedisStream, nil
	}
	stream := strings.TrimSpace(raw)
	if !utf8.ValidString(stream) || stream == "" || len(stream) > maximumStreamBytes {
		return "", ErrInvalidConfig
	}
	return stream, nil
}

// RelayRunner owns continuous SQL outbox delivery until its context is canceled.
type RelayRunner interface {
	Run(context.Context, sqlkit.Session) error
}

// RunLifecycle supervises HTTP and relay execution under one bounded shutdown budget.
func RunLifecycle(
	ctx context.Context,
	server *http.Server,
	listener net.Listener,
	relay RelayRunner,
	db sqlkit.Session,
	health *RuntimeHealth,
	shutdownLimit time.Duration,
) error {
	if server == nil || listener == nil || relay == nil || isNilInterface(relay) || db == nil || health == nil || shutdownLimit <= 0 {
		return ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	relayCtx, cancelRelay := context.WithCancel(context.Background())
	defer cancelRelay()
	health.SetRelayRunning(true)
	relayDone := make(chan error, 1)
	go func() {
		err := relay.Run(relayCtx, db)
		health.SetRelayRunning(false)
		relayDone <- err
	}()
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()

	shutdownHTTP := func(deadline time.Time) error {
		shutdownCtx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return safeStageError("http_shutdown", "failed")
		}
		return nil
	}
	waitServer := func() error {
		err := <-serverDone
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return safeStageError("http_serve", "failed")
		}
		return nil
	}
	waitRelay := func(expected bool, deadline time.Time) error {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			health.SetRelayRunning(false)
			return safeStageError("relay_shutdown", "timeout")
		}
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case err := <-relayDone:
			if expected && (err == nil || errors.Is(err, context.Canceled)) {
				return nil
			}
			return safeStageError("relay", "stopped")
		case <-timer.C:
			health.SetRelayRunning(false)
			return safeStageError("relay_shutdown", "timeout")
		}
	}

	select {
	case <-ctx.Done():
		deadline := time.Now().Add(shutdownLimit)
		httpErr := shutdownHTTP(deadline)
		serverErr := waitServer()
		cancelRelay()
		relayErr := waitRelay(true, deadline)
		return errors.Join(httpErr, serverErr, relayErr)
	case err := <-relayDone:
		health.SetRelayRunning(false)
		deadline := time.Now().Add(shutdownLimit)
		httpErr := shutdownHTTP(deadline)
		serverErr := waitServer()
		if ctx.Err() != nil && errors.Is(err, context.Canceled) {
			return errors.Join(httpErr, serverErr)
		}
		return errors.Join(safeStageError("relay", "stopped"), httpErr, serverErr)
	case err := <-serverDone:
		deadline := time.Now().Add(shutdownLimit)
		cancelRelay()
		relayErr := waitRelay(true, deadline)
		if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
			return relayErr
		}
		return errors.Join(safeStageError("http_serve", "failed"), relayErr)
	}
}

func safeStageError(stage string, class string) error {
	return fmt.Errorf("%s: %s", stage, class)
}

// DefaultShutdownLimit returns the shared HTTP drain and relay join deadline.
func DefaultShutdownLimit() time.Duration {
	return defaultShutdownLimit
}
