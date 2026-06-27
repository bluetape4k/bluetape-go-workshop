# DynamoDB conditional repository example

Issue: #61

## Decision

Use a focused DynamoDB repository example to teach conditional writes before
combining DynamoDB with S3 or SQS workflow orchestration. The example keeps the
application surface small: create a catalog item if absent, update its name only
when the caller has the current version, and query the tenant partition.

## Why

DynamoDB conditional write failures are not ordinary infrastructure failures.
They usually mean the business predicate was false: a row already exists, or the
caller is stale. If every caller assembles key attributes, condition
expressions, and conflict handling, the same consistency rule gets repeated and
eventually diverges.

The example therefore makes the repository own:

- `TENANT#...` / `ITEM#...` key construction;
- create-if-absent and expected-version condition expressions;
- `ErrConditionalConflict` as the application conflict signal;
- typed AWS SDK error preservation for diagnostics;
- tenant query shape with `pk` equality and `begins_with(sk, ITEM#)`.

## Verification shape

- Deterministic fake-client tests assert the exact `PutItem`, `UpdateItem`, and
  `Query` expressions.
- Conflict tests assert AWS SDK `ConditionalCheckFailedException` is still
  discoverable with `errors.As` after wrapping.
- Cancellation and validation tests prove caller-owned failures stop before the
  DynamoDB client boundary.
- README diagrams explain the ownership boundary separately from the runtime
  success, conflict, and query sequence.
- The Floci smoke test remains opt-in because it starts Docker-backed DynamoDB
  emulation and should run serially with other container suites.
