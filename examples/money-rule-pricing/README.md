# money-rule-pricing

[English](README.md) | [한국어](README.ko.md)

Gin cart pricing example for `money`.

This example prices a cart with decimal-backed `money.Money` values and small
application-local pricing rules. It demonstrates rounded line totals, currency
mismatch rejection, accepted discounts, and rejected coupon rules.

## Scenario

A checkout service receives a cart quote request. The service validates that all
line items use the same currency, calculates rounded line totals, applies a VIP
percentage discount and a fixed coupon discount, and returns rule decisions with
the final total.

The example intentionally keeps rules local to the application. It does not try
to build a reusable rule engine.

## What It Demonstrates

- Decimal-backed monetary arithmetic from `bluetape-go/money`.
- String amount inputs and outputs instead of `float64`.
- Currency mismatch rejection before arithmetic.
- Rounded line totals and rounded discount totals at the cart boundary.
- Deterministic accepted and rejected pricing rule decisions.
- Stable public HTTP errors for invalid request, invalid money, and mixed
  currencies.

## Why Not `float64`

Money needs exact decimal behavior and explicit currency boundaries. A binary
floating-point value can make values such as `0.1` approximate and can hide
rounding decisions until a late total. This example keeps all public monetary
values as strings and delegates arithmetic to `money.Money`, so rounding and
currency checks happen at named boundaries.

## Run

```bash
go run ./examples/money-rule-pricing
```

The service listens on `127.0.0.1:8098` by default.

```bash
curl http://127.0.0.1:8098/healthz
```

Quote a cart:

```bash
curl -s -X POST http://127.0.0.1:8098/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "cart_id":"cart-1001",
    "currency":"USD",
    "customer_tier":"vip",
    "coupon_code":"SAVE10",
    "items":[
      {"sku":"book-1","unit_price":"19.995","currency":"USD","quantity":2},
      {"sku":"pen-1","unit_price":"2.50","currency":"USD","quantity":1}
    ]
  }' | jq
```

Expected totals include subtotal `USD 42.49`, discount total `USD 14.25`, and
total `USD 28.24`.

Try a rejected rule:

```bash
curl -s -X POST http://127.0.0.1:8098/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "cart_id":"cart-1002",
    "currency":"USD",
    "coupon_code":"SAVE10",
    "items":[{"sku":"clip-1","unit_price":"4.00","currency":"USD","quantity":1}]
  }' | jq '.rules'
```

The `coupon-save10` rule is rejected with `discount_exceeds_subtotal`, and the
total stays unchanged.

Try a currency mismatch:

```bash
curl -s -X POST http://127.0.0.1:8098/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "cart_id":"cart-1003",
    "currency":"USD",
    "items":[{"sku":"coffee-1","unit_price":"5.00","currency":"EUR","quantity":1}]
  }'
```

Expected public error code: `currency_mismatch`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/quotes` | Price one cart quote request. |

## Boundary Notes

- This example does not convert currencies. Mixed currencies are rejected.
- Rules are local functions, not a reusable promotion framework.
- Rejected rules are returned in a successful quote when the cart itself is
  valid.
- Invalid money, invalid JSON, and mixed currencies are HTTP errors.

## Test

```bash
go test -count=1 ./examples/money-rule-pricing/...
go test -race -count=1 ./examples/money-rule-pricing/...
```
