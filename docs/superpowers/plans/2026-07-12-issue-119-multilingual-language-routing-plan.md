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

**Complexity:** Medium. 이 작업은 실제 output과 맞아야 하며 locale parity를 보존해야 한다.

**Required skills:** `bluetape-writer`; approved spec이 asset 대신 compact routing table과 linear prose를 선택했으므로
`bluetape-diagram`은 N/A이다.

**Files:**

- Create: `examples/multilingual-language-routing/README.md`
- Create: `examples/multilingual-language-routing/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [x] **Step 1: 실제 deterministic output 캡처**

Task 3의 `go run` command 두 개를 모두 실행한다. English/Korean shared moderation,
Japanese tokenization, Chinese unsupported review, mixed ordered reasons, 별도로 표시된 threshold `1.0`
low-confidence fallback에 사용할 representative JSON은 stdout에서 직접 선택한다.
numeric confidence나 byte offset을 손으로 만들어내면 안 된다.

- [x] **Step 2: English README 작성**

title은 `# multilingual-language-routing`를 사용하고, 바로 다음에 `English | [한국어](README.ko.md)`를 둔다.
다음 section을 정확히 포함한다.

1. Package Lesson: detector evidence와 application route policy의 차이.
2. Routing Matrix: 여섯 spec row와 정확한 machine value.
3. Evidence Contract: confidence 네 개의 내림차순, ISO code, Latin/Hangul/Kana/Han hint, mixed section UTF-8 byte span.
4. Run: lazy와 `--preload` command 및 실제 output.
5. Test: focused normal/race command와 정확한 18-call bounded contract.
6. Lifecycle and Cost: construct once/reuse, 정성적 preload tradeoff, repeated work/allocation을 수반하는 세 public evidence view,
   production gather-only-needed guidance, 그리고 in-process startup/memory claim 없음.
7. Boundaries: processor execution 없음, security/compliance decision 없음, original-text redaction/logging/access-control 책임, certainty claim 없음.

- [x] **Step 3: source-equivalent 의미를 가진 Korean README 작성**

title은 `# multilingual-language-routing`를 사용하고, 바로 다음에 `[English](README.md) | 한국어`를 둔다.
English README의 모든 command, route/reason value, code symbol, numeric threshold, boundary를 보존한다.
자연스러운 한국어 기술 문장으로 쓰되 lifecycle, security, privacy guidance를 축약하지 않는다.

- [x] **Step 4: milestone 순서에 맞춰 root README navigation 업데이트**

두 root table과 run section에서 `examples/multilingual-language-routing`를
`examples/japanese-search-preparation` 바로 뒤에 삽입한다. `README.md`에는 English public prose를,
`README.ko.md`에는 source-equivalent Korean을 사용한다. 기존 example 순서를 바꾸지 말고 두 run command와
두 focused test command를 모두 추가한다.

- [x] **Step 5: program 기준으로 documentation 검증**

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
git diff --check
```

README snippet을 stdout과 field-by-field로 비교한다. 두 locale에서 모든 route value, 여섯 review reason,
`0.70`, `8`, `--preload`, `UTF-8`, `authentication`, `authorization`, `compliance`,
그리고 redaction/logging 문구를 검색한다.

- [x] **Step 6: Task 4 commit**

```bash
git add README.md README.ko.md \
  examples/multilingual-language-routing/README.md \
  examples/multilingual-language-routing/README.ko.md
git commit -m "docs: explain multilingual language routing"
```

## Task 5: Verification, Review, Lesson, and Delivery

**Complexity:** High workflow gate. 여기서는 feature behavior를 추가하면 안 된다.

**Required skills:** `verification-before-completion`, Type A verifier와 여섯 code-review perspective,
그리고 delivery를 위한 `finishing-a-development-branch`.

**Files:**

- Create: `docs/lessons/2026-07-12-issue-119-multilingual-language-routing.md`
- Optionally create only when useful: `docs/review/2026-07-12-issue-119-multilingual-language-routing-review.md`

- [x] **Step 1: focused verification을 처음부터 실행**

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

모든 command는 관찰된 exit code 0을 반환해야 한다. process handle을 잃었거나 exit code 없이 PASS처럼 보이는 output은
유효하지 않으므로 다시 실행해야 한다.

- [x] **Step 2: authoritative order로 repository gate 실행**

targeted cheap check를 먼저 실행한 뒤, 단일 authoritative gate를 실행한다.

```bash
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check origin/develop...HEAD
```

`make ci`가 최종 full test/race proof를 소유한다. heavyweight command를 여러 agent나 worktree에서 병렬 실행하지 않는다.
fail-then-pass가 발생하면 수락하기 전에 root-cause investigation이 필요하다.

- [x] **Step 3: exact diff 및 conditional hazard audit**

planned example, README pair, spec/plan, `.gitignore`, lesson/review artifact만 변경되었는지 확인한다.
구체적인 N/A evidence를 기록한다.

- new Go module/registration: N/A, existing module 아래의 package이다.
- dependencies/catalog: N/A, `go.mod`와 `go.sum`이 변경되지 않는다.
- workflow/Nightly/coverage: N/A, 기존 `go test ./...` discovery가 이를 포함한다.
- Docker/Testcontainers/database/HTTP: N/A, 추가하지 않는다.
- public bluetape-go API/CHANGELOG: N/A, workshop-internal package만 다룬다.
- diagram asset: N/A, 승인된 small table/linear flow가 더 명확하다.

- [x] **Step 4: spec/plan verifier 및 pre-PR review 실행**

verifier는 모든 spec acceptance row와 모든 plan checkbox를 현재 file 및 fresh command에 매핑한다.
그다음 performance, stability, security, operator/Ops, developer/API, user/caller code-review lens와
main integration을 실행한다. P0/P1은 delivery를 막는다. 수정 후 focused proof를 다시 실행하고, 영향받은 lens만 다시 실행한다.
모든 P2/P3는 해결하거나 정당화한다.

- [x] **Step 5: lesson 작성 및 commit**

간결한 context, decision, 예상 밖의 detector/fixture 또는 concurrency evidence, outcome, 정확한 verification command,
review miss, future guard를 작성한다. PR 생성 전에 commit한다.

```bash
git add docs/lessons/2026-07-12-issue-119-multilingual-language-routing.md
git commit -m "docs: record language routing lessons"
```

- [ ] **Step 6: push 및 PR 생성**

feature branch를 push한다. #119를 해결하는 English PR을 생성하고 `debop`에게 assign하며 milestone `0.8.0`,
label `enhancement`, `examples`를 지정한다. repository PR template을 사용하고 validation보다 앞에서 why/what을 설명하며
`## DoD Status`로 끝낸다. live title, body, base/head, assignee, milestone, label, issue link를 확인한다.

- [ ] **Step 7: live PR review 및 CI 수렴**

actual PR diff를 대상으로 post-PR review를 실행한다. required check가 모두 SUCCESS로 끝날 때까지 모니터링한다.
green 이후 review와 unresolved thread를 다시 읽는다. 새 feedback은 implementation/review를 다시 연다.
fresh evidence로 최종 PR DoD status를 업데이트한다.

- [ ] **Step 8: merge, umbrella issue 업데이트, sync, cleanup**

사용자가 승인한 delivery scope 안에서 CI/review 수렴 후 workspace-default rebase merge를 사용한다.
#119가 닫혔는지 확인한다. live closure 이후에만 umbrella #34에서 #119를 check한다.
실제 local `develop` checkout을 sync하고 SHA가 `origin/develop`와 같은지 확인한다.
그다음 ancestry 또는 patch-equivalence가 증명된 뒤 integrated worktree와 local feature branch를 제거한다.

- [ ] **Step 9: 최종 DoD 보고**

A-01부터 A-11, CG-01부터 CG-17까지 모든 row를 evidence 또는 구체적인 N/A와 함께 보고한다.
P0/P1 convergence, focused/full command, PR/CI/review/merge state, issue metadata,
local/upstream SHA equality, clean worktree list를 포함한다.
필수 format: `Required checks: X/Y; N/A: N; Blocked: 0`.

## Plan Review Gate

Task 1 전에 performance, stability, security, operator/Ops, developer/API, user/caller lens와
main integration으로 이 plan을 review한다. 모든 spec row가 더 앞에서 산출되는 task 및 정확한 command에 매핑되는지,
나중 file이 필요한 task가 없는지, trigger된 모든 risk에 rerun point가 있는지 확인한다.
P0=0/P1=0일 때만 닫고, code 전에 reviewed plan을 commit한다.

## Plan Review Results

native read-only performance, security, developer/API lane은 evidence-backed finding을 반환했다.
Stability/Ops와 user/caller lane은 bounded wait와 immediate-return request 이후에도 응답하지 않았다.
따라서 main session은 해당 perspective와 operator/Ops에 model-routing timeout fallback을 적용했다.

| Priority | Lens | Evidence | Resolution |
|---|---|---|---|
| P1 | Performance | 기존 concurrency helper는 concurrent call 전에 같은 lazy router로 expectation을 계산했다. | 모든 target-router prewarming을 제거했다. focused process가 fresh lazy router를 먼저 호출하고 hard-coded route/reason contract와 cross-round equality를 검증한다. |
| P1 | Performance | `maxActive`는 `Router.Route` 내부 overlap이 아니라 release gate 전에 parked된 goroutine을 측정했다. | metric과 overlap claim을 제거했다. 준비된 participant 여섯 개, 정확한 outcome 18개, request별 동일 result 세 개, race cleanliness를 assert한다. |
| P1 | Developer/API | `<0 || >1`은 `math.NaN()`을 허용한다. | RED NaN configuration case와 `math.IsNaN` rejection을 추가했다. |
| P1 | Developer/API | plan은 approved machine value가 `unknown`인데 `Unknown`을 사용했다. | fixture와 implementation을 lowercase로 바꾸고 empty ISO code를 assert했다. |
| P2 | Performance | “Three detector passes”는 세 public API 뒤의 작업량을 과소표현했다. | 이를 세 public evidence view로 이름 바꾸고 throughput claim 없이 repeated detector work/projection allocation을 문서화했다. |
| P2 | Performance/Ops | in-process lazy/preloaded 비교는 startup time이나 memory behavior를 증명할 수 없다. | proof를 option wiring과 behavioral equality로 제한했다. lifecycle tradeoff는 정성적 upstream guidance로 유지한다. |
| P2 | Developer/API | stale spec은 concurrency contract 변경 뒤에도 overlap assertion을 약속했다. | spec을 ready-gate, exact-result, determinism, race proof로 업데이트했다. |
| P2 | Developer/API | positional-argument rejection은 implementation step 뒤에서야 테스트됐다. | escaped positional-argument test를 initial RED CLI suite로 옮겼다. |
| P3 | Security | `%v`는 diagnostic에 newline/control character를 그대로 echo할 수 있다. | `%q`로 바꾸고 empty stdout을 포함한 exact escaped stderr test를 추가했다. |
| P3 | Developer/API | `unicode/utf8` import가 암묵적이었다. | implementation step에 정확한 import instruction을 추가했다. |

main integration은 task ordering, spec traceability, bilingual parity, fail-closed routing,
lifecycle/rollback evidence, main-session commit ownership, rebase merge/sync boundary를 확인했다.
최신 convergence: P0=0, P1=0. 나열된 모든 P2/P3 finding은 이 plan에서 해결됐다.

## Execution Stop Conditions

- pinned low-confidence fixture가 v0.18.0에서 explicit `1.0` threshold 아래의 Korean으로 detect되지 않거나
  승인되지 않은 reason을 얻으면 중단한다.
- task에 new dependency, module, workflow, HTTP integration, reusable library abstraction이 필요하면 중단한다.
  이는 approved scope를 변경한다.
- `go.mod`, `go.sum`, workflow, Nightly, unrelated example file이 변경되면 중단한다.
- P0/P1 review finding 또는 설명되지 않은 fail-then-pass test가 있으면 중단한다.
- live merge integration이 증명되지 않았다면 destructive cleanup 전에 중단한다.
