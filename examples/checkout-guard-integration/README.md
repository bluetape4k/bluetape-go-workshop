# checkout-guard-integration

[English](README.md) | [한국어](README.ko.md)

Gin checkout guard example for `id`, `jwt`, `money`, and `probabilistic`.

This example composes the focused utility lessons from
[`id-jwt-boundary`](../id-jwt-boundary/README.md),
[`token-refresh-claims`](../token-refresh-claims/README.md),
[`money-rule-pricing`](../money-rule-pricing/README.md),
[`multi-currency-invoice-rules`](../multi-currency-invoice-rules/README.md),
and
[`probabilistic-dedupe-admission`](../probabilistic-dedupe-admission/README.md).

## Scenario

A checkout API receives an authenticated submission. The service verifies the
JWT access-token contract, prices the checkout with decimal-backed money,
applies local eligibility rules, checks an idempotency key through a Bloom
filter, and returns generated internal request/order IDs.

The Bloom filter protects the hot path from repeated submissions, but a hit is
only `probably_seen`. Production checkout idempotency still needs a durable
authoritative store.

## What It Demonstrates

- JWT claim checks for issuer, audience, `token_use`, role, scope, and session.
- UUID v7 request and order IDs generated inside the trust boundary.
- Single-currency checkout pricing with rounded subtotal, discount, tax, and
  final total.
- Local rule decisions for VIP service discount, EU tax-like adjustment, and
  restricted category denial.
- Bloom filter admission with `admit` and `probably_seen` paths.
- Stable public HTTP errors that do not leak raw parser diagnostics.

## Run

```bash
go run ./examples/checkout-guard-integration
```

The service listens on `127.0.0.1:8101` by default.

```bash
curl http://127.0.0.1:8101/healthz
```

Issue a local demo access token:

```bash
TOKEN=$(
  curl -s -X POST http://127.0.0.1:8101/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["checkout:submit"],"ttl_seconds":300}' \
  | jq -r '.token'
)
```

Submit a guarded checkout:

```bash
curl -s -X POST http://127.0.0.1:8101/checkout/guard \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "checkout_id":"chk-1001",
    "idempotency_key":"idem-1001",
    "customer_tier":"vip",
    "region":"EU",
    "currency":"USD",
    "items":[
      {
        "line_id":"svc-1",
        "sku":"support-plan",
        "unit_price":"19.995",
        "currency":"USD",
        "quantity":2,
        "category":"service"
      }
    ]
  }' | jq
```

Expected totals include subtotal `39.99`, discount `2.00`, tax `7.60`, and
total `45.59` in `USD`.

Run the same checkout again with the same `idempotency_key` to see the public
`duplicate_submission` error.

Try a rule denial:

```bash
curl -s -X POST http://127.0.0.1:8101/checkout/guard \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"checkout_id":"chk-1002","idempotency_key":"idem-1002","currency":"USD","items":[{"line_id":"restricted","sku":"restricted","unit_price":"12.00","currency":"USD","quantity":1,"category":"restricted"}]}'
```

Expected public error code: `rule_denied`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/tokens` | Issue a local demo access token. |
| `POST` | `/checkout/guard` | Verify claims, price checkout, evaluate rules, and admit or reject the submission. |

## Boundary Notes

- `/tokens` is a local demo endpoint, not an identity provider.
- The checkout request has one settlement currency. Mixed line currencies are
  rejected as `invalid_money`.
- `regional-vat` is illustrative and is not tax compliance logic.
- `probably_seen` can include Bloom false positives; pair it with durable
  idempotency storage in production.

## Test

```bash
go test -count=1 ./examples/checkout-guard-integration/...
go test -race -count=1 ./examples/checkout-guard-integration/...
```
