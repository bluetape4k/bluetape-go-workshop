# Code review: issue #112 exchange-rate pricing

## 범위

- 새 runnable example: `examples/exchange-rate-pricing`
- architecture, request sequence, stale quote policy를 위한 새 README diagram
- root README navigation과 focused run instruction
- provider-backed money conversion example lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- provider I/O는 `money.ExchangeRateProvider` 뒤에 격리되고 child context deadline을 받는다.
- mixed line currency는 provider I/O 전에 거부된다.
- language-only 또는 unsupported locale은 안정적인 `invalid_locale` HTTP error로 mapping된다.
- provider failure detail은 HTTP error response에 노출되지 않는다.
- stale fallback은 보이게 유지되며 explicit `allow_stale_quote` opt-in이 필요하다.
- test는 fresh quote, stale opt-in, stale rejection, provider failure, unsupported locale,
  currency mismatch, provider deadline propagation, router error mapping을 다룬다.

## 잔여 Risk

runtime example은 의도적으로 deterministic static provider를 사용한다. future milestone에서
network-backed provider behavior가 필요하면 live ECB 또는 IMF provider wiring은 별도
integration-oriented example로 남긴다.
