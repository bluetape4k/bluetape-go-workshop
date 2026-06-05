// Package catalogrefresh demonstrates retry and timeout policies for SKU refresh.
package catalogrefresh

import (
	"context"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

// Product is the refreshed catalog read model used by the storefront.
type Product struct {
	SKU        string
	Name       string
	PriceCents int
	Inventory  int
}

// Source loads the latest product state from an upstream catalog provider.
type Source func(context.Context, string) (Product, error)

// Options configures the catalog refresh policies.
type Options struct {
	MaxAttempts int
	Timeout     time.Duration
	OnEvent     resilience.EventHandler
}

// Refresher protects one SKU refresh with retry and per-attempt timeout.
type Refresher struct {
	retry   *resilience.RetryPolicy[Product]
	timeout *resilience.TimeoutPolicy[Product]
}

// New creates a catalog refresher.
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

// Refresh loads one SKU and returns the refreshed read model.
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
