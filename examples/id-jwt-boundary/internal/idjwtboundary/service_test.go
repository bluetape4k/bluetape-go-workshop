package idjwtboundary

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/id"
	btjwt "github.com/bluetape4k/bluetape-go/jwt"
	"github.com/gin-gonic/gin"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestRouterCreatesOrderWithValidToken(t *testing.T) {
	router := newTestRouter(t, testConfig())

	token := issueToken(t, router, TokenRequest{
		Subject:    "customer-1001",
		Role:       "customer",
		Scopes:     []string{"orders:create"},
		TTLSeconds: 900,
	})

	rec := performJSON(t, router, http.MethodPost, "/orders", OrderRequest{
		SKU:      "sku-blue-tape",
		Quantity: 2,
	}, "Bearer "+token)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /orders status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response OrderResponse
	decodeJSON(t, rec, &response)

	if response.Subject != "customer-1001" {
		t.Fatalf("subject = %q", response.Subject)
	}
	if response.Role != "customer" {
		t.Fatalf("role = %q", response.Role)
	}
	if response.Scope != "orders:create" {
		t.Fatalf("scope = %q", response.Scope)
	}
	if response.SKU != "sku-blue-tape" || response.Quantity != 2 {
		t.Fatalf("order echo = %+v", response)
	}
	assertUUIDV7(t, response.OrderID)
	assertUUIDV7(t, response.RequestID)
	if response.OrderID == response.RequestID {
		t.Fatalf("order_id and request_id must differ: %q", response.OrderID)
	}
}

func TestRouterRejectsTokenBoundaryFailures(t *testing.T) {
	now := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		token         func(t *testing.T, router http.Handler) string
		authorization string
		wantStatus    int
		wantCode      string
	}{
		{
			name:       "missing token",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "missing_token",
		},
		{
			name: "expired token",
			token: func(t *testing.T, _ http.Handler) string {
				provider := newTokenProvider(t, testSecret, func() time.Time { return now.Add(-2 * time.Hour) })
				token, err := provider.Compose(
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(DefaultAudience),
					btjwt.WithExpiresAfter(time.Minute),
					btjwt.WithClaim("role", "customer"),
					btjwt.WithClaim("scope", "orders:create"),
				)
				if err != nil {
					t.Fatalf("compose expired token: %v", err)
				}
				return token
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "expired_token",
		},
		{
			name:          "malformed token",
			authorization: "Bearer not-a-jwt",
			wantStatus:    http.StatusUnauthorized,
			wantCode:      "invalid_token",
		},
		{
			name: "wrong signing key",
			token: func(t *testing.T, _ http.Handler) string {
				provider := newTokenProvider(t, strings.Repeat("x", 32), func() time.Time { return now })
				token, err := provider.Compose(
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(DefaultAudience),
					btjwt.WithExpiresAfter(15*time.Minute),
					btjwt.WithClaim("role", "customer"),
					btjwt.WithClaim("scope", "orders:create"),
				)
				if err != nil {
					t.Fatalf("compose wrong-key token: %v", err)
				}
				return token
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_token",
		},
		{
			name: "valid token without required scope",
			token: func(t *testing.T, router http.Handler) string {
				return issueToken(t, router, TokenRequest{
					Subject:    "customer-1001",
					Role:       "customer",
					Scopes:     []string{"orders:read"},
					TTLSeconds: 900,
				})
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, testConfig(WithClock(func() time.Time { return now })))
			auth := tt.authorization
			if tt.token != nil {
				auth = "Bearer " + tt.token(t, router)
			}

			rec := performJSON(t, router, http.MethodPost, "/orders", OrderRequest{
				SKU:      "sku-blue-tape",
				Quantity: 1,
			}, auth)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			assertNoLeak(t, rec.Body.String())
		})
	}
}

func TestRouterRejectsInvalidOrderRequests(t *testing.T) {
	router := newTestRouter(t, testConfig())
	token := issueToken(t, router, TokenRequest{
		Subject:    "customer-1001",
		Role:       "customer",
		Scopes:     []string{"orders:create"},
		TTLSeconds: 900,
	})

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"sku":`},
		{name: "blank sku", body: `{"sku":" ","quantity":1}`},
		{name: "zero quantity", body: `{"sku":"sku-blue-tape","quantity":0}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/orders", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assertError(t, rec, http.StatusBadRequest, "invalid_request")
			assertNoLeak(t, rec.Body.String())
		})
	}
}

func TestIssueTokenRejectsInvalidRequest(t *testing.T) {
	router := newTestRouter(t, testConfig())

	rec := performJSON(t, router, http.MethodPost, "/tokens", TokenRequest{
		Subject:    "",
		Role:       "customer",
		Scopes:     []string{"orders:create"},
		TTLSeconds: 900,
	}, "")

	assertError(t, rec, http.StatusBadRequest, "invalid_request")
	assertNoLeak(t, rec.Body.String())
}

func TestCreateOrderMapsIDGeneratorFailureToInternalError(t *testing.T) {
	router := newTestRouter(t, testConfig(WithIDGenerator(failingIDGenerator{})))
	token := issueToken(t, router, TokenRequest{
		Subject:    "customer-1001",
		Role:       "customer",
		Scopes:     []string{"orders:create"},
		TTLSeconds: 900,
	})

	rec := performJSON(t, router, http.MethodPost, "/orders", OrderRequest{
		SKU:      "sku-blue-tape",
		Quantity: 1,
	}, "Bearer "+token)

	assertError(t, rec, http.StatusInternalServerError, "internal_error")
	assertNoLeak(t, rec.Body.String())
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(t, testConfig())

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

func testConfig(options ...Option) []Option {
	now := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	base := []Option{
		WithClock(func() time.Time { return now }),
		WithSecret([]byte(testSecret)),
		WithIDGenerator(&sequenceIDGenerator{values: []string{
			"018f4c7c-0c00-7c00-8000-000000000000",
			"018f4c7c-0c00-7c00-8000-000000000001",
		}}),
	}
	return append(base, options...)
}

func newTestRouter(t *testing.T, options []Option) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	service, err := NewService(options...)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return NewRouter(service)
}

func issueToken(t *testing.T, router http.Handler, request TokenRequest) string {
	t.Helper()

	rec := performJSON(t, router, http.MethodPost, "/tokens", request, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /tokens status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response TokenResponse
	decodeJSON(t, rec, &response)
	if response.TokenType != "Bearer" {
		t.Fatalf("token_type = %q", response.TokenType)
	}
	if response.Token == "" {
		t.Fatal("token must not be blank")
	}
	if response.ExpiresInSeconds != request.TTLSeconds {
		t.Fatalf("expires_in_seconds = %d", response.ExpiresInSeconds)
	}
	return response.Token
}

func performJSON(t *testing.T, router http.Handler, method, path string, body any, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	req := httptest.NewRequestWithContext(context.Background(), method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
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
		t.Fatalf("error_code = %q, want %q", response.ErrorCode, wantCode)
	}
	if response.Message == "" {
		t.Fatal("message must not be blank")
	}
}

func assertNoLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{testSecret, "not-a-jwt", "signature is invalid", "token is malformed", "0123456789abcdef"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaks %q: %s", forbidden, body)
		}
	}
}

func assertUUIDV7(t *testing.T, value string) {
	t.Helper()
	parsed, err := id.ParseUUID(value)
	if err != nil {
		t.Fatalf("ParseUUID(%q): %v", value, err)
	}
	if parsed != value {
		t.Fatalf("parsed uuid = %q, want %q", parsed, value)
	}
	if len(value) != 36 {
		t.Fatalf("uuid length = %d for %q", len(value), value)
	}
	if value[14] != '7' {
		t.Fatalf("uuid version = %q for %q", value[14], value)
	}
}

func newTokenProvider(t *testing.T, secret string, clock func() time.Time) *btjwt.Provider {
	t.Helper()
	provider, err := btjwt.NewFixedHMACProvider(
		btjwt.HS256,
		[]byte(secret),
		btjwt.WithClock(clock),
		btjwt.WithKeyIDGenerator(func() (string, error) { return "local-demo-key", nil }),
	)
	if err != nil {
		t.Fatalf("NewFixedHMACProvider: %v", err)
	}
	return provider
}

type failingIDGenerator struct{}

func (failingIDGenerator) NextString() (string, error) {
	return "", errInjectedIDFailure
}

type sequenceIDGenerator struct {
	values []string
}

func (g *sequenceIDGenerator) NextString() (string, error) {
	if len(g.values) == 0 {
		return "", errInjectedIDFailure
	}
	value := g.values[0]
	g.values = g.values[1:]
	return value, nil
}
