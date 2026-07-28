# Gin Content Moderation Workflow Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** multilingual content를 moderate/store하고, inspectable record를 노출하며, bounded POST JSON request로 accepted record를 search하는 Issue #67 Gin example을 만든다.

**Architecture:** framework-independent `moderationapi.Service`가 reusable bluetape-go detector, Japanese/simple tokenizer, blockword dictionary, bounded mutex-protected record store를 소유한다. 얇은 Gin adapter는 strict JSON/media-type handling, request deadline, stable public error, safe diagnostic을 소유한다. `main`은 HTTP server 하나와 bounded signal-driven shutdown을 소유한다.

**Tech Stack:** Go 1.25, bluetape-go v0.18.0 `textsearch`, `textsearch/japanese`, `textsearch/language`, Gin v1.12.0, standard `net/http`, bluetape-go concurrency test helper를 사용한다.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/gin-content-moderation-workflow/internal/moderationapi/model.go` | Configuration, request/response, record, projection, sentinel, deep-copy helper를 정의한다. |
| `examples/gin-content-moderation-workflow/internal/moderationapi/service.go` | shared detector/tokenizer/dictionary, create/get/search policy, bounded store, context check를 구현한다. |
| `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go` | domain, byte-span, search, cancellation, copy-isolation, capacity, race-safe concurrency test를 담는다. |
| `examples/gin-content-moderation-workflow/internal/moderationapi/server.go` | Gin route, strict bounded JSON, timeout/error mapping, health, safe request diagnostic을 구현한다. |
| `examples/gin-content-moderation-workflow/internal/moderationapi/server_test.go` | HTTP contract, body closure, media type, malformed input, cancellation, redaction, route test를 담는다. |
| `examples/gin-content-moderation-workflow/main.go` | app construction, loopback-safe address policy, `http.Server`, signal, bounded shutdown을 담당한다. |
| `examples/gin-content-moderation-workflow/main_test.go` | public listener 없이 address/server default와 fake-server lifecycle test를 검증한다. |
| `examples/gin-content-moderation-workflow/README.md` | English lesson, run/curl example, boundary, validation을 설명한다. |
| `examples/gin-content-moderation-workflow/README.ko.md` | public lesson과 동등한 Korean 문서를 제공한다. |
| `README.md`, `README.ko.md` | root navigation entry를 갱신한다. |

이 example은 testable subsystem 하나이며 split plan이 필요하지 않다. module, dependency, database, container,
workflow, coverage, changelog, public bluetape-go API, diagram을 추가하지 않는다. rollback은 example과 root link를 제거하는 것이다.
모든 in-memory record는 restart 또는 rollback마다 의도적으로 사라진다.

## Spec Coverage Matrix

| Approved spec area | Plan task | Primary evidence |
|---|---|---|
| Configuration, shared lifecycle, Go API, sentinels | Task 1 | constructor test와 focused package test |
| Routing, evidence, masking, Japanese/simple terms, atomic create | Task 2 | create table, UTF-8 span assertion, race run |
| Get, accepted-only metadata search, cursor/limit/copy semantics | Task 3 | get/search table, cancellation, bounded stress |
| Strict JSON/media types/body closure/timeouts/public errors/health | Task 4 | `httptest`, tracking body, blocking workflow, redaction assertion |
| Loopback safety, server timeouts, signals, graceful/forced shutdown | Task 5 | fake-server lifecycle 및 race test |
| English/Korean lesson, risks, curl examples, root discovery | Task 6 | locale parity inspection 및 runnable focused test |
| P0/P1 review, full CI, rollback evidence, PR/issue closure | Task 7 | fresh `make ci`, PR CI, merged/synchronized SHA proof |

conditional JVM, coroutine, Exposed, Spring Boot, streaming, database, Testcontainers,
migration, module-registration, BOM, coverage-aggregation check는 N/A이다. 이 작업은 new dependency나
generated/published API가 없는 existing-module in-memory Go example이기 때문이다.

### Task 1: domain contract 및 reusable service construction 정의

**Complexity:** Medium

**Dependencies:** approved design commit `8f108d9`; implementation task dependency는 없다.

**Write scope:** `model.go`, `service.go`의 constructor portion, focused constructor test만 포함한다.

**Pattern and hazards:** `bluetape-go-patterns`의 constructor validation, sentinel wrapping, immutable shared component, NFC normalization, caller-owned copy를 사용한다. `NaN`/infinity는 명시적으로 reject한다. sibling example `internal` package를 import하거나 tokenizer/matcher algorithm을 재구현하지 않는다.

**Files:**

- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/model.go`
- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/service.go`
- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go`

- [ ] **Step 1: 실패하는 configuration 및 construction test 작성**

`TestDefaultConfig`, `TestNewServiceRejectsInvalidConfig`, `TestNewServiceBuildsSharedComponents`,
`TestZeroValueServiceFailsClosed`를 추가한다. invalid table은 zero/negative integer limit, 20 미만 search maximum,
0 미만/1 초과 confidence, `math.NaN()`, 양쪽 infinity를 실행해야 한다.

```go
func TestNewServiceRejectsInvalidConfidence(t *testing.T) {
    for _, confidence := range []float64{-0.01, 1.01, math.NaN(), math.Inf(-1), math.Inf(1)} {
        cfg := DefaultConfig().Service
        cfg.MinimumConfidence = confidence
        _, err := NewService(cfg)
        if !errors.Is(err, ErrInvalidConfig) {
            t.Fatalf("confidence %v: err = %v, want ErrInvalidConfig", confidence, err)
        }
    }
}
```

- [ ] **Step 2: focused test 실행 및 RED 확인**

실행한다.

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'Test(DefaultConfig|NewService|ZeroValue)'
```

기대값: `DefaultConfig`, `NewService`, domain type이 아직 없으므로 FAIL한다.

- [ ] **Step 3: minimum domain 및 constructor surface 구현**

exact spec type과 default를 정의한다. 다음을 포함한다.

```go
var (
    ErrInvalidConfig      = errors.New("moderationapi: invalid config")
    ErrInvalidRequest     = errors.New("moderationapi: invalid request")
    ErrDuplicateContentID = errors.New("moderationapi: duplicate content id")
    ErrRecordNotFound     = errors.New("moderationapi: record not found")
    ErrStoreCapacity      = errors.New("moderationapi: store capacity reached")
    ErrWorkflow           = errors.New("moderationapi: workflow failure")
)

const (
    OutcomeAllowed      Outcome = "allowed"
    OutcomeMasked       Outcome = "masked"
    OutcomeManualReview Outcome = "manual-review"
)

func DefaultConfig() AppConfig {
    return AppConfig{
        Service: ServiceConfig{MinimumConfidence: 0.70, MinimumRunes: 8, MaximumContentRunes: 8_000, MaximumRecords: 1_000, MaximumSearchResults: 100},
        HTTP: HTTPConfig{MaximumBodyBytes: 64 << 10, RequestTimeout: 2 * time.Second},
    }
}
```

`NewService`는 configuration을 validate하고, `IgnoreCase`, `NormalizeNFC`, `BoundaryUnicodeWord`로 fixed blockword
entry 다섯 개를 compile해야 한다. four-language detector 하나, `japanese.Search` tokenizer 하나,
`textsearch.NewSimpleTokenizer()` 하나를 만들고, `WithClock`이 없으면 `time.Now`를 설치하며, bounded map을 initialize한다.
nil clock과 zero-value receiver는 panic 대신 error를 반환한다.

모든 sentinel, 보존된 bluetape-go construction cause, `context.Canceled`, `context.DeadlineExceeded`에 대해
`errors.Is`를 assert한다. 어떤 test도 formatted error text matching에 의존하면 안 된다.

- [ ] **Step 4: focused test 실행 및 GREEN 확인**

Step 2 command를 실행한다.

기대값: constructor/default/zero-value case가 cover되고 PASS한다.

- [ ] **Step 5: constructor slice format 및 commit**

```bash
gofmt -w examples/gin-content-moderation-workflow/internal/moderationapi/model.go examples/gin-content-moderation-workflow/internal/moderationapi/service.go examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go
git add examples/gin-content-moderation-workflow/internal/moderationapi
git commit -m "feat: initialize moderation workflow service"
```

### Task 2: create routing, moderation, immutable storage 구현

**Complexity:** High

**Dependencies:** Task 1 domain 및 shared component.

**Write scope:** `service.go`의 create-path addition과 `service_test.go`의 create-focused test.

**Pattern and hazards:** 각 policy branch를 TDD로 진행한다. original UTF-8 byte offset을 보존하고 `BlockwordDictionary.Process`를 사용하며, algorithm을 중복하지 말고 bluetape-go output을 project한다. context는 cooperative이다. duplicate/capacity check 뒤 non-blocking clock을 호출하고, commit 직전 lock 안에서 context를 다시 확인한다.

**Files:**

- Modify: `examples/gin-content-moderation-workflow/internal/moderationapi/service.go`
- Modify: `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go`

- [ ] **Step 1: 실패하는 create-policy table test 작성**

allowed English, allowed Korean, masked English, exact multibyte span을 가진 masked Korean,
Kana와 Search-mode token span을 가진 Japanese, short text, mixed language, unknown input,
Han-only ambiguity, unsupported Chinese table case를 추가한다. manual-review가 fixed placeholder를 사용하고
ordered evidence를 보존하는지 assert한다.

```go
func TestCreateMasksKoreanAndPreservesByteSpan(t *testing.T) {
    svc := newTestService(t)
    record, err := svc.Create(context.Background(), CreateRequest{ContentID: "ko-1", Content: "배송 욕설 문의입니다"})
    if err != nil { t.Fatal(err) }
    if record.Outcome != OutcomeMasked || record.DisplayText != "배송 ** 문의입니다" { t.Fatalf("record = %+v", record) }
    finding := record.Findings[0]
    if record.Content[finding.Start:finding.End] != "욕설" { t.Fatalf("span = %d:%d", finding.Start, finding.End) }
}
```

- [ ] **Step 2: create test 실행 및 RED 확인**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestCreate'
```

기대값: `Create`와 policy projection이 없으므로 FAIL한다.

- [ ] **Step 3: validation 및 routing helper 구현**

`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`를 사용하는 path-safe ID helper 하나, UTF-8/rune validation,
post-trim collision detection을 가진 metadata-key trimming, copied value를 사용한다. Issue #119 reason order를 정확히 구현한다.

```go
var reviewReasonOrder = []ReviewReason{
    ReasonTextTooShort,
    ReasonLanguageUnknown,
    ReasonLowConfidence,
    ReasonMixedLanguage,
    ReasonAmbiguousCJKScript,
    ReasonUnsupportedLanguage,
}
```

confident non-mixed English/Korean은 moderation으로 route한다. Kana가 있는 confident non-mixed Japanese는
Japanese preparation과 moderation으로 route한다. 모든 review reason은 `manual-review`로 보낸다.
detector call, moderation, tokenization, record stamping 전에 `ctx.Err()`를 확인한다.

- [ ] **Step 4: library-owned masking 및 term projection 구현**

supported content에는 `NewBlockwordRequest`와 `BlockwordDictionary.Process`를 호출한다.
match를 copied finding으로 변환한다. future query와 record 모두에 shared `prepareTerms` helper 하나를 사용한다.

```go
var simpleTokenizeOptions = textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC}

func simpleIndexTerm(token textsearch.Token) (string, bool) {
    if token.POS != textsearch.POSWord && token.POS != textsearch.POSNumber { return "", false }
    term := strings.ToLower(token.Normalized)
    return term, term != ""
}
```

Japanese projection은 noun/verb를 선택하고, nonempty Kagome base form을 우선하며, NFC-normalize하고,
original token byte span을 유지하며, term을 stable-deduplicate해야 한다. 다른 tokenizer나 offset mapper를 구현하지 않는다.

- [ ] **Step 5: atomic duplicate/capacity/cancellation commit 구현**

complete unstamped candidate를 만들고 context를 확인한 뒤 lock을 잡고 context를 다시 확인한다. capacity보다 duplicate를 먼저 확인한다.
insert 가능한 candidate에 대해서만 serialized non-blocking clock을 호출하고 stamp한 뒤 context를 다시 확인하고 value 하나를 insert한다.
반환되는 모든 map/slice는 deep-copy한다. duplicate no-overwrite, duplicate precedence가 있는 capacity,
metadata copying, context를 cancel하는 clock을 통한 immediate pre-commit cancellation에 대해 failing-then-green test를 추가한다.
final under-lock check가 insertion을 막는지 assert한다.

- [ ] **Step 6: create test 및 race validation 실행**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'Test(Create|Duplicate|Capacity|Metadata)'
go test -race -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestCreate'
```

기대값: PASS. multibyte slice는 original text와 같고 cancellation은 store를 변경하지 않는다.

- [ ] **Step 7: create workflow format 및 commit**

```bash
gofmt -w examples/gin-content-moderation-workflow/internal/moderationapi/*.go
git add examples/gin-content-moderation-workflow/internal/moderationapi
git commit -m "feat: create multilingual moderation records"
```

### Task 3: Implement get, bounded cursor search, and concurrent reuse

**Complexity:** High

**Dependencies:** Task 2 immutable stored records and shared term projection.

**Write scope:** Get/search and concurrency additions only.

**Pattern and hazards:** Snapshot immutable pointers under a short read lock, scan outside it, check context every 32 entries, sort before limit, and return deep copies. No durable or inverted index. Cursor pagination is append-aware but not snapshot-isolated.

**Files:**

- Modify: `examples/gin-content-moderation-workflow/internal/moderationapi/service.go`
- Modify: `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go`

- [ ] **Step 1: Write failing get and search contract tests**

Cover missing/invalid IDs, original-content by-ID visibility, accepted-only
search, exact metadata AND filtering, all-distinct-term semantics, Korean and
English case/NFC parity, Japanese Search-mode queries, stable ID ordering,
default/custom limits, exclusive cursor continuation, `truncated`, empty
non-nil results, and copy isolation:

```go
func TestSearchExcludesManualReviewAndPaginates(t *testing.T) {
    svc := seededSearchService(t)
    first, err := svc.Search(context.Background(), SearchRequest{Query: "delivery", Limit: 1})
    if err != nil { t.Fatal(err) }
    if len(first.Hits) != 1 || !first.Truncated || first.NextAfterContentID == "" { t.Fatalf("first = %+v", first) }
    second, err := svc.Search(context.Background(), SearchRequest{Query: "delivery", AfterContentID: first.NextAfterContentID, Limit: 1})
    if err != nil { t.Fatal(err) }
    if len(second.Hits) != 1 || second.Hits[0].ContentID <= first.Hits[0].ContentID { t.Fatalf("second = %+v", second) }
}
```

- [ ] **Step 2: Run get/search tests and observe RED**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'Test(Get|Search)'
```

Expected: FAIL because `Get`/`Search` are absent.

- [ ] **Step 3: Implement Get and Search**

`Get` validates the canonical ID, checks context, copies the stored record, and
returns `ErrRecordNotFound` when absent. `Search` validates UTF-8, 8,000-rune
query, metadata, configured limit, and cursor; prepares distinct query terms
once; snapshots at most `MaximumRecords` immutable pointers; checks context on
entry zero and every 32 entries; selects only allowed/masked all-term matches
whose IDs exceed the cursor; sorts; computes the next cursor only when another
match exists; and returns at most the effective limit without raw content,
findings, or stored terms.

- [ ] **Step 4: Add deterministic cancellation and concurrency tests**

Use a counting context that begins returning `context.Canceled` at a scan
checkpoint. Use `testing/concurrency.NewGoroutineStressTester` and a bounded
ready gate to run creates, gets, and searches against one service. Assert one
winner for one duplicate ID, exact completion counts, writer progress during
search, stable outcomes, and mutation isolation. Do not use sleeps.

Add a table-driven pre-canceled-context test for `Create`, `Get`, and `Search`.
Every method must preserve `errors.Is(err, context.Canceled)`; `Create` must also
prove that no record was inserted.

- [ ] **Step 5: Run focused and race tests**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'Test(Get|Search|Concurrent)'
go test -race -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi
```

Expected: PASS with no race, deadlock, timeout, or leaked worker.

- [ ] **Step 6: Format and commit query behavior**

```bash
gofmt -w examples/gin-content-moderation-workflow/internal/moderationapi/*.go
git add examples/gin-content-moderation-workflow/internal/moderationapi
git commit -m "feat: search accepted moderation records"
```

### Task 4: Add the strict Gin HTTP boundary

**Complexity:** High

**Dependencies:** Task 3 complete `Workflow` contract.

**Write scope:** `server.go`, `server_test.go`; no service policy changes.

**Pattern and hazards:** Gin exists because it is the lesson. Set trusted proxies to nil. Enforce body limits before buffering, reject duplicate keys and unknown fields, close bodies on every path, do not log caller content, and never write after client cancellation.

**Files:**

- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/server.go`
- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/server_test.go`

- [ ] **Step 1: Write failing route and success-response tests**

Create a narrow fake `Workflow` and test `201` create, `200` by-ID get,
POST-search pagination JSON, liveness, method misses, original-content presence
only on create/get, and its absence from search. Verify `engine.SetTrustedProxies(nil)` succeeds and no proxy is trusted.

- [ ] **Step 2: Run server tests and observe RED**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestHTTP'
```

Expected: FAIL because `NewEngine` and handlers are absent.

- [ ] **Step 3: Implement strict bounded JSON decoding**

At handler entry `defer c.Request.Body.Close()`. Parse media type with
`mime.ParseMediaType`; accept only `application/json` and absent/UTF-8 charset.
Reject non-identity `Content-Encoding`. Wrap the body with
`http.MaxBytesReader(c.Writer, body, MaximumBodyBytes)`, read the bounded bytes,
run a JSON-token duplicate-key validator for the root and metadata object, then
reject non-UTF-8 body bytes, decode with `DisallowUnknownFields`, and require
EOF. Map wrapped service errors with `errors.Is`, never direct error equality:

```go
func mapWorkflowError(err error) publicError {
    switch {
    case errors.Is(err, ErrInvalidRequest):
        return publicError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "request is invalid"}
    case errors.Is(err, ErrDuplicateContentID):
        return publicError{Status: http.StatusConflict, Code: "duplicate_content_id", Message: "content_id already exists"}
    case errors.Is(err, ErrRecordNotFound):
        return publicError{Status: http.StatusNotFound, Code: "record_not_found", Message: "record was not found"}
    case errors.Is(err, ErrStoreCapacity):
        return publicError{Status: http.StatusServiceUnavailable, Code: "store_capacity_reached", Message: "record capacity is reached"}
    default:
        return publicError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "internal error"}
    }
}
```

Keep `request_too_large`, `request_timeout`, `unsupported_media_type`,
`unsupported_content_encoding`, and `internal_error` stable and separate.

- [ ] **Step 4: Implement deadlines, error mapping, and safe diagnostics**

Derive `context.WithTimeout(c.Request.Context(), RequestTimeout)` for every
workflow call. Map `context.DeadlineExceeded` to 408. When the parent request is
canceled, return without attempting another write. Log only route pattern,
method, status, stable code, and elapsed duration. Never log ID, content,
display text, metadata, findings, terms, body bytes, or wrapped workflow cause.

- [ ] **Step 5: Add negative boundary tests**

Cover malformed/trailing/unknown/duplicate-key JSON, duplicate metadata keys,
invalid UTF-8 JSON strings,
missing/wrong media type, invalid charset, content encoding, exactly-at-limit
and over-limit bodies, tracking-body closure on every early return, escaped path
separators, every stable error mapping, a workflow blocked until its context is
done, parent cancellation without response rewrite, and logger redaction using
sentinel secret strings.

- [ ] **Step 6: Run HTTP tests and race validation**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestHTTP'
go test -race -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestHTTP'
```

Expected: PASS; body-close counters equal one and captured logs contain none of the sentinel secrets.

- [ ] **Step 7: Format and commit the Gin adapter**

```bash
gofmt -w examples/gin-content-moderation-workflow/internal/moderationapi/*.go
git add examples/gin-content-moderation-workflow/internal/moderationapi
git commit -m "feat: expose Gin moderation workflow API"
```

### Task 5: Add application lifecycle and runnable server

**Complexity:** Medium

**Dependencies:** Task 4 engine construction.

**Write scope:** `main.go`, `main_test.go` only.

**Pattern and hazards:** One application-owned service/server. Default loopback, explicit insecure remote opt-in, fixed server timeouts, `SIGINT`/`SIGTERM`, five-second drain, forced close, joined listen result, safe logs, and non-zero failures. No service close method or background goroutine.

**Files:**

- Create: `examples/gin-content-moderation-workflow/main.go`
- Create: `examples/gin-content-moderation-workflow/main_test.go`

- [ ] **Step 1: Write failing address and server-default tests**

Test default `127.0.0.1:8080` and accepted `127.0.0.1`/`[::1]` loopback
overrides. Use a table to reject `:8080`, `0.0.0.0:8080`, `[::]:8080`, a
representative private IPv4 address, and a representative public IPv4 address
without `ALLOW_UNAUTHENTICATED_REMOTE=1`; assert each is accepted only with the
explicit opt-in. Also assert exact `ReadHeaderTimeout=2s`, `ReadTimeout=5s`,
`WriteTimeout=5s`, and `IdleTimeout=30s`.

- [ ] **Step 2: Write failing fake-server lifecycle tests**

Define a test fake implementing `ListenAndServe`, `Shutdown`, and `Close` with
channels. Cover early listen failure, `http.ErrServerClosed`, graceful context
cancellation, shutdown deadline and forced close, forced-close failure, listen
result joining, no leaked goroutine, and safe lifecycle log fields.

- [ ] **Step 3: Run main tests and observe RED**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow -run 'Test(Address|HTTPServer|RunServer)'
```

Expected: FAIL because lifecycle helpers are absent.

- [ ] **Step 4: Implement `main`, `NewHTTPServer`, and `RunServer`**

Build `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`,
construct one service/engine/server, and run it. `RunServer` starts exactly one
listen goroutine, selects listen completion versus context completion, calls
`Shutdown` with a five-second background timeout, calls `Close` after deadline,
always joins the listen result, and returns an error for every non-normal
startup/listen/shutdown/close path. Operational logs follow the spec allowlist.

- [ ] **Step 5: Run focused and race tests**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow
go test -race -count=1 ./examples/gin-content-moderation-workflow/...
```

Expected: PASS without binding a public port or leaking a lifecycle goroutine.

- [ ] **Step 6: Commit the runnable application**

```bash
gofmt -w examples/gin-content-moderation-workflow/*.go
git add examples/gin-content-moderation-workflow/main.go examples/gin-content-moderation-workflow/main_test.go
git commit -m "feat: run moderation workflow server"
```

### Task 6: Add bilingual public documentation and navigation

**Complexity:** Medium

**Dependencies:** Tasks 1-5 final command names, payloads, and behavior.

**Write scope:** Example README pair and two root navigation files only.

**Pattern and hazards:** Use `bluetape-writer`; keep English/Korean structure and examples equivalent. Never describe language detection as certainty or moderation as a security/compliance boundary. Show the unauthenticated original-content risk and remote-bind opt-in prominently.

**Files:**

- Create: `examples/gin-content-moderation-workflow/README.md`
- Create: `examples/gin-content-moderation-workflow/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **Step 1: Write both README files**

Each locale must include the prerequisite links (#53, #54, #55, #118, #119),
package lesson, architecture ownership, run command, default loopback address,
create/get/search/health curl commands, metadata exact-match and cursor example,
expected allowed/masked/manual-review responses, byte-span explanation, all
stable error codes, lifecycle/concurrency bounds, 64 KiB/8,000-rune/2-second/
1,000-record/20-default/100-maximum limits, validation commands, restart data
loss, linear-search limitation, heuristic boundary, original-content exposure,
and `ALLOW_UNAUTHENTICATED_REMOTE=1` warning.
Both locales must also state that metadata is caller-owned exact-match data,
appears in create/get/search responses, and must not contain credentials or
secrets.

- [ ] **Step 2: Add matching root navigation entries**

Place the example in the text-search/multilingual sequence after Issue #119 in
both root READMEs. Use the same description meaning in both languages and a
relative link to the corresponding example README.

- [ ] **Step 3: Verify public examples against the application**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/...
rg -n 'gin-content-moderation-workflow|POST /moderation/records/search|ALLOW_UNAUTHENTICATED_REMOTE' README.md README.ko.md examples/gin-content-moderation-workflow/README*.md
```

Expected: tests PASS and every required phrase appears in both locale surfaces.

- [ ] **Step 4: Commit documentation**

```bash
git add README.md README.ko.md examples/gin-content-moderation-workflow/README.md examples/gin-content-moderation-workflow/README.ko.md
git commit -m "docs: explain Gin moderation workflow"
```

### Task 7: Validate, review, and prepare integration

**Complexity:** High

**Dependencies:** Tasks 1-6 complete.

**Write scope:** Fix only evidence-backed findings in Issue #67 files; do not change dependencies, workflows, coverage, or unrelated examples.

**Risk prediction:** Highest risks are UTF-8 span corruption, context cancellation committing a record, duplicate/capacity precedence races, strict JSON bypass, sensitive diagnostic leakage, and shutdown goroutine leakage. Their signals and rerun points are the focused span/cancellation/concurrency/HTTP/lifecycle tests before the full gate. Roll back the responsible task commit if a repair expands scope beyond the approved design.

- [ ] **Step 1: Run formatting, dependency, static, focused, full, and race gates**

```bash
make fmt-check
make tidy-check
go test -count=1 ./examples/gin-content-moderation-workflow/...
go test -race -count=1 ./examples/gin-content-moderation-workflow/...
make vet
make lint
go test -count=1 ./...
make ci
```

Expected: every command exits 0 from a fresh invocation. Lost handles or missing exit codes are not evidence; rerun those commands.

- [ ] **Step 2: Run six-lens implementation review and quality review**

Review performance, stability, security, operator/ops, developer/API, and
user/caller behavior against the approved spec. Require P0=0/P1=0, fix or
explicitly defer every P2/P3 with rationale, rerun affected focused tests and
`make ci`, then run fresh code-quality and bilingual-documentation reviews.

- [ ] **Step 3: Record the lesson and final branch evidence**

Confirm no diagram, changelog, dependency, workflow, module registration,
container, database, or public-library release artifact is required. Record the
exact focused/race/full commands, clean status, commit range, and Issue #67
acceptance mapping.

- [ ] **Step 4: Commit review fixes, if any**

```bash
git add examples/gin-content-moderation-workflow README.md README.ko.md
git commit -m "fix: harden Gin moderation workflow example"
```

Skip this commit when the review produces no file changes.

- [ ] **Step 5: Push, open the PR, wait for CI, merge, and synchronize**

After local P0=0/P1=0 and `make ci` success:

```bash
git push -u origin feat/issue-67-gin-content-moderation-workflow
gh pr create --base develop --head feat/issue-67-gin-content-moderation-workflow --title "feat: add Gin content moderation workflow" --body $'Closes #67\n\n## Summary\n- add the Gin moderation workflow example\n- compose v0.18.0 language, Japanese, and blockword APIs\n- document bounded JSON search and lifecycle behavior\n\n## Validation\n- make ci\n- go test -race -count=1 ./examples/gin-content-moderation-workflow/...'
pr_number=$(gh pr view --json number --jq .number)
gh pr checks --watch "$pr_number"
gh pr merge "$pr_number" --rebase --delete-branch
git -C /Users/debop/work/bluetape4k/bluetape-go-workshop pull --ff-only origin develop
cd /Users/debop/work/bluetape4k/bluetape-go-workshop
git worktree remove .worktrees/feat-issue-67-gin-content-moderation-workflow
git branch -d feat/issue-67-gin-content-moderation-workflow
gh issue comment 34 --body "Issue #67 is complete and merged; all text-track integration prerequisites are now delivered."
git rev-parse develop
git rev-parse origin/develop
```

Expected: required CI checks succeed, the PR is rebase-merged, Issue #67 closes,
umbrella #34 reflects completion, local `develop` equals `origin/develop`, and
the feature worktree and local feature branch are removed only after merge.
