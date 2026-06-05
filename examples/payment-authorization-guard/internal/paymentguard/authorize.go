// Package paymentguard demonstrates circuit breaker and bulkhead protection for
// a payment authorization gateway.
package paymentguard

import (
	"context"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

const (
	defaultFailureThreshold = 2
	defaultOpenTimeout      = 250 * time.Millisecond
	defaultMaxConcurrent    = 1
)

// Request describes the non-sensitive order metadata needed for authorization.
type Request struct {
	MerchantID  string
	OrderID     string
	AmountCents int
}

// Authorization is the gateway response returned to the order workflow.
type Authorization struct {
	OrderID     string
	Approved    bool
	ProviderRef string
}

// Gateway authorizes a payment request with an upstream payment provider.
type Gateway func(context.Context, Request) (Authorization, error)

// Options configures the payment authorization guard policies.
type Options struct {
	FailureThreshold int
	OpenTimeout      time.Duration
	MaxConcurrent    int
	OnEvent          resilience.EventHandler
	Now              func() time.Time
}

// Authorizer protects payment authorization calls with a circuit breaker and a
// bulkhead.
type Authorizer struct {
	breaker  *resilience.CircuitBreakerPolicy[Authorization]
	bulkhead *resilience.BulkheadPolicy[Authorization]
}

// New creates a payment authorizer with zero-value friendly defaults.
func New(options Options) (*Authorizer, error) {
	failureThreshold := options.FailureThreshold
	if failureThreshold == 0 {
		failureThreshold = defaultFailureThreshold
	}
	if failureThreshold < 0 {
		return nil, fmt.Errorf("failure threshold must not be negative")
	}

	openTimeout := options.OpenTimeout
	if openTimeout == 0 {
		openTimeout = defaultOpenTimeout
	}
	if openTimeout < 0 {
		return nil, fmt.Errorf("open timeout must not be negative")
	}

	maxConcurrent := options.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	if maxConcurrent < 0 {
		return nil, fmt.Errorf("max concurrent must not be negative")
	}

	breaker, err := resilience.NewCircuitBreaker[Authorization](resilience.CircuitBreakerOptions{
		Name:             "payment-authorization-breaker",
		FailureThreshold: failureThreshold,
		OpenTimeout:      openTimeout,
		Now:              options.Now,
		OnEvent:          options.OnEvent,
	})
	if err != nil {
		return nil, err
	}

	bulkhead, err := resilience.NewBulkhead[Authorization](resilience.BulkheadOptions{
		Name:          "payment-authorization-bulkhead",
		MaxConcurrent: maxConcurrent,
		Wait:          false,
		OnEvent:       options.OnEvent,
	})
	if err != nil {
		return nil, err
	}

	return &Authorizer{breaker: breaker, bulkhead: bulkhead}, nil
}

// Authorize sends one payment authorization request through the guard.
func (a *Authorizer) Authorize(ctx context.Context, request Request, gateway Gateway) (Authorization, error) {
	if a == nil {
		return Authorization{}, fmt.Errorf("authorizer must not be nil")
	}
	if request.MerchantID == "" {
		return Authorization{}, fmt.Errorf("merchant id must not be empty")
	}
	if request.OrderID == "" {
		return Authorization{}, fmt.Errorf("order id must not be empty")
	}
	if request.AmountCents <= 0 {
		return Authorization{}, fmt.Errorf("amount cents must be positive")
	}
	if gateway == nil {
		return Authorization{}, fmt.Errorf("gateway must not be nil")
	}

	return resilience.Run(ctx, func(ctx context.Context) (Authorization, error) {
		authorization, err := gateway(ctx, request)
		if err != nil {
			return Authorization{}, fmt.Errorf("authorize payment %s: %w", request.OrderID, err)
		}
		if authorization.OrderID == "" {
			authorization.OrderID = request.OrderID
		}
		return authorization, nil
	}, a.breaker, a.bulkhead)
}
