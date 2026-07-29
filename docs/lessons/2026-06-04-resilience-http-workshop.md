# Resilience HTTP Workshop 예제

## 맥락

`bluetape-go` milestone `0.2.0`은 HTTP resilience policy를 추가했다. library Epic을 닫기 전에, public API가 애플리케이션 형태의 코드에서 동작함을 증명하는 실행 가능한 service 예제가 필요했다.

## 결정

`examples/resilience-http-web`를 얇은 `chi` 기반 HTTP service로 추가한다. 예제는 재사용 helper abstraction이 아니라 policy wiring과 event visibility에 집중한다.

## 결과

예제는 outbound retry/timeout/circuit-breaker 조합, inbound bulkhead protection, typed error handling, low-cardinality event hook을 보여 준다.

## 검증

- `go test -count=1 ./examples/resilience-http-web/...`
- `make ci`
- `git diff --check`

## 이후 Guard

`bluetape-go`가 새 feature를 추가할 때는 library API가 GitHub pseudo-version에 맞춰 compile될 만큼 안정된 뒤에만 workshop 예제를 추가한다.
