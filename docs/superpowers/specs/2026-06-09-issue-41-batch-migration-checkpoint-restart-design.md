# Issue #41 Design: Batch Migration Checkpoint Restart Example

## Goal

Add a focused milestone 0.5.0 batch example that teaches checkpoint restore for
an account migration job. The example must prove that a completed chunk is not
reprocessed after restart.

## Non-Goals

- Durable queue processing.
- Distributed scheduling or leader election.
- HTTP/Gin operations API.
- Database-backed checkpoint storage.
- Partial writer side-effect replay; that is covered by
  `chunked-csv-import-checkpoint`.

## Example

Path:

- `examples/account-migration-checkpoint-restart`

Domain:

- `LegacyAccount`: deterministic input row.
- `TargetAccount`: normalized migrated row.
- `MigrationCheckpoint`: `{next_index}` cursor.
- `RecordingCheckpointStore`: small in-memory `batch.CheckpointStore` wrapper
  that records load/save activity for demo output and tests.
- `TargetAccountStore`: idempotent in-memory sink with write/read logs.

Constants:

- `JobName = "account-migration"`
- `StepName = "legacy-account-migration"`
- `DefaultCheckpointKey = "account-migration-v1"`
- `DefaultChunkSize = 2`

## Scenario Contract

Input accounts:

- `acct-1001`
- `acct-1002`
- `acct-1003`
- `acct-1004`
- `acct-1005`

First run:

- reads and writes `acct-1001`, `acct-1002`,
- saves checkpoint `next_index=2`,
- fails while processing `acct-1003`,
- leaves target store with two migrated accounts.

Restart run:

- loads checkpoint `next_index=2`,
- starts from `acct-1003`,
- writes `acct-1003`, `acct-1004`, `acct-1005`,
- saves final checkpoint `next_index=5`,
- proves `acct-1001`, `acct-1002` were not read or written again.

## API and Error Contract

Public package API should stay example-sized:

- `RunDemo(ctx context.Context) (DemoResult, error)`
- `RunMigration(ctx context.Context, options RunOptions) (MigrationRun, error)`
- constructors for reader/store/sink where tests need them.

Sentinel errors:

- `ErrInvalidAccount`
- `ErrMigrationCrash`
- `ErrInvalidCheckpoint`
- `ErrDuplicateAccount`

Errors returned from processor, reader restore, and writer must wrap sentinels
with `%w` so tests and callers can use `errors.Is`.

`nil` context is normalized to `context.Background()`.

## Checkpoint Contract

- The checkpoint value must be a typed `MigrationCheckpoint`.
- The stored value must include only `NextIndex`.
- `Restore` must reject wrong types and out-of-range cursor values.
- Checkpoint save must happen only through `batch.Step` after successful chunk
  writes or safe filtered/skipped items.
- `RunMigration` must accept any `batch.CheckpointStore`; the demo uses
  `RecordingCheckpointStore`.

## Concurrency and Stress Contract

The example has shared mutable state in the checkpoint store and target store,
so tests must include bounded stress coverage:

- concurrent complete demo runs with independent stores/sinks,
- concurrent store/sink access with uniqueness and ordering assertions,
- stress tests pass under normal `go test`,
- the same stress tests pass under `go test -race`.

## Documentation and Diagram Contract

Example README files must include:

- Example Scenario,
- Architecture,
- Sequence Diagram,
- checkpoint key and chunk size,
- restart contract,
- tests and production hardening.

Diagram assets:

- `account-migration-checkpoint-restart-scenario.{dot,plain,svg,png}`
- `account-migration-checkpoint-restart-architecture.{dot,plain,svg,png}`
- `account-migration-checkpoint-restart-sequence.{dot,plain,svg,png}`
- matching `*-graphviz.svg/png` evidence files.

Final README embeds must use PNG only.

## Acceptance Tests

Required tests:

- first run fails after checkpointed first chunk and restart completes,
- completed first chunk is not reprocessed on restart,
- final checkpoint is `next_index=5`,
- invalid checkpoint type and range fail with `ErrInvalidCheckpoint`,
- cancellation before work returns `batch.StatusCancelled` and leaves no writes,
- cancellation during processing returns cancelled status and does not retry,
- duplicate target write fails with `ErrDuplicateAccount`,
- report projection omits runtime timestamps,
- bounded stress normal/race coverage.

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Target repository confirmed | Done | `bluetape4k/bluetape-go-workshop` worktree on `origin/develop`. |
| Relevant memory/GNO searched | Done | GNO found bluetape-go #5/#30/#153 and workshop #73 artifacts. |
| User intent and boundaries clear | Done | User asked to continue next example; #41 is the next focused prerequisite before #75. |
| Existing APIs inspected | Done | `batch.Step`, `CheckpointReader`, `CheckpointStore`, `MemoryCheckpointStore`. |
| Race/stress expectations explicit | Done | Required in spec for shared state, checkpoint, ordering, and uniqueness contracts. |
| Diagram requirements explicit | Done | Scenario, Architecture, Sequence Diagram with PNG embeds and Graphviz evidence. |
