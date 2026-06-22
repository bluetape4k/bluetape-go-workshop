package invoicerules

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

func TestRouterEvaluatesInvoice(t *testing.T) {
	router := newTestRouter()

	rec := performJSON(t, router, http.MethodPost, "/invoices/evaluate", InvoiceRequest{
		InvoiceID:    "inv-http-1001",
		CustomerTier: "vip",
		Region:       "EU",
		Lines: []LineItemRequest{
			{
				LineID:   "svc-usd",
				Amount:   "19.995",
				Currency: "USD",
				Quantity: 2,
				Category: "service",
			},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /invoices/evaluate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response InvoiceResponse
	decodeJSON(t, rec, &response)
	if len(response.TotalsByCurrency) != 1 {
		t.Fatalf("totals = %d, want 1", len(response.TotalsByCurrency))
	}
	assertCurrencyTotal(t, response.TotalsByCurrency[0], "USD", "39.99", "2.00", "7.60", "45.59")
	assertRule(t, response.Rules, "vip-service-discount", "svc-usd", "USD", RuleAccepted, "2.00", "")
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
			body:       `{"invoice_id":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "invalid currency",
			body: `{
				"invoice_id":"inv-http-1002",
				"lines":[{"line_id":"bad-currency","amount":"12.00","currency":"XXX","quantity":1,"category":"service"}]
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_money",
		},
		{
			name: "invalid amount",
			body: `{
				"invoice_id":"inv-http-1003",
				"lines":[{"line_id":"bad-amount","amount":"not-money","currency":"USD","quantity":1,"category":"service"}]
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_money",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/invoices/evaluate", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if strings.Contains(rec.Body.String(), "not-money") || strings.Contains(rec.Body.String(), "XXX") {
				t.Fatalf("error response leaked raw parser input: %s", rec.Body.String())
			}
		})
	}
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
