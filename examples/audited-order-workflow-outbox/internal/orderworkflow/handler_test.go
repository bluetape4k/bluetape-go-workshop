package orderworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestHTTPDefaultConfig(t *testing.T) {
	config := DefaultHTTPConfig()
	if config.MaximumBodyBytes != 32<<10 || config.OperationTimeout != 2*time.Second || config.MaximumConcurrent != 32 {
		t.Fatalf("DefaultHTTPConfig() = %+v", config)
	}
}

func TestHTTPNewEngineRejectsInvalidDependencies(t *testing.T) {
	service := &stubCommandService{}
	reader := audit.NewMemoryRepository()
	health := readyHealth{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name    string
		service CommandService
		reader  audit.HistoryReader
		health  HealthReader
		config  HTTPConfig
		logger  *slog.Logger
	}{
		{name: "nil service", reader: reader, health: health, config: DefaultHTTPConfig(), logger: logger},
		{name: "nil reader", service: service, health: health, config: DefaultHTTPConfig(), logger: logger},
		{name: "nil health", service: service, reader: reader, config: DefaultHTTPConfig(), logger: logger},
		{name: "zero body limit", service: service, reader: reader, health: health, config: HTTPConfig{OperationTimeout: time.Second, MaximumConcurrent: 1}, logger: logger},
		{name: "zero timeout", service: service, reader: reader, health: health, config: HTTPConfig{MaximumBodyBytes: 1, MaximumConcurrent: 1}, logger: logger},
		{name: "zero concurrency", service: service, reader: reader, health: health, config: HTTPConfig{MaximumBodyBytes: 1, OperationTimeout: time.Second}, logger: logger},
		{name: "nil logger", service: service, reader: reader, health: health, config: DefaultHTTPConfig()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(tt.service, tt.reader, tt.health, tt.config, tt.logger)
			if engine != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewEngine() = (%v, %v)", engine, err)
			}
		})
	}
	var typedNil *stubCommandService
	if engine, err := NewEngine(typedNil, reader, health, DefaultHTTPConfig(), logger); engine != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewEngine(typed nil) = (%v, %v)", engine, err)
	}
}

func TestHTTPStrictJSON(t *testing.T) {
	config := DefaultHTTPConfig()
	config.MaximumBodyBytes = 128
	engine := newHandlerTestEngine(t, &stubCommandService{}, audit.NewMemoryRepository(), readyHealth{}, config, io.Discard)
	tests := []struct {
		name        string
		body        []byte
		contentType string
		encoding    string
		wantStatus  int
		wantCode    string
	}{
		{name: "valid", body: []byte(`{"order_id":"order-1","command_id":"cmd-1"}`), contentType: "application/json", wantStatus: 201},
		{name: "utf8 charset", body: []byte(`{"order_id":"order-1","command_id":"cmd-1"}`), contentType: "application/json; charset=utf-8", wantStatus: 201},
		{name: "identity", body: []byte(`{"order_id":"order-1","command_id":"cmd-1"}`), contentType: "application/json", encoding: "identity", wantStatus: 201},
		{name: "missing content type", body: []byte(`{}`), wantStatus: 415, wantCode: "unsupported_media_type"},
		{name: "wrong content type", body: []byte(`{}`), contentType: "text/plain", wantStatus: 415, wantCode: "unsupported_media_type"},
		{name: "wrong charset", body: []byte(`{}`), contentType: "application/json; charset=ascii", wantStatus: 415, wantCode: "unsupported_media_type"},
		{name: "gzip", body: []byte(`{}`), contentType: "application/json", encoding: "gzip", wantStatus: 415, wantCode: "unsupported_content_encoding"},
		{name: "multiple encodings", body: []byte(`{}`), contentType: "application/json", encoding: "identity, gzip", wantStatus: 415, wantCode: "unsupported_content_encoding"},
		{name: "invalid utf8", body: []byte{0xff}, contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "unknown field", body: []byte(`{"order_id":"order-1","command_id":"cmd-1","secret":"x"}`), contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "duplicate nested key", body: []byte(`{"order_id":"order-1","command_id":"cmd-1","metadata":{"a":"1","a":"2"}}`), contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "trailing value", body: []byte(`{"order_id":"order-1","command_id":"cmd-1"} {}`), contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "array", body: []byte(`[]`), contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "scalar", body: []byte(`1`), contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "empty", contentType: "application/json", wantStatus: 400, wantCode: "invalid_request"},
		{name: "oversized", body: bytes.Repeat([]byte(" "), 129), contentType: "application/json", wantStatus: 413, wantCode: "request_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &handlerTrackingBody{Reader: bytes.NewReader(tt.body)}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/orders", body)
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}
			if tt.encoding != "" {
				request.Header.Set("Content-Encoding", tt.encoding)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if tt.wantCode != "" {
				assertHTTPError(t, response, tt.wantStatus, tt.wantCode)
			}
			if !body.closed.Load() {
				t.Fatal("request body was not closed")
			}
		})
	}
}

func TestHTTPNumericJSONBoundaries(t *testing.T) {
	engine := newHandlerTestEngine(t, &stubCommandService{}, audit.NewMemoryRepository(), readyHealth{}, DefaultHTTPConfig(), io.Discard)
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "maximum int64", body: `{"aggregate":{"type":"order","id":"order-1"},"from_revision":9223372036854775807,"limit":1}`, wantStatus: 200},
		{name: "overflow", body: `{"aggregate":{"type":"order","id":"order-1"},"from_revision":9223372036854775808,"limit":1}`, wantStatus: 400},
		{name: "fraction", body: `{"aggregate":{"type":"order","id":"order-1"},"from_revision":1.5,"limit":1}`, wantStatus: 400},
		{name: "exponent", body: `{"aggregate":{"type":"order","id":"order-1"},"from_revision":1e1,"limit":1}`, wantStatus: 400},
		{name: "negative", body: `{"aggregate":{"type":"order","id":"order-1"},"from_revision":-1,"limit":1}`, wantStatus: 400},
		{name: "zero", body: `{"aggregate":{"type":"order","id":"order-1"},"from_revision":0,"limit":1}`, wantStatus: 400},
		{name: "limit maximum", body: `{"aggregate":{"type":"order","id":"order-1"},"limit":100}`, wantStatus: 200},
		{name: "limit overflow", body: `{"aggregate":{"type":"order","id":"order-1"},"limit":101}`, wantStatus: 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performHandlerJSON(t, engine, http.MethodPost, "/audit/history/search", tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHTTPRoutesCommandsAndErrors(t *testing.T) {
	now := normalizeTimestamp(testWorkflowNow)
	service := &stubCommandService{
		createOrder:     Order{OrderID: "order-1", Status: StatusPending, Revision: 1, UpdatedAt: now},
		transitionOrder: Order{OrderID: "order-1", Status: StatusConfirmed, Revision: 2, UpdatedAt: now},
	}
	engine := newHandlerTestEngine(t, service, audit.NewMemoryRepository(), readyHealth{}, DefaultHTTPConfig(), io.Discard)

	created := performHandlerJSON(t, engine, http.MethodPost, "/orders", `{"order_id":"order-1","command_id":"cmd-create","metadata":{"channel":"workshop"}}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	assertHTTPSuccessFields(t, created, "cmd-create", false)

	service.createReplayed = true
	replayed := performHandlerJSON(t, engine, http.MethodPost, "/orders", `{"order_id":"order-1","command_id":"cmd-create","metadata":{"channel":"workshop"}}`)
	if replayed.Code != http.StatusOK {
		t.Fatalf("replay status = %d, body = %s", replayed.Code, replayed.Body.String())
	}
	assertHTTPSuccessFields(t, replayed, "cmd-create", true)

	transition := performHandlerJSON(t, engine, http.MethodPost, "/orders/transitions", `{"order_id":"order-1","command_id":"cmd-confirm","action":"confirm"}`)
	if transition.Code != http.StatusOK {
		t.Fatalf("transition status = %d, body = %s", transition.Code, transition.Body.String())
	}
	assertHTTPSuccessFields(t, transition, "cmd-confirm", false)

	errorCases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid", err: ErrInvalidCommand, status: 400, code: "invalid_request"},
		{name: "not found", err: ErrNotFound, status: 404, code: "order_not_found"},
		{name: "conflict", err: ErrConflict, status: 409, code: "invalid_transition"},
		{name: "timeout", err: context.DeadlineExceeded, status: 408, code: "request_timeout"},
		{name: "storage", err: errors.New("provider-secret-marker"), status: 500, code: "internal_error"},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			service.createErr = tt.err
			response := performHandlerJSON(t, engine, http.MethodPost, "/orders", `{"order_id":"order-1","command_id":"cmd-error"}`)
			assertHTTPError(t, response, tt.status, tt.code)
			if strings.Contains(response.Body.String(), "provider-secret-marker") {
				t.Fatalf("provider error leaked: %s", response.Body.String())
			}
			service.createErr = nil
		})
	}
}

func TestHTTPHistoryPaginationAndDetail(t *testing.T) {
	repository := audit.NewMemoryRepository()
	create := CreateCommand{OrderID: "order-1", CommandID: "cmd-create"}
	firstOrder := Order{OrderID: "order-1", Status: StatusPending, Revision: 1, UpdatedAt: normalizeTimestamp(testWorkflowNow)}
	first, err := buildCreateEntry("workshop", firstOrder.UpdatedAt, create, firstOrder)
	if err != nil {
		t.Fatal(err)
	}
	confirm := TransitionCommand{OrderID: "order-1", CommandID: "cmd-confirm", Action: ActionConfirm}
	secondOrder := Order{OrderID: "order-1", Status: StatusConfirmed, Revision: 2, UpdatedAt: firstOrder.UpdatedAt.Add(time.Second)}
	second, err := buildTransitionEntry("workshop", secondOrder.UpdatedAt, confirm, secondOrder)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Append(context.Background(), first, second); err != nil {
		t.Fatal(err)
	}
	engine := newHandlerTestEngine(t, &stubCommandService{}, repository, readyHealth{}, DefaultHTTPConfig(), io.Discard)

	pageOne := performHandlerJSON(t, engine, http.MethodPost, "/audit/history/search", `{"aggregate":{"type":"order","id":"order-1"},"from_revision":1,"limit":1}`)
	if pageOne.Code != 200 || !strings.Contains(pageOne.Body.String(), `"next_from_revision":2`) || !strings.Contains(pageOne.Body.String(), `"revision":1`) {
		t.Fatalf("page one = %d %s", pageOne.Code, pageOne.Body.String())
	}
	pageTwo := performHandlerJSON(t, engine, http.MethodPost, "/audit/history/search", `{"aggregate":{"type":"order","id":"order-1"},"from_revision":2,"limit":1}`)
	if pageTwo.Code != 200 || !strings.Contains(pageTwo.Body.String(), `"next_from_revision":null`) || !strings.Contains(pageTwo.Body.String(), `"revision":2`) {
		t.Fatalf("page two = %d %s", pageTwo.Code, pageTwo.Body.String())
	}
	detail := performHandlerJSON(t, engine, http.MethodPost, "/audit/history/detail", `{"aggregate":{"type":"order","id":"order-1"},"revision":2}`)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), `"event_type":"order.confirmed"`) {
		t.Fatalf("detail = %d %s", detail.Code, detail.Body.String())
	}
	missing := performHandlerJSON(t, engine, http.MethodPost, "/audit/history/detail", `{"aggregate":{"type":"order","id":"order-1"},"revision":3}`)
	assertHTTPError(t, missing, 404, "audit_entry_not_found")
}

func TestHTTPRouteMatrixAndHealth(t *testing.T) {
	engine := newHandlerTestEngine(t, &stubCommandService{}, audit.NewMemoryRepository(), readyHealth{}, DefaultHTTPConfig(), io.Discard)
	tests := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{method: http.MethodPost, path: "/orders", body: `{"order_id":"order-1","command_id":"cmd-1"}`, status: 201},
		{method: http.MethodPost, path: "/orders/transitions", body: `{"order_id":"order-1","command_id":"cmd-2","action":"confirm"}`, status: 200},
		{method: http.MethodPost, path: "/audit/history/search", body: `{"aggregate":{"type":"order","id":"order-1"},"limit":1}`, status: 200},
		{method: http.MethodPost, path: "/audit/history/detail", body: `{"aggregate":{"type":"order","id":"order-1"},"revision":1}`, status: 404},
		{method: http.MethodGet, path: "/healthz", status: 200},
		{method: http.MethodGet, path: "/readyz", status: 200},
		{method: http.MethodGet, path: "/statusz", status: 200},
	}
	for _, tt := range tests {
		response := performHandlerJSON(t, engine, tt.method, tt.path, tt.body)
		if response.Code != tt.status {
			t.Fatalf("%s %s = %d, body = %s", tt.method, tt.path, response.Code, response.Body.String())
		}
		wrongMethod := http.MethodGet
		if tt.method == http.MethodGet {
			wrongMethod = http.MethodPost
		}
		wrong := performHandlerJSON(t, engine, wrongMethod, tt.path, `{}`)
		assertHTTPError(t, wrong, 405, "method_not_allowed")
	}
	unknown := performHandlerJSON(t, engine, http.MethodGet, "/missing", "")
	assertHTTPError(t, unknown, 404, "route_not_found")

	degraded := newHandlerTestEngine(t, &stubCommandService{}, audit.NewMemoryRepository(), readyHealth{databaseReady: true, relayRunning: true, redisReady: false}, DefaultHTTPConfig(), io.Discard)
	ready := performHandlerJSON(t, degraded, http.MethodGet, "/readyz", "")
	if ready.Code != 200 || !strings.Contains(ready.Body.String(), `"delivery":"degraded"`) {
		t.Fatalf("degraded readiness = %d %s", ready.Code, ready.Body.String())
	}
	unavailable := newHandlerTestEngine(t, &stubCommandService{}, audit.NewMemoryRepository(), readyHealth{databaseReady: false, relayRunning: true, redisReady: true}, DefaultHTTPConfig(), io.Discard)
	notReady := performHandlerJSON(t, unavailable, http.MethodGet, "/readyz", "")
	if notReady.Code != 503 {
		t.Fatalf("unavailable readiness = %d %s", notReady.Code, notReady.Body.String())
	}
}

func TestHTTPOverloadClosesBodyWithoutCallingService(t *testing.T) {
	started := make(chan struct{}, 32)
	release := make(chan struct{})
	service := &stubCommandService{createFunc: func(ctx context.Context, command CreateCommand) (Order, bool, error) {
		started <- struct{}{}
		select {
		case <-release:
			return Order{OrderID: command.OrderID, Status: StatusPending, Revision: 1, UpdatedAt: normalizeTimestamp(testWorkflowNow)}, false, nil
		case <-ctx.Done():
			return Order{}, false, ctx.Err()
		}
	}}
	engine := newHandlerTestEngine(t, service, audit.NewMemoryRepository(), readyHealth{}, DefaultHTTPConfig(), io.Discard)
	var wait sync.WaitGroup
	wait.Add(32)
	for index := range 32 {
		go func(index int) {
			defer wait.Done()
			response := performHandlerJSON(t, engine, http.MethodPost, "/orders", `{"order_id":"order-`+strconv.Itoa(index)+`","command_id":"cmd-`+strconv.Itoa(index)+`"}`)
			if response.Code != 201 {
				t.Errorf("held response = %d %s", response.Code, response.Body.String())
			}
		}(index)
	}
	for range 32 {
		<-started
	}
	body := &handlerTrackingBody{Reader: bytes.NewReader([]byte(`{"order_id":"order-overload","command_id":"cmd-overload"}`))}
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/orders", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	assertHTTPError(t, response, 429, "too_many_requests")
	if response.Header().Get("Retry-After") != "1" || response.Header().Get("Connection") != "close" || !body.closed.Load() {
		t.Fatalf("overload headers/body = %#v closed=%v", response.Header(), body.closed.Load())
	}
	if service.createCalls.Load() != 32 {
		t.Fatalf("service calls = %d, want 32", service.createCalls.Load())
	}
	close(release)
	wait.Wait()
}

func TestHTTPTimeoutAndRedaction(t *testing.T) {
	secret := "identity-payload-metadata-endpoint-provider"
	service := &stubCommandService{createFunc: func(ctx context.Context, _ CreateCommand) (Order, bool, error) {
		<-ctx.Done()
		return Order{}, false, errors.Join(ctx.Err(), errors.New(secret))
	}}
	config := DefaultHTTPConfig()
	config.OperationTimeout = 10 * time.Millisecond
	var logs bytes.Buffer
	engine := newHandlerTestEngine(t, service, &errorHistoryReader{err: errors.New(secret)}, statusErrorHealth{err: errors.New(secret)}, config, &logs)
	response := performHandlerJSON(t, engine, http.MethodPost, "/orders", `{"order_id":"order-secret","command_id":"cmd-secret"}`)
	assertHTTPError(t, response, 408, "request_timeout")
	status := performHandlerJSON(t, engine, http.MethodGet, "/statusz", "")
	if status.Code != 200 || !strings.Contains(status.Body.String(), `"status":"degraded"`) {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}
	if strings.Contains(response.Body.String()+status.Body.String()+logs.String(), secret) {
		t.Fatalf("sensitive marker leaked: response=%s status=%s logs=%s", response.Body.String(), status.Body.String(), logs.String())
	}
}

type stubCommandService struct {
	createOrder     Order
	createReplayed  bool
	createErr       error
	transitionOrder Order
	transitionErr   error
	createFunc      func(context.Context, CreateCommand) (Order, bool, error)
	createCalls     atomic.Int64
}

func (s *stubCommandService) Create(ctx context.Context, command CreateCommand) (Order, bool, error) {
	s.createCalls.Add(1)
	if s.createFunc != nil {
		return s.createFunc(ctx, command)
	}
	order := s.createOrder
	if order.OrderID == "" {
		order = Order{OrderID: command.OrderID, Status: StatusPending, Revision: 1, UpdatedAt: normalizeTimestamp(testWorkflowNow)}
	}
	return order, s.createReplayed, s.createErr
}

func (s *stubCommandService) Transition(_ context.Context, command TransitionCommand) (Order, bool, error) {
	order := s.transitionOrder
	if order.OrderID == "" {
		order = Order{OrderID: command.OrderID, Status: StatusConfirmed, Revision: 2, UpdatedAt: normalizeTimestamp(testWorkflowNow)}
	}
	return order, false, s.transitionErr
}

type readyHealth struct {
	databaseReady bool
	relayRunning  bool
	redisReady    bool
}

func (h readyHealth) Readiness(context.Context) (Readiness, error) {
	value := Readiness{DatabaseReady: true, RelayRunning: true, RedisReady: true}
	if h.databaseReady || h.relayRunning || h.redisReady {
		value = Readiness{DatabaseReady: h.databaseReady, RelayRunning: h.relayRunning, RedisReady: h.redisReady}
	}
	return value, nil
}

func (readyHealth) Status(context.Context) (DeliverySnapshot, error) {
	return DeliverySnapshot{
		RedisState: "available", RelayState: "running",
		Delivery: DeliveryStatus{Pending: 1, Retrying: 2, Claimed: 3, Published: 4, DeadLetter: 5, OldestPendingSeconds: 6},
	}, nil
}

type statusErrorHealth struct{ err error }

func (h statusErrorHealth) Readiness(context.Context) (Readiness, error) {
	return Readiness{}, h.err
}

func (h statusErrorHealth) Status(context.Context) (DeliverySnapshot, error) {
	return DeliverySnapshot{}, h.err
}

type errorHistoryReader struct{ err error }

func (r *errorHistoryReader) Find(context.Context, audit.Query) ([]audit.Entry, error) {
	return nil, r.err
}
func (r *errorHistoryReader) LoadHistory(context.Context, audit.AggregateID) (audit.History, bool, error) {
	return audit.History{}, false, r.err
}
func (r *errorHistoryReader) Latest(context.Context, audit.AggregateID) (audit.Entry, bool, error) {
	return audit.Entry{}, false, r.err
}
func (r *errorHistoryReader) LatestSnapshot(context.Context, audit.AggregateID) (audit.Entry, bool, error) {
	return audit.Entry{}, false, r.err
}
func (r *errorHistoryReader) PreviousSnapshot(context.Context, audit.AggregateID, audit.Revision) (audit.Entry, bool, error) {
	return audit.Entry{}, false, r.err
}

type handlerTrackingBody struct {
	*bytes.Reader
	closed atomic.Bool
}

func (b *handlerTrackingBody) Close() error {
	b.closed.Store(true)
	return nil
}

func newHandlerTestEngine(t *testing.T, service CommandService, reader audit.HistoryReader, health HealthReader, config HTTPConfig, output io.Writer) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine, err := NewEngine(service, reader, health, config, logger)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func performHandlerJSON(t *testing.T, handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertHTTPSuccessFields(t *testing.T, response *httptest.ResponseRecorder, commandID string, replayed bool) {
	t.Helper()
	var envelope struct {
		RequestID string `json:"request_id"`
		Data      struct {
			EventID        string `json:"event_id"`
			IdempotencyKey string `json:"idempotency_key"`
			Delivery       string `json:"delivery"`
			Replayed       bool   `json:"replayed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.RequestID == "" || envelope.Data.EventID != commandID || envelope.Data.IdempotencyKey != commandID ||
		envelope.Data.Delivery != "asynchronous" || envelope.Data.Replayed != replayed {
		t.Fatalf("success envelope = %+v", envelope)
	}
}

func assertHTTPError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var envelope struct {
		RequestID string          `json:"request_id"`
		Data      json.RawMessage `json:"data"`
		Error     struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v; body=%s", err, response.Body.String())
	}
	if response.Code != status || envelope.RequestID == "" || envelope.Error.Code != code || envelope.Error.Message == "" || envelope.Data != nil {
		t.Fatalf("error envelope = status %d %+v, want %d %s", response.Code, envelope, status, code)
	}
}
