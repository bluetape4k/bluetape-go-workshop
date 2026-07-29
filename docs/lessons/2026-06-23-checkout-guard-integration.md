# Checkout Guard 통합 Lesson

## 요약

`examples/checkout-guard-integration`은 v0.6.0 focused utility example을 하나의
checkout submission boundary로 조합한다. 이 boundary에는 ID generation, JWT claim
verification, money/rule pricing, probabilistic repeated-submission admission이 포함된다.

## 결정

- token issue는 local 및 demo-only로 유지한다. lesson은 identity-provider design이 아니라
  protected checkout boundary다.
- pricing이나 admission 전에 `token_use`, issuer, audience, role, scope, session ID를
  요구한다.
- pricing은 single-currency로 유지한다. mixed line currency는 conversion하지 않고 reject한다.
- `vip-service-discount`, `regional-vat`, `restricted-category`에는 local rule function을
  사용하며 generic rule engine은 도입하지 않는다.
- request validation과 rule pricing이 성공한 뒤에만 Bloom filter로 idempotency key를
  admit한다.
- `probably_seen`은 authoritative proof가 아니라 possible duplicate 또는 false positive로
  문서화한다.

## 경계

- payment capture, inventory reservation, fulfillment workflow, ledger,
  persistent checkout store는 없다.
- token refresh flow는 없다. `token-refresh-claims`가 해당 focused lesson을 담당한다.
- multi-currency conversion은 없다. `multi-currency-invoice-rules`가 grouped currency
  total을 담당한다.

## 검증

```bash
go test -count=1 ./examples/checkout-guard-integration/...
go test -race -count=1 ./examples/checkout-guard-integration/...
```
