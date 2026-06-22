package moneypricing

import (
	"errors"
	"testing"
)

func TestServiceQuotesCartWithRoundedMoneyAndAcceptedDiscounts(t *testing.T) {
	service := NewService()

	quote, err := service.Quote(QuoteRequest{
		CartID:       "cart-1001",
		Currency:     "USD",
		CustomerTier: "vip",
		CouponCode:   "SAVE10",
		Items: []LineItemRequest{
			{SKU: "book-1", UnitPrice: "19.995", Currency: "USD", Quantity: 2},
			{SKU: "pen-1", UnitPrice: "2.50", Currency: "USD", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMoney(t, quote.Subtotal, "USD", "42.49")
	assertMoney(t, quote.DiscountTotal, "USD", "14.25")
	assertMoney(t, quote.Total, "USD", "28.24")

	if len(quote.Items) != 2 {
		t.Fatalf("quote items = %d, want 2", len(quote.Items))
	}
	assertMoney(t, quote.Items[0].LineTotal, "USD", "39.99")

	assertRule(t, quote.Rules, "vip-ten-percent", RuleAccepted, "4.25", "")
	assertRule(t, quote.Rules, "coupon-save10", RuleAccepted, "10.00", "")
}

func TestServiceRejectsMixedCurrencyBeforePricing(t *testing.T) {
	service := NewService()

	_, err := service.Quote(QuoteRequest{
		CartID:   "cart-1002",
		Currency: "USD",
		Items: []LineItemRequest{
			{SKU: "book-1", UnitPrice: "15.00", Currency: "USD", Quantity: 1},
			{SKU: "coffee-1", UnitPrice: "5.00", Currency: "EUR", Quantity: 1},
		},
	})

	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Quote() error = %v, want ErrCurrencyMismatch", err)
	}
}

func TestServiceReportsRejectedRulesWithoutChangingTotal(t *testing.T) {
	service := NewService()

	quote, err := service.Quote(QuoteRequest{
		CartID:     "cart-1003",
		Currency:   "USD",
		CouponCode: "SAVE10",
		Items: []LineItemRequest{
			{SKU: "clip-1", UnitPrice: "4.00", Currency: "USD", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMoney(t, quote.Subtotal, "USD", "4.00")
	assertMoney(t, quote.DiscountTotal, "USD", "0.00")
	assertMoney(t, quote.Total, "USD", "4.00")
	assertRule(t, quote.Rules, "coupon-save10", RuleRejected, "", "discount_exceeds_subtotal")
}

func TestServiceRejectsCouponWhenPriorDiscountConsumesRemainingSubtotal(t *testing.T) {
	service := NewService()

	quote, err := service.Quote(QuoteRequest{
		CartID:       "cart-1004",
		Currency:     "USD",
		CustomerTier: "vip",
		CouponCode:   "SAVE10",
		Items: []LineItemRequest{
			{SKU: "bag-1", UnitPrice: "10.00", Currency: "USD", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMoney(t, quote.Subtotal, "USD", "10.00")
	assertMoney(t, quote.DiscountTotal, "USD", "1.00")
	assertMoney(t, quote.Total, "USD", "9.00")
	assertRule(t, quote.Rules, "vip-ten-percent", RuleAccepted, "1.00", "")
	assertRule(t, quote.Rules, "coupon-save10", RuleRejected, "", "discount_exceeds_subtotal")
}

func TestServiceReportsUnsupportedCoupon(t *testing.T) {
	service := NewService()

	quote, err := service.Quote(QuoteRequest{
		CartID:     "cart-1005",
		Currency:   "USD",
		CouponCode: "BOGUS",
		Items: []LineItemRequest{
			{SKU: "book-1", UnitPrice: "15.00", Currency: "USD", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Quote() error = %v", err)
	}

	assertMoney(t, quote.DiscountTotal, "USD", "0.00")
	assertMoney(t, quote.Total, "USD", "15.00")
	assertRule(t, quote.Rules, "coupon", RuleRejected, "", "unsupported_coupon")
}

func assertMoney(t *testing.T, got MoneyValue, wantCurrency, wantAmount string) {
	t.Helper()
	if got.Currency != wantCurrency || got.Amount != wantAmount {
		t.Fatalf("money = %+v, want %s %s", got, wantCurrency, wantAmount)
	}
}

func assertRule(t *testing.T, rules []RuleDecision, name string, status RuleStatus, amount, reason string) {
	t.Helper()
	for _, rule := range rules {
		if rule.Name != name {
			continue
		}
		if rule.Status != status {
			t.Fatalf("rule %s status = %s, want %s", name, rule.Status, status)
		}
		if amount != "" {
			if rule.Amount == nil {
				t.Fatalf("rule %s amount is nil, want %s", name, amount)
			}
			assertMoney(t, *rule.Amount, "USD", amount)
		}
		if reason != "" && rule.Reason != reason {
			t.Fatalf("rule %s reason = %q, want %q", name, rule.Reason, reason)
		}
		return
	}
	t.Fatalf("rule %s not found in %+v", name, rules)
}
