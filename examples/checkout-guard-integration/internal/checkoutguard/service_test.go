package checkoutguard

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestServiceAuthorizesCheckoutAndCalculatesMoney(t *testing.T) {
	service := newTestService(t)
	token := issueTestToken(t, service, "customer", []string{"profile:read", RequiredScope})

	response, err := service.GuardCheckout("Bearer "+token.Token, validCheckoutRequest("idem-1001"))
	if err != nil {
		t.Fatalf("GuardCheckout() error = %v", err)
	}

	if response.CheckoutID != "chk-1001" {
		t.Fatalf("checkout_id = %q", response.CheckoutID)
	}
	if response.RequestID != "req-1001" {
		t.Fatalf("request_id = %q", response.RequestID)
	}
	if response.OrderID != "order-1001" {
		t.Fatalf("order_id = %q", response.OrderID)
	}
	if response.Subject != "customer-1001" {
		t.Fatalf("subject = %q", response.Subject)
	}
	if response.SessionID != "session-1001" {
		t.Fatalf("session_id = %q", response.SessionID)
	}
	if response.Admission.Decision != DecisionAdmit || !response.Admission.Accepted {
		t.Fatalf("admission = %+v, want admitted", response.Admission)
	}
	assertMoney(t, response.Pricing.Subtotal, "USD", "39.99")
	assertMoney(t, response.Pricing.DiscountTotal, "USD", "2.00")
	assertMoney(t, response.Pricing.TaxTotal, "USD", "7.60")
	assertMoney(t, response.Pricing.Total, "USD", "45.59")
	assertRule(t, response.Rules, "vip-service-discount", "svc-1", RuleAccepted, "2.00", "")
	assertRule(t, response.Rules, "regional-vat", "svc-1", RuleAccepted, "7.60", "")
}

func TestServiceRejectsInvalidClaims(t *testing.T) {
	service := newTestService(t)
	token := issueTestToken(t, service, "customer", []string{"profile:read"})

	_, err := service.GuardCheckout("Bearer "+token.Token, validCheckoutRequest("idem-claims"))

	if !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("GuardCheckout() error = %v, want ErrInvalidClaims", err)
	}
}

func TestServiceRejectsRuleDeniedCategory(t *testing.T) {
	service := newTestService(t)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})
	request := validCheckoutRequest("idem-rule-denied")
	request.Items[0].Category = "restricted"

	_, err := service.GuardCheckout("Bearer "+token.Token, request)

	if !errors.Is(err, ErrRuleDenied) {
		t.Fatalf("GuardCheckout() error = %v, want ErrRuleDenied", err)
	}
}

func TestServiceRejectsDuplicateSubmission(t *testing.T) {
	service := newTestService(t)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})

	if _, err := service.GuardCheckout("Bearer "+token.Token, validCheckoutRequest("idem-duplicate")); err != nil {
		t.Fatalf("first GuardCheckout() error = %v", err)
	}
	_, err := service.GuardCheckout("Bearer "+token.Token, validCheckoutRequest("idem-duplicate"))

	if !errors.Is(err, ErrDuplicateSubmission) {
		t.Fatalf("second GuardCheckout() error = %v, want ErrDuplicateSubmission", err)
	}
}

func TestServiceRejectsInvalidMoney(t *testing.T) {
	service := newTestService(t)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})
	request := validCheckoutRequest("idem-money")
	request.Items[0].Currency = "EUR"

	_, err := service.GuardCheckout("Bearer "+token.Token, request)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("GuardCheckout() error = %v, want ErrInvalidMoney", err)
	}
}

func TestServiceAdmitsConcurrentDuplicateOnlyOnce(t *testing.T) {
	service := newTestService(t)
	token := issueTestToken(t, service, "customer", []string{RequiredScope})
	const goroutines = 24

	var wg sync.WaitGroup
	results := make(chan CheckoutResponse, goroutines)
	errorsCh := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := service.GuardCheckout("Bearer "+token.Token, validCheckoutRequest("idem-concurrent"))
			if err != nil {
				errorsCh <- err
				return
			}
			results <- response
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)

	accepted := 0
	duplicates := 0
	for err := range errorsCh {
		if !errors.Is(err, ErrDuplicateSubmission) {
			t.Fatalf("GuardCheckout() error = %v, want duplicate or nil", err)
		}
		duplicates++
	}
	for response := range results {
		if response.Admission.Decision != DecisionAdmit {
			t.Fatalf("unexpected accepted response = %+v", response.Admission)
		}
		accepted++
	}
	if accepted != 1 {
		t.Fatalf("accepted = %d, want 1", accepted)
	}
	if duplicates != goroutines-1 {
		t.Fatalf("duplicates = %d, want %d", duplicates, goroutines-1)
	}
}

func issueTestToken(t *testing.T, service *Service, role string, scopes []string) TokenResponse {
	t.Helper()
	response, err := service.IssueToken(TokenRequest{
		Subject:    "customer-1001",
		Role:       role,
		Scopes:     scopes,
		TTLSeconds: 300,
	})
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	return response
}

func validCheckoutRequest(idempotencyKey string) CheckoutRequest {
	return CheckoutRequest{
		CheckoutID:     "chk-1001",
		IdempotencyKey: idempotencyKey,
		CustomerTier:   "vip",
		Region:         "EU",
		Currency:       "USD",
		Items: []LineItemRequest{
			{
				LineID:    "svc-1",
				SKU:       "support-plan",
				UnitPrice: "19.995",
				Currency:  "USD",
				Quantity:  2,
				Category:  "service",
			},
		},
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService(
		WithClock(func() time.Time { return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC) }),
		WithSecret([]byte("0123456789abcdef0123456789abcdef")),
		WithIDGenerator(newSequenceIDGenerator(
			"session-1001",
			"jti-1001",
			"req-1001",
			"order-1001",
		)),
		WithBloomConfig(100, 0.01),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func assertMoney(t *testing.T, got MoneyValue, currency, amount string) {
	t.Helper()
	if got.Currency != currency || got.Amount != amount {
		t.Fatalf("money = %+v, want %s %s", got, currency, amount)
	}
}

func assertRule(t *testing.T, rules []RuleDecision, name, lineID string, status RuleStatus, amount, reason string) {
	t.Helper()
	for _, rule := range rules {
		if rule.Name != name || rule.LineID != lineID {
			continue
		}
		if rule.Status != status {
			t.Fatalf("rule %s/%s status = %s, want %s", name, lineID, rule.Status, status)
		}
		if amount != "" {
			if rule.Amount == nil {
				t.Fatalf("rule %s/%s amount is nil, want %s", name, lineID, amount)
			}
			assertMoney(t, *rule.Amount, "USD", amount)
		}
		if reason != "" && rule.Reason != reason {
			t.Fatalf("rule %s/%s reason = %q, want %q", name, lineID, rule.Reason, reason)
		}
		return
	}
	t.Fatalf("rule %s/%s not found in %+v", name, lineID, rules)
}

type sequenceIDGenerator struct {
	mu     sync.Mutex
	values []string
	next   int
}

func newSequenceIDGenerator(values ...string) *sequenceIDGenerator {
	return &sequenceIDGenerator{values: values}
}

func (g *sequenceIDGenerator) NextString() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.next >= len(g.values) {
		value := "seq-extra"
		g.next++
		return value, nil
	}
	value := g.values[g.next]
	g.next++
	return value, nil
}
