package checkoutguard

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

func TestRouterGuardsCheckout(t *testing.T) {
	service := newTestService(t)
	router := newTestRouter(service)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})

	rec := performJSON(t, router, http.MethodPost, "/checkout/guard", validCheckoutRequest("idem-http-1001"), "Bearer "+token.Token)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /checkout/guard status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response CheckoutResponse
	decodeJSON(t, rec, &response)
	assertMoney(t, response.Pricing.Total, "USD", "45.59")
}

func TestRouterMapsPublicErrors(t *testing.T) {
	service := newTestService(t)
	router := newTestRouter(service)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})
	underScopedToken := issueTestToken(t, service, "customer", []string{"profile:read"})

	tests := []struct {
		name          string
		authorization string
		body          string
		wantStatus    int
		wantCode      string
	}{
		{
			name:       "missing token",
			body:       mustJSON(t, validCheckoutRequest("idem-error-missing-token")),
			wantStatus: http.StatusUnauthorized,
			wantCode:   "missing_token",
		},
		{
			name:          "malformed json",
			authorization: "Bearer " + token.Token,
			body:          `{"checkout_id":`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "invalid_request",
		},
		{
			name:          "invalid claims",
			authorization: "Bearer " + underScopedToken.Token,
			body:          mustJSON(t, validCheckoutRequest("idem-error-claims")),
			wantStatus:    http.StatusForbidden,
			wantCode:      "invalid_claims",
		},
		{
			name:          "invalid money",
			authorization: "Bearer " + token.Token,
			body: `{
				"checkout_id":"chk-http-1002",
				"idempotency_key":"idem-error-money",
				"currency":"USD",
				"items":[{"line_id":"bad","sku":"bad","unit_price":"not-money","currency":"USD","quantity":1,"category":"service"}]
			}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_money",
		},
		{
			name:          "rule denied",
			authorization: "Bearer " + token.Token,
			body: mustJSON(t, CheckoutRequest{
				CheckoutID:     "chk-http-1003",
				IdempotencyKey: "idem-error-rule",
				Currency:       "USD",
				Items: []LineItemRequest{
					{LineID: "restricted", SKU: "restricted", UnitPrice: "12.00", Currency: "USD", Quantity: 1, Category: "restricted"},
				},
			}),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "rule_denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/checkout/guard", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			if strings.Contains(rec.Body.String(), "not-money") {
				t.Fatalf("error response leaked raw parser input: %s", rec.Body.String())
			}
		})
	}
}

func TestRouterMapsDuplicateSubmission(t *testing.T) {
	service := newTestService(t)
	router := newTestRouter(service)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})

	first := performJSON(t, router, http.MethodPost, "/checkout/guard", validCheckoutRequest("idem-http-duplicate"), "Bearer "+token.Token)
	if first.Code != http.StatusCreated {
		t.Fatalf("first checkout status = %d, body = %s", first.Code, first.Body.String())
	}
	second := performJSON(t, router, http.MethodPost, "/checkout/guard", validCheckoutRequest("idem-http-duplicate"), "Bearer "+token.Token)
	assertError(t, second, http.StatusConflict, "duplicate_submission")
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(newTestService(t))

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

func newTestRouter(service *Service) http.Handler {
	gin.SetMode(gin.TestMode)
	return NewRouter(service)
}

func performJSON(t *testing.T, handler http.Handler, method, target string, body any, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, &payload)
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
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

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return string(payload)
}
