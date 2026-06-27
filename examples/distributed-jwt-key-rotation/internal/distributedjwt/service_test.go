package distributedjwt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestServiceSharesRotatedKeysAcrossInstances(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	now := fixedNow()

	issuer, err := NewService(ctx, client, testOptions("shop-auth", &now, "issuer")...)
	if err != nil {
		t.Fatalf("NewService issuer error = %v", err)
	}
	verifier, err := NewService(ctx, client, testOptions("shop-auth", &now, "verifier")...)
	if err != nil {
		t.Fatalf("NewService verifier error = %v", err)
	}

	first, err := issuer.IssueToken(ctx, IssueRequest{
		Subject:    "customer-42",
		Scopes:     []string{"orders:read"},
		TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("IssueToken first error = %v", err)
	}
	firstProfile, err := verifier.VerifyToken(ctx, first.AccessToken)
	if err != nil {
		t.Fatalf("VerifyToken first error = %v", err)
	}
	if firstProfile.Subject != "customer-42" || firstProfile.KID != first.KID {
		t.Fatalf("first profile = %+v, response = %+v", firstProfile, first)
	}

	rotation, err := issuer.RotateKey(ctx)
	if err != nil {
		t.Fatalf("RotateKey error = %v", err)
	}
	if rotation.KID == first.KID {
		t.Fatalf("forced rotation kept KID %q", rotation.KID)
	}
	second, err := issuer.IssueToken(ctx, IssueRequest{
		Subject:    "customer-43",
		Scopes:     []string{"orders:read", "orders:write"},
		TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("IssueToken second error = %v", err)
	}
	if second.KID != rotation.KID {
		t.Fatalf("second kid = %q, want rotated %q", second.KID, rotation.KID)
	}
	if _, err := verifier.VerifyToken(ctx, first.AccessToken); err != nil {
		t.Fatalf("old token should stay readable inside retention window: %v", err)
	}
	secondProfile, err := verifier.VerifyToken(ctx, second.AccessToken)
	if err != nil {
		t.Fatalf("VerifyToken second error = %v", err)
	}
	if secondProfile.Scope != "orders:read orders:write" {
		t.Fatalf("scope = %q", secondProfile.Scope)
	}
}

func TestServiceRejectsUnknownKIDAndExpiredToken(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	now := fixedNow()
	service, err := NewService(ctx, client, testOptions("shop-auth", &now, "main")...)
	if err != nil {
		t.Fatalf("NewService main error = %v", err)
	}
	foreign, err := NewService(ctx, client, testOptions("foreign-auth", &now, "foreign")...)
	if err != nil {
		t.Fatalf("NewService foreign error = %v", err)
	}

	unknown, err := foreign.IssueToken(ctx, IssueRequest{Subject: "customer-99", Scopes: []string{"orders:read"}, TTLSeconds: 600})
	if err != nil {
		t.Fatalf("IssueToken unknown error = %v", err)
	}
	if _, err := service.VerifyToken(ctx, unknown.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("unknown kid error = %v, want ErrInvalidToken", err)
	}

	expired, err := service.IssueToken(ctx, IssueRequest{Subject: "customer-42", Scopes: []string{"orders:read"}, TTLSeconds: 60})
	if err != nil {
		t.Fatalf("IssueToken expired setup error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := service.VerifyToken(ctx, expired.AccessToken); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expired token error = %v, want ErrExpiredToken", err)
	}
}

func TestServiceHonorsCancelledContextBeforeRepositoryIO(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	now := fixedNow()
	service, err := NewService(ctx, client, testOptions("shop-auth", &now, "cancel")...)
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	if _, err := service.IssueToken(cancelled, IssueRequest{Subject: "customer-42", Scopes: []string{"orders:read"}, TTLSeconds: 60}); !errors.Is(err, context.Canceled) {
		t.Fatalf("IssueToken cancelled error = %v, want context.Canceled", err)
	}
	if _, err := service.RotateKey(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("RotateKey cancelled error = %v, want context.Canceled", err)
	}
}

func TestRouterUsesCachedProviderWithKeyRevalidation(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	now := fixedNow()
	service, err := NewService(ctx, client, testOptions("shop-auth", &now, "http")...)
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}
	router := NewRouter(service)

	token := issueHTTPToken(t, router)
	first := getProfile(t, router, token.AccessToken)
	second := getProfile(t, router, token.AccessToken)
	if first.KID != second.KID || first.Subject != second.Subject {
		t.Fatalf("profiles should be stable across warm cached verification: first=%+v second=%+v", first, second)
	}

	if _, err := service.RotateKey(ctx); err != nil {
		t.Fatalf("RotateKey error = %v", err)
	}
	afterRotate := getProfile(t, router, token.AccessToken)
	if afterRotate.KID != first.KID {
		t.Fatalf("warm hit should revalidate retained key, kid = %q, want %q", afterRotate.KID, first.KID)
	}
}

func newRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := redistestcontainer.Start(context.Background(), t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func testOptions(namespace string, now *time.Time, nodeID string) []Option {
	return []Option{
		WithNamespace(namespace),
		WithClock(func() time.Time { return *now }),
		WithKeyTTL(time.Hour),
		WithOperationTimeout(time.Second),
		WithNodeID(nodeID),
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
}

func issueHTTPToken(t *testing.T, router http.Handler) TokenResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/tokens", jsonBody(t, IssueRequest{
		Subject:    "customer-42",
		Scopes:     []string{"orders:read"},
		TTLSeconds: 300,
	}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /tokens status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response TokenResponse
	decodeJSON(t, rec, &response)
	return response
}

func getProfile(t *testing.T, router http.Handler, token string) ProfileResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /profile status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response ProfileResponse
	decodeJSON(t, rec, &response)
	return response
}

func init() {
	gin.SetMode(gin.TestMode)
}

func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
	return bytes.NewReader(body.Bytes())
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(target); err != nil {
		t.Fatalf("decode JSON %q: %v", rec.Body.String(), err)
	}
}
