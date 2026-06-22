# Issue #29/#75 Design: Customer Migration Batch Integration Example

## Classification

- Work type: Type A - Full Feature.
- Basis: issue #29 is the v0.5.0 batch umbrella; open child issues #42, #43,
  and #75 require a Gin operations API, leader-guarded scheduled execution, and
  a milestone-level integration example.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `feat/issue-29-batch-integration` under
  `.worktrees/feat-issue-29-batch-integration`.

## Goal

Add one runnable v0.5.0 customer migration batch integration example that
combines checkpoint restart, a Gin operations API, leader-guarded scheduled
execution, retry/dead-letter behavior, deterministic status output, and
English/Korean README walkthroughs.

This PR should be able to close #42, #43, #75, and then #29 because the focused
checkpoint/retry prerequisites #41, #73, and #74 are already implemented.

## Current Evidence

- `gh issue view 29` shows #29 as the umbrella for checkpoint, restart, report,
  scheduled execution, retries, and integration.
- `gh issue view 42` requires Gin start/status/report handlers while keeping
  batch policy outside handlers.
- `gh issue view 43` requires a small scheduler loop guarded by existing leader
  primitives, with held/missing/cancellation tests and no long sleeps.
- `gh issue view 75` requires an integration example that composes checkpoint
  restart, operations API, scheduled execution, retry/dead-letter behavior, and
  import fixtures.
- Existing examples provide the local patterns:
  - `examples/account-migration-checkpoint-restart` for checkpoint restore.
  - `examples/chunked-csv-import-checkpoint` for replay after a failed chunk.
  - `examples/retry-dead-letter-batch-worker` for `batch.RetryPolicy` and
    `batch.SkipPolicy`.
  - `examples/operations-report-policy` and `examples/order-fulfillment-integration`
    for Gin handler shape, status mapping, and stable report projection.
  - `examples/leader-coordination-jobs` for leader-owned scheduled work.
- `go doc github.com/bluetape4k/bluetape-go/batch` confirms `Step`, `Job`,
  `CheckpointReader`, `CheckpointStore`, `RetryErrors`, and `SkipErrors`.
- `go doc github.com/bluetape4k/bluetape-go/leader` confirms the existing
  `leader.Elector` contract and sentinel errors.
- `go doc github.com/gin-gonic/gin.Engine` confirms `gin.New`,
  `ServeHTTP`, and standard `net/http` integration used by repository tests.

## Non-Goals

- Do not add durable queues, Redis, NATS, databases, or object storage.
- Do not implement a generic scheduler, queue worker, retry framework, or
  checkpoint storage framework.
- Do not import existing focused example `internal` packages across example
  boundaries; Go `internal` package visibility intentionally prevents that.
- Do not claim in-memory checkpoint stores, leader fakes, or dead-letter lists
  are production durable.
- Do not add new dependencies.

## Approaches Considered

### A. One integrated Gin example directory

Create `examples/customer-migration-batch-integration` with a single internal
package that owns the domain engine, in-memory operations service, leader-gated
scheduler, Gin handlers, tests, CLI entrypoint, and README pair.

This is the selected approach. It keeps the milestone example runnable,
reviewable, and faithful to #75 while satisfying #42 and #43 in the same
scenario.

### B. Three separate example directories for #42, #43, and #75

This would keep each child issue maximally focused, but it would duplicate
batch fixtures and delay the milestone integration. It also makes #75 a thin
wrapper instead of the user-visible milestone example.

Rejected because the repository already has focused examples for checkpoint
and retry, and the remaining useful gap is integration.

### C. Import focused example packages into the integration example

This would avoid duplicate domain concepts, but the existing focused examples
place code under `examples/<name>/internal/...`. A sibling example cannot import
those packages without violating Go's `internal` visibility rule.

Rejected because preserving example-local boundaries is preferable to moving
existing packages or broadening public API surface.

## Example

- Path: `examples/customer-migration-batch-integration`
- Package: `internal/customermigration`
- Runnable entrypoint: `main.go`
- HTTP framework: Gin
- Default address: `127.0.0.1:8095`
- Package dependency focus: `batch`, `leader`, and standard-library
  `context`, `net/http`, `sync`, and `time`.
- Chunk size: `2`, fixed in the example so the restart demo leaves the
  checkpoint at `NextIndex=2` after the simulated crash.

## Scenario

A customer migration service exposes an operations API and a leader-guarded
scheduled trigger. Each run imports deterministic customer records in chunks,
stores a small checkpoint cursor, retries one transient customer enrichment
failure, dead-letters one permanent customer, and can restart after a simulated
writer crash without reprocessing the completed chunk.

Default records:

- `cust-1001`: succeeds.
- `cust-1002`: succeeds.
- `cust-1003`: fails transiently once, then succeeds on retry.
- `cust-1004`: permanent validation failure; records a dead letter and is
  skipped.
- `cust-1005`: succeeds after restart.

The demo flow has two visible runs:

1. A manual API start with `crash_after_new_writes=3` fails after the first
   checkpointed chunk and a partial second chunk.
2. A scheduler tick under leader guard starts a restart run with the same
   checkpoint store and sink. It restores the checkpoint, replays the failed
   chunk, treats the already-written boundary customer as an idempotent no-op,
   finishes remaining work, and reports the final checkpoint.

## Domain Model

Customer source:

```go
type CustomerRecord struct {
    Index    int
    ID       string
    Email    string
    Segment  string
    Scenario FailureScenario
    Reason   string
}
```

Migrated customer:

```go
type MigratedCustomer struct {
    ID       string `json:"id"`
    Email    string `json:"email"`
    Segment  string `json:"segment"`
    Attempts int    `json:"attempts"`
}
```

Checkpoint:

```go
type Checkpoint struct {
    NextIndex int `json:"next_index"`
}
```

Dead letter:

```go
type DeadLetter struct {
    CustomerID string `json:"customer_id"`
    Reason     string `json:"reason"`
    Attempts   int    `json:"attempts"`
}
```

## Batch Engine Contract

- `RunBatch(ctx, options)` creates a `batch.Step[CustomerRecord, MigratedCustomer]`
  and wraps it in a `batch.Job`.
- Reader implements `batch.CheckpointReader`.
- Processor validates records, normalizes email/segment, retries only
  `ErrTransientCustomer`, and dead-letters only `ErrPermanentCustomer`.
- Writer stores migrated customers idempotently by customer ID and can simulate
  a crash after a configured count of new writes.
- Duplicate writes at a replay boundary are deterministic no-ops, not fatal
  duplicate errors. The response records duplicate skip counts separately so
  restart behavior is visible without failing the batch.
- Checkpoint store is replaceable through `batch.CheckpointStore`; the example
  uses an in-memory recording store.
- `batch.Step` is configured with `ChunkSize: 2`. Tests must prove this
  default because the checkpoint/replay scenario depends on it.
- All reader, processor, writer, store, scheduler, and handler paths check or
  propagate `context.Context`.
- Caller-owned `context.Canceled` and `context.DeadlineExceeded` are not
  retried.
- Retry policy is fixed at `batch.RetryErrors(3, errors.Is(err, ErrTransientCustomer))`.
  No backoff or sleeping is used in this workshop example; cancellation before
  or during processor work returns `batch.StatusCancelled` and is not retried.
- Skip policy is fixed at `batch.SkipErrors(2, errors.Is(err, ErrPermanentCustomer))`.
  Skip exhaustion fails the run and exposes a stable `permanent_customer`
  diagnostic code.

Sentinel errors:

- `ErrInvalidCustomer`
- `ErrTransientCustomer`
- `ErrPermanentCustomer`
- `ErrWriterCrash`
- `ErrDuplicateCustomer`
- `ErrInvalidCheckpoint`
- `ErrRunInProgress`
- `ErrNotLeader`
- `ErrInvalidRunID`
- `ErrInvalidCrashAfter`

Errors returned from domain logic wrap sentinels with `%w` for `errors.Is`.

## Operations Service State Contract

`Service` owns the run lifecycle and all public snapshots:

- `mu sync.Mutex`
- `active bool`
- shared checkpoint store with its own lock and snapshot API
- migrated-customer sink with its own lock and snapshot API
- dead-letter store with its own lock and snapshot API
- latest run response
- latest report projection
- active run cancel function, when a run is in progress

Run entrypoints (`StartManual` and `RunScheduledTick`) acquire `mu`, reject when
`active` is true, create a run-scoped cancelable context, set `active=true`,
store the cancel function, then release the lock while executing the batch. A
`defer` reacquires `mu`, stores defensive-copy snapshots, clears the cancel
function, sets `active=false`, and records the latest report even on
cancellation or failure.

Read entrypoints (`Status` and `Report`) acquire `mu` only long enough to return
defensive copies of immutable latest projections and high-level active state.
They must not read live store internals while a batch is mutating them unless
they use the stores' locked snapshot APIs. HTTP DTOs never expose aliased
slices, maps, checkpoint values, dead-letter values, or migrated-customer
values.

`CancelActiveRun` cancels the currently active manual or scheduled run and
returns a stable cancellation response. If no run is active it returns
`404 Not Found` with a dedicated `no_active_run` error for cancel requests.

Tests must use a latchable writer or runner hook so concurrent manual start,
scheduled tick, status, and report requests overlap deterministically. Add a
bounded stress test with at least 8 goroutines and 25 iterations per goroutine
that repeats mixed manual start, scheduled tick, status, report, and cancel
access, proves at most one run is active, proves public snapshots are valid, and
then reruns the same package under `go test -race`.

## Operations API Contract

### `GET /healthz`

Returns `200 OK` with `{"status":"ok"}`.

### `POST /batch/start`

Request:

```json
{
  "run_id": "manual-001",
  "crash_after_new_writes": 3
}
```

Behavior:

- Validates `run_id` as 1..64 characters of `A-Z`, `a-z`, `0-9`, `_`, `.`, and
  `-` after trimming whitespace.
- `crash_after_new_writes` is optional. Omitted or zero means no crash. Positive
  values must be in `1..len(default records)`. Negative or out-of-range values
  return `400 Bad Request`.
- Rejects a second active run with `409 Conflict`.
- Starts and completes the local batch synchronously for deterministic workshop
  output.
- Returns status `200 OK` for completed runs, `409 Conflict` for failed runs,
  and `408 Request Timeout` for caller cancellation.
- All JSON POST handlers share one capped decoder path. They cap request bodies
  at 8 KiB and return `413 Request Entity Too Large` for oversized requests.

### `POST /batch/schedule/tick`

Request:

```json
{
  "run_id": "scheduled-001"
}
```

Behavior:

- Checks leadership through the configured leader gate before running.
- If leadership is held, starts one batch run and returns the same response
  shape as `/batch/start`.
- If leadership is missing, returns `409 Conflict` with a stable
  `not_leader` error body.
- Does not loop or sleep; tests and README curls trigger exactly one
  deterministic tick.
- Uses the same 8 KiB capped JSON decoder and oversized-body error mapping as
  `/batch/start`.

### `POST /batch/cancel`

Request:

```json
{
  "reason": "operator requested stop"
}
```

Behavior:

- Cancels the active run, if one exists.
- Returns `202 Accepted` as a successful cancellation-request acknowledgement
  with `status="cancel_requested"` and no `error_code`.
- Returns `404 Not Found` with `no_active_run` when no run is active.
- Uses the same 8 KiB capped JSON decoder and oversized-body error mapping as
  the other POST handlers.

### `GET /batch/status`

Returns latest run status, checkpoint, migrated customer IDs, dead letters,
leader state, rejection code, and whether a run is currently active.

### `GET /batch/report`

Returns the latest timestamp-free batch report projection. If no run exists,
returns `404 Not Found`.

## Scheduler and Leader Contract

The example uses a tiny acquisition-oriented `LeaderGate` interface around the
existing `leader.Elector` shape:

```go
type LeaderGate interface {
    RunIfLeader(ctx context.Context, run func(context.Context) (RunResponse, error)) (RunResponse, error)
    LeaderHeld() bool
}
```

The production-shaped adapter can wrap `leader.Elector` by calling
`Campaign(ctx)`, running the callback only while leadership is held, and
resigning with a bounded cleanup context, such as `context.WithTimeout` with a
small fixed timeout over `context.WithoutCancel(ctx)` or an equivalent bounded
cleanup context that survives caller cancellation. `leader.ErrAlreadyLeader` is
treated as held leadership and runs the callback without taking ownership of a
newly acquired leadership lease; the adapter resigns only when this call
successfully acquired leadership. Resign failure is recorded in the response
diagnostics but does not hide the original batch result. Tests use a
deterministic in-memory gate that records campaign and resign counts and reports
held/missing state through `LeaderHeld`.

The scheduler owns a tiny injectable ticker loop for the runnable demo. It
executes one leader-guarded tick per interval until its context is canceled, and
tests use a manual ticker channel to avoid sleeps. The scheduler owns no durable
queue semantics. The explicit `/batch/schedule/tick` endpoint remains for
deterministic curl and test control.

Required tests:

- held leadership runs one tick;
- `leader.ErrAlreadyLeader` is treated as a runnable held-leadership path
  without an extra resign;
- missing leadership returns `ErrNotLeader` without mutating checkpoint, sink,
  or dead-letter state;
- campaign cancellation preserves `context.Canceled` or
  `context.DeadlineExceeded`, maps HTTP to `408 Request Timeout` with
  `request_cancelled`, does not set `not_leader`, and does not run the batch;
- resign cleanup is bounded, cannot hang indefinitely, and records cleanup
  failure without hiding the batch result;
- request cancellation after leadership acquisition still attempts bounded
  resign cleanup;
- scheduler loop stops on context cancellation and does not sleep in tests.

`GET /batch/status` includes `leader_held` and `last_rejection_code` so the
workshop user can distinguish idle, active, not-leader, and failed-run states.
The runnable `main.go` exposes a deterministic `LEADER_MODE=held|missing`
setting so README users can reproduce both successful scheduled runs and
`not_leader` responses.

## HTTP Trust Boundary and Server Contract

This is a local workshop server, not an authenticated operations plane.

- `main.go` binds to `127.0.0.1:8095` by default.
- `HTTP_ADDR` may override the address only to another loopback bind.
  Non-loopback binds such as `:8095` or `0.0.0.0:8095` are rejected by default
  because the operations API is unauthenticated.
- README files must state that widening the bind address requires an explicit
  trusted-network or authentication boundary.
- The Gin router must call and check `SetTrustedProxies(nil)` and must not trust
  forwarded headers for any security decision.
- The runnable `http.Server` must set `ReadHeaderTimeout`, `ReadTimeout`,
  `WriteTimeout`, and `IdleTimeout`.
- `main.go` must handle SIGINT/SIGTERM and call `Shutdown` with a bounded
  context.
- Tests cover request cancellation at handler level; smoke validation covers
  `go run` startup and graceful termination.

## HTTP Status and Response Shape

Successful and failed batch runs return a stable `runResponse`. HTTP responses
also include `error_code`, `error_message`, and `failed_phase` when the run did
not complete.

```json
{
  "run_id": "scheduled-001",
  "trigger": "schedule",
  "status": "completed",
  "checkpoint": {"next_index": 5},
  "read_ids": ["cust-1003", "cust-1004", "cust-1005"],
  "accepted_ids": ["cust-1003", "cust-1005"],
  "new_written_ids": ["cust-1005"],
  "migrated_ids": ["cust-1001", "cust-1002", "cust-1003", "cust-1005"],
  "dead_letters": [
    {"customer_id": "cust-1004", "reason": "blocked customer", "attempts": 1}
  ],
  "summary": {
    "read_count": 3,
    "write_count": 2,
    "retry_count": 1,
    "skip_count": 1,
    "duplicate_skip_count": 1,
    "failure": false
  },
  "report": {"name": "customer-migration-batch", "status": "completed"}
}
```

Report projection must omit runtime timestamps.

`summary.write_count` follows the upstream `batch.Report.WriteCount` for items
accepted by successful writer calls. `new_written_ids` is the smaller domain
delta for newly inserted customers, and `duplicate_skip_count` explains replayed
boundary records that were accepted as idempotent no-ops.

HTTP DTOs expose customer IDs and counts, not customer email values. Email is
kept inside the internal domain sink and CLI demo output only. Tests assert that
all public HTTP responses, including failed runs and error bodies, do not
include fixture email strings. Public `error_message` values are allowlisted
and must not be raw domain errors.

Stable API error codes:

| Condition | HTTP status | Code |
|---|---:|---|
| malformed JSON | 400 | `invalid_request` |
| invalid `run_id` | 400 | `invalid_run_id` |
| invalid `crash_after_new_writes` | 400 | `invalid_crash_after_new_writes` |
| oversized body | 413 | `request_too_large` |
| run already active | 409 | `run_in_progress` |
| scheduled tick without leadership | 409 | `not_leader` |
| simulated writer crash | 409 | `writer_crash` |
| retry exhaustion | 409 | `transient_customer_exhausted` |
| skip exhaustion | 409 | `permanent_customer` |
| no latest report | 404 | `report_not_found` |
| cancel without active run | 404 | `no_active_run` |
| caller cancellation/deadline or canceled run result | 408 | `request_cancelled` |

## Documentation

Add English and Korean README files with:

- example scenario;
- run command and curl smoke commands for manual crash, status, report,
  leader-held scheduled restart, active-run cancel, not-leader rejection,
  malformed JSON, blank run ID, and oversized body;
- checkpoint key and chunk size;
- API endpoint table;
- leader-guarded scheduler explanation;
- retry/dead-letter policy table;
- restart contract and duplicate boundary behavior;
- relationship to focused examples #41, #73, and #74;
- local runbook notes for Ctrl-C shutdown, port collision, and state reset by
  restarting the in-memory demo process;
- `HTTP_ADDR` loopback-only override and `LEADER_MODE=held|missing` demo
  controls;
- `/healthz` as process liveness only, with `/batch/status` as the operator
  diagnosis/readiness surface for active state, leadership state, checkpoint,
  latest failure, and rejection code;
- production hardening notes for durable checkpoint stores, queues, schedulers,
  database upserts, idempotency keys, auth/trusted-network boundaries, metrics,
  structured run lifecycle logs, and dead-letter replay.

Update root `README.md` and `README.ko.md`:

- example table row;
- 0.5.0 run section;
- roadmap wording if needed.

No new diagrams are required for this pass; the current work closes the
functional milestone gap. Existing root map imagery can be refreshed in a
follow-up if the project wants diagram parity for the new example.

## Tests

Focused tests must cover:

- health endpoint;
- manual start fails on simulated writer crash and leaves checkpoint at the
  previous successful chunk;
- default chunk size is `2`;
- scheduled tick under leadership restarts from checkpoint and completes;
- tiny scheduler loop triggers a leader-guarded run and stops on cancellation
  without long sleeps;
- completed first chunk is not reprocessed on restart;
- transient customer retry increments retry count and succeeds;
- permanent customer records exactly one dead letter and increments skip count;
- missing leadership rejects scheduled tick without mutating checkpoint or
  migrated customers;
- status and report endpoints return stable latest-run projections;
- malformed JSON, blank/invalid run IDs, invalid crash counters, and oversized
  request bodies on both POST endpoints return deterministic error responses;
- active-run cancellation endpoint cancels work, clears active state, and maps
  repeated/no-active cancellation to `no_active_run`;
- caller cancellation maps to `408 Request Timeout`;
- cancellation releases the active-run guard;
- concurrent manual start, scheduled tick, cancel, status, and report requests
  allow at most one active run, return defensive snapshots, and never race;
- bounded shared-state stress repeats concurrent manual start, scheduled tick,
  cancel, status, and report access under normal tests and then under the race
  detector;
- all public HTTP response bodies, including status/report, failed runs, and
  error responses, do not expose fixture email values;
- `main.go` constructs an `http.Server` with bounded timeouts and graceful
  shutdown;
- `HTTP_ADDR` accepts loopback binds and rejects non-loopback binds;
- Gin trusted proxies are disabled with `SetTrustedProxies(nil)`;
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`.

## Validation

- `go test -count=1 ./examples/customer-migration-batch-integration/...`
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`
- `go run ./examples/customer-migration-batch-integration`, then execute the
  README curl smoke flow for `/healthz`, manual crash, status, report,
  scheduled restart, cancel, not-leader, malformed JSON, blank run ID, and
  oversized body
- `go test -p 1 ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `GOFLAGS=-p=1 make ci`
- `git diff --check`

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Architecture pre-design ran or skipped | Done | Local pre-design selected one integrated example after comparing three approaches. |
| Step 1-R research incorporated | Done | Issue, GNO, current examples, `go doc`, and root README evidence are listed above. |
| Current-behavior claims cite evidence | Done | Each major dependency and existing pattern cites current files or command evidence. |
| Spec path confirmed inside worktree | Done | This file lives under `.worktrees/feat-issue-29-batch-integration/docs/superpowers/specs/`. |
| Risks/failure modes included | Done | Cancellation, retry/dead-letter, checkpoint replay, leader missing, and active-run conflict are explicit. |
| Approach comparison included | Done | Approaches A/B/C compared and B/C rejected with repository rationale. |
| Brainstorming process | Done | User gave concrete "작업하자" execution direction and AGENTS autonomy forbids permission handoff; material design is captured here instead of stopping for approval. |
| Go pattern compliance | Done | Context, sentinel errors, race/stress, Gin boundaries, and README impact are specified. |
| Open questions resolved | Done | No blocking ambiguity remains; durable infrastructure and diagrams are explicitly out of scope. |
