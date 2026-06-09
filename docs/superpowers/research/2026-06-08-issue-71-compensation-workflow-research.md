# Issue #71 Research: Compensation Workflow Example

## Sources Checked

- GitHub issue #71: focused v0.4.0 compensation workflow example.
- GitHub issue #28: 0.4.0 umbrella track and sequencing.
- `examples/fulfillment-workflow-runner`: request-scoped Gin workflow runner.
- `examples/operations-report-policy`: stable `workreport.Report` projection and
  HTTP status mapping.
- `examples/payment-authorization-state`: payment-specific state and retry
  boundaries.
- `github.com/bluetape4k/bluetape-go@v0.4.0/workflow`: `Sequential`,
  `Parallel`, `Conditional`, and context-aware `Work`.
- `github.com/bluetape4k/bluetape-go@v0.4.0/workreport`: report statuses,
  failure policies, aggregation, and caller-visible errors.

## Current Package Evidence

- `workflow.Sequential` runs work functions in input order and stops according
  to `workreport.FailurePolicy`.
- `workreport.Report` preserves caller-visible `Err` values and child report
  order.
- `workflow` does not provide durable compensation or rollback semantics. A
  compensation example should therefore show an application boundary around the
  runner, not claim a built-in saga engine.

## Example Selection

The issue scenario is suitable because it composes the previous 0.4.0 examples:

- inventory reservation is a reversible side effect
- payment authorization is a reversible side effect
- shipment creation is a later step whose failure requires reverse compensation
- the report tree can preserve both the original failure and compensation
  outcomes

## Decision

Build `examples/compensation-workflow` as a Gin example.

The forward path uses `workflow.Sequential` for:

1. `reserve-inventory`
2. `authorize-payment`
3. `create-shipment`

Each successful reversible step registers one compensation handler. When a
later step fails, the example runs registered compensation handlers in reverse
order with `workflow.Sequential("compensation", workreport.ContinueOnFailure,
...)` so one compensation failure does not hide or stop later cleanup.

## Constraints

- Keep all side effects in request-scoped memory flags.
- Preserve the original forward error in the top-level response.
- Return a stable JSON report projection without timestamps.
- Document that production compensation needs durable state, idempotent
  compensators, retry policy, audit logging, and operator intervention paths.

