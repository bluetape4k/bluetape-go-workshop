# Issue #73 Research: Chunked CSV Import Checkpoint Example

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #73 `[v0.5.0] Add chunked CSV import checkpoint example`
- Milestone: `0.5.0`
- Work type: Type A - Full Feature

## Sources Checked

- GitHub issue #73.
- Related GitHub issues:
  - #29 `[v0.5.0] Add batch checkpoint and restart workshop examples`
  - #41 `[v0.5.0] Add batch migration checkpoint restart example`
- Current workshop examples and root README navigation.
- Upstream `github.com/bluetape4k/bluetape-go/batch` from module version
  `v0.5.1`:
  - `batch/checkpoint.go`
  - `batch/interfaces.go`
  - `batch/step.go`
  - `batch/job.go`
  - `batch/report.go`
  - `batch/step_test.go`
- Upstream batch research:
  - `bluetape-go/docs/research/2026-06-01-milestone-0.5.0-batch-research.md`
- GNO:
  - `gno query "issue 73 chunked CSV import checkpoint restart bluetape-go-workshop batch" -c bluetape4k-github --fast --no-rerank`
  - `gno query "0.5.0 batch checkpoint restart csv import chunk reader writer" -c bluetape4k-docs --no-rerank`
  - `gno query "issue 73 chunked CSV import checkpoint restart batch" -c wiki --no-rerank`

## Current Evidence

Issue #73 asks for a focused 0.5.0 CSV import example that processes a small
fixture in chunks, records the last successful checkpoint, fails mid-run, and
restarts without duplicating completed rows. It also requires English/Korean
README navigation and a README link to #41 as the base checkpoint/restart track.

Issue #29 is the 0.5.0 umbrella for batch processing examples and states:

- prove restart/checkpoint behavior with deterministic tests
- keep durable queue semantics out of scope unless upstream batch support
  explicitly provides them
- use Gin only at the HTTP operations boundary

Issue #41 remains open and describes the base checkpoint/restart example. Issue
#73 should therefore link to #41 as the base track, but must not claim #41 is
already implemented.

The workshop repository currently has no `examples/*batch*`, `*checkpoint*`,
or `*csv*` example. Existing milestone examples use:

- isolated `examples/<name>` directories
- `internal/<domain>` packages for implementation
- runnable `main.go`
- focused package tests plus race tests
- English/Korean README files
- decorated README diagrams under `docs/images/readme-diagrams`
- root `README.md` and `README.ko.md` navigation updates

## Upstream Batch API Evidence

`batch.Reader[T]` owns lifecycle methods:

- `Open(context.Context) error`
- `Read(context.Context) (T, bool, error)`
- `Close(context.Context) error`

`batch.Processor[I,O]` transforms or filters one item and receives context.

`batch.Writer[T]` persists a chunk with `Write(context.Context, []T) error`.

`batch.CheckpointReader` adds:

- `Restore(context.Context, any) error`
- `Checkpoint(context.Context) (any, bool, error)`

`batch.CheckpointStore` persists checkpoints by key and the in-memory
implementation is concurrency-safe for tests/local jobs.

`batch.Step.Run` restores a checkpoint before the read loop and saves the
reader checkpoint only after a processed item is filtered/skipped or after a
chunk write succeeds. This means a writer failure after partially committing a
chunk cannot safely advance the checkpoint; on restart, that failed chunk may be
read again. The example therefore needs an idempotent writer keyed by customer
ID to demonstrate duplicate prevention at the batch boundary.

`batch.Job` aggregates step reports and stops after the first failing step.
`batch.Report` exposes `Name`, `Status`, counts, `Err`, and child reports; it
also includes runtime timestamps that README/CLI output should not expose as
golden output.

## GNO Results

`bluetape4k-github` returned the upstream `bluetape-go` batch epic #5 and task
#30 as closed evidence for the package direction. It did not return workshop
issue #73 directly.

`bluetape4k-docs` returned:

- `bluetape-go/docs/research/2026-06-01-milestone-0.5.0-batch-research.md`
- Kotlin `bluetape4k-batch` design/plan documents that use the same
  reader/processor/writer, chunk, checkpoint, and restart vocabulary

The `wiki` collection returned no direct result for #73. Both GNO commands
emitted the local Metal backend warning
`ggml_metal_library_init_from_source: error compiling source`, but result sets
were still returned for the GitHub/docs collections.

## Design Decision

Build a new local runnable example under
`examples/chunked-csv-import-checkpoint`.

The example should be a local batch job, not an HTTP service. This follows issue
#29's rule to use Gin only at HTTP operations boundaries. The lesson is restart
and checkpoint semantics, not web routing.

The example will:

1. Read a deterministic customer CSV fixture.
2. Process rows through `batch.NewStep` with `ChunkSize=2`.
3. Persist checkpoint records in `batch.MemoryCheckpointStore` under a stable
   checkpoint key.
4. Simulate a writer crash after partially committing a chunk.
5. Restart with a fresh reader and writer over the same checkpoint store and
   customer sink.
6. Prove that completed rows are not duplicated and that the replayed failed
   chunk is idempotent by customer ID.

## Rejected Options

- HTTP API. Issue #73 is a batch example and #29 limits Gin to HTTP operations
  boundaries. Adding HTTP would distract from checkpoint semantics.
- Durable database/checkpoint store. The milestone API provides
  `CheckpointStore`; issue #73 only requires a small deterministic fixture.
  Durable stores belong to later integration examples.
- Advancing the checkpoint after a partial writer commit. The upstream step
  contract saves checkpoints after successful chunk writes. Advancing after a
  failed writer would risk silent data loss.
- Hiding `batch.Reader`, `batch.Processor`, `batch.Writer`, and
  `CheckpointReader` behind a generic import service. The workshop should make
  the batch package surface visible.

## Implementation Constraints

- Keep the fixture small, deterministic, and local to the example.
- Use `encoding/csv` and the upstream `batch` package; add no dependencies.
- Check context at reader, processor, writer, checkpoint, and runner boundaries.
- Close reader and writer resources on success, failure, and cancellation.
- Project reports into stable output without runtime timestamps.
- Keep duplicate prevention explicit and domain-shaped: customer ID is the
  idempotency key.
- README files must include:
  - Example Scenario
  - Architecture
  - Sequence Diagram
  - link to #41 as the base checkpoint/restart track
- Diagram assets must follow `bluetape4k-diagram`:
  - final README PNG/SVG assets use the decorated workshop baseline
  - Graphviz `.dot`, `.plain`, and `*-graphviz.*` artifacts remain route
    evidence
  - generator prints concrete L/R/T/B margin evidence
  - each rendered PNG is visually inspected

## Test Implications

Focused tests should cover:

- initial import succeeds and checkpoints the final CSV position
- simulated mid-run writer failure leaves the checkpoint at the previous
  successful chunk
- restart restores from the checkpoint, replays the failed chunk, skips the
  already partially committed customer ID, and completes all rows
- malformed CSV or invalid rows fail visibly
- caller cancellation returns a cancelled batch report and does not advance the
  checkpoint
- resources close on success/failure/cancellation
- race test over the example package
