# Issue #73 Plan: Chunked CSV Import Checkpoint Example

## Work Type

Type A - Full Feature.

Reason: new runnable example directory, Go implementation, fixture, tests,
bilingual READMEs, generated diagrams, root navigation updates, review
artifacts, and PR.

## Implementation Tasks

1. Add `examples/chunked-csv-import-checkpoint/testdata/customers.csv`.
   - Five deterministic customer rows.
   - Header exactly `customer_id,email,tier`.
2. Add `examples/chunked-csv-import-checkpoint/internal/csvimport`.
   - Domain types: `CSVRow`, `Customer`, `Checkpoint`, result DTOs.
   - Sentinel errors for invalid CSV/header/row and simulated crash.
   - Stable JSON projection helpers that omit `batch.Report` timestamps.
3. Implement CSV reader.
   - Parse fixture with `encoding/csv`.
   - Validate header and field count.
   - Track `NextRow` over data rows.
   - Implement `batch.CheckpointReader`.
   - Check context in `Open`, `Read`, `Restore`, `Checkpoint`, and `Close`.
   - Record close state for tests.
4. Implement processor.
   - Trim customer ID, email, and tier.
   - Normalize email/tier to lowercase.
   - Reject blank fields with row context.
   - Check context before processing.
5. Implement in-memory idempotent writer/sink.
   - Customer ID is the idempotency key.
   - Track new commits, duplicate skips, write attempts, and committed order.
   - Simulate a crash after a configurable number of new commits.
   - Check context before and during writes.
   - Record close state for tests.
6. Implement runner.
   - Build `batch.Step[CSVRow, Customer]` with chunk size `2` and checkpoint
     key `customer-csv-import`.
   - Wrap step in `batch.Job`.
   - Provide a demo function that runs first failure plus restart over the same
     checkpoint store and sink.
   - Keep public result output deterministic and timestamp-free.
7. Add `examples/chunked-csv-import-checkpoint/main.go`.
   - Run the demo with `context.Background()`.
   - Print indented JSON.
   - Exit non-zero only if the demo setup itself fails unexpectedly.
8. Add focused tests.
   - initial import success writes all fixture rows and checkpoints final row.
   - mid-run simulated crash keeps checkpoint at previous successful chunk.
   - restart replays the failed chunk, records one duplicate skip, and commits
     each customer exactly once.
   - malformed header fails before writing.
   - invalid row fails with row context.
   - cancellation before work and during processing returns
     `batch.StatusCancelled`, closes any opened resources, and only preserves
     previously committed checkpoints.
   - race validation over the example package.
9. Add README files.
   - `examples/chunked-csv-import-checkpoint/README.md`
   - `examples/chunked-csv-import-checkpoint/README.ko.md`
   - Include Example Scenario, run command, sample output, checkpoint/chunk
     explanation, Architecture, Sequence Diagram, production hardening, and #41
     link.
10. Add `scripts/generate-chunked-csv-import-checkpoint-diagrams.sh`.
    - Generate `.dot`, `.plain`, `*-graphviz.svg`, `*-graphviz.png`.
    - Generate final decorated SVG/PNG assets for scenario, architecture, and
      sequence diagrams.
    - Print geometry gate output with concrete `margins=L/R/T/B`.
    - Validate font roles and forbidden UI fonts.
11. Update root navigation.
    - `README.md` example table, run section, 0.5.0 roadmap row.
    - `README.ko.md` equivalent Korean updates.
    - `workshop-example-map` diagram assets and route evidence.
12. Add Step 6-R review/verifier artifacts after implementation.
    - Code review findings with P0/P1/P2/P3 counts.
    - Verifier checklist with command evidence.

## Validation Plan

- `bash scripts/generate-chunked-csv-import-checkpoint-diagrams.sh`
- visual inspection of:
  - `docs/images/readme-diagrams/chunked-csv-import-checkpoint-scenario.png`
  - `docs/images/readme-diagrams/chunked-csv-import-checkpoint-architecture.png`
  - `docs/images/readme-diagrams/chunked-csv-import-checkpoint-sequence.png`
  - `docs/images/readme-diagrams/workshop-example-map.png`
- `go test -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go run ./examples/chunked-csv-import-checkpoint`
- `go test -run '^$' ./examples/chunked-csv-import-checkpoint`
- `git diff --check`
- `golangci-lint cache clean && make ci`
- GitHub PR checks

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Checkpoint semantics become off-by-one | Tests assert `NextRow=2` after the failed second chunk and `NextRow=5` after restart. |
| Domain-level commits are confused with `batch.Report.WriteCount` | Result DTO and tests track sink-level new commits and duplicate skips separately from report counts. |
| Writer failure silently duplicates a row on restart | Sink uses customer ID idempotency and tests assert committed IDs are unique. |
| Cancellation advances checkpoint too far | Cancellation tests assert checkpoints only reflect successful prior chunks. |
| Example overclaims durability | README production hardening states memory store is demo-only and durable stores/transactions are production work. |
| Diagram output repeats prior margin/decorator defects | Generator prints concrete margin values, validates font roles, emits Graphviz evidence, and final PNGs are inspected. |

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Every spec requirement maps to a task | Done | Tasks cover code, tests, README, diagrams, navigation, review, and validation. |
| Dependencies ordered correctly | Done | Fixture/domain come before runner/tests/docs; validation comes after generation. |
| Concrete verification commands named | Done | Targeted tests, race, run, diff, CI, and PR checks are listed. |
| User-visible docs covered | Done | EN/KO example README and root README updates are explicit. |
| Scope bounded to approved spec | Done | No HTTP, durable store, queue, scheduler, or new dependency tasks are included. |
