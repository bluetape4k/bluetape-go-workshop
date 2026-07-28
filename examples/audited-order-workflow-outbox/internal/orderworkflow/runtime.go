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

// Probe 는 엔드포인트나 원본 오류를 노출하지 않고 런타임 의존성 하나를 점검한다.
type Probe func(context.Context) error

// DeliveryStatusReader 는 민감 정보가 제거된 SQL outbox 상태 스냅샷을 제공한다.
type DeliveryStatusReader interface {
	DeliveryStatus(context.Context, time.Time) (DeliveryStatus, error)
}

// RuntimeHealth 는 릴레이 소유권을 추적하고 Redis 저하 상태를 SQL readiness와 분리한다.
type RuntimeHealth struct {
	databaseProbe Probe
	redisProbe    Probe
	delivery      DeliveryStatusReader
	now           func() time.Time
	relayRunning  atomic.Bool
}

// NewRuntimeHealth 는 제한된 probe들로 애플리케이션 상태 모델을 구성한다.
func NewRuntimeHealth(databaseProbe Probe, redisProbe Probe, delivery DeliveryStatusReader, now func() time.Time) (*RuntimeHealth, error) {
	if databaseProbe == nil || redisProbe == nil || delivery == nil || isNilInterface(delivery) || now == nil {
		return nil, ErrInvalidConfig
	}
	return &RuntimeHealth{databaseProbe: databaseProbe, redisProbe: redisProbe, delivery: delivery, now: now}, nil
}

// SetRelayRunning 은 감독 대상 릴레이의 생명주기 상태를 기록한다.
func (h *RuntimeHealth) SetRelayRunning(running bool) {
	if h != nil {
		h.relayRunning.Store(running)
	}
}

// RelayRunning 은 감독 대상 릴레이가 작업을 처리해야 하는 상태인지 보고한다.
func (h *RuntimeHealth) RelayRunning() bool {
	return h != nil && h.relayRunning.Load()
}

// Readiness 는 Redis를 저하 가능 의존성으로 취급하면서 SQL과 릴레이 readiness를 보고한다.
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

// Status 는 민감 정보가 제거된 Redis, 릴레이, 집계 전달 상태를 반환한다.
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

// ConfigureDatabase 는 워크숍용 제한된 PostgreSQL pool 설정을 적용한다.
func ConfigureDatabase(db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(databasePoolSize)
	db.SetMaxIdleConns(databasePoolSize)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
}

// NewRedisOptions 는 address에 대한 제한된 Redis 클라이언트 설정을 반환한다.
func NewRedisOptions(address string) *redis.Options {
	return &redis.Options{
		Addr: address, PoolSize: redisPoolSize, MinIdleConns: 1,
		PoolTimeout: 2 * time.Second,
	}
}

// NewHTTPServer 는 헤더, I/O, idle 시간이 제한된 서버를 구성한다.
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

// DefaultRelayOptions 는 연속 전달 학습에 맞춘 보수적인 값을 반환한다.
func DefaultRelayOptions() sqloutbox.RelayOptions {
	return sqloutbox.RelayOptions{
		ClaimLimit: 16, MaxAttempts: 3,
		RetryDelay: 250 * time.Millisecond, IdleDelay: 50 * time.Millisecond,
	}
}

// ValidateLoopbackAddress 는 IP 리터럴 루프백 host와 유효한 port만 허용한다.
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

// ValidateRedisStream 은 제한된 Redis stream 이름을 정규화하거나 기본값을 반환한다.
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

// RelayRunner 는 context가 취소될 때까지 이어지는 SQL outbox 전달을 소유한다.
type RelayRunner interface {
	Run(context.Context, sqlkit.Session) error
}

// RunLifecycle 은 하나의 제한된 종료 예산 안에서 HTTP와 릴레이 실행을 감독한다.
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

// DefaultShutdownLimit 은 HTTP drain과 릴레이 join에 공유되는 제한 시간을 반환한다.
func DefaultShutdownLimit() time.Duration {
	return defaultShutdownLimit
}
