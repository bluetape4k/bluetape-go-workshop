package dedupe

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterAdmitsAndThenMarksProbablySeen(t *testing.T) {
	router := newTestRouter(t)

	first := performJSON(t, router, http.MethodPost, "/events/admit", EventRequest{
		EventID: "evt-http-1001",
		Source:  "checkout",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	var firstDecision AdmitResponse
	decodeJSON(t, first, &firstDecision)
	if firstDecision.Decision != DecisionAdmit || !firstDecision.Accepted {
		t.Fatalf("first decision = %+v, want admitted", firstDecision)
	}

	second := performJSON(t, router, http.MethodPost, "/events/admit", EventRequest{
		EventID: "evt-http-1001",
		Source:  "checkout",
	})
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
	var secondDecision AdmitResponse
	decodeJSON(t, second, &secondDecision)
	if secondDecision.Decision != DecisionProbablySeen || secondDecision.Accepted {
		t.Fatalf("second decision = %+v, want probably seen rejection", secondDecision)
	}
}

func TestRouterMapsInvalidRequest(t *testing.T) {
	router := newTestRouter(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"event_id":`},
		{name: "missing event id", body: `{"source":"checkout"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/events/admit", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, http.StatusBadRequest, "invalid_request")
		})
	}
}

func TestRouterReturnsStats(t *testing.T) {
	router := newTestRouter(t)
	_ = performJSON(t, router, http.MethodPost, "/events/admit", EventRequest{EventID: "evt-http-1002"})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/filters/current", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /filters/current status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var stats FilterStats
	decodeJSON(t, rec, &stats)
	if stats.ExpectedInsertions != 100 {
		t.Fatalf("expected_insertions = %d, want 100", stats.ExpectedInsertions)
	}
	if stats.ApproximateElementCount == 0 {
		t.Fatalf("approximate_element_count = 0, want > 0")
	}
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("GET /healthz body = %s", rec.Body.String())
	}
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return NewRouter(newTestService(t))
}

func performJSON(t *testing.T, handler http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, &payload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var response ErrorResponse
	decodeJSON(t, rec, &response)
	if response.ErrorCode != wantCode {
		t.Fatalf("error_code = %q, want %q, body = %s", response.ErrorCode, wantCode, rec.Body.String())
	}
}
