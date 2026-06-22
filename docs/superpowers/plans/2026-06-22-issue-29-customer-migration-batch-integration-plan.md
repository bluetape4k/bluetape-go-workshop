# Customer Migration Batch Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `examples/customer-migration-batch-integration`, a runnable v0.5.0 workshop example that composes checkpoint restart, retry/dead-letter behavior, a Gin operations API, a cancel operation, and a leader-guarded scheduled tick/loop so PR text can close #42, #43, #75, and umbrella #29.

**Architecture:** Keep one example-local `internal/customermigration` package. The batch engine owns source records, checkpoint replay, retry/skip policy, idempotent sink writes, fixed chunk size `2`, and report projection. The service owns run lifecycle behind `sync.Mutex`, active-run guarding, defensive public snapshots, cancelation, leader gating, scheduler loop orchestration, and Gin handlers. Checkpoint/sink/dead-letter stores own their own locks and expose snapshot APIs for race-safe active-run reads. `main.go` only wires local loopback server configuration, demo leader mode, timeouts, signal handling, and graceful shutdown.

**Tech Stack:** Go, `github.com/bluetape4k/bluetape-go/batch`, `github.com/bluetape4k/bluetape-go/leader`, Gin, standard-library `context`, `errors`, `net/http`, `sync`, `time`, `os/signal`, and existing repo Make targets. No new dependencies.

---

## Constraints

- Apply `$bluetape-go-patterns` to every Go code task: context-first APIs, sentinel error wrapping, race-safe shared state, table-driven tests, `gofmt`, and no package-level mutable state.
- Preserve example-local boundaries. Do not import sibling example `internal` packages.
- Write tests before implementation for each behavioral surface.
- Keep HTTP DTOs ID/count based. `/batch/status` and `/batch/report` must not expose fixture email values.
- Keep server local-only by default: `127.0.0.1:8095`. `HTTP_ADDR` may override only to a loopback bind; reject non-loopback binds by default. Do not trust forwarded headers for security decisions.
- Keep docs bilingual where the repo already does: add example README pair and update root `README.md` / `README.ko.md`.
- No diagrams, infrastructure services, queues, databases, Redis, NATS, new scheduler framework, or new dependencies in this pass.

## Planned Files

- `examples/customer-migration-batch-integration/main.go`
- `examples/customer-migration-batch-integration/main_test.go`
- `examples/customer-migration-batch-integration/README.md`
- `examples/customer-migration-batch-integration/README.ko.md`
- `examples/customer-migration-batch-integration/internal/customermigration/engine.go`
- `examples/customer-migration-batch-integration/internal/customermigration/engine_test.go`
- `examples/customer-migration-batch-integration/internal/customermigration/service.go`
- `examples/customer-migration-batch-integration/internal/customermigration/service_test.go`
- `examples/customer-migration-batch-integration/internal/customermigration/scheduler.go`
- `examples/customer-migration-batch-integration/internal/customermigration/scheduler_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-customer-migration-batch-integration.md`
- `docs/review/2026-06-22-issue-29-batch-integration-code-review.md`

## Implementation Tasks

- [ ] **A. Engine TDD red tests [complexity: medium]**
  - Create `engine_test.go` with tests for:
    - manual run with `CrashAfterNewWrites: 3` fails with `ErrWriterCrash`;
    - failure leaves checkpoint at `NextIndex: 2`;
    - the default engine uses fixed `ChunkSize: 2`;
    - partial second chunk writes `cust-1003`;
    - restart run resumes from checkpoint, accepts replayed `cust-1003` as duplicate no-op, writes `cust-1005`, dead-letters `cust-1004`, and completes at `NextIndex: 5`;
    - transient `cust-1003` increments retry count and succeeds;
    - permanent `cust-1004` records exactly one dead letter and increments skip count;
    - invalid checkpoint and caller cancellation return stable sentinel/diagnostic behavior;
    - report projection has no timestamps and no email values.
  - Run `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration` and keep the expected compile/fail output as TDD evidence.

- [ ] **B. Engine implementation [complexity: medium]**
  - Implement default records, `FailureScenario`, sentinel errors, `RunOptions`, `RunResponse`, `Summary`, report DTOs, in-memory checkpoint store, sink, dead-letter store, reader, processor, writer, and `RunBatch`.
  - Give checkpoint store, sink, and dead-letter store their own locks plus defensive snapshot APIs. Status/report may read immutable latest projections under `Service.mu`; if they need live store state while active they must use these locked snapshot APIs.
  - Use `batch.Step` with `ChunkSize: 2`, `batch.Job`, `batch.CheckpointReader`, `batch.RetryErrors(3, ...)`, and `batch.SkipErrors(2, ...)`.
  - Make writer idempotent by customer ID. Track `accepted_ids`, `new_written_ids`, `migrated_ids`, and `duplicate_skip_count` separately from upstream report `WriteCount`.
  - Propagate `context.Context`; do not retry caller cancellation/deadline.
  - Run `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`.

- [ ] **C. Service and HTTP TDD red tests [complexity: high]**
  - Create `service_test.go` with tests for:
    - `GET /healthz`;
    - `POST /batch/start` crash response, stable status code, and latest status snapshot;
    - `POST /batch/schedule/tick` under held leadership restarts and completes;
    - missing leadership returns `not_leader` without mutating checkpoint, sink, or dead letters;
    - status/report expose stable latest-run projections and omit emails;
    - every public response body across success, crash, not-leader, malformed JSON, cancellation, no-active cancel, and report-not-found paths omits fixture email strings and uses allowlisted public error messages;
    - malformed JSON, blank/invalid run IDs, invalid crash counters, and oversized bodies on both POST endpoints return stable error codes;
    - no latest report returns `report_not_found`;
    - `POST /batch/cancel` cancels active work, returns `202 Accepted` with `status="cancel_requested"` and no `error_code`, clears active state after the run observes cancellation, and returns `no_active_run` when repeated with no active run;
    - request cancellation maps to `request_cancelled`;
    - cancellation releases the active-run guard;
    - deterministic concurrent manual start, scheduled tick, cancel, status, and report requests allow at most one active run and return defensive snapshots;
    - bounded mixed-access stress uses at least 8 goroutines and 25 iterations per goroutine, asserts at most one active run, and repeats the exact handler-visible checkpoint/sink/dead-letter snapshot paths under normal and race tests;
    - deterministic leader cases cover held, missing, `leader.ErrAlreadyLeader`, campaign cancellation before acquisition, request cancellation after acquisition, resign timeout, and resign failure diagnostics. Campaign cancellation must preserve `context.Canceled`/`context.DeadlineExceeded`, map HTTP to `request_cancelled`, avoid `not_leader`, and avoid batch mutation.
  - Use a latchable runner hook so tests overlap active work without sleeping.
  - Run `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`.

- [ ] **D. Service, leader gate, and Gin implementation [complexity: high]**
  - Implement `Service` with `mu`, `active`, active-run cancel function, shared locked checkpoint store, locked sink, locked dead-letter store, latest response, latest report, `last_rejection_code`, and deterministic test hooks.
  - Implement `StartManual`, `RunScheduledTick`, `CancelActiveRun`, `Status`, `Report`, validation helpers, defensive-copy helpers, allowlisted public error messages, and stable error mapping.
  - Implement `LeaderGate` with `RunIfLeader` and `LeaderHeld`; include deterministic in-memory gate for tests and a production-shaped adapter around `leader.Elector`.
  - Treat `leader.ErrAlreadyLeader` as held leadership without taking ownership of a newly acquired lease. Resign only when this call acquired leadership.
  - Use a fixed resign cleanup timeout over `context.WithoutCancel(ctx)` or equivalent bounded cleanup context so cleanup is attempted even after caller cancellation and cannot hang indefinitely.
  - Implement `NewRouter` with `gin.New`, recovery middleware, no trusted forwarded headers, capped 8 KiB JSON body decoding, shared error response writer, and routes:
    - `GET /healthz`
    - `POST /batch/start`
    - `POST /batch/schedule/tick`
    - `POST /batch/cancel`
    - `GET /batch/status`
    - `GET /batch/report`
  - Call and check `router.SetTrustedProxies(nil)`. Add a forwarded-header spoofing regression test if any handler surfaces client IP.
  - Run `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`.

- [ ] **E. Scheduler and main entrypoint TDD/implementation [complexity: medium]**
  - Create `scheduler_test.go` proving an injectable ticker loop triggers leader-guarded runs, stops on context cancellation, and uses manual test ticks without sleeps.
  - Implement the tiny scheduler loop around `Service.RunScheduledTick`; keep durable queue semantics out of scope.
  - Create `main_test.go` that verifies `newHTTPServer` sets `Addr`, handler, `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`.
  - Add bind parsing tests for default `127.0.0.1:8095`, loopback `HTTP_ADDR`, invalid `HTTP_ADDR`, and rejected non-loopback `HTTP_ADDR`.
  - Implement `main.go` with default bind `127.0.0.1:8095`, loopback-only `HTTP_ADDR`, `LEADER_MODE=held|missing`, `http.Server`, SIGINT/SIGTERM handling, and bounded graceful shutdown.
  - Run `go test -count=1 ./examples/customer-migration-batch-integration/...`.

- [ ] **F. Documentation [complexity: medium]**
  - Add English and Korean example READMEs with scenario, run command, curl commands for manual crash, status, report, leader-held scheduled restart, active-run cancel, not-leader rejection, malformed JSON, blank run ID, and oversized body.
  - Document checkpoint key, chunk size, endpoint table, `/healthz` as process liveness only, `/batch/status` as operator diagnosis/readiness, leader-guarded tick and tiny scheduler loop, retry/dead-letter policy, restart duplicate boundary behavior, relation to focused #41/#73/#74 examples, Ctrl-C shutdown, port collision, loopback-only `HTTP_ADDR`, `LEADER_MODE=held|missing`, process restart reset, and production hardening.
  - Update root `README.md` and `README.ko.md` example tables and v0.5.0 run section.

- [ ] **G. Focused verification [complexity: medium]**
  - Run:
    - `go test -count=1 ./examples/customer-migration-batch-integration/...`
    - `go test -race -count=1 ./examples/customer-migration-batch-integration/...`
    - `go run ./examples/customer-migration-batch-integration`, then run the README curl smoke flow for `/healthz`, manual crash, `/batch/status`, `/batch/report`, `/batch/schedule/tick`, `/batch/cancel`, not-leader with `LEADER_MODE=missing`, malformed JSON, blank run ID, and oversized body. Record expected HTTP status and `error_code` where applicable.
    - Terminate the running process with SIGTERM/SIGINT, wait within the configured shutdown timeout plus a small margin, assert the process exits, and record whether any active run was canceled or allowed to finish.
  - Fix failures before broader verification.

- [ ] **H. Cleanup pass [complexity: medium]**
  - Because the implementation will touch more than three files, write a short cleanup checklist in the working notes before editing cleanup.
  - Prefer deletion and consolidation over new abstractions. Keep example code readable for workshop users.
  - Rerun the focused tests after cleanup.

- [ ] **I. Performance/stability scan [complexity: medium]**
  - Confirm no long sleeps, unbounded goroutines, unbounded body reads, leaked contexts, lock-held batch execution, or caller-canceled resign cleanup.
  - Confirm concurrency/stress tests exercise status/report snapshots during active runs.
  - Run the package race test again if cleanup changed service state.

- [ ] **J. Full repository verification [complexity: high]**
  - Run:
    - `go test -p 1 ./...`
    - `make fmt-check`
    - `make tidy-check`
    - `make vet`
    - `make lint`
    - `GOFLAGS=-p=1 make ci`
    - `git diff --check`
  - Record failures exactly and fix in scope.

- [ ] **K. Step 6-R code review and fixes [complexity: high]**
  - Run six-lane code review plus integration review per `bluetape4k-full-feature`.
  - Save `docs/review/2026-06-22-issue-29-batch-integration-code-review.md`.
  - Fix all P0/P1 findings and rerun affected review lanes plus relevant tests.

- [ ] **L. Lessons, commit, and PR [complexity: medium]**
  - Add `docs/lessons/2026-06-22-customer-migration-batch-integration.md` with pitfalls, commands, review findings, and follow-up risks.
  - Commit using Lore protocol.
  - Push branch and create a PR with `Closes #29`, `Closes #42`, `Closes #43`, and `Closes #75`.
  - Match PR assignee, milestone `0.5.0`, and labels from the linked issue set when GitHub permits it.
  - End PR body with the required `## DoD Status` section.

## Acceptance Criteria Mapping

| Spec requirement | Plan coverage |
|---|---|
| Checkpoint crash/restart and duplicate replay | A, B, C, D, G |
| Gin operations API | C, D, F, G |
| Active-run cancel operation | C, D, F, G |
| Leader-guarded scheduled tick and scheduler loop | C, D, E, F |
| Retry/dead-letter policy | A, B, F |
| Shared state race safety | C, D, G, I, J |
| Local HTTP trust boundary and timeouts | D, E, F, G |
| Error code stability | C, D, F |
| No email exposure in all public HTTP responses | A, C, D, F |
| Bilingual docs and root README updates | F |
| No new dependencies | B, D, J |
| Full verification and review | G, H, I, J, K |

## Risk Assumptions

- The example is process-local and in-memory; process restart resets checkpoint, migrated sink, and dead letters by design.
- The leader adapter is production-shaped but tests use deterministic fakes because the workshop example does not start a distributed coordinator.
- `GOFLAGS=-p=1 make ci` may take longer than focused package checks because the repository includes Testcontainers examples; run it after targeted checks pass.
- Non-loopback HTTP binds are rejected by default because the operations API is unauthenticated; production exposure requires a separate auth/trusted-network design.
- PR merge is out of scope unless the user explicitly requests merge after review/CI.
