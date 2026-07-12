# Issue #119 Multilingual Language Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a deterministic CLI example that routes bluetape-go v0.18.0 language evidence to moderation, Japanese tokenization, or manual review while comparing lazy and preloaded detector lifecycle.

**Architecture:** A framework-independent `internal/routing.Router` owns one reusable four-language detector and creates call-local evidence and decisions. A fixed-fixture preview and a thin testable CLI expose identical route decisions in lazy and preloaded modes; bilingual READMEs explain the policy and its security/privacy boundary.

**Tech Stack:** Go 1.26.3, `github.com/bluetape4k/bluetape-go/textsearch/language` v0.18.0, standard `flag`, `encoding/json`, `sync`, and `sync/atomic` packages.

---

## Approved Inputs and Execution Contract

- Spec: `docs/superpowers/specs/2026-07-12-issue-119-multilingual-language-routing-design.md`
- Issue: GitHub #119, milestone `0.8.0`, labels `enhancement` and `examples`, assignee `debop`
- Base: `origin/develop` at `3f68aea7eeb42c45f09bd927f52a451f164ad8bb`
- Worktree: `.worktrees/feat-issue-119-multilingual-language-routing`
- Branch: `feat/issue-119-multilingual-language-routing`
- Dependency baseline: `github.com/bluetape4k/bluetape-go v0.18.0`
- Implementation workflow: after this plan is approved, use `subagent-driven-development` with a fresh `executor` for each disjoint write task, then a `verifier` and `code-reviewer` before the next task.
- Executors stop with an uncommitted scoped diff. The main session integrates the diff, applies review repairs, reruns proof, and owns every commit and external effect. PR creation, CI monitoring, rebase merge, issue update, local sync, and worktree cleanup follow the user's already approved delivery scope.

## File Map and Ownership

| File | Responsibility | Write task |
|---|---|---|
| `examples/multilingual-language-routing/internal/routing/router.go` | Types, config, shared detector, evidence projection, route matrix | Task 1 only |
| `examples/multilingual-language-routing/internal/routing/router_test.go` | Core, lifecycle, fixture, concurrency, and race contracts | Tasks 1-2, disjoint test sections |
| `examples/multilingual-language-routing/internal/routing/preview.go` | Fixed fixtures and deterministic preview metadata | Task 2 only |
| `examples/multilingual-language-routing/main.go` | Testable flag parsing and deterministic JSON output | Task 3 only |
| `examples/multilingual-language-routing/main_test.go` | CLI help/error/determinism/mode-equivalence tests | Task 3 only |
| `examples/multilingual-language-routing/README.md` | English lesson and actual output | Task 4 only |
| `examples/multilingual-language-routing/README.ko.md` | Source-equivalent natural Korean lesson | Task 4 only |
| `README.md`, `README.ko.md` | Root example navigation and run commands | Task 4 only |
| `docs/lessons/2026-07-12-issue-119-multilingual-language-routing.md` | Durable outcome, evidence, misses, future guard | Task 5 only |

No `go.mod`, `go.sum`, workflow, Nightly, Docker, Testcontainers, coverage,
catalog, or AGENTS file should change. Any such diff is a stop condition.

## Spec-to-Task Traceability

| Acceptance criterion | Task and proof |
|---|---|
| Four-language subset and reusable detector | Task 1 constructor tests and package inspection |
| English/Korean common moderation route | Task 1 route matrix table tests |
| Kana-backed Japanese route and Han-only fail-closed behavior | Task 1 Japanese/Chinese tests |
| Confidence list, ISO codes, sections, script hints | Task 1 evidence tests and byte slicing |
| Short/unknown/low-confidence/mixed ordered review reasons | Task 1 exact table assertions |
| Lazy/preloaded lifecycle equality | Task 2 preview comparison |
| Shared detector bounded race proof | Task 2 exact 18-call concurrency test plus `go test -race` |
| Deterministic CLI and `--preload` | Task 3 repeated byte comparison and decoded decision comparison |
| Bilingual lesson and root navigation | Task 4 locale parity review and run-output evidence |
| Heuristic/security/privacy boundary | Tasks 2 and 4 preview notes and README boundary sections |
| Repository quality | Task 5 `make ci`, diff audit, verifier and review convergence |

## Risk Prediction

| Risk | Signal | Mitigation | Rerun/rollback point |
|---|---|---|---|
| Lazy first-use model state races | `go test -race` report or inconsistent decisions | Construct once; keep results call-local; bounded shared-router test in both modes | Return to Task 1/2 and rerun focused normal/race from the beginning |
| Han-only input reaches Japanese processing | Route is `japanese-tokenization` without `kana` hint | Japanese route requires detected Japanese plus Kana; exact negative fixtures | Revert route-matrix edit and rerun all Task 1 tables |
| Detector upgrade changes low-confidence fixture | Candidate is no longer detected below the explicit `1.0` threshold | Keep default `0.70`; isolate fixture `support 문의 订单 delivery` in a threshold `1.0` policy check and fail intentionally | Re-evaluate fixture/policy in spec; never weaken assertion silently |
| Mixed section offsets corrupt UTF-8 slicing | `decision.Text[start:end] != section.Text` | Preserve upstream byte spans unchanged and assert every section | Return to evidence projection and rerun focused tests/race |
| CLI modes differ beyond lifecycle metadata | Decoded decisions differ or stdout is nondeterministic | One constructor/preview path parameterized only by preload option | Return to Task 2/3; compare decoded payloads before docs |
| Original text is copied to logs | README/preview omit privacy boundary | Fixed non-sensitive CLI fixtures; explicit caller redaction/access-control note | Block Task 4 completion and PR until both locales match |
| Three detector passes are treated as throughput guidance | README claims performance or omits cost | Explain this is inspectable teaching evidence and production should gather only needed views | Block docs review; no benchmark numbers may be added |

## Task 1: Route Policy and Evidence Contract

**Complexity:** High. This task owns the complete behavior matrix and upstream error preservation.

**Required skills:** `test-driven-development`, `bluetape-go-patterns`.

**Files:**

- Create: `examples/multilingual-language-routing/internal/routing/router.go`
- Create: `examples/multilingual-language-routing/internal/routing/router_test.go`

- [ ] **Step 1: Write failing configuration and input tests**

Create `router_test.go` in package `routing` with exact table assertions:
Its import block includes `errors`, `math`, `reflect`, `slices`, `strings`,
`testing`, and the bluetape-go `language` package used below.

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

- [ ] **Step 2: Run tests and observe the RED state**

Run:

```bash
go test -count=1 ./examples/multilingual-language-routing/internal/routing
```

Expected: FAIL to compile because `Config`, `DefaultConfig`, `NewRouter`,
`Router`, `Request`, `ErrInvalidConfig`, and `ErrInvalidRequest` do not exist.

- [ ] **Step 3: Add the minimal configuration, types, and constructor**

Create `router.go` with package documentation, English GoDoc, and these exact
contracts:

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

Add the initial `Route` validation path so Step 1 compiles and passes; return a
non-nil empty-slice decision after validation until the behavior tests are
added:

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

- [ ] **Step 4: Run configuration/input tests and observe GREEN**

Run the focused package test. Expected: PASS with no race or compile failure.

- [ ] **Step 5: Write failing route and evidence table tests**

Add fixtures with exact expected routes and reasons:

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

Before locking the explicit low-confidence expectation, confirm the pinned
v0.18.0 fixture is detected as Korean below `1.0`, while its Korean plus
`Unknown` sections do not create `mixed-language`. If the live pinned dependency
does not match, stop and reopen the spec rather than changing the threshold or
weakening the test.

- [ ] **Step 6: Run route tests and observe the RED state**

Run the focused package test. Expected: FAIL because `Route` does not project
confidence/sections/hints or apply the route matrix.

- [ ] **Step 7: Implement the complete route decision**

Add `"unicode/utf8"` to the `router.go` import block. Replace the provisional
`Route` body after validation with:

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

Implement `scriptHints`, `projectConfidences`, `projectSections`, and
`hasMultipleLanguages` as pure helpers. `scriptHints` appends only in the order
Latin, Hangul, Kana, Han. `hasMultipleLanguages` ignores `language.Unknown` and
returns true after seeing two distinct non-Unknown section languages.
Projection functions allocate non-nil slices with `make`, copy every ISO field,
and never sort or alter upstream offsets.

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

- [ ] **Step 8: Run focused tests and refactor while green**

Run:

```bash
go test -count=1 ./examples/multilingual-language-routing/internal/routing
```

Expected: PASS. Then run `gofmt` on both files and rerun the same command.

- [ ] **Step 9: Commit Task 1**

```bash
git add examples/multilingual-language-routing/internal/routing/router.go \
  examples/multilingual-language-routing/internal/routing/router_test.go
git commit -m "feat: add multilingual language routing policy"
```

Use Lore body fields with the exact focused test evidence. Do not include files
owned by later tasks.

## Task 2: Preview, Lifecycle Equality, and Concurrent Reuse

**Complexity:** High. This task owns fixture stability, preload equivalence, and race proof.

**Required skills:** `test-driven-development`, `bluetape-go-patterns`.

**Files:**

- Create: `examples/multilingual-language-routing/internal/routing/preview.go`
- Modify: `examples/multilingual-language-routing/internal/routing/router_test.go`

- [ ] **Step 1: Write failing preview and lifecycle tests**

Add tests that call `NewPreview(false)` twice and `NewPreview(true)` once. Assert:

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

Also assert lifecycle notes mention construct-once/reuse and lazy/preloaded
tradeoffs, and boundary notes mention heuristic, security/compliance,
redaction/logging, and the intentional cost of gathering three public evidence
views. The test must not claim that an in-process comparison measures startup
time, memory, or model-cache behavior; it proves option wiring and route
equivalence only.

- [ ] **Step 2: Run preview tests and observe RED**

Expected: compile failure because `Preview` and `NewPreview` do not exist.

- [ ] **Step 3: Implement fixed preview data**

Create `preview.go` with:

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

- [ ] **Step 4: Run preview tests and observe GREEN**

Run the focused package test. Expected: PASS and exact decision equality across
model-loading modes.

- [ ] **Step 5: Write the failing bounded concurrency test**

Use six requests per round and three rounds. The helper below makes the entry
gate and exact call arithmetic explicit:

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

- [ ] **Step 6: Run normal and race tests**

```bash
go test -count=1 -run '^TestRouterConcurrentFirstUse$' ./examples/multilingual-language-routing/internal/routing
go test -race -count=1 -run '^TestRouterConcurrentFirstUse$' ./examples/multilingual-language-routing/internal/routing
go test -count=1 ./examples/multilingual-language-routing/internal/routing
go test -race -count=1 ./examples/multilingual-language-routing/internal/routing
```

Expected: the isolated first-use commands and both full package commands PASS.
Every concurrency subtest releases exactly six ready goroutines per round,
returns exactly three identical decisions per request and 18 total decisions,
and produces no race report. The test makes no claim about instrumented overlap
inside the upstream detector.

- [ ] **Step 7: Commit Task 2**

```bash
git add examples/multilingual-language-routing/internal/routing/preview.go \
  examples/multilingual-language-routing/internal/routing/router_test.go
git commit -m "test: prove language router lifecycle reuse"
```

Record both normal and race commands in the Lore body.

## Task 3: Deterministic CLI and Preload Flag

**Complexity:** Medium. This task owns only process/flag/output behavior.

**Required skills:** `test-driven-development`, `bluetape-go-patterns`.

**Files:**

- Create: `examples/multilingual-language-routing/main.go`
- Create: `examples/multilingual-language-routing/main_test.go`

- [ ] **Step 1: Write failing CLI tests**

Test `run(args, stdout, stderr)` directly:

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

- [ ] **Step 2: Run CLI tests and observe RED**

Expected: compile failure because `run` and `main.go` do not exist.

- [ ] **Step 3: Implement the CLI seam**

Create `main.go`:

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

- [ ] **Step 4: Run CLI and package proof**

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Expected: tests PASS; both commands exit 0 with valid indented JSON; decoded
decisions are equal and model-loading/config metadata reflects the mode.

- [ ] **Step 5: Commit Task 3**

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

- [ ] **Step 1: Capture actual deterministic output**

Run both `go run` commands from Task 3. Select representative JSON directly
from stdout for English/Korean shared moderation, Japanese tokenization,
Chinese unsupported review, mixed ordered reasons, and the separately labeled
threshold `1.0` low-confidence fallback. Do not
hand-invent numeric confidences or byte offsets.

- [ ] **Step 2: Write the English README**

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

- [ ] **Step 3: Write the Korean README with source-equivalent meaning**

Use title `# multilingual-language-routing`, then `[English](README.md) | 한국어`.
Preserve every command, route/reason value, code symbol, numeric threshold, and
boundary from the English README. Use natural Korean technical prose; do not
abbreviate lifecycle, security, or privacy guidance.

- [ ] **Step 4: Update root README navigation in milestone order**

Insert `examples/multilingual-language-routing` immediately after
`examples/japanese-search-preparation` in both root tables and run sections.
Use English public prose in `README.md` and source-equivalent Korean in
`README.ko.md`. Add both run commands and both focused test commands without
reordering existing examples.

- [ ] **Step 5: Verify documentation against the program**

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
git diff --check
```

Compare README snippets to stdout field-for-field. Search both locales for all
route values, all six review reasons, `0.70`, `8`, `--preload`, `UTF-8`,
`authentication`, `authorization`, `compliance`, and redaction/logging wording.

- [ ] **Step 6: Commit Task 4**

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

- [ ] **Step 1: Run focused verification from scratch**

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Every command must return an observed exit code 0. Lost process handles or
PASS-looking output without an exit code are invalid and must be rerun.

- [ ] **Step 2: Run repository gates in authoritative order**

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

- [ ] **Step 3: Audit exact diff and conditional hazards**

Confirm only the planned example, README pair, spec/plan, `.gitignore`, and
lesson/review artifacts changed. Record concrete N/A evidence:

- new Go module/registration: N/A, this is a package under the existing module;
- dependencies/catalog: N/A, `go.mod` and `go.sum` unchanged;
- workflow/Nightly/coverage: N/A, existing `go test ./...` discovery covers it;
- Docker/Testcontainers/database/HTTP: N/A, none added;
- public bluetape-go API/CHANGELOG: N/A, workshop-internal package only;
- diagram asset: N/A, approved small table/linear flow is clearer.

- [ ] **Step 4: Run spec/plan verifier and pre-PR review**

The verifier maps every spec acceptance row and every plan checkbox to current
files and fresh commands. Then run performance, stability, security,
operator/Ops, developer/API, and user/caller code-review lenses plus main
integration. P0/P1 blocks delivery; fix, rerun focused proof, and rerun only
affected lenses. Resolve or justify every P2/P3.

- [ ] **Step 5: Write and commit the lesson**

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
