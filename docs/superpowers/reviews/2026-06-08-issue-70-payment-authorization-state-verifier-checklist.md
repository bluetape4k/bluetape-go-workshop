# Issue #70 Verifier Checklist

## Scope

- `docs/superpowers/research/2026-06-08-issue-70-payment-authorization-state-research.md`
- `docs/superpowers/specs/2026-06-08-issue-70-payment-authorization-state-design.md`
- `docs/superpowers/plans/2026-06-08-issue-70-payment-authorization-state-plan.md`
- `examples/payment-authorization-state`
- `docs/images/readme-diagrams/payment-authorization-state-*`
- root README navigation and workshop example map

## Checklist

| Requirement | Evidence | Result |
|---|---|---|
| Step 2-R spec review exists | `docs/superpowers/reviews/2026-06-08-issue-70-payment-authorization-state-spec-review.md` | PASS |
| Step 3-R plan review exists | `docs/superpowers/reviews/2026-06-08-issue-70-payment-authorization-state-plan-review.md` | PASS |
| Example is runnable | `examples/payment-authorization-state/main.go` and compile smoke `go test -run '^$' ./examples/payment-authorization-state` | PASS |
| Transition table is explicit | `newMachine` registers authorize, capture, fail, cancel, and final states in `server.go`. | PASS |
| Invalid transitions are clear | HTTP error mapping returns stable codes for invalid, guard, final, concurrent, and idempotency conflict failures. | PASS |
| Idempotent retry is app-layer | `transitionWithIdempotency` checks replay before transition, stores only successful transition responses, and marks replays. | PASS |
| Failed transitions are not stored as replay | Test coverage confirms a failed key can later run a valid transition without `idempotent_replay=true`. | PASS |
| Concurrent access is serialized | Transition plus idempotency writes are guarded by `Server.mu`; race test covers concurrent duplicate authorize requests. | PASS |
| README scenario | `examples/payment-authorization-state/README.md` and `README.ko.md` include scenario sections. | PASS |
| README architecture | English and Korean READMEs include Architecture sections with PNG embeds. | PASS |
| README sequence | English and Korean READMEs include Sequence Diagram sections with PNG embeds. | PASS |
| Root navigation | `README.md`, `README.ko.md`, and `workshop-example-map.png` include `payment-authorization-state`. | PASS |
| Code review | `docs/superpowers/reviews/2026-06-08-issue-70-payment-authorization-state-code-review.md` has P0=0/P1=0. | PASS |

## Verification Commands

- `bash scripts/generate-payment-authorization-state-diagrams.sh`
- `go test -count=1 ./examples/payment-authorization-state/...`
- `go test -race -count=1 ./examples/payment-authorization-state/...`
- `go test -run '^$' ./examples/payment-authorization-state`
- `git diff --check`
- `make ci`

## Status

Final repository-wide validation is recorded in the PR body and final report.

