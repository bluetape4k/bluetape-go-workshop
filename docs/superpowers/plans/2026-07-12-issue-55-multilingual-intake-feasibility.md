# Multilingual Intake Feasibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a runnable support-intake evaluator that reports English, Korean, Japanese, mixed, short, and unknown language outcomes while tokenizing only supported Japanese input.

**Architecture:** A framework-independent `intake.Evaluator` owns one reusable three-language Lingua detector and one reusable Kagome tokenizer. The command builds the evaluator once, evaluates deterministic fixtures, and prints JSON; application policy turns short, uncertain, or mixed results into explicit manual-review outcomes.

**Tech Stack:** Go 1.26, bluetape-go v0.18.0 `textsearch/language`, `textsearch/japanese`, `testing/concurrency`, standard `encoding/json`.

---

### Task 1: Upgrade the stable bluetape-go baseline

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Upgrade only the bluetape-go module**

Run: `go get github.com/bluetape4k/bluetape-go@v0.18.0`

Expected: `go.mod` requires `github.com/bluetape4k/bluetape-go v0.18.0`; no unrelated direct dependency is added.

- [ ] **Step 2: Normalize the module graph**

Run: `go mod tidy`

Expected: Kagome and Lingua transitive requirements required by imported bluetape-go subpackages are recorded consistently.

### Task 2: Drive the evaluator contract with failing tests

**Files:**
- Create: `examples/multilingual-intake-feasibility/internal/intake/evaluator_test.go`
- Create: `examples/multilingual-intake-feasibility/internal/intake/evaluator.go`

- [ ] **Step 1: Write tests against the desired API**

The tests construct `NewEvaluator(DefaultConfig())`, call `Evaluate(Message{ID, Text})`, and assert:

```go
report, err := evaluator.Evaluate(Message{ID: "ticket-ja", Text: "配送状況を確認したいです。注文番号を教えてください。"})
if err != nil { t.Fatal(err) }
if report.Language != "Japanese" || report.Tokenizer != "kagome-ipa" || report.ManualReview { t.Fatalf("report = %+v", report) }
for _, token := range report.SelectedTokens {
    if report.Text[token.Start:token.End] != token.Text { t.Fatalf("span = %+v", token) }
}
```

Table-driven cases cover English, Korean, Japanese, mixed English/Japanese, short input, numeric/unknown input, and blank input. Error assertions use `errors.Is(err, language.ErrBlankText)`.

- [ ] **Step 2: Verify RED**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/internal/intake`

Expected: FAIL because `NewEvaluator`, `DefaultConfig`, `Message`, and `Report` do not exist.

- [ ] **Step 3: Implement the minimal evaluator**

Define:

```go
type Config struct { MinimumConfidence float64; MinimumRunes int; PreloadModels bool }
type Message struct { ID string `json:"id"`; Text string `json:"text"` }
type SelectedToken struct { Text string `json:"text"`; Start int `json:"start"`; End int `json:"end"`; POS string `json:"pos"` }
type Report struct { ID string; Text string; Language string; Confidence float64; Mixed bool; ScriptHints []string; Tokenizer string; SelectedTokens []SelectedToken; ManualReview bool; ReviewReasons []string }
type Evaluator struct { detector *language.Detector; japanese *japanese.Tokenizer; config Config }
```

`NewEvaluator` selects English, Korean, and Japanese; applies `WithPreloadedLanguageModels` only when requested; and constructs Kagome once. `Evaluate` validates the message, detects language/confidence/sections, records script hints, requires manual review for short, unknown, low-confidence, or mixed input, and tokenizes only a confident non-mixed Japanese result. Selected tokens preserve source order and include nouns or verbs.

- [ ] **Step 4: Verify GREEN**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/internal/intake`

Expected: PASS for normal, boundary, error, and byte-span cases.

### Task 3: Prove reusable detector/tokenizer concurrency

**Files:**
- Modify: `examples/multilingual-intake-feasibility/internal/intake/evaluator_test.go`

- [ ] **Step 1: Add a bounded shared-instance stress test**

Use:

```go
tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 6, RoundsPerTask: 3, Timeout: 5 * time.Second})
report := tester.RunT(t, tasks...)
if report.Completed != 18 { t.Fatalf("report = %+v", report) }
```

Every task calls the same evaluator instance and asserts its expected language, review state, tokenizer, and byte spans.

- [ ] **Step 2: Verify normal and race execution**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/...`

Run: `go test -race -count=1 ./examples/multilingual-intake-feasibility/...`

Expected: both PASS with exactly 18 completed stress calls.

### Task 4: Add the runnable preview and bilingual documentation

**Files:**
- Create: `examples/multilingual-intake-feasibility/main.go`
- Create: `examples/multilingual-intake-feasibility/README.md`
- Create: `examples/multilingual-intake-feasibility/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **Step 1: Add deterministic preview construction**

Add `NewPreview() (Preview, error)` to `evaluator.go`. It evaluates fixed English, Korean, Japanese, mixed, short, and numeric samples and returns scenario, lifecycle notes, heuristic boundaries, reports, and test commands.

- [ ] **Step 2: Add the CLI**

`main.go` builds the preview, encodes it with `json.MarshalIndent`, and prints it. Errors terminate with a descriptive `log.Fatalf` message.

- [ ] **Step 3: Document the lesson in both locales**

Both READMEs describe the package lesson, run command, representative JSON fields, confidence/manual-review policy, Japanese byte spans/POS selection, unsupported English/Korean tokenization, lazy versus preloaded Lingua models, Kagome IPA dictionary footprint, and race-test command.

- [ ] **Step 4: Register the example in root navigation**

Add one example-table row after `gin-text-search-service` and one run section after the Gin text example in both root READMEs.

- [ ] **Step 5: Verify the preview**

Run: `go run ./examples/multilingual-intake-feasibility`

Expected: valid JSON containing six reports, a Japanese `kagome-ipa` report with selected tokens, and review reasons for mixed/short/unknown fixtures.

### Task 5: Complete repository verification and review

**Files:**
- Review all files changed by Tasks 1-4.

- [ ] **Step 1: Format and verify the module**

Run: `gofmt -w examples/multilingual-intake-feasibility`

Run: `make fmt-check && make tidy-check && make vet && make lint`

Expected: all commands PASS.

- [ ] **Step 2: Run focused and repository verification**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/...`

Run: `go test -race -count=1 ./examples/multilingual-intake-feasibility/...`

Run: `make ci`

Expected: all commands PASS.

- [ ] **Step 3: Review scope and evidence**

Run: `git diff --check`

Run: `git diff --stat && git diff -- go.mod go.sum examples/multilingual-intake-feasibility README.md README.ko.md`

Expected: only the approved dependency, example, plan, and bilingual navigation changes; P0=0 and P1=0.
