// Package paymentguard 는 결제 승인 게이트웨이에 circuit breaker 와 bulkhead 보호를 적용하는 방법을 보여준다.
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

// Request 는 승인에 필요한 비민감 주문 메타데이터를 설명한다.
type Request struct {
	MerchantID  string
	OrderID     string
	AmountCents int
}

// Authorization 은 주문 워크플로로 반환되는 게이트웨이 응답이다.
type Authorization struct {
	OrderID     string
	Approved    bool
	ProviderRef string
}

// Gateway 는 업스트림 결제 provider 로 결제 요청을 승인한다.
type Gateway func(context.Context, Request) (Authorization, error)

// Options 는 결제 승인 보호 정책을 설정한다.
type Options struct {
	FailureThreshold int
	OpenTimeout      time.Duration
	MaxConcurrent    int
	OnEvent          resilience.EventHandler
	Now              func() time.Time
}

// Authorizer 는 circuit breaker 와 bulkhead 로 결제 승인 호출을 보호한다.
type Authorizer struct {
	breaker  *resilience.CircuitBreakerPolicy[Authorization]
	bulkhead *resilience.BulkheadPolicy[Authorization]
}

// New 는 zero-value 친화적인 기본값으로 결제 승인기를 생성한다.
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

// Authorize 는 하나의 결제 승인 요청을 보호 계층을 통해 전송한다.
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
