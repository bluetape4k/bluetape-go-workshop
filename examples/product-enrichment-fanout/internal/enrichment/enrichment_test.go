package enrichment_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/product-enrichment-fanout/internal/enrichment"
	"github.com/bluetape4k/bluetape-go/concurrency"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestEnrichProductSuccessAndOptionalFailure(t *testing.T) {
	product, err := enrichment.Enrich(context.Background(), "sku-1", enrichment.Providers{
		Price:           func(context.Context, string) (int, error) { return 1299, nil },
		Inventory:       func(context.Context, string) (int, error) { return 7, nil },
		Recommendations: func(context.Context, string) ([]string, error) { return nil, errors.New("optional down") },
		ReviewSummary:   func(context.Context, string) (string, error) { return "4.8/5", nil },
	})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if product.PriceCents != 1299 || product.Inventory != 7 || len(product.OptionalErrors) != 1 {
		t.Fatalf("product = %+v", product)
	}
}

func TestEnrichProductRequiredFailureCancels(t *testing.T) {
	_, err := enrichment.Enrich(context.Background(), "sku-1", enrichment.Providers{
		Price:           func(context.Context, string) (int, error) { return 0, errors.New("pricing down") },
		Inventory:       func(ctx context.Context, _ string) (int, error) { <-ctx.Done(); return 0, ctx.Err() },
		Recommendations: func(context.Context, string) ([]string, error) { return nil, nil },
		ReviewSummary:   func(context.Context, string) (string, error) { return "", nil },
	})
	if err == nil {
		t.Fatal("expected required provider failure")
	}
}

func TestEnrichProductStressUsesGoroutineStressTester(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32
	provider := func(ctx context.Context, _ string) (int, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			observed := maxActive.Load()
			if current <= observed || maxActive.CompareAndSwap(observed, current) {
				break
			}
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Millisecond):
			return 1, nil
		}
	}
	task := func(ctx context.Context) error {
		_, err := enrichment.Enrich(ctx, "sku-1", enrichment.Providers{
			Price:           provider,
			Inventory:       provider,
			Recommendations: func(context.Context, string) ([]string, error) { return []string{"sku-2"}, nil },
			ReviewSummary:   func(context.Context, string) (string, error) { return "ok", nil },
		})
		return err
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 6, RoundsPerTask: 3, Timeout: time.Second})
	report := tester.RunT(t, task, task, task, task, task, task)
	if report.Completed != 18 {
		t.Fatalf("report = %+v", report)
	}
	if maxActive.Load() > 12 {
		t.Fatalf("unexpected global provider concurrency = %d", maxActive.Load())
	}
}

func TestEnrichProductCapturesPanicAsError(t *testing.T) {
	err := <-concurrency.Go(context.Background(), func(context.Context) error {
		panic("provider panic")
	})
	var panicErr concurrency.PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("expected panic error, got %v", err)
	}
}
