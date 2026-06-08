# Issue #40 Research: Work Report and Failure Policy Example

## Context

- Issue: `#40 [v0.4.0] Add work report and failure policy example`
- Parent: `#28 [Epic] v0.4.0 state and workflow examples`
- Target milestone theme: v0.4.0 state/workflow examples
- Workshop baseline: public HTTP API examples use Gin when the framework is part
  of the lesson.

## Current Repository Evidence

- `examples/order-lifecycle-state-api` already teaches the `state` package with a
  Gin API and stable transition responses.
- `examples/fulfillment-workflow-runner` already teaches `workflow` runners,
  request cancellation, branch cancellation, and report projection.
- `README.md` and `README.ko.md` list v0.4.0 as state/workflow examples and
  already include the state API plus fulfillment workflow runner.
- The next v0.4.0 gap is a focused example where `workreport.Report` and
  `workreport.FailurePolicy` are the primary lesson, not only an output shape of
  workflow runners.

## Library Evidence

Observed in `github.com/bluetape4k/bluetape-go@v0.5.1/workreport`:

- `FailurePolicy` supports `StopOnFailure` and `ContinueOnFailure`.
- `Aggregate(name, policy, children...)` returns:
  - `completed` when all children complete.
  - for `StopOnFailure`, the first non-completed child determines the parent
    status, error, reason, and the copied child slice is truncated through that
    failing child.
  - for `ContinueOnFailure`, all child reports are preserved and the parent is
    `partial` when any child is non-completed.
- `Report` constructors set runtime timestamps, so example HTTP responses should
  project a stable DTO and omit timestamps.
- Status vocabulary is `completed`, `failed`, `partial`, `aborted`, and
  `cancelled`. There is no dedicated `skipped` status; a caller-defined skip can
  be represented as `aborted` with a reason.
- Unknown policies return `ErrUnknownFailurePolicy` through
  `workreport.FailurePolicyError`.

## Scenario Decision

Build `examples/operations-report-policy`, a Gin API for an operations checklist
report. The API models a small release-readiness run:

1. `load-catalog-snapshot` completes when the request starts normally.
2. `validate-products` completes or fails depending on request input.
3. `notify-partner` can include a failed first attempt plus a completed retry,
   making retry evidence visible without pretending the aggregate is fully
   successful.
4. `refresh-search-index` completes or is recorded as caller-skipped using
   `aborted` with a reason.

The handler chooses `StopOnFailure` or `ContinueOnFailure` from request input and
uses `workreport.Aggregate` directly. This keeps the example centered on report
aggregation semantics rather than workflow runner mechanics.

## Rejected Directions

- Durable job engine: out of scope for v0.4.0 example and would hide the package
  lesson behind persistence mechanics.
- Wrapping `workflow.Sequential`: already covered by `fulfillment-workflow-runner`;
  #40 should make `workreport.Aggregate` and failure policy behavior explicit.
- Reporting retry as full success after one failed attempt: misleading for the
  current `workreport` aggregate model because preserved failed attempts should
  keep the parent partial.
- Adding observability dependencies: issue requests report output suitable for
  logs/dashboards without requiring an observability stack.

## Acceptance Mapping

- Deterministic work report output: project reports to a stable JSON DTO without
  timestamps; assert exact status/name/summary fields in tests.
- Retry behavior: include nested `notify-partner` attempt reports and assert the
  failed first attempt plus completed retry are preserved.
- Skip behavior: expose skipped index refresh as `aborted` with a reason and
  count it in the summary.
- Fail-fast behavior: `StopOnFailure` truncates later reports after the first
  failed child.
- Documentation: English/Korean READMEs include scenario, architecture, sequence
  diagram, report fields, and production hardening gaps.
