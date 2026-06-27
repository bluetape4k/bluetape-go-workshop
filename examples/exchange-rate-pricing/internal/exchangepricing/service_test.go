package exchangepricing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/money"
)

func TestServiceQuotesDisplayTotalWithFreshProviderRate(t *testing.T) {
	provider := newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil)
	service := NewService(provider, WithOperationTimeout(200*time.Millisecond))

	quote, err := service.Quote(context.Background(), QuoteRequest{
		QuoteID:      "quote-1001",
		BaseCurrency: "USD",
		Locale:       "ko-KR",
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "19.995", Currency: "USD", Quantity: 2},
			{SKU: "support", UnitPrice: "5.00", Currency: "USD", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMoney(t, quote.Subtotal, "USD", "44.99")
	assertMoney(t, quote.DisplayTotal, "KRW", "58487")
	if !quote.ConversionApplied {
		t.Fatal("ConversionApplied = false, want true")
	}
	if quote.Rate.Source != money.ECBSource || quote.Rate.Rate != "1300" || quote.Rate.Stale {
		t.Fatalf("unexpected rate metadata: %+v", quote.Rate)
	}
	if !provider.sawDeadline {
		t.Fatal("provider did not see a context deadline")
	}
}

func TestServiceAllowsStaleQuoteWhenCallerOptsIn(t *testing.T) {
	cause := errors.New("upstream timed out")
	provider := newScriptedProvider(t, money.USD, money.KRW, "1295", true, cause)
	service := NewService(provider)

	quote, err := service.Quote(context.Background(), QuoteRequest{
		QuoteID:         "quote-1002",
		BaseCurrency:    "USD",
		Locale:          "ko-KR",
		AllowStaleQuote: true,
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "10.00", Currency: "USD", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMoney(t, quote.DisplayTotal, "KRW", "12950")
	if !quote.Rate.Stale {
		t.Fatalf("Rate.Stale = false, want true: %+v", quote.Rate)
	}
	if quote.Rate.RefreshError == nil || *quote.Rate.RefreshError != "upstream timed out" {
		t.Fatalf("RefreshError = %v, want sanitized stale cause", quote.Rate.RefreshError)
	}
}

func TestServiceRejectsStaleQuoteByDefault(t *testing.T) {
	provider := newScriptedProvider(t, money.USD, money.KRW, "1295", true, errors.New("provider down"))
	service := NewService(provider)

	_, err := service.Quote(context.Background(), QuoteRequest{
		QuoteID:      "quote-1003",
		BaseCurrency: "USD",
		Locale:       "ko-KR",
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "10.00", Currency: "USD", Quantity: 1},
		},
	})

	if !errors.Is(err, ErrStaleQuote) {
		t.Fatalf("Quote() error = %v, want ErrStaleQuote", err)
	}
}

func TestServiceMapsProviderRefreshFailure(t *testing.T) {
	provider := &scriptedProvider{err: errors.Join(money.ErrExchangeRateProvider, errors.New("network unavailable"))}
	service := NewService(provider)

	_, err := service.Quote(context.Background(), QuoteRequest{
		QuoteID:      "quote-1004",
		BaseCurrency: "USD",
		Locale:       "ko-KR",
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "10.00", Currency: "USD", Quantity: 1},
		},
	})

	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("Quote() error = %v, want ErrProviderUnavailable", err)
	}
}

func TestServiceRejectsUnsupportedLocale(t *testing.T) {
	service := NewService(newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil))

	_, err := service.Quote(context.Background(), QuoteRequest{
		QuoteID:      "quote-1005",
		BaseCurrency: "USD",
		Locale:       "en",
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "10.00", Currency: "USD", Quantity: 1},
		},
	})

	if !errors.Is(err, ErrInvalidLocale) {
		t.Fatalf("Quote() error = %v, want ErrInvalidLocale", err)
	}
}

func TestServiceRejectsMixedLineCurrencyBeforeProviderIO(t *testing.T) {
	provider := newScriptedProvider(t, money.USD, money.KRW, "1300", false, nil)
	service := NewService(provider)

	_, err := service.Quote(context.Background(), QuoteRequest{
		QuoteID:      "quote-1006",
		BaseCurrency: "USD",
		Locale:       "ko-KR",
		Items: []LineItemRequest{
			{SKU: "pro-plan", UnitPrice: "10.00", Currency: "EUR", Quantity: 1},
		},
	})

	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Quote() error = %v, want ErrCurrencyMismatch", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0 before currency mismatch rejection", provider.calls)
	}
}

type scriptedProvider struct {
	quote       money.ExchangeRateQuote
	err         error
	calls       int
	sawDeadline bool
}

func newScriptedProvider(t *testing.T, base, target money.Currency, rate string, stale bool, refreshErr error) *scriptedProvider {
	t.Helper()
	exchangeRate, err := money.NewExchangeRate(base, target, rate)
	if err != nil {
		t.Fatalf("NewExchangeRate(%s,%s,%s) error = %v", base, target, rate, err)
	}
	observedAt := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	return &scriptedProvider{
		quote: money.ExchangeRateQuote{
			Rate:         exchangeRate,
			Source:       money.ECBSource,
			ObservedAt:   observedAt,
			FetchedAt:    observedAt.Add(2 * time.Hour),
			ExpiresAt:    observedAt.Add(26 * time.Hour),
			Stale:        stale,
			RefreshError: refreshErr,
		},
	}
}

func (p *scriptedProvider) Rate(ctx context.Context, _ money.Currency, _ money.Currency) (money.ExchangeRateQuote, error) {
	p.calls++
	_, p.sawDeadline = ctx.Deadline()
	if err := ctx.Err(); err != nil {
		return money.ExchangeRateQuote{}, err
	}
	if p.err != nil {
		return money.ExchangeRateQuote{}, p.err
	}
	return p.quote, nil
}

func assertMoney(t *testing.T, got MoneyValue, wantCurrency, wantAmount string) {
	t.Helper()
	if got.Currency != wantCurrency || got.Amount != wantAmount {
		t.Fatalf("money = %+v, want %s %s", got, wantCurrency, wantAmount)
	}
}
