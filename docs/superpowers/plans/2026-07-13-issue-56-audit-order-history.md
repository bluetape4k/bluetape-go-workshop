# Issue #56 Audit Order History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable order lifecycle example that atomically appends bluetape-go audit entries before updating an in-memory current-state projection.

**Architecture:** An internal `orderhistory.Service` owns explicit order transitions, a caller-injected `audit.Repository`, and a mutex-protected teaching projection. A preview layer executes a deterministic lifecycle and projects immutable audit entries into stable JSON; bilingual documentation explains why this is audit history rather than event sourcing.

**Tech Stack:** Go 1.25, bluetape-go v0.18.0 `audit`, standard library `context`, `encoding/json`, `sync`, `testing`.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/audit-order-history/internal/orderhistory/model.go` | Statuses, commands, order snapshots, validation sentinels and copies. |
| `examples/audit-order-history/internal/orderhistory/service.go` | Constructor, state machine, append-before-mutation boundary, history/query facade. |
| `examples/audit-order-history/internal/orderhistory/service_test.go` | Success, failure, cancellation, duplicate, defensive-copy, and concurrency proof. |
| `examples/audit-order-history/internal/orderhistory/preview.go` | Deterministic lifecycle and JSON-facing projections. |
| `examples/audit-order-history/internal/orderhistory/preview_test.go` | Preview content and determinism proof. |
| `examples/audit-order-history/main.go` | Build memory repository, run preview, print indented JSON. |
| `examples/audit-order-history/main_test.go` | Exact stdout JSON and failure propagation. |
| `examples/audit-order-history/README.md` | English lesson, commands, expected behavior, production boundaries. |
| `examples/audit-order-history/README.ko.md` | Korean parity documentation. |
| `README.md`, `README.ko.md` | Root example navigation. |
| `docs/lessons/2026-07-13-issue-56-audit-order-history.md` | Durable implementation lesson and proof. |

## Task 1: Define the domain and constructor contract

**Complexity:** Medium. **Depends on:** approved spec. **Write scope:** `model.go`, `service_test.go`, then `service.go`.

- [ ] **Step 1: Write failing constructor and validation tests**

Add table tests proving nil repository and invalid author wrap
`ErrInvalidConfig`, nil clock is accepted, `Service{}` methods do not panic,
IDs use `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`, and cancellation reasons require
valid UTF-8 and at most 256 runes.

```go
service, err := NewService(nil, Options{Author: "workshop"})
if !errors.Is(err, ErrInvalidConfig) || service != nil { t.Fatalf("...") }
```

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/audit-order-history/internal/orderhistory`

Expected: FAIL because the package and symbols do not exist.

- [ ] **Step 3: Implement minimal domain and constructor**

Define `StatusPending`, `StatusConfirmed`, `StatusShipped`, `StatusCancelled`,
the exact spec command types, `Order`, `Options`, the five service sentinels,
validation helpers, and `NewService`. Default a nil clock to UTC `time.Now`;
detect missing service dependencies in every operation.

- [ ] **Step 4: Run GREEN and format**

Run: `gofmt -w examples/audit-order-history/internal/orderhistory/*.go && go test -count=1 ./examples/audit-order-history/internal/orderhistory`

Expected: PASS for constructor/validation tests.

## Task 2: Implement append-before-mutation transitions

**Complexity:** High. **Depends on:** Task 1. **Write scope:** `service_test.go`, `service.go`.

- [ ] **Step 1: Write failing lifecycle tests**

Test create-confirm-ship and both cancellation paths. Assert current revision,
UTC timestamps, event types `order.created|confirmed|shipped|cancelled`, stable
command ID in `EventID` and `IdempotencyKey`, author, schema version, status
payload, and `ChangeMetadata` before/after attributes.

```go
created, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
if err != nil || created.Status != StatusPending || created.Revision != 1 { t.Fatalf("...") }
```

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(Lifecycle|Cancel)'`

Expected: FAIL because transition methods are absent.

- [ ] **Step 3: Implement minimal transition engine**

Validate context/input before locking, recheck context under the mutex, validate
the current state, build payload with `json.Marshal`, call
`audit.NewAggregateID`, `audit.NewDomainEvent`, `audit.NewChangeMetadata`, and
`audit.NewEntry`, then call `repo.Append`. Update the projection only after
append returns nil. Do not check cancellation between append success and the
infallible map assignment.

- [ ] **Step 4: Run GREEN**

Run: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(Lifecycle|Cancel)'`

Expected: PASS with revisions 1..3 and ordered history.

## Task 3: Prove failure atomicity, cancellation, and duplicate detection

**Complexity:** High. **Depends on:** Task 2. **Write scope:** `service_test.go`, minimal `service.go` fixes.

- [ ] **Step 1: Add failing negative-path tests**

Cover duplicate order, missing order, invalid and terminal transitions, reused
command ID across aggregates, repository failure, pre-canceled context, cancel
during append, and a wrapper repository that cancels immediately after a
successful append but returns nil. Assert history and current state remain
aligned in every case and repository errors preserve `errors.Is`.

```go
repo := &failingRepository{err: errRepositoryUnavailable}
_, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
if !errors.Is(err, errRepositoryUnavailable) { t.Fatalf("...") }
if _, ok := service.Current("order-1"); ok { t.Fatal("state mutated") }
```

- [ ] **Step 2: Run RED, repair, and rerun GREEN**

Run: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(Rejects|Repository|Cancellation|Duplicate)'`

Expected RED: at least one unimplemented failure contract. Expected GREEN after
the smallest service correction: all listed cases PASS.

## Task 4: Add bounded history/query and concurrency proof

**Complexity:** High. **Depends on:** Task 3. **Write scope:** `service_test.go`, `service.go`.

- [ ] **Step 1: Write failing query/copy/concurrency tests**

Prove absent history returns `(History{}, false, nil)`, no-match `Find` returns a
non-nil empty slice, default query limit returns exactly 20 of 25 stored entries
in repository order, limit above 100 and a non-order aggregate type wrap
`audit.ErrInvalidQuery`, revision windows/newest-first work, and returned history
is isolated. The 20-row cap bounds returned cardinality only: the v0.18 memory
repository still scans/copies O(total stored entries).

Add deterministic channel-barrier concurrency cases with no sleeps: 16
goroutines create 16 unrelated orders and must produce 16 successes, 16 current
snapshots, and 16 one-entry histories; after one order is created, 16 concurrent
unique confirm commands must produce exactly one success and 15
`ErrInvalidTransition` failures, with current revision 2 and contiguous history
revisions `[1,2]`. Use `sync.WaitGroup` and a closed start channel.

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(History|Find|Defensive|Concurrent)'`

Expected: FAIL until query normalization and read methods exist.

- [ ] **Step 3: Implement and run GREEN**

Delegate `History` to `LoadHistory`; clone current-state values; normalize
`Find.AggregateType` to `order`, default zero limit to 20, reject limits above
100, and delegate to the repository.

Run: `go test -count=1 ./examples/audit-order-history/internal/orderhistory`

Expected: PASS.

- [ ] **Step 4: Run focused race proof**

Run: `go test -count=20 ./examples/audit-order-history/internal/orderhistory -run '^TestServiceConcurrent'`

Expected: all 20 deterministic repetitions PASS with exact winner/error counts.

Run: `go test -race -count=1 ./examples/audit-order-history/internal/orderhistory`

Expected: PASS with no race report.

## Task 5: Build the deterministic preview and CLI

**Complexity:** Medium. **Depends on:** Task 4. **Write scope:** `preview_test.go`, `preview.go`, `main_test.go`, `main.go`.

- [ ] **Step 1: Write failing preview tests**

Use a fixed sequence of UTC instants. Assert current `order-1001` is shipped at
revision 3, full history is revision 1..3, recent history is newest-first 3..2,
all projected metadata is stable, and two runs marshal to identical bytes.

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/audit-order-history/...`

Expected: FAIL because preview and CLI are absent.

- [ ] **Step 3: Implement projection and CLI**

Create JSON-facing `Preview`, `OrderView`, and `EntryView` types rather than
exposing raw repository internals. `BuildPreview` uses the service public API;
`run(io.Writer)` constructs `audit.NewMemoryRepository`, injects a deterministic
clock, marshals with `json.MarshalIndent`, appends one newline, and propagates
errors. `main` writes failures to stderr and exits nonzero.

- [ ] **Step 4: Run GREEN and runnable proof**

Run: `go test -count=1 ./examples/audit-order-history/...`

Run: `go run ./examples/audit-order-history`

Expected: tests PASS and stdout is one deterministic JSON object containing
`current`, `history`, and `recent_history`.

## Task 6: Document the lesson in both locales

**Complexity:** Medium. **Depends on:** Task 5. **Write scope:** both example READMEs and both root READMEs.

- [ ] **Step 1: Write English and Korean example READMEs**

Both documents include the package lesson, `go run` and focused/race commands,
expected lifecycle, preview field explanation, append commit point, duplicate
retry failure, audit-vs-event-sourcing comparison, and production gaps:
non-durability, retention/deletion, pagination, O(total stored entries) in-memory
scan/copy behavior despite bounded returned cardinality, schema migration, encryption,
access control, PII/redaction, payload limits, and SQL/outbox ownership.

- [ ] **Step 2: Add root navigation parity**

Add adjacent rows in `README.md` and `README.ko.md` linking the example and both
localized README files with package `audit`.

- [ ] **Step 3: Verify docs against output**

Run: `rg -n 'audit-order-history|go run ./examples/audit-order-history|event sourcing|이벤트 소싱' README.md README.ko.md examples/audit-order-history/README*.md`

Expected: both root links and both lesson documents are present; every shown
command and behavior matches source.

## Task 7: Verify, review, and capture the lesson

**Complexity:** High. **Depends on:** Tasks 1-6. **Write scope:** review fixes and lesson only.

- [ ] **Step 1: Run targeted and repository gates**

Run in order:

```bash
go test -count=1 ./examples/audit-order-history/...
go test -race -count=1 ./examples/audit-order-history/...
go run ./examples/audit-order-history
git diff --check
make ci
```

Expected: all commands exit 0; preview JSON matches docs.

- [ ] **Step 2: Complete Type A verifier and six-lens review**

Map the exact spec and this plan to the diff, run performance/stability/security/
Ops/developer/user reviews, repair every P0/P1, rerun affected proof, and record
P0=0/P1=0. Repository hazards are N/A with evidence: no module, dependency,
workflow, container, DB, benchmark, public API, coverage, or diagram change.

- [ ] **Step 3: Write the durable lesson**

Create `docs/lessons/2026-07-13-issue-56-audit-order-history.md` with context,
append-as-commit decision, late-cancellation surprise, outcome, commands,
review misses, and the future guard that durable code needs a SQL/outbox boundary.

- [ ] **Step 4: Commit the complete validated branch**

```bash
git add docs examples README.md README.ko.md
git commit -m "feat: add audit order history example"
```

Expected: clean feature branch containing only Issue #56 artifacts.

## Task 8: Deliver and integrate

**Complexity:** Medium. **Depends on:** Task 7 and approved delivery scope. **Write scope:** GitHub PR/issue metadata, then local git state.

- [ ] **Step 1: Push and create the PR**

Push `feat/issue-56-audit-order-history`; create an English PR linked with
`Closes #56`, assign `debop`, copy milestone `0.9.0` and labels
`enhancement,examples`, and end the body with `## DoD Status`.

- [ ] **Step 2: Verify live PR and CI**

Run live metadata checks and `gh pr checks --watch`. Required checks must reach
successful terminal conclusions; skipped required evidence is blocking.

- [ ] **Step 3: Rebase merge and sync**

Rebase-merge the PR, fetch/prune, fast-forward local `develop`, verify it equals
`origin/develop`, delete the local feature branch, and remove/prune the worktree.
Never delete an unmerged or dirty worktree.

- [ ] **Step 4: Update the milestone epic**

Check #56 in issue #35 while leaving #58, #57, and #68 open. Verify #56 closed,
PR merged, worktree absent, and main checkout clean.

## Risk Prediction

| Risk | Signal | Mitigation and rerun point |
|---|---|---|
| Audit append succeeds but projection remains stale | cancel/error observed after append | Treat append as commit point; update state without a late context check; rerun cancellation tests. |
| Concurrent revisions conflict or race | race report, revision gap, intermittent invalid transition | Hold one service mutex across validation, append, and state assignment; rerun focused tests and race. |
| Duplicate retry is mistaken for successful idempotency | README or tests claim replay success | Preserve `audit.ErrRevisionConflict` and state duplicate detection explicitly; rerun duplicate test and docs review. |
| Query/full-history cost becomes misleading | claim that limit bounds repository work/allocation | Bound returned cardinality to default 20/max 100, test 25-to-20 ordering, document the O(total stored entries) in-memory scan/copy and demo-only `History`; rerun query tests and caller/performance review. |
| Sensitive cancellation metadata leaks | arbitrary payload or unbounded/invalid reason | Fixed payload schema, valid UTF-8, 256-rune bound, explicit PII warning; rerun validation/security review. |

## Acceptance Traceability

| Spec requirement | Plan task | Proof |
|---|---|---|
| Lifecycle and append-before-mutation | 2 | lifecycle tests and ordered history |
| Failure/cancellation/duplicates | 3 | negative-path tests and `errors.Is` |
| Query, absence, copies, concurrency | 4 | focused tests and race |
| Deterministic JSON | 5 | preview/CLI tests and `go run` |
| Bilingual lesson and navigation | 6 | source/output comparison and `rg` |
| Full repository quality | 7 | `git diff --check`, `make ci`, verifier/review |
| PR, CI, merge, sync, cleanup | 8 | live GitHub and git/worktree evidence |

## Plan Review Record

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0 after repair | Added exact 16-goroutine contention outcomes, 20 repetitions, race proof, 25-to-20 query proof, and O(total) cost language. |
| Stability | P0=0, P1=0 | Failure atomicity, cancellation timing, deterministic concurrency, and rerun points are ordered before integration. |
| Security | P0=0, P1=0 | Negative input, UTF-8/size, PII, fixed payload, and error-preservation tasks are explicit. |
| Operator/Ops | P0=0, P1=0 | Durability, retention, migration, diagnostics, rollback, and delivery evidence are assigned. |
| Developer/API | P0=0, P1=0 | Every spec API and error contract maps to an ordered TDD task and command. |
| User/caller | P0=0, P1=0 | Preview, bilingual docs, exact commands, unsupported claims, and misuse warnings are assigned. |
| Main integration | P0=0, P1=0 | Acceptance traceability is complete; no task depends on a later artifact; hazards are valid N/A. |

The stability/Ops and security/user lanes timed out after bounded waits; their
required fallback perspectives and the remaining developer/API and user/Ops
checks were completed independently in the main session.
