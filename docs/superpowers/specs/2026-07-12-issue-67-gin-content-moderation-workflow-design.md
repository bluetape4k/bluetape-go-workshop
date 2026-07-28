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

반환되는 각 불변 `Record`는 다음 값을 포함한다.

- 정규화된 content ID, 원문 content, 표시용 text, 복사된 metadata
- `allowed`, `masked`, `manual-review` 중 하나의 outcome
- 감지된 언어와 ISO 코드, 최상위 confidence, confidence 목록, detector
  section, script hint, 정렬된 review reason
- 원본 UTF-8 byte span과 일치한 term을 포함한 moderation finding
- 일본어 경로가 선택된 경우 token text, 정규화된 term, 원본 UTF-8 byte span을
  담은 Japanese Search-mode term
- 주입된 clock이 제공한 UTC 생성 timestamp

create, get, search가 반환하는 모든 map과 slice는 깊은 복사본이다. 저장소
entry는 호출자가 변경할 수 있도록 노출되지 않는다.

## Workflow and Policy

`Create`는 다음 단계를 순서대로 수행한다.

1. context를 확인하고 text를 다시 쓰지 않은 채 request를 검증하고 정규화한다.
2. script hint를 수집한 뒤 공유 detector를 호출해 최상위 언어, confidence,
   다국어 section을 얻는다.
3. Issue #119 정책을 적용한다. 확신도 높은 비혼합 English 또는 Korean 입력은
   moderation을 선택한다. Kana가 포함된 확신도 높은 비혼합 Japanese 입력은
   Japanese preparation과 moderation을 선택한다. Chinese, Han-only 모호성,
   unknown, 짧은 입력, 낮은 confidence, 혼합 입력은 manual review를 선택한다.
4. 지원되는 경로에서는 moderation matcher를 실행한다. 일치 항목이 있으면 마스킹된
   표시 text와 `masked` outcome을 만든다. 일치 항목이 없으면 content를 표시
   text로 유지하고 `allowed`를 만든다.
5. 검색 가능한 term 집합을 준비한다. Japanese 경로에서는 공유 Japanese tokenizer가
   만든 project Search-mode term과 byte span을 사용하고, English/Korean에서는
   bluetape-go simple tokenizer가 만든 정규화된 Unicode-word term을 사용한다.
   tokenization 실패는 오류를 반환하며 record를 만들지 않는다.
6. timestamp가 없는 후보 값을 만들고, context를 확인한 뒤 write lock을 획득한다.
   lock 안에서 context를 다시 확인하고, 중복 content ID를 거부하며, 저장소 capacity를
   강제한다. 이후 non-blocking 주입 clock으로 timestamp를 찍고, context를 한 번 더
   확인한 뒤 불변 값을 원자적으로 commit한다. 저장소 lock을 잡고 있는 동안 detector,
   tokenizer, matcher 호출은 발생하지 않는다. clock 호출은 해당 lock으로 직렬화되며,
   중복 또는 capacity 초과 request에는 호출되지 않는다.

Manual-review는 HTTP 또는 service 오류가 아니라 성공적으로 저장된 outcome이다.
표시 text는 검토되지 않은 원문 대신 고정된 비민감 placeholder를 사용한다. by-ID
endpoint에서 검사 가능한 evidence는 유지하지만 search에서는 제외한다.

고정된 교육용 moderation 정책은 Issue #53 entry인 `bad`, `bad wolf`, `scam`,
`욕설`, `무료 돈`을 기존 ID, severity, category metadata와 함께 compile한다.
이 정책은 `IgnoreCase`, NFC normalization, `BoundaryUnicodeWord`, 최소 middle
severity, `*` mask를 사용한 뒤 `BlockwordDictionary.Process`를 호출한다. 따라서
match 선택과 multibyte masking은 계속 library가 소유한다. 이 통합은 예제 algorithm을
복사하지 않기 위해 Issue #53의 application-specific allowlist subtraction을 의도적으로
포함하지 않는다.

## Search Semantics

service는 read lock 밖에서 query를 한 번 준비한다. Japanese preparation은 각 record가
아니라 query의 script evidence로 선택한다. Kana가 있는 query는 Japanese tokenizer를
사용하고, 그 밖의 query는 record 생성 때와 같은 normalization option으로
bluetape-go simple tokenizer를 사용한다. 준비된 term이 비어 있으면 유효하지 않다.
이 예제는 두 번째 Unicode scanner나 normalization algorithm을 구현하지 않는다.

service는 짧은 read lock 안에서 불변 저장 record pointer의 snapshot을 만든 뒤 lock을
해제한다. 그런 다음 적어도 32개 record마다 context를 확인하고, metadata가 일치하며
prepared-term set이 모든 고유 query term을 포함하는 accepted record를 선택한다. match는
content ID로 정렬하고, effective limit까지만 search projection으로 복사한다. 1,000개
record capacity는 snapshot allocation, scan 작업, sorting 비용을 제한한다. 구현은
영속 index나 inverted index를 만들지 않는다. 제한된 선형 scan은 명시적이며 lifecycle
lesson에 적합하다.

term preparation은 record 생성과 search가 함께 사용하는 하나의 helper다. English/Korean의
경우 `TokenizeOptions{Normalize: textsearch.NormalizeNFC}`와 생략된 whitespace option으로
`textsearch.NewSimpleTokenizer()`를 호출한다. `POSWord`와 `POSNumber`만 유지하고, 각
정규화 term에 `strings.ToLower`를 적용하며, 빈 값을 버리고 첫 출현 순서를 보존해
deduplication한다. Japanese의 경우 Issue #118 projection을 사용한다. 즉 Search-mode
token, noun/verb selection, 값이 있을 때의 NFC-normalized base form과 그 밖의 token
text, 동일한 stable deduplication을 사용한다. matching은 결과 slice를 set처럼 다룬다.
stable order는 evidence와 deterministic JSON을 위한 것이다.

## Go API Shape

internal package는 다음 constructor-only contract를 사용한다.

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

`CreateRequest`는 content ID, content, metadata를 포함한다. `SearchRequest`는
query, metadata, limit, 선택적인 exclusive content-ID cursor를 포함한다.
`SearchResponse`는 nil이 아닌 hit slice, `Truncated`, next cursor를 포함한다.
handler가 보는 `Workflow`가 유일한 production interface다. detector/tokenizer/matcher
interface는 새로 만들지 않는다. `HTTPServer`는 `ListenAndServe`, `Shutdown`, `Close`를
갖는 좁은 lifecycle test seam이다. `main`은 `SIGINT`/`SIGTERM`용
`signal.NotifyContext`를 `RunServer`에 제공한다.

기본 clock은 `time.Now`다. nil 주입 clock, nil workflow, nil logger, zero-value
service, 유효하지 않은 configuration은 panic이 아니라 fail closed로 처리한다. 각 method는
Gin에서 받은 request context를 그대로 받아들이며, adapter가 workflow 호출 전에 timeout을
추가한다. `context.Canceled`와 `context.DeadlineExceeded`는 모든 service boundary에서
`errors.Is`로 계속 식별 가능해야 한다.

## Errors

package는 `ErrInvalidConfig`, `ErrInvalidRequest`, `ErrDuplicateContentID`,
`ErrRecordNotFound`, `ErrStoreCapacity`, `ErrWorkflow` sentinel을 정의한다.
입력 field detail은 `ErrInvalidRequest`로 unwrap되는 private typed error를 사용한다.
client는 이 detail을 받지 않는다. 예기치 않은 detector/tokenizer 실패는 `ErrWorkflow`를
wrap하고 여러 `%w` operand를 통해 library cause를 보존한다. caller는 `errors.Is`를
사용하며, export된 concrete error type은 필요하지 않다. context cancellation과 deadline
오류가 관찰되면 항상 `ErrWorkflow`보다 우선한다.

Gin adapter는 항상 다음 형태를 반환한다.

```json
{
  "error": {
    "code": "duplicate_content_id",
    "message": "content_id already exists"
  }
}
```

안정적인 mapping은 다음과 같다.

| Condition | HTTP | Code |
|---|---:|---|
| malformed/unknown/trailing JSON 또는 유효하지 않은 field | 400 | `invalid_request` |
| 누락되었거나 지원하지 않는 JSON media type | 415 | `unsupported_media_type` |
| 지원하지 않는 request content encoding | 415 | `unsupported_content_encoding` |
| request body가 64 KiB 초과 | 413 | `request_too_large` |
| handler deadline 초과 | 408 | `request_timeout` |
| 중복 content ID | 409 | `duplicate_content_id` |
| 누락된 record | 404 | `record_not_found` |
| 교육용 store capacity 도달 | 503 | `store_capacity_reached` |
| 예기치 않은 detector/tokenizer/service 실패 | 500 | `internal_error` |

response는 wrapped cause, 입력 content, finding, stack detail을 절대 노출하지 않는다.
method/path miss는 Gin의 일반 404/405 동작을 사용하며 domain error envelope 밖에 있다.

## Diagnostics

예제는 구조화된 single-line operational event를 stderr의 표준 logger에 기록한다. event는
startup, shutdown 시작/완료/실패, forced close, internal workflow failure, capacity
rejection을 포함한다. request event에는 route pattern, HTTP method, status, 안정적인
error code, elapsed duration만 포함할 수 있다. startup event에는 설정된 listen address와
timeout 값을 포함할 수 있다. log에는 content ID, content/display text, metadata key 또는
value, finding, token, language section, request body, wrapped request-processing cause가
절대 포함되지 않는다. test는 logger를 주입하고 internal-error 및 capacity path에서 sentinel
secret이 나타나지 않는지 assert한다.

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
