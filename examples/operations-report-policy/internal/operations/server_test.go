package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestServerHealth(t *testing.T) {
	server := newTestServer(t)

	response := performRequest(t, server, http.MethodGet, "/healthz", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestServerRunCompleted(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		RunID:         "release-1001",
		Policy:        policyContinueOnFailureLabel,
		ProductsValid: boolPtr(true),
	})
	if response.Code != http.StatusOK {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if body.RunID != "release-1001" || body.Policy != policyContinueOnFailureLabel || body.Status != string(workreport.StatusCompleted) {
		t.Fatalf("response identity = %#v", body)
	}
	assertSummary(t, body.Summary, summaryDTO{
		Completed: 6,
		Total:     6,
		Terminal:  true,
		Success:   true,
	})
	assertReport(t, body.Report, "operations-run", workreport.StatusCompleted)
	assertChild(t, body.Report, "load-catalog-snapshot", workreport.StatusCompleted)
	assertChild(t, body.Report, "validate-products", workreport.StatusCompleted)
	assertChild(t, body.Report, "notify-partner", workreport.StatusCompleted)
	assertChild(t, body.Report, "refresh-search-index", workreport.StatusCompleted)
	assertChild(t, body.Report, "write-run-summary", workreport.StatusCompleted)
}

func TestServerRunContinueOnFailurePreservesAllOutcomes(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		RunID:                    "release-partial",
		Policy:                   policyContinueOnFailureLabel,
		ProductsValid:            boolPtr(false),
		RetryPartnerNotification: true,
		SkipSearchIndex:          true,
	})
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusMultiStatus, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "operations-run", workreport.StatusPartial)
	assertSummary(t, body.Summary, summaryDTO{
		Completed: 3,
		Failed:    2,
		Partial:   2,
		Aborted:   1,
		Total:     8,
		Terminal:  true,
		Failure:   true,
	})
	validateProducts := assertChild(t, body.Report, "validate-products", workreport.StatusFailed)
	if validateProducts.Error != ErrProductValidation.Error() {
		t.Fatalf("validation error = %q, want %q", validateProducts.Error, ErrProductValidation.Error())
	}
	notifyPartner := assertChild(t, body.Report, "notify-partner", workreport.StatusPartial)
	assertChild(t, notifyPartner, "notify-partner-attempt-1", workreport.StatusFailed)
	assertChild(t, notifyPartner, "notify-partner-attempt-2", workreport.StatusCompleted)
	refresh := assertChild(t, body.Report, "refresh-search-index", workreport.StatusAborted)
	if refresh.Reason != "skipped by request" {
		t.Fatalf("refresh reason = %q, want skipped by request", refresh.Reason)
	}
	assertChild(t, body.Report, "write-run-summary", workreport.StatusCompleted)
}

func TestServerRunStopOnFailureTruncatesAfterFirstFailure(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		RunID:                    "release-stop",
		Policy:                   policyStopOnFailureLabel,
		ProductsValid:            boolPtr(false),
		RetryPartnerNotification: true,
		SkipSearchIndex:          true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "operations-run", workreport.StatusFailed)
	assertSummary(t, body.Summary, summaryDTO{
		Completed: 1,
		Failed:    2,
		Total:     3,
		Terminal:  true,
		Failure:   true,
	})
	assertChild(t, body.Report, "load-catalog-snapshot", workreport.StatusCompleted)
	assertChild(t, body.Report, "validate-products", workreport.StatusFailed)
	assertNoChild(t, body.Report, "notify-partner")
	assertNoChild(t, body.Report, "refresh-search-index")
	assertNoChild(t, body.Report, "write-run-summary")
}

func TestServerRunStopOnFailureStopsAtRetryPartial(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		RunID:                    "release-retry-stop",
		Policy:                   policyStopOnFailureLabel,
		ProductsValid:            boolPtr(true),
		RetryPartnerNotification: true,
		SkipSearchIndex:          true,
	})
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusMultiStatus, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "operations-run", workreport.StatusPartial)
	notifyPartner := assertChild(t, body.Report, "notify-partner", workreport.StatusPartial)
	assertChild(t, notifyPartner, "notify-partner-attempt-1", workreport.StatusFailed)
	assertChild(t, notifyPartner, "notify-partner-attempt-2", workreport.StatusCompleted)
	assertNoChild(t, body.Report, "refresh-search-index")
	assertNoChild(t, body.Report, "write-run-summary")
}

func TestServerRunSkipOnlyReturnsPartial(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		RunID:           "release-skip",
		Policy:          policyContinueOnFailureLabel,
		ProductsValid:   boolPtr(true),
		SkipSearchIndex: true,
	})
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusMultiStatus, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "operations-run", workreport.StatusPartial)
	assertChild(t, body.Report, "refresh-search-index", workreport.StatusAborted)
	assertSummary(t, body.Summary, summaryDTO{
		Completed: 4,
		Partial:   1,
		Aborted:   1,
		Total:     6,
		Terminal:  true,
		Failure:   true,
	})
}

func TestServerRunCallerCancellationMapsToRequestTimeout(t *testing.T) {
	server := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := postRunWithContext(ctx, t, server, RunRequest{
		RunID:         "release-cancelled",
		Policy:        policyContinueOnFailureLabel,
		ProductsValid: boolPtr(true),
	})
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("cancelled run status = %d, want %d: %s", response.Code, http.StatusRequestTimeout, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "operations-run", workreport.StatusCancelled)
	assertSummary(t, body.Summary, summaryDTO{
		Cancelled: 1,
		Total:     1,
		Terminal:  true,
		Failure:   true,
	})
	if !body.Report.Cancelled {
		t.Fatalf("report should be cancelled: %#v", body.Report)
	}
}

func TestServerRunBadRequests(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		name string
		body []byte
	}{
		{name: "malformed", body: []byte("{")},
		{name: "missing run id", body: []byte(`{"policy":"continue_on_failure","products_valid":true}`)},
		{name: "blank run id", body: []byte(`{"run_id":"   ","policy":"continue_on_failure","products_valid":true}`)},
		{name: "missing policy", body: []byte(`{"run_id":"release-1","products_valid":true}`)},
		{name: "unknown policy", body: []byte(`{"run_id":"release-1","policy":"unknown","products_valid":true}`)},
		{name: "missing products flag", body: []byte(`{"run_id":"release-1","policy":"continue_on_failure"}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(t, server, http.MethodPost, "/operations/report", tt.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			body := decodeResponse[errorResponse](t, response)
			if body.Code != "invalid_request" {
				t.Fatalf("code = %q, want invalid_request", body.Code)
			}
		})
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	server, err := NewServer(Options{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func postRun(t *testing.T, server *Server, request RunRequest) *httptest.ResponseRecorder {
	t.Helper()
	return postRunWithContext(t.Context(), t, server, request)
}

func postRunWithContext(ctx context.Context, t *testing.T, server *Server, request RunRequest) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return performRequestWithContext(ctx, t, server, http.MethodPost, "/operations/report", body)
}

func performRequest(t *testing.T, handler http.Handler, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return performRequestWithContext(t.Context(), t, handler, method, path, body)
}

func performRequestWithContext(ctx context.Context, t *testing.T, handler http.Handler, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
	return value
}

func assertReport(t *testing.T, report reportNode, name string, status workreport.Status) {
	t.Helper()

	if report.Name != name || report.Status != status {
		t.Fatalf("report = %#v, want name=%q status=%q", report, name, status)
	}
}

func assertChild(t *testing.T, parent reportNode, name string, status workreport.Status) reportNode {
	t.Helper()

	for _, child := range parent.Children {
		if child.Name == name {
			assertReport(t, child, name, status)
			return child
		}
	}
	t.Fatalf("child %q not found in %#v", name, parent.Children)
	return reportNode{}
}

func assertNoChild(t *testing.T, parent reportNode, name string) {
	t.Helper()

	for _, child := range parent.Children {
		if child.Name == name {
			t.Fatalf("child %q should not be present in %#v", name, parent.Children)
		}
	}
}

func assertSummary(t *testing.T, actual summaryDTO, expected summaryDTO) {
	t.Helper()

	if actual != expected {
		t.Fatalf("summary = %#v, want %#v", actual, expected)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
