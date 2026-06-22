package tokenrefresh

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	btjwt "github.com/bluetape4k/bluetape-go/jwt"
	"github.com/gin-gonic/gin"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestRouterIssuesSessionAndValidatesAccessToken(t *testing.T) {
	router := newTestRouter(t, testOptions())

	session := issueSession(t, router, SessionRequest{
		Subject:    "customer-1001",
		Role:       "customer",
		Scopes:     []string{"profile:read"},
		TTLSeconds: 300,
	})

	if session.TokenType != "Bearer" {
		t.Fatalf("token_type = %q", session.TokenType)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatalf("session tokens must not be blank: %+v", session)
	}
	if session.AccessExpiresInSeconds != 300 {
		t.Fatalf("access_expires_in_seconds = %d", session.AccessExpiresInSeconds)
	}
	if session.RefreshExpiresInSeconds <= session.AccessExpiresInSeconds {
		t.Fatalf("refresh ttl must be longer than access ttl: %+v", session)
	}
	if session.SessionID == "" {
		t.Fatalf("session_id must not be blank")
	}

	rec := performJSON(t, router, http.MethodGet, "/profile", nil, "Bearer "+session.AccessToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /profile status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var profile ProfileResponse
	decodeJSON(t, rec, &profile)
	if profile.Subject != "customer-1001" || profile.Role != "customer" {
		t.Fatalf("profile identity = %+v", profile)
	}
	if profile.Scope != "profile:read" {
		t.Fatalf("scope = %q", profile.Scope)
	}
	if profile.SessionID != session.SessionID {
		t.Fatalf("session_id = %q, want %q", profile.SessionID, session.SessionID)
	}
	if profile.AccessExpiresInSeconds <= 0 {
		t.Fatalf("access_expires_in_seconds = %d", profile.AccessExpiresInSeconds)
	}
}

func TestRouterRejectsAccessTokenFailures(t *testing.T) {
	now := fixedNow()

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
			name: "expired access token",
			token: func(t *testing.T, _ http.Handler) string {
				return composeToken(t, testSecret, func() time.Time { return now.Add(-2 * time.Hour) },
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(AccessAudience),
					btjwt.WithExpiresAfter(time.Minute),
					btjwt.WithJWTID("expired-access-jti"),
					btjwt.WithClaim("token_use", TokenUseAccess),
					btjwt.WithClaim("role", "customer"),
					btjwt.WithClaim("scope", "profile:read"),
					btjwt.WithClaim("session_id", "session-expired"),
				)
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
				return composeToken(t, strings.Repeat("x", 32), func() time.Time { return now },
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(AccessAudience),
					btjwt.WithExpiresAfter(15*time.Minute),
					btjwt.WithJWTID("wrong-key-jti"),
					btjwt.WithClaim("token_use", TokenUseAccess),
					btjwt.WithClaim("role", "customer"),
					btjwt.WithClaim("scope", "profile:read"),
					btjwt.WithClaim("session_id", "session-wrong-key"),
				)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_token",
		},
		{
			name: "wrong audience",
			token: func(t *testing.T, _ http.Handler) string {
				return composeToken(t, testSecret, func() time.Time { return now },
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(RefreshAudience),
					btjwt.WithExpiresAfter(15*time.Minute),
					btjwt.WithJWTID("wrong-audience-jti"),
					btjwt.WithClaim("token_use", TokenUseAccess),
					btjwt.WithClaim("role", "customer"),
					btjwt.WithClaim("scope", "profile:read"),
					btjwt.WithClaim("session_id", "session-wrong-audience"),
				)
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "invalid_claims",
		},
		{
			name: "refresh token submitted as access token",
			token: func(t *testing.T, router http.Handler) string {
				session := issueSession(t, router, validSessionRequest())
				return session.RefreshToken
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "invalid_claims",
		},
		{
			name: "missing required scope",
			token: func(t *testing.T, router http.Handler) string {
				session := issueSession(t, router, SessionRequest{
					Subject:    "customer-1001",
					Role:       "customer",
					Scopes:     []string{"profile:write"},
					TTLSeconds: 300,
				})
				return session.AccessToken
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "invalid_claims",
		},
		{
			name: "wrong token use",
			token: func(t *testing.T, _ http.Handler) string {
				return composeToken(t, testSecret, func() time.Time { return now },
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(AccessAudience),
					btjwt.WithExpiresAfter(15*time.Minute),
					btjwt.WithJWTID("wrong-use-jti"),
					btjwt.WithClaim("token_use", "delegation"),
					btjwt.WithClaim("role", "customer"),
					btjwt.WithClaim("scope", "profile:read"),
					btjwt.WithClaim("session_id", "session-wrong-use"),
				)
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "invalid_claims",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, testOptions(WithClock(func() time.Time { return now })))
			auth := tt.authorization
			if tt.token != nil {
				auth = "Bearer " + tt.token(t, router)
			}

			rec := performJSON(t, router, http.MethodGet, "/profile", nil, auth)

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			assertNoLeak(t, rec.Body.String())
		})
	}
}

func TestRouterRefreshesAccessToken(t *testing.T) {
	router := newTestRouter(t, testOptions())
	session := issueSession(t, router, validSessionRequest())

	rec := performJSON(t, router, http.MethodPost, "/tokens/refresh", RefreshRequest{
		RefreshToken: session.RefreshToken,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /tokens/refresh status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response AccessTokenResponse
	decodeJSON(t, rec, &response)
	if response.TokenType != "Bearer" {
		t.Fatalf("token_type = %q", response.TokenType)
	}
	if response.AccessToken == "" {
		t.Fatalf("access_token must not be blank")
	}
	if response.SessionID != session.SessionID {
		t.Fatalf("session_id = %q, want %q", response.SessionID, session.SessionID)
	}
	if response.ExpiresInSeconds <= 0 {
		t.Fatalf("expires_in_seconds = %d", response.ExpiresInSeconds)
	}

	profileRec := performJSON(t, router, http.MethodGet, "/profile", nil, "Bearer "+response.AccessToken)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("GET /profile refreshed token status = %d, body = %s", profileRec.Code, profileRec.Body.String())
	}
	var profile ProfileResponse
	decodeJSON(t, profileRec, &profile)
	if profile.Subject != "customer-1001" || profile.SessionID != session.SessionID {
		t.Fatalf("profile after refresh = %+v", profile)
	}
}

func TestRouterRejectsRefreshTokenFailures(t *testing.T) {
	now := fixedNow()

	tests := []struct {
		name       string
		request    func(t *testing.T, router http.Handler) RefreshRequest
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing refresh token",
			request:    func(_ *testing.T, _ http.Handler) RefreshRequest { return RefreshRequest{} },
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name: "access token submitted as refresh token",
			request: func(t *testing.T, router http.Handler) RefreshRequest {
				session := issueSession(t, router, validSessionRequest())
				return RefreshRequest{RefreshToken: session.AccessToken}
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "invalid_claims",
		},
		{
			name: "malformed refresh token",
			request: func(_ *testing.T, _ http.Handler) RefreshRequest {
				return RefreshRequest{RefreshToken: "not-a-jwt"}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_token",
		},
		{
			name: "expired refresh token",
			request: func(t *testing.T, _ http.Handler) RefreshRequest {
				token := composeToken(t, testSecret, func() time.Time { return now.Add(-48 * time.Hour) },
					btjwt.WithIssuer(DefaultIssuer),
					btjwt.WithSubject("customer-1001"),
					btjwt.WithAudience(RefreshAudience),
					btjwt.WithExpiresAfter(time.Hour),
					btjwt.WithJWTID("expired-refresh-jti"),
					btjwt.WithClaim("token_use", TokenUseRefresh),
					btjwt.WithClaim("session_id", "session-expired-refresh"),
				)
				return RefreshRequest{RefreshToken: token}
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "expired_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, testOptions(WithClock(func() time.Time { return now })))

			rec := performJSON(t, router, http.MethodPost, "/tokens/refresh", tt.request(t, router), "")

			assertError(t, rec, tt.wantStatus, tt.wantCode)
			assertNoLeak(t, rec.Body.String())
		})
	}
}

func TestHealthz(t *testing.T) {
	router := newTestRouter(t, testOptions())

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

func validSessionRequest() SessionRequest {
	return SessionRequest{
		Subject:    "customer-1001",
		Role:       "customer",
		Scopes:     []string{"profile:read"},
		TTLSeconds: 300,
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
}

func testOptions(options ...Option) []Option {
	base := []Option{
		WithClock(fixedNow),
		WithSecret([]byte(testSecret)),
		WithIDGenerator(&sequenceIDGenerator{values: []string{
			"session-1001",
			"access-jti-1001",
			"refresh-jti-1001",
			"access-jti-1002",
			"session-1002",
			"access-jti-1003",
			"refresh-jti-1003",
			"access-jti-1004",
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

func issueSession(t *testing.T, router http.Handler, request SessionRequest) SessionResponse {
	t.Helper()

	rec := performJSON(t, router, http.MethodPost, "/sessions", request, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /sessions status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var response SessionResponse
	decodeJSON(t, rec, &response)
	return response
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

func composeToken(t *testing.T, secret string, clock func() time.Time, options ...btjwt.ComposeOption) string {
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
	token, err := provider.Compose(options...)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	return token
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
