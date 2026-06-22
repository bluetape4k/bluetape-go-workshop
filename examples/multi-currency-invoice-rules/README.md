# multi-currency-invoice-rules

[English](README.md) | [한국어](README.ko.md)

Gin invoice evaluation example for `money`.

This example extends the base
[`money-rule-pricing`](../money-rule-pricing/README.md) lesson from #45. It
keeps the same decimal-backed money contract, then evaluates invoice lines by
currency so discounts, tax-like adjustments, and rounding decisions stay
explicit.

## Scenario

An invoice service receives line items in multiple currencies. The service
rounds every line using the line currency, applies a VIP service discount,
applies an illustrative EU regional VAT adjustment, and returns totals grouped
by currency. No exchange-rate conversion is performed.

Rules are application-local decision functions. This example does not build a
generic rule engine.

## What It Demonstrates

- Multi-currency invoice totals without cross-currency arithmetic.
- `money.Money` parsing, same-currency arithmetic, and currency-scale rounding.
- Zero-minor-unit rounding for currencies such as JPY.
- Rule decisions that carry line, currency, status, optional amount, and reason.
- Stable public HTTP errors for invalid request and invalid money input.

## Run

```bash
go run ./examples/multi-currency-invoice-rules
```

The service listens on `127.0.0.1:8100` by default.

```bash
curl http://127.0.0.1:8100/healthz
```

Evaluate an invoice:

```bash
curl -s -X POST http://127.0.0.1:8100/invoices/evaluate \
  -H 'Content-Type: application/json' \
  -d '{
    "invoice_id":"inv-1001",
    "customer_tier":"vip",
    "region":"EU",
    "lines":[
      {
        "line_id":"svc-usd",
        "description":"implementation workshop",
        "amount":"19.995",
        "currency":"USD",
        "quantity":2,
        "category":"service"
      },
      {
        "line_id":"goods-eur",
        "description":"reference kit",
        "amount":"10.00",
        "currency":"EUR",
        "quantity":1,
        "category":"goods"
      },
      {
        "line_id":"goods-jpy",
        "description":"yen-priced kit",
        "amount":"100.60",
        "currency":"JPY",
        "quantity":1,
        "category":"tax_exempt"
      }
    ]
  }' | jq
```

Expected grouped totals include:

- `USD`: subtotal `39.99`, discount `2.00`, tax `7.60`, total `45.59`
- `EUR`: subtotal `10.00`, discount `0.00`, tax `2.00`, total `12.00`
- `JPY`: subtotal `101`, discount `0`, tax `0`, total `101`

Try invalid currency input:

```bash
curl -s -X POST http://127.0.0.1:8100/invoices/evaluate \
  -H 'Content-Type: application/json' \
  -d '{
    "invoice_id":"inv-1002",
    "lines":[
      {"line_id":"bad-currency","amount":"12.00","currency":"XXX","quantity":1,"category":"service"}
    ]
  }'
```

Expected public error code: `invalid_money`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/invoices/evaluate` | Evaluate one invoice and return grouped currency totals. |

## Boundary Notes

- This example does not convert currencies. Totals are grouped by currency.
- `regional-vat` is an illustrative tax-like adjustment, not compliance logic.
- Rules are local functions, not a reusable rule framework.
- Public error responses do not echo raw invalid amounts or currency parser
  diagnostics.

## Test

```bash
go test -count=1 ./examples/multi-currency-invoice-rules/...
go test -race -count=1 ./examples/multi-currency-invoice-rules/...
```
