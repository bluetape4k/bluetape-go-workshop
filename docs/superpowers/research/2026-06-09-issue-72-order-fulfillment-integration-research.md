# Issue #72 Research: Order Fulfillment Workflow Integration Example

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #72 `[v0.4.0] Add order fulfillment workflow integration example`
- Milestone: `0.4.0`
- Work type: Type A - Full Feature

## Sources Checked

- GitHub issue #72.
- Closed issue #71 and PR #86 merge result for compensation workflow.
- Existing 0.4.0 examples:
  - `examples/order-lifecycle-state-api`
  - `examples/payment-authorization-state`
  - `examples/fulfillment-workflow-runner`
  - `examples/operations-report-policy`
  - `examples/compensation-workflow`
- Existing design/research/lesson artifacts:
  - `docs/superpowers/specs/2026-06-08-issue-39-fulfillment-workflow-runner-design.md`
  - `docs/superpowers/specs/2026-06-08-issue-71-compensation-workflow-design.md`
  - `docs/superpowers/research/2026-06-08-issue-71-compensation-workflow-research.md`
  - `docs/lessons/2026-06-08-fulfillment-workflow-runner.md`
  - `docs/lessons/2026-06-08-compensation-workflow.md`
- GNO `bluetape4k-docs` results for 0.4.0 state/workflow/workreport research.

## Current Evidence

The 0.4.0 workshop already has focused examples for the individual primitives:

- `order-lifecycle-state-api` demonstrates `state.Machine` transition legality,
  final states, guard rejection, and stable HTTP error mapping.
- `payment-authorization-state` demonstrates payment-specific state transitions
  plus application-layer idempotent replay.
- `fulfillment-workflow-runner` demonstrates request-scoped `workflow`
  composition with `Sequential`, `Parallel`, `Conditional`, cancellation, and
  stable `workreport.Report` projection.
- `operations-report-policy` demonstrates deterministic `workreport`
  aggregation and summary projection for `StopOnFailure` versus
  `ContinueOnFailure`.
- `compensation-workflow` demonstrates application-owned compensation stacks,
  reverse cleanup order, original-error preservation, and cleanup failure
  reporting.

Issue #72 asks for a milestone-level integration example, not another focused
primitive example. The new example should therefore show how these concepts fit
together in one order flow while keeping the code request-scoped and workshop
size.

## Design Decision

Build a new Gin example under `examples/order-fulfillment-integration`.

The example will create one request-scoped order run per HTTP request:

1. A local `state.Machine` owns order lifecycle states.
2. A `workflow.Sequential` runner owns step execution order.
3. Successful reversible steps register compensation handlers.
4. A compensation runner uses `workflow.Sequential` with
   `workreport.ContinueOnFailure` after a later workflow failure.
5. `workreport.Report` is projected into a stable JSON response with a summary.

This keeps the example close to production vocabulary while avoiding durable
orchestration, persistence, queues, external services, and background workers.

## Rejected Options

- Reusing existing example packages directly. They are intentionally example
  local and have different request/response shapes; importing them would turn
  the integration example into wrapper glue instead of teaching the integrated
  domain flow.
- Adding a durable saga engine or in-memory run store. The milestone primitive
  is request-scoped state/workflow behavior, not long-running orchestration.
- Adding database, queue, Redis, or Testcontainers dependencies. Issue #72 says
  keep external dependencies out unless package behavior requires them.

## Implementation Constraints

- Use Gin because issue #72 explicitly requires a public service example.
- Keep all mutable state request-scoped.
- Do not expose `workreport.Report` timestamps in JSON.
- Preserve original workflow errors when compensation also fails.
- Include diagram assets that follow `bluetape4k-diagram`:
  - final README PNG/SVG assets use the existing decorated workshop baseline
  - Graphviz `.dot`, `.plain`, and `*-graphviz.*` artifacts remain route
    evidence
  - generator prints concrete L/R/T/B margin evidence
  - each rendered PNG is visually inspected

## Test Implications

Focused tests should cover:

- health endpoint
- happy path reaches `shipped`
- invalid lifecycle transition maps to `409` with state/report evidence
- shipment failure after reversible steps maps to `409`, sets lifecycle to
  `cancelled`, and runs compensation in reverse order
- compensation failure preserves the original shipment error
- malformed JSON and invalid request fields map to `400`
- parallel requests do not share lifecycle or side-effect state
- race test over the example package
