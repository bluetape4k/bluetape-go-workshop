# Lesson: Order Fulfillment Integration Example

## Change

- Added `examples/order-fulfillment-integration` as the v0.4.0 milestone-level
  integration example.
- Combined `state.Machine`, `workflow.Sequential`, `workreport.Report`
  projection, and application-owned compensation in one Gin API.
- Preserved original forward errors while still reporting compensation child
  failures.
- Added cancellation coverage for both pre-side-effect cancellation and
  cancellation after a reversible side effect was registered.
- Added English/Korean README files and scenario, architecture, and sequence
  diagram assets.

## Guardrails

- Keep the example request-scoped. Do not imply it is a durable workflow engine
  or saga coordinator.
- Keep `state`, `workflow`, and `workreport` visible in code and docs; avoid
  hiding the lesson behind a generic orchestration wrapper.
- Preserve `original_error` even when compensation fails.
- Use `context.WithoutCancel` only for bounded cleanup after a reversible side
  effect has already been registered.
- For README diagrams, keep the decorated workshop baseline, PNG embeds, SVG
  siblings, Graphviz route evidence, and concrete L/R/T/B margin output.

## Validation

- `bash scripts/generate-order-fulfillment-integration-diagrams.sh`
- visual inspection of:
  - `order-fulfillment-integration-scenario.png`
  - `order-fulfillment-integration-architecture.png`
  - `order-fulfillment-integration-sequence.png`
  - `workshop-example-map.png`
- `go test -count=1 ./examples/order-fulfillment-integration/...`
- `go test -race -count=1 ./examples/order-fulfillment-integration/...`
- `go test -run '^$' ./examples/order-fulfillment-integration`
