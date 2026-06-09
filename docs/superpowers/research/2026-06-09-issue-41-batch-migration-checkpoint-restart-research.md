# Issue #41 Research: Batch Migration Checkpoint Restart Example

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #41 `[v0.5.0] Add batch migration checkpoint restart example`
- Milestone: `0.5.0`
- Parent track: #29

## Current Evidence

- Worktree: `.worktrees/feat-issue-41-batch-migration-checkpoint-restart`
- Baseline: `origin/develop` at `a3529c5`
- Dependency: `github.com/bluetape4k/bluetape-go v0.6.0`
- CodeGraph: not initialized in this repository. Direct source inspection and GNO results are used as fallback evidence.

## Issue Requirements

Issue #41 asks for a scenario-shaped batch migration example that:

- processes account or catalog rows through reader/processor/writer chunks,
- persists checkpoints,
- crashes after a known chunk,
- restarts from the checkpoint,
- proves completed chunks are not incorrectly reprocessed,
- keeps checkpoint storage replaceable and small,
- documents chunk size, checkpoint key, and restart contract.

## bluetape-go Batch API

`batch.StepOptions` in v0.6.0 supports:

- `CheckpointStore batch.CheckpointStore`
- `CheckpointKey string`

`batch.CheckpointStore` is:

```go
type CheckpointStore interface {
    Load(context.Context, string) (any, bool, error)
    Save(context.Context, string, any) error
}
```

`batch.CheckpointReader` is:

```go
type CheckpointReader interface {
    Restore(context.Context, any) error
    Checkpoint(context.Context) (any, bool, error)
}
```

`batch.Step` restores checkpoint after reader/writer open. It saves a checkpoint
after filtered/skipped processor items and after successful chunk writes. If a
writer chunk is skipped while checkpointing is enabled, `ErrUnsafeWriterSkipCheckpoint`
prevents unsafe checkpoint advancement.

## Existing Workshop Examples

- `examples/chunked-csv-import-checkpoint`: CSV-focused companion example. It
  demonstrates partial writer side effect, replay from the last safe cursor, and
  idempotent duplicate handling.
- `examples/retry-dead-letter-batch-worker`: retry/dead-letter focused example
  with bounded stress tests for shared state and retry behavior.

Issue #41 should not duplicate the CSV file import scenario. It should teach the
base migration restart contract: after a successful chunk is checkpointed, a
later crash should restart from that checkpoint and should not re-read the
completed chunk.

## Proposed Example Shape

Example path:

- `examples/account-migration-checkpoint-restart`

Scenario:

- Five deterministic legacy accounts are migrated in chunks of two.
- First run writes the first chunk and saves checkpoint `next_index=2`.
- The processor then simulates a crash on `acct-1003`.
- Restart uses the same checkpoint store and target sink, restores `next_index=2`,
  and migrates `acct-1003` through `acct-1005`.
- The operation log proves `acct-1001` and `acct-1002` were not re-read or
  re-written during restart.

Checkpoint contract:

- `Checkpoint{NextIndex int}` only.
- `CheckpointKey = "account-migration-v1"`.
- Storage remains replaceable through `batch.CheckpointStore`; the example uses
  an in-memory recording store for visible demo output.

## Test Requirements

Required focused tests:

- restart resumes from `next_index=2`,
- completed first chunk is not reprocessed on restart,
- checkpoint value is small and typed,
- invalid checkpoint type/range fails visibly,
- cancellation returns cancelled status and does not advance checkpoint,
- report projection omits timestamps.

Required race/stress:

- bounded concurrent complete migrations over independent stores/sinks,
- bounded concurrent sink/store access to prove mutex protection and uniqueness,
- run stress coverage in normal `go test` and under `go test -race`.

## Documentation Requirements

- Example README pair: `README.md`, `README.ko.md`.
- Required sections: Example Scenario, Architecture, Sequence Diagram.
- PNG embeds only with matching SVG, DOT, PLAIN, and Graphviz evidence.
- Root `README.md`, `README.ko.md`, and workshop example map updated.

## Risks and Decisions

- Durable checkpoint persistence is out of scope; production hardening must name
  durable store, transactional writes, idempotent migration, audit, metrics, and
  replay tooling.
- No new dependencies are needed.
- This is a local batch example, not a Gin API. Gin remains for HTTP operations
  examples such as #42.
