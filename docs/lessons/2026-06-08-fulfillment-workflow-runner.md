# Fulfillment Workflow Runner Example

## Context

Issue #39 added the first v0.4.0 workflow runner HTTP example. The example had
to use Gin, exercise `bluetape-go/workflow`, cover sequential, parallel, and
conditional runners, and include bilingual README content with scenario,
architecture, and sequence diagrams.

## Decision

Keep the workflow request-scoped. The handler builds one runner per request,
passes the HTTP request context into the runner, and projects `workreport.Report`
into deterministic JSON without runtime timestamps. Avoid durable workflow
engine behavior, retry scheduling, databases, queues, and mutable workflow
context maps in this example.

## Outcome

- Added `examples/fulfillment-workflow-runner` with a runnable Gin main package.
- Added focused tests for success, conditional skip, inventory failure, payment
  failure with sibling cancellation, caller cancellation, and bad requests.
- Added bilingual README content that explains workflow boundaries, failure
  semantics, cancellation, and production hardening gaps.
- Added generated README diagrams for scenario, architecture, and sequence
  flows with PNG/SVG plus DOT, Plain, and Graphviz evidence.

## Verification

- `bash scripts/generate-fulfillment-workflow-diagrams.sh`
- `go test -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -run '^$' ./examples/fulfillment-workflow-runner`
- `git diff --check`
- `make ci`
- Individual rendered PNG inspection for the scenario, architecture, and
  sequence diagrams.

## Future Guard

For request-scoped workflow examples, keep cancellation ownership visible in
tests. If a failure is expected to cancel a slow sibling, make the test prove the
sibling entered cancellable work before the fast branch fails. For README
diagrams, inspect the rendered PNG after generator changes; the first draft had
a scenario title-gap issue and a sequence failure-label overlap that only showed
up in the rendered image.
