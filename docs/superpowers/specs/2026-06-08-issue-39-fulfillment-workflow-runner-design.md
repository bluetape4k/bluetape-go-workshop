# Issue 39 Fulfillment Workflow Runner Design

## Classification

- Work type: Type A - Full Feature.
- Basis: issue #39 adds a new runnable Gin example directory, Go code, tests,
  English/Korean README files, root README entries, diagram assets, review
  artifacts, lessons, and a PR.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `feat/issue-39-fulfillment-workflow-runner` under
  `.worktrees/feat-issue-39-fulfillment-workflow-runner`.

## Problem

The workshop needs a 0.4.0 example that shows how the released
`workflow` runner composes ordinary Go work functions in an application-shaped
fulfillment flow. The example must go beyond package API snippets: readers
should see the boundary between sequential validation, parallel side-effect
branches, conditional shipment creation, and cancellation/failure reporting.

## Current Evidence

- GitHub issue #39 requires a fulfillment workflow runner example with
  sequential, parallel, conditional, failure, skip, and cancellation coverage.
- `github.com/bluetape4k/bluetape-go/workflow` provides `Sequential`,
  `Parallel`, and `Conditional` runners over context-aware work functions.
- `github.com/bluetape4k/bluetape-go/workreport` provides terminal statuses and
  failure policies shared by workflow runner results.
- The repository already uses Gin for the 0.4.0 public HTTP example
  `examples/order-lifecycle-state-api`.
- The user requires each example README to include an example scenario,
  Architecture, and Sequence Diagram following `bluetape4k-diagram` gates.

## Goals

- Add a runnable `examples/fulfillment-workflow-runner` Gin API.
- Demonstrate `workflow.Sequential`, `workflow.Parallel`, and
  `workflow.Conditional` in one fulfillment scenario.
- Return a stable JSON projection of the `workreport.Report` tree so tests and
  README examples can show execution boundaries.
- Cover success, step failure, conditional shipment skip, and cancellation in
  tests.
- Keep the example in-memory and deterministic.
- Update root README navigation and roadmap wording for 0.4.0.
- Add English/Korean example README files with scenario, Architecture, Sequence
  Diagram, run commands, endpoints, failure semantics, and production caveats.

## Non-Goals

- Do not implement a durable workflow engine, saga coordinator, retry scheduler,
  message queue, database, or external payment/inventory service.
- Do not add new runtime dependencies beyond already-present Gin and
  bluetape-go packages.
- Do not create a generic workflow DSL or mutable shared context map.
- Do not hide `workflow` semantics behind excessive application scaffolding.

## Proposed Example Shape

Directory:

```text
examples/fulfillment-workflow-runner/
  main.go
  README.md
  README.ko.md
  internal/fulfillment/
    server.go
    server_test.go
```

Package name: `fulfillment`.

`Server` implements `http.Handler`, owns a Gin router, and builds a fresh
workflow runner for each request. Workflow state stays request-scoped so tests
can prove behavior without cross-request races.

## Domain Scenario

A fulfillment request contains:

- `order_id`
- `stock_available`
- `payment_authorized`
- `requires_shipment`

The API models this flow:

1. Sequential `fulfillment` runner starts with `validate-order`.
2. A parallel `risk-checks` runner runs:
   - `reserve-inventory`
   - `authorize-payment`
3. A conditional `shipment-decision` runner creates shipment only when
   `requires_shipment` is true. Otherwise it records a skipped shipment branch
   as completed without creating a shipment.

## HTTP API

Use Gin for all routes:

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/healthz` | Return service health. |
| `POST` | `/fulfillment/run` | Bind scenario input, run workflow, return report tree. |

Response conventions:

- `200 OK`: workflow completed successfully or completed with an intentional
  conditional skip.
- `409 Conflict`: workflow failed or partially completed because an ordinary
  branch failed.
- `408 Request Timeout`: caller context cancellation or deadline caused a
  cancelled workflow report.
- `400 Bad Request`: malformed JSON or invalid request fields.

## Report Projection

Expose a JSON shape that is stable for tests and README examples:

- workflow name
- status
- optional error message
- optional reason
- child reports recursively
- derived flags:
  - `success`
  - `failure`
  - `cancelled`

The API must not expose timestamps in the JSON response because
`workreport.Report` timestamps are intentionally runtime values and would make
README snippets and tests noisy.

## Error and Cancellation Contract

- Validation failure is an ordinary failed report and maps to `409` when the
  request shape is valid but the domain cannot proceed.
- Inventory reservation failure and payment authorization failure must be
  visible as named child reports under the parallel branch.
- `workflow.Parallel` with `StopOnFailure` should cancel sibling work on
  failure; tests must assert a cancellation child when a slow sibling observes
  the shared context cancellation.
- A cancelled caller context must produce a `cancelled` report and map to `408`.
- Malformed JSON and invalid fields must not enter the workflow; they return
  `400`.

## Design Options

### Option A - One Gin Endpoint With Request-Scoped Workflow

Build the runner per request and return the report tree.

Benefits:

- Small and scenario-shaped.
- No cross-request shared state.
- Directly exposes sequential, parallel, conditional semantics.

Costs:

- Not a production orchestration engine.

### Option B - Domain Function Only

Expose only `RunFulfillment(ctx, input)` and tests.

Benefits:

- Smaller code.

Costs:

- Misses the workshop's public HTTP API direction and gives no runnable service.

### Option C - Durable Fulfillment Store

Track workflow runs and statuses in memory or persistence.

Benefits:

- Closer to production workflow UX.

Costs:

- Adds storage and lifecycle ownership unrelated to the `workflow` runner.

## Decision

Adopt Option A.

The issue is about demonstrating lightweight runner semantics. A request-scoped
Gin workflow keeps the example runnable, testable, and clear while deferring
durability and orchestration concerns to later integration examples.

Rejected:

- Option B because issue #39 and workshop direction require public Gin examples.
- Option C because persistence would distract from the runner contract and add
  failure modes not owned by `workflow`.

## Test Strategy

Focused package tests:

- health endpoint returns OK.
- success request returns completed workflow with validate, parallel risk
  checks, and shipment creation children.
- conditional skip request returns completed workflow without shipment creation.
- inventory failure returns `409`, parent failed/partial status, and visible
  child failure.
- payment failure cancels a slow inventory sibling under `StopOnFailure`.
- caller cancellation returns `408` and a cancelled report.
- malformed JSON and invalid request fields return `400`.

Validation commands:

- `bash scripts/generate-fulfillment-workflow-diagrams.sh`
- `go test -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -run '^$' ./examples/fulfillment-workflow-runner`
- `go test -count=1 ./...`
- `git diff --check`
- `make ci`

## Documentation and Diagram Impact

- Add `examples/fulfillment-workflow-runner/README.md`.
- Add `examples/fulfillment-workflow-runner/README.ko.md`.
- Add generated README diagram assets:
  - scenario
  - architecture
  - sequence
- Add `scripts/generate-fulfillment-workflow-diagrams.sh` with deterministic
  gate summaries following `bluetape4k-diagram`.
- Update root `README.md` and `README.ko.md` example tables and 0.4.0 run
  instructions.
- README must explain:
  - workflow step boundaries
  - failure policy semantics
  - cancellation propagation
  - conditional skip behavior
  - production hardening gaps

## Risks

1. **Overclaiming durability**: `workflow` is not a durable workflow engine.
   README and API wording must avoid durable orchestration claims.
2. **Weak cancellation test**: a test that cancels before any work starts does
   not prove sibling cancellation. Include a parallel branch test where one
   branch fails and another observes context cancellation.
3. **Noisy report timestamps**: returning raw `workreport.Report` would expose
   dynamic timestamps. Use a stable response projection.
4. **Diagram review misses**: scenario and sequence diagrams are connector
   heavy. Generate PNG/SVG pairs, run deterministic geometry gate summaries,
   and inspect rendered PNGs before PR.

