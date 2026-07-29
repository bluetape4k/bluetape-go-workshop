# Issue #70 계획: Payment Authorization State 예제

## 목표

`state.Machine` 주변의 payment authorization state transition과 app-layer
idempotent retry behavior를 보여 주는 Gin API인
`examples/payment-authorization-state`를 만든다.

## 구현 단계

1. 예제 package skeleton을 추가한다.
   - `examples/payment-authorization-state/main.go`
   - `examples/payment-authorization-state/internal/paymentauth/server.go`
   - `examples/payment-authorization-state/internal/paymentauth/server_test.go`
2. payment state model을 구현한다.
   - states: `requested`, `authorized`, `captured`, `failed`, `cancelled`
   - events: `authorize`, `capture`, `fail`, `cancel`
   - final states: `captured`, `failed`, `cancelled`
3. Gin endpoint를 구현한다.
   - `GET /healthz`
   - `GET /payments/current`
   - `GET /payments/current/transitions/:event/can`
   - `POST /payments/current/transitions`
4. idempotency를 구현한다.
   - transition command에 `idempotency_key`를 요구한다.
   - 같은 key/event는 `idempotent_replay=true`로 replay한다.
   - 같은 key/different event는 `409 idempotency_conflict`로 거부한다.
   - 성공한 transition response만 저장한다.
5. valid transition, invalid transition, guard rejection, idempotent replay,
   key conflict, final-state rejection, bad request, concurrency, race용 focused
   test를 추가한다.
6. README.md와 README.ko.md를 추가한다.
   - #38 prerequisite link
   - scenario
   - state model
   - idempotency behavior
   - production hardening gaps
   - architecture and sequence diagram sections
7. `scripts/generate-payment-authorization-state-diagrams.sh`를 추가하고
   PNG/SVG/DOT/plain asset을 생성한다.
8. root README.md/README.ko.md와 workshop example map을 갱신한다.
9. lesson 및 Step 6-R review artifact를 추가한다.
10. PR을 열기 전에 focused test, race test, compile smoke, diagram inspection,
    diff check, repository CI gate로 검증한다.

## 검증 명령

```bash
bash scripts/generate-payment-authorization-state-diagrams.sh
go test -count=1 ./examples/payment-authorization-state/...
go test -race -count=1 ./examples/payment-authorization-state/...
go test -run '^$' ./examples/payment-authorization-state
git diff --check
make ci
```

## PR 산출물

- planning commit 이후 implementation commit
- `## DoD Status`로 끝나는 English PR body
- test 및 CI check evidence
- 명시 요청 전 merge 금지
