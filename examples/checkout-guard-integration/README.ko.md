# checkout-guard-integration

[English](README.md) | [한국어](README.ko.md)

`id`, `jwt`, `money`, `probabilistic`를 조합하는 Gin checkout guard 예제입니다.

이 예제는
[`id-jwt-boundary`](../id-jwt-boundary/README.ko.md),
[`token-refresh-claims`](../token-refresh-claims/README.ko.md),
[`money-rule-pricing`](../money-rule-pricing/README.ko.md),
[`multi-currency-invoice-rules`](../multi-currency-invoice-rules/README.ko.md),
[`probabilistic-dedupe-admission`](../probabilistic-dedupe-admission/README.ko.md)
의 집중 예제를 하나의 checkout boundary로 조합합니다.

## 시나리오

Checkout API가 인증된 submission을 받습니다. Service는 JWT access-token
contract를 검증하고, decimal-backed money로 checkout을 가격 계산하며, local
eligibility rule을 평가하고, idempotency key를 Bloom filter로 확인한 뒤 내부
request/order ID를 반환합니다.

Bloom filter는 반복 submission으로부터 hot path를 보호하지만 hit는
`probably_seen`일 뿐입니다. Production checkout idempotency에는 durable
authoritative store가 여전히 필요합니다.

## 보여주는 것

- Issuer, audience, `token_use`, role, scope, session을 포함한 JWT claim check.
- Trust boundary 안에서 생성되는 UUID v7 request/order ID.
- Rounded subtotal, discount, tax, final total을 가진 single-currency checkout
  pricing.
- VIP service discount, EU tax-like adjustment, restricted category denial을
  표현하는 local rule decision.
- `admit`과 `probably_seen` path를 가진 Bloom filter admission.
- Raw parser diagnostic을 노출하지 않는 stable public HTTP error.

## 실행

```bash
go run ./examples/checkout-guard-integration
```

Service는 기본적으로 `127.0.0.1:8101`에서 실행됩니다.

```bash
curl http://127.0.0.1:8101/healthz
```

Local demo access token을 발급합니다:

```bash
TOKEN=$(
  curl -s -X POST http://127.0.0.1:8101/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["checkout:submit"],"ttl_seconds":300}' \
  | jq -r '.token'
)
```

Guarded checkout을 submit합니다:

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

예상 total은 `USD` 기준 subtotal `39.99`, discount `2.00`, tax `7.60`, total
`45.59`를 포함합니다.

같은 `idempotency_key`로 동일 checkout을 다시 실행하면 public
`duplicate_submission` error를 확인할 수 있습니다.

Rule denial을 확인합니다:

```bash
curl -s -X POST http://127.0.0.1:8101/checkout/guard \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"checkout_id":"chk-1002","idempotency_key":"idem-1002","currency":"USD","items":[{"line_id":"restricted","sku":"restricted","unit_price":"12.00","currency":"USD","quantity":1,"category":"restricted"}]}'
```

예상 public error code는 `rule_denied`입니다.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/tokens` | Local demo access token을 발급합니다. |
| `POST` | `/checkout/guard` | Claim 검증, checkout pricing, rule 평가, submission admission/rejection을 수행합니다. |

## Boundary Notes

- `/tokens`는 local demo endpoint이며 identity provider가 아닙니다.
- Checkout request는 하나의 settlement currency만 가집니다. Mixed line currency는
  `invalid_money`로 거부합니다.
- `regional-vat`은 예시용 tax-like adjustment이며 compliance logic이 아닙니다.
- `probably_seen`에는 Bloom false positive가 포함될 수 있습니다. Production에서는
  durable idempotency storage와 함께 사용해야 합니다.

## 테스트

```bash
go test -count=1 ./examples/checkout-guard-integration/...
go test -race -count=1 ./examples/checkout-guard-integration/...
```
