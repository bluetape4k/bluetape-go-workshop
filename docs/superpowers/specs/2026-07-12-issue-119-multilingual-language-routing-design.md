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

Both commands print deterministic indented JSON:

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Default-policy fixtures cover confident English, Korean, Japanese with Kana,
Chinese/Han-only, mixed English/Japanese, short text, and numeric unknown text.
A separately labeled low-confidence policy check uses threshold `1.0` and the
fixed multilingual-script fixture `support 문의 订单 delivery`. Tests lock only
that the pinned v0.18.0 result is detected below the configured threshold and
produces `low-confidence`; its `Unknown` section does not independently create
`mixed-language`. Documentation does not publish the numeric confidence as a
universal detector guarantee.

Within one mode, repeated output is byte-identical. Lazy and preloaded outputs
have identical request decisions; only explicit lifecycle/configuration
metadata differs.

The three public evidence views per request are intentional because this lesson
exposes single-language, confidence-list, and mixed-section evidence together.
Those APIs can repeat detector computation and allocate result projections. The
README must state that production callers should request only the evidence their
policy needs; this example is not a hot-path throughput template.

The command accepts only fixed, non-sensitive fixtures. `Decision` retains the
original request text to prove section spans, so callers embedding the package
shape remain responsible for redaction, access control, and avoiding sensitive
text in logs or telemetry.

## Failure Modes and Guards

1. **Han-only text is sent to a Japanese tokenizer.** Require a Kana hint for
   the Japanese route and send Han-only Japanese/Chinese evidence to manual
   review with an explicit reason.
2. **Uncertainty is hidden as an internal error.** Model short, unknown,
   low-confidence, mixed, ambiguous, and unsupported input as valid decisions;
   reserve errors for invalid requests/configuration or detector failures.
3. **Lazy and preloaded routers drift in behavior.** Construct both from one
   policy path and compare every decision while allowing only lifecycle
   metadata to differ.
4. **Mixed sections expose invalid offsets.** Preserve upstream byte offsets
   and test that every `text[start:end]` equals the returned section text,
   including multibyte Japanese input.
5. **Shared detector reuse mutates caller-visible slices.** Allocate all result
   projections per call and prove exact results under bounded normal/race tests.
6. **Routing is mistaken for authorization or compliance.** State in both
   README locales and preview boundaries that language detection is heuristic
   evidence and cannot gate authentication, authorization, sanctions, or
   compliance decisions.
7. **A configured low-confidence case changes silently after dependency
   upgrades.** Keep the default threshold at `0.70`, isolate a threshold `1.0`
   policy check, and lock only `detected && confidence < threshold` plus ordered
   reasons. A future detector change must force an intentional fixture/policy
   review rather than weakening the assertion.

## Testing

Tests are written before implementation and cover:

1. default and invalid configuration, selected four-language subset, and nil
   router behavior;
2. confident English and Korean sharing the `moderation` route while retaining
   distinct language/ISO metadata;
3. confident Japanese with Kana selecting `japanese-tokenization`;
4. Chinese and Han-only ambiguity selecting manual review with the correct
   reason;
5. blank ID, blank text, and oversized text with `errors.Is` preservation;
6. short, numeric unknown, and mixed default fixtures plus the separate
   threshold `1.0` low-confidence check with exact ordered reasons;
7. four descending confidence entries and stable ISO codes;
8. distinct-language section mixing and exact UTF-8 byte-span slicing;
9. deterministic, non-nil caller-owned slices;
10. lazy/preloaded decision equality and intentional lifecycle metadata
    difference, scoped to option wiring and behavioral equivalence rather than
    in-process startup or memory measurement;
11. bounded shared-router reuse with six-participant release gates, exact
    completion/results, cross-round determinism, and race proof;
12. CLI flag parsing, repeated deterministic JSON, and route equality across
    both model-loading modes.

CLI tests use an injected argument slice and output/error writers. Unknown
flags return a non-zero result with a deterministic error, while `--help`
returns the standard successful usage path. Preview construction failures are
written to stderr and return non-zero without partial JSON on stdout.

Focused validation is:

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Repository validation remains:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

No container-backed test is added. Existing repository-wide heavyweight tests
remain serialized through the authoritative `make ci` gate.

## Compatibility and Migration

This is a new standalone example and changes no bluetape-go API, existing
workshop API, persisted data, wire contract, dependency, or caller behavior.
The root README pair gains navigation only. No migration or rollback procedure
is needed beyond removing the example and navigation entries before merge.

Future examples may copy the policy shape, but they must import bluetape-go
directly rather than treating this `internal` package or preview JSON as a
stable public API.

## Documentation

The example README pair will include:

- package lesson, routing matrix, and data flow;
- run and focused test commands;
- representative route and manual-review output;
- confidence, mixed-section, script-hint, and UTF-8 byte-span semantics;
- lazy versus preloaded lifecycle tradeoff without benchmark claims;
- the deliberate cost of collecting all three public evidence views and the
  production guidance to request only what a policy uses;
- supported and unsupported routes;
- the heuristic, security, and compliance boundary.

The README pair also warns that decisions retain original text for span
inspection and therefore must not be copied into logs or telemetry without the
caller's redaction and access-control policy.

`README.md` remains English and `README.ko.md` remains source-equivalent natural
Korean. The root README pair adds one navigation row and compact run command.
A diagram is N/A: the three-stage linear flow and six-row routing table are
clearer and more inspectable as text for this small example.

## Non-goals

- No HTTP/Gin endpoint; Issue #67 owns integrated web routing.
- No moderator, tokenizer, queue, persistence, database, cache, or container.
- No new detector dependency or copied detector/script algorithm.
- No automatic fallback from unsupported Chinese to Japanese processing.
- No ranking, translation, locale negotiation, or multilingual content merge.
- No detector accuracy, startup latency, or memory benchmark claim.
- No use of detected language for authentication, authorization, sanctions,
  compliance, or other security decisions.

## Acceptance Mapping

| Issue requirement | Design evidence |
|---|---|
| Small language subset | One reusable English/Korean/Japanese/Chinese detector. |
| Confidence lists and explicit low-confidence fallback | Four descending confidence entries plus a separate threshold `1.0` check that preserves the default `0.70` policy. |
| Mixed sections and script hints | Detector sections with exact byte slices; Latin/Hangul/Kana/Han hints. |
| Deterministic routing policy | Ordered reasons and one explicit route matrix. |
| Lazy/preloaded lifecycle comparison | One constructor path, `--preload`, equal decisions, qualitative metadata only. |
| Shared detector race proof | Bounded 6-worker/6-task/3-round test under normal and race commands. |
| English, Korean, Japanese/CJK, short/unknown/mixed coverage | Fixed fixture set and exact route/reason assertions. |
| Bilingual example docs and root navigation | README locale pair plus root navigation pair. |
| Heuristic/security boundary | Preview notes and both README locales explicitly reject security/compliance use. |
| Repository quality | Focused commands followed by the authoritative `make ci` gate. |

## Spec Review

The required native review lanes were dispatched with read-only scopes for
performance, stability, and user/caller perspectives. They did not respond
after repeated bounded waits and an immediate-return request, so the main
session applied the workflow's timeout fallback and completed all six lenses
against this exact artifact.

| Priority | Lens | Evidence | Resolution |
|---|---|---|---|
| P1 | Stability | The original concurrency wording could mean 108 calls while requiring 18. | Defined six requests per round across a pool of up to six workers, for exactly 18 calls over three rounds. |
| P1 | Developer/API | Japanese without Kana and without an explicit fallback could leave the route matrix incomplete. | Any detected Japanese result without Kana now receives `ambiguous-cjk-script`. |
| P1 | Security | `Decision` retains original text but the caller's logging/privacy duty was unstated. | Limited the CLI to fixed fixtures and added explicit redaction, access-control, and telemetry guidance. |
| P2 | Performance | Three detector queries per request could be mistaken for a production hot-path template. | Documented the teaching cost and production guidance to gather only required evidence. |
| P2 | Operator/Ops | CLI error/help behavior and partial-output boundary were not explicit. | Added injected CLI seams, non-zero error behavior, standard help, stderr, and no partial JSON. |
| P2 | User/caller | Canonical ID behavior and lazy/preloaded comparison fields were ambiguous. | Specified trimmed IDs, byte-exact text, equal decisions, and the two explicit lifecycle/config fields. |
| P1 | Evidence integrity | Planning-time v0.18.0 execution disproved the original claim that `문의 注文 support` was below the default `0.70` threshold. | With user approval, preserved default `0.70` and isolated a threshold `1.0` check using stable fixture `support 문의 订单 delivery`; tests assert the relation, not a universal numeric claim. |
| P1 | Performance/stability | The original `maxActive` gate measured waiting goroutines and could not prove overlap inside the detector. | Removed the internal-overlap claim; require a fresh lazy target, six ready participants, exact outcomes, cross-round determinism, and race proof. |

Integration review found no remaining contradiction, unsupported assumption,
or open material user decision. Latest convergence: P0=0, P1=0. The listed P2
items are resolved in this artifact and require no follow-up issue.

## Definition of Done

- The command prints deterministic evidence for every route and review state.
- English and Korean share `moderation` while preserving language metadata.
- Japanese processing is selected only with confident, non-mixed Kana evidence.
- Chinese/Han-only and all uncertain inputs fail closed to manual review.
- Lazy and preloaded routers produce identical decisions.
- Focused normal/race tests and `make ci` pass from fresh executions.
- Both example and root README locale pairs match actual command behavior.
- Type A spec, plan, verifier, six-lane reviews, lesson, PR metadata, CI, and
  final review gates are complete before the explicit merge boundary.
