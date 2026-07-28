# Multi-Currency Invoice Rule Lesson 정리

## 요약

`examples/multi-currency-invoice-rules`는 #45 `examples/money-rule-pricing`
baseline을 single-currency cart total에서 currency별로 group된 invoice total로 확장한다.

## 결정

- public JSON의 모든 monetary amount는 string과 explicit currency로 유지한다.
- parsing, multiplication, addition, subtraction, currency-scale rounding에는
  `money.Money`를 사용한다.
- invoice total은 currency별로 group하고 `conversion_applied`는 `false`로 설정한다.
- 현재 `bluetape-go` surface가 generic rule engine을 제공하지 않으므로
  `vip-service-discount`와 `regional-vat`는 example-local rule decision function으로
  유지한다.
- JPY로 zero-minor-unit rounding을 눈에 보이게 만든다.

## 경계

- exchange rate나 cross-currency settlement는 없다.
- `regional-vat`는 설명용이며 tax compliance logic이 아니다.
- rule decision은 workshop을 위한 public evidence이지 reusable promotion framework가
  아니다.

## 검증

```bash
go test -count=1 ./examples/multi-currency-invoice-rules/...
go test -race -count=1 ./examples/multi-currency-invoice-rules/...
```
