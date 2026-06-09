# Lesson: Compensation workflow examples need original-error and cleanup evidence

## Context

Issue #71 adds a request-scoped `workflow` + `workreport` example that demonstrates
compensation after a later fulfillment step fails.

## Learned

- Compensation examples should separate the forward failure from cleanup
  failures. The response needs an `original_error` field and a nested report tree
  so callers can see why the workflow failed and which cleanup steps ran.
- Reverse cleanup order is part of the contract. Tests should assert the exact
  compensation child order, not only the final HTTP status.
- A request-scoped example can use in-memory side-effect flags, but the README
  must state that durable storage, idempotent external commands, retry policy,
  and outbox/workflow-engine support are required for production compensation.
- For README diagrams, Graphviz final renders with semantic line colors are a
  better fit for this flow than rigid grid layouts because forward, failure, and
  reverse paths are easier to distinguish.

## Reuse

Apply the same checks to future workflow examples that model rollback-style or
saga-like behavior: preserve the original error, report cleanup failures, assert
cleanup order, and document the durability boundary.
