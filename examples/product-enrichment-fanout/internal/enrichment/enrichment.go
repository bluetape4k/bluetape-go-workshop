// Package enrichment demonstrates context-aware product fan-out.
package enrichment

import (
	"context"
	"fmt"
	"sync"

	"github.com/bluetape4k/bluetape-go/concurrency"
)

// Product is the enriched product detail view.
type Product struct {
	ID              string
	PriceCents      int
	Inventory       int
	Recommendations []string
	ReviewSummary   string
	OptionalErrors  []error
}

// Providers groups downstream product enrichment functions.
type Providers struct {
	Price           func(context.Context, string) (int, error)
	Inventory       func(context.Context, string) (int, error)
	Recommendations func(context.Context, string) ([]string, error)
	ReviewSummary   func(context.Context, string) (string, error)
}

// Enrich builds a product view from required and optional providers.
func Enrich(ctx context.Context, id string, providers Providers) (Product, error) {
	result := Product{ID: id}
	group := concurrency.NewGroup(ctx)
	group.SetLimit(2)

	var mu sync.Mutex
	var optional []error

	group.Go(func(ctx context.Context) error {
		price, err := providers.Price(ctx, id)
		if err != nil {
			return fmt.Errorf("price: %w", err)
		}
		mu.Lock()
		result.PriceCents = price
		mu.Unlock()
		return nil
	})
	group.Go(func(ctx context.Context) error {
		inventory, err := providers.Inventory(ctx, id)
		if err != nil {
			return fmt.Errorf("inventory: %w", err)
		}
		mu.Lock()
		result.Inventory = inventory
		mu.Unlock()
		return nil
	})
	group.Go(func(ctx context.Context) error {
		recommendations, err := providers.Recommendations(ctx, id)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			optional = append(optional, fmt.Errorf("recommendations: %w", err))
			return nil
		}
		result.Recommendations = recommendations
		return nil
	})
	group.Go(func(ctx context.Context) error {
		summary, err := providers.ReviewSummary(ctx, id)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			optional = append(optional, fmt.Errorf("review_summary: %w", err))
			return nil
		}
		result.ReviewSummary = summary
		return nil
	})

	if err := group.Wait(); err != nil {
		return Product{}, err
	}
	result.OptionalErrors = optional
	return result, nil
}
