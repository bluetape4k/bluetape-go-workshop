# Issue #71 Plan: Compensation Workflow Example

## Work Type

Type A - Full Feature.

Reason: new runnable example directory, Go implementation, tests, bilingual
READMEs, generated diagrams, root navigation, review artifacts, lesson, PR, and
CI.

## Implementation Tasks

1. Add `examples/compensation-workflow/main.go` with `HTTP_ADDR` default
   `:8087`.
2. Add `examples/compensation-workflow/internal/compensation/server.go`.
   - Gin routes: `GET /healthz`, `POST /compensation/fulfillment`.
   - Request validation and stable error responses.
   - Forward `workflow.Sequential` for inventory, payment, shipment.
   - Reverse-order compensation stack for successful reversible steps.
   - Top-level report preserving the original forward error.
3. Add `server_test.go`.
   - success
   - shipment failure with reverse compensation order
   - payment failure with inventory release only
   - compensation failure with original error preserved and later compensation
     still executed
   - inventory failure without compensation
   - caller cancellation
   - bad requests
4. Add English/Korean READMEs.
   - scenario
   - architecture
   - sequence
   - API examples
   - compensation vs state transition guidance
   - production hardening notes
5. Add `scripts/generate-compensation-workflow-diagrams.sh`.
   - DOT/plain/Graphviz SVG/PNG evidence.
   - final SVG/PNG assets.
   - deterministic gate summary with zero bad geometry/font counts.
6. Update root README navigation and run section.
7. Regenerate workshop example map to include `compensation-workflow`.
8. Add lesson and Step 6-R review/verifier artifacts.

## Validation Plan

- `bash scripts/generate-compensation-workflow-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/compensation-workflow/...`
- `go test -race -count=1 ./examples/compensation-workflow/...`
- `go test -run '^$' ./examples/compensation-workflow`
- `git diff --check`
- `code-review-graph build --repo "$PWD"`
- `golangci-lint cache clean && make ci`
- GitHub PR checks

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Example overclaims durable saga behavior | README and code comments call compensation an app-layer request-scoped teaching boundary. |
| Original error hidden by compensation failure | Response has `original_error`; top-level report keeps original error; tests assert it. |
| Compensation order unclear | Tests assert `void-payment` before `release-inventory`; diagram shows reverse order. |
| Diagram layout regression | Use Graphviz evidence, generated PNG pairs, geometry gate output, and visual preview. |

