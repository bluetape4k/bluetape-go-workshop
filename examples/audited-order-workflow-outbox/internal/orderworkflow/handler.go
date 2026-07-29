package orderworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/gin-gonic/gin"
)

var (
	errRequestTooLarge            = errors.New("request body is too large")
	errUnsupportedMediaType       = errors.New("unsupported media type")
	errUnsupportedContentEncoding = errors.New("unsupported content encoding")
	errInvalidHTTPRequest         = errors.New("invalid HTTP request")
	requestSequence               atomic.Uint64
)

// HTTPConfig 는 요청 크기, 처리 시간, 동시 실행 요청 수의 상한을 정의한다.
type HTTPConfig struct {
	MaximumBodyBytes  int64
	OperationTimeout  time.Duration
	MaximumConcurrent int
}

// DefaultHTTPConfig 는 루프백 워크숍 서버에 맞춘 보수적인 설정을 반환한다.
func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{MaximumBodyBytes: 32 << 10, OperationTimeout: 2 * time.Second, MaximumConcurrent: 32}
}

// CommandService 는 검증된 주문 명령을 적용하고 표준 replay 여부를 보고한다.
type CommandService interface {
	Create(context.Context, CreateCommand) (Order, bool, error)
	Transition(context.Context, TransitionCommand) (Order, bool, error)
}

// Readiness 는 영속 명령 처리 가능 여부와 Redis 전달 상태를 분리해 표현한다.
type Readiness struct {
	DatabaseReady bool
	RelayRunning  bool
	RedisReady    bool
}

// DeliverySnapshot 은 제한된 운영용 전달 카운터와 상태를 담는다.
type DeliverySnapshot struct {
	RedisState string
	RelayState string
	Delivery   DeliveryStatus
}

// HealthReader 는 readiness와 민감 정보가 제거된 전달 진단값을 제공한다.
type HealthReader interface {
	Readiness(context.Context) (Readiness, error)
	Status(context.Context) (DeliverySnapshot, error)
}

// NewEngine 은 엄격한 JSON 명령, 감사, 상태 점검 라우트를 구성한다.
func NewEngine(service CommandService, reader audit.HistoryReader, health HealthReader, config HTTPConfig, logger *slog.Logger) (*gin.Engine, error) {
	if service == nil || isNilInterface(service) || reader == nil || isNilInterface(reader) ||
		health == nil || isNilInterface(health) || logger == nil || config.MaximumBodyBytes <= 0 ||
		config.OperationTimeout <= 0 || config.MaximumConcurrent <= 0 {
		return nil, ErrInvalidConfig
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = false
	engine.UseRawPath = true
	engine.UnescapePathValues = false
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("%w: trusted proxies", ErrInvalidConfig)
	}
	adapter := &httpAdapter{
		service: service, reader: reader, health: health,
		config: config, logger: logger, semaphore: make(chan struct{}, config.MaximumConcurrent),
	}
	engine.Use(adapter.requestID, adapter.closeBody, adapter.recoverPanic, adapter.limitConcurrency)
	engine.NoRoute(adapter.notFound)
	engine.NoMethod(adapter.methodNotAllowed)
	engine.POST("/orders", adapter.createOrder)
	engine.POST("/orders/transitions", adapter.transitionOrder)
	engine.POST("/audit/history/search", adapter.searchHistory)
	engine.POST("/audit/history/detail", adapter.historyDetail)
	engine.GET("/healthz", adapter.healthz)
	engine.GET("/readyz", adapter.readyz)
	engine.GET("/statusz", adapter.statusz)
	return engine, nil
}

type httpAdapter struct {
	service   CommandService
	reader    audit.HistoryReader
	health    HealthReader
	config    HTTPConfig
	logger    *slog.Logger
	semaphore chan struct{}
}

type publicHTTPError struct {
	status  int
	code    string
	message string
}

type aggregateJSON struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type createOrderJSON struct {
	OrderID   string         `json:"order_id"`
	CommandID string         `json:"command_id"`
	Metadata  audit.Metadata `json:"metadata,omitempty"`
}

type transitionOrderJSON struct {
	OrderID   string         `json:"order_id"`
	CommandID string         `json:"command_id"`
	Action    Action         `json:"action"`
	Reason    string         `json:"reason,omitempty"`
	Metadata  audit.Metadata `json:"metadata,omitempty"`
}

type searchHistoryJSON struct {
	Aggregate      aggregateJSON `json:"aggregate"`
	FromRevision   json.Number   `json:"from_revision"`
	ToRevision     json.Number   `json:"to_revision"`
	FromRecordedAt *time.Time    `json:"from_recorded_at"`
	ToRecordedAt   *time.Time    `json:"to_recorded_at"`
	Limit          json.Number   `json:"limit"`
}

type historyDetailJSON struct {
	Aggregate aggregateJSON `json:"aggregate"`
	Revision  json.Number   `json:"revision"`
}

func (a *httpAdapter) requestID(c *gin.Context) {
	id := fmt.Sprintf("req-%016x", requestSequence.Add(1))
	c.Set("request_id", id)
	c.Header("X-Request-ID", id)
	c.Next()
}

func (a *httpAdapter) closeBody(c *gin.Context) {
	defer func() {
		if c.Request != nil && c.Request.Body != nil {
			_ = c.Request.Body.Close()
		}
	}()
	c.Next()
}

func (a *httpAdapter) recoverPanic(c *gin.Context) {
	defer func() {
		if recover() != nil {
			a.logger.Error("http request failed", "stage", "handler", "class", "panic", "request_id", requestID(c))
			a.respondError(c, publicHTTPError{status: 500, code: "internal_error", message: "internal error"})
			c.Abort()
		}
	}()
	c.Next()
}

func (a *httpAdapter) limitConcurrency(c *gin.Context) {
	select {
	case a.semaphore <- struct{}{}:
		defer func() { <-a.semaphore }()
		c.Next()
	default:
		c.Header("Retry-After", "1")
		c.Header("Connection", "close")
		a.respondError(c, publicHTTPError{status: 429, code: "too_many_requests", message: "too many requests"})
		c.Abort()
	}
}

func (a *httpAdapter) createOrder(c *gin.Context) {
	var request createOrderJSON
	if err := a.decodeJSON(c, &request); err != nil {
		a.respondDecodeError(c, err)
		return
	}
	command, err := CreateCommand(request).validate()
	if err != nil {
		a.respondServiceError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.OperationTimeout)
	defer cancel()
	order, replayed, err := a.service.Create(ctx, command)
	if err != nil {
		a.respondServiceError(c, err)
		return
	}
	if err := ctx.Err(); err != nil {
		a.respondServiceError(c, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	a.respondData(c, status, gin.H{
		"order": order, "event_id": command.CommandID, "idempotency_key": command.CommandID,
		"delivery": "asynchronous", "replayed": replayed,
	})
}

func (a *httpAdapter) transitionOrder(c *gin.Context) {
	var request transitionOrderJSON
	if err := a.decodeJSON(c, &request); err != nil {
		a.respondDecodeError(c, err)
		return
	}
	command, err := TransitionCommand(request).validate()
	if err != nil {
		a.respondServiceError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.OperationTimeout)
	defer cancel()
	order, replayed, err := a.service.Transition(ctx, command)
	if err != nil {
		a.respondServiceError(c, err)
		return
	}
	if err := ctx.Err(); err != nil {
		a.respondServiceError(c, err)
		return
	}
	a.respondData(c, http.StatusOK, gin.H{
		"order": order, "event_id": command.CommandID, "idempotency_key": command.CommandID,
		"delivery": "asynchronous", "replayed": replayed,
	})
}

func (a *httpAdapter) searchHistory(c *gin.Context) {
	var request searchHistoryJSON
	if err := a.decodeJSON(c, &request); err != nil {
		a.respondDecodeError(c, err)
		return
	}
	aggregate, err := validateAggregate(request.Aggregate)
	if err != nil {
		a.respondError(c, invalidAuditQueryError())
		return
	}
	fromRevision, err := parseOptionalPositiveInt64(request.FromRevision)
	if err != nil {
		a.respondError(c, invalidAuditQueryError())
		return
	}
	toRevision, err := parseOptionalPositiveInt64(request.ToRevision)
	if err != nil || (fromRevision != 0 && toRevision != 0 && fromRevision > toRevision) {
		a.respondError(c, invalidAuditQueryError())
		return
	}
	limit, err := parseRequiredPositiveInt64(request.Limit)
	if err != nil || limit > 100 || (request.FromRecordedAt != nil && request.ToRecordedAt != nil && request.FromRecordedAt.After(*request.ToRecordedAt)) {
		a.respondError(c, invalidAuditQueryError())
		return
	}
	query := audit.Query{
		Aggregate: &aggregate, FromRevision: audit.Revision(fromRevision), ToRevision: audit.Revision(toRevision),
		Limit: int(limit) + 1,
	}
	if request.FromRecordedAt != nil {
		query.FromRecordedAt = *request.FromRecordedAt
	}
	if request.ToRecordedAt != nil {
		query.ToRecordedAt = *request.ToRecordedAt
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.OperationTimeout)
	defer cancel()
	entries, err := a.reader.Find(ctx, query)
	if err != nil {
		a.respondServiceError(c, err)
		return
	}
	if err := ctx.Err(); err != nil {
		a.respondServiceError(c, err)
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	var next *audit.Revision
	if len(entries) > int(limit) {
		value := entries[limit].Revision
		next = &value
		entries = entries[:limit]
	}
	a.respondData(c, http.StatusOK, gin.H{"entries": entries, "next_from_revision": next})
}

func (a *httpAdapter) historyDetail(c *gin.Context) {
	var request historyDetailJSON
	if err := a.decodeJSON(c, &request); err != nil {
		a.respondDecodeError(c, err)
		return
	}
	aggregate, err := validateAggregate(request.Aggregate)
	if err != nil {
		a.respondError(c, invalidAuditQueryError())
		return
	}
	revision, err := parseRequiredPositiveInt64(request.Revision)
	if err != nil {
		a.respondError(c, invalidAuditQueryError())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.OperationTimeout)
	defer cancel()
	entries, err := a.reader.Find(ctx, audit.Query{
		Aggregate: &aggregate, FromRevision: audit.Revision(revision), ToRevision: audit.Revision(revision), Limit: 1,
	})
	if err != nil {
		a.respondServiceError(c, err)
		return
	}
	if err := ctx.Err(); err != nil {
		a.respondServiceError(c, err)
		return
	}
	if len(entries) == 0 {
		a.respondError(c, publicHTTPError{status: 404, code: "audit_entry_not_found", message: "audit entry was not found"})
		return
	}
	a.respondData(c, http.StatusOK, gin.H{"entry": entries[0]})
}

func (a *httpAdapter) healthz(c *gin.Context) {
	a.respondData(c, http.StatusOK, gin.H{"status": "ok"})
}

func (a *httpAdapter) readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.OperationTimeout)
	defer cancel()
	readiness, err := a.health.Readiness(ctx)
	status := http.StatusOK
	state := "ready"
	if err != nil || !readiness.DatabaseReady || !readiness.RelayRunning {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	delivery := "available"
	if err != nil || !readiness.RedisReady {
		delivery = "degraded"
	}
	a.respondData(c, status, gin.H{"status": state, "delivery": delivery})
}

func (a *httpAdapter) statusz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 250*time.Millisecond)
	defer cancel()
	snapshot, err := a.health.Status(ctx)
	if err != nil {
		a.respondData(c, http.StatusOK, gin.H{
			"status": "degraded", "redis": "unknown", "relay": "unknown",
			"pending": int64(0), "retrying": int64(0), "claimed": int64(0),
			"published": int64(0), "dead_letter": int64(0), "oldest_pending_seconds": int64(0),
		})
		return
	}
	a.respondData(c, http.StatusOK, gin.H{
		"status": "ok", "redis": snapshot.RedisState, "relay": snapshot.RelayState,
		"pending": snapshot.Delivery.Pending, "retrying": snapshot.Delivery.Retrying,
		"claimed": snapshot.Delivery.Claimed, "published": snapshot.Delivery.Published,
		"dead_letter":            snapshot.Delivery.DeadLetter,
		"oldest_pending_seconds": snapshot.Delivery.OldestPendingSeconds,
	})
}

func (a *httpAdapter) notFound(c *gin.Context) {
	a.respondError(c, publicHTTPError{status: 404, code: "route_not_found", message: "route was not found"})
}

func (a *httpAdapter) methodNotAllowed(c *gin.Context) {
	a.respondError(c, publicHTTPError{status: 405, code: "method_not_allowed", message: "method is not allowed"})
}

func (a *httpAdapter) decodeJSON(c *gin.Context, target any) error {
	mediaType, parameters, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") || len(parameters) > 1 {
		return errUnsupportedMediaType
	}
	for key, value := range parameters {
		if !strings.EqualFold(key, "charset") || !strings.EqualFold(value, "utf-8") {
			return errUnsupportedMediaType
		}
	}
	encoding := strings.TrimSpace(c.GetHeader("Content-Encoding"))
	if encoding != "" && !strings.EqualFold(encoding, "identity") {
		return errUnsupportedContentEncoding
	}
	limited := http.MaxBytesReader(c.Writer, c.Request.Body, a.config.MaximumBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		var maximum *http.MaxBytesError
		if errors.As(err, &maximum) {
			return errRequestTooLarge
		}
		return fmt.Errorf("read body: %w", err)
	}
	if !utf8.Valid(body) || len(bytes.TrimSpace(body)) == 0 {
		return errInvalidHTTPRequest
	}
	if err := rejectDuplicateJSONObjectKeys(body); err != nil {
		return errInvalidHTTPRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errInvalidHTTPRequest
	}
	if err := requireDecoderEOF(decoder); err != nil {
		return errInvalidHTTPRequest
	}
	return nil
}

func rejectDuplicateJSONObjectKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var parseValue func(bool) error
	parseValue = func(requireObject bool) error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, isDelimiter := token.(json.Delim)
		if requireObject && (!isDelimiter || delimiter != '{') {
			return errors.New("top-level value is not an object")
		}
		if !isDelimiter {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				if keyErr != nil {
					return keyErr
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("object key is not a string")
				}
				if _, found := seen[key]; found {
					return errors.New("duplicate object key")
				}
				seen[key] = struct{}{}
				if err := parseValue(false); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := parseValue(false); err != nil {
					return err
				}
			}
		default:
			return errors.New("unexpected closing delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := parseValue(true); err != nil {
		return err
	}
	return requireDecoderEOF(decoder)
}

func requireDecoderEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func validateAggregate(value aggregateJSON) (audit.AggregateID, error) {
	typ, err := normalizeIdentifier(value.Type)
	if err != nil {
		return audit.AggregateID{}, err
	}
	id, err := normalizeIdentifier(value.ID)
	if err != nil {
		return audit.AggregateID{}, err
	}
	return audit.NewAggregateID(typ, id)
}

func parseOptionalPositiveInt64(number json.Number) (int64, error) {
	if number.String() == "" {
		return 0, nil
	}
	return parseRequiredPositiveInt64(number)
}

func parseRequiredPositiveInt64(number json.Number) (int64, error) {
	if number.String() == "" {
		return 0, errInvalidHTTPRequest
	}
	value, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || value <= 0 {
		return 0, errInvalidHTTPRequest
	}
	return value, nil
}

func (a *httpAdapter) respondDecodeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errRequestTooLarge):
		a.respondError(c, publicHTTPError{status: 413, code: "request_too_large", message: "request body is too large"})
	case errors.Is(err, errUnsupportedMediaType):
		a.respondError(c, publicHTTPError{status: 415, code: "unsupported_media_type", message: "application/json is required"})
	case errors.Is(err, errUnsupportedContentEncoding):
		a.respondError(c, publicHTTPError{status: 415, code: "unsupported_content_encoding", message: "content encoding is unsupported"})
	case errors.Is(err, errInvalidHTTPRequest):
		a.respondError(c, publicHTTPError{status: 400, code: "invalid_request", message: "request is invalid"})
	default:
		a.respondError(c, publicHTTPError{status: 500, code: "internal_error", message: "internal error"})
	}
}

func (a *httpAdapter) respondServiceError(c *gin.Context, err error) {
	if errors.Is(err, context.Canceled) && errors.Is(c.Request.Context().Err(), context.Canceled) {
		c.Abort()
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		a.respondError(c, publicHTTPError{status: 408, code: "request_timeout", message: "request timed out"})
		return
	}
	switch {
	case errors.Is(err, ErrInvalidCommand), errors.Is(err, audit.ErrInvalidQuery):
		a.respondError(c, publicHTTPError{status: 400, code: "invalid_request", message: "request is invalid"})
	case errors.Is(err, ErrNotFound):
		a.respondError(c, publicHTTPError{status: 404, code: "order_not_found", message: "order was not found"})
	case errors.Is(err, ErrConflict):
		a.respondError(c, publicHTTPError{status: 409, code: "invalid_transition", message: "order transition conflicts with current state"})
	default:
		a.respondError(c, publicHTTPError{status: 500, code: "internal_error", message: "internal error"})
	}
}

func invalidAuditQueryError() publicHTTPError {
	return publicHTTPError{status: 400, code: "invalid_audit_query", message: "audit query is invalid"}
}

func (a *httpAdapter) respondData(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"request_id": requestID(c), "data": data})
}

func (a *httpAdapter) respondError(c *gin.Context, public publicHTTPError) {
	if c.Writer.Written() {
		return
	}
	if public.status >= 500 {
		a.logger.Error("http request failed", "stage", "request", "class", public.code, "request_id", requestID(c))
	}
	c.JSON(public.status, gin.H{
		"request_id": requestID(c),
		"error":      gin.H{"code": public.code, "message": public.message},
	})
}

func requestID(c *gin.Context) string {
	value, found := c.Get("request_id")
	if !found {
		return "req-unknown"
	}
	id, ok := value.(string)
	if !ok || id == "" {
		return "req-unknown"
	}
	return id
}

func isNilInterface(value any) bool {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
