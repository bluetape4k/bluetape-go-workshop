package searchapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestServiceSearchMaskUsesUnicodeBoundaryAndPolicy(t *testing.T) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SearchMask(SearchMaskRequest{
		RequestID: "search-1001",
		Text:      "쿠폰 sale refundable refund window bad wolf",
		Mask:      "#",
	})
	if err != nil {
		t.Fatalf("SearchMask() error = %v", err)
	}

	if result.MaskedText != "## #### refundable ############# ########" {
		t.Fatalf("MaskedText = %q", result.MaskedText)
	}
	gotIDs := matchIDs(result.Matches)
	wantIDs := []string{"promo-coupon", "promo-sale", "policy-refund-window", "support-bad-wolf"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("match ids = %#v, want %#v", gotIDs, wantIDs)
	}
	if result.Matches[0].Text != "쿠폰" || result.Matches[0].Start != 0 || result.Matches[0].End != 6 {
		t.Fatalf("first match = %#v", result.Matches[0])
	}
	if len(result.UnicodeCaveats) < 2 {
		t.Fatalf("UnicodeCaveats = %#v, want boundary caveats", result.UnicodeCaveats)
	}
}

func TestHTTPSearchMaskResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := newTestServer(t)

	response := performJSON(api, http.MethodPost, "/text/search-mask", SearchMaskRequest{
		RequestID: "search-2001",
		Text:      "sale 쿠폰 badge bad wolf",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("POST /text/search-mask status = %d body = %s", response.Code, response.Body.String())
	}

	var body SearchMaskResult
	decodeJSON(t, response, &body)
	if body.RequestID != "search-2001" {
		t.Fatalf("RequestID = %q", body.RequestID)
	}
	if body.MaskedText != "**** ** badge ********" {
		t.Fatalf("MaskedText = %q", body.MaskedText)
	}
	if gotIDs := matchIDs(body.Matches); !reflect.DeepEqual(gotIDs, []string{"promo-sale", "promo-coupon", "support-bad-wolf"}) {
		t.Fatalf("match ids = %#v", gotIDs)
	}
	if body.Summary.TotalMatches != 3 || body.Summary.PatternsHit != 3 {
		t.Fatalf("Summary = %#v", body.Summary)
	}
}

func TestHTTPValidationErrorsAreStable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := newTestServer(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid JSON",
			body:       `{"request_id":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "missing request id",
			body:       `{"text":"sale"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "blank text",
			body:       `{"request_id":"search-err","text":"   "}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/text/search-mask", bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			api.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
			}
			var body ErrorResponse
			decodeJSON(t, response, &body)
			if body.Code != tt.wantCode {
				t.Fatalf("error code = %q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}

func TestPreviewDocumentsCurlAndBoundaryContract(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.Endpoint != "POST /text/search-mask" {
		t.Fatalf("Endpoint = %q", preview.Endpoint)
	}
	if len(preview.CurlExamples) < 2 {
		t.Fatalf("CurlExamples = %#v, want curl examples", preview.CurlExamples)
	}
	if len(preview.UnicodeCaveats) < 2 {
		t.Fatalf("UnicodeCaveats = %#v, want Unicode caveats", preview.UnicodeCaveats)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	service, err := NewService(DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server, err := NewServer(service)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func performJSON(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	payload, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode JSON body %q: %v", response.Body.String(), err)
	}
}

func matchIDs(matches []SearchMatch) []string {
	ids := make([]string, len(matches))
	for i, match := range matches {
		ids[i] = match.PatternID
	}
	return ids
}
