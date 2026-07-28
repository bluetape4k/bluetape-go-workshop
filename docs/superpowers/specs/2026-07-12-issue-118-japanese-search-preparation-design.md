# Issue #118 Japanese Search Preparation 설계

## 상태

bluetape-go v0.18.0 기반 Issue #118 승인 설계.

## 목표

작은 Japanese product catalog를 deterministic search용으로 준비하는
application-shaped command를 추가한다. Command는 Kagome search-mode token, 유용한
part-of-speech metadata, 원본 UTF-8 byte span, normalized index term, 실제
`textsearch` matching, support-text masking pass를 노출해야 한다.

이 예제는 application이 released package를 조합하는 방법을 설명한다. Reusable
workshop infrastructure를 추가하거나 package algorithm을 복사하지 않는다.

## 맥락

Issue #55는 multilingual feasibility baseline을 세우고 shared detector/tokenizer
reuse를 증명했다. Issue #118은 lesson을 Japanese product search preparation으로
좁히며, #119의 language-routing 예제와 #67의 Gin integration 예제보다 앞서야
한다.

관련 v0.18.0 surface:

- `japanese.NewTokenizer(japanese.WithMode(japanese.Search))` for reusable
  Kagome IPA search-mode tokenization;
- `japanese.IsNoun`, `japanese.IsVerb`, and Kagome metadata keys for selecting
  index candidates;
- `textsearch.Token` for original byte spans and normalized token text;
- `textsearch.Matcher` for deterministic matching over prepared index terms;
- `textsearch.BlockwordDictionary` for deterministic support-text masking.

## 선택한 접근

세 개의 명시적 stage를 가진 local package 하나를 사용한다.

1. shared Japanese tokenizer로 product와 query text를 prepare한다.
2. query-local matcher로 immutable in-memory prepared catalog를 search한다.
3. shared blockword dictionary로 설정된 support-text term을 mask한다.

이 방식은 morphological analysis, search policy, masking policy 사이의 boundary를
보존하면서 command를 응집도 있게 유지한다. 단일 monolithic method는 span ownership을
흐린다. 별도 command는 fixture와 lifecycle setup을 중복하고 요청된 integration을
보여주지 못한다.

## 기각한 대안

### 단일 monolithic service method

Token span, prepared index position, masking span은 owner가 다르므로 기각한다. 이를
하나의 output transformation으로 결합하면 caller가 잘못된 text를 slice하기 쉬우며,
테스트가 실패한 contract를 격리하기 어렵다.

### Search와 masking command 분리

두 command가 tokenizer와 fixture lifecycle setup을 반복하므로 기각한다. 더 중요한
점은 masking evidence가 다른 방식으로는 valid한 search projection에서 token을
제외하는 방법을 예제가 증명하지 못한다는 것이다.

### Generic workshop index abstraction

Issue #118에는 inspectable local catalog 하나가 필요하므로 기각한다. Reusable index
interface는 library 또는 여러 real call site를 가진 이후 application에 속하는
ranking, persistence, update, concurrency contract를 발명하게 된다.

## Package 및 파일

구현은 다음 위치에 둔다.

```text
examples/japanese-search-preparation/
  main.go
  README.md
  README.ko.md
  internal/catalogprep/
    service.go
    service_test.go
```

Root `README.md`와 `README.ko.md`는 예제를 link한다. Umbrella Issue #34는 completed
child Issue #55를 표시하고, Issue #118은 delivery 전까지 open 상태로 유지한다.

## Domain 계약

### 입력

`ProductInput`은 caller-owned SKU, Japanese title, Japanese support text를 포함한다.
세 값은 모두 필수다. `SearchRequest`는 비어 있지 않은 Japanese query를 포함한다.

예제는 `textsearch.NewTokenizeRequest`가 지원하는 input size만 허용한다. 두 번째
size limit을 추가하거나 input을 조용히 truncate하지 않는다.

### Go API Shape

Internal example package는 constructor-only `Service`를 노출한다.

```go
func NewService(products []ProductInput, policy MaskPolicy) (*Service, error)
func (s *Service) Products() []PreparedProduct
func (s *Service) Search(request SearchRequest) (SearchResult, error)
func NewPreview() (Preview, error)
```

`DefaultProducts`와 `DefaultMaskPolicy`는 command fixture를 제공한다. `Service`의
zero value는 의도적으로 사용할 수 없다. 모든 method는 receiver를 확인하고 panic
대신 `ErrInvalidService`를 반환한다. `Products`는 deep copy를 반환하므로 caller가
concurrent search에 사용되는 catalog나 token metadata를 변경할 수 없다.

### Prepared Token

각 selected token은 다음을 기록한다.

- source field(`title` 또는 `support_text`)
- original surface text
- normalized text
- Kagome가 제공할 때 base form
- 전체 Kagome POS metadata
- field-local, start-inclusive/end-exclusive UTF-8 byte offset
- token이 search index에 적합한지 여부

Noun과 verb만 선택한다. Index term은 존재하면 NFC-normalized base form이고, 없으면
normalized surface text다. Duplicate term은 첫 등장 순서를 보존하면서 제거한다.
Title term은 support text term보다 앞선다.

Byte span은 항상 원본 field를 slice한다. Masked projection이나 prepared index
string 안의 position이 아니다.

### Masking

Fixed local blockword policy는 support text 안의 설정된 unsafe term을 감지한다.
Service는 원본 support text, masked projection, match span을 반환한다. Masked
match와 겹치는 selected support token은 evidence로 보이지만 non-indexable로 표시되어
index term에서 제외된다.

Japanese text는 whitespace word boundary를 안정적으로 노출하지 않으므로 masking
policy는 explicit substring semantics를 사용한다. README는 이를 semantic
moderation이 아니라 deterministic local policy로 식별해야 한다.

### Prepared Index 및 Search

각 product는 unique indexable term을 space로 join한 inspectable index string을
소유한다. Search는 같은 tokenizer와 term projection으로 query를 prepare한 뒤,
space-delimited index 위에서 NFC normalization 및 Unicode word boundary를 사용하는
query-local `textsearch.Matcher`를 compile한다.

Product는 모든 unique query term이 있을 때만 hit이다. Matched term은 query order로
반환한다. Hit는 output이 deterministic하도록 SKU 기준으로 sort한다. 예제는 relevance
ranking을 주장하지 않는다.

비어 있지 않은 query가 noun 또는 verb term을 만들지 않으면, search는 모든 product를
match하는 대신 명시적인 invalid-query error를 반환한다.

## Service Lifecycle

`NewService`는 다음을 구성한다.

- `japanese.Search` mode의 Kagome IPA tokenizer 하나
- immutable blockword dictionary 하나
- immutable prepared catalog 하나

Service는 이 값들을 call 사이에서 reuse한다. Query matcher와 모든 mutable result
slice는 한 call에 local하다. Service는 goroutine, channel, I/O, timer, shutdown
책임이 없다. 따라서 `context.Context`를 받지 않는다. Context를 추가하면 이 CPU-only
bounded fixture가 의미 있게 지킬 수 없는 cancellation behavior를 암시하게 된다.

Kagome의 IPA dictionary는 bluetape-go가 이미 제공하는 opt-in binary 및 memory cost다.
Command는 tokenizer를 한 번 construct하고 reuse를 문서화한다. Universal startup 또는
memory measurement를 publish하지 않는다.

## Error

Local package는 invalid product, invalid query input, invalid 또는 nil service use를
위한 sentinel error를 정의한다. Constructor와 method는 cause를 `%w`로 wrap하므로
caller는 local sentinel과 upstream tokenization error 모두에 `errors.Is`를 사용할 수
있다.

다음 경우는 fail closed한다.

- blank SKU, title, support text
- blank query
- upstream tokenization limit을 초과하는 input
- noun 또는 verb term이 없는 prepared query
- tokenizer, matcher, dictionary construction failure

Command는 이 failure를 startup/preview error로 취급하고 non-zero로 종료한다. Fixture
하나가 invalid하면 partial catalog를 publish하지 않는다.

## Failure Mode 및 Guard

1. **Normalized term이 원래 위치를 잃는다.** Prepared token은 field-local source
   span을 유지하고 테스트는 모든 token에 대해 원본 field를 slice한다. Index-string
   position은 source span으로 절대 노출하지 않는다.
2. **Masked unsafe term이 search로 leak된다.** Mask match는 index projection 전에
   원본 support text에 대해 계산하며, 겹치는 모든 token은 evidence로 보존하지만
   index에서는 제외한다.
3. **Concurrent reuse가 shared result를 변경한다.** Catalog, tokenizer,
   dictionary는 한 번 construct하고, query matcher와 result slice는 call-local이다.
   Catalog access는 deep copy를 반환한다. Exact bounded stress 및 race test가 이
   contract를 guard한다.
4. **Japanese substring behavior를 semantic moderation으로 오해한다.** Output과
   README는 boundary policy를 명시적으로 이름 붙이고 security, compliance,
   semantic-safety claim을 하지 않는다.
5. **빈 prepared query가 우연히 전체 catalog를 match한다.** Search는 matcher 생성
   전에 selected noun 또는 verb를 만들지 않는 query를 reject한다.

## Command Output

`go run ./examples/japanese-search-preparation`는 다음을 포함하는 deterministic
indented JSON을 출력한다.

- scenario 및 tokenizer/masking lifecycle note
- selected token, span, POS, masked support text, index term을 가진 prepared
  product
- expected matching 및 no-match outcome을 가진 fixed query 최소 두 개
- 명시적인 boundary note와 validation command

Fixture는 사람이 직접 inspect할 수 있을 만큼 작게 유지한다. 두 README locale의
expected output snippet은 command가 생성한 값을 사용해야 한다.

## 테스트

테스트는 구현 전에 작성하며 다음을 다룬다.

1. title과 support text의 normal search-mode tokenization.
2. noun/verb filtering 및 유용한 POS/base-form metadata.
3. 원본 multibyte byte span을 보존하는 NFC normalization.
4. `errors.Is` 보존을 포함한 blank 및 oversized product/query failure.
5. support-text detection, masking, match span, 겹치는 term의 index exclusion.
6. deterministic all-term matching, query-order matched term, stable SKU ordering,
   no-match result.
7. noun/verb term을 만들지 않는 query text.
8. normal 및 race execution 아래 exact completion/result를 가진 bounded
   shared-service reuse.
9. command와 문서가 사용하는 결정적 preview content.

Bounded stress test는 fixed worker/task set에서 하나의 service를 공유한다. Exact
completed-call count, exact hit SKU, byte-span slicing, stable masking output을
assert한다. Focused `go test`와 `go test -race`가 모두 필요하다.

저장소 validation은 다음과 같다.

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

이 issue는 container-backed test를 추가하지 않는다. 기존 repository-wide
Testcontainers test는 normal `make ci` gate를 통해 계속 순차 실행된다.

## Compatibility 및 Migration

이는 새 standalone example이다. 기존 workshop API, wire format, persisted value를
바꾸지 않으므로 caller migration은 필요하지 않다. Root README에는 navigation만
추가된다. 예제는 이미 pin된 bluetape-go v0.18.0 의존성을 사용하며 module
requirement를 추가하지 않는다.

Future issue는 여기서 보여준 policy를 빌릴 수 있지만, 이 예제의 `internal` package
또는 JSON preview를 compatibility surface로 취급하지 말고 bluetape-go package를
직접 import해야 한다.

## 문서

예제 README pair는 다음을 문서화한다.

- package lesson과 three-stage data flow
- run 및 focused test command
- representative token, POS metadata, field-local byte span, index term, search
  hit, masked support text
- normal versus search-mode intent
- Kagome IPA dictionary footprint 및 construct-once/reuse lifecycle
- NFC normalization versus original span semantics
- Japanese substring masking boundary
- unsupported non-Japanese tokenization, ranking, persistence, HTTP path

Root README pair는 navigation row 하나와 compact run section 하나를 추가한다.

## 비목표

- HTTP endpoint, database, durable index, ranking engine, stemming API, generic
  search abstraction 없음.
- Language detection 또는 routing 없음. Issue #119가 해당 lesson을 소유한다.
- 두 번째 tokenizer, dictionary, detector dependency 없음.
- Rune-offset conversion 없음. Span은 UTF-8 byte offset으로 유지한다.
- Noun/verb selection, substring masking, language-specific tokenization이
  security, compliance, semantic moderation decision이라는 claim 없음.
- Workshop 내부에서 `textsearch` matching, normalization, masking, Kagome
  tokenization algorithm 중복 없음.

## Acceptance Mapping

| Issue requirement | Design evidence |
|---|---|
| Japanese tokenizer and application-owned lifecycle | `NewService`가 `japanese.Search` tokenizer 하나를 construct하고 reuse한다. |
| Normalization, POS filtering, and multibyte spans | Prepared-token contract와 field-local slicing test. |
| Search/masking integration | Space-delimited prepared index와 query matcher, overlap exclusion을 가진 support dictionary. |
| Empty/invalid input | `%w`를 통해 upstream error를 보존하는 local sentinel. |
| Concurrent reuse and race proof | Focused normal/race test 아래 exact bounded stress assertion. |
| Lifecycle and unsupported boundaries | Paired README와 deterministic preview note. |
| Root navigation and repository quality | Root README pair와 complete `make ci` gate. |

## Definition of Done

- Fixed command output은 preparation, matching, masking과 각각의 distinct span
  contract를 보여준다.
- 모든 Issue #118 acceptance criterion은 deterministic normal, failure, edge,
  race test 중 하나로 mapping된다.
- English/Korean README content는 synchronized 상태를 유지하고 root navigation은
  두 locale file로 resolve된다.
- Clean branch에서 focused normal/race test와 repository `make ci`가 통과한다.
- Final review는 unsupported behavior를 문서화한 상태로 P0=0 및 P1=0을 보고한다.
- 사용자가 merge와 local synchronization을 명시적으로 승인하지 않는 한 delivery는
  review-ready PR에서 멈춘다.
