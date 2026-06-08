# Lesson: Payment Authorization State Example

## Change

- Added `examples/payment-authorization-state` with a runnable Gin main package.
- Added a payment-specific `state.Machine` with requested, authorized,
  captured, failed, and cancelled states.
- Added app-layer idempotent retry behavior around successful transitions.
- Added tests for valid transitions, invalid transitions, guard rejection,
  final-state rejection, idempotent replay, idempotency key conflict, failed
  transition non-storage, and concurrent duplicate transition safety.
- Added English/Korean README files and diagram assets for scenario,
  architecture, and sequence sections.

## Guardrails

- Keep idempotency clearly outside `state.Machine`; the package owns transition
  legality, not replay semantics.
- Store only successful transition responses as idempotent replays.
- Keep in-memory state and idempotency storage documented as workshop-only.
- Use durable state and idempotency storage before adapting the example to
  production payments.

## Validation

- `bash scripts/generate-payment-authorization-state-diagrams.sh`
- visual inspection of:
  - `payment-authorization-state-scenario.png`
  - `payment-authorization-state-architecture.png`
  - `payment-authorization-state-sequence.png`
  - `workshop-example-map.png`
- `go test -count=1 ./examples/payment-authorization-state/...`
- `go test -race -count=1 ./examples/payment-authorization-state/...`
- `go test -run '^$' ./examples/payment-authorization-state`
