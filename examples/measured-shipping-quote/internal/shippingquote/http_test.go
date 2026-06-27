package shippingquote

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

func TestRouterQuotesMeasurements(t *testing.T) {
	router := newTestRouter()
	divisor := 6000.0

	rec := performJSON(t, router, http.MethodPost, "/quotes", QuoteRequest{
		QuoteID:             "ship-http-1001",
		DestinationZone:     "jp-tokyo",
		Width:               "12 in",
		Height:              "10 in",
		Length:              "18 in",
		Weight:              "5 lb",
		DimensionalDivisor:  &divisor,
		PreferredLengthUnit: "in",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /quotes status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var quote QuoteResponse
	decodeJSON(t, rec, &quote)
	if quote.QuoteID != "ship-http-1001" {
		t.Fatalf("QuoteID = %q", quote.QuoteID)
	}
	assertMeasurement(t, quote.Dimensions.Width, "in", 12)
	if quote.DimensionalDivisor.Amount != 6000 || quote.DimensionalDivisor.Unit != defaultDimensionalDivisorCM {
		t.Fatalf("dimensional divisor = %#v", quote.DimensionalDivisor)
	}
}

func TestRouterMapsPublicErrors(t *testing.T) {
	router := newTestRouter()

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "malformed json",
			body:       `{"quote_id":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "parse failure",
			body: `{
				"quote_id":"ship-http-1002",
				"destination_zone":"kr-seoul",
				"width":"many cm",
				"height":"30 cm",
				"length":"20 cm",
				"weight":"3 kg"
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_measure",
		},
		{
			name: "incompatible unit",
			body: `{
				"quote_id":"ship-http-1003",
				"destination_zone":"kr-seoul",
				"width":"40 kg",
				"height":"30 cm",
				"length":"20 cm",
				"weight":"3 kg"
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "incompatible_unit",
		},
		{
			name: "zero dimensional divisor",
			body: `{
				"quote_id":"ship-http-1004",
				"destination_zone":"kr-seoul",
				"width":"40 cm",
				"height":"30 cm",
				"length":"20 cm",
				"weight":"3 kg",
				"dimensional_divisor":0
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_dimensional_divisor",
		},
		{
			name: "declared max side mismatch",
			body: `{
				"quote_id":"ship-http-1005",
				"destination_zone":"kr-seoul",
				"width":"40 cm",
				"height":"30 cm",
				"length":"20 cm",
				"weight":"3 kg",
				"declared_max_side":"10 cm"
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/quotes", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if strings.Contains(rec.Body.String(), "many cm") || strings.Contains(rec.Body.String(), "40 kg") {
				t.Fatalf("error response leaked raw measurement: %s", rec.Body.String())
			}
		})
	}
}

func TestRouterRejectsOversizeJSONBody(t *testing.T) {
	router := newTestRouter()

	body := `{"quote_id":"ship-http-large","destination_zone":"kr-seoul","width":"40 cm","height":"30 cm","length":"20 cm","weight":"3 kg","padding":"` +
		strings.Repeat("x", maxJSONBodySize) + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/quotes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assertError(t, rec, http.StatusBadRequest, "invalid_request")
}

func TestHealthz(t *testing.T) {
	router := newTestRouter()

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

func newTestRouter() http.Handler {
	gin.SetMode(gin.TestMode)
	return NewRouter(NewService())
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
