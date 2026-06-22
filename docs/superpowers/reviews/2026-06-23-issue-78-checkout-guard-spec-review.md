# Issue #78 Spec Review

## Verdict

Proceed with implementation. The scope is correctly integration-shaped while
keeping each package's lesson narrow and avoiding a new checkout framework.

## Six-Lane Review

| Lane | Concern | Decision |
|---|---|---|
| Correctness | Money arithmetic | Keep one settlement currency per checkout and reject mismatched line currencies. |
| Domain | Dedupe semantics | Name Bloom output `probably_seen` and document durable idempotency as a production requirement. |
| Security | JWT boundary | Require `token_use`, issuer, audience, role, scope, and session ID before any checkout response. |
| Stability | Shared state | Guard Bloom admission and deterministic test IDs for concurrent duplicate tests. |
| Developer/API | Reuse | Use first-party `id`, `jwt`, `money`, and `probabilistic` APIs directly; do not import sibling `internal` examples. |
| User/Docs | Composition story | README should show how focused examples compose, not introduce a broad framework. |

## Required Adjustments

- Place the example at `examples/checkout-guard-integration` so the route is
  short and clearly integration-focused.
- Return `409` for duplicate submissions, but explain that a Bloom match can be
  a false positive.
- Add both service-level and HTTP-level tests for public error mapping.
- Include a race run for the new package because the example owns shared
  admission state.

## Blockers

None.
