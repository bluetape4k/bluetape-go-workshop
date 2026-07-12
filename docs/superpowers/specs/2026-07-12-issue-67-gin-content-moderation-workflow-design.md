# Issue #67 Gin Content Moderation Workflow Design

## Status

User-approved design for Issue #67, based on bluetape-go v0.18.0. The Type A
specification review has converged at P0=0/P1=0. Implementation planning remains
blocked until the user reviews this committed artifact.

## Goal

Add an application-shaped Gin example that accepts multilingual content,
routes it with shared language-detection evidence, moderates supported text,
prepares Japanese search terms, stores an inspectable in-memory record, and
searches accepted records through a JSON request.

The lesson is composition at a production-shaped HTTP boundary: one
application-owned detector and tokenizer, a framework-independent service,
bounded JSON input, cooperative request deadlines, stable errors,
cancellation-aware commit, and concurrent reuse. It demonstrates policy and
lifecycle; it does not claim to provide a production moderation, privacy, or
durable-search system.

## Context and Current Evidence

This is the final integration example under milestone epic #34. Its completed
prerequisites supply the individual lessons:

- Issue #53 introduced blockword moderation and masking;
- Issue #54 introduced text-search matching and byte-span evidence;
- Issue #55 established multilingual intake feasibility;
- Issue #118 isolated Japanese search preparation; and
- Issue #119 defined confidence- and script-aware language routing.

The example composes bluetape-go v0.18.0 directly. It cannot import sibling
example `internal` packages, and it must not copy their package algorithms.
The relevant library surfaces are the text-search blockword matcher and masking
helpers, `textsearch.SimpleTokenizer` for Unicode-word preparation, the
language detector and script hints, and the Japanese Search-mode tokenizer and
token byte spans.

## Chosen Approach

Use a thin Gin adapter over one framework-independent `Service`. The service
owns one detector, one Japanese tokenizer, one stateless simple tokenizer, one
immutable moderation policy, and one mutex-protected in-memory record map.
Request-specific evidence and result slices remain call-local. HTTP handlers
own JSON limits, strict decoding, timeouts, body closure, and error-to-status
mapping.

This keeps the example application-shaped without turning workshop code into a
generic framework. A narrow service interface at the Gin boundary permits
deterministic cancellation and error-mapping tests without sleeps or mutable
production hooks.

## Rejected Alternatives

### Fully pluggable stage pipeline

Rejected because separate interfaces for detector, router, moderator,
tokenizer, and repository would make substitution easy but obscure the
composition lesson behind framework-like abstraction.

### One monolithic Gin handler

Rejected because it couples transport and policy, makes cancellation before
commit difficult to prove, and prevents direct service testing.

### Import sibling workshop examples

Rejected because their `internal` packages are intentionally inaccessible and
are not reusable libraries. The integration must use bluetape-go public APIs
and retain only application policy and projections locally.

## Package and Files

The implementation will live under:

```text
examples/gin-content-moderation-workflow/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/moderationapi/
    model.go
    service.go
    service_test.go
    server.go
    server_test.go
```

The root `README.md` and `README.ko.md` will link the example. No dependency,
module, workflow, container, database, or coverage-policy change is required.

## Configuration and Lifecycle

Application configuration owns these explicit defaults:

```go
type AppConfig struct {
    Service ServiceConfig
    HTTP    HTTPConfig
}

type ServiceConfig struct {
    MinimumConfidence   float64
    MinimumRunes        int
    MaximumContentRunes int
    MaximumRecords      int
    MaximumSearchResults int
}

type HTTPConfig struct {
    MaximumBodyBytes int64
    RequestTimeout   time.Duration
}
```

Defaults are minimum confidence `0.70`, minimum routing length `8` runes,
maximum content length `8,000` runes, maximum JSON body size `64 KiB`, and a
request timeout of two seconds. The teaching store holds at most 1,000 records;
a search defaults to 20 results and may request at most the configured maximum,
which defaults to 100 and cannot be below 20. Invalid non-positive limits or
timeout fail construction. Minimum confidence must be finite and in `[0,1]`;
constructors explicitly reject `NaN` and either infinity.

`NewService` constructs exactly one language detector for English, Korean,
Japanese, and Chinese, one Japanese tokenizer configured in Search mode, one
stateless simple tokenizer, and one moderation matcher. They live for the
application lifetime and are never created per request. `main` constructs one
service and one Gin engine.

The HTTP process binds to `127.0.0.1:8080` by default; `HTTP_ADDR` may explicitly
override it. A non-loopback address is rejected unless
`ALLOW_UNAUTHENTICATED_REMOTE=1` is also set, making the example's missing auth
boundary an explicit opt-in. Its `http.Server` uses a two-second read-header
timeout, five-second read timeout, five-second write timeout, and 30-second idle
timeout. Gin trusted proxies are set to `nil`.

`main` treats construction and non-`ErrServerClosed` listen failures as fatal.
It handles `SIGINT` and `SIGTERM`, gives `Server.Shutdown` five seconds to drain,
then calls `Server.Close` if the deadline expires. Shutdown or forced-close
failures are logged safely and produce a non-zero exit. Lifecycle logic is
factored behind a narrow local server interface so tests drive signals and
deadlines without binding a public port. The service owns no background
goroutine or external resource, so no artificial service `Close` method is
added.

## HTTP Contract

### Create a moderation record

```http
POST /moderation/records
Content-Type: application/json
```

```json
{
  "content_id": "article-123",
  "content": "배송 문의입니다",
  "metadata": {
    "tenant_id": "demo",
    "category": "support"
  }
}
```

`content_id` is trimmed, then must match
`[A-Za-z0-9][A-Za-z0-9._-]{0,127}`. The same canonicalization and validation
apply to lookup and search cursors, so every accepted ID round-trips safely as a
Gin path segment; encoded separators, controls, and other escaped path syntax
are rejected. `content` is preserved byte for byte so reported spans slice the
submitted UTF-8 bytes; blank or over-limit content is rejected. Metadata is
optional, string-to-string, copied on input, and stored with the record. Keys
are trimmed and must be nonblank; two source keys that collide after trimming
are invalid. Values are preserved rather than trimmed because they are
caller-owned exact-match dimensions. Metadata is observable in API responses
and must not contain credentials or secrets.

Creation returns `201 Created` and the complete stored record. Reusing an
existing canonical `content_id` returns `409 Conflict`; it never overwrites the
record, even when the payload is identical. A new ID after the teaching store
reaches its configured capacity returns `503 Service Unavailable`; duplicate
checking takes precedence so a known ID remains a conflict at capacity.

### Get one record

```http
GET /moderation/records/:content_id
```

The response includes original content, display text, metadata, routing and
language evidence, findings, Japanese terms, outcome, and timestamps. This is
an intentionally inspectable teaching endpoint. The README states that a real
application must own authentication, authorization, retention, audit logging,
and log redaction before exposing original content.

### Search accepted records

```http
POST /moderation/records/search
Content-Type: application/json
```

```json
{
  "query": "배송 문의",
  "limit": 20,
  "after_content_id": "article-100",
  "metadata": {
    "tenant_id": "demo"
  }
}
```

The JSON body avoids URI-length constraints and supports extensible filters.
`query` must be valid UTF-8, nonblank, and no longer than 8,000 runes. Metadata
uses the same key validation as creation. Every supplied metadata key/value
must exactly match the stored record. A missing or zero `limit` uses 20; values
above `ServiceConfig.MaximumSearchResults` or below zero are invalid.
`after_content_id` is optional and exclusive; when present, only canonical IDs
lexically greater than it are eligible. Search includes only `allowed` and
`masked` records; `manual-review` records are never searchable.

Japanese queries use terms from the shared Search-mode tokenizer. English and
Korean queries use normalized Unicode-word terms. All distinct query terms are
required. Search responses contain `content_id`, outcome, `display_text`,
metadata, and detected language, but omit original content, prepared terms, and
raw findings. Results are sorted by canonical `content_id`, truncated to the
effective limit, and returned with `truncated` plus `next_after_content_id` set
to the final returned ID when another match exists. Passing that value as the
next exclusive cursor continues the existing ordered result set without
duplicates. The in-memory endpoint has no snapshot isolation: concurrent
inserts may appear on later pages or be skipped when their ID sorts before the
cursor. Empty results are a successful non-nil JSON array.

### Health

`GET /healthz` is liveness-only. It returns a small stable `200 OK` response and
does not run the detector, tokenizer, or moderation pipeline. Store capacity
does not change liveness. No readiness endpoint is provided because the example
has no external dependency.

## Strict JSON and Request Bounds

Both POST endpoints require `Content-Type: application/json`, optionally with a
case-insensitive `charset=utf-8` parameter. Missing, malformed, or other media
types return `415 Unsupported Media Type` before reading the body. A nonempty
`Content-Encoding` other than `identity` is rejected with
`unsupported_content_encoding`; compressed request decoding is not provided.
Handlers install `http.MaxBytesReader` before decoding and close the request
body on every path. A bounded token preflight rejects duplicate keys at the
root and metadata-object levels before typed decoding. Decoding also rejects
unknown fields, trailing JSON values, malformed JSON, non-object metadata, and
an oversized body. The adapter distinguishes body overflow from ordinary
invalid JSON.

Each workflow request derives a two-second timeout context from the incoming
request. Configuration remains injectable so tests use deterministic short
deadlines. The service checks context before each bounded synchronous stage,
during a record scan, and again while holding the store lock immediately before
commit. This is cooperative cancellation between library calls, not a hard
CPU-latency guarantee for a detector or tokenizer call already in progress. A
canceled or expired operation cannot create a record. The adapter returns a
stable timeout response when its own deadline expires and preserves client
cancellation internally without attempting an unreliable response write.

## Domain Model

Each immutable returned `Record` contains:

- canonical content ID, original content, display text, and copied metadata;
- `allowed`, `masked`, or `manual-review` outcome;
- detected language and ISO codes, top confidence, confidence list, detector
  sections, script hints, and ordered review reasons;
- moderation findings with original UTF-8 byte spans and matched terms;
- Japanese Search-mode terms with token text, normalized term, and original
  UTF-8 byte spans when the Japanese route is selected; and
- one UTC creation timestamp supplied by an injected clock.

All maps and slices returned by create, get, and search are deep copies. Store
entries are never exposed for caller mutation.

## Workflow and Policy

`Create` performs these steps in order:

1. Check context and validate/canonicalize the request without rewriting text.
2. Collect script hints and call the shared detector for top language,
   confidences, and multilingual sections.
3. Apply the Issue #119 policy: confident non-mixed English or Korean selects
   moderation; confident non-mixed Japanese with Kana selects Japanese
   preparation plus moderation; Chinese, Han-only ambiguity, unknown, short,
   low-confidence, or mixed input selects manual review.
4. For a supported route, run the moderation matcher. A match creates a masked
   display text and the `masked` outcome; no match preserves content as display
   text and creates `allowed`.
5. Prepare the searchable term set: project Search-mode terms and byte spans
   from the shared Japanese tokenizer for the Japanese route, or normalized
   Unicode-word terms from bluetape-go's simple tokenizer for English/Korean.
   Tokenization failure returns an error and no record.
6. Stamp the complete candidate with the injected clock, check context, acquire
   the write lock, check context again, reject duplicate content ID, enforce
   store capacity, and commit the immutable value atomically. No clock,
   detector, tokenizer, or matcher call occurs while holding the store lock.

Manual-review is a successful stored outcome, not an HTTP or service error. Its
display text is a fixed non-sensitive placeholder rather than the unreviewed
original. It retains evidence for the inspectable by-ID endpoint but is
excluded from search.

The fixed teaching moderation policy compiles the Issue #53 entries `bad`,
`bad wolf`, `scam`, `욕설`, and `무료 돈` with their existing IDs, severities,
and category metadata. It uses `IgnoreCase`, NFC normalization,
`BoundaryUnicodeWord`, minimum middle severity, and the `*` mask, then calls
`BlockwordDictionary.Process` so match selection and multibyte masking remain
library-owned. This integration deliberately omits Issue #53's application-
specific allowlist subtraction instead of copying that example algorithm.

## Search Semantics

The service prepares the query once outside the read lock. Japanese preparation
is selected from query script evidence, not from each record. Queries with Kana
use the Japanese tokenizer; other queries use bluetape-go's simple tokenizer
with the same normalization options used at record creation. Empty prepared
terms are invalid. The example does not implement a second Unicode scanner or
normalization algorithm.

Under a short read lock, the service snapshots pointers to immutable stored
records and releases the lock. It then checks context at least every 32 records,
selects accepted records whose metadata matches and whose prepared-term set
contains every distinct query term, sorts matches by content ID, and copies at
most the effective limit into search projections. The 1,000-record capacity
bounds snapshot allocation, scan work, and sorting. The implementation does not
build a durable or inverted index; bounded linear scanning is explicit and
appropriate to the lifecycle lesson.

Term preparation is one shared helper used by record creation and search. For
English/Korean it calls `textsearch.NewSimpleTokenizer()` with
`TokenizeOptions{Normalize: textsearch.NormalizeNFC}` and omitted whitespace,
keeps `POSWord` and `POSNumber`, applies `strings.ToLower` to each normalized
term, drops empty values, and deduplicates while preserving first occurrence.
For Japanese it uses the Issue #118 projection: Search-mode tokens, noun/verb
selection, NFC-normalized base form when present and token text otherwise, and
the same stable deduplication. Matching treats the resulting slices as sets;
stable order exists for evidence and deterministic JSON only.

## Go API Shape

The internal package uses these constructor-only contracts:

```go
func DefaultConfig() AppConfig
func NewService(config ServiceConfig, options ...ServiceOption) (*Service, error)
func WithClock(clock func() time.Time) ServiceOption

func (s *Service) Create(ctx context.Context, request CreateRequest) (Record, error)
func (s *Service) Get(ctx context.Context, contentID string) (Record, error)
func (s *Service) Search(ctx context.Context, request SearchRequest) (SearchResponse, error)

type Workflow interface {
    Create(context.Context, CreateRequest) (Record, error)
    Get(context.Context, string) (Record, error)
    Search(context.Context, SearchRequest) (SearchResponse, error)
}

func NewEngine(workflow Workflow, config HTTPConfig, logger *log.Logger) (*gin.Engine, error)
func NewHTTPServer(address string, handler http.Handler) *http.Server
func RunServer(ctx context.Context, server HTTPServer, logger *log.Logger) error
```

`CreateRequest` contains content ID, content, and metadata. `SearchRequest`
contains query, metadata, limit, and the optional exclusive content-ID cursor.
`SearchResponse` contains a non-nil hit slice, `Truncated`, and the next cursor.
The handler-facing `Workflow` is the only production interface;
detector/tokenizer/matcher interfaces are not invented. `HTTPServer` is a narrow
lifecycle test seam with `ListenAndServe`, `Shutdown`, and `Close`. `main`
supplies a `signal.NotifyContext` for `SIGINT`/`SIGTERM` to `RunServer`.

The default clock is `time.Now`; a nil injected clock, nil workflow, nil logger,
zero-value service, or invalid configuration fails closed rather than
panicking. Each method accepts the request context from Gin unchanged; the
adapter adds its timeout before calling the workflow. `context.Canceled` and
`context.DeadlineExceeded` remain discoverable through `errors.Is` across every
service boundary.

## Errors

The package defines `ErrInvalidConfig`, `ErrInvalidRequest`,
`ErrDuplicateContentID`, `ErrRecordNotFound`, `ErrStoreCapacity`, and
`ErrWorkflow` sentinels. Input field details use a private typed error that
unwraps to `ErrInvalidRequest`; clients never receive those details. Unexpected
detector/tokenizer failures wrap `ErrWorkflow` and preserve the library cause
through multiple `%w` operands. Callers use `errors.Is`; no exported concrete
error type is required. Context cancellation and deadline errors take
precedence over `ErrWorkflow` whenever observed.

The Gin adapter always returns:

```json
{
  "error": {
    "code": "duplicate_content_id",
    "message": "content_id already exists"
  }
}
```

The stable mapping is:

| Condition | HTTP | Code |
|---|---:|---|
| malformed/unknown/trailing JSON or invalid fields | 400 | `invalid_request` |
| missing or unsupported JSON media type | 415 | `unsupported_media_type` |
| unsupported request content encoding | 415 | `unsupported_content_encoding` |
| request body over 64 KiB | 413 | `request_too_large` |
| handler deadline exceeded | 408 | `request_timeout` |
| duplicate content ID | 409 | `duplicate_content_id` |
| missing record | 404 | `record_not_found` |
| teaching store at capacity | 503 | `store_capacity_reached` |
| unexpected detector/tokenizer/service failure | 500 | `internal_error` |

Responses never expose wrapped causes, input content, findings, or stack
details. Method/path misses use Gin's normal 404/405 behavior and are outside
the domain error envelope.

## Diagnostics

The example writes structured, single-line operational events to the standard
logger on stderr. Events cover startup, shutdown start/completion/failure,
forced close, internal workflow failure, and capacity rejection. Request events
may contain only the route pattern, HTTP method, status, stable error code, and
elapsed duration. Startup events may contain the configured listen address and
timeout values. Logs never contain content IDs, content/display text, metadata
keys or values, findings, tokens, language sections, request bodies, or wrapped
request-processing causes. Tests inject a logger and assert that sentinel
secrets do not appear in internal-error and capacity paths.

## Release and Rollback

The example adds no schema, migration, service dependency, or compatibility
step. Rollback means replacing the example binary/configuration with the prior
revision and restarting it. Every restart, replacement, crash, or rollback
intentionally loses all in-memory moderation records. Both README languages
warn operators not to use this store for durable evidence or audit retention.

## Failure Modes

| Failure | Behavior | Evidence |
|---|---|---|
| Detector, tokenizer, or matcher fails | Return `internal_error`; do not commit; log only the safe category | service and HTTP injected-failure tests |
| Deadline or client cancellation arrives | Cooperatively stop between stages or during scan; recheck under the commit lock; never commit afterward | pre-stage, scan, pre-commit, and handler timeout tests |
| Concurrent callers create one ID | Exactly one atomic create succeeds; every other completed call receives conflict without overwrite | bounded start-gate test under race detector |
| Store reaches 1,000 records | Preserve existing records; duplicate remains conflict; new ID returns capacity error; liveness stays healthy | capacity precedence and health tests |
| Shutdown drain exceeds five seconds | Force close, join the listen path, log the safe event, and exit non-zero without leaving a goroutine | fake-server lifecycle test |
| Malformed, duplicate-key, encoded, or oversized JSON arrives | Reject at the HTTP boundary, close the body, and never invoke the workflow | strict-decoder and tracking-body tests |

## Concurrency and Ownership

One service is safe for concurrent create, get, and search calls. Detector,
tokenizer, and moderation policy are shared only according to their documented
concurrency contracts; request projections are local. The record map is the
only application mutable state and is guarded by `sync.RWMutex`.

Duplicate creation is linearized inside the write lock: concurrent requests
for one ID yield exactly one success and the rest conflicts. No context check,
tokenization, detection, or masking is performed while holding the store lock.
Get and search return deep copies so callers cannot race with internal state.

Focused tests and the race detector will prove concurrent reuse, duplicate
linearization, simultaneous create/search/get, and result-copy isolation. A
bounded start gate coordinates workers without timing sleeps. Lifecycle tests
also prove the listen result is joined after graceful or forced shutdown so the
runner leaks no goroutine.

## Test Strategy

Service tests cover:

- default and custom configuration, including `NaN`, infinities, and search
  maxima below/equal/above the default limit;
- allowed English and Korean records;
- masked content with exact UTF-8 byte spans;
- Japanese Search-mode terms and original byte spans;
- short, mixed, unknown, ambiguous CJK, and unsupported manual review;
- duplicate rejection without overwrite;
- capacity enforcement and duplicate precedence at capacity;
- metadata copying and exact-match filtering;
- ID/path/cursor grammar, metadata-key trimming, and post-trim collision
  rejection;
- create/query term-projection parity, case normalization, deduplication, and
  empty-term rejection;
- all-term English/Korean and Japanese searches, exclusions, stable ordering,
  limits, cursor continuation, truncation, and empty results;
- cancellation before work, during a bounded scan, and immediately before
  commit;
- writer progress during concurrent bounded searches;
- returned map/slice mutation isolation; and
- concurrent reuse under `go test -race`.

Cancellation tests use a deterministic counting context for scan checkpoints
and a test clock that cancels the context before the commit-lock check; they do
not sleep or add mutable production hooks.

HTTP tests cover successful create/get/search, strict JSON decoding, body
closure, the 64 KiB boundary, timeout cancellation through an injected service,
duplicate object keys, JSON media types, UTF-8 charset parameters, content
encoding, every stable error mapping, original-content omission from search,
escaped path rejection, health, and trusted-proxy configuration. Main tests
cover server construction, loopback enforcement and explicit remote opt-in,
safe diagnostics, signal-driven graceful shutdown, shutdown-deadline forced
close, and lifecycle failure exits without binding a public port.

Validation runs focused package tests first, then focused race tests, then
`go test -count=1 ./...` and the repository-authoritative `make ci`.

## Documentation

English and Korean README files show the lesson, run command, request/response
examples, metadata search, outcomes, error codes, lifecycle ownership,
concurrency behavior, and validation commands. They explicitly state the
limits of in-memory storage, linear search, blockword policy, automatic
language detection, unauthenticated original-content lookup, and application-
owned security/privacy controls.

Root English and Korean READMEs add matching navigation entries. No diagram is
needed because the four-stage linear workflow is clearer as text and API
examples.

## Acceptance Boundary

The issue is complete when the example composes bluetape-go v0.18.0 without a
new dependency, all accepted and manual-review paths are inspectable, only
accepted records are searchable through the POST JSON contract, duplicate and
timeout semantics are deterministic, race tests prove shared lifecycle safety,
both README languages are discoverable, and the full repository gate passes.

Durable persistence/indexing, authentication/authorization, compliance policy,
production moderation accuracy, generalized framework abstractions, and
changes to bluetape-go public APIs remain out of scope.
