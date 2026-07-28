// Package catalogrefresh는 SKU refresh에 retry와 timeout policy를 적용하는 방식을 보여준다.
package catalogrefresh

import (
	"context"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

// Product는 storefront가 사용하는 refreshed catalog read model이다.
type Product struct {
	SKU        string
	Name       string
	PriceCents int
	Inventory  int
}

// Source는 upstream catalog provider에서 최신 product state를 load한다.
type Source func(context.Context, string) (Product, error)

// Options는 catalog refresh policy를 설정한다.
type Options struct {
	MaxAttempts int
	Timeout     time.Duration
	OnEvent     resilience.EventHandler
}

// Refresher는 retry와 per-attempt timeout으로 SKU refresh 하나를 보호한다.
type Refresher struct {
	retry   *resilience.RetryPolicy[Product]
	timeout *resilience.TimeoutPolicy[Product]
}

// New는 catalog refresher를 만든다.
func New(options Options) (*Refresher, error) {
	attempts := options.MaxAttempts
	if attempts == 0 {
		attempts = 3
	}
	timeout := options.Timeout
	if timeout == 0 {
		timeout = 250 * time.Millisecond
	}

	retry, err := resilience.NewRetry[Product](resilience.RetryOptions{
		Name:        "catalog-refresh-retry",
		MaxAttempts: attempts,
		Backoff:     resilience.NoBackoff(),
		OnEvent:     options.OnEvent,
	})
	if err != nil {
		return nil, err
	}
	deadline, err := resilience.NewTimeout[Product](resilience.TimeoutOptions{
		Name:    "catalog-refresh-timeout",
		Timeout: timeout,
		OnEvent: options.OnEvent,
	})
	if err != nil {
		return nil, err
	}
	return &Refresher{retry: retry, timeout: deadline}, nil
}

// Refresh는 SKU 하나를 load하고 refreshed read model을 반환한다.
func (r *Refresher) Refresh(ctx context.Context, sku string, source Source) (Product, error) {
	if r == nil {
		return Product{}, fmt.Errorf("refresher must not be nil")
	}
	if sku == "" {
		return Product{}, fmt.Errorf("sku must not be empty")
	}
	if source == nil {
		return Product{}, fmt.Errorf("source must not be nil")
	}

	return resilience.Run(ctx, func(ctx context.Context) (Product, error) {
		return source(ctx, sku)
	}, r.retry, r.timeout)
}
