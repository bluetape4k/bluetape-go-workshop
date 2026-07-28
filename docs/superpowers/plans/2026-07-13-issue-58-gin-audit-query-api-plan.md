# Issue #58 Gin Audit Query API Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** bluetape-go v0.18.0 위에서 strict aggregate-scoped POST JSON audit history search와 exact aggregate/revision detail lookup을 제공하는 runnable Gin service를 만든다.

**Architecture:** framework-independent internal `auditquery.Service`가 bounded request를 `audit.Query`로 변환하고, revision continuation을 위해 `limit + 1`을 사용하며, typed audit error를 보존한다. Gin adapter는 strict input, deadline, safe public error, diagnostics를 소유한다. `main`은 deterministic fixture construction, loopback policy, server timeout, joined shutdown을 소유한다.

**Tech Stack:** Go 1.25, bluetape-go v0.18.0 `audit`, Gin v1.12.0, standard `net/http`, `httptest`, race detection을 사용한다.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/gin-audit-query-api/internal/auditquery/model.go` | Config, request/response, sentinel, canonical validation을 정의한다. |
| `examples/gin-audit-query-api/internal/auditquery/service.go` | Reader translation, pagination, detail lookup, typed error를 구현한다. |
| `examples/gin-audit-query-api/internal/auditquery/service_test.go` | Filter, page, error, cancellation, concurrent read를 검증한다. |
| `examples/gin-audit-query-api/internal/auditquery/fixture.go` | Deterministic validated order entry와 seeding을 제공한다. |
| `examples/gin-audit-query-api/internal/auditquery/fixture_test.go` | Fixture identity, order, metadata, cancellation을 검증한다. |
| `examples/gin-audit-query-api/internal/auditquery/server.go` | Gin route, strict JSON, timeout, error, body/log ownership을 구현한다. |
| `examples/gin-audit-query-api/internal/auditquery/server_test.go` | HTTP success/failure, strict input, timeout, redaction을 검증한다. |
| `examples/gin-audit-query-api/main.go` | Wiring, address policy, server default, shutdown을 담당한다. |
| `examples/gin-audit-query-api/main_test.go` | public listener 없이 address 및 lifecycle proof를 검증한다. |
| `examples/gin-audit-query-api/README.md`, `README.ko.md` | bilingual runnable lesson과 trust boundary를 설명한다. |
| `README.md`, `README.ko.md` | root navigation을 갱신한다. |
| `docs/lessons/2026-07-13-issue-58-gin-audit-query-api.md` | durable decision과 evidence를 기록한다. |

## Spec Coverage

| Requirement | Task | Evidence |
|---|---|---|
| Config, typed-nil behavior, stable errors | 1 | constructor/validation RED-GREEN |
| Filters, both page directions, detail, cancellation, reader errors | 2 | service table과 race |
| Deterministic order scenario | 3 | fixture assertion |
| Strict Gin boundary and safe errors/logs | 4 | `httptest`와 blocking fake |
| Loopback and joined shutdown | 5 | fake-server lifecycle test |
| Bilingual curl lesson and discovery | 6 | locale review와 live smoke |
| Full CI, review, lesson, PR/CI/merge/sync/cleanup | 7 | fresh local/live evidence |

module, dependency, workflow, database, container, benchmark, changelog, diagram, public library API, migration task는 trigger되지 않는다. rollback은 example, root link, issue-specific durable artifact를 삭제한다.

## Task 1: model 및 constructor contract 정의

**Complexity:** Medium. **Depends on:** approved spec commit `ff89576`.

**Files:** `examples/gin-audit-query-api/internal/auditquery/` 아래에 `model.go`, `service.go`, `service_test.go`를 생성한다.

- [x] **Step 1: 실패하는 test 작성**

default limit `20/100`, nil 및 typed-nil `audit.HistoryReader`, non-positive limit, maximum보다 큰 default, 100보다 큰 maximum, path-unsafe aggregate identifier, invalid revision/time range, negative/oversized request limit, nil context normalization을 테스트한다.

~~~go
var reader *audit.MemoryRepository
service, err := NewService(reader, DefaultServiceConfig())
if service != nil || !errors.Is(err, ErrInvalidConfig) {
    t.Fatalf("service = %v, err = %v", service, err)
}
~~~

- [x] **Step 2: RED 실행**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(DefaultServiceConfig|NewService|Validate)'
~~~

기대값: package와 symbol이 없으므로 FAIL한다.

- [x] **Step 3: minimum contract 구현**

exact spec type/tag, sentinel 세 개, default config, canonical ID validation, typed-nil detection, context normalization, constructor를 정의한다. audit semantic을 중복 구현하지 말고 `audit.NewAggregateID`와 `audit.Query.Validate`를 사용한다. 자체 sentinel과 audit sentinel은 `%w`로 보존한다.

- [x] **Step 4: GREEN, format, commit 실행**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(DefaultServiceConfig|NewService|Validate)'
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: define audit query service contract"
~~~

기대값: PASS.

## Task 2: exact lookup 및 revision pagination 구현

**Complexity:** High. **Depends on:** Task 1.

**Files:** `service.go`와 `service_test.go`를 수정한다.

- [x] **Step 1: 실패하는 behavior test 작성**

real memory repository를 사용해 ascending/newest page, first-unseen inclusive continuation, inclusive revision/time filter, default, custom limit, empty non-nil entries, detail success, `ErrEntryNotFound`, panic 없는 zero-value service failure를 증명한다.

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

`audit.HistoryReader`를 embed한 fake에서 `Find`를 override하여 cancellation, deadline, `audit.ValidationError`, opaque sentinel을 반환하게 한다. call이 한 번인지와 `errors.Is`/`errors.As` 보존을 assert한다.

- [x] **Step 2: RED 실행**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestService_(Search|Get)'
~~~

기대값: `Search`와 `Get`이 없으므로 FAIL한다.

- [x] **Step 3: minimal behavior 구현**

`Search`는 exact aggregate 하나를 validate하고 `effectiveLimit + 1`을 사용한다. reader를 한 번 호출하고 extra entry를 trim한 뒤 해당 revision을 `next.from_revision` 또는 `next.to_revision`으로 반환한다. `Get`은 from/to revision이 같은 query와 limit one을 사용하고 empty를 `ErrEntryNotFound`로 map한다. 둘 다 reader cause를 보존하고 zero-value service에서 fail closed한다.

- [x] **Step 4: bounded concurrent/cancellation test 추가**

concurrent search/detail read에는 start barrier와 exact count를 사용하며 sleep은 사용하지 않는다. pre-canceled context를 추가하고 invalid input이 reader를 호출하지 않는지 증명한다.

- [x] **Step 5: GREEN, race, commit 실행**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestService_(Search|Get|Concurrent)'
go test -race -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestService_(Search|Get|Concurrent)'
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: query paginated audit history"
~~~

기대값: duplicate, gap, retry, race 없이 PASS.

## Task 3: deterministic order fixture 추가

**Complexity:** Medium. **Depends on:** Task 2.

**Files:** `fixture.go`와 `fixture_test.go`를 생성한다.

- [x] **Step 1: 실패하는 fixture test 작성**

두 aggregate가 revision `1..4`와 `1..2`, fixed UTC timestamp, unique event/idempotency ID, author `workshop`, safe `source=fixture` metadata, valid payload/change metadata, exact lifecycle order, defensive copy를 갖도록 요구한다. pre-canceled seed는 repository를 empty로 남겨야 한다.

- [x] **Step 2: RED 실행**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestSeedRepository'
~~~

기대값: `SeedRepository`가 없으므로 FAIL한다.

- [x] **Step 3: validated entry 구현**

`audit.NewAggregateID`, `NewDomainEvent`, `NewChangeMetadata`, `NewEntry`를 사용한다. 각 aggregate는 contiguous batch 하나로 append하고 모든 cause를 wrap한다.

- [x] **Step 4: GREEN 및 commit 실행**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'TestSeedRepository'
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: seed deterministic audit history"
~~~

## Task 4: strict Gin adapter 구현

**Complexity:** High. **Depends on:** Tasks 1-3.

**Files:** `server.go`와 `server_test.go`를 생성한다.

- [x] **Step 1: 실패하는 route/response test 작성**

narrow `QueryService` interface를 정의한다. health, successful POST/detail, empty search, detail 404, wrong method, trailing slash, encoded/path-unsafe ID, invalid revision, nil/typed-nil dependency를 테스트한다.

- [x] **Step 2: RED 실행**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(NewEngine|HTTP)'
~~~

기대값: adapter가 없으므로 FAIL한다.

- [x] **Step 3: engine 및 error mapping 구현**

default `32 << 10`과 two seconds를 사용한다. `gin.New`, method-not-allowed, no trailing redirect, raw non-unescaped path, no trusted proxies를 적용한다. 세 spec route만 등록한다. audit sentinel을 `ErrInvalidRequest`보다 먼저 map하고, 그다음 not-found, adapter timeout, opaque failure를 map한다. log에는 route template, method, status, code, elapsed만 남긴다.

- [x] **Step 4: 실패하는 strict-input test 작성**

missing/wrong content type, invalid charset, unsupported encoding, invalid UTF-8, empty/malformed JSON, nested duplicate key, unknown field, trailing JSON, one-byte oversize, 모든 path의 request-body closure를 cover한다.

- [x] **Step 5: strict decoding 구현**

`mime.ParseMediaType`, `http.MaxBytesReader`, UTF-8 validation, duplicate-key rejection, `DisallowUnknownFields`, one-value EOF validation을 사용한다.

- [x] **Step 6: timeout/cancellation/redaction 증명**

blocking fake는 context completion에서 반환한다. adapter deadline이 408을 주는지, client cancellation이 unreliable response 없이 보존되는지, secret-like body/aggregate/metadata/reader error value가 public error나 log에 나타나지 않는지 assert한다.

- [x] **Step 7: GREEN, race, commit 실행**

~~~bash
gofmt -w examples/gin-audit-query-api/internal/auditquery/*.go
go test -count=1 ./examples/gin-audit-query-api/internal/auditquery -run 'Test(NewEngine|HTTP)'
go test -race -count=1 ./examples/gin-audit-query-api/internal/auditquery
git add examples/gin-audit-query-api/internal/auditquery
git commit -m "feat: expose audit queries through Gin"
~~~

## Task 5: runtime configuration 및 joined shutdown 소유

**Complexity:** High. **Depends on:** Tasks 3-4.

**Files:** `main.go`와 `main_test.go`를 생성한다.

- [x] **Step 1: 실패하는 runtime test 작성**

default, IPv4/IPv6 loopback, localhost, invalid/missing port, hostname/non-loopback rejection, explicit remote opt-in을 테스트한다. fixed header/read/write/idle/max-header value를 assert한다. fake server로 listen failure, normal shutdown, shutdown failure then close, close failure aggregation, listener join을 sleep이나 public listener 없이 증명한다.

- [x] **Step 2: RED 실행**

~~~bash
go test -count=1 ./examples/gin-audit-query-api -run 'Test(ResolveHTTPAddr|NewHTTPServer|RunServer)'
~~~

기대값: runtime function이 없으므로 FAIL한다.

- [x] **Step 3: wiring 및 lifecycle 구현**

memory repository를 seed하고 service/engine을 구성한다. Gin release mode를 사용하고 address를 resolve하며 `http.Server` 하나를 만든다. `signal.NotifyContext`를 사용한다. `RunServer`는 buffered result channel을 소유하고 shutdown 뒤 join하며 graceful failure에서는 `Close`를 강제한다.

- [x] **Step 4: GREEN, race, commit 실행**

~~~bash
gofmt -w examples/gin-audit-query-api/*.go
go test -count=1 ./examples/gin-audit-query-api -run 'Test(ResolveHTTPAddr|NewHTTPServer|RunServer)'
go test -race -count=1 ./examples/gin-audit-query-api
git add examples/gin-audit-query-api/main.go examples/gin-audit-query-api/main_test.go
git commit -m "feat: run Gin audit query server"
~~~

## Task 6: lesson 문서화 및 smoke-test

**Complexity:** Medium. **Depends on:** Tasks 1-5.

**Files:** 두 example README를 생성하고 두 root README를 수정한다.

- [x] **Step 1: source-equivalent locale docs 작성**

language switch, lesson, route, run command, POST/detail curl, response shape, 양방향 continuation, metadata-filter non-goal, loopback/remote behavior, auth/tenant/payload/retention/size/durability/rate-limit/health boundary를 포함한다.

- [x] **Step 2: 두 root navigation entry 추가**

각 root README에서 example을 `audit-order-history` 옆에 배치한다.

- [x] **Step 3: bounded live smoke 실행**

사용하지 않는 loopback port에서 시작하고 bounded readiness loop로 `/healthz`를 poll한다. 문서화된 POST 및 detail curl을 실행하고 `jq`로 validate한 뒤 terminate하고 process exit을 assert한다. fixed sleep은 readiness evidence가 아니다.

- [x] **Step 4: 검증 및 commit**

~~~bash
go test -count=1 ./examples/gin-audit-query-api/...
git diff --check
git add examples/gin-audit-query-api/README.md examples/gin-audit-query-api/README.ko.md README.md README.ko.md
git commit -m "docs: explain Gin audit query example"
~~~

## Task 7: 검증, review, 학습, delivery, merge, cleanup

**Complexity:** High. **Depends on:** Tasks 1-6.

- [x] **Step 1: fresh validation 실행**

~~~bash
gofmt -w examples/gin-audit-query-api/*.go examples/gin-audit-query-api/internal/auditquery/*.go
git diff --check
go test -count=1 ./examples/gin-audit-query-api/...
go test -race -count=1 ./examples/gin-audit-query-api/...
make ci
~~~

failure가 발생하면 owner로 되돌리고 affected proof를 처음부터 다시 실행해야 한다.

- [x] **Step 2: spec/plan 및 six review perspective 검증**

exact spec, plan, issue, diff, test, docs, triggered hazard를 다시 읽는다. 모든 acceptance criterion을 매핑한다. performance, stability, security, operator, developer/API, user/caller, main integration을 P0=0/P1=0으로 수렴한다.

- [x] **Step 3: lesson 및 review repair commit**

lesson은 aggregate-scoped POST 선택, metadata-filter rejection, first-unseen inclusive pagination, strict Gin ownership, actual proof, miss, future guard를 기록한다.

~~~bash
git add docs/lessons docs/review examples README.md README.ko.md
git commit -m "docs: record audit query API lessons"
~~~

- [ ] **Step 4: push 및 PR 생성**

issue metadata를 refresh한다. branch를 push하고 `debop`에게 assign된 English PR을 milestone `0.9.0` 및 mirrored label과 함께 생성한다. body는 `## DoD Status`로 끝내고 모든 live metadata, head/base, linked issue를 확인한다.

- [ ] **Step 5: CI 및 review state 대기**

모든 required check가 succeed해야 한다. review, comment, decision, unresolved thread를 다시 읽는다. failure/comment는 implementation과 fresh proof로 되돌아간다.

- [ ] **Step 6: approved authority 아래 rebase-merge**

head SHA, mergeability, check, P0/P1, review thread, authority를 즉시 refresh한다. rebase-merge하고 live state `MERGED`를 확인한다.

- [ ] **Step 7: owned state sync 및 cleanup**

새 user dirt를 보존한다. 실제 `develop`을 fast-forward하고 local/upstream SHA를 증명한다. integration ancestry/patch equivalence를 확인한 뒤 owned worktree를 remove, prune하고 integrated local branch를 삭제하며 clean/preserved state를 보고한다.

## Risk Prediction

| Risk | Signal | Mitigation | Rerun |
|---|---|---|---|
| Page gap/duplicate | repeated/missing revision | first-unseen inclusive boundary, 양방향/time filter | Task 2 tests/race |
| Sensitive leakage | output/log에 raw audit input/error 노출 | fixed message, route log, secret canary, loopback | Task 4 tests/race |
| Timeout confusion | client cancel 뒤 response | explicit deadline ownership과 blocking fake | Task 4 timeout/cancel |
| Listener leak | listener join 전에 return | result channel과 exact fake call | Task 5 lifecycle/race |
| Incorrect metadata filter | page incompleteness | request field 없음. 추가 전 spec 재개방 | stop implementation |
| README drift | curl/response mismatch | live bounded smoke | Task 6 smoke/tests |

## Plan Review Record

subagent interface가 mandatory installed `agent_type`을 제공할 수 없어서 main session이 여섯 isolated plan read를 수행하고 finding을 통합했다.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0 | bounded page/lookahead, response-budget docs, focused race, live smoke를 배정했다. |
| Stability | P0=0, P1=0 | cancellation, body closure, no-retry reader error, lifecycle failure, forced close, listener join에 ordered test가 있다. |
| Security | P0=0, P1=0 | strict decoding, path validation, proxy/loopback policy, canary redaction, trust-boundary docs가 명시적이다. |
| Operator/Ops | P0=0, P1=0 | health semantic, diagnostics, shutdown, rollback, CI, merge, sync, cleanup에 owner와 proof가 있다. |
| Developer/API | repair 이후 P0=0, P1=0 | zero-value method proof를 method가 존재한 뒤로 옮겼고 final formatter command가 Go file을 target하도록 고쳤다. |
| User/caller | P0=0, P1=0 | POST/body, continuation, detail/empty distinction, curl, unsupported metadata filter, 두 locale이 cover되어 있다. |
| Main integration | P0=0, P1=0 | 모든 spec criterion이 ordered task에 매핑된다. 나중 code에 의존하는 task가 없고 trigger된 모든 risk에 rerun point가 있다. |
