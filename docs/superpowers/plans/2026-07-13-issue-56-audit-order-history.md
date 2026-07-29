# Issue #56 Audit Order History Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** bluetape-go audit entry를 in-memory current-state projection 업데이트 전에 atomic하게 append하는 runnable order lifecycle example을 만든다.

**Architecture:** internal `orderhistory.Service`가 명시적인 order transition, caller-injected `audit.Repository`, mutex-protected teaching projection을 소유한다. preview layer는 deterministic lifecycle을 실행하고 immutable audit entry를 stable JSON으로 project한다. bilingual documentation은 이것이 event sourcing이 아니라 audit history인 이유를 설명한다.

**Tech Stack:** Go 1.25, bluetape-go v0.18.0 `audit`, standard library `context`, `encoding/json`, `sync`, `testing`을 사용한다.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/audit-order-history/internal/orderhistory/model.go` | Status, command, order snapshot, validation sentinel, copy를 정의한다. |
| `examples/audit-order-history/internal/orderhistory/service.go` | Constructor, state machine, append-before-mutation boundary, history/query facade를 구현한다. |
| `examples/audit-order-history/internal/orderhistory/service_test.go` | Success, failure, cancellation, duplicate, defensive-copy, concurrency proof를 담는다. |
| `examples/audit-order-history/internal/orderhistory/preview.go` | Deterministic lifecycle과 JSON-facing projection을 제공한다. |
| `examples/audit-order-history/internal/orderhistory/preview_test.go` | Preview content와 determinism proof를 검증한다. |
| `examples/audit-order-history/main.go` | memory repository를 만들고 preview를 실행한 뒤 indented JSON을 출력한다. |
| `examples/audit-order-history/main_test.go` | exact stdout JSON과 failure propagation을 검증한다. |
| `examples/audit-order-history/README.md` | English lesson, command, expected behavior, production boundary를 설명한다. |
| `examples/audit-order-history/README.ko.md` | Korean parity documentation을 제공한다. |
| `README.md`, `README.ko.md` | root example navigation을 갱신한다. |
| `docs/lessons/2026-07-13-issue-56-audit-order-history.md` | durable implementation lesson과 proof를 기록한다. |

## Task 1: domain 및 constructor contract 정의

**Complexity:** Medium. **Depends on:** approved spec. **Write scope:** `model.go`, `service_test.go`, then `service.go`.

- [ ] **Step 1: 실패하는 constructor 및 validation test 작성**

nil repository와 invalid author가 `ErrInvalidConfig`를 wrap하는지, nil clock이 허용되는지, `Service{}` method가 panic하지 않는지, ID가 `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`을 사용하는지, cancellation reason이 valid UTF-8이고 최대 256 rune인지 table test로 증명한다.

```go
service, err := NewService(nil, Options{Author: "workshop"})
if !errors.Is(err, ErrInvalidConfig) || service != nil { t.Fatalf("...") }
```

- [ ] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/audit-order-history/internal/orderhistory`

기대값: package와 symbol이 아직 없으므로 FAIL한다.

- [ ] **Step 3: minimal domain 및 constructor 구현**

`StatusPending`, `StatusConfirmed`, `StatusShipped`, `StatusCancelled`, exact spec command type, `Order`, `Options`, service sentinel 다섯 개, validation helper, `NewService`를 정의한다. nil clock은 UTC `time.Now`로 default한다. 모든 operation에서 missing service dependency를 detect한다.

- [ ] **Step 4: GREEN 및 format 실행**

실행: `gofmt -w examples/audit-order-history/internal/orderhistory/*.go && go test -count=1 ./examples/audit-order-history/internal/orderhistory`

기대값: constructor/validation test가 PASS한다.

## Task 2: append-before-mutation transition 구현

**Complexity:** High. **Depends on:** Task 1. **Write scope:** `service_test.go`, `service.go`.

- [ ] **Step 1: 실패하는 lifecycle test 작성**

create-confirm-ship과 두 cancellation path를 테스트한다. current revision, UTC timestamp, event type `order.created|confirmed|shipped|cancelled`, `EventID`와 `IdempotencyKey`의 stable command ID, author, schema version, status payload, `ChangeMetadata` before/after attribute를 assert한다.

```go
created, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
if err != nil || created.Status != StatusPending || created.Revision != 1 { t.Fatalf("...") }
```

- [ ] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(Lifecycle|Cancel)'`

기대값: transition method가 없으므로 FAIL한다.

- [ ] **Step 3: minimal transition engine 구현**

lock 전에 context/input을 validate하고, mutex 내부에서 context를 다시 확인한다. current state를 validate하고 `json.Marshal`로 payload를 만든 뒤 `audit.NewAggregateID`, `audit.NewDomainEvent`, `audit.NewChangeMetadata`, `audit.NewEntry`를 호출하고 `repo.Append`를 실행한다. append가 nil을 반환한 뒤에만 projection을 업데이트한다. append success와 infallible map assignment 사이에서는 cancellation을 확인하지 않는다.

- [ ] **Step 4: GREEN 실행**

실행: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(Lifecycle|Cancel)'`

기대값: revision 1..3과 ordered history로 PASS한다.

## Task 3: failure atomicity, cancellation, duplicate detection 증명

**Complexity:** High. **Depends on:** Task 2. **Write scope:** `service_test.go`, minimal `service.go` fixes.

- [ ] **Step 1: 실패하는 negative-path test 추가**

duplicate order, missing order, invalid/terminal transition, aggregate 간 reused command ID, repository failure, pre-canceled context, append 중 cancel, successful append 직후 cancel하지만 nil을 반환하는 wrapper repository를 cover한다. 모든 case에서 history와 current state가 aligned로 남고 repository error가 `errors.Is`를 보존하는지 assert한다.

```go
repo := &failingRepository{err: errRepositoryUnavailable}
_, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
if !errors.Is(err, errRepositoryUnavailable) { t.Fatalf("...") }
if _, ok := service.Current("order-1"); ok { t.Fatal("state mutated") }
```

- [ ] **Step 2: RED 실행, 수정, GREEN 재실행**

실행: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(Rejects|Repository|Cancellation|Duplicate)'`

기대 RED: 아직 구현되지 않은 failure contract가 하나 이상 있다. smallest service correction 이후 기대 GREEN: 나열된 모든 case가 PASS한다.

## Task 4: bounded history/query 및 concurrency proof 추가

**Complexity:** High. **Depends on:** Task 3. **Write scope:** `service_test.go`, `service.go`.

- [ ] **Step 1: 실패하는 query/copy/concurrency test 작성**

absent history가 `(History{}, false, nil)`을 반환하는지, no-match `Find`가 non-nil empty slice를 반환하는지, default query limit이 저장된 25 entry 중 repository order 기준 정확히 20개를 반환하는지, 100 초과 limit과 non-order aggregate type이 `audit.ErrInvalidQuery`를 wrap하는지, revision window/newest-first가 동작하는지, returned history가 isolated인지 증명한다. 20-row cap은 returned cardinality만 제한한다. v0.18 memory repository는 여전히 O(total stored entries) scan/copy를 수행한다.

sleep 없는 deterministic channel-barrier concurrency case를 추가한다. goroutine 16개가 서로 다른 order 16개를 create하고 success 16개, current snapshot 16개, one-entry history 16개를 만들어야 한다. 하나의 order 생성 후 unique confirm command 16개를 concurrent 실행하면 정확히 success 1개와 `ErrInvalidTransition` failure 15개를 만들고 current revision은 2, contiguous history revision은 `[1,2]`여야 한다. `sync.WaitGroup`과 closed start channel을 사용한다.

- [ ] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/audit-order-history/internal/orderhistory -run 'TestService_(History|Find|Defensive|Concurrent)'`

기대값: query normalization과 read method가 없으므로 FAIL한다.

- [ ] **Step 3: 구현 및 GREEN 실행**

`History`는 `LoadHistory`에 delegate한다. current-state value를 clone한다. `Find.AggregateType`은 `order`로 normalize하고, zero limit은 20으로 default하며, 100 초과 limit은 reject하고 repository에 delegate한다.

실행: `go test -count=1 ./examples/audit-order-history/internal/orderhistory`

기대값: PASS.

- [ ] **Step 4: focused race proof 실행**

실행: `go test -count=20 ./examples/audit-order-history/internal/orderhistory -run '^TestServiceConcurrent'`

기대값: deterministic repetition 20회가 정확한 winner/error count로 모두 PASS한다.

실행: `go test -race -count=1 ./examples/audit-order-history/internal/orderhistory`

기대값: race report 없이 PASS한다.

## Task 5: deterministic preview 및 CLI 작성

**Complexity:** Medium. **Depends on:** Task 4. **Write scope:** `preview_test.go`, `preview.go`, `main_test.go`, `main.go`.

- [ ] **Step 1: 실패하는 preview test 작성**

고정된 UTC instant sequence를 사용한다. current `order-1001`이 revision 3에서 shipped인지, full history가 revision 1..3인지, recent history가 newest-first 3..2인지, 모든 projected metadata가 stable인지, 두 run이 동일한 byte로 marshal되는지 assert한다.

- [ ] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/audit-order-history/...`

기대값: preview와 CLI가 없으므로 FAIL한다.

- [ ] **Step 3: projection 및 CLI 구현**

raw repository internal을 노출하지 말고 JSON-facing `Preview`, `OrderView`, `EntryView` type을 만든다. `BuildPreview`는 service public API를 사용한다. `run(io.Writer)`는 `audit.NewMemoryRepository`를 만들고 deterministic clock을 inject하며, `json.MarshalIndent`로 marshal하고 newline 하나를 붙인 뒤 error를 propagate한다. `main`은 failure를 stderr에 쓰고 nonzero로 exit한다.

- [ ] **Step 4: GREEN 및 runnable proof 실행**

실행: `go test -count=1 ./examples/audit-order-history/...`

실행: `go run ./examples/audit-order-history`

기대값: test가 PASS하고 stdout은 `current`, `history`, `recent_history`를 포함한 deterministic JSON object 하나다.

## Task 6: 두 locale에 lesson 문서화

**Complexity:** Medium. **Depends on:** Task 5. **Write scope:** 두 example README와 두 root README.

- [ ] **Step 1: English 및 Korean example README 작성**

두 문서 모두 package lesson, `go run` 및 focused/race command, expected lifecycle, preview field explanation, append commit point, duplicate retry failure, audit-vs-event-sourcing comparison, production gap을 포함한다. production gap은 non-durability, retention/deletion, pagination, bounded returned cardinality에도 불구하고 발생하는 O(total stored entries) in-memory scan/copy behavior, schema migration, encryption, access control, PII/redaction, payload limit, SQL/outbox ownership을 다룬다.

- [ ] **Step 2: root navigation parity 추가**

`README.md`와 `README.ko.md`에 package `audit`로 example과 두 localized README file을 link하는 adjacent row를 추가한다.

- [ ] **Step 3: output 기준 docs 검증**

실행: `rg -n 'audit-order-history|go run ./examples/audit-order-history|event sourcing|이벤트 소싱' README.md README.ko.md examples/audit-order-history/README*.md`

기대값: 두 root link와 두 lesson document가 존재한다. 표시된 모든 command와 behavior가 source와 일치한다.

## Task 7: 검증, review, lesson 기록

**Complexity:** High. **Depends on:** Tasks 1-6. **Write scope:** review fix와 lesson만 포함한다.

- [ ] **Step 1: targeted 및 repository gate 실행**

순서대로 실행한다.

```bash
go test -count=1 ./examples/audit-order-history/...
go test -race -count=1 ./examples/audit-order-history/...
go run ./examples/audit-order-history
git diff --check
make ci
```

기대값: 모든 command가 exit 0으로 끝난다. preview JSON은 docs와 일치한다.

- [ ] **Step 2: Type A verifier 및 six-lens review 완료**

exact spec과 이 plan을 diff에 매핑하고 performance/stability/security/Ops/developer/user review를 실행한다. 모든 P0/P1을 수정하고 affected proof를 다시 실행하며 P0=0/P1=0을 기록한다. repository hazard는 evidence와 함께 N/A이다: module, dependency, workflow, container, DB, benchmark, public API, coverage, diagram change가 없다.

- [ ] **Step 3: durable lesson 작성**

`docs/lessons/2026-07-13-issue-56-audit-order-history.md`를 만들고 context, append-as-commit decision, late-cancellation surprise, outcome, command, review miss, durable code에는 SQL/outbox boundary가 필요하다는 future guard를 기록한다.

- [ ] **Step 4: complete validated branch commit**

```bash
git add docs examples README.md README.ko.md
git commit -m "feat: add audit order history example"
```

기대값: Issue #56 artifact만 포함한 clean feature branch.

## Task 8: delivery 및 integration

**Complexity:** Medium. **Depends on:** Task 7 and approved delivery scope. **Write scope:** GitHub PR/issue metadata, then local git state.

- [ ] **Step 1: push 및 PR 생성**

`feat/issue-56-audit-order-history`를 push한다. `Closes #56`로 linked English PR을 만들고 `debop`에게 assign한다. milestone `0.9.0`, label `enhancement,examples`를 복사하고 body는 `## DoD Status`로 끝낸다.

- [ ] **Step 2: live PR 및 CI 검증**

live metadata check와 `gh pr checks --watch`를 실행한다. required check는 successful terminal conclusion에 도달해야 하며, skipped required evidence는 blocking이다.

- [ ] **Step 3: rebase merge 및 sync**

PR을 rebase-merge하고 fetch/prune한 뒤 local `develop`을 fast-forward한다. local `develop`이 `origin/develop`와 같은지 확인하고 local feature branch를 삭제하며 worktree를 remove/prune한다. unmerged 또는 dirty worktree는 절대 삭제하지 않는다.

- [ ] **Step 4: milestone epic 업데이트**

#35 issue에서 #56만 check하고 #58, #57, #68은 open으로 둔다. #56 closure, PR merge, worktree absence, main checkout clean을 확인한다.

## Risk Prediction

| Risk | Signal | Mitigation and rerun point |
|---|---|---|
| Audit append는 성공했지만 projection이 stale로 남음 | append 뒤 cancel/error 관찰 | append를 commit point로 취급한다. late context check 없이 state를 업데이트하고 cancellation test를 다시 실행한다. |
| concurrent revision conflict 또는 race | race report, revision gap, intermittent invalid transition | validation, append, state assignment 전체에 service mutex 하나를 유지한다. focused test와 race를 다시 실행한다. |
| duplicate retry를 successful idempotency로 오해 | README 또는 test가 replay success를 주장 | `audit.ErrRevisionConflict`와 state duplicate detection을 명시적으로 보존한다. duplicate test와 docs review를 다시 실행한다. |
| query/full-history cost가 오해를 만든다 | limit이 repository work/allocation을 제한한다는 주장 | returned cardinality를 default 20/max 100으로 제한하고 25-to-20 ordering을 테스트한다. O(total stored entries) in-memory scan/copy와 demo-only `History`를 문서화하고 query test 및 caller/performance review를 다시 실행한다. |
| sensitive cancellation metadata leak | arbitrary payload 또는 unbounded/invalid reason | fixed payload schema, valid UTF-8, 256-rune bound, explicit PII warning을 적용하고 validation/security review를 다시 실행한다. |

## Acceptance Traceability

| Spec requirement | Plan task | Proof |
|---|---|---|
| Lifecycle and append-before-mutation | 2 | lifecycle test와 ordered history |
| Failure/cancellation/duplicates | 3 | negative-path test와 `errors.Is` |
| Query, absence, copies, concurrency | 4 | focused test와 race |
| Deterministic JSON | 5 | preview/CLI test와 `go run` |
| Bilingual lesson and navigation | 6 | source/output comparison과 `rg` |
| Full repository quality | 7 | `git diff --check`, `make ci`, verifier/review |
| PR, CI, merge, sync, cleanup | 8 | live GitHub 및 git/worktree evidence |

## Plan Review Record

| Lens | Result | Resolution |
|---|---|---|
| Performance | repair 이후 P0=0, P1=0 | 정확한 16-goroutine contention outcome, 20 repetition, race proof, 25-to-20 query proof, O(total) cost language를 추가했다. |
| Stability | P0=0, P1=0 | failure atomicity, cancellation timing, deterministic concurrency, rerun point가 integration 전에 배치되어 있다. |
| Security | P0=0, P1=0 | negative input, UTF-8/size, PII, fixed payload, error-preservation task가 명시적이다. |
| Operator/Ops | P0=0, P1=0 | durability, retention, migration, diagnostics, rollback, delivery evidence가 배정되어 있다. |
| Developer/API | P0=0, P1=0 | 모든 spec API와 error contract가 순서 있는 TDD task와 command에 매핑되어 있다. |
| User/caller | P0=0, P1=0 | preview, bilingual docs, exact command, unsupported claim, misuse warning이 배정되어 있다. |
| Main integration | P0=0, P1=0 | acceptance traceability가 완전하다. 나중 artifact에 의존하는 task가 없고 hazard N/A가 유효하다. |

stability/Ops 및 security/user lane은 bounded wait 뒤 timeout되었다. 필요한 fallback perspective와 남은 developer/API 및 user/Ops check는 main session에서 독립적으로 완료했다.
