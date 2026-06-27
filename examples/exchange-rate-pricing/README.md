# exchange-rate-pricing

[English](README.md) | [한국어](README.ko.md)

Gin display-pricing example for provider-backed `money` conversion.

This example starts from a cart priced in one base currency, chooses the buyer's
display currency from a locale, asks an `ExchangeRateProvider` for a rate, and
returns both the base subtotal and the converted display total. It is meant to
show the application boundary around `money.ConvertWithProvider`, not to call a
live exchange-rate service.

## Scenario

A pricing API receives a quote request from a buyer in Korea. The cart is priced
in `USD`, but the buyer locale is `ko-KR`, so the response should show a `KRW`
display total. The service keeps the original subtotal visible, converts only at
the display-pricing boundary, and returns rate metadata so the caller can see
which source and freshness policy shaped the total.

![Provider-backed exchange-rate pricing architecture](../../docs/images/readme-diagrams/exchange-rate-pricing-architecture.png)

## What It Demonstrates

- Locale-to-currency selection with `money.CurrencyByLocale`.
- Provider-backed conversion with `money.ConvertWithProvider`.
- String money inputs and outputs instead of `float64`.
- A provider I/O deadline created below the HTTP request context.
- Public response metadata for rate source, observed time, fetched time,
  expiry, and stale status.
- Explicit stale-quote policy: stale fallback is allowed only when the caller
  sets `allow_stale_quote`.
- Deterministic provider test doubles instead of live ECB or IMF endpoints.

## Request Flow

`POST /quotes` stays intentionally small. The HTTP handler validates JSON and
body size, then the service validates money fields, computes a base subtotal,
maps locale to display currency, and performs the provider-backed conversion.

![Exchange-rate quote request sequence](../../docs/images/readme-diagrams/exchange-rate-pricing-sequence.png)

The conversion call is the only provider I/O point:

1. `CurrencyByLocale("ko-KR")` selects `KRW`.
2. `subtotalLines` rejects mixed line currencies before provider I/O.
3. `operationContext` wraps the request context with a shorter provider
   deadline.
4. `ConvertWithProvider(ctx, subtotal, target, provider)` returns the converted
   money value and the quote metadata used for that conversion.
5. The response keeps both `subtotal` and `display_total`, so downstream code
   does not need to infer what was converted.

## Freshness Policy

The example separates provider availability from ordinary pricing validity. A
valid cart can still fail if the provider cannot return an acceptable quote.

![Exchange-rate freshness and failure policy](../../docs/images/readme-diagrams/exchange-rate-pricing-stale-policy.png)

| Provider result | Caller policy | HTTP result | Why |
|---|---|---|---|
| Fresh quote | No opt-in required | `200 OK` | Normal path still includes source and freshness metadata. |
| Stale fallback | `allow_stale_quote=true` | `200 OK` | Caller accepts the freshness tradeoff and sees `rate.stale=true`. |
| Stale fallback | Default policy | `503 stale_quote` | The API fails closed instead of hiding stale trust. |
| Provider error, no quote | Any policy | `503 exchange_rate_unavailable` | Raw provider details stay out of public errors. |

## Code Map

- `main.go` wires the loopback-only Gin server and a deterministic demo
  provider. The demo provider returns static `USD/KRW`, `USD/EUR`, `EUR/KRW`,
  and `EUR/USD` rates with `ECB demo snapshot` metadata.
- `internal/exchangepricing/service.go` owns request validation, base subtotal
  calculation, locale currency selection, provider conversion, stale policy, and
  public HTTP error mapping.
- `internal/exchangepricing/service_test.go` proves fresh quotes, stale quote
  opt-in, stale quote rejection, provider failure mapping, unsupported locale
  handling, currency mismatch rejection, and provider deadline propagation.
- `internal/exchangepricing/http_test.go` proves the public router response and
  stable public error codes.

## Run

```bash
go run ./examples/exchange-rate-pricing
```

The service listens on `127.0.0.1:8101` by default. `HTTP_ADDR` may override the
address, but the example accepts only loopback addresses.

```bash
curl http://127.0.0.1:8101/healthz
```

Quote a USD cart for a Korean buyer:

```bash
curl -s -X POST http://127.0.0.1:8101/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "quote_id":"quote-1001",
    "base_currency":"USD",
    "locale":"ko-KR",
    "items":[
      {"sku":"pro-plan","unit_price":"19.995","currency":"USD","quantity":2},
      {"sku":"support","unit_price":"5.00","currency":"USD","quantity":1}
    ]
  }' | jq
```

Expected highlights:

```json
{
  "base_currency": "USD",
  "display_currency": "KRW",
  "subtotal": {"amount": "44.99", "currency": "USD"},
  "display_total": {"amount": "58487", "currency": "KRW"},
  "rate": {
    "source": "ECB demo snapshot",
    "rate": "1300",
    "stale": false
  },
  "conversion_applied": true
}
```

Try an unsupported locale:

```bash
curl -s -X POST http://127.0.0.1:8101/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "quote_id":"quote-1002",
    "base_currency":"USD",
    "locale":"en",
    "items":[{"sku":"pro-plan","unit_price":"10.00","currency":"USD","quantity":1}]
  }'
```

Expected public error code: `invalid_locale`.

Try a currency mismatch:

```bash
curl -s -X POST http://127.0.0.1:8101/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "quote_id":"quote-1003",
    "base_currency":"USD",
    "locale":"ko-KR",
    "items":[{"sku":"pro-plan","unit_price":"10.00","currency":"EUR","quantity":1}]
  }'
```

Expected public error code: `currency_mismatch`. The provider is not called for
this request.

## Boundary Notes

- The runtime demo provider is deterministic and does not call live ECB or IMF
  endpoints. Live provider integration belongs behind the same
  `ExchangeRateProvider` interface.
- `CurrencyByLocale` requires a locale with an explicit supported region, such
  as `ko-KR` or `en-US`; language-only tags are rejected.
- The API exposes `refresh_error` only when it returns an allowed stale quote.
  Public HTTP errors use stable codes instead of raw provider details.
- The example converts the final subtotal for display. It does not price each
  line in a different currency.

## Test

```bash
go test -count=1 ./examples/exchange-rate-pricing/...
go test -race -count=1 ./examples/exchange-rate-pricing/...
```
