# Issue #41 Plan: Batch Migration Checkpoint Restart Example

## Scope

Implement `examples/account-migration-checkpoint-restart` as a focused 0.5.0
batch checkpoint/restart example.

## Tasks

### T1 - Example Package

- Add `internal/accountmigration`.
- Define account fixture, `MigrationCheckpoint`, result/report projections, and
  sentinel errors.
- Implement checkpoint-aware reader with `Restore` and `Checkpoint`.
- Implement processor with deterministic crash injection after the first
  checkpointed chunk.
- Implement target sink and writer with idempotency and duplicate detection.
- Implement `RunMigration` and `RunDemo`.

### T2 - Tests

- Add table/focused tests for:
  - first run fails after first chunk checkpoint,
  - restart resumes at `next_index=2` and completes,
  - completed first chunk is not re-read or re-written,
  - final checkpoint is `next_index=5`,
  - invalid checkpoint type/range wraps `ErrInvalidCheckpoint`,
  - duplicate target write wraps `ErrDuplicateAccount`,
  - cancellation before work and during processing,
  - timestamp-free report projection.
- Add bounded stress tests for:
  - concurrent complete migration runs with independent stores/sinks,
  - concurrent checkpoint store and target sink access.
- Stress tests must pass in normal `go test` and under `go test -race`.

### T3 - CLI

- Add `main.go` that prints deterministic indented JSON from `RunDemo`.
- Keep output free of runtime timestamps.

### T4 - README

- Add English and Korean README files.
- Include Example Scenario, Architecture, Sequence Diagram, checkpoint key,
  chunk size, restart contract, tests, related examples, and production
  hardening.

### T5 - Diagrams

- Add generator script:
  - `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`
- Generate scenario, architecture, and sequence assets:
  - DOT
  - PLAIN
  - Graphviz SVG/PNG
  - decorated final SVG/PNG
- Gate output must include concrete margins and zero geometry failures.
- Inspect every rendered PNG.

### T6 - Root Navigation

- Add the example to root `README.md` and `README.ko.md`.
- Update workshop example map and generated map assets.

### T7 - Review and Verification

- Add Step 6-R code review artifact and verifier checklist.
- Run:
  - `bash scripts/generate-account-migration-checkpoint-restart-diagrams.sh`
  - `go test -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`
  - `go test -race -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`
  - `go test -count=1 ./examples/account-migration-checkpoint-restart/...`
  - `go test -race -count=1 ./examples/account-migration-checkpoint-restart/...`
  - `go run ./examples/account-migration-checkpoint-restart`
  - `go test -run '^$' ./examples/account-migration-checkpoint-restart`
  - `go vet ./examples/account-migration-checkpoint-restart/...`
  - `golangci-lint run ./examples/account-migration-checkpoint-restart/...`
  - `git diff --check`
  - `golangci-lint cache clean && make ci`

### T8 - PR

- Commit with Lore trailers.
- Push branch.
- Create PR with `Closes #41`.
- Verify PR body is non-empty and final section is `## DoD Status`.
- Watch GitHub CI.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Every spec requirement mapped to task | Done | T1-T8 map implementation, tests, docs, diagrams, review, PR. |
| Task ordering valid | Done | Source, tests, CLI, docs/diagrams, review, validation, PR. |
| Race/stress planned | Done | T2 and T7 require normal and race stress runs. |
| README and localized docs covered | Done | T4 and T6. |
| Diagram gate covered | Done | T5 requires generator gate and PNG inspection. |
| Verification commands concrete | Done | T7 lists exact commands. |
