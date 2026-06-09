# Issue #74 Plan: Retry and Dead-Letter Batch Worker Example

## Scope

- Issue: #74, `[v0.5.0] Add retry and dead letter batch worker example`
- Spec:
  `docs/superpowers/specs/2026-06-09-issue-74-retry-dead-letter-batch-worker-design.md`
- Research:
  `docs/superpowers/research/2026-06-09-issue-74-retry-dead-letter-batch-worker-research.md`
- Worktree:
  `.worktrees/feat-issue-74-retry-dead-letter-batch-worker`
- Branch: `feat/issue-74-retry-dead-letter-batch-worker`

## Ordering Constraints

- Commit research, spec, spec review, plan, and plan review before
  implementation.
- Apply `bluetape-go-patterns` to Go code, tests, README examples, and review
  gates.
- Apply `bluetape4k-diagram` to README diagram generation and visual
  inspection.
- Use existing `batch` APIs from `bluetape-go v0.5.1`; do not add runtime
  dependencies.
- Keep all work in the feature worktree.

## Tasks

### 1. Commit Planning Artifacts

- complexity: low
- expected files:
  - research, spec, spec review, plan, plan review under `docs/superpowers/`
- action:
  - Run `git diff --check`.
  - Commit planning artifacts with Lore-format trailers before Step 4.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Implement Ticket Worker Domain

- complexity: high
- expected files:
  - `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`
- action:
  - Define `Ticket`, `ProcessedTicket`, `DeadLetter`, result DTOs, sentinel
    errors, and deterministic fixture.
  - Implement `TicketReader`, `TicketProcessor`, `TicketWriter`, in-memory
    processed sink, and dead-letter store.
  - Use `errors.Is`-compatible wrapping for transient/permanent/duplicate
    errors.
  - Check `context.Context` in reader, processor, writer, and runner paths.
  - Keep output deterministic and timestamp-free.
- verification:
  - `gofmt -w examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`
  - `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`

### 3. Implement Batch Runner and Main

- complexity: medium
- expected files:
  - `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`
  - `examples/retry-dead-letter-batch-worker/main.go`
- action:
  - Build `batch.Step[Ticket, ProcessedTicket]` with chunk size `2`.
  - Configure `RetryPolicy` for transient errors with `MaxAttempts=3`.
  - Configure `SkipPolicy` for permanent errors with `MaxSkips=2`.
  - Wrap in `batch.Job` named `ticket-batch-worker`.
  - Print indented JSON from `main.go`.
- verification:
  - `go run ./examples/retry-dead-letter-batch-worker`
  - `go test -run '^$' ./examples/retry-dead-letter-batch-worker`

### 4. Add Focused Tests

- complexity: high
- expected files:
  - `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker_test.go`
- action:
  - Test completed demo counts, processed order, retry count, skip count, and
    dead-letter contents.
  - Test transient success after retry with attempt evidence.
  - Test permanent failure records one dead-letter and is skipped.
  - Test skip budget exhaustion fails and wraps `ErrPermanentTicket`.
  - Test duplicate writer failure is not converted into dead-letter.
  - Test cancellation before work and during retry.
  - Test report projection omits runtime timestamps.
- verification:
  - `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`
  - `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`

### 5. Generate README Diagrams

- complexity: medium
- expected files:
  - `scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.dot`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.plain`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*-graphviz.svg`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*-graphviz.png`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.svg`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.png`
- action:
  - Create scenario, Architecture, and Sequence Diagram assets.
  - Preserve decorated workshop baseline and semantic route colors.
  - Print geometry summaries with concrete margins.
  - Inspect each rendered PNG before commit/PR.
- verification:
  - `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
  - rendered PNG inspection with `view_image`
  - `git diff --check`

### 6. Write Example README Pair

- complexity: medium
- expected files:
  - `examples/retry-dead-letter-batch-worker/README.md`
  - `examples/retry-dead-letter-batch-worker/README.ko.md`
- action:
  - Add language switch, scenario, run command, policy table, sample output,
    Architecture, Sequence Diagram, and production hardening notes.
  - Link #40 and #73 as nearby concepts.
  - Embed PNG diagrams only.
- verification:
  - `rg -n "Example Scenario|Architecture|Sequence Diagram|retry|dead-letter" examples/retry-dead-letter-batch-worker/README.md`
  - `rg -n "예제 시나리오|Architecture|Sequence Diagram|retry|dead-letter" examples/retry-dead-letter-batch-worker/README.ko.md`

### 7. Update Root Navigation

- complexity: low
- expected files:
  - `README.md`
  - `README.ko.md`
  - `docs/images/readme-diagrams/workshop-example-map.*`
- action:
  - Add the new example table row and run command.
  - Regenerate or patch the workshop example map with Graphviz evidence and
    final PNG/SVG assets.
- verification:
  - `rg -n "retry-dead-letter-batch-worker" README.md README.ko.md`
  - visual inspection of `workshop-example-map.png`

### 8. Step 6-R Review and Verification

- complexity: medium
- expected files:
  - `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-code-review.md`
  - `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-verifier-checklist.md`
- action:
  - Run local 7-Tier Go review over the full diff.
  - Apply `bluetape-go-patterns` P0/P1 gate.
  - Fix all P0/P1 findings before PR.
- verification:
  - review artifact reports `P0=0 P1=0`

### 9. Final Validation, Commit, Push, PR

- complexity: medium
- expected files:
  - PR body ending with `## DoD Status`
- action:
  - Run full validation command set.
  - Commit with Lore-format trailers.
  - Push branch and create PR linked to #74.
  - Verify PR body and GitHub CI.
- verification:
  - `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
  - `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`
  - `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`
  - `go run ./examples/retry-dead-letter-batch-worker`
  - `go test -run '^$' ./examples/retry-dead-letter-batch-worker`
  - `git diff --check`
  - `golangci-lint cache clean && make ci`
  - `gh pr checks`

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Dead-letter behavior hides batch skip semantics | Tests assert both domain dead-letter records and `batch.Report.SkipCount`. |
| Retry loop is accidentally reimplemented | Runner must use `batch.RetryPolicy`; tests assert `RetryCount`. |
| Cancellation is retried or dead-lettered | Cancellation tests assert `StatusCancelled`, no retry, no DLT record. |
| Writer errors are misclassified as dead letters | Duplicate writer test must fail the batch and leave DLT unchanged. |
| README diagrams repeat prior visual defects | Generator prints concrete margin gate output and PNGs are visually inspected. |

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Every spec requirement maps to a task | Done | Code, tests, docs, diagrams, navigation, review, validation. |
| Dependencies ordered correctly | Done | Planning artifacts precede implementation; domain before tests/docs. |
| Concrete verification commands named | Done | Targeted, race, run, diagram, diff, CI commands listed. |
| Public docs covered | Done | EN/KO example README and root README pair. |
| Scope bounded to approved spec | Done | No durable queue, DB, Gin, scheduler, or new dependency. |
