# Issue 39 Fulfillment Workflow Runner Research

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #39 `[v0.4.0] Add fulfillment workflow runner example`
- Milestone: `0.4.0`
- Package focus: `github.com/bluetape4k/bluetape-go/workflow`,
  `github.com/bluetape4k/bluetape-go/workreport`

## Current Repository Evidence

- Root README states public HTTP API examples use Gin when the framework is part
  of the lesson.
- `examples/order-lifecycle-state-api` is the current 0.4.0 predecessor and
  uses this shape:
  - `main.go`
  - `README.md`
  - `README.ko.md`
  - `internal/<domain>/server.go`
  - `internal/<domain>/server_test.go`
- The repository currently depends on `github.com/bluetape4k/bluetape-go
  v0.5.1`, which includes the released 0.4.0 `workflow` and `workreport`
  packages.
- CodeGraph was initialized in the feature worktree and reported 35 files, 484
  nodes, and 1,081 edges before design work.

## bluetape-go Package Evidence

Local module cache source for `github.com/bluetape4k/bluetape-go@v0.5.1` shows:

- `workflow.Sequential(name, policy, works...)` runs work in input order.
  `StopOnFailure` stops at the first failed or partial child. Cancelled or
  aborted child reports always stop the sequence.
- `workflow.Parallel(name, policy, works...)` starts all work items with a
  shared cancellable context and preserves child reports in input order.
  `StopOnFailure`, aborted, and cancelled child reports cancel siblings and
  wait for started goroutines.
- `workflow.Conditional(name, predicate, trueWork, falseWork...)` evaluates one
  predicate and runs exactly one selected branch.
- Work functions receive `context.Context` and return `workreport.Report`.
- `workreport.Report` exposes terminal statuses:
  `completed`, `failed`, `partial`, `aborted`, and `cancelled`.
- `workreport.StopOnFailure` and `workreport.ContinueOnFailure` provide the
  failure policy model for aggregation.

GNO retrieval found `bluetape-go` PR #142, `feat: add workflow runners`, as the
source PR for the package behavior. The local dependency source is used as the
implementation authority for this workshop example.

## Issue Requirements

Issue #39 requires:

- A workflow runner example for sequential, parallel, and conditional steps.
- Fulfillment modeled as:
  - validate
  - reserve inventory and authorize payment in parallel
  - conditional shipment creation
- Cancellation and failure propagation visible in tests.
- Tests for success, step failure, conditional skip, and cancellation.
- README documentation for workflow step boundaries and failure semantics.
- Root README navigation under 0.4.0.
- Scenario-shaped example, not a thin API snippet.
- Gin for public HTTP API examples.
- English and Korean README files kept in sync.

## Adopted Direction

Add `examples/fulfillment-workflow-runner`, a Gin API that runs one in-memory
fulfillment workflow per request. The request body selects scenario inputs:

- order identifier
- stock availability
- payment authorization result
- shipment requirement

The server returns a stable JSON view of the `workreport.Report` tree. The
example should show the reader how sequential, parallel, and conditional
workflow runners compose ordinary Go functions while preserving cancellation and
failure semantics.

## Rejected Directions

- Durable workflow engine simulation: rejected because `workflow` is explicitly
  lightweight and non-durable.
- Mutable shared workflow context map: rejected because `workflow` documents
  ordinary closures and explicit inputs instead.
- Testcontainers or external services: rejected because the issue focuses on
  runner semantics, not infrastructure integration.
- A pure domain-only example without HTTP: rejected because the issue and
  workshop direction require Gin for public API examples.

## Validation Targets

- `go test -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
- `bash scripts/generate-fulfillment-workflow-diagrams.sh`
- `git diff --check`
- `make ci`

