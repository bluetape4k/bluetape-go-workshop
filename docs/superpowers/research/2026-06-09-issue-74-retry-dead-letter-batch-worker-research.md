# Issue #74 Retry and Dead-Letter Batch Worker Research

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #74, `[v0.5.0] Add retry and dead letter batch worker example`
- Parent: #29, 0.5.0 batch checkpoint and restart workshop examples
- Dependency baseline: `github.com/bluetape4k/bluetape-go v0.5.1`

## Current Evidence

- #74 requires a focused example for retry policy and dead-letter handling in
  batch workers.
- #29 says durable queue semantics stay out of scope unless upstream batch
  support explicitly provides them.
- `batch.Step` in `bluetape-go v0.5.1` already exposes:
  - `RetryPolicy` for processor and writer failures.
  - `SkipPolicy` for processor item skips and writer chunk skips.
  - `RetryCount`, `SkipCount`, `ReadCount`, and `WriteCount` in `batch.Report`.
  - `ErrUnsafeWriterSkipCheckpoint` when writer chunk skipping would advance a
    checkpoint unsafely.
- `workreport.Report` is the existing workshop shape for named child outcomes,
  failure policies, stable status values, and timestamp-free projections.
- `examples/operations-report-policy` already teaches retry evidence and
  failure policies, but it does not use `batch.Step`.
- `examples/chunked-csv-import-checkpoint` already teaches checkpoint/restart,
  but it intentionally defers retry/dead-letter handling to later examples.
- GNO lookup found `bluetape-go` issue #30, which introduced retry, skip,
  checkpoint, and restart batch features.
- CodeGraph was not initialized for this repository or worktree, so source
  discovery used current repository files, `gh issue view`, `rg`, and
  `go list` module source paths instead.

## Adopted Direction

Create an in-memory support-ticket queue worker under
`examples/retry-dead-letter-batch-worker`.

The example will keep `batch` primitives visible:

- `TicketReader` feeds deterministic queued work items.
- `TicketProcessor` classifies outcomes:
  - transient item failure succeeds after bounded retries;
  - permanent item failure records a dead-letter entry and returns a skippable
    error;
  - context cancellation returns caller cancellation without retry.
- `TicketWriter` stores successfully processed ticket receipts.
- `batch.Step` owns `RetryPolicy`, `SkipPolicy`, and the final batch report.
- A stable result projection exposes processed receipts, dead letters, retry
  count, skip count, and a timestamp-free report tree.

## Rejected Options

| Option | Reason |
|---|---|
| Durable queue or database-backed dead-letter table | #29 keeps durable queue semantics out of scope; would distract from `batch` policy behavior. |
| Custom retry loop outside `batch.Step` | It would hide the upstream `batch.RetryPolicy` contract the milestone is meant to teach. |
| Writer-level dead-letter skip with checkpointing | `batch.ErrUnsafeWriterSkipCheckpoint` shows why failed writer chunks cannot be skipped safely when checkpoints are active. |
| New dependency for queues or scheduling | Not required for a focused local example and conflicts with the no-new-dependency default. |

## Design Notes

- Dead-letter entries are domain records created by the processor before it
  returns a permanent, skippable sentinel error.
- Retry is only for transient processor errors in this focused example.
- `batch.SkipPolicy` turns permanent item errors into item skips; the domain
  dead-letter list makes those skips inspectable.
- The example should not claim production durability. README hardening notes
  must call out durable queue, persistent dead-letter storage, idempotent
  handlers, observability, backoff, and alerting as production concerns.

## Validation Implications

- Tests must assert transient success after retry, permanent dead-letter
  capture, retry/skip counts, no retry for context cancellation, and stable
  report projection.
- Race validation is mandatory because the example owns shared in-memory sink
  state and dead-letter storage.
- README diagrams must follow the decorated workshop baseline and print
  concrete margin gate evidence.
