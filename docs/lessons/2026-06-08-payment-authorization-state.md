# 교훈: Payment Authorization State 예제

## 변경

- 실행 가능한 Gin main package가 있는 `examples/payment-authorization-state`를 추가했다.
- requested, authorized, captured, failed, cancelled state를 가진 payment-specific `state.Machine`을 추가했다.
- successful transition 주변에 app-layer idempotent retry behavior를 추가했다.
- valid transition, invalid transition, guard rejection, final-state rejection, idempotent replay, idempotency key conflict, failed transition non-storage, concurrent duplicate transition safety에 대한 테스트를 추가했다.
- scenario, architecture, sequence section을 위한 English/Korean README 파일과 diagram asset을 추가했다.

## Guardrail

- idempotency는 `state.Machine` 밖에 명확히 둔다. package는 transition legality를 소유하지 replay semantics를 소유하지 않는다.
- successful transition response만 idempotent replay로 저장한다.
- in-memory state와 idempotency storage가 workshop-only임을 문서화한다.
- 예제를 production payment에 적용하기 전에 durable state와 idempotency storage를 사용한다.

## 검증

- `bash scripts/generate-payment-authorization-state-diagrams.sh`
- 다음 항목을 visual inspection했다.
  - `payment-authorization-state-scenario.png`
  - `payment-authorization-state-architecture.png`
  - `payment-authorization-state-sequence.png`
  - `workshop-example-map.png`
- `go test -count=1 ./examples/payment-authorization-state/...`
- `go test -race -count=1 ./examples/payment-authorization-state/...`
- `go test -run '^$' ./examples/payment-authorization-state`
