# Issue #119 Multilingual Language Routing 설계

## 상태

bluetape-go v0.18.0 기반 Issue #119 승인 설계. Type A spec review가 P0=0/P1=0으로
수렴하고 사용자가 이 committed artifact를 review하기 전까지 implementation
planning은 blocked 상태로 유지한다.

## 목표

Language detector evidence를 작고 deterministic한 content-routing decision으로
바꾸는 application-shaped command를 추가한다. 예제는 English와 Korean이 moderation
route를 공유하게 하고, Kana가 있는 confident Japanese text를 Japanese-tokenization
route로 보내며, unsupported 또는 uncertain input을 internal error가 아니라
manual-review decision으로 노출해야 한다.

Lesson은 routing policy와 detector lifecycle이다. Moderator나 tokenizer를 실행하지
않으며 language detection에서 security, compliance, certainty claim을 만들지 않는다.

## 맥락 및 현재 근거

Issue #55는 English, Korean, Japanese detection과 Japanese tokenization의
feasibility를 세웠다. 이후 Issue #118은 Japanese search preparation을 분리했다.
Issue #119는 token projection이나 search behavior를 반복하면 안 된다. 대신 이후
Issue #67의 Gin integration이 빌릴 수 있는 decision layer를 소유한다.

관련 bluetape-go v0.18.0 surface:

- `language.NewDetector` with an explicit language subset;
- `language.WithPreloadedLanguageModels` for an opt-in startup lifecycle;
- `Detector.Detect`, `Detector.Confidences`, and `Detector.DetectMultiple`;
- `Detector.Languages` for inspecting the configured subset;
- `language.ContainsLatin`, `ContainsKorean`, `ContainsJapanese`, and
  `ContainsChinese` for Latin, Hangul, Kana, and Han script hints;
- upstream validation boundary를 위한 `language.MaxTextLength`, `ErrBlankText`,
  `ErrTextTooLong`.

`Confidences`는 이미 descending value와 ISO code를 반환한다. Mixed-language section은
start-inclusive/end-exclusive UTF-8 byte offset과 text slice를 이미 보존한다.
Workshop은 detector logic을 재구현하지 않고 이 contract를 노출한다.

## 선택한 접근

Reusable detector 하나를 소유하는 framework-independent `Router` 하나를 사용한다.
Constructor는 English, Korean, Japanese, Chinese를 선택하고 lazy 또는 preloaded
model initialization 중 하나를 적용한다. 각 request는 하나의 explicit routing
matrix를 적용하기 전에 detection, confidence, section, script evidence를 call-local
값으로 모은다.

이 방식은 reusable workshop framework를 만들지 않고 lifecycle과 policy를 함께
유지한다. CLI는 `--preload`를 제공하므로 같은 fixture가 두 construction mode를 모두
보여주면서 동일한 routing decision을 생성할 수 있다.

## 기각한 대안

### Top detected language에서 직접 route

Han-only text는 Japanese tokenizer를 안전하게 선택할 충분한 script evidence 없이도
Japanese 또는 Chinese로 분류될 수 있으므로 기각한다. 또한 low confidence와 mixed
section을 caller에게 숨긴다.

### Router 내부에서 moderation 및 Japanese tokenization 실행

Issue #119는 processor behavior가 아니라 selection을 설명하므로 기각한다. 이는
Issue #55/#118을 중복하고 관련 없는 lifecycle cost를 결합하며, future Gin example이
workshop-internal pipeline에 의존하게 만든다.

### Lazy 및 preloaded model용 router 분리

Model loading은 routing-policy variant가 아니라 detector construction 선택이므로
기각한다. 두 router implementation은 route behavior drift를 허용하고 lifecycle
comparison을 흐린다.

### 다른 detector 또는 script library 추가

bluetape-go v0.18.0이 필요한 confidence, section, ISO-code, script-hint API를 이미
노출하므로 기각한다. 다른 의존성은 lesson을 개선하지 않고 competing semantics를
추가한다.

## Package 및 파일

구현은 다음 위치에 둔다.

```text
examples/multilingual-language-routing/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/routing/
    router.go
    router_test.go
```

Root `README.md`와 `README.ko.md`는 새 예제를 link한다. Dependency, workflow,
module, container, database, HTTP registration 변경은 필요하지 않다.

## Configuration 및 Lifecycle

`Config`는 application policy를 소유한다.

```go
type Config struct {
    MinimumConfidence float64 `json:"minimum_confidence"`
    MinimumRunes      int     `json:"minimum_runes"`
    PreloadModels     bool    `json:"preload_models"`
}
```

기본값은 minimum confidence `0.70`, minimum length `8` rune, lazy model loading이다.
`PreloadModels=true`는 `language.WithPreloadedLanguageModels()`를 추가하지만 선택된
language subset이나 routing rule은 바꾸지 않는다.

`NewRouter`는 English, Korean, Japanese, Chinese용 detector를 정확히 하나
construct한다. Router에는 per-call mutable state, goroutine, channel, I/O, timer,
shutdown method, context contract가 없다. Command는 router 하나를 만들고 reuse한다.
Lazy loading이 기본값이며, `--preload`는 construction cost를 upfront로 지불한다.
Documentation은 universal startup-time 또는 memory number 없이 이 qualitative
tradeoff를 설명한다.

## Domain 계약

### 입력

`Request`는 필수 caller-owned ID와 text를 포함한다. Blank ID는 local invalid-request
error를 반환한다. Router는 ID 주변 whitespace를 trim하고 canonical ID를 반환하지만,
section offset이 원본 byte를 slice해야 하므로 text는 trim하거나 rewrite하지 않는다.
Blank text와 upstream maximum을 초과한 text는 `%w`를 통해 bluetape-go sentinel을
보존한다. `MinimumRunes`보다 짧은 nonblank input은 error가 아니라 valid manual-review
decision이다.

### 출력

`Decision`은 다음을 포함한다.

- request ID와 original text
- detected language name, ISO 639-1/639-3 code, top confidence
- 네 개 configured language confidence 전체를 descending order로
- language, ISO code, original byte span, exact text slice를 가진 detector-reported
  section
- `latin`, `hangul`, `kana`, `han`에서 나온 ordered script hint
- selected route
- `manual_review` 및 ordered non-nil `review_reasons` slice

Unknown detection은 명시적인 `unknown` language value와 empty ISO code를 사용한다.
JSON shape가 안정적으로 유지되도록 성공 slice는 비어 있어도 non-nil이다. Confidence
ordering은 detector가 소유한다. 예제는 이를 verify하지만 probability를 re-rank하거나
synthesize하지 않는다.

### Route

Route 값:

- confident non-mixed English 또는 Korean에는 `moderation`
- Kana hint가 있는 confident non-mixed Japanese에는 `japanese-tokenization`
- review reason이 하나라도 있으면 `manual-review`

Han-only input이 보이도록 Chinese를 detector subset에 의도적으로 포함하지만, 이 예제는
Chinese processor를 지원하지 않는다.

| Evidence | Route | Review reason |
|---|---|---|
| Confident English, non-mixed | `moderation` | none |
| Confident Korean, non-mixed | `moderation` | none |
| Confident Japanese with Kana, non-mixed | `japanese-tokenization` | none |
| Confident Chinese/Han-only | `manual-review` | `unsupported-language` |
| Japanese detection without Kana and with Han | `manual-review` | `ambiguous-cjk-script` |
| Short, unknown, low-confidence, or mixed input | `manual-review` | corresponding ordered reasons |

Route name은 downstream destination만 설명한다. Command는 moderator, tokenizer,
queue, HTTP service를 호출하지 않는다.

## Decision Flow

모든 request에 대해 `Route`는 다음 단계를 순서대로 수행한다.

1. Router construction, ID, blank text, upstream length를 검증한다.
2. Latin, Hangul, Kana, Han의 고정 순서로 script hint를 수집한다.
3. Shared detector에서 `Detect`, `Confidences`, `DetectMultiple`을 호출한다.
4. Detector section에 서로 다른 non-`Unknown` language가 둘 이상 있으면 input을
   mixed로 표시한다. `Unknown` section은 evidence로 남지만 두 번째 language
   decision을 만들지 않는다. Script hint는 evidence로 남으며 detector section을
   독립적으로 재정의하지 않는다.
5. Review reason을 다음 정확한 순서로 append한다.
   `text-too-short`, `language-unknown`, `low-confidence`, `mixed-language`,
   `ambiguous-cjk-script`, `unsupported-language`.
6. Reason이 하나라도 있으면 `manual-review`를 선택한다. 그렇지 않으면 confident
   English/Korean/Japanese route matrix를 적용한다.

`language-unknown`은 language가 감지되지 않았을 때 적용한다. `low-confidence`는
threshold 아래의 detected language에만 적용하므로 unknown input이 같은 uncertainty에
대해 두 이름을 받지 않는다. Japanese guard는 Kana를 요구한다. Kana 없는 Japanese
result는 Japanese processor를 호출할 permission이 아니라 ambiguous CJK evidence로
취급한다. Han은 별도의 observable script hint로 유지한다. Chinese result는
`unsupported-language`를 만든다. 하나의 fixture가 여러 independent guard를 trigger해도
reason은 deduplicate되고 안정적이다.

## Go API Shape

Internal package는 constructor-only 사용을 노출한다.

```go
func DefaultConfig() Config
func NewRouter(config Config) (*Router, error)
func (r *Router) Route(request Request) (Decision, error)
func NewPreview(preload bool) (Preview, error)
```

Zero value는 의도적으로 사용할 수 없다. Nil 또는 uninitialized router에서 `Route`를
호출하면 panic 대신 `ErrInvalidConfig`로 fail closed한다. 반환되는 모든 slice는
caller-owned copy다.

`Preview`는 scenario name, default configuration, `lazy` 또는 `preloaded`
`model_loading` 값, qualitative lifecycle note, heuristic boundary, fixed
default-policy decision, 명시적인 low-confidence policy check 하나, validation
command를 포함한다. Lifecycle field는 동등한 lazy/preloaded preview 사이에서 lesson
behavior가 의도적으로 다른 유일한 부분이다. Serialized `preload_models`
configuration field도 선택된 mode를 반영한다. Default request decision과
low-confidence policy check는 model-loading mode 사이에서 같아야 한다.

Dedicated low-confidence check는 같은 detector subset과 lifecycle을 사용하지만 fixed
multilingual-script fixture `support 문의 订单 delivery`에 대해 `MinimumConfidence`를
`1.0`으로 올린다. Pin된 v0.18.0에서 detected confidence는 `1.0`보다 낮으므로, 이
check는 default `0.70` policy를 바꾸지 않고 `low-confidence`를 만든다. 정확한
confidence는 test evidence이며 문서화된 accuracy guarantee가 아니다.

## Error

Package는 다음을 정의한다.

- `[0,1]` 밖의 confidence, non-positive minimum rune, nil/uninitialized router
  use에 대한 `ErrInvalidConfig`
- blank request ID에 대한 `ErrInvalidRequest`

Detector construction과 request operation은 cause를 `%w`로 wrap한다.
`language.ErrBlankText`와 `language.ErrTextTooLong`은 `errors.Is`로 찾을 수 있어야
한다. Unknown, short, low-confidence, mixed, ambiguous-CJK,
unsupported-language outcome은 valid `Decision` 값이며 error가 아니다.

Invalid input 또는 detector failure에서는 partial decision을 반환하지 않는다.

## Concurrency 계약

하나의 `Router`와 그 detector는 concurrent reuse에 안전하다. 모든 request result,
reason slice, confidence projection, section projection은 call-local이다. 테스트는
capacity 6 worker pool을 사용하고 round마다 request 6개를 dispatch하며, 3 round를
실행해 정확히 18 route call을 만든다. Bounded entry gate는 각 round를 release하기
전에 6 goroutine이 모두 준비되었음을 증명한다. Exact outcome, cross-round
determinism, race detector가 safe reuse를 증명한다. 이 테스트는 upstream detector
내부 overlap을 계측한다고 주장하지 않는다. Focused test process는 route call 전에
fresh lazy router를 exercise한다. 같은 gate는 scheduler parallelism에 의존하지 않고
`GOMAXPROCS=1`에서도 완료된다.

같은 테스트는 focused normal 및 race command에서 실행한다. Unbounded goroutine을
만들지 않고, correctness를 위해 sleep하지 않으며, latency claim을 publish하지 않는다.

## Command Output

두 command는 deterministic indented JSON을 출력한다.

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Default-policy fixture는 confident English, Korean, Kana가 있는 Japanese,
Chinese/Han-only, mixed English/Japanese, short text, numeric unknown text를 다룬다.
별도 label이 붙은 low-confidence policy check는 threshold `1.0`과 fixed
multilingual-script fixture `support 문의 订单 delivery`를 사용한다. 테스트는 pin된
v0.18.0 result가 configured threshold 아래로 detect되고 `low-confidence`를 만든다는
점만 lock한다. 해당 `Unknown` section은 독립적으로 `mixed-language`를 만들지 않는다.
문서는 numeric confidence를 universal detector guarantee로 publish하지 않는다.

한 mode 안에서 repeated output은 byte-identical이다. Lazy output과 preloaded output은
동일한 request decision을 갖고, 명시적인 lifecycle/configuration metadata만 다르다.

Request마다 세 개의 public evidence view를 노출하는 것은 의도적이다. 이 lesson은
single-language, confidence-list, mixed-section evidence를 함께 보여준다. 해당 API는
detector computation을 반복하고 result projection을 allocate할 수 있다. README는
production caller가 자신의 policy에 필요한 evidence만 요청해야 하며, 이 예제가
hot-path throughput template이 아니라고 설명해야 한다.

Command는 fixed non-sensitive fixture만 받는다. `Decision`은 section span을 증명하기
위해 original request text를 보존하므로, package shape를 embedding하는 caller는
redaction, access control, log 또는 telemetry의 sensitive text 회피 책임을 계속
가진다.

## Failure Mode 및 Guard

1. **Han-only text가 Japanese tokenizer로 전송된다.** Japanese route에는 Kana hint를
   요구하고 Han-only Japanese/Chinese evidence는 명시적인 reason과 함께 manual
   review로 보낸다.
2. **Uncertainty가 internal error로 숨겨진다.** Short, unknown, low-confidence,
   mixed, ambiguous, unsupported input은 valid decision으로 model한다. Error는
   invalid request/configuration 또는 detector failure에만 둔다.
3. **Lazy router와 preloaded router의 behavior가 drift된다.** 둘 다 하나의 policy
   path에서 construct하고, lifecycle metadata만 달라도 되도록 허용하면서 모든
   decision을 비교한다.
4. **Mixed section이 invalid offset을 노출한다.** Upstream byte offset을 보존하고
   multibyte Japanese input을 포함해 모든 `text[start:end]`가 반환된 section text와
   같은지 테스트한다.
5. **Shared detector reuse가 caller-visible slice를 변경한다.** 모든 result
   projection은 call마다 allocate하고 bounded normal/race test 아래 exact result를
   증명한다.
6. **Routing을 authorization 또는 compliance로 오해한다.** 두 README locale과
   preview boundary에서 language detection은 heuristic evidence이며 authentication,
   authorization, sanctions, compliance decision을 gate할 수 없다고 설명한다.
7. **설정된 low-confidence case가 dependency upgrade 이후 조용히 바뀐다.** Default
   threshold는 `0.70`으로 유지하고 threshold `1.0` policy check를 분리하며,
   `detected && confidence < threshold`와 ordered reason만 lock한다. Future detector
   change는 assertion을 약화하는 대신 의도적인 fixture/policy review를 강제해야 한다.

## 테스트

테스트는 구현 전에 작성하며 다음을 다룬다.

1. Default 및 invalid configuration, selected four-language subset, nil router
   behavior.
2. Confident English와 Korean이 distinct language/ISO metadata를 유지하면서
   `moderation` route를 공유하는지.
3. Kana가 있는 confident Japanese가 `japanese-tokenization`을 선택하는지.
4. Chinese 및 Han-only ambiguity가 올바른 reason과 함께 manual review를
   선택하는지.
5. `errors.Is` 보존을 포함한 blank ID, blank text, oversized text.
6. Short, numeric unknown, mixed default fixture와 별도 threshold `1.0`
   low-confidence check가 exact ordered reason을 갖는지.
7. Four descending confidence entry와 stable ISO code.
8. Distinct-language section mixing과 exact UTF-8 byte-span slicing.
9. Deterministic non-nil caller-owned slice.
10. Lazy/preloaded decision equality 및 intentional lifecycle metadata 차이.
    이는 in-process startup 또는 memory measurement가 아니라 option wiring과
    behavioral equivalence 범위다.
11. Six-participant release gate, exact completion/result, cross-round
    determinism, race proof를 가진 bounded shared-router reuse.
12. CLI flag parsing, repeated deterministic JSON, 두 model-loading mode 사이의
    route equality.

CLI test는 injected argument slice와 output/error writer를 사용한다. Unknown flag는
deterministic error와 함께 non-zero result를 반환하고, `--help`는 표준 successful
usage path를 반환한다. Preview construction failure는 stderr에 쓰고 stdout에 partial
JSON 없이 non-zero를 반환한다.

Focused validation:

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

저장소 validation:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

Container-backed test는 추가하지 않는다. 기존 repository-wide heavyweight test는
authoritative `make ci` gate를 통해 계속 serialized 상태로 실행된다.

## Compatibility 및 Migration

이는 새 standalone example이며 bluetape-go API, 기존 workshop API, persisted data,
wire contract, dependency, caller behavior를 변경하지 않는다. Root README pair에는
navigation만 추가된다. Merge 전에 example과 navigation entry를 제거하는 것 외에는
migration 또는 rollback procedure가 필요하지 않다.

Future example은 policy shape를 복사할 수 있지만, 이 `internal` package 또는 preview
JSON을 stable public API로 취급하지 말고 bluetape-go를 직접 import해야 한다.

## 문서

예제 README pair는 다음을 포함한다.

- package lesson, routing matrix, data flow
- run 및 focused test command
- representative route 및 manual-review output
- confidence, mixed-section, script-hint, UTF-8 byte-span semantics
- benchmark claim 없는 lazy versus preloaded lifecycle tradeoff
- 세 public evidence view를 모두 수집하는 deliberate cost와 policy가 사용하는
  evidence만 요청하라는 production guidance
- supported 및 unsupported route
- heuristic, security, compliance boundary

README pair는 decision이 span inspection을 위해 original text를 보존하므로 caller의
redaction 및 access-control policy 없이 log나 telemetry로 복사하면 안 된다고
경고한다.

`README.md`는 English로 유지하고 `README.ko.md`는 source-equivalent natural Korean으로
유지한다. Root README pair는 navigation row 하나와 compact run command를 추가한다.
Diagram은 N/A다. 이 작은 예제에서는 three-stage linear flow와 six-row routing table이
text로 더 명확하고 inspect하기 쉽다.

## 비목표

- HTTP/Gin endpoint 없음. Issue #67이 integrated web routing을 소유한다.
- Moderator, tokenizer, queue, persistence, database, cache, container 없음.
- 새 detector dependency 또는 복사한 detector/script algorithm 없음.
- Unsupported Chinese에서 Japanese processing으로의 automatic fallback 없음.
- Ranking, translation, locale negotiation, multilingual content merge 없음.
- Detector accuracy, startup latency, memory benchmark claim 없음.
- Authentication, authorization, sanctions, compliance 또는 다른 security
  decision에 detected language를 사용하지 않음.

## Acceptance Mapping

| Issue requirement | Design evidence |
|---|---|
| Small language subset | Reusable English/Korean/Japanese/Chinese detector 하나. |
| Confidence lists and explicit low-confidence fallback | Default `0.70` policy를 보존하는 four descending confidence entry와 별도 threshold `1.0` check. |
| Mixed sections and script hints | Exact byte slice를 가진 detector section, Latin/Hangul/Kana/Han hint. |
| Deterministic routing policy | Ordered reason과 명시적인 route matrix 하나. |
| Lazy/preloaded lifecycle comparison | Constructor path 하나, `--preload`, equal decision, qualitative metadata만. |
| Shared detector race proof | Normal 및 race command 아래 bounded 6-worker/6-task/3-round test. |
| English, Korean, Japanese/CJK, short/unknown/mixed coverage | Fixed fixture set 및 exact route/reason assertion. |
| Bilingual example docs and root navigation | README locale pair와 root navigation pair. |
| Heuristic/security boundary | Preview note와 두 README locale이 security/compliance 사용을 명시적으로 거부. |
| Repository quality | Focused command 이후 authoritative `make ci` gate. |

## Spec Review

필수 native review lane은 performance, stability, user/caller 관점의 read-only
scope로 dispatch되었다. Repeated bounded wait와 immediate-return request 이후에도
응답하지 않았으므로, main session은 workflow의 timeout fallback을 적용하고 이 exact
artifact에 대해 여섯 lens를 모두 완료했다.

| Priority | Lens | Evidence | Resolution |
|---|---|---|---|
| P1 | Stability | 원래 concurrency wording은 18 calls를 요구하면서 108 calls로 읽힐 수 있었다. | 최대 six worker pool에서 round마다 six request로 정의해 three round 동안 정확히 18 calls가 되게 했다. |
| P1 | Developer/API | Kana 없는 Japanese와 명시적 fallback 부재는 route matrix를 불완전하게 만들 수 있었다. | Kana 없는 모든 detected Japanese result가 이제 `ambiguous-cjk-script`를 받는다. |
| P1 | Security | `Decision`은 original text를 보존하지만 caller의 logging/privacy duty가 명시되지 않았다. | CLI를 fixed fixture로 제한하고 redaction, access-control, telemetry guidance를 명시적으로 추가했다. |
| P2 | Performance | Request마다 세 detector query를 수행하는 것이 production hot-path template으로 오해될 수 있었다. | Teaching cost와 필요한 evidence만 수집하라는 production guidance를 문서화했다. |
| P2 | Operator/Ops | CLI error/help behavior와 partial-output boundary가 명시적이지 않았다. | Injected CLI seam, non-zero error behavior, standard help, stderr, partial JSON 없음 조건을 추가했다. |
| P2 | User/caller | Canonical ID behavior와 lazy/preloaded comparison field가 모호했다. | Trimmed ID, byte-exact text, equal decision, 두 explicit lifecycle/config field를 명시했다. |
| P1 | Evidence integrity | Planning-time v0.18.0 execution은 `문의 注文 support`가 default `0.70` threshold 아래라는 원래 claim을 반증했다. | 사용자 승인으로 default `0.70`을 보존하고 stable fixture `support 문의 订单 delivery`를 사용하는 threshold `1.0` check를 분리했다. 테스트는 universal numeric claim이 아니라 relation을 assert한다. |
| P1 | Performance/stability | 원래 `maxActive` gate는 waiting goroutine을 측정했고 detector 내부 overlap을 증명할 수 없었다. | Internal-overlap claim을 제거했다. Fresh lazy target, six ready participants, exact outcome, cross-round determinism, race proof를 요구한다. |

Integration review는 남은 contradiction, unsupported assumption, open material user
decision을 찾지 못했다. 최신 convergence: P0=0, P1=0. 나열된 P2 item은 이 artifact
안에서 해결되었으며 follow-up issue가 필요하지 않다.

## Definition of Done

- Command는 모든 route와 review state에 대한 deterministic evidence를 출력한다.
- English와 Korean은 language metadata를 보존하면서 `moderation`을 공유한다.
- Japanese processing은 confident non-mixed Kana evidence가 있을 때만 선택한다.
- Chinese/Han-only 및 모든 uncertain input은 manual review로 fail closed한다.
- Lazy router와 preloaded router는 동일한 decision을 생성한다.
- Fresh execution에서 focused normal/race test와 `make ci`가 통과한다.
- Example 및 root README locale pair는 실제 command behavior와 일치한다.
- 명시적인 merge boundary 전에 Type A spec, plan, verifier, six-lane review,
  lesson, PR metadata, CI, final review gate가 완료된다.
