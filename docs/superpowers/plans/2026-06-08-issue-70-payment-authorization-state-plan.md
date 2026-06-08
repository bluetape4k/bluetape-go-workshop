# Issue #70 Plan: Payment Authorization State Example

## Objective

Create `examples/payment-authorization-state`, a Gin API that demonstrates
payment authorization state transitions and app-layer idempotent retry behavior
around `state.Machine`.

## Implementation Steps

1. Add the example package skeleton:
   - `examples/payment-authorization-state/main.go`
   - `examples/payment-authorization-state/internal/paymentauth/server.go`
   - `examples/payment-authorization-state/internal/paymentauth/server_test.go`
2. Implement the payment state model:
   - states: `requested`, `authorized`, `captured`, `failed`, `cancelled`
   - events: `authorize`, `capture`, `fail`, `cancel`
   - final states: `captured`, `failed`, `cancelled`
3. Implement Gin endpoints:
   - `GET /healthz`
   - `GET /payments/current`
   - `GET /payments/current/transitions/:event/can`
   - `POST /payments/current/transitions`
4. Implement idempotency:
   - require `idempotency_key` on transition commands
   - replay same key/event with `idempotent_replay=true`
   - reject same key/different event with `409 idempotency_conflict`
   - store only successful transition responses
5. Add focused tests for valid transitions, invalid transitions, guard rejection,
   idempotent replay, key conflict, final-state rejection, bad requests,
   concurrency, and race.
6. Add README.md and README.ko.md with:
   - #38 prerequisite link
   - scenario
   - state model
   - idempotency behavior
   - production hardening gaps
   - architecture and sequence diagram sections
7. Add `scripts/generate-payment-authorization-state-diagrams.sh` and generate
   PNG/SVG/DOT/plain assets.
8. Update root README.md/README.ko.md and workshop example map.
9. Add lesson and Step 6-R review artifacts.
10. Verify with focused tests, race test, compile smoke, diagram inspection,
    diff check, and repository CI gate before opening the PR.

## Validation Commands

```bash
bash scripts/generate-payment-authorization-state-diagrams.sh
go test -count=1 ./examples/payment-authorization-state/...
go test -race -count=1 ./examples/payment-authorization-state/...
go test -run '^$' ./examples/payment-authorization-state
git diff --check
make ci
```

## PR Deliverables

- implementation commit(s) after the planning commit
- PR body in English ending with `## DoD Status`
- evidence for tests and CI checks
- no merge until explicitly requested
