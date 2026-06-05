package catalogrefresh_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/catalog-refresh-resilience/internal/catalogrefresh"
	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestRefreshRetriesTransientFailure(t *testing.T) {
	var events []resilience.Event
	refresher, err := catalogrefresh.New(catalogrefresh.Options{
		MaxAttempts: 3,
		Timeout:     time.Second,
		OnEvent: func(_ context.Context, event resilience.Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("new refresher: %v", err)
	}

	attempts := 0
	product, err := refresher.Refresh(context.Background(), "sku-1", func(context.Context, string) (catalogrefresh.Product, error) {
		attempts++
		if attempts == 1 {
			return catalogrefresh.Product{}, errors.New("catalog upstream unavailable")
		}
		return catalogrefresh.Product{
			SKU:        "sku-1",
			Name:       "Travel Mug",
			PriceCents: 1299,
			Inventory:  5,
		}, nil
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if product.SKU != "sku-1" || product.PriceCents != 1299 || product.Inventory != 5 {
		t.Fatalf("product = %+v", product)
	}
	if !hasEvent(events, resilience.PolicyTypeRetry, resilience.EventRetry) {
		t.Fatalf("events = %+v, want retry event", events)
	}
	if !hasEvent(events, resilience.PolicyTypeRetry, resilience.EventSuccess) {
		t.Fatalf("events = %+v, want retry success event", events)
	}
}

func TestRefreshReportsPolicyTimeout(t *testing.T) {
	refresher, err := catalogrefresh.New(catalogrefresh.Options{
		MaxAttempts: 1,
		Timeout:     20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new refresher: %v", err)
	}

	_, err = refresher.Refresh(context.Background(), "sku-2", func(ctx context.Context, _ string) (catalogrefresh.Product, error) {
		<-ctx.Done()
		return catalogrefresh.Product{}, ctx.Err()
	})
	if !errors.Is(err, resilience.ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
	if !errors.Is(err, resilience.ErrRetryExhausted) {
		t.Fatalf("err = %v, want retry exhausted wrapper", err)
	}
}

func TestRefreshRejectsInvalidInput(t *testing.T) {
	refresher, err := catalogrefresh.New(catalogrefresh.Options{})
	if err != nil {
		t.Fatalf("new refresher: %v", err)
	}

	if _, err := refresher.Refresh(context.Background(), "", func(context.Context, string) (catalogrefresh.Product, error) {
		return catalogrefresh.Product{}, nil
	}); err == nil {
		t.Fatal("expected blank sku error")
	}
	if _, err := refresher.Refresh(context.Background(), "sku-3", nil); err == nil {
		t.Fatal("expected nil source error")
	}
}

func hasEvent(events []resilience.Event, policyType string, kind resilience.EventKind) bool {
	for _, event := range events {
		if event.PolicyType == policyType && event.Kind == kind {
			return true
		}
	}
	return false
}
