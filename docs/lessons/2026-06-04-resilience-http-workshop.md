# Resilience HTTP Workshop Example

## Context

`bluetape-go` milestone `0.2.0` added HTTP resilience policies. Before closing
the library Epic, the workshop needed a runnable service example that proves the
public API works in application-shaped code.

## Decision

Add `examples/resilience-http-web` as a thin chi-based HTTP service. Keep the
example focused on policy wiring and event visibility, not on reusable helper
abstractions.

## Outcome

The example demonstrates outbound retry/timeout/circuit-breaker composition,
inbound bulkhead protection, typed error handling, and low-cardinality event
hooks.

## Verification

- `go test -count=1 ./examples/resilience-http-web/...`
- `make ci`
- `git diff --check`

## Future Guard

When `bluetape-go` adds a new feature, add workshop examples only after the
library API is stable enough to compile against a GitHub pseudo-version.
