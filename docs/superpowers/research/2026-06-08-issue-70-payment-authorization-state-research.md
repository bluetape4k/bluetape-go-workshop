# Issue #70 Research: Payment Authorization State Transition Example

## Context

- Issue: `#70 [v0.4.0] Add payment authorization state transition example`
- Umbrella: `#28 [v0.4.0] Add state machine and workflow workshop examples`
- Parent roadmap epic: `#27`
- Current completed focused examples:
  - `#38` order lifecycle state API
  - `#39` fulfillment workflow runner
  - `#40` operations report policy

## Repository Evidence

- `examples/order-lifecycle-state-api` already teaches a general order finite
  state machine with Gin routes, allowed events, guard rejection, final-state
  rejection, and concurrent transition tests.
- The #70 example should not duplicate the entire order lifecycle API. It should
  narrow the domain to payment authorization and add one new application concern:
  idempotent command retry behavior around `state.Machine`.
- Root README now groups v0.4.0 under state/workflow/report examples, so #70 can
  be added as a payment-specific state example under the same lane.

## Library Evidence

Observed in `github.com/bluetape4k/bluetape-go@v0.5.1/state`:

- `state.Machine` is concurrency-safe.
- `Transition(ctx, event)`:
  - checks context before lookup and before commit
  - evaluates guards before acquiring the write lock
  - rejects stale concurrent transitions with `ErrConcurrentTransition`
- `CanTransition(ctx, event)` evaluates guards but never mutates state.
- `AllowedEvents()` returns registered events for the current state and does not
  evaluate guards.
- Final states reject further transitions through `ErrFinalState`.
- The package does not implement idempotency. Idempotent retry behavior must live
  at the application boundary.

## Scenario Decision

Build `examples/payment-authorization-state`, a Gin API for a single in-memory
payment authorization.

States:

- `requested`
- `authorized`
- `captured`
- `failed`
- `cancelled`

Events:

- `authorize`: `requested -> authorized`, guarded by positive amount.
- `capture`: `authorized -> captured`.
- `fail`: `requested -> failed` and `authorized -> failed`.
- `cancel`: `requested -> cancelled` and `authorized -> cancelled`.

Final states:

- `captured`
- `failed`
- `cancelled`

Idempotency:

- Transition commands accept an `idempotency_key`.
- A successful command stores its response by key.
- Repeating the same key and event returns the stored transition response with
  `idempotent_replay=true` and does not mutate state.
- Reusing a key with a different event returns `409 Conflict`.
- Invalid or guard-rejected transitions are not stored as successful idempotent
  results.

## Rejected Directions

- Calling an external payment gateway: unnecessary infrastructure for the state
  package lesson.
- Encoding gateway approval as request-scoped mutable state inside a guard:
  guards should remain safe for `CanTransition` calls.
- Reusing the order lifecycle example path: #70 should be readable as a separate
  payment-domain lesson and should link #38 as a prerequisite.
- Building a durable idempotency store: out of scope; README should call out that
  production idempotency needs durable storage.

## Acceptance Mapping

- Runnable example under `examples/`: add `examples/payment-authorization-state`.
- Valid transitions: tests cover authorize, capture, fail, and cancel paths.
- Invalid transitions: tests cover capture before authorize and final-state
  rejection.
- Retry/idempotency: tests cover same-key replay and key reuse conflict.
- README prerequisite: README links `examples/order-lifecycle-state-api`.
- README sync: add English and Korean READMEs and root navigation.
