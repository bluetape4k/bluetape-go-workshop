# Issue #67 Gin Content Moderation Workflow 설계

## 상태

bluetape-go v0.18.0 기반 Issue #67 사용자 승인 설계. Type A specification review는
P0=0/P1=0으로 수렴했다. 사용자가 이 committed artifact를 review하기 전까지
implementation planning은 blocked 상태로 유지한다.

## 목표

Multilingual content를 받고, shared language-detection evidence로 route하며,
supported text를 moderate하고, Japanese search term을 prepare하며, inspectable
in-memory record를 저장하고, JSON request로 accepted record를 search하는
application-shaped Gin 예제를 추가한다.

Lesson은 production-shaped HTTP boundary에서의 composition이다. 즉 하나의
application-owned detector와 tokenizer, framework-independent service, bounded JSON
input, cooperative request deadline, stable error, cancellation-aware commit,
concurrent reuse를 보여준다. 이 예제는 policy와 lifecycle을 설명하지만 production
moderation, privacy, durable-search system을 제공한다고 주장하지 않는다.

## 맥락 및 현재 근거

이 예제는 milestone epic #34 아래의 최종 integration example이다. 완료된 prerequisite은
개별 lesson을 제공한다.

- Issue #53은 blockword moderation과 masking을 도입했다.
- Issue #54는 text-search matching과 byte-span evidence를 도입했다.
- Issue #55는 multilingual intake feasibility를 세웠다.
- Issue #118은 Japanese search preparation을 분리했다.
- Issue #119는 confidence- 및 script-aware language routing을 정의했다.

예제는 bluetape-go v0.18.0을 직접 compose한다. Sibling example `internal` package를
import할 수 없고, 그 package algorithm을 복사하면 안 된다. 관련 library surface는
text-search blockword matcher와 masking helper, Unicode-word preparation을 위한
`textsearch.SimpleTokenizer`, language detector 및 script hint, Japanese Search-mode
tokenizer와 token byte span이다.

## 선택한 접근

Framework-independent `Service` 하나 위에 얇은 Gin adapter를 둔다. Service는 detector
하나, Japanese tokenizer 하나, stateless simple tokenizer 하나, immutable moderation
policy 하나, mutex-protected in-memory record map 하나를 소유한다. Request-specific
evidence와 result slice는 call-local로 유지한다. HTTP handler는 JSON limit, strict
decoding, timeout, body closure, error-to-status mapping을 소유한다.

이 방식은 workshop code를 generic framework로 만들지 않고 예제를 application-shaped로
유지한다. Gin boundary의 좁은 service interface는 sleep 또는 mutable production hook
없이 deterministic cancellation 및 error-mapping test를 가능하게 한다.

## 기각한 대안

### Fully pluggable stage pipeline

Detector, router, moderator, tokenizer, repository용 별도 interface는 substitution을
쉽게 만들지만 composition lesson을 framework-like abstraction 뒤에 숨기므로 기각한다.

### 하나의 monolithic Gin handler

Transport와 policy를 결합하고 commit 전 cancellation 증명을 어렵게 하며 direct
service testing을 막으므로 기각한다.

### Sibling workshop example import

해당 `internal` package는 의도적으로 접근할 수 없고 reusable library가 아니므로
기각한다. Integration은 bluetape-go public API를 사용하고 application policy와
projection만 local로 유지해야 한다.

## Package 및 파일

구현은 다음 위치에 둔다.

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

Root `README.md`와 `README.ko.md`는 예제를 link한다. Dependency, module, workflow,
container, database, coverage-policy 변경은 필요하지 않다.

## Configuration 및 Lifecycle

Application configuration은 다음 explicit default를 소유한다.

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

기본값은 minimum confidence `0.70`, minimum routing length `8` rune, maximum content
length `8,000` rune, maximum JSON body size `64 KiB`, request timeout 2초다.
Teaching store는 최대 1,000 record를 보관한다. Search는 기본적으로 20 result를
반환하며 configured maximum까지만 요청할 수 있다. 이 maximum의 기본값은 100이고
20보다 작을 수 없다. 유효하지 않은 non-positive limit 또는 timeout은 construction을
실패시킨다. Minimum confidence는 finite 값이며 `[0,1]` 안에 있어야 한다. Constructor는
`NaN`과 양/음 infinity를 명시적으로 거부한다.

`NewService`는 English, Korean, Japanese, Chinese용 language detector 하나, Search
mode로 설정된 Japanese tokenizer 하나, stateless simple tokenizer 하나, moderation
matcher 하나를 정확히 construct한다. 이들은 application lifetime 동안 유지되며
request마다 생성하지 않는다. `main`은 service 하나와 Gin engine 하나를 construct한다.

HTTP process는 기본적으로 `127.0.0.1:8080`에 bind한다. `HTTP_ADDR`는 이를 명시적으로
override할 수 있다. `ALLOW_UNAUTHENTICATED_REMOTE=1`도 설정하지 않으면
non-loopback address를 거부해, 이 예제의 missing auth boundary를 명시적인 opt-in으로
만든다. `http.Server`는 2초 read-header timeout, 5초 read timeout, 5초 write timeout,
30초 idle timeout을 사용한다. Gin trusted proxy는 `nil`로 설정한다.

`main`은 construction failure와 non-`ErrServerClosed` listen failure를 fatal로
취급한다. `SIGINT`와 `SIGTERM`을 처리하고 `Server.Shutdown`에 drain 시간 5초를 준
뒤, deadline이 만료되면 `Server.Close`를 호출한다. Shutdown 또는 forced-close failure는
안전하게 log하고 non-zero exit을 만든다. Lifecycle logic은 좁은 local server
interface 뒤로 factoring하여 테스트가 public port bind 없이 signal과 deadline을
구동할 수 있게 한다. Service는 background goroutine이나 external resource를 소유하지
않으므로 artificial service `Close` method를 추가하지 않는다.

## HTTP 계약

### Moderation record 생성

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

`content_id`는 trim한 뒤 `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`와 match해야 한다.
Lookup과 search cursor에도 같은 canonicalization 및 validation을 적용하므로, 모든
accepted ID는 Gin path segment로 안전하게 round-trip된다. Encoded separator,
control, 그 밖의 escaped path syntax는 reject한다. Reported span이 제출된 UTF-8
byte를 slice해야 하므로 `content`는 byte for byte로 보존한다. Blank 또는 over-limit
content는 reject한다. Metadata는 optional string-to-string 값이며 input에서 copy하고
record와 함께 저장한다. Key는 trim하고 nonblank여야 한다. Trim 이후 충돌하는 두
source key는 invalid다. Value는 caller-owned exact-match dimension이므로 trim하지
않고 보존한다. Metadata는 API response에서 관찰 가능하므로 credential 또는 secret을
포함하면 안 된다.

Creation은 `201 Created`와 complete stored record를 반환한다. 기존 canonical
`content_id`를 재사용하면 `409 Conflict`를 반환하며, payload가 동일해도 record를
overwrite하지 않는다. Teaching store가 configured capacity에 도달한 뒤 새 ID를
사용하면 `503 Service Unavailable`을 반환한다. Duplicate checking이 우선하므로 known
ID는 capacity 상태에서도 conflict로 남는다.

### Record 하나 조회

```http
GET /moderation/records/:content_id
```

응답은 original content, display text, metadata, routing 및 language evidence,
finding, Japanese term, outcome, timestamp를 포함한다. 이는 의도적으로 inspectable한
teaching endpoint다. README는 real application이 original content를 노출하기 전에
authentication, authorization, retention, audit logging, log redaction을 소유해야
한다고 설명한다.

### Accepted record 검색

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

JSON body는 URI-length constraint를 피하고 extensible filter를 지원한다. `query`는
valid UTF-8, nonblank, 8,000 rune 이하이어야 한다. Metadata는 creation과 같은 key
validation을 사용한다. 제공된 모든 metadata key/value는 stored record와 정확히 match해야
한다. 누락되었거나 0인 `limit`은 20을 사용한다. `ServiceConfig.MaximumSearchResults`
보다 크거나 0보다 작은 값은 invalid다. `after_content_id`는 optional 및 exclusive다.
존재하면 그보다 lexically greater인 canonical ID만 eligible하다. Search는 `allowed` 및
`masked` record만 포함한다. `manual-review` record는 절대 searchable하지 않다.

Japanese query는 shared Search-mode tokenizer의 term을 사용한다. English와 Korean
query는 normalized Unicode-word term을 사용한다. 모든 distinct query term이 필요하다.
Search response는 `content_id`, outcome, `display_text`, metadata, detected
language를 포함하지만 original content, prepared term, raw finding은 생략한다.
Result는 canonical `content_id` 기준으로 sort하고 effective limit으로 truncate한다.
다른 match가 있으면 `truncated`와 함께 `next_after_content_id`를 final returned ID로
설정한다. 그 값을 다음 exclusive cursor로 넘기면 duplicate 없이 기존 ordered result
set을 계속 조회한다. In-memory endpoint에는 snapshot isolation이 없다. Concurrent
insert는 이후 page에 나타나거나, ID가 cursor 앞에 sort되면 skip될 수 있다. Empty
result는 successful non-nil JSON array다.

### Health

`GET /healthz`는 liveness-only다. 작고 안정적인 `200 OK` response를 반환하며 detector,
tokenizer, moderation pipeline을 실행하지 않는다. Store capacity는 liveness를 바꾸지
않는다. 예제에는 external dependency가 없으므로 readiness endpoint를 제공하지 않는다.

## Strict JSON 및 Request Bounds

두 POST endpoint는 optional case-insensitive `charset=utf-8` parameter를 포함할 수 있는
`Content-Type: application/json`을 요구한다. 누락, malformed, 다른 media type은 body를
읽기 전에 `415 Unsupported Media Type`을 반환한다. `identity`가 아닌 nonempty
`Content-Encoding`은 `unsupported_content_encoding`으로 reject한다. Compressed request
decoding은 제공하지 않는다. Handler는 decode 전에 `http.MaxBytesReader`를 설치하고 모든
path에서 request body를 닫는다. Bounded token preflight는 typed decoding 전에 root 및
metadata-object level duplicate key를 reject한다. Decoding은 unknown field, trailing
JSON value, malformed JSON, non-object metadata, oversized body도 reject한다. Adapter는
body overflow를 일반 invalid JSON과 구분한다.

각 workflow request는 incoming request에서 2초 timeout context를 derive한다.
Configuration은 injectable로 유지해 테스트가 deterministic short deadline을 사용하게
한다. Service는 각 bounded synchronous stage 전, record scan 중, commit 직전 store lock을
잡은 상태에서 다시 context를 확인한다. 이는 이미 진행 중인 detector 또는 tokenizer
call에 대한 hard CPU-latency guarantee가 아니라 library call 사이의 cooperative
cancellation이다. Canceled 또는 expired operation은 record를 만들 수 없다. Adapter는
자체 deadline이 만료되면 안정적인 timeout response를 반환하고, client cancellation은
신뢰할 수 없는 response write를 시도하지 않은 채 내부적으로 보존한다.

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
6. Build the unstamped candidate, check context, acquire the write lock, check
   context again, reject duplicate content ID, enforce store capacity, stamp it
   with the non-blocking injected clock, check context once more, and commit the
   immutable value atomically. No detector, tokenizer, or matcher call occurs
   while holding the store lock. The clock is serialized by that lock and is
   never called for duplicate or over-capacity requests.

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
for one ID yield exactly one success and the rest conflicts. Only bounded
constant-time commit operations run under that lock: context checks,
duplicate/capacity checks, the documented non-blocking serialized clock,
stamping, and insertion. Detection, tokenization, masking, and other expensive
work remain outside. Get and search return deep copies so callers cannot race
with internal state.

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

Cancellation tests use pre-canceled contexts for every workflow method, a
deterministic counting context for scan checkpoints, and a test clock that
cancels the context before the final under-lock commit check. They do not sleep
or add mutable production hooks.

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
