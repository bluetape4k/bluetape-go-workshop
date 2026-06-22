# Issue #77 Design: Multi-Currency Invoice Rule Evaluation Example

## Frame

- Issue: #77 `[v0.6.0] Add multi currency invoice rule evaluation example`
- Milestone: `0.6.0`
- Branch/worktree: `feat/issue-77-multi-currency-invoice` under
  `.worktrees/feat-issue-77-multi-currency-invoice`.
- Base example: #45 `examples/money-rule-pricing` teaches single-currency cart
  pricing with `money.Money` and local rule decisions.

## Goal

Create `examples/multi-currency-invoice-rules`, a runnable Gin example that
evaluates invoice lines grouped by currency, applies explicit discount and
tax-like rules without exchange-rate conversion, and exposes rounded totals per
currency.

This example teaches invoice-level multi-currency semantics. It is not a
general rule engine, tax engine, exchange-rate service, accounting ledger, or
invoice persistence layer.

## Evidence

- `gh issue view 77` requires a runnable example, money utilities, rule-engine
  style invoice decisions, visible rounding, invalid currency rejection, and
  README navigation that links #45 as the base money/rule pricing example.
- `examples/money-rule-pricing` already proves local rule decisions with
  `accepted`, `rejected`, and `skipped` statuses.
- `github.com/bluetape4k/bluetape-go/money` provides explicit ISO 4217
  currencies, `Money.Round()` with currency scale, and same-currency
  arithmetic. There is no dedicated reusable rules package in the current
  module surface, so rules stay example-local.

## HTTP Contract

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/invoices/evaluate` | Evaluate one invoice and return grouped currency totals plus rule decisions. |

## Request Contract

```json
{
  "invoice_id": "inv-1001",
  "customer_tier": "vip",
  "region": "EU",
  "lines": [
    {
      "line_id": "svc-1",
      "description": "Implementation workshop",
      "amount": "19.995",
      "currency": "USD",
      "quantity": 2,
      "category": "service"
    }
  ]
}
```

## Rule Contract

- `vip-service-discount`
  - Applies a 5% discount to service lines when `customer_tier = vip`.
  - Skips non-service lines and non-VIP customers with explicit reasons.
- `regional-vat`
  - Applies a 20% tax-like adjustment to taxable lines when `region = EU`.
  - Computes tax on the line total after the line's discount.
  - Skips `tax_exempt` lines and non-EU regions with explicit reasons.
- Rules never combine values across currencies. Each rule decision carries a
  currency and an optional rounded amount.

## Money Contract

- Parse every line currency with `money.ParseCurrency`.
- Parse every line amount with `money.New`.
- Round each line total, discount, tax, and final currency total with
  `Money.Round()`.
- Group totals by currency code. Do not convert or merge currencies.
- Reject `XXX`, empty, or otherwise invalid currency input as `invalid_money`.

## Error Contract

Public responses use this shape:

```json
{"error_code":"invalid_money","message":"invalid money input"}
```

Allowed public codes:

- `invalid_request` (`400`)
- `invalid_money` (`400`)
- `invoice_error` (`500`)

Responses must not echo raw invalid amount or currency parser diagnostics.

## Non-Goals

- No exchange rates or cross-currency settlement.
- No reusable rule framework.
- No persistent invoices, payment capture, accounting export, or locale tax
  compliance.
- No new dependencies beyond existing Gin and `bluetape-go/money`.

## Documentation Requirements

- Add English/Korean README pair under
  `examples/multi-currency-invoice-rules`.
- Root English/Korean README tables include the example.
- Root run section links #45 as the base money/rule pricing example and states
  that #77 extends it to multi-currency invoices without conversion.
- Add `docs/lessons/2026-06-22-multi-currency-invoice-rules.md`.

## Test Requirements

- Currency-specific rounding is visible, including a zero-minor-unit currency.
- Discount eligibility applies only to VIP service lines.
- Invalid currency input is rejected as `ErrInvalidMoney` and public
  `invalid_money`.
- HTTP success and error paths map to stable public response shapes.
- `main.go` keeps loopback binding and bounded server timeouts.

## Completion Criteria

- Issue #77 acceptance criteria are implemented.
- PR body closes #77 and ends with `## DoD Status`.
- PR metadata mirrors the issue: assignee `debop`, milestone `0.6.0`, labels
  `enhancement` and `examples`.
