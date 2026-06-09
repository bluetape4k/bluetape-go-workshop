# Issue #72 Plan: Order Fulfillment Workflow Integration Example

## Work Type

Type A - Full Feature.

Reason: new runnable example directory, Go implementation, tests, bilingual
READMEs, generated diagrams, root navigation updates, review artifacts, and PR.

## Implementation Tasks

1. Add `examples/order-fulfillment-integration/main.go` with `HTTP_ADDR`
   default `:8088`.
2. Add `examples/order-fulfillment-integration/internal/orderfulfillment`.
   - Gin routes: `GET /healthz`, `POST /orders/fulfillment`.
   - Request validation for malformed JSON, blank `order_id`, and non-positive
     totals.
   - Request-scoped `state.Machine` for draft/submitted/paid/packed/shipped/
     cancelled lifecycle transitions.
   - `workflow.Sequential` forward runner with `StopOnFailure`.
   - Reverse compensation runner with `ContinueOnFailure`.
   - Stable `workreport` projection and summary without timestamp fields.
3. Add focused tests.
   - health
   - shipped happy path
   - invalid transition inside workflow
   - shipment failure with reverse compensation and cancelled state
   - compensation failure preserving original shipment error
   - caller cancellation before/during workflow
   - invalid requests
   - parallel request independence
4. Add English/Korean READMEs.
   - scenario
   - how the example integrates smaller 0.4.0 examples
   - API examples
   - Architecture
   - Sequence Diagram
   - production hardening notes
5. Add `scripts/generate-order-fulfillment-integration-diagrams.sh`.
   - DOT/plain/Graphviz SVG/PNG route evidence.
   - final decorated SVG/PNG assets.
   - geometry gate output with concrete `margins=L/R/T/B` values.
6. Update root `README.md` and `README.ko.md` example maps and quickstart
   entries.
7. Add Step 6-R review/verifier artifacts after implementation.

## Validation Plan

- `bash scripts/generate-order-fulfillment-integration-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/order-fulfillment-integration/...`
- `go test -race -count=1 ./examples/order-fulfillment-integration/...`
- `go test -run '^$' ./examples/order-fulfillment-integration`
- `git diff --check`
- `golangci-lint cache clean && make ci`
- GitHub PR checks

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Integration example hides the smaller concepts | README and code keep state, workflow, workreport, and compensation boundaries explicit. |
| Example overclaims durable workflow behavior | README states it is request-scoped and lists durable storage, outbox, idempotency, and retry hardening. |
| Compensation failure hides the forward error | Response has `original_error`; tests assert it remains shipment failure even when compensation fails. |
| Cancellation skips cleanup after side effects | Spec and tests require compensation with `context.WithoutCancel` once a reversible side effect exists. |
| Diagram drift repeats margin/decorator regressions | Generator uses decorated assets and prints concrete margin evidence, not only raw Graphviz output. |
