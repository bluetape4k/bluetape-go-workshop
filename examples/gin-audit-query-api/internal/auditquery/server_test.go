package auditquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestDefaultHTTPConfig(t *testing.T) {
	config := DefaultHTTPConfig()
	if config.MaximumBodyBytes != 32<<10 || config.RequestTimeout != 2*time.Second {
		t.Fatalf("DefaultHTTPConfig() = %+v", config)
	}
}

func TestNewEngineRejectsInvalidDependencies(t *testing.T) {
	service := &stubQueryService{}
	logger := log.New(io.Discard, "", 0)
	tests := []struct {
		name    string
		service QueryService
		config  HTTPConfig
		logger  *log.Logger
	}{
		{name: "nil service", config: DefaultHTTPConfig(), logger: logger},
		{name: "zero body limit", service: service, config: HTTPConfig{RequestTimeout: time.Second}, logger: logger},
		{name: "zero timeout", service: service, config: HTTPConfig{MaximumBodyBytes: 1024}, logger: logger},
		{name: "nil logger", service: service, config: DefaultHTTPConfig()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine, err := NewEngine(test.service, test.config, test.logger)
			if engine != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewEngine() engine = %v, err = %v", engine, err)
			}
		})
	}
	var typedNil *stubQueryService
	if engine, err := NewEngine(typedNil, DefaultHTTPConfig(), logger); engine != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewEngine(typed nil) engine = %v, err = %v", engine, err)
	}
}

func TestHTTPSearchAndDetail(t *testing.T) {
	engine, _ := newHTTPTestEngine(t, DefaultHTTPConfig())
	search := performJSONRequest(t, engine, http.MethodPost, "/audit/history/search", `{"aggregate":{"type":"order","id":"order-1001"},"newest_first":true,"limit":2}`)
	if search.Code != http.StatusOK || search.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("search status = %d, content-type = %q, body = %s", search.Code, search.Header().Get("Content-Type"), search.Body.String())
	}
	var response SearchResponse
	if err := json.Unmarshal(search.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	assertRevisions(t, response.Entries, 4, 3)
	if response.Page.Next == nil || response.Page.Next.ToRevision != 2 {
		t.Fatalf("search page = %+v", response.Page)
	}

	detail := httptest.NewRecorder()
	engine.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/audit/aggregates/order/order-1001/revisions/3", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"event_type":"order.packed"`) {
		t.Fatalf("detail status = %d, body = %s", detail.Code, detail.Body.String())
	}

	missing := httptest.NewRecorder()
	engine.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/audit/aggregates/order/order-1001/revisions/99", nil))
	assertPublicError(t, missing, http.StatusNotFound, "audit_entry_not_found")
}

func TestHTTPStrictJSONAndRouteFailures(t *testing.T) {
	config := DefaultHTTPConfig()
	config.MaximumBodyBytes = 128
	engine, _ := newHTTPTestEngine(t, config)
	tests := []struct {
		name        string
		method      string
		path        string
		body        []byte
		contentType string
		encoding    string
		status      int
		code        string
	}{
		{name: "missing content type", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{}`), status: 415, code: "unsupported_media_type"},
		{name: "wrong content type", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{}`), contentType: "text/plain", status: 415, code: "unsupported_media_type"},
		{name: "wrong charset", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{}`), contentType: "application/json; charset=ascii", status: 415, code: "unsupported_media_type"},
		{name: "unsupported encoding", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{}`), contentType: "application/json", encoding: "gzip", status: 415, code: "unsupported_content_encoding"},
		{name: "invalid utf8", method: http.MethodPost, path: "/audit/history/search", body: []byte{0xff}, contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "unknown field", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{"aggregate":{"type":"order","id":"order-1001"},"metadata":{"tenant":"secret"}}`), contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "duplicate nested key", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{"aggregate":{"type":"order","type":"other","id":"order-1001"}}`), contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "trailing json", method: http.MethodPost, path: "/audit/history/search", body: []byte(`{"aggregate":{"type":"order","id":"order-1001"}} {}`), contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "oversized", method: http.MethodPost, path: "/audit/history/search", body: bytes.Repeat([]byte(" "), 129), contentType: "application/json", status: 413, code: "request_too_large"},
		{name: "wrong method", method: http.MethodGet, path: "/audit/history/search", status: 405, code: "method_not_allowed"},
		{name: "trailing slash", method: http.MethodPost, path: "/audit/history/search/", body: []byte(`{}`), contentType: "application/json", status: 404, code: "route_not_found"},
		{name: "encoded path", method: http.MethodGet, path: "/audit/aggregates/order/order%2F1001/revisions/1", status: 400, code: "invalid_request"},
		{name: "invalid revision", method: http.MethodGet, path: "/audit/aggregates/order/order-1001/revisions/nope", status: 400, code: "invalid_audit_query"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &trackingBody{Reader: bytes.NewReader(test.body)}
			request := httptest.NewRequest(test.method, test.path, body)
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			if test.encoding != "" {
				request.Header.Set("Content-Encoding", test.encoding)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assertPublicError(t, recorder, test.status, test.code)
			if !body.closed.Load() {
				t.Fatal("request body was not closed")
			}
		})
	}
}

func TestHTTPMapsAuditErrorsBeforeOwnSentinel(t *testing.T) {
	validation := audit.ValidationError{Kind: audit.ErrInvalidQuery, Field: "aggregate", Value: "secret-aggregate"}
	service := &stubQueryService{searchErr: errors.Join(ErrInvalidRequest, validation)}
	var logs bytes.Buffer
	engine, err := NewEngine(service, DefaultHTTPConfig(), log.New(&logs, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	recorder := performJSONRequest(t, engine, http.MethodPost, "/audit/history/search", `{"aggregate":{"type":"order","id":"secret-order"}}`)
	assertPublicError(t, recorder, http.StatusBadRequest, "invalid_audit_query")
	if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(logs.String(), "secret") {
		t.Fatalf("sensitive value leaked: body=%s logs=%s", recorder.Body.String(), logs.String())
	}
}

func TestHTTPTimeoutAndClientCancellation(t *testing.T) {
	service := &blockingQueryService{called: make(chan error, 2)}
	config := DefaultHTTPConfig()
	config.RequestTimeout = 10 * time.Millisecond
	engine, err := NewEngine(service, config, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	recorder := performJSONRequest(t, engine, http.MethodPost, "/audit/history/search", `{"aggregate":{"type":"order","id":"order-1"}}`)
	assertPublicError(t, recorder, http.StatusRequestTimeout, "request_timeout")
	if cause := <-service.called; !errors.Is(cause, context.DeadlineExceeded) {
		t.Fatalf("service timeout cause = %v", cause)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/audit/history/search", strings.NewReader(`{"aggregate":{"type":"order","id":"order-1"}}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	canceled := httptest.NewRecorder()
	engine.ServeHTTP(canceled, request)
	if canceled.Body.Len() != 0 {
		t.Fatalf("client cancellation body = %s", canceled.Body.String())
	}
	if cause := <-service.called; !errors.Is(cause, context.Canceled) {
		t.Fatalf("service cancellation cause = %v", cause)
	}
}

type stubQueryService struct {
	search    SearchResponse
	searchErr error
	entry     audit.Entry
	getErr    error
}

func (s *stubQueryService) Search(context.Context, SearchRequest) (SearchResponse, error) {
	return s.search, s.searchErr
}

func (s *stubQueryService) Get(context.Context, AggregateRequest, audit.Revision) (audit.Entry, error) {
	return s.entry, s.getErr
}

type blockingQueryService struct {
	called chan error
}

func (s *blockingQueryService) Search(ctx context.Context, _ SearchRequest) (SearchResponse, error) {
	<-ctx.Done()
	s.called <- ctx.Err()
	return SearchResponse{}, ctx.Err()
}

func (s *blockingQueryService) Get(ctx context.Context, _ AggregateRequest, _ audit.Revision) (audit.Entry, error) {
	<-ctx.Done()
	s.called <- ctx.Err()
	return audit.Entry{}, ctx.Err()
}

type trackingBody struct {
	*bytes.Reader
	closed atomic.Bool
}

func (b *trackingBody) Close() error {
	b.closed.Store(true)
	return nil
}

func newHTTPTestEngine(t *testing.T, config HTTPConfig) (http.Handler, *bytes.Buffer) {
	t.Helper()
	repository := audit.NewMemoryRepository()
	if err := SeedRepository(context.Background(), repository); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repository, DefaultServiceConfig())
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	engine, err := NewEngine(service, config, log.New(&logs, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	return engine, &logs
}

func performJSONRequest(t *testing.T, handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func assertPublicError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status || !strings.Contains(recorder.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("status = %d, body = %s; want status %d code %s", recorder.Code, recorder.Body.String(), status, code)
	}
}
