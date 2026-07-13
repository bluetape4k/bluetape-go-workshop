# Issue #58 Gin Audit Query API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable Gin service that exposes strict aggregate-scoped POST JSON audit history search and exact aggregate/revision detail lookup over bluetape-go v0.18.0.

**Architecture:** An internal framework-independent `auditquery.Service` translates bounded requests into `audit.Query`, uses `limit + 1` for revision continuation, and preserves typed audit errors. A Gin adapter owns strict input, deadlines, safe public errors, and diagnostics; `main` owns deterministic fixture construction, loopback policy, server timeouts, and joined shutdown.

**Tech Stack:** Go 1.25, bluetape-go v0.18.0 `audit`, Gin v1.12.0, standard `net/http`, `httptest`, and race detection.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/gin-audit-query-api/internal/auditquery/model.go` | Config, requests/responses, sentinels, canonical validation. |
| `examples/gin-audit-query-api/internal/auditquery/service.go` | Reader translation, pagination, detail lookup, typed errors. |
| `examples/gin-audit-query-api/internal/auditquery/service_test.go` | Filters, pages, errors, cancellation, concurrent reads. |
| `examples/gin-audit-query-api/internal/auditquery/fixture.go` | Deterministic validated order entries and seeding. |
| `examples/gin-audit-query-api/internal/auditquery/fixture_test.go` | Fixture identity, order, metadata, cancellation. |
| `examples/gin-audit-query-api/internal/auditquery/server.go` | Gin routes, strict JSON, timeout, errors, body/log ownership. |
| `examples/gin-audit-query-api/internal/auditquery/server_test.go` | HTTP success/failure, strict input, timeout, redaction. |
| `examples/gin-audit-query-api/main.go` | Wiring, address policy, server defaults, shutdown. |
| `examples/gin-audit-query-api/main_test.go` | Address and lifecycle proof without a public listener. |
| `examples/gin-audit-query-api/README.md`, `README.ko.md` | Bilingual runnable lesson and trust boundary. |
| `README.md`, `README.ko.md` | Root navigation. |
| `docs/lessons/2026-07-13-issue-58-gin-audit-query-api.md` | Durable decision and evidence. |

## Spec Coverage

| Requirement | Task | Evidence |
|---|---|---|
| Config, typed-nil behavior, stable errors | 1 | constructor/validation RED-GREEN |
| Filters, both page directions, detail, cancellation, reader errors | 2 | service tables and race |
| Deterministic order scenario | 3 | fixture assertions |
| Strict Gin boundary and safe errors/logs | 4 | `httptest` and blocking fakes |
| Loopback and joined shutdown | 5 | fake-server lifecycle tests |
| Bilingual curl lesson and discovery | 6 | locale review and live smoke |
| Full CI, review, lesson, PR/CI/merge/sync/cleanup | 7 | fresh local/live evidence |

No module, dependency, workflow, database, container, benchmark, changelog,
diagram, public library API, or migration task is triggered. Rollback deletes
the example, root links, and issue-specific durable artifacts.

## Task 1: Define the model and constructor contract

**Complexity:** Medium. **Depends on:** approved spec commit `ff89576`.

**Files:** Create `model.go`, `service.go`, and `service_test.go` under
`examples/gin-audit-query-api/internal/auditquery/`.

- [x] **Step 1: Write failing tests**

Test default limits `20/100`, nil and typed-nil `audit.HistoryReader`,
non-positive limits, default above maximum, maximum above 100, path-unsafe
aggregate identifiers, invalid revision/time ranges,
negative/oversized request limits, and nil context normalization.

~~~go
var reader *audit.MemoryRepository
service, err := NewService(reader, DefaultServiceConfig())
if service != nil || !errors.Is(err, ErrInvalidConfig) {
    t.Fatalf("service = %v, err = %v", service, err)
}
~~~

- [x] **Step 2: Run RED**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(DefaultServiceConfig|NewService|Validate)'
~~~

Expected: FAIL because the package and symbols do not exist.

- [x] **Step 3: Implement the minimum contract**

Define the exact spec types/tags, three sentinels, default config, canonical ID
validation, typed-nil detection, context normalization, and constructor. Use
`audit.NewAggregateID` and `audit.Query.Validate` rather than duplicating audit
semantics. Preserve own and audit sentinels with `%w`.

- [x] **Step 4: Run GREEN, format, and commit**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(DefaultServiceConfig|NewService|Validate)'
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: define audit query service contract"
~~~

Expected: PASS.

## Task 2: Implement exact lookup and revision pagination

**Complexity:** High. **Depends on:** Task 1.

**Files:** Modify `service.go` and `service_test.go`.

- [x] **Step 1: Write failing behavior tests**

Use a real memory repository to prove ascending/newest pages, first-unseen
inclusive continuation, inclusive revision/time filters, defaults, custom
limits, empty non-nil entries, detail success, `ErrEntryNotFound`, and
zero-value service failure without panic.

~~~go
first, err := service.Search(ctx, SearchRequest{
    Aggregate: AggregateRequest{Type: "order", ID: "order-1"},
    NewestFirst: true,
    Limit: 2,
})
if err != nil || !first.Page.HasMore || first.Page.Next.ToRevision != 2 {
    t.Fatalf("first = %+v, err = %v", first, err)
}
~~~

Embed `audit.HistoryReader` in a fake overriding `Find` to return cancellation,
deadline, `audit.ValidationError`, and an opaque sentinel. Assert one call and
preserved `errors.Is`/`errors.As`.

- [x] **Step 2: Run RED**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestService_(Search|Get)'
~~~

Expected: FAIL because `Search` and `Get` are absent.

- [x] **Step 3: Implement minimal behavior**

`Search` validates one exact aggregate, uses `effectiveLimit + 1`, invokes the
reader once, trims the extra entry, and returns its revision in
`next.from_revision` or `next.to_revision`. `Get` queries equal from/to
revision with limit one and maps empty to `ErrEntryNotFound`. Both preserve
reader causes and fail closed on zero-value service.

- [x] **Step 4: Add bounded concurrent/cancellation tests**

Use a start barrier and exact counts, never sleeps, for concurrent search/detail
reads. Add pre-canceled contexts and prove invalid input does not invoke the
reader.

- [x] **Step 5: Run GREEN, race, and commit**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestService_(Search|Get|Concurrent)'
go test -race -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestService_(Search|Get|Concurrent)'
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: query paginated audit history"
~~~

Expected: PASS with no duplicate, gap, retry, or race.

## Task 3: Add the deterministic order fixture

**Complexity:** Medium. **Depends on:** Task 2.

**Files:** Create `fixture.go` and `fixture_test.go`.

- [x] **Step 1: Write failing fixture tests**

Require two aggregates with revisions `1..4` and `1..2`, fixed UTC timestamps,
unique event/idempotency IDs, author `workshop`, safe `source=fixture` metadata,
valid payload/change metadata, exact lifecycle order, defensive copies, and a
pre-canceled seed leaving the repository empty.

- [x] **Step 2: Run RED**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestSeedRepository'
~~~

Expected: FAIL because `SeedRepository` is absent.

- [x] **Step 3: Implement validated entries**

Use `audit.NewAggregateID`, `NewDomainEvent`, `NewChangeMetadata`, and
`NewEntry`. Append each aggregate as one contiguous batch and wrap every cause.

- [x] **Step 4: Run GREEN and commit**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestSeedRepository'
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: seed deterministic audit history"
~~~

## Task 4: Implement the strict Gin adapter

**Complexity:** High. **Depends on:** Tasks 1-3.

**Files:** Create `server.go` and `server_test.go`.

- [x] **Step 1: Write failing route/response tests**

Define a narrow `QueryService` interface. Test health, successful POST/detail,
empty search, detail 404, wrong method, trailing slash, encoded/path-unsafe
IDs, invalid revision, and nil/typed-nil dependencies.

- [x] **Step 2: Run RED**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(NewEngine|HTTP)'
~~~

Expected: FAIL because the adapter is absent.

- [x] **Step 3: Implement engine and error mapping**

Use defaults `32 << 10` and two seconds; `gin.New`; method-not-allowed; no
trailing redirect; raw non-unescaped paths; no trusted proxies. Register only
the three spec routes. Map audit sentinels before `ErrInvalidRequest`, then
not-found, adapter timeout, and opaque failures. Log only route template,
method, status, code, and elapsed.

- [x] **Step 4: Write failing strict-input tests**

Cover missing/wrong content type, invalid charset, unsupported encoding, invalid
UTF-8, empty/malformed JSON, nested duplicate keys, unknown fields, trailing
JSON, one-byte oversize, and request-body closure on every path.

- [x] **Step 5: Implement strict decoding**

Use `mime.ParseMediaType`, `http.MaxBytesReader`, UTF-8 validation, duplicate-key
rejection, `DisallowUnknownFields`, and one-value EOF validation.

- [x] **Step 6: Prove timeout/cancellation/redaction**

A blocking fake returns on context completion. Assert adapter deadline gives
408, client cancellation is preserved without an unreliable response, and
secret-like body, aggregate, metadata, and reader error values never appear in
public errors or logs.

- [x] **Step 7: Run GREEN, race, and commit**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(NewEngine|HTTP)'
go test -race -count=1 ./examples/gin-audit-query-api/internal/auditquery
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: expose audit queries through Gin"
~~~

## Task 5: Own runtime configuration and joined shutdown

**Complexity:** High. **Depends on:** Tasks 3-4.

**Files:** Create `main.go` and `main_test.go`.

- [x] **Step 1: Write failing runtime tests**

Test default, IPv4/IPv6 loopback, localhost, invalid/missing ports,
hostname/non-loopback rejection, and explicit remote opt-in. Assert fixed
header/read/write/idle/max-header values. Fake servers prove listen failure,
normal shutdown, shutdown failure then close, close failure aggregation, and
listener join without sleeps or a public listener.

- [x] **Step 2: Run RED**

~~~bash
go test -count=1 ./examples/gin-audit-query-api -run 'Test(ResolveHTTPAddr|NewHTTPServer|RunServer)'
~~~

Expected: FAIL because runtime functions are absent.

- [x] **Step 3: Implement wiring and lifecycle**

Seed a memory repository, construct service/engine, use Gin release mode,
resolve address, create one `http.Server`, and use `signal.NotifyContext`.
`RunServer` owns a buffered result channel, joins it after shutdown, and forces
`Close` on graceful failure.

- [x] **Step 4: Run GREEN, race, and commit**

~~~bash
gofmt -w examples/gin-audit-query-api/*.go
go test -count=1 ./examples/gin-audit-query-api -run 'Test(ResolveHTTPAddr|NewHTTPServer|RunServer)'
go test -race -count=1 ./examples/gin-audit-query-api
git add examples/gin-audit-query-api/main.go examples/gin-audit-query-api/main_test.go
git commit -m "feat: run Gin audit query server"
~~~

## Task 6: Document and smoke-test the lesson

**Complexity:** Medium. **Depends on:** Tasks 1-5.

**Files:** Create both example READMEs; modify both root READMEs.

- [x] **Step 1: Write source-equivalent locale docs**

Include language switch, lesson, routes, run command, POST/detail curls,
response shapes, both continuation directions, metadata-filter non-goal,
loopback/remote behavior, and auth/tenant/payload/retention/size/durability/
rate-limit/health boundaries.

- [x] **Step 2: Add both root navigation entries**

Place the example beside `audit-order-history` in each root README.

- [x] **Step 3: Run bounded live smoke**

Start on an unused loopback port, poll `/healthz` with a bounded readiness loop,
run the documented POST and detail curls, validate with `jq`, terminate, and
assert process exit. No fixed sleep is readiness evidence.

- [x] **Step 4: Verify and commit**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/...
git diff --check
git add examples/gin-audit-query-api/README.md examples/gin-audit-query-api/README.ko.md README.md README.ko.md
git commit -m "docs: explain Gin audit query example"
~~~

## Task 7: Verify, review, learn, deliver, merge, and clean

**Complexity:** High. **Depends on:** Tasks 1-6.

- [x] **Step 1: Run fresh validation**

~~~bash
gofmt -w examples/gin-audit-query-api/*.go examples/gin-audit-query-api/internal/auditquery/*.go
git diff --check
go test -count=1 ./examples/gin-audit-query-api/...
go test -race -count=1 ./examples/gin-audit-query-api/...
make ci
~~~

Any failure returns to its owner and requires affected proof from the beginning.

- [x] **Step 2: Verify spec/plan and six review perspectives**

Re-read the exact spec, plan, issue, diff, tests, docs, and triggered hazards.
Map every acceptance criterion. Converge performance, stability, security,
operator, developer/API, user/caller, and main integration at P0=0/P1=0.

- [x] **Step 3: Commit lesson and review repairs**

The lesson records aggregate-scoped POST selection, metadata-filter rejection,
first-unseen inclusive pagination, strict Gin ownership, actual proof, misses,
and future guard.

~~~bash
git add docs/lessons docs/review examples README.md README.ko.md
git commit -m "docs: record audit query API lessons"
~~~

- [ ] **Step 4: Push and create PR**

Refresh issue metadata; push the branch; create an English PR assigned to
`debop` with milestone `0.9.0` and mirrored labels. End the body with
`## DoD Status` and verify all live metadata, head/base, and linked issue.

- [ ] **Step 5: Wait for CI and review state**

All required checks must succeed. Re-read reviews, comments, decision, and
unresolved threads. Failures/comments return to implementation and fresh proof.

- [ ] **Step 6: Rebase-merge under approved authority**

Immediately refresh head SHA, mergeability, checks, P0/P1, review threads, and
authority. Rebase-merge and verify live state `MERGED`.

- [ ] **Step 7: Sync and clean owned state**

Preserve new user dirt, fast-forward real `develop`, prove local/upstream SHAs,
verify integration ancestry/patch equivalence, remove the owned worktree, prune,
delete the integrated local branch, and report clean/preserved state.

## Risk Prediction

| Risk | Signal | Mitigation | Rerun |
|---|---|---|---|
| Page gap/duplicate | repeated/missing revision | first-unseen inclusive boundary; both directions/time filters | Task 2 tests/race |
| Sensitive leakage | raw audit input/error in output/log | fixed messages, route logs, secret canaries, loopback | Task 4 tests/race |
| Timeout confusion | response after client cancel | explicit deadline ownership and blocking fake | Task 4 timeout/cancel |
| Listener leak | return before listener join | result channel and exact fake calls | Task 5 lifecycle/race |
| Incorrect metadata filter | page incompleteness | no request field; reopen spec before addition | stop implementation |
| README drift | curl/response mismatch | live bounded smoke | Task 6 smoke/tests |

## Plan Review Record

The subagent interface cannot supply the mandatory installed `agent_type`, so
the main session performed six isolated plan reads and integrated the findings.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0 | Bounded page/lookahead, response-budget docs, focused race, and live smoke are assigned. |
| Stability | P0=0, P1=0 | Cancellation, body closure, no-retry reader errors, lifecycle failure, forced close, and listener join have ordered tests. |
| Security | P0=0, P1=0 | Strict decoding, path validation, proxy/loopback policy, canary redaction, and trust-boundary docs are explicit. |
| Operator/Ops | P0=0, P1=0 | Health semantics, diagnostics, shutdown, rollback, CI, merge, sync, and cleanup have owners and proof. |
| Developer/API | P0=0, P1=0 after repair | Moved zero-value method proof after the methods exist and corrected the final formatter command to target Go files. |
| User/caller | P0=0, P1=0 | POST/body, continuation, detail/empty distinction, curl, unsupported metadata filter, and both locales are covered. |
| Main integration | P0=0, P1=0 | Every spec criterion maps to an ordered task; no task depends on later code and all triggered risks have rerun points. |
