package moderationapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type stubWorkflow struct {
	create func(context.Context, CreateRequest) (Record, error)
	get    func(context.Context, string) (Record, error)
	search func(context.Context, SearchRequest) (SearchResponse, error)
}

func (s stubWorkflow) Create(ctx context.Context, request CreateRequest) (Record, error) {
	if s.create != nil {
		return s.create(ctx, request)
	}
	return Record{}, nil
}
func (s stubWorkflow) Get(ctx context.Context, id string) (Record, error) {
	if s.get != nil {
		return s.get(ctx, id)
	}
	return Record{}, nil
}
func (s stubWorkflow) Search(ctx context.Context, request SearchRequest) (SearchResponse, error) {
	if s.search != nil {
		return s.search(ctx, request)
	}
	return SearchResponse{Hits: []SearchHit{}}, nil
}

func TestHTTPSuccessRoutes(t *testing.T) {
	workflow := stubWorkflow{
		create: func(_ context.Context, request CreateRequest) (Record, error) {
			return Record{ContentID: request.ContentID, Content: request.Content, Metadata: request.Metadata, Outcome: OutcomeAllowed}, nil
		},
		get: func(_ context.Context, id string) (Record, error) {
			return Record{ContentID: id, Content: "original-secret", Outcome: OutcomeAllowed}, nil
		},
		search: func(_ context.Context, request SearchRequest) (SearchResponse, error) {
			return SearchResponse{Hits: []SearchHit{{ContentID: "article-100", Outcome: OutcomeAllowed, DisplayText: "safe", Metadata: request.Metadata}}, Truncated: true, NextAfterContentID: "article-100"}, nil
		},
	}
	engine := newHTTPTestEngine(t, workflow, DefaultConfig().HTTP, io.Discard)

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		status      int
		contains    []string
		excludes    []string
		closesBody  bool
	}{
		{name: "create", method: http.MethodPost, path: "/moderation/records", body: `{"content_id":"article-100","content":"original-secret","metadata":{"tenant":"demo"}}`, contentType: "application/json", status: http.StatusCreated, contains: []string{`"content_id":"article-100"`, `"content":"original-secret"`}, closesBody: true},
		{name: "get", method: http.MethodGet, path: "/moderation/records/article-100", status: http.StatusOK, contains: []string{`"content":"original-secret"`}, closesBody: true},
		{name: "search", method: http.MethodPost, path: "/moderation/records/search", body: `{"query":"delivery","metadata":{"tenant":"demo"}}`, contentType: "application/json; charset=UTF-8", status: http.StatusOK, contains: []string{`"truncated":true`, `"next_after_content_id":"article-100"`}, excludes: []string{`"content":`, `"terms":`, `"findings":`}, closesBody: true},
		{name: "health", method: http.MethodGet, path: "/healthz", status: http.StatusOK, contains: []string{`"status":"ok"`}, closesBody: true},
		{name: "method miss", method: http.MethodPut, path: "/healthz", status: http.StatusMethodNotAllowed},
		{name: "path miss", method: http.MethodGet, path: "/missing", status: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &trackingBody{Reader: strings.NewReader(test.body)}
			request := httptest.NewRequestWithContext(context.Background(), test.method, test.path, body)
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			for _, value := range test.contains {
				if !strings.Contains(response.Body.String(), value) {
					t.Fatalf("body = %s, missing %s", response.Body.String(), value)
				}
			}
			for _, value := range test.excludes {
				if strings.Contains(response.Body.String(), value) {
					t.Fatalf("body = %s, contains %s", response.Body.String(), value)
				}
			}
			if test.closesBody && body.closed != 1 {
				t.Fatalf("body close count = %d", body.closed)
			}
		})
	}
}

func TestHTTPRejectsStrictJSONAndUnsupportedRepresentations(t *testing.T) {
	config := DefaultConfig().HTTP
	config.MaximumBodyBytes = 96
	engine := newHTTPTestEngine(t, stubWorkflow{}, config, io.Discard)
	tests := []struct {
		name, body, contentType, contentEncoding, code string
		status                                         int
	}{
		{name: "missing media", body: `{}`, status: 415, code: "unsupported_media_type"},
		{name: "wrong media", body: `{}`, contentType: "text/plain", status: 415, code: "unsupported_media_type"},
		{name: "invalid charset", body: `{}`, contentType: "application/json; charset=latin1", status: 415, code: "unsupported_media_type"},
		{name: "encoding", body: `{}`, contentType: "application/json", contentEncoding: "gzip", status: 415, code: "unsupported_content_encoding"},
		{name: "malformed", body: `{`, contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "trailing", body: `{"query":"delivery"}{}`, contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "unknown", body: `{"query":"delivery","secret":"value"}`, contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "duplicate root", body: `{"query":"delivery","query":"other"}`, contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "duplicate metadata", body: `{"query":"delivery","metadata":{"tenant":"a","tenant":"b"}}`, contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "invalid utf8", body: string([]byte{'{', '"', 'q', 'u', 'e', 'r', 'y', '"', ':', '"', 0xff, '"', '}'}), contentType: "application/json", status: 400, code: "invalid_request"},
		{name: "too large", body: `{"query":"` + strings.Repeat("x", 100) + `"}`, contentType: "application/json", status: 413, code: "request_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &trackingBody{Reader: strings.NewReader(test.body)}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/moderation/records/search", body)
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set("Content-Encoding", test.contentEncoding)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if body.closed != 1 {
				t.Fatalf("body close count = %d", body.closed)
			}
		})
	}
}

func TestHTTPAcceptsBodyExactlyAtLimit(t *testing.T) {
	body := `{"query":"` + strings.Repeat("x", 84) + `"}`
	config := DefaultConfig().HTTP
	config.MaximumBodyBytes = int64(len(body))
	engine := newHTTPTestEngine(t, stubWorkflow{}, config, io.Discard)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/moderation/records/search", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPRejectsMetadataKeysCollidingAfterTrim(t *testing.T) {
	service := newCreateTestService(t)
	engine := newHTTPTestEngine(t, service, DefaultConfig().HTTP, io.Discard)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/moderation/records", strings.NewReader(`{"content_id":"article-100","content":"Please review the delivery status.","metadata":{"tenant":"a"," tenant ":"b"}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPRejectsEscapedPathSeparator(t *testing.T) {
	engine := newHTTPTestEngine(t, stubWorkflow{}, DefaultConfig().HTTP, io.Discard)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/moderation/records/article%2Fsecret", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPNewEngineRejectsInvalidDependencies(t *testing.T) {
	valid := DefaultConfig().HTTP
	var nilWorkflow *Service
	for name, setup := range map[string]func() (Workflow, HTTPConfig, *log.Logger){
		"nil workflow":       func() (Workflow, HTTPConfig, *log.Logger) { return nil, valid, log.Default() },
		"typed nil workflow": func() (Workflow, HTTPConfig, *log.Logger) { return nilWorkflow, valid, log.Default() },
		"nil logger":         func() (Workflow, HTTPConfig, *log.Logger) { return stubWorkflow{}, valid, nil },
		"body limit": func() (Workflow, HTTPConfig, *log.Logger) {
			config := valid
			config.MaximumBodyBytes = 0
			return stubWorkflow{}, config, log.Default()
		},
		"timeout": func() (Workflow, HTTPConfig, *log.Logger) {
			config := valid
			config.RequestTimeout = 0
			return stubWorkflow{}, config, log.Default()
		},
	} {
		t.Run(name, func(t *testing.T) {
			workflow, config, logger := setup()
			if _, err := NewEngine(workflow, config, logger); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestHTTPMapsWorkflowErrorsAndRedactsLogs(t *testing.T) {
	tests := []struct {
		name, code string
		err        error
		status     int
	}{
		{name: "invalid", err: fmtWrap(ErrInvalidRequest), status: 400, code: "invalid_request"},
		{name: "duplicate", err: fmtWrap(ErrDuplicateContentID), status: 409, code: "duplicate_content_id"},
		{name: "missing", err: fmtWrap(ErrRecordNotFound), status: 404, code: "record_not_found"},
		{name: "capacity", err: fmtWrap(ErrStoreCapacity), status: 503, code: "store_capacity_reached"},
		{name: "internal", err: errors.New("SENTINEL-WORKFLOW-SECRET"), status: 500, code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			engine := newHTTPTestEngine(t, stubWorkflow{create: func(context.Context, CreateRequest) (Record, error) { return Record{}, test.err }}, DefaultConfig().HTTP, &logs)
			body := &trackingBody{Reader: strings.NewReader(`{"content_id":"SENTINEL-ID","content":"SENTINEL-CONTENT","metadata":{"SENTINEL-METADATA-KEY":"SENTINEL-METADATA-VALUE"}}`)}
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/moderation/records", body)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
			}
			for _, secret := range []string{"SENTINEL-ID", "SENTINEL-CONTENT", "SENTINEL-METADATA-KEY", "SENTINEL-METADATA-VALUE", "SENTINEL-WORKFLOW-SECRET"} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("logs leaked %q: %s", secret, logs.String())
				}
			}
			if body.closed != 1 {
				t.Fatalf("body close count = %d", body.closed)
			}
		})
	}
}

func TestHTTPWorkflowTimeout(t *testing.T) {
	config := DefaultConfig().HTTP
	config.RequestTimeout = 5 * time.Millisecond
	workflow := stubWorkflow{search: func(ctx context.Context, _ SearchRequest) (SearchResponse, error) {
		<-ctx.Done()
		return SearchResponse{}, ctx.Err()
	}}
	engine := newHTTPTestEngine(t, workflow, config, io.Discard)
	body := &trackingBody{Reader: strings.NewReader(`{"query":"delivery"}`)}
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/moderation/records/search", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusRequestTimeout || !strings.Contains(response.Body.String(), `"code":"request_timeout"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if body.closed != 1 {
		t.Fatalf("body close count = %d", body.closed)
	}
}

func TestHTTPParentCancellationDoesNotWrite(t *testing.T) {
	workflow := stubWorkflow{get: func(ctx context.Context, _ string) (Record, error) { return Record{}, ctx.Err() }}
	engine := newHTTPTestEngine(t, workflow, DefaultConfig().HTTP, io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := &trackingBody{Reader: strings.NewReader("")}
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/moderation/records/article-100", body)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Body.Len() != 0 {
		t.Fatalf("response body = %s", response.Body.String())
	}
	if body.closed != 1 {
		t.Fatalf("body close count = %d", body.closed)
	}
}

func TestHTTPLateSuccessHonorsDeadlineAndParentCancellation(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		config := DefaultConfig().HTTP
		config.RequestTimeout = 5 * time.Millisecond
		workflow := stubWorkflow{search: func(ctx context.Context, _ SearchRequest) (SearchResponse, error) {
			<-ctx.Done()
			return SearchResponse{Hits: []SearchHit{}}, nil
		}}
		engine := newHTTPTestEngine(t, workflow, config, io.Discard)
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/moderation/records/search", strings.NewReader(`{"query":"delivery"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusRequestTimeout || !strings.Contains(response.Body.String(), `"code":"request_timeout"`) {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})

	t.Run("parent cancellation", func(t *testing.T) {
		workflow := stubWorkflow{get: func(ctx context.Context, _ string) (Record, error) {
			<-ctx.Done()
			return Record{ContentID: "must-not-write"}, nil
		}}
		engine := newHTTPTestEngine(t, workflow, DefaultConfig().HTTP, io.Discard)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/moderation/records/article-100", nil)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Body.Len() != 0 {
			t.Fatalf("response body = %s", response.Body.String())
		}
	})
}

func TestHTTPDoesNotTrustForwardedClientIP(t *testing.T) {
	engine, err := NewEngine(stubWorkflow{}, DefaultConfig().HTTP, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	engine.GET("/test-client-ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test-client-ip", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Body.String() != "192.0.2.10" {
		t.Fatalf("ClientIP = %q, spoofed forwarded address was trusted", response.Body.String())
	}
}

func newHTTPTestEngine(t *testing.T, workflow Workflow, config HTTPConfig, output io.Writer) http.Handler {
	t.Helper()
	engine, err := NewEngine(workflow, config, log.New(output, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

type trackingBody struct {
	io.Reader
	closed int
}

func (b *trackingBody) Close() error { b.closed++; return nil }

func fmtWrap(err error) error { return errors.Join(errors.New("wrapped"), err) }
