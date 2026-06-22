# money-rule-pricing

[English](README.md) | [한국어](README.ko.md)

`money`를 사용하는 Gin cart pricing 예제입니다.

이 예제는 decimal-backed `money.Money` 값과 application-local pricing rule로
cart를 가격 계산합니다. Rounded line total, currency mismatch rejection,
accepted discount, rejected coupon rule을 함께 보여줍니다.

## 시나리오

Checkout service가 cart quote request를 받습니다. Service는 모든 line item이 같은
통화인지 검증하고, rounded line total을 계산하며, VIP percentage discount와 fixed
coupon discount를 적용한 뒤 rule decision과 final total을 반환합니다.

Rule은 의도적으로 application 내부에만 둡니다. 이 예제는 reusable rule engine을
만들지 않습니다.

## 보여주는 것

- `bluetape-go/money`의 decimal-backed monetary arithmetic.
- `float64` 대신 string amount input/output.
- Arithmetic 전에 수행하는 currency mismatch rejection.
- Cart boundary에서 수행하는 rounded line total과 rounded discount total.
- Deterministic accepted/rejected pricing rule decision.
- Invalid request, invalid money, mixed currency에 대한 stable public HTTP error.

## 왜 `float64`를 쓰지 않는가

Money는 정확한 decimal 동작과 명시적인 currency boundary가 필요합니다. Binary
floating-point 값은 `0.1` 같은 값을 근사치로 표현할 수 있고, rounding 결정을 늦은
total 단계까지 숨길 수 있습니다. 이 예제는 public money value를 string으로 유지하고
연산을 `money.Money`에 맡겨 rounding과 currency check가 명명된 boundary에서 일어나게
합니다.

## 실행

```bash
go run ./examples/money-rule-pricing
```

Service는 기본적으로 `127.0.0.1:8098`에서 실행됩니다.

```bash
curl http://127.0.0.1:8098/healthz
```

Cart를 quote합니다:

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

예상 total은 subtotal `USD 42.49`, discount total `USD 14.25`, total
`USD 28.24`입니다.

Rejected rule을 확인합니다:

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

`coupon-save10` rule은 `discount_exceeds_subtotal`로 reject되고 total은 바뀌지
않습니다.

Currency mismatch를 확인합니다:

```bash
curl -s -X POST http://127.0.0.1:8098/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "cart_id":"cart-1003",
    "currency":"USD",
    "items":[{"sku":"coffee-1","unit_price":"5.00","currency":"EUR","quantity":1}]
  }'
```

예상 public error code는 `currency_mismatch`입니다.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/quotes` | Cart quote request 하나를 가격 계산합니다. |

## Boundary Notes

- 이 예제는 currency conversion을 하지 않습니다. Mixed currency는 reject합니다.
- Rule은 local function이며 reusable promotion framework가 아닙니다.
- Cart 자체가 valid하면 rejected rule은 성공 quote response 안에 반환됩니다.
- Invalid money, invalid JSON, mixed currency는 HTTP error입니다.

## 테스트

```bash
go test -count=1 ./examples/money-rule-pricing/...
go test -race -count=1 ./examples/money-rule-pricing/...
```
