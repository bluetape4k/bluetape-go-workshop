# multi-currency-invoice-rules

[English](README.md) | [한국어](README.ko.md)

`money`를 사용하는 Gin invoice evaluation 예제입니다.

이 예제는 #45의 기본
[`money-rule-pricing`](../money-rule-pricing/README.ko.md) lesson을 확장합니다.
같은 decimal-backed money contract를 유지하면서 invoice line을 통화별로 평가해
discount, tax-like adjustment, rounding decision이 명시적으로 보이게 합니다.

## 시나리오

Invoice service가 여러 통화의 line item을 받습니다. Service는 각 line을 해당 통화
scale로 round하고, VIP service discount와 예시용 EU regional VAT adjustment를
적용한 뒤 currency별 total을 반환합니다. Exchange-rate conversion은 수행하지
않습니다.

Rule은 application-local decision function입니다. 이 예제는 generic rule engine을
만들지 않습니다.

## 보여주는 것

- Cross-currency arithmetic 없이 만드는 multi-currency invoice total.
- `money.Money` parsing, same-currency arithmetic, currency-scale rounding.
- JPY 같은 zero-minor-unit currency rounding.
- Line, currency, status, optional amount, reason을 가진 rule decision.
- Invalid request와 invalid money input에 대한 stable public HTTP error.

## 실행

```bash
go run ./examples/multi-currency-invoice-rules
```

Service는 기본적으로 `127.0.0.1:8100`에서 실행됩니다.

```bash
curl http://127.0.0.1:8100/healthz
```

Invoice를 evaluate합니다:

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

예상 grouped total:

- `USD`: subtotal `39.99`, discount `2.00`, tax `7.60`, total `45.59`
- `EUR`: subtotal `10.00`, discount `0.00`, tax `2.00`, total `12.00`
- `JPY`: subtotal `101`, discount `0`, tax `0`, total `101`

Invalid currency input을 확인합니다:

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

예상 public error code는 `invalid_money`입니다.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/invoices/evaluate` | Invoice 하나를 평가하고 currency별 total을 반환합니다. |

## Boundary Notes

- 이 예제는 currency conversion을 하지 않습니다. Total은 currency별로 group합니다.
- `regional-vat`은 예시용 tax-like adjustment이며 compliance logic이 아닙니다.
- Rule은 local function이며 reusable rule framework가 아닙니다.
- Public error response는 raw invalid amount나 currency parser diagnostic을 노출하지
  않습니다.

## 테스트

```bash
go test -count=1 ./examples/multi-currency-invoice-rules/...
go test -race -count=1 ./examples/multi-currency-invoice-rules/...
```
