package invoicerules

import (
	"errors"
	"testing"
)

func TestServiceEvaluatesMultiCurrencyInvoiceWithRules(t *testing.T) {
	service := NewService()

	invoice, err := service.Evaluate(InvoiceRequest{
		InvoiceID:    "inv-1001",
		CustomerTier: "vip",
		Region:       "EU",
		Lines: []LineItemRequest{
			{
				LineID:      "svc-usd",
				Description: "implementation workshop",
				Amount:      "19.995",
				Currency:    "USD",
				Quantity:    2,
				Category:    "service",
			},
			{
				LineID:      "goods-eur",
				Description: "reference kit",
				Amount:      "10.00",
				Currency:    "EUR",
				Quantity:    1,
				Category:    "goods",
			},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if invoice.InvoiceID != "inv-1001" {
		t.Fatalf("invoice_id = %q", invoice.InvoiceID)
	}
	if len(invoice.TotalsByCurrency) != 2 {
		t.Fatalf("totals = %d, want 2: %+v", len(invoice.TotalsByCurrency), invoice.TotalsByCurrency)
	}
	eur := invoice.TotalsByCurrency[0]
	usd := invoice.TotalsByCurrency[1]
	assertCurrencyTotal(t, eur, "EUR", "10.00", "0.00", "2.00", "12.00")
	assertCurrencyTotal(t, usd, "USD", "39.99", "2.00", "7.60", "45.59")

	assertRule(t, invoice.Rules, "vip-service-discount", "svc-usd", "USD", RuleAccepted, "2.00", "")
	assertRule(t, invoice.Rules, "regional-vat", "svc-usd", "USD", RuleAccepted, "7.60", "")
	assertRule(t, invoice.Rules, "vip-service-discount", "goods-eur", "EUR", RuleSkipped, "", "category_not_service")
	assertRule(t, invoice.Rules, "regional-vat", "goods-eur", "EUR", RuleAccepted, "2.00", "")
}

func TestServiceRoundsJPYWithoutMinorUnits(t *testing.T) {
	service := NewService()

	invoice, err := service.Evaluate(InvoiceRequest{
		InvoiceID:    "inv-1002",
		CustomerTier: "standard",
		Region:       "US",
		Lines: []LineItemRequest{
			{
				LineID:      "jp-goods",
				Description: "yen-priced kit",
				Amount:      "100.60",
				Currency:    "JPY",
				Quantity:    1,
				Category:    "goods",
			},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if len(invoice.TotalsByCurrency) != 1 {
		t.Fatalf("totals = %d, want 1", len(invoice.TotalsByCurrency))
	}
	assertCurrencyTotal(t, invoice.TotalsByCurrency[0], "JPY", "101", "0", "0", "101")
	assertRule(t, invoice.Rules, "regional-vat", "jp-goods", "JPY", RuleSkipped, "", "region_not_eu")
}

func TestServiceRejectsInvalidCurrencyInput(t *testing.T) {
	service := NewService()

	_, err := service.Evaluate(InvoiceRequest{
		InvoiceID: "inv-1003",
		Lines: []LineItemRequest{
			{
				LineID:   "bad-currency",
				Amount:   "12.00",
				Currency: "XXX",
				Quantity: 1,
				Category: "service",
			},
		},
	})

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("Evaluate() error = %v, want ErrInvalidMoney", err)
	}
}

func TestServiceRejectsInvalidRequest(t *testing.T) {
	service := NewService()

	_, err := service.Evaluate(InvoiceRequest{
		InvoiceID: "inv-1004",
		Lines: []LineItemRequest{
			{LineID: "qty-zero", Amount: "12.00", Currency: "USD", Quantity: 0, Category: "service"},
		},
	})

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Evaluate() error = %v, want ErrInvalidRequest", err)
	}
}

func assertCurrencyTotal(t *testing.T, got CurrencyTotal, currency, subtotal, discount, tax, total string) {
	t.Helper()
	if got.Currency != currency {
		t.Fatalf("currency = %q, want %q", got.Currency, currency)
	}
	assertMoney(t, got.Subtotal, currency, subtotal)
	assertMoney(t, got.DiscountTotal, currency, discount)
	assertMoney(t, got.TaxTotal, currency, tax)
	assertMoney(t, got.Total, currency, total)
}

func assertMoney(t *testing.T, got MoneyValue, currency, amount string) {
	t.Helper()
	if got.Currency != currency || got.Amount != amount {
		t.Fatalf("money = %+v, want %s %s", got, currency, amount)
	}
}

func assertRule(
	t *testing.T,
	rules []RuleDecision,
	name, lineID, currency string,
	status RuleStatus,
	amount, reason string,
) {
	t.Helper()
	for _, rule := range rules {
		if rule.Name != name || rule.LineID != lineID {
			continue
		}
		if rule.Currency != currency {
			t.Fatalf("rule %s/%s currency = %q, want %q", name, lineID, rule.Currency, currency)
		}
		if rule.Status != status {
			t.Fatalf("rule %s/%s status = %s, want %s", name, lineID, rule.Status, status)
		}
		if amount != "" {
			if rule.Amount == nil {
				t.Fatalf("rule %s/%s amount is nil, want %s", name, lineID, amount)
			}
			assertMoney(t, *rule.Amount, currency, amount)
		}
		if reason != "" && rule.Reason != reason {
			t.Fatalf("rule %s/%s reason = %q, want %q", name, lineID, rule.Reason, reason)
		}
		return
	}
	t.Fatalf("rule %s/%s not found in %+v", name, lineID, rules)
}
