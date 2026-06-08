# Order Lifecycle State API Example

## Context

Issue #38 added the first v0.4.0 state-machine HTTP example. The example had to
use Gin, exercise `bluetape-go/state`, and include bilingual README content with
scenario, architecture, and sequence diagrams.

## Decision

Keep the service intentionally in-memory and expose one order through a small
Gin router. Let `state.Machine` own transition legality, final-state rejection,
guard rejection, and concurrent transition conflicts. Keep the handler limited
to JSON binding, event parsing, and HTTP error mapping.

## Outcome

- Added `examples/order-lifecycle-state-api` with a runnable main package.
- Added focused tests for allowed transitions, invalid transitions, guard
  rejection, final-state rejection, bad requests, and concurrent duplicate
  transition safety.
- Added README and README.ko content with finite-state-machine vs workflow
  runner guidance.
- Added generated README diagrams for scenario, architecture, and sequence
  flows with PNG/SVG plus DOT, Plain, and Graphviz evidence.

## Verification

- `bash scripts/generate-order-lifecycle-diagrams.sh`
- `go test -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -race -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -count=1 ./...`
- `make ci`
- Individual rendered PNG inspection for the scenario, architecture, and
  sequence diagrams.

## Future Guard

Add the framework import before relying on `go mod tidy`; otherwise tidy removes
the dependency and the direct dependency check becomes misleading. For README
diagrams, regenerate assets from a checked-in script, keep PNG embeds in both
locales, and inspect the rendered PNGs after every visual route change.
