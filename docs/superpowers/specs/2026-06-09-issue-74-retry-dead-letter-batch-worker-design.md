# Issue #74 Design: Retry and Dead-Letter Batch Worker Example

## Classification

- Work type: Type A - Full Feature.
- Basis: new runnable example directory, Go implementation, tests, English and
  Korean README files, generated README diagrams, root navigation updates,
  review artifacts, and PR.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `feat/issue-74-retry-dead-letter-batch-worker` under
  `.worktrees/feat-issue-74-retry-dead-letter-batch-worker`.

## Goal

Add a focused v0.5.0 batch example that demonstrates bounded retry,
classification of transient versus permanent work-item failures, and
domain-visible dead-letter handling without introducing a durable queue.

## Non-Goals

- Do not add Gin, background scheduling, leader election, NATS, Redis, or a
  database.
- Do not implement a generic queue framework.
- Do not add a new retry library or custom retry loop outside `batch.Step`.
- Do not claim the in-memory queue or dead-letter list is production durable.
- Do not skip failed writer chunks with checkpointing.

## Example

- Path: `examples/retry-dead-letter-batch-worker`
- Package: `internal/ticketworker`
- Runnable entrypoint: `main.go`
- Package dependency focus: `batch`, with timestamp-free report projection.

## Scenario

A support operations team runs a batch worker over queued ticket notifications.
Most tickets process once. One ticket fails transiently on the first attempt and
succeeds on retry. One ticket is permanently invalid because its delivery
address is blocked; the processor records a dead-letter entry with ticket ID,
classification, attempts, and reason, then returns a skippable permanent error.

The completed batch report shows:

- all input tickets were read;
- valid tickets were written;
- one permanent ticket was skipped;
- retry count reflects only the transient retry;
- dead-letter records preserve the skipped item and reason.

## Domain Model

```go
type Ticket struct {
    ID       string
    Channel  string
    Address  string
    Scenario FailureScenario
}

type ProcessedTicket struct {
    ID      string `json:"id"`
    Channel string `json:"channel"`
    Address string `json:"address"`
    Attempts int   `json:"attempts"`
}

type DeadLetter struct {
    TicketID string `json:"ticket_id"`
    Reason   string `json:"reason"`
    Attempts int    `json:"attempts"`
}
```

`FailureScenario` is local to the example and supports deterministic fixture
behavior: success, transient-once, and permanent.

## Batch Design

Reader:

- reads a deterministic in-memory slice;
- validates non-empty ticket IDs during open;
- tracks the next index;
- checks `ctx.Err()` in `Open`, `Read`, and `Close`;
- records close state for tests.

Processor:

- validates ticket fields;
- tracks per-ticket attempts in memory;
- returns a transient sentinel error while attempts are below the configured
  transient-success threshold;
- records a `DeadLetter` before returning a permanent sentinel error;
- never retries `context.Canceled` or `context.DeadlineExceeded`;
- wraps sentinel errors with ticket context for `errors.Is` checks.

Writer:

- stores processed tickets in an in-memory sink;
- preserves processed order;
- rejects duplicate ticket IDs with a wrapped error;
- checks context before and during writes;
- records close state for tests.

Runner:

- creates a `batch.Step[Ticket, ProcessedTicket]` with:
  - `Name: "ticket-retry-dead-letter-worker"`
  - `ChunkSize: 2`
  - `RetryPolicy: batch.RetryErrors(3, errors.Is(err, ErrTransientTicket))`
  - `SkipPolicy: batch.SkipErrors(2, errors.Is(err, ErrPermanentTicket))`
- wraps the step in a `batch.Job` named `ticket-batch-worker`;
- returns a deterministic JSON projection without timestamps.

## CLI Output

`go run ./examples/retry-dead-letter-batch-worker` prints stable JSON similar to:

```json
{
  "job_name": "ticket-batch-worker",
  "chunk_size": 2,
  "status": "completed",
  "read_count": 4,
  "write_count": 3,
  "retry_count": 1,
  "skip_count": 1,
  "processed": ["ticket-1001", "ticket-1002", "ticket-1004"],
  "dead_letters": [
    {"ticket_id": "ticket-1003", "reason": "blocked address", "attempts": 1}
  ]
}
```

Exact field names may change during implementation, but the output must remain
timestamp-free and deterministic.

## Failure and Cancellation Contracts

- Transient ticket failure succeeds after one retry and increments
  `batch.Report.RetryCount`.
- Permanent ticket failure records a dead-letter entry, is skipped by
  `batch.SkipPolicy`, increments `SkipCount`, and does not stop the job.
- Exceeding the skip budget fails the batch and preserves the first permanent
  error.
- Duplicate writer output fails the batch because writer errors are not used as
  dead-letter events in this example.
- Caller cancellation returns `batch.StatusCancelled`, closes opened resources,
  does not retry cancellation, and does not create a dead-letter entry for a
  cancelled item.

## Diagrams

Generate README diagram assets under `docs/images/readme-diagrams/`:

- `retry-dead-letter-batch-worker-scenario`
- `retry-dead-letter-batch-worker-architecture`
- `retry-dead-letter-batch-worker-sequence`

README files embed PNG only. SVG files remain next to PNG files for review.
Graphviz `.dot`, `.plain`, `*-graphviz.svg`, and `*-graphviz.png` remain route
evidence. Final README SVG/PNG assets must use the decorated workshop baseline,
including outer frame, title/subtitle, content bands or panels, footer callout,
pastel cards, semantic connector colors, and concrete `margins=L/R/T/B` gate
output.

## Documentation

Add English and Korean README files that include:

- Example Scenario
- how to run the local batch demonstration
- retry and dead-letter policy table
- output shape
- Architecture
- Sequence Diagram
- relationship to #40 operations failure policies and #73 checkpoint/restart
- production hardening notes for durable queues, persistent DLT storage,
  idempotent side effects, backoff/jitter, metrics, alerting, and replay tools

Update root `README.md` and `README.ko.md`:

- example table row
- run section
- 0.5.0 roadmap wording if needed
- workshop example map diagram

## Tests

Focused tests must cover:

- successful demo completes with three processed tickets, one dead letter,
  retry count `1`, and skip count `1`;
- transient item succeeds after retry and keeps attempt evidence;
- permanent item is recorded exactly once in dead letters and skipped;
- skip budget exhaustion fails the batch and wraps `ErrPermanentTicket`;
- writer duplicate error fails and is not converted into a dead-letter record;
- cancellation before work returns `batch.StatusCancelled`, closes resources,
  and does not write or dead-letter anything;
- cancellation during transient retry returns `batch.StatusCancelled` and does
  not retry caller-owned cancellation;
- report projection omits runtime timestamps;
- `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`.

## Validation

- `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`
- `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`
- `go run ./examples/retry-dead-letter-batch-worker`
- `go test -run '^$' ./examples/retry-dead-letter-batch-worker`
- `git diff --check`
- `golangci-lint cache clean && make ci`

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Target repository confirmed | Done | `bluetape4k/bluetape-go-workshop`, worktree path recorded. |
| User intent and boundary clear | Done | Implement #74 focused retry/dead-letter batch example. |
| Current source evidence checked | Done | Issue #74/#29, `bluetape-go v0.5.1` batch/workreport APIs, #73/#40 examples. |
| Non-goals recorded | Done | No durable queue, DB, Gin, scheduler, or custom retry framework. |
| Error and cancellation contracts explicit | Done | Transient, permanent, skip exhaustion, writer error, cancellation. |
| README diagram requirements explicit | Done | PNG embeds, SVG siblings, Graphviz evidence, decorated baseline, margin gate. |
| Test expectations explicit | Done | Success, failure, cancellation, projection, race validation. |
