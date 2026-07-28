# Issue #119 Multilingual Language Routing 구현 계획

> **에이전트 작업자 참고:** 필수 하위 스킬: `superpowers:subagent-driven-development`(권장) 또는 `superpowers:executing-plans`를 사용해 이 계획을 작업 단위로 구현한다. 단계 추적에는 체크박스(`- [ ]`) 문법을 사용한다.

**목표:** Lazy detector lifecycle과 preloaded detector lifecycle을 비교하면서 bluetape-go v0.18.0 language evidence를 moderation, Japanese tokenization, manual review로 route하는 deterministic CLI example을 만든다.

**아키텍처:** 프레임워크에 독립적인 `internal/routing.Router`가 재사용 가능한 four-language detector 하나를 소유하고 call-local evidence와 decision을 만든다. Fixed-fixture preview와 얇고 testable한 CLI는 lazy/preloaded mode에서 동일한 route decision을 노출한다. Bilingual README는 policy와 security/privacy boundary를 설명한다.

**기술 스택:** Go 1.26.3, `github.com/bluetape4k/bluetape-go/textsearch/language` v0.18.0, 표준 `flag`, `encoding/json`, `sync`, `sync/atomic` package.

---

## 승인된 입력과 실행 계약

- Spec: `docs/superpowers/specs/2026-07-12-issue-119-multilingual-language-routing-design.md`
- Issue: GitHub #119, milestone `0.8.0`, labels `enhancement` and `examples`, assignee `debop`
- Base: `origin/develop` at `3f68aea7eeb42c45f09bd927f52a451f164ad8bb`
- Worktree: `.worktrees/feat-issue-119-multilingual-language-routing`
- Branch: `feat/issue-119-multilingual-language-routing`
- Dependency baseline: `github.com/bluetape4k/bluetape-go v0.18.0`
- Implementation workflow: 이 계획이 승인된 뒤 서로 겹치지 않는 write task마다 fresh `executor`와 함께 `subagent-driven-development`를 사용하고, 다음 task 전에 `verifier`와 `code-reviewer`를 거친다.
- Executor는 uncommitted scoped diff 상태에서 멈춘다. Main session은 diff를 통합하고 review repair를 적용하며 proof를 다시 실행하고 모든 commit과 external effect를 소유한다. PR creation, CI monitoring, rebase merge, issue update, local sync, worktree cleanup은 사용자가 이미 승인한 delivery scope를 따른다.

## 파일 지도와 Ownership

| File | 책임 | Write task |
|---|---|---|
| `examples/multilingual-language-routing/internal/routing/router.go` | Type, config, shared detector, evidence projection, route matrix | Task 1 only |
| `examples/multilingual-language-routing/internal/routing/router_test.go` | Core, lifecycle, fixture, concurrency, race contract | Tasks 1-2, disjoint test sections |
| `examples/multilingual-language-routing/internal/routing/preview.go` | Fixed fixture와 deterministic preview metadata | Task 2 only |
| `examples/multilingual-language-routing/main.go` | Testable flag parsing과 deterministic JSON output | Task 3 only |
| `examples/multilingual-language-routing/main_test.go` | CLI help/error/determinism/mode-equivalence test | Task 3 only |
| `examples/multilingual-language-routing/README.md` | English lesson과 actual output | Task 4 only |
| `examples/multilingual-language-routing/README.ko.md` | Source-equivalent natural Korean lesson | Task 4 only |
| `README.md`, `README.ko.md` | Root example navigation과 run command | Task 4 only |
| `docs/lessons/2026-07-12-issue-119-multilingual-language-routing.md` | Durable outcome, evidence, miss, future guard | Task 5 only |

`go.mod`, `go.sum`, workflow, Nightly, Docker, Testcontainers, coverage, catalog, AGENTS file은 변경하면 안 된다. 이런 diff는 stop condition이다.

## Spec-to-Task Traceability

| Acceptance criterion | Task와 proof |
|---|---|
| Four-language subset과 reusable detector | Task 1 constructor test와 package inspection |
| English/Korean common moderation route | Task 1 route matrix table test |
| Kana-backed Japanese route와 Han-only fail-closed behavior | Task 1 Japanese/Chinese test |
| Confidence list, ISO code, section, script hint | Task 1 evidence test와 byte slicing |
| Short/unknown/low-confidence/mixed ordered review reason | Task 1 exact table assertion |
| Lazy/preloaded lifecycle equality | Task 2 preview comparison |
| Shared detector bounded race proof | Task 2 exact 18-call concurrency test와 `go test -race` |
| Deterministic CLI와 `--preload` | Task 3 repeated byte comparison과 decoded decision comparison |
| Bilingual lesson과 root navigation | Task 4 locale parity review와 run-output evidence |
| Heuristic/security/privacy boundary | Tasks 2 and 4 preview note와 README boundary section |
| Repository quality | Task 5 `make ci`, diff audit, verifier와 review convergence |

## Risk Prediction

| Risk | Signal | Mitigation | Rerun/rollback point |
|---|---|---|---|
| Lazy first-use model state race | `go test -race` report 또는 inconsistent decision | 한 번만 construct하고 result는 call-local로 유지한다. 두 mode 모두 bounded shared-router test를 둔다. | Task 1/2로 돌아가 focused normal/race를 처음부터 다시 실행한다. |
| Han-only input이 Japanese processing에 도달 | `kana` hint 없이 route가 `japanese-tokenization` | Japanese route는 detected Japanese와 Kana를 모두 요구한다. Exact negative fixture를 둔다. | Route-matrix edit을 revert하고 모든 Task 1 table을 다시 실행한다. |
| Detector upgrade가 low-confidence fixture를 변경 | 명시적 `1.0` threshold 아래에서 candidate가 더 이상 감지되지 않음 | Default `0.70`을 유지한다. `support 문의 订单 delivery` fixture를 threshold `1.0` policy check에 격리하고 의도적으로 fail한다. | Spec에서 fixture/policy를 재평가한다. Assertion을 조용히 약화하지 않는다. |
| Mixed section offset이 UTF-8 slicing을 손상 | `decision.Text[start:end] != section.Text` | Upstream byte span을 그대로 보존하고 모든 section을 assert한다. | Evidence projection으로 돌아가 focused test/race를 다시 실행한다. |
| CLI mode가 lifecycle metadata 외에 다름 | Decoded decision이 다르거나 stdout이 nondeterministic | Constructor/preview path 하나만 두고 preload option으로만 parameterize한다. | Task 2/3으로 돌아가 docs 전 decoded payload를 비교한다. |
| Original text가 log에 copy됨 | README/preview가 privacy boundary를 생략 | Fixed non-sensitive CLI fixture, 명시적 caller redaction/access-control note | 두 locale이 일치할 때까지 Task 4 completion과 PR을 막는다. |
| Three detector pass가 throughput guidance로 취급됨 | README가 performance를 claim하거나 cost를 생략 | Inspectable teaching evidence이며 production은 필요한 view만 수집해야 한다고 설명한다. | Docs review를 막는다. Benchmark number는 추가하면 안 된다. |

## Task 1: Route Policy와 Evidence Contract

**Complexity:** High. 이 task는 complete behavior matrix와 upstream error preservation을 소유한다.

**Required skills:** `test-driven-development`, `bluetape-go-patterns`.

**Files:**

- Create: `examples/multilingual-language-routing/internal/routing/router.go`
- Create: `examples/multilingual-language-routing/internal/routing/router_test.go`

- [x] **Step 1: 실패하는 configuration 및 input test 작성**

Package `routing`에 exact table assertion을 가진 `router_test.go`를 만든다. Import block은 `errors`, `math`, `reflect`, `slices`, `strings`, `testing`, 그리고 아래에서 사용하는 bluetape-go `language` package를 포함한다.

```go
func TestDefaultConfigAndSelectedLanguages(t *testing.T) {
    config := DefaultConfig()
    if config != (Config{MinimumConfidence: 0.70, MinimumRunes: 8}) {
        t.Fatalf("DefaultConfig() = %#v", config)
    }
    router, err := NewRouter(config)
    if err != nil {
        t.Fatal(err)
    }
    want := []language.Language{
        language.Chinese, language.English, language.Japanese, language.Korean,
    }
    got := router.detector.Languages()
    slices.SortFunc(got, func(a, b language.Language) int {
        return strings.Compare(a.String(), b.String())
    })
    slices.SortFunc(want, func(a, b language.Language) int {
        return strings.Compare(a.String(), b.String())
    })
    if !reflect.DeepEqual(got, want) {
        t.Fatalf("Languages() = %v, want %v", got, want)
    }
}

func TestNewRouterRejectsInvalidConfig(t *testing.T) {
    tests := []Config{
        {MinimumConfidence: -0.01, MinimumRunes: 8},
        {MinimumConfidence: 1.01, MinimumRunes: 8},
        {MinimumConfidence: 0.70, MinimumRunes: 0},
        {MinimumConfidence: math.NaN(), MinimumRunes: 8},
    }
    for _, config := range tests {
        if _, err := NewRouter(config); !errors.Is(err, ErrInvalidConfig) {
            t.Fatalf("NewRouter(%#v) error = %v", config, err)
        }
    }
}

func TestRouteRejectsInvalidRequests(t *testing.T) {
    router, err := NewRouter(DefaultConfig())
    if err != nil {
        t.Fatal(err)
    }
    if _, err := (*Router)(nil).Route(Request{ID: "id", Text: "valid support request"}); !errors.Is(err, ErrInvalidConfig) {
        t.Fatalf("nil Router.Route error = %v", err)
    }
    if _, err := router.Route(Request{ID: "  ", Text: "valid support request"}); !errors.Is(err, ErrInvalidRequest) {
        t.Fatalf("blank ID error = %v", err)
    }
    if _, err := router.Route(Request{ID: "blank", Text: "  "}); !errors.Is(err, language.ErrBlankText) {
        t.Fatalf("blank text error = %v", err)
    }
    oversized := strings.Repeat("a", language.MaxTextLength+1)
    if _, err := router.Route(Request{ID: "large", Text: oversized}); !errors.Is(err, language.ErrTextTooLong) {
        t.Fatalf("oversized text error = %v", err)
    }
}
```

- [x] **Step 2: Test 실행 및 RED state 관찰**

Run:

```bash
go test -count=1 ./examples/multilingual-language-routing/internal/routing
```

기대값: `Config`, `DefaultConfig`, `NewRouter`, `Router`, `Request`, `ErrInvalidConfig`, `ErrInvalidRequest`가 없어서 compile FAIL한다.

- [x] **Step 3: 최소 configuration, type, constructor 추가**

Package documentation, English GoDoc, 다음 exact contract를 포함한 `router.go`를 만든다.

```go
package routing

import (
    "errors"
    "fmt"
    "math"
    "strings"

    "github.com/bluetape4k/bluetape-go/textsearch/language"
)

const (
    RouteModeration           = "moderation"
    RouteJapaneseTokenization = "japanese-tokenization"
    RouteManualReview         = "manual-review"

    ReviewTextTooShort        = "text-too-short"
    ReviewLanguageUnknown     = "language-unknown"
    ReviewLowConfidence       = "low-confidence"
    ReviewMixedLanguage       = "mixed-language"
    ReviewAmbiguousCJKScript  = "ambiguous-cjk-script"
    ReviewUnsupportedLanguage = "unsupported-language"
)

var (
    ErrInvalidConfig  = errors.New("routing: invalid config")
    ErrInvalidRequest = errors.New("routing: invalid request")
)

type Config struct {
    MinimumConfidence float64 `json:"minimum_confidence"`
    MinimumRunes      int     `json:"minimum_runes"`
    PreloadModels     bool    `json:"preload_models"`
}

type Request struct {
    ID   string `json:"id"`
    Text string `json:"text"`
}

type Confidence struct {
    Language string  `json:"language"`
    Value    float64 `json:"value"`
    ISO6391  string  `json:"iso_639_1"`
    ISO6393  string  `json:"iso_639_3"`
}

type Section struct {
    Language string `json:"language"`
    Start    int    `json:"start"`
    End      int    `json:"end"`
    Text     string `json:"text"`
    ISO6391  string `json:"iso_639_1"`
    ISO6393  string `json:"iso_639_3"`
}

type Decision struct {
    ID            string       `json:"id"`
    Text          string       `json:"text"`
    Language      string       `json:"language"`
    ISO6391       string       `json:"iso_639_1"`
    ISO6393       string       `json:"iso_639_3"`
    Confidence    float64      `json:"confidence"`
    Confidences   []Confidence `json:"confidences"`
    Sections      []Section    `json:"sections"`
    ScriptHints   []string     `json:"script_hints"`
    Route         string       `json:"route"`
    ManualReview  bool         `json:"manual_review"`
    ReviewReasons []string     `json:"review_reasons"`
}

type Router struct {
    detector *language.Detector
    config   Config
}

func DefaultConfig() Config {
    return Config{MinimumConfidence: 0.70, MinimumRunes: 8}
}

func NewRouter(config Config) (*Router, error) {
    if math.IsNaN(config.MinimumConfidence) || config.MinimumConfidence < 0 || config.MinimumConfidence > 1 || config.MinimumRunes <= 0 {
        return nil, fmt.Errorf("%w: confidence must be in [0,1] and minimum runes must be positive", ErrInvalidConfig)
    }
    options := make([]language.Option, 0, 1)
    if config.PreloadModels {
        options = append(options, language.WithPreloadedLanguageModels())
    }
    detector, err := language.NewDetector([]language.Language{
        language.English, language.Korean, language.Japanese, language.Chinese,
    }, options...)
    if err != nil {
        return nil, fmt.Errorf("create language router detector: %w", err)
    }
    return &Router{detector: detector, config: config}, nil
}
```

Step 1이 compile되고 통과하도록 initial `Route` validation path를 추가한다. Behavior test가 추가되기 전까지는 validation 뒤 non-nil empty-slice decision을 반환한다.

```go
func (r *Router) Route(request Request) (Decision, error) {
    if r == nil || r.detector == nil {
        return Decision{}, fmt.Errorf("%w: router is nil", ErrInvalidConfig)
    }
    id := strings.TrimSpace(request.ID)
    if id == "" {
        return Decision{}, fmt.Errorf("%w: id is required", ErrInvalidRequest)
    }
    if _, err := r.detector.Detect(request.Text); err != nil {
        return Decision{}, fmt.Errorf("detect %q language: %w", id, err)
    }
    return Decision{
        ID: id, Text: request.Text, Confidences: []Confidence{}, Sections: []Section{},
        ScriptHints: []string{}, Route: RouteManualReview, ReviewReasons: []string{},
    }, nil
}
```

- [x] **Step 4: Configuration/input test 실행 및 GREEN 관찰**

Focused package test를 실행한다. 기대값은 race 또는 compile failure 없이 PASS다.

- [x] **Step 5: 실패하는 route 및 evidence table test 작성**

Exact expected route와 reason을 가진 fixture를 추가한다.

```go
func TestRoutePolicy(t *testing.T) {
    router, err := NewRouter(DefaultConfig())
    if err != nil { t.Fatal(err) }
    tests := []struct {
        id, text, language, route string
        review bool
        reasons []string
    }{
        {"en", "Please review the delivery status for order 12345.", "English", RouteModeration, false, []string{}},
        {"ko", "주문 번호 12345의 배송 상태를 확인해 주세요.", "Korean", RouteModeration, false, []string{}},
        {"ja", "配送状況を確認してください。注文番号は12345です。", "Japanese", RouteJapaneseTokenization, false, []string{}},
        {"zh", "请确认订单12345的配送状态。", "Chinese", RouteManualReview, true, []string{ReviewUnsupportedLanguage}},
        {"short", "help", "English", RouteManualReview, true, []string{ReviewTextTooShort}},
        {"unknown", "12345 67890", "unknown", RouteManualReview, true, []string{ReviewLanguageUnknown}},
        {"mixed", "Please check 配送状況を確認してください for order 12345.", "Japanese", RouteManualReview, true, []string{ReviewMixedLanguage}},
    }
    for _, test := range tests {
        t.Run(test.id, func(t *testing.T) {
            got, routeErr := router.Route(Request{ID: "  " + test.id + "  ", Text: test.text})
            if routeErr != nil { t.Fatal(routeErr) }
            if got.ID != test.id || got.Text != test.text || got.Language != test.language || got.Route != test.route || got.ManualReview != test.review {
                t.Fatalf("Route() = %#v", got)
            }
            if test.language == "unknown" && (got.ISO6391 != "" || got.ISO6393 != "") {
                t.Fatalf("unknown ISO codes = %q/%q", got.ISO6391, got.ISO6393)
            }
            if !reflect.DeepEqual(got.ReviewReasons, test.reasons) {
                t.Fatalf("ReviewReasons = %v, want %v", got.ReviewReasons, test.reasons)
            }
            if got.Confidences == nil || got.Sections == nil || got.ScriptHints == nil || got.ReviewReasons == nil {
                t.Fatalf("successful slices must be non-nil: %#v", got)
            }
        })
    }
}

func TestLowConfidenceFallbackUsesExplicitThreshold(t *testing.T) {
    config := DefaultConfig()
    config.MinimumConfidence = 1.0
    router, err := NewRouter(config)
    if err != nil { t.Fatal(err) }
    got, err := router.Route(Request{ID: "low", Text: "support 문의 订单 delivery"})
    if err != nil { t.Fatal(err) }
    if got.Language != "Korean" || got.Confidence >= config.MinimumConfidence {
        t.Fatalf("low-confidence evidence = %#v", got)
    }
    if got.Route != RouteManualReview || !got.ManualReview || !reflect.DeepEqual(got.ReviewReasons, []string{ReviewLowConfidence}) {
        t.Fatalf("low-confidence decision = %#v", got)
    }
}

func TestRouteEvidence(t *testing.T) {
    router, err := NewRouter(DefaultConfig())
    if err != nil { t.Fatal(err) }
    text := "Please check 配送状況を確認してください for order 12345."
    got, err := router.Route(Request{ID: "mixed", Text: text})
    if err != nil { t.Fatal(err) }
    if len(got.Confidences) != 4 { t.Fatalf("len(Confidences) = %d", len(got.Confidences)) }
    for i := 1; i < len(got.Confidences); i++ {
        if got.Confidences[i-1].Value < got.Confidences[i].Value {
            t.Fatalf("Confidences are not descending: %v", got.Confidences)
        }
    }
    if !reflect.DeepEqual(got.ScriptHints, []string{"latin", "kana", "han"}) {
        t.Fatalf("ScriptHints = %v", got.ScriptHints)
    }
    languages := map[string]struct{}{}
    for _, section := range got.Sections {
        if section.Start < 0 || section.End > len(text) || section.Start > section.End || text[section.Start:section.End] != section.Text {
            t.Fatalf("invalid section: %#v", section)
        }
        languages[section.Language] = struct{}{}
    }
    if len(languages) < 2 { t.Fatalf("section languages = %v", languages) }
}

func TestJapaneseWithoutKanaFailsClosed(t *testing.T) {
    router, err := NewRouter(DefaultConfig())
    if err != nil { t.Fatal(err) }
    got, err := router.Route(Request{ID: "han", Text: "注文番号配送確認依頼"})
    if err != nil { t.Fatal(err) }
    if got.Language == "Japanese" && !slices.Contains(got.ReviewReasons, ReviewAmbiguousCJKScript) {
        t.Fatalf("Japanese Han-only decision = %#v", got)
    }
    if got.Route != RouteManualReview { t.Fatalf("Route = %q", got.Route) }
}
```

명시적 low-confidence expectation을 고정하기 전에 pinned v0.18.0 fixture가 `1.0` 아래에서 Korean으로 감지되고, Korean plus `Unknown` section이 `mixed-language`를 만들지 않는지 확인한다. Live pinned dependency가 맞지 않으면 threshold를 바꾸거나 test를 약화하지 말고 중단한 뒤 spec을 다시 연다.

- [x] **Step 6: Route test 실행 및 RED state 관찰**

Focused package test를 실행한다. 기대값은 `Route`가 confidence/section/hint를 project하지 않거나 route matrix를 적용하지 않아 FAIL하는 것이다.

- [x] **Step 7: Complete route decision 구현**

`router.go` import block에 `"unicode/utf8"`을 추가한다. Validation 뒤 provisional `Route` body를 다음으로 교체한다.

```go
detected, err := r.detector.Detect(request.Text)
if err != nil { return Decision{}, fmt.Errorf("detect %q language: %w", id, err) }
rawConfidences, err := r.detector.Confidences(request.Text)
if err != nil { return Decision{}, fmt.Errorf("compute %q confidences: %w", id, err) }
rawSections, err := r.detector.DetectMultiple(request.Text)
if err != nil { return Decision{}, fmt.Errorf("detect %q language sections: %w", id, err) }

decision := Decision{
    ID: id, Text: request.Text, Language: detected.Language.String(),
    ISO6391: detected.ISO6391, ISO6393: detected.ISO6393,
    Confidence: detected.Confidence,
    Confidences: projectConfidences(rawConfidences),
    Sections: projectSections(rawSections),
    ScriptHints: scriptHints(request.Text),
    Route: RouteManualReview,
    ReviewReasons: []string{},
}
if !detected.Detected { decision.Language = "unknown" }
if utf8.RuneCountInString(request.Text) < r.config.MinimumRunes {
    decision.ReviewReasons = append(decision.ReviewReasons, ReviewTextTooShort)
}
if !detected.Detected || detected.Language == language.Unknown {
    decision.ReviewReasons = append(decision.ReviewReasons, ReviewLanguageUnknown)
} else if detected.Confidence < r.config.MinimumConfidence {
    decision.ReviewReasons = append(decision.ReviewReasons, ReviewLowConfidence)
}
if hasMultipleLanguages(rawSections) {
    decision.ReviewReasons = append(decision.ReviewReasons, ReviewMixedLanguage)
}
if detected.Language == language.Japanese && !language.ContainsJapanese(request.Text) {
    decision.ReviewReasons = append(decision.ReviewReasons, ReviewAmbiguousCJKScript)
}
if detected.Language == language.Chinese {
    decision.ReviewReasons = append(decision.ReviewReasons, ReviewUnsupportedLanguage)
}
decision.ManualReview = len(decision.ReviewReasons) > 0
if !decision.ManualReview {
    switch detected.Language {
    case language.English, language.Korean:
        decision.Route = RouteModeration
    case language.Japanese:
        decision.Route = RouteJapaneseTokenization
    default:
        decision.Route = RouteManualReview
        decision.ManualReview = true
        decision.ReviewReasons = append(decision.ReviewReasons, ReviewUnsupportedLanguage)
    }
}
return decision, nil
```

`scriptHints`, `projectConfidences`, `projectSections`, `hasMultipleLanguages`를 pure helper로 구현한다. `scriptHints`는 Latin, Hangul, Kana, Han 순서로만 append한다. `hasMultipleLanguages`는 `language.Unknown`을 무시하고 서로 다른 non-Unknown section language 두 개를 보면 true를 반환한다. Projection function은 `make`로 non-nil slice를 할당하고 모든 ISO field를 copy하며 upstream offset을 sort하거나 변경하지 않는다.

```go
func scriptHints(text string) []string {
    hints := make([]string, 0, 4)
    if language.ContainsLatin(text) { hints = append(hints, "latin") }
    if language.ContainsKorean(text) { hints = append(hints, "hangul") }
    if language.ContainsJapanese(text) { hints = append(hints, "kana") }
    if language.ContainsChinese(text) { hints = append(hints, "han") }
    return hints
}

func projectConfidences(values []language.Confidence) []Confidence {
    result := make([]Confidence, len(values))
    for i, value := range values {
        result[i] = Confidence{
            Language: value.Language.String(), Value: value.Value,
            ISO6391: value.ISO6391, ISO6393: value.ISO6393,
        }
    }
    return result
}

func projectSections(values []language.Section) []Section {
    result := make([]Section, len(values))
    for i, value := range values {
        result[i] = Section{
            Language: value.Language.String(), Start: value.Start, End: value.End,
            Text: value.Text, ISO6391: value.ISO6391, ISO6393: value.ISO6393,
        }
    }
    return result
}

func hasMultipleLanguages(values []language.Section) bool {
    seen := make(map[language.Language]struct{}, 2)
    for _, value := range values {
        if value.Language == language.Unknown { continue }
        seen[value.Language] = struct{}{}
        if len(seen) > 1 { return true }
    }
    return false
}
```

- [x] **Step 8: Focused test 실행 및 green 상태에서 refactor**

Run:

```bash
go test -count=1 ./examples/multilingual-language-routing/internal/routing
```

기대값: PASS. 그런 다음 두 파일에 `gofmt`를 실행하고 같은 명령을 다시 실행한다.

- [x] **Step 9: Commit Task 1**

```bash
git add examples/multilingual-language-routing/internal/routing/router.go \
  examples/multilingual-language-routing/internal/routing/router_test.go
git commit -m "feat: add multilingual language routing policy"
```

Exact focused test evidence를 포함한 Lore body field를 사용한다. 이후 task가 소유한 파일은 포함하지 않는다.

## Task 2: Preview, Lifecycle Equality, and Concurrent Reuse

**Complexity:** High. 이 작업은 fixture stability, preload equivalence, race proof를 소유한다.

**Required skills:** `test-driven-development`, `bluetape-go-patterns`.

**Files:**

- Create: `examples/multilingual-language-routing/internal/routing/preview.go`
- Modify: `examples/multilingual-language-routing/internal/routing/router_test.go`

- [x] **Step 1: 실패하는 preview 및 lifecycle 테스트 작성**

`NewPreview(false)`를 두 번, `NewPreview(true)`를 한 번 호출하는 테스트를 추가한다. 다음을 검증한다.

```go
lazyA, err := NewPreview(false)
if err != nil { t.Fatal(err) }
lazyB, err := NewPreview(false)
if err != nil { t.Fatal(err) }
preloaded, err := NewPreview(true)
if err != nil { t.Fatal(err) }
if !reflect.DeepEqual(lazyA, lazyB) { t.Fatalf("lazy previews differ") }
if lazyA.ModelLoading != "lazy" || preloaded.ModelLoading != "preloaded" {
    t.Fatalf("model loading = %q/%q", lazyA.ModelLoading, preloaded.ModelLoading)
}
if lazyA.Config.PreloadModels || !preloaded.Config.PreloadModels {
    t.Fatalf("preload config = %v/%v", lazyA.Config.PreloadModels, preloaded.Config.PreloadModels)
}
if !reflect.DeepEqual(lazyA.Decisions, preloaded.Decisions) {
    t.Fatalf("routing changed by model loading mode")
}
if !reflect.DeepEqual(lazyA.LowConfidenceFallback.Decision, preloaded.LowConfidenceFallback.Decision) {
    t.Fatalf("low-confidence routing changed by model loading mode")
}
if len(lazyA.Decisions) != 7 { t.Fatalf("len(Decisions) = %d", len(lazyA.Decisions)) }
if lazyA.LowConfidenceFallback.Config.MinimumConfidence != 1.0 ||
    !reflect.DeepEqual(lazyA.LowConfidenceFallback.Decision.ReviewReasons, []string{ReviewLowConfidence}) {
    t.Fatalf("LowConfidenceFallback = %#v", lazyA.LowConfidenceFallback)
}
```

또한 lifecycle notes가 construct-once/reuse와 lazy/preloaded tradeoff를 언급하는지 확인한다.
boundary notes는 heuristic, security/compliance, redaction/logging, 그리고 세 가지 public
evidence view를 모으는 의도적 비용을 언급해야 한다. 이 테스트는 in-process 비교가 startup
time, memory, model-cache 동작을 측정한다고 주장하면 안 된다. 검증 대상은 option wiring과 route
equivalence뿐이다.

- [x] **Step 2: preview 테스트 실행 및 RED 확인**

기대값: `Preview`와 `NewPreview`가 아직 없으므로 compile failure가 발생한다.

- [x] **Step 3: 고정 preview data 구현**

`preview.go`를 다음 내용으로 생성한다.

```go
type Preview struct {
    Scenario            string     `json:"scenario"`
    Config              Config     `json:"config"`
    ModelLoading        string     `json:"model_loading"`
    LifecycleNotes      []string   `json:"lifecycle_notes"`
    HeuristicBoundaries []string   `json:"heuristic_boundaries"`
    Decisions           []Decision `json:"decisions"`
    LowConfidenceFallback PolicyCheck `json:"low_confidence_fallback"`
    TestCommands        []string   `json:"test_commands"`
}

type PolicyCheck struct {
    Config   Config   `json:"config"`
    Decision Decision `json:"decision"`
}

func NewPreview(preload bool) (Preview, error) {
    config := DefaultConfig()
    config.PreloadModels = preload
    router, err := NewRouter(config)
    if err != nil { return Preview{}, err }
    requests := []Request{
        {ID: "preview-en", Text: "Please review the delivery status for order 12345."},
        {ID: "preview-ko", Text: "주문 번호 12345의 배송 상태를 확인해 주세요."},
        {ID: "preview-ja", Text: "配送状況を確認してください。注文番号は12345です。"},
        {ID: "preview-zh", Text: "请确认订单12345的配送状态。"},
        {ID: "preview-mixed", Text: "Please check 配送状況を確認してください for order 12345."},
        {ID: "preview-short", Text: "help"},
        {ID: "preview-unknown", Text: "12345 67890"},
    }
    decisions := make([]Decision, len(requests))
    for i, request := range requests {
        decisions[i], err = router.Route(request)
        if err != nil { return Preview{}, fmt.Errorf("route preview %q: %w", request.ID, err) }
    }
    lowConfig := config
    lowConfig.MinimumConfidence = 1.0
    lowRouter, err := NewRouter(lowConfig)
    if err != nil { return Preview{}, fmt.Errorf("create low-confidence policy router: %w", err) }
    lowDecision, err := lowRouter.Route(Request{ID: "preview-low", Text: "support 문의 订单 delivery"})
    if err != nil { return Preview{}, fmt.Errorf("route low-confidence preview: %w", err) }
    loading := "lazy"
    if preload { loading = "preloaded" }
    return Preview{
        Scenario: "route multilingual content while preserving heuristic uncertainty",
        Config: config, ModelLoading: loading, Decisions: decisions,
        LowConfidenceFallback: PolicyCheck{Config: lowConfig, Decision: lowDecision},
        LifecycleNotes: []string{
            "Construct one detector and reuse it across requests.",
            "Lazy loading defers model work; preloading moves it to construction without changing routes.",
            "The example gathers three public evidence views with repeated detector work and projection allocations; production policies should request only what they use.",
        },
        HeuristicBoundaries: []string{
            "Language evidence is heuristic and must not control authentication, authorization, sanctions, or compliance.",
            "Decisions retain original text for span inspection; callers own redaction, logging, telemetry, and access control.",
            "Unsupported or uncertain evidence routes to manual review rather than a processor.",
        },
        TestCommands: []string{
            "go test -count=1 ./examples/multilingual-language-routing/...",
            "go test -race -count=1 ./examples/multilingual-language-routing/...",
        },
    }, nil
}
```

- [x] **Step 4: preview 테스트 실행 및 GREEN 확인**

focused package test를 실행한다. 기대값: PASS, 그리고 model-loading mode 사이에서 decision이 정확히 동일하다.

- [x] **Step 5: 실패하는 bounded concurrency 테스트 작성**

round마다 여섯 개의 request를 사용하고 세 round를 실행한다. 아래 helper는 entry gate와 정확한 call arithmetic을 명시한다.

```go
func exerciseConcurrentRoutes(t *testing.T, router *Router, requests []Request) {
    t.Helper()
    expected := map[string]struct {
        route string
        reasons []string
    }{
        "en": {RouteModeration, []string{}},
        "ko": {RouteModeration, []string{}},
        "ja": {RouteJapaneseTokenization, []string{}},
        "zh": {RouteManualReview, []string{ReviewUnsupportedLanguage}},
        "mixed": {RouteManualReview, []string{ReviewMixedLanguage}},
        "unknown": {RouteManualReview, []string{ReviewLanguageUnknown}},
    }
    var completed, readyCount atomic.Int64
    errCh := make(chan error, 18)
    resultCh := make(chan Decision, 18)
    for round := 0; round < 3; round++ {
        var ready sync.WaitGroup
        ready.Add(len(requests))
        release := make(chan struct{})
        var roundDone sync.WaitGroup
        roundDone.Add(len(requests))
        for _, request := range requests {
            request := request
            go func() {
                defer roundDone.Done()
                readyCount.Add(1)
                ready.Done()
                <-release
                decision, err := router.Route(request)
                if err != nil { errCh <- err; return }
                resultCh <- decision
                completed.Add(1)
            }()
        }
        ready.Wait()
        if got := readyCount.Load(); got != int64((round+1)*len(requests)) {
            t.Fatalf("ready after round %d = %d", round, got)
        }
        close(release)
        roundDone.Wait()
    }
    close(errCh)
    close(resultCh)
    for err := range errCh { t.Errorf("Route error: %v", err) }
    first := make(map[string]Decision, len(requests))
    counts := make(map[string]int, len(requests))
    for decision := range resultCh {
        contract, ok := expected[decision.ID]
        if !ok || decision.Route != contract.route || !reflect.DeepEqual(decision.ReviewReasons, contract.reasons) {
            t.Errorf("Decision[%s] = %#v, want route=%q reasons=%v", decision.ID, decision, contract.route, contract.reasons)
        }
        if previous, exists := first[decision.ID]; exists && !reflect.DeepEqual(decision, previous) {
            t.Errorf("Decision[%s] changed across rounds: %#v != %#v", decision.ID, decision, previous)
        } else if !exists {
            first[decision.ID] = decision
        }
        counts[decision.ID]++
    }
    if completed.Load() != 18 { t.Fatalf("completed = %d", completed.Load()) }
    for id := range expected {
        if counts[id] != 3 { t.Fatalf("counts[%s] = %d", id, counts[id]) }
    }
}

func TestRouterConcurrentFirstUse(t *testing.T) {
    requests := []Request{
        {ID: "en", Text: "Please review the delivery status for order 12345."},
        {ID: "ko", Text: "주문 번호 12345의 배송 상태를 확인해 주세요."},
        {ID: "ja", Text: "配送状況を確認してください。注文番号は12345です。"},
        {ID: "zh", Text: "请确认订单12345的配送状态。"},
        {ID: "mixed", Text: "Please check 配送状況を確認してください for order 12345."},
        {ID: "unknown", Text: "12345 67890"},
    }
    for _, preload := range []bool{false, true} {
        t.Run(fmt.Sprintf("preload=%t", preload), func(t *testing.T) {
            config := DefaultConfig(); config.PreloadModels = preload
            router, err := NewRouter(config); if err != nil { t.Fatal(err) }
            exerciseConcurrentRoutes(t, router, requests)
        })
    }
    t.Run("GOMAXPROCS=1", func(t *testing.T) {
        previous := runtime.GOMAXPROCS(1)
        t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
        router, err := NewRouter(DefaultConfig()); if err != nil { t.Fatal(err) }
        exerciseConcurrentRoutes(t, router, requests)
    })
}
```

- [x] **Step 6: normal 및 race 테스트 실행**

```bash
go test -count=1 -run '^TestRouterConcurrentFirstUse$' ./examples/multilingual-language-routing/internal/routing
go test -race -count=1 -run '^TestRouterConcurrentFirstUse$' ./examples/multilingual-language-routing/internal/routing
go test -count=1 ./examples/multilingual-language-routing/internal/routing
go test -race -count=1 ./examples/multilingual-language-routing/internal/routing
```

기대값: isolated first-use command와 두 full package command가 모두 PASS한다.
각 concurrency subtest는 round마다 준비된 goroutine 여섯 개를 정확히 release하고,
request마다 동일한 decision 세 개와 총 decision 18개를 반환하며, race report를 만들지 않는다.
이 테스트는 upstream detector 내부의 instrumented overlap에 대해 어떤 주장도 하지 않는다.

- [x] **Step 7: Task 2 commit**

```bash
git add examples/multilingual-language-routing/internal/routing/preview.go \
  examples/multilingual-language-routing/internal/routing/router_test.go
git commit -m "test: prove language router lifecycle reuse"
```

normal command와 race command를 모두 Lore body에 기록한다.

## Task 3: Deterministic CLI and Preload Flag

**Complexity:** Medium. 이 작업은 process/flag/output 동작만 소유한다.

**Required skills:** `test-driven-development`, `bluetape-go-patterns`.

**Files:**

- Create: `examples/multilingual-language-routing/main.go`
- Create: `examples/multilingual-language-routing/main_test.go`

- [x] **Step 1: 실패하는 CLI 테스트 작성**

`run(args, stdout, stderr)`를 직접 테스트한다.

```go
func TestRunIsDeterministic(t *testing.T) {
    var first, second, stderr bytes.Buffer
    if code := run(nil, &first, &stderr); code != 0 { t.Fatalf("code=%d stderr=%q", code, stderr.String()) }
    stderr.Reset()
    if code := run(nil, &second, &stderr); code != 0 { t.Fatalf("code=%d stderr=%q", code, stderr.String()) }
    if first.String() != second.String() { t.Fatalf("stdout differs") }
}

func TestRunPreloadChangesOnlyLifecycleMetadata(t *testing.T) {
    var lazyOut, preloadOut, stderr bytes.Buffer
    if code := run(nil, &lazyOut, &stderr); code != 0 { t.Fatal(stderr.String()) }
    stderr.Reset()
    if code := run([]string{"--preload"}, &preloadOut, &stderr); code != 0 { t.Fatal(stderr.String()) }
    var lazy, preloaded routing.Preview
    if err := json.Unmarshal(lazyOut.Bytes(), &lazy); err != nil { t.Fatal(err) }
    if err := json.Unmarshal(preloadOut.Bytes(), &preloaded); err != nil { t.Fatal(err) }
    if !reflect.DeepEqual(lazy.Decisions, preloaded.Decisions) { t.Fatalf("decisions differ") }
    if !reflect.DeepEqual(lazy.LowConfidenceFallback.Decision, preloaded.LowConfidenceFallback.Decision) { t.Fatalf("low-confidence decisions differ") }
    if lazy.ModelLoading != "lazy" || preloaded.ModelLoading != "preloaded" { t.Fatalf("loading differs incorrectly") }
}

func TestRunFlagErrorsAndHelp(t *testing.T) {
    var stdout, stderr bytes.Buffer
    if code := run([]string{"--unknown"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
        t.Fatalf("unknown flag code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
    }
    stdout.Reset(); stderr.Reset()
    if code := run([]string{"--help"}, &stdout, &stderr); code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--preload") {
        t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
    }
}

func TestRunQuotesUnexpectedArgument(t *testing.T) {
    var stdout, stderr bytes.Buffer
    if code := run([]string{"bad\n\x1b[31m"}, &stdout, &stderr); code != 2 {
        t.Fatalf("code = %d", code)
    }
    if stdout.Len() != 0 {
        t.Fatalf("stdout = %q", stdout.String())
    }
    want := "unexpected argument: \"bad\\n\\x1b[31m\"\n"
    if stderr.String() != want || strings.Contains(stderr.String(), "\x1b") {
        t.Fatalf("stderr = %q, want %q", stderr.String(), want)
    }
}
```

- [x] **Step 2: CLI 테스트 실행 및 RED 확인**

기대값: `run`과 `main.go`가 아직 없으므로 compile failure가 발생한다.

- [x] **Step 3: CLI seam 구현**

`main.go`를 생성한다.

```go
package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"

    "github.com/bluetape4k/bluetape-go-workshop/examples/multilingual-language-routing/internal/routing"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
    flags := flag.NewFlagSet("multilingual-language-routing", flag.ContinueOnError)
    flags.SetOutput(stderr)
    preload := flags.Bool("preload", false, "preload selected language models during router construction")
    if err := flags.Parse(args); err != nil {
        if errors.Is(err, flag.ErrHelp) { return 0 }
        return 2
    }
    if flags.NArg() != 0 {
        fmt.Fprintf(stderr, "unexpected argument: %q\n", flags.Arg(0))
        return 2
    }
    preview, err := routing.NewPreview(*preload)
    if err != nil {
        fmt.Fprintf(stderr, "build multilingual language routing preview: %v\n", err)
        return 1
    }
    encoder := json.NewEncoder(stdout)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(preview); err != nil {
        fmt.Fprintf(stderr, "encode multilingual language routing preview: %v\n", err)
        return 1
    }
    return 0
}
```

- [x] **Step 4: CLI 및 package proof 실행**

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

기대값: test가 PASS한다. 두 command는 exit 0으로 끝나고 유효한 indented JSON을 출력한다.
decode된 decision은 서로 같으며, model-loading/config metadata는 mode를 반영한다.

- [x] **Step 5: Task 3 commit**

```bash
git add examples/multilingual-language-routing/main.go \
  examples/multilingual-language-routing/main_test.go
git commit -m "feat: add language routing preview command"
```

## Task 4: Bilingual Lesson and Root Navigation

**Complexity:** Medium. This task must match real output and preserve locale parity.

**Required skills:** `bluetape-writer`; `bluetape-diagram` is N/A because the
approved spec selects a compact routing table and linear prose instead of an
asset.

**Files:**

- Create: `examples/multilingual-language-routing/README.md`
- Create: `examples/multilingual-language-routing/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [x] **Step 1: Capture actual deterministic output**

Run both `go run` commands from Task 3. Select representative JSON directly
from stdout for English/Korean shared moderation, Japanese tokenization,
Chinese unsupported review, mixed ordered reasons, and the separately labeled
threshold `1.0` low-confidence fallback. Do not
hand-invent numeric confidences or byte offsets.

- [x] **Step 2: Write the English README**

Use title `# multilingual-language-routing`, then `English | [한국어](README.ko.md)`.
Include these exact sections:

1. Package Lesson: detector evidence versus application route policy.
2. Routing Matrix: the six spec rows and exact machine values.
3. Evidence Contract: four descending confidences, ISO codes, Latin/Hangul/Kana/Han hints, mixed section UTF-8 byte spans.
4. Run: both lazy and `--preload` commands plus actual output.
5. Test: focused normal/race commands and exact 18-call bounded contract.
6. Lifecycle and Cost: construct once/reuse, qualitative preload tradeoff,
   three public evidence views with repeated work/allocation, production
   gather-only-needed guidance, and no in-process startup/memory claim.
7. Boundaries: no processor execution, no security/compliance decision, original-text redaction/logging/access-control duty, no certainty claim.

- [x] **Step 3: Write the Korean README with source-equivalent meaning**

Use title `# multilingual-language-routing`, then `[English](README.md) | 한국어`.
Preserve every command, route/reason value, code symbol, numeric threshold, and
boundary from the English README. Use natural Korean technical prose; do not
abbreviate lifecycle, security, or privacy guidance.

- [x] **Step 4: Update root README navigation in milestone order**

Insert `examples/multilingual-language-routing` immediately after
`examples/japanese-search-preparation` in both root tables and run sections.
Use English public prose in `README.md` and source-equivalent Korean in
`README.ko.md`. Add both run commands and both focused test commands without
reordering existing examples.

- [x] **Step 5: Verify documentation against the program**

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
git diff --check
```

Compare README snippets to stdout field-for-field. Search both locales for all
route values, all six review reasons, `0.70`, `8`, `--preload`, `UTF-8`,
`authentication`, `authorization`, `compliance`, and redaction/logging wording.

- [x] **Step 6: Commit Task 4**

```bash
git add README.md README.ko.md \
  examples/multilingual-language-routing/README.md \
  examples/multilingual-language-routing/README.ko.md
git commit -m "docs: explain multilingual language routing"
```

## Task 5: Verification, Review, Lesson, and Delivery

**Complexity:** High workflow gate; no feature behavior should be added here.

**Required skills:** `verification-before-completion`, Type A verifier and six
code-review perspectives, then `finishing-a-development-branch` for delivery.

**Files:**

- Create: `docs/lessons/2026-07-12-issue-119-multilingual-language-routing.md`
- Optionally create only when useful: `docs/review/2026-07-12-issue-119-multilingual-language-routing-review.md`

- [x] **Step 1: Run focused verification from scratch**

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Every command must return an observed exit code 0. Lost process handles or
PASS-looking output without an exit code are invalid and must be rerun.

- [x] **Step 2: Run repository gates in authoritative order**

Run targeted cheap checks first, then the single authoritative gate:

```bash
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check origin/develop...HEAD
```

`make ci` owns the final full test/race proof. Do not run heavyweight commands
in parallel across agents or worktrees. A fail-then-pass requires root-cause
investigation before acceptance.

- [x] **Step 3: Audit exact diff and conditional hazards**

Confirm only the planned example, README pair, spec/plan, `.gitignore`, and
lesson/review artifacts changed. Record concrete N/A evidence:

- new Go module/registration: N/A, this is a package under the existing module;
- dependencies/catalog: N/A, `go.mod` and `go.sum` unchanged;
- workflow/Nightly/coverage: N/A, existing `go test ./...` discovery covers it;
- Docker/Testcontainers/database/HTTP: N/A, none added;
- public bluetape-go API/CHANGELOG: N/A, workshop-internal package only;
- diagram asset: N/A, approved small table/linear flow is clearer.

- [x] **Step 4: Run spec/plan verifier and pre-PR review**

The verifier maps every spec acceptance row and every plan checkbox to current
files and fresh commands. Then run performance, stability, security,
operator/Ops, developer/API, and user/caller code-review lenses plus main
integration. P0/P1 blocks delivery; fix, rerun focused proof, and rerun only
affected lenses. Resolve or justify every P2/P3.

- [x] **Step 5: Write and commit the lesson**

Write concise context, decision, surprising detector/fixture or concurrency
evidence, outcome, exact verification commands, review misses, and future guard.
Commit it before PR creation:

```bash
git add docs/lessons/2026-07-12-issue-119-multilingual-language-routing.md
git commit -m "docs: record language routing lessons"
```

- [ ] **Step 6: Push and create the PR**

Push the feature branch. Create an English PR assigned to `debop`, milestone
`0.8.0`, labels `enhancement` and `examples`, resolving #119. Use the repository
PR template, explain why/what before validation, and end with `## DoD Status`.
Verify live title, body, base/head, assignee, milestone, labels, and issue link.

- [ ] **Step 7: Converge live PR review and CI**

Run the post-PR review against the actual PR diff. Monitor required checks until
all conclude SUCCESS. After green, reread reviews and unresolved threads; any
new feedback reopens implementation/review. Update the final PR DoD status with
fresh evidence.

- [ ] **Step 8: Merge, update umbrella issue, sync, and clean**

Under the user's approved delivery scope, use the workspace-default rebase merge
after CI/review convergence. Verify #119 closed. Update umbrella #34
to check #119 only after live closure. Sync the real local `develop` checkout,
verify its SHA equals `origin/develop`, then remove the integrated worktree and
local feature branch only after ancestry or patch-equivalence is proven.

- [ ] **Step 9: Final DoD report**

Report every A-01 through A-11 and CG-01 through CG-17 row with evidence or
concrete N/A, P0/P1 convergence, focused/full commands, PR/CI/review/merge
state, issue metadata, local/upstream SHA equality, and clean worktree list.
Required format: `Required checks: X/Y; N/A: N; Blocked: 0`.

## Plan Review Gate

Before Task 1, review this plan through performance, stability, security,
operator/Ops, developer/API, and user/caller lenses plus main integration.
Verify every spec row maps to an earlier-producing task and exact command, no
task needs a later file, and all triggered risks have rerun points. Close only
at P0=0/P1=0 and commit the reviewed plan before code.

## Plan Review Results

Native read-only performance, security, and developer/API lanes returned
evidence-backed findings. Stability/Ops and user/caller lanes did not respond
after bounded waits and immediate-return requests, so the main session applied
the model-routing timeout fallback for those perspectives and operator/Ops.

| Priority | Lens | Evidence | Resolution |
|---|---|---|---|
| P1 | Performance | The original concurrency helper computed expectations with the same lazy router before concurrent calls. | Removed all target-router prewarming; a focused process invokes a fresh lazy router first and validates hard-coded route/reason contracts plus cross-round equality. |
| P1 | Performance | `maxActive` measured goroutines parked before the release gate, not overlap inside `Router.Route`. | Removed the metric and overlap claim; assert six ready participants, exact 18 outcomes, three identical results per request, and race cleanliness. |
| P1 | Developer/API | `<0 || >1` accepts `math.NaN()`. | Added a RED NaN configuration case and `math.IsNaN` rejection. |
| P1 | Developer/API | Plan used `Unknown` while the approved machine value is `unknown`. | Changed the fixture and implementation to lowercase and asserted empty ISO codes. |
| P2 | Performance | “Three detector passes” understated the work behind three public APIs. | Renamed these three public evidence views and documented repeated detector work/projection allocation with no throughput claim. |
| P2 | Performance/Ops | In-process lazy/preloaded comparison cannot prove startup time or memory behavior. | Scoped proof to option wiring and behavioral equality; lifecycle tradeoffs remain qualitative upstream guidance. |
| P2 | Developer/API | The stale spec promised overlap assertions after the concurrency contract changed. | Updated the spec to ready-gate, exact-result, determinism, and race proof. |
| P2 | Developer/API | Positional-argument rejection was tested after its implementation step. | Moved the escaped positional-argument test into the initial RED CLI suite. |
| P3 | Security | `%v` could echo newline/control characters into diagnostics. | Changed to `%q` and added an exact escaped stderr test with empty stdout. |
| P3 | Developer/API | The `unicode/utf8` import was implicit. | Added the exact import instruction to the implementation step. |

Main integration confirmed task ordering, spec traceability, bilingual parity,
fail-closed routing, lifecycle/rollback evidence, main-session commit ownership,
and rebase merge/sync boundaries. Latest convergence: P0=0, P1=0. All listed
P2/P3 findings are resolved in this plan.

## Execution Stop Conditions

- Stop if the pinned low-confidence fixture is not detected as Korean below the
  explicit `1.0` threshold or gains an unapproved reason under v0.18.0.
- Stop if a task needs a new dependency, module, workflow, HTTP integration, or
  reusable library abstraction; that changes the approved scope.
- Stop if `go.mod`, `go.sum`, workflow, Nightly, or unrelated example files
  change.
- Stop on any P0/P1 review finding or unexplained fail-then-pass test.
- Stop before destructive cleanup unless live merge integration is proven.
