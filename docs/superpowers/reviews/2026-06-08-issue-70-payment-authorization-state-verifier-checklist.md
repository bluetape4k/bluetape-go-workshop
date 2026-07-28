# Issue #70 verifier checklist

## 범위

- `docs/superpowers/research/2026-06-08-issue-70-payment-authorization-state-research.md`
- `docs/superpowers/specs/2026-06-08-issue-70-payment-authorization-state-design.md`
- `docs/superpowers/plans/2026-06-08-issue-70-payment-authorization-state-plan.md`
- `examples/payment-authorization-state`
- `docs/images/readme-diagrams/payment-authorization-state-*`
- root README navigation and workshop example map

## checklist

| 요구사항 | 근거 | 결과 |
|---|---|---|
| Step 2-R spec review 존재 | `docs/superpowers/reviews/2026-06-08-issue-70-payment-authorization-state-spec-review.md` | PASS |
| Step 3-R plan review 존재 | `docs/superpowers/reviews/2026-06-08-issue-70-payment-authorization-state-plan-review.md` | PASS |
| example runnable | `examples/payment-authorization-state/main.go`와 compile smoke `go test -run '^$' ./examples/payment-authorization-state` | PASS |
| transition table 명시성 | `newMachine`은 `server.go`에서 authorize, capture, fail, cancel, final state를 등록한다. | PASS |
| invalid transition 명확성 | HTTP error mapping은 invalid, guard, final, concurrent, idempotency conflict failure에 stable code를 반환한다. | PASS |
| idempotent retry는 app-layer | `transitionWithIdempotency`는 transition 전에 replay를 확인하고 successful transition response만 저장하며 replay를 표시한다. | PASS |
| failed transition은 replay로 저장되지 않음 | test coverage는 failed key가 나중에 `idempotent_replay=true` 없이 valid transition을 실행할 수 있음을 확인한다. | PASS |
| concurrent access serialization | transition과 idempotency write는 `Server.mu`로 보호된다. race test는 concurrent duplicate authorize request를 다룬다. | PASS |
| README scenario | `examples/payment-authorization-state/README.md`와 `README.ko.md`는 scenario section을 포함한다. | PASS |
| README architecture | English/Korean README는 PNG embed가 있는 Architecture section을 포함한다. | PASS |
| README sequence | English/Korean README는 PNG embed가 있는 Sequence Diagram section을 포함한다. | PASS |
| root navigation | `README.md`, `README.ko.md`, `workshop-example-map.png`는 `payment-authorization-state`를 포함한다. | PASS |
| code review | `docs/superpowers/reviews/2026-06-08-issue-70-payment-authorization-state-code-review.md`에는 P0=0/P1=0이 있다. | PASS |

## 검증 명령

- `bash scripts/generate-payment-authorization-state-diagrams.sh`
- `go test -count=1 ./examples/payment-authorization-state/...`
- `go test -race -count=1 ./examples/payment-authorization-state/...`
- `go test -run '^$' ./examples/payment-authorization-state`
- `git diff --check`
- `make ci`

## 상태

최종 repository-wide validation은 PR body와 final report에 기록되어 있다.
