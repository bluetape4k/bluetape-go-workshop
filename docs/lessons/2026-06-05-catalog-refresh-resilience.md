# Catalog Refresh Resilience Workshop Example

## Context

Issue #13 needed a `bluetape-go` v0.1.1 workshop example. The milestone is
mostly quality closure, but the tag line includes the first retry and timeout
resilience primitives. The example had to avoid duplicating the broader HTTP
resilience service and had to document the scenario with richer README diagrams.

## Decision

Add `examples/catalog-refresh-resilience` as plain domain code. A SKU refresh
job injects a catalog `Source` function and wraps it with `resilience.Run` using
retry as the outer policy and timeout as the inner policy. This proves a timeout
budget is created per retry attempt while keeping the example small.

README diagrams are stored as SVG/PNG pairs under
`docs/images/readme-diagrams/`, with Graphviz evidence for the flow and policy
sequence diagrams.

## Outcome

The example covers:

- transient upstream failure followed by retry success;
- slow source timeout visible through `errors.Is(err, resilience.ErrTimeout)`;
- retry exhaustion preserving the timeout as the last cause;
- invalid input checks before policy execution.

## Verification

- `codegraph init && codegraph index`
- `code-review-graph build --repo .`
- `go test -count=1 ./examples/catalog-refresh-resilience/...`
- `git diff --check`
- Diagram PNG render and visual inspection for:
  - `catalog-refresh-scenario-flow.png`
  - `catalog-refresh-policy-sequence.png`
  - `catalog-refresh-outcome-matrix.png`

## Future Guard

For small Go workshop examples, keep the code path scenario-first, but make the
README rich enough to explain the operational shape. When README diagrams are
added, commit PNG embeds plus matching SVG sources, and keep Graphviz evidence
for node-and-connector diagrams.
