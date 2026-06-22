# Issue #45 Design: Money and Rule Based Pricing Example

## Goal

Add a runnable `examples/money-rule-pricing` Gin API that demonstrates money-safe cart pricing with `github.com/bluetape4k/bluetape-go/money` and small application-local rule primitives.

The example must teach why prices are not represented with `float64`, how currency mismatch is rejected before arithmetic, how rounded totals are produced at the money boundary, and how accepted and rejected pricing rules are reported deterministically.

## Current Evidence

- `bluetape-go` v0.6.2 exposes `github.com/bluetape4k/bluetape-go/money` with `Money`, `Currency`, `New`, `Parse`, `Add`, `Sub`, `Mul`, `Round`, `RoundTo`, `Sum`, and sentinel errors such as `ErrCurrencyMismatch`.
- The current module does not expose a dedicated `rule` or `rules` package. Rule behavior for #45 should therefore stay example-local and small instead of inventing a reusable rules framework in the workshop.
- Recent Gin examples bind HTTP services to loopback by default, expose `/healthz`, cap JSON bodies, and document stable public error codes.

## User Story

As a workshop reader, I can submit a cart quote request with line-item prices, a currency, a customer tier, and an optional coupon. The API returns a subtotal, accepted discounts, rejected rules, and final total using decimal-backed `money.Money` arithmetic.

## Scope

- Add `examples/money-rule-pricing`.
- Add an internal package `examples/money-rule-pricing/internal/moneypricing`.
- Add English and Korean READMEs for the example.
- Add root README entries for the example under the examples table.
- Add a concise lesson note under `docs/lessons`.

## Non-Goals

- Do not add reusable pricing, rule-engine, or promotion framework packages.
- Do not add persistent storage, authentication, or external service calls.
- Do not support cross-currency conversion. Mixed item currencies are rejected.
- Do not use floating point for monetary arithmetic.

## API Contract

### Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/quotes` | Price one cart quote request. |

### Request Shape

```json
{
  "cart_id": "cart-1001",
  "currency": "USD",
  "customer_tier": "vip",
  "coupon_code": "SAVE10",
  "items": [
    {"sku": "book-1", "unit_price": "19.995", "currency": "USD", "quantity": 2}
  ]
}
```

### Response Shape

The success response includes string money fields:

- `subtotal`
- `discount_total`
- `total`
- per-line `line_total`
- `rules[]` with `name`, `status`, optional `amount`, and optional `reason`

Rule status values are `accepted`, `rejected`, and `skipped` when a rule was evaluated but did not apply.

### Public Errors

| Code | HTTP | Meaning |
|---|---:|---|
| `invalid_request` | 400 | Malformed JSON or missing/invalid cart fields. |
| `invalid_money` | 400 | Invalid currency or decimal amount. |
| `currency_mismatch` | 400 | Item currency differs from cart currency. |
| `pricing_error` | 500 | Unexpected pricing failure. |

Error responses must not echo raw parser diagnostics that expose implementation internals.

## Pricing Rules

Rules are deterministic and evaluated in this order:

1. `vip-ten-percent`: accepts a 10 percent subtotal discount for `customer_tier = "vip"`.
2. `coupon-save10`: accepts a fixed 10.00 discount for `coupon_code = "SAVE10"` when subtotal can cover it.
3. Unsupported coupon codes are represented as a rejected `coupon` decision and do not reduce the total.
4. A fixed discount greater than the current subtotal is rejected with `discount_exceeds_subtotal` and does not reduce the total.

Discount totals are rounded to the cart currency scale before subtraction.

## Acceptance Criteria

- `go test -count=1 ./examples/money-rule-pricing/...` passes.
- `go test -race -count=1 ./examples/money-rule-pricing/...` passes.
- `go test -p 1 ./...` passes.
- `make fmt-check`, `make tidy-check`, `make vet`, and `make lint` pass or any pre-existing unrelated failure is documented with evidence.
- Tests cover:
  - rounded line totals and rounded discount totals,
  - rejected mixed currency cart input,
  - valid VIP and coupon discounts,
  - rejected coupon/discount paths,
  - HTTP success and public error mapping.
- README.md and README.ko.md explain why money is not represented with `float64`.
- Root English/Korean READMEs link the new example and list `money` as the bluetape-go package.

## Step 2-R Integrated Review

Native subagent spawning is not available in this Codex surface, so the six review lanes were run as independent main-session checks using the full-feature reference contract.

| Priority | Area | Finding | Required spec edit |
|---|---|---|---|
| P2 | Developer | There is no upstream `rule` package in the current dependency, so the issue title could tempt a workshop-local framework. | Record rule primitives as example-local and non-reusable. Done. |
| P2 | User | Readers need visible rejected-rule behavior, not only validation errors. | Add `rules[]` decisions and rejected coupon/discount acceptance criteria. Done. |
| P2 | Security | HTTP errors should not leak parser internals. | Add public error code contract. Done. |
| P3 | Operator | The example should stay local and deterministic. | Add non-goals for storage, auth, and external calls. Done. |

Final Step 2-R verdict: P0 = 0, P1 = 0.
