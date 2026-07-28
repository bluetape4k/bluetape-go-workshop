# Provider-backed exchange-rate pricing 예제

Issue: #112

## 결정

실행 가능한 Gin display-pricing 예제에서 `money.CurrencyByLocale`과
`money.ConvertWithProvider`를 직접 사용한다. demo provider와 test provider double은
deterministic하게 둔다.

## 이유

lesson은 application pricing과 provider-backed exchange-rate conversion 사이의 boundary다.
app은 subtotal validation, locale selection, provider I/O deadline, stale quote policy를
소유한다. `bluetape-go/money`는 currency parsing, locale currency default, conversion
validation, quote metadata를 소유한다.

## 검증 형태

- service test는 fresh conversion, stale opt-in, stale rejection, provider failure
  mapping, unsupported locale handling, provider I/O 전 currency mismatch를 assert한다.
- router test는 public error code와 provider detail이 HTTP failure response로 누출되지
  않음을 assert한다.
- README diagram은 architecture, request sequence, freshness/failure policy를 분리해
  reader가 test를 먼저 읽지 않아도 conversion boundary를 따라갈 수 있게 한다.
