package exchangepricing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/money"
	"github.com/gin-gonic/gin"
)

func TestRouterQuotesDisplayPrice(t *testing.T) {
	router := newTestRouter(t, newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil))

	rec := performJSON(t, router, http.MethodPost, "/quotes", QuoteRequest{
		QuoteID:      "quote-http-1001",
		BaseCurrency: "USD",
		Locale:       "ko-KR",
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "19.995", Currency: "USD", Quantity: 2},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /quotes status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var quote QuoteResponse
	decodeJSON(t, rec, &quote)
	assertMoney(t, quote.Subtotal, "USD", "39.99")
	assertMoney(t, quote.DisplayTotal, "KRW", "51987")
	if quote.Rate.Source != money.ECBSource {
		t.Fatalf("rate source = %q", quote.Rate.Source)
	}
}

func TestRouterMapsPublicErrors(t *testing.T) {
	tests := []struct {
		name       string
		provider   money.ExchangeRateProvider
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "malformed json",
			provider:   newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil),
			body:       `{"quote_id":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "unsupported locale",
			provider:   newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil),
			body:       `{"quote_id":"q-1","base_currency":"USD","locale":"en","items":[{"sku":"sku-1","unit_price":"10.00","currency":"USD","quantity":1}]}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_locale",
		},
		{
			name:       "currency mismatch",
			provider:   newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil),
			body:       `{"quote_id":"q-2","base_currency":"USD","locale":"ko-KR","items":[{"sku":"sku-1","unit_price":"10.00","currency":"EUR","quantity":1}]}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "currency_mismatch",
		},
		{
			name:       "stale rejected",
			provider:   newScriptedProvider(t, money.USD, money.KRW, "1300", true, errors.New("refresh failed")),
			body:       `{"quote_id":"q-3","base_currency":"USD","locale":"ko-KR","items":[{"sku":"sku-1","unit_price":"10.00","currency":"USD","quantity":1}]}`,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "stale_quote",
		},
		{
			name:       "provider unavailable",
			provider:   &scriptedProvider{err: errors.Join(money.ErrExchangeRateProvider, errors.New("network unavailable"))},
			body:       `{"quote_id":"q-4","base_currency":"USD","locale":"ko-KR","items":[{"sku":"sku-1","unit_price":"10.00","currency":"USD","quantity":1}]}`,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "exchange_rate_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, tt.provider)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/quotes", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if strings.Contains(rec.Body.String(), "network unavailable") || strings.Contains(rec.Body.String(), "refresh failed") {
				t.Fatalf("error response leaked provider detail: %s", rec.Body.String())
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(t, newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil))

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

func newTestRouter(t *testing.T, provider money.ExchangeRateProvider) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return NewRouter(NewService(provider))
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
