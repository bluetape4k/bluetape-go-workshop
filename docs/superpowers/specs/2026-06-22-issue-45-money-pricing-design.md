# Issue #45 설계: Money and Rule Based Pricing 예제

## 목표

`github.com/bluetape4k/bluetape-go/money`와 작은 application-local rule primitive로
money-safe cart pricing을 보여주는 실행 가능한 `examples/money-rule-pricing` Gin
API를 추가한다.

이 예제는 가격을 `float64`로 표현하지 않는 이유, currency mismatch를 산술 전에
거부하는 방법, money boundary에서 rounded total을 만드는 방법, accepted/rejected
pricing rule을 결정적으로 보고하는 방법을 설명해야 한다.

## 현재 근거

- `bluetape-go` v0.6.2는 `Money`, `Currency`, `New`, `Parse`, `Add`, `Sub`,
  `Mul`, `Round`, `RoundTo`, `Sum`, `ErrCurrencyMismatch` 같은 sentinel error를
  포함한 `github.com/bluetape4k/bluetape-go/money`를 노출한다.
- 현재 module은 전용 `rule` 또는 `rules` package를 노출하지 않는다. 따라서 #45의
  rule 동작은 워크숍에서 reusable rules framework를 발명하지 말고 작고
  example-local로 유지해야 한다.
- 최근 Gin 예제는 기본적으로 HTTP service를 loopback에 bind하고 `/healthz`를
  노출하며 JSON body를 제한하고 안정적인 public error code를 문서화한다.

## 사용자 이야기

워크숍 독자로서 나는 line-item price, currency, customer tier, 선택적 coupon을 담은
cart quote request를 제출할 수 있다. API는 decimal-backed `money.Money` 산술로
subtotal, accepted discount, rejected rule, final total을 반환한다.

## 범위

- `examples/money-rule-pricing`을 추가한다.
- Internal package `examples/money-rule-pricing/internal/moneypricing`을 추가한다.
- 예제용 English/Korean README를 추가한다.
- Root README의 example table 아래에 예제 항목을 추가한다.
- `docs/lessons` 아래에 간결한 lesson note를 추가한다.

## 비목표

- reusable pricing, rule-engine, promotion framework package를 추가하지 않는다.
- persistent storage, authentication, external service call을 추가하지 않는다.
- cross-currency conversion을 지원하지 않는다. Mixed item currency는 거부한다.
- monetary arithmetic에 floating point를 사용하지 않는다.

## API 계약

### Endpoint

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/healthz` | Process liveness 전용. |
| `POST` | `/quotes` | Cart quote request 하나를 가격 계산한다. |

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

성공 응답은 string money field를 포함한다.

- `subtotal`
- `discount_total`
- `total`
- per-line `line_total`
- `rules[]` with `name`, `status`, optional `amount`, and optional `reason`

Rule status 값은 `accepted`, `rejected`, 그리고 rule을 평가했지만 적용하지 않았을
때의 `skipped`다.

### Public Error

| Code | HTTP | 의미 |
|---|---:|---|
| `invalid_request` | 400 | 잘못된 JSON 또는 누락/유효하지 않은 cart field. |
| `invalid_money` | 400 | 유효하지 않은 currency 또는 decimal amount. |
| `currency_mismatch` | 400 | Item currency가 cart currency와 다름. |
| `pricing_error` | 500 | 예상하지 못한 pricing failure. |

Error response는 implementation internal을 노출하는 raw parser diagnostic을 echo하면
안 된다.

## Pricing Rule

Rule은 결정적이며 다음 순서로 평가한다.

1. `vip-ten-percent`: `customer_tier = "vip"`에 대해 subtotal 10 percent discount를 accept한다.
2. `coupon-save10`: subtotal이 감당할 수 있을 때 `coupon_code = "SAVE10"`에 대해 고정 10.00 discount를 accept한다.
3. 지원하지 않는 coupon code는 rejected `coupon` decision으로 표현하며 total을 줄이지 않는다.
4. 현재 subtotal보다 큰 fixed discount는 `discount_exceeds_subtotal`로 reject하고 total을 줄이지 않는다.

Discount total은 subtraction 전에 cart currency scale로 round한다.

## Acceptance Criteria

- `go test -count=1 ./examples/money-rule-pricing/...`가 통과한다.
- `go test -race -count=1 ./examples/money-rule-pricing/...`가 통과한다.
- `go test -p 1 ./...`가 통과한다.
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint`가 통과하거나,
  기존의 관련 없는 failure를 evidence와 함께 문서화한다.
- 테스트는 다음을 다룬다.
  - rounded line total과 rounded discount total
  - rejected mixed currency cart input
  - valid VIP 및 coupon discount
  - rejected coupon/discount path
  - HTTP success 및 public error mapping
- README.md와 README.ko.md는 money를 `float64`로 표현하지 않는 이유를 설명한다.
- Root English/Korean README는 새 예제를 link하고 `money`를 bluetape-go package로
  나열한다.

## Step 2-R Integrated Review

이 Codex surface에서는 native subagent spawning을 사용할 수 없으므로, 여섯 review
lane은 full-feature reference contract를 사용한 독립 main-session check로 실행했다.

| Priority | Area | Finding | Required spec edit |
|---|---|---|---|
| P2 | Developer | 현재 dependency에는 upstream `rule` package가 없으므로 issue title이 workshop-local framework를 유도할 수 있다. | Rule primitive를 example-local 및 non-reusable로 기록했다. 완료. |
| P2 | User | 독자는 validation error뿐 아니라 visible rejected-rule 동작이 필요하다. | `rules[]` decision과 rejected coupon/discount acceptance criteria를 추가했다. 완료. |
| P2 | Security | HTTP error가 parser internal을 leak하면 안 된다. | Public error code contract를 추가했다. 완료. |
| P3 | Operator | 예제는 local 및 deterministic 상태를 유지해야 한다. | Storage, auth, external call 비목표를 추가했다. 완료. |

최종 Step 2-R 판정: P0 = 0, P1 = 0.
