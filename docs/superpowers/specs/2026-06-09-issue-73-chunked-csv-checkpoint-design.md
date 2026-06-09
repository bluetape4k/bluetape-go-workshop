# Issue #73 Design: Chunked CSV Import Checkpoint Example

## Goal

Add a focused v0.5.0 batch example that demonstrates chunked CSV import,
checkpoint persistence, restart from the last successful chunk, and duplicate
prevention when a failed chunk is replayed.

## Non-Goals

- Do not add an HTTP service, Gin route, or background scheduler.
- Do not add a durable database, Redis, queue, or object storage checkpoint
  adapter.
- Do not implement a generic CSV import framework.
- Do not claim `batch.MemoryCheckpointStore` is production durability.
- Do not advance checkpoints after a partially failed writer chunk.

## Example

- Path: `examples/chunked-csv-import-checkpoint`
- Package: `internal/csvimport`
- Runnable entrypoint: `main.go`
- Fixture: `testdata/customers.csv`
- Package dependency focus: `batch`

## Scenario

A merchant uploads a customer CSV file. The import job reads rows in chunks of
two, validates and normalizes each customer, writes committed customers to a
domain sink, and stores a checkpoint after each successful chunk write.

The first run simulates a crash while writing the second chunk. One customer in
that chunk is already committed before the writer returns an error. Because the
chunk failed, the checkpoint remains at the end of the first successful chunk.
The restart run creates a fresh reader, restores the checkpoint, replays the
failed chunk, skips the already committed customer ID, writes the remaining
rows, and advances the checkpoint to the end of the file.

This teaches two linked contracts:

1. Checkpoints represent the next unread CSV row after committed work.
2. Writers must be idempotent at the batch boundary because a failed chunk can
   be replayed.

## Domain Model

CSV columns:

- `customer_id`
- `email`
- `tier`

Imported customer:

```go
type Customer struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Tier  string `json:"tier"`
}
```

Checkpoint:

```go
type Checkpoint struct {
    NextRow int `json:"next_row"`
}
```

`NextRow` is zero-based over parsed data rows, excluding the CSV header. A final
checkpoint of `5` means all five fixture rows have been read and committed.

## Batch Design

Reader:

- parses the fixture with `encoding/csv`
- validates the exact header
- trims fields
- tracks `nextRow`
- implements `batch.CheckpointReader`
- restores only from `Checkpoint`
- checks `ctx.Err()` in `Open`, `Read`, `Restore`, `Checkpoint`, and `Close`

Processor:

- validates non-blank customer ID, email, and tier
- normalizes email to lowercase and tier to lowercase
- returns wrapped errors that include row/customer context
- checks `ctx.Err()` before work

Writer:

- writes chunks into an in-memory customer sink
- treats `Customer.ID` as the idempotency key
- records duplicate skips separately from successful new commits
- can simulate a crash after a configured new commit count
- returns a wrapped `ErrSimulatedCrash` sentinel for restart tests
- checks `ctx.Err()` before writing and between chunk items

Runner:

- creates a `batch.Step[CSVRow, Customer]` with:
  - `Name: "chunked-customer-csv-import"`
  - `ChunkSize: 2`
  - `CheckpointStore: batch.NewMemoryCheckpointStore()` for the demo
  - `CheckpointKey: "customer-csv-import"`
- wraps the step in a `batch.Job` named `customer-csv-import`
- returns a stable result projection without timestamps

## CLI Output

`go run ./examples/chunked-csv-import-checkpoint` prints stable JSON showing the
failure and restart demonstration:

```json
{
  "checkpoint_key": "customer-csv-import",
  "chunk_size": 2,
  "first_run": {
    "status": "failed",
    "read_count": 4,
    "write_count": 2,
    "checkpoint": {"next_row": 2}
  },
  "restart_run": {
    "status": "completed",
    "read_count": 3,
    "write_count": 3,
    "checkpoint": {"next_row": 5}
  },
  "customers_imported": 5,
  "duplicate_skips": 1
}
```

Exact field names may change during implementation, but the output must stay
timestamp-free and deterministic.

## Failure and Cancellation Contracts

- Malformed CSV headers fail during reader open.
- Invalid customer rows fail during processing and wrap row/customer context.
- Simulated writer crash returns `ErrSimulatedCrash` and leaves the checkpoint
  at the previous successful chunk.
- Caller cancellation returns `batch.StatusCancelled`, closes opened resources,
  and does not save a later checkpoint.
- A nil context is accepted by upstream `batch.Step.Run` as background context,
  but this example should call it with explicit contexts in tests and runner
  code.

## Diagrams

Generate README diagram assets under `docs/images/readme-diagrams/`:

- `chunked-csv-import-checkpoint-scenario`
- `chunked-csv-import-checkpoint-architecture`
- `chunked-csv-import-checkpoint-sequence`

README files embed PNG only. SVG files remain next to PNG files for review.
Graphviz `.dot`, `.plain`, `*-graphviz.svg`, and `*-graphviz.png` remain route
evidence. Final README SVG/PNG assets must use the decorated workshop baseline,
not raw Graphviz output.

The generator must print deterministic geometry evidence including concrete
`margins=L/R/T/B` values and must fail before rendering if the documented
threshold is exceeded.

## Documentation

Add English and Korean README files that include:

- Example Scenario
- how to run the local batch demonstration
- output shape
- checkpoint key and chunk size
- why the failed chunk is replayed
- why the writer is idempotent by customer ID
- Architecture
- Sequence Diagram
- production hardening notes for durable checkpoint stores, transactions,
  database upserts, audit records, retry/dead-letter handling, and file schema
  evolution
- a link to issue #41 as the base checkpoint/restart track

Update root `README.md` and `README.ko.md`:

- example table row
- run section
- roadmap row for 0.5.0 batch examples
- workshop example map diagram

## Tests

Focused tests must cover:

- initial import completes, writes all fixture rows, and checkpoints `NextRow`
  to the row count
- simulated mid-run failure returns failed status, wraps `ErrSimulatedCrash`,
  commits the partial customer once, and keeps the checkpoint at the previous
  successful chunk
- restart creates a fresh reader, restores the checkpoint, replays the failed
  chunk, skips the duplicate customer ID, completes the file, and writes each
  customer exactly once
- malformed CSV header fails before writing
- invalid customer row fails with row context
- cancellation before or during processing returns `batch.StatusCancelled`,
  closes resources, and leaves checkpoint state unchanged
- `go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...`

## Validation

- `bash scripts/generate-chunked-csv-import-checkpoint-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go test -run '^$' ./examples/chunked-csv-import-checkpoint`
- `git diff --check`
- `golangci-lint cache clean && make ci`
- GitHub PR checks
