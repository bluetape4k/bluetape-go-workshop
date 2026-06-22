package moneypricing

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

func TestRouterQuotesCart(t *testing.T) {
	router := newTestRouter()

	rec := performJSON(t, router, http.MethodPost, "/quotes", QuoteRequest{
		CartID:       "cart-http-1001",
		Currency:     "USD",
		CustomerTier: "vip",
		CouponCode:   "SAVE10",
		Items: []LineItemRequest{
			{SKU: "book-1", UnitPrice: "19.995", Currency: "USD", Quantity: 2},
			{SKU: "pen-1", UnitPrice: "2.50", Currency: "USD", Quantity: 1},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /quotes status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var quote QuoteResponse
	decodeJSON(t, rec, &quote)
	assertMoney(t, quote.Total, "USD", "28.24")
	assertRule(t, quote.Rules, "vip-ten-percent", RuleAccepted, "4.25", "")
	assertRule(t, quote.Rules, "coupon-save10", RuleAccepted, "10.00", "")
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
			body:       `{"cart_id":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "invalid money",
			body: `{
				"cart_id":"cart-http-1002",
				"currency":"USD",
				"items":[{"sku":"book-1","unit_price":"not-money","currency":"USD","quantity":1}]
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_money",
		},
		{
			name: "currency mismatch",
			body: `{
				"cart_id":"cart-http-1003",
				"currency":"USD",
				"items":[{"sku":"book-1","unit_price":"10.00","currency":"EUR","quantity":1}]
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "currency_mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/quotes", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if strings.Contains(rec.Body.String(), "not-money") {
				t.Fatalf("error response leaked raw amount: %s", rec.Body.String())
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
