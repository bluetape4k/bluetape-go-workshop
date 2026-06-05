package paymentguard_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/payment-authorization-guard/internal/paymentguard"
	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestAuthorizeApprovesPaymentWithDefaults(t *testing.T) {
	authorizer, err := paymentguard.New(paymentguard.Options{})
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	authorization, err := authorizer.Authorize(context.Background(), validRequest(), func(_ context.Context, request paymentguard.Request) (paymentguard.Authorization, error) {
		return paymentguard.Authorization{
			OrderID:     request.OrderID,
			Approved:    true,
			ProviderRef: "auth-1001",
		}, nil
	})
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if !authorization.Approved {
		t.Fatalf("authorization = %+v, want approved", authorization)
	}
	if authorization.OrderID != "order-1001" || authorization.ProviderRef != "auth-1001" {
		t.Fatalf("authorization = %+v", authorization)
	}
}

func TestAuthorizeRepeatedFailuresOpenCircuitBeforeGateway(t *testing.T) {
	gatewayErr := errors.New("processor unavailable")
	var events eventLog
	calls := 0

	authorizer, err := paymentguard.New(paymentguard.Options{
		FailureThreshold: 2,
		OpenTimeout:      time.Minute,
		OnEvent:          events.record,
	})
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	gateway := func(context.Context, paymentguard.Request) (paymentguard.Authorization, error) {
		calls++
		return paymentguard.Authorization{}, gatewayErr
	}

	for attempt := 0; attempt < 2; attempt++ {
		_, err := authorizer.Authorize(context.Background(), validRequest(), gateway)
		if !errors.Is(err, gatewayErr) {
			t.Fatalf("attempt %d err = %v, want gateway error", attempt+1, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}

	_, err = authorizer.Authorize(context.Background(), validRequest(), gateway)
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", err)
	}
	if calls != 2 {
		t.Fatalf("open circuit called gateway; calls = %d, want 2", calls)
	}

	snapshot := events.snapshot()
	if !hasEvent(snapshot, resilience.PolicyTypeCircuitBreaker, resilience.EventCircuitStateTransition) {
		t.Fatalf("events = %+v, want circuit transition", snapshot)
	}
	if !hasEvent(snapshot, resilience.PolicyTypeCircuitBreaker, resilience.EventCircuitRejected) {
		t.Fatalf("events = %+v, want circuit rejection", snapshot)
	}
}

func TestAuthorizeBulkheadOverflowRejectsConcurrentCall(t *testing.T) {
	var events eventLog
	authorizer, err := paymentguard.New(paymentguard.Options{
		MaxConcurrent: 1,
		OnEvent:       events.record,
	})
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := authorizer.Authorize(context.Background(), validRequest(), func(context.Context, paymentguard.Request) (paymentguard.Authorization, error) {
			close(entered)
			<-release
			return paymentguard.Authorization{Approved: true, ProviderRef: "auth-held"}, nil
		})
		done <- err
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first gateway call did not enter")
	}

	secondCalls := 0
	_, err = authorizer.Authorize(context.Background(), validRequest(), func(context.Context, paymentguard.Request) (paymentguard.Authorization, error) {
		secondCalls++
		return paymentguard.Authorization{}, nil
	})
	if !errors.Is(err, resilience.ErrBulkheadRejected) {
		t.Fatalf("err = %v, want ErrBulkheadRejected", err)
	}
	if secondCalls != 0 {
		t.Fatalf("overflow called second gateway; calls = %d, want 0", secondCalls)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first authorize: %v", err)
	}

	snapshot := events.snapshot()
	if !hasEvent(snapshot, resilience.PolicyTypeBulkhead, resilience.EventBulkheadAccepted) {
		t.Fatalf("events = %+v, want bulkhead admission", snapshot)
	}
	if !hasEvent(snapshot, resilience.PolicyTypeBulkhead, resilience.EventBulkheadRejected) {
		t.Fatalf("events = %+v, want bulkhead rejection", snapshot)
	}
}

func TestAuthorizeRejectsInvalidInput(t *testing.T) {
	authorizer, err := paymentguard.New(paymentguard.Options{})
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	gateway := func(context.Context, paymentguard.Request) (paymentguard.Authorization, error) {
		return paymentguard.Authorization{}, nil
	}

	tests := []struct {
		name    string
		request paymentguard.Request
	}{
		{name: "blank merchant", request: paymentguard.Request{OrderID: "order-1001", AmountCents: 4900}},
		{name: "blank order", request: paymentguard.Request{MerchantID: "merchant-7", AmountCents: 4900}},
		{name: "zero amount", request: paymentguard.Request{MerchantID: "merchant-7", OrderID: "order-1001"}},
		{name: "negative amount", request: paymentguard.Request{MerchantID: "merchant-7", OrderID: "order-1001", AmountCents: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := authorizer.Authorize(context.Background(), tt.request, gateway); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	if _, err := authorizer.Authorize(context.Background(), validRequest(), nil); err == nil {
		t.Fatal("expected nil gateway error")
	}

	var nilAuthorizer *paymentguard.Authorizer
	if _, err := nilAuthorizer.Authorize(context.Background(), validRequest(), gateway); err == nil {
		t.Fatal("expected nil authorizer error")
	}
}

func TestNewRejectsNegativeOptions(t *testing.T) {
	tests := []struct {
		name    string
		options paymentguard.Options
	}{
		{name: "failure threshold", options: paymentguard.Options{FailureThreshold: -1}},
		{name: "open timeout", options: paymentguard.Options{OpenTimeout: -time.Millisecond}},
		{name: "max concurrent", options: paymentguard.Options{MaxConcurrent: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := paymentguard.New(tt.options); err == nil {
				t.Fatal("expected option validation error")
			}
		})
	}
}

func validRequest() paymentguard.Request {
	return paymentguard.Request{
		MerchantID:  "merchant-7",
		OrderID:     "order-1001",
		AmountCents: 4900,
	}
}

type eventLog struct {
	mu     sync.Mutex
	events []resilience.Event
}

func (l *eventLog) record(_ context.Context, event resilience.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) snapshot() []resilience.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]resilience.Event(nil), l.events...)
}

func hasEvent(events []resilience.Event, policyType string, kind resilience.EventKind) bool {
	for _, event := range events {
		if event.PolicyType == policyType && event.Kind == kind {
			return true
		}
	}
	return false
}
