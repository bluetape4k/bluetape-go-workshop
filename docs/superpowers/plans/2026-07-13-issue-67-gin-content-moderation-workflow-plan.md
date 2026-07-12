# Gin Content Moderation Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Issue #67 Gin example that moderates and stores multilingual content, exposes inspectable records, and searches accepted records through bounded POST JSON requests.

**Architecture:** A framework-independent `moderationapi.Service` owns the reusable bluetape-go detector, Japanese and simple tokenizers, blockword dictionary, and bounded mutex-protected record store. A thin Gin adapter owns strict JSON/media-type handling, request deadlines, stable public errors, and safe diagnostics; `main` owns one HTTP server and bounded signal-driven shutdown.

**Tech Stack:** Go 1.25, bluetape-go v0.18.0 `textsearch`, `textsearch/japanese`, and `textsearch/language`, Gin v1.12.0, standard `net/http`, and bluetape-go concurrency test helpers.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/gin-content-moderation-workflow/internal/moderationapi/model.go` | Configuration, requests/responses, records, projections, sentinels, and deep-copy helpers |
| `examples/gin-content-moderation-workflow/internal/moderationapi/service.go` | Shared detector/tokenizers/dictionary, create/get/search policy, bounded store, and context checks |
| `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go` | Domain, byte-span, search, cancellation, copy-isolation, capacity, and race-safe concurrency tests |
| `examples/gin-content-moderation-workflow/internal/moderationapi/server.go` | Gin routes, strict bounded JSON, timeout/error mapping, health, and safe request diagnostics |
| `examples/gin-content-moderation-workflow/internal/moderationapi/server_test.go` | HTTP contract, body closure, media types, malformed input, cancellation, redaction, and route tests |
| `examples/gin-content-moderation-workflow/main.go` | App construction, loopback-safe address policy, `http.Server`, signals, and bounded shutdown |
| `examples/gin-content-moderation-workflow/main_test.go` | Address/server defaults and fake-server lifecycle tests without a public listener |
| `examples/gin-content-moderation-workflow/README.md` | English lesson, run/curl examples, boundaries, and validation |
| `examples/gin-content-moderation-workflow/README.ko.md` | Korean equivalent of the public lesson |
| `README.md`, `README.ko.md` | Root navigation entries |

The example is one testable subsystem and does not need a split plan. It adds no
module, dependency, database, container, workflow, coverage, changelog, public
bluetape-go API, or diagram. Rollback is removal of the example and root links;
all in-memory records are intentionally lost on every restart or rollback.

## Spec Coverage Matrix

| Approved spec area | Plan task | Primary evidence |
|---|---|---|
| Configuration, shared lifecycle, Go API, sentinels | Task 1 | constructor tests and focused package tests |
| Routing, evidence, masking, Japanese/simple terms, atomic create | Task 2 | create tables, UTF-8 span assertions, race run |
| Get, accepted-only metadata search, cursor/limit/copy semantics | Task 3 | get/search tables, cancellation, bounded stress |
| Strict JSON/media types/body closure/timeouts/public errors/health | Task 4 | `httptest`, tracking body, blocking workflow, redaction assertions |
| Loopback safety, server timeouts, signals, graceful/forced shutdown | Task 5 | fake-server lifecycle and race tests |
| English/Korean lesson, risks, curl examples, root discovery | Task 6 | locale parity inspection and runnable focused tests |
| P0/P1 review, full CI, rollback evidence, PR/issue closure | Task 7 | fresh `make ci`, PR CI, merged/synchronized SHA proof |

Conditional JVM, coroutine, Exposed, Spring Boot, streaming, database,
Testcontainers, migration, module-registration, BOM, and coverage-aggregation
checks are N/A because this is an existing-module in-memory Go example with no
new dependency or generated/published API.

### Task 1: Define the domain contract and reusable service construction

**Complexity:** Medium

**Dependencies:** Approved design commit `8f108d9`; no implementation task dependency.

**Write scope:** `model.go`, constructor portion of `service.go`, focused constructor tests only.

**Pattern and hazards:** Use `bluetape-go-patterns` constructor validation, sentinel wrapping, immutable shared components, NFC normalization, and caller-owned copies. Reject `NaN`/infinity explicitly. Do not import sibling example `internal` packages or reproduce tokenizer/matcher algorithms.

**Files:**

- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/model.go`
- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/service.go`
- Create: `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go`

- [ ] **Step 1: Write failing configuration and construction tests**

Add `TestDefaultConfig`, `TestNewServiceRejectsInvalidConfig`,
`TestNewServiceBuildsSharedComponents`, and `TestZeroValueServiceFailsClosed`.
The invalid table must exercise zero/negative integer limits, a search maximum
below 20, confidence below zero/above one, `math.NaN()`, and both infinities:

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

- [ ] **Step 2: Run the focused tests and observe RED**

Run:

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'Test(DefaultConfig|NewService|ZeroValue)'
```

Expected: FAIL because `DefaultConfig`, `NewService`, and domain types do not yet exist.

- [ ] **Step 3: Implement the minimum domain and constructor surface**

Define the exact spec types and defaults, including:

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

`NewService` must validate configuration, compile the five fixed blockword
entries with `IgnoreCase`, `NormalizeNFC`, and `BoundaryUnicodeWord`, construct
one four-language detector, one `japanese.Search` tokenizer, and one
`textsearch.NewSimpleTokenizer()`, install `time.Now` unless `WithClock` is
provided, and initialize the bounded map. A nil clock and zero-value receiver
return an error rather than panic.

Also assert `errors.Is` for every sentinel, preserved bluetape-go construction
cause, `context.Canceled`, and `context.DeadlineExceeded`; no test may depend on
matching formatted error text.

- [ ] **Step 4: Run focused tests and observe GREEN**

Run the Step 2 command.

Expected: PASS with constructor/default/zero-value cases covered.

- [ ] **Step 5: Format and commit the constructor slice**

```bash
gofmt -w examples/gin-content-moderation-workflow/internal/moderationapi/model.go examples/gin-content-moderation-workflow/internal/moderationapi/service.go examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go
git add examples/gin-content-moderation-workflow/internal/moderationapi
git commit -m "feat: initialize moderation workflow service"
```

### Task 2: Implement create routing, moderation, and immutable storage

**Complexity:** High

**Dependencies:** Task 1 domain and shared components.

**Write scope:** Create-path additions in `service.go` and create-focused tests in `service_test.go`.

**Pattern and hazards:** TDD each policy branch. Preserve original UTF-8 byte offsets, use `BlockwordDictionary.Process`, and project bluetape-go outputs rather than duplicating algorithms. Context is cooperative; after duplicate/capacity checks, call the non-blocking clock and recheck context under the lock immediately before commit.

**Files:**

- Modify: `examples/gin-content-moderation-workflow/internal/moderationapi/service.go`
- Modify: `examples/gin-content-moderation-workflow/internal/moderationapi/service_test.go`

- [ ] **Step 1: Write failing create-policy table tests**

Add table cases for allowed English, allowed Korean, masked English, masked
Korean with exact multibyte spans, Japanese with Kana and Search-mode token
spans, short text, mixed language, unknown input, Han-only ambiguity, and
unsupported Chinese. Assert that manual-review uses the fixed placeholder and
retains ordered evidence:

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

- [ ] **Step 2: Run create tests and observe RED**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestCreate'
```

Expected: FAIL because `Create` and policy projections are absent.

- [ ] **Step 3: Implement validation and routing helpers**

Use one path-safe ID helper with
`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`, UTF-8/rune validation, metadata-key
trimming with post-trim collision detection, and copied values. Implement the
Issue #119 reason order exactly:

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

Route confident non-mixed English/Korean to moderation, confident non-mixed
Japanese with Kana to Japanese preparation plus moderation, and every review
reason to `manual-review`. Check `ctx.Err()` before detector calls, moderation,
tokenization, and record stamping.

- [ ] **Step 4: Implement library-owned masking and term projections**

For supported content call `NewBlockwordRequest` and
`BlockwordDictionary.Process`. Convert matches to copied findings. Use one
shared `prepareTerms` helper for both future queries and records:

```go
var simpleTokenizeOptions = textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC}

func simpleIndexTerm(token textsearch.Token) (string, bool) {
    if token.POS != textsearch.POSWord && token.POS != textsearch.POSNumber { return "", false }
    term := strings.ToLower(token.Normalized)
    return term, term != ""
}
```

Japanese projection must select nouns/verbs, prefer a nonempty Kagome base
form, NFC-normalize it, retain original token byte spans, and stable-deduplicate
terms. Do not implement another tokenizer or offset mapper.

- [ ] **Step 5: Implement atomic duplicate/capacity/cancellation commit**

Build the complete unstamped candidate, check context, lock, check context
again, then check duplicate before capacity. Call the serialized non-blocking
clock only for an insertable candidate, stamp it, recheck context, and insert
one value. Deep-copy every returned map/slice. Add failing-then-green tests for
duplicate no-overwrite, capacity with duplicate precedence, metadata copying,
and immediate pre-commit cancellation via a clock that cancels its context;
assert that the final under-lock check prevents insertion.

- [ ] **Step 6: Run create tests and race validation**

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'Test(Create|Duplicate|Capacity|Metadata)'
go test -race -count=1 ./examples/gin-content-moderation-workflow/internal/moderationapi -run 'TestCreate'
```

Expected: PASS; multibyte slices equal original text and cancellation leaves the store unchanged.

- [ ] **Step 7: Format and commit the create workflow**

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
