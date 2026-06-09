# Issue 41 Code Review

Branch: `feat/issue-41-batch-migration-checkpoint-restart`
Scope:
- `examples/account-migration-checkpoint-restart/**`
- `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`
- `docs/images/readme-diagrams/account-migration-checkpoint-restart-*`
- root `README.md`, `README.ko.md`, and `workshop-example-map` assets

CodeGraph note: `CodeGraph not initialized in /Users/debop/work/bluetape4k/bluetape-go-workshop`; structural impact was reviewed from the branch diff, changed imports, and direct call surface.

## Tier Findings

| Tier | Focus | P0 | P1 | P2 | P3 | Result |
|---|---|---:|---:|---:|---:|---|
| 1 | Security | 0 | 0 | 0 | 0 | No network, auth, secrets, SQL, deserialization, or user-controlled trust boundary introduced. |
| 2 | Ops/SRE reliability | 0 | 0 | 0 | 0 | Context cancellation is checked in reader, processor, writer, and checkpoint store; deterministic report exposes restart evidence. |
| 3 | Structural impact | 0 | 0 | 0 | 0 | New standalone example only; no shared package API or module dependency change. |
| 4 | Go code quality | 0 | 0 | 0 | 0 | Uses `batch.StepOptions`, typed errors, context propagation, defensive copies, and mutex-protected shared stores. |
| 5 | Tests/types/silent failure | 0 | 0 | 0 | 0 | Tests assert checkpoint cursor, restart read IDs, invalid checkpoint failures, cancellation, deterministic projection, concurrency, and stress. |
| 6 | Performance/stability | 0 | 0 | 0 | 0 | No unbounded production goroutines, retries, polling, or hot-path reflection; bounded stress covers shared state. |
| 7 | Docs/release/evidence | 0 | 0 | 0 | 0 | English/Korean README and scenario/architecture/sequence diagrams added; root map and run docs updated. |

Final blocker gate: `P0 = 0`, `P1 = 0`.

## Evidence

- Restart contract: `RunDemo` shares one checkpoint store and target store across first and restart runs, then records final checkpoint and checkpoint activity (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:127`).
- Step wiring: `RunMigration` creates a checkpoint-aware `batch.Step` with chunk size, `CheckpointStore`, and `CheckpointKey` (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:170`).
- Restore contract: `MigrationReader.Restore` validates `MigrationCheckpoint` and bounds `NextIndex` before moving the cursor (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:293`).
- Save contract: `MigrationReader.Checkpoint` returns the next unread source index (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:312`).
- Shared state safety: `TargetAccountStore` and `RecordingCheckpointStore` protect maps and operation logs with mutexes (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:368`, `examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:496`).
- Restart assertion: tests prove the restart reads `acct-1003..acct-1005` and does not reprocess `acct-1001` or `acct-1002` (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:16`).
- Invalid checkpoint assertion: tests cover wrong type, negative index, and out-of-range index (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:56`).
- Concurrency/stress assertion: tests run concurrent demos and concurrent checkpoint/target store access (`examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:158`, `examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:190`).
- README coverage: scenario, Architecture, Sequence Diagram, checkpoint contract, test commands, and production hardening are documented (`examples/account-migration-checkpoint-restart/README.md:8`).
- Diagram gate: `scripts/generate-account-migration-checkpoint-restart-diagrams.sh` validates fonts, marker sizes, margins, and emits concrete gate lines (`scripts/generate-account-migration-checkpoint-restart-diagrams.sh:58`).

## Risk Pattern Scan

Command:

```bash
rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" examples/account-migration-checkpoint-restart
```

Result:
- `context.Background()` appears in CLI/test/demo call sites and `normalizeContext(nil)` fallback only.
- `go func` appears only in `TestConcurrentDemoRunsRemainDeterministic` and `TestStoresStressConcurrentAccess`.
- No `context.TODO`, `time.Tick`, `http.ListenAndServe`, `panic`, `RealIP`, or `X-Forwarded-For` hits.

## Validation Snapshot

- `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`: PASS
  - scenario: `nodes=8 routes=6 segments=9 margins=44/44/34/34 fontFallback=0`
  - architecture: `nodes=8 routes=8 segments=13 margins=44/44/34/34 fontFallback=0`
  - sequence: `nodes=10 routes=10 segments=10 margins=44/44/34/34 fontFallback=0`
- PNG visual inspection: PASS for scenario, architecture, sequence, and `workshop-example-map.png`.
- `go test -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`: PASS
- `go test -race -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`: PASS
- `go test -count=1 ./examples/account-migration-checkpoint-restart/...`: PASS
- `go test -race -count=1 ./examples/account-migration-checkpoint-restart/...`: PASS
- `go run ./examples/account-migration-checkpoint-restart`: PASS
- `go vet ./examples/account-migration-checkpoint-restart/...`: PASS
- `golangci-lint run ./examples/account-migration-checkpoint-restart/...`: PASS
- `git diff --check`: PASS
- `go test -run '^$' ./examples/account-migration-checkpoint-restart/...`: PASS
- `golangci-lint cache clean && make ci`: PASS
