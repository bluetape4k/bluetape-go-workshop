# Multilingual Intake Feasibility 구현 계획

> **에이전트 작업자 참고:** 필수 하위 스킬: `superpowers:subagent-driven-development`(권장) 또는 `superpowers:executing-plans`를 사용해 이 계획을 작업 단위로 구현한다. 단계 추적에는 체크박스(`- [ ]`) 문법을 사용한다.

**목표:** English, Korean, Japanese, mixed, short, unknown language outcome을 보고하고, 지원되는 Japanese input만 tokenizing하는 runnable support-intake evaluator를 추가한다.

**아키텍처:** 프레임워크에 독립적인 `intake.Evaluator`가 재사용 가능한 three-language Lingua detector 하나와 재사용 가능한 Kagome tokenizer 하나를 소유한다. Command는 evaluator를 한 번 만들고 결정적 fixture를 평가한 뒤 JSON을 출력한다. Application policy는 short, uncertain, mixed result를 명시적인 manual-review outcome으로 바꾼다.

**기술 스택:** Go 1.26, bluetape-go v0.18.0 `textsearch/language`, `textsearch/japanese`, `testing/concurrency`, 표준 `encoding/json`.

---

### Task 1: Stable bluetape-go baseline upgrade

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: bluetape-go module만 upgrade**

Run: `go get github.com/bluetape4k/bluetape-go@v0.18.0`

기대값: `go.mod`가 `github.com/bluetape4k/bluetape-go v0.18.0`을 요구한다. 관련 없는 direct dependency는 추가하지 않는다.

- [ ] **Step 2: Module graph normalize**

Run: `go mod tidy`

기대값: import한 bluetape-go subpackage가 요구하는 Kagome 및 Lingua transitive requirement가 일관되게 기록된다.

### Task 2: 실패 테스트로 evaluator contract 고정

**Files:**
- Create: `examples/multilingual-intake-feasibility/internal/intake/evaluator_test.go`
- Create: `examples/multilingual-intake-feasibility/internal/intake/evaluator.go`

- [ ] **Step 1: 원하는 API에 대한 test 작성**

테스트는 `NewEvaluator(DefaultConfig())`를 구성하고 `Evaluate(Message{ID, Text})`를 호출한 뒤 다음을 검증한다.

```go
report, err := evaluator.Evaluate(Message{ID: "ticket-ja", Text: "配送状況を確認したいです。注文番号を教えてください。"})
if err != nil { t.Fatal(err) }
if report.Language != "Japanese" || report.Tokenizer != "kagome-ipa" || report.ManualReview { t.Fatalf("report = %+v", report) }
for _, token := range report.SelectedTokens {
    if report.Text[token.Start:token.End] != token.Text { t.Fatalf("span = %+v", token) }
}
```

Table-driven case는 English, Korean, Japanese, mixed English/Japanese, short input, numeric/unknown input, blank input을 다룬다. Error assertion은 `errors.Is(err, language.ErrBlankText)`를 사용한다.

- [ ] **Step 2: Verify RED**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/internal/intake`

기대값: `NewEvaluator`, `DefaultConfig`, `Message`, `Report`가 없어서 FAIL한다.

- [ ] **Step 3: 최소 evaluator 구현**

다음을 정의한다.

```go
type Config struct { MinimumConfidence float64; MinimumRunes int; PreloadModels bool }
type Message struct { ID string `json:"id"`; Text string `json:"text"` }
type SelectedToken struct { Text string `json:"text"`; Start int `json:"start"`; End int `json:"end"`; POS string `json:"pos"` }
type Report struct { ID string; Text string; Language string; Confidence float64; Mixed bool; ScriptHints []string; Tokenizer string; SelectedTokens []SelectedToken; ManualReview bool; ReviewReasons []string }
type Evaluator struct { detector *language.Detector; japanese *japanese.Tokenizer; config Config }
```

`NewEvaluator`는 English, Korean, Japanese를 선택한다. 요청된 경우에만 `WithPreloadedLanguageModels`를 적용하고 Kagome를 한 번 구성한다. `Evaluate`는 message를 검증하고 language/confidence/section을 감지하며 script hint를 기록한다. Short, unknown, low-confidence, mixed input에는 manual review를 요구하고, confident non-mixed Japanese result만 tokenize한다. Selected token은 source order를 보존하고 noun 또는 verb를 포함한다.

- [ ] **Step 4: Verify GREEN**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/internal/intake`

기대값: normal, boundary, error, byte-span case가 PASS한다.

### Task 3: 재사용 detector/tokenizer concurrency 증명

**Files:**
- Modify: `examples/multilingual-intake-feasibility/internal/intake/evaluator_test.go`

- [ ] **Step 1: 제한된 shared-instance stress test 추가**

Use:

```go
tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 6, RoundsPerTask: 3, Timeout: 5 * time.Second})
report := tester.RunT(t, tasks...)
if report.Completed != 18 { t.Fatalf("report = %+v", report) }
```

모든 task는 같은 evaluator instance를 호출하고 expected language, review state, tokenizer, byte span을 검증한다.

- [ ] **Step 2: Normal 및 race execution 검증**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/...`

Run: `go test -race -count=1 ./examples/multilingual-intake-feasibility/...`

기대값: 두 명령이 모두 PASS하고 completed stress call이 정확히 18개다.

### Task 4: Runnable preview와 bilingual documentation 추가

**Files:**
- Create: `examples/multilingual-intake-feasibility/main.go`
- Create: `examples/multilingual-intake-feasibility/README.md`
- Create: `examples/multilingual-intake-feasibility/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **Step 1: Deterministic preview construction 추가**

`evaluator.go`에 `NewPreview() (Preview, error)`를 추가한다. 이 함수는 고정 English, Korean, Japanese, mixed, short, numeric sample을 평가하고 scenario, lifecycle note, heuristic boundary, report, test command를 반환한다.

- [ ] **Step 2: CLI 추가**

`main.go`는 preview를 만들고 `json.MarshalIndent`로 encode한 뒤 출력한다. 오류가 발생하면 설명적인 `log.Fatalf` message로 종료한다.

- [ ] **Step 3: 두 locale에 lesson 문서화**

두 README는 package lesson, run command, representative JSON field, confidence/manual-review policy, Japanese byte span/POS selection, 지원하지 않는 English/Korean tokenization, lazy versus preloaded Lingua model, Kagome IPA dictionary footprint, race-test command를 설명한다.

- [ ] **Step 4: Root navigation에 example 등록**

두 root README에서 `gin-text-search-service` 뒤에 example-table row 하나를 추가하고, Gin text example 뒤에 run section 하나를 추가한다.

- [ ] **Step 5: Preview 검증**

Run: `go run ./examples/multilingual-intake-feasibility`

기대값: 여섯 개 report, selected token을 포함한 Japanese `kagome-ipa` report, mixed/short/unknown fixture의 review reason을 담은 valid JSON이다.

### Task 5: Repository verification과 review 완료

**Files:**
- Task 1-4에서 변경한 모든 파일을 review한다.

- [ ] **Step 1: Module format 및 검증**

Run: `gofmt -w examples/multilingual-intake-feasibility`

Run: `make fmt-check && make tidy-check && make vet && make lint`

기대값: 모든 명령이 PASS한다.

- [ ] **Step 2: Focused 및 repository verification 실행**

Run: `go test -count=1 ./examples/multilingual-intake-feasibility/...`

Run: `go test -race -count=1 ./examples/multilingual-intake-feasibility/...`

Run: `make ci`

기대값: 모든 명령이 PASS한다.

- [ ] **Step 3: Scope와 evidence review**

Run: `git diff --check`

Run: `git diff --stat && git diff -- go.mod go.sum examples/multilingual-intake-feasibility README.md README.ko.md`

기대값: 승인된 dependency, example, plan, bilingual navigation 변경만 존재한다. P0=0, P1=0이어야 한다.
