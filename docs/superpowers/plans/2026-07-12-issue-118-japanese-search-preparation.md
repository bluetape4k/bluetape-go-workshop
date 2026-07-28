# Japanese Search Preparation 구현 계획

> **에이전트 작업자 참고:** 필수 하위 스킬: `superpowers:subagent-driven-development`(권장) 또는 `superpowers:executing-plans`를 사용해 이 계획을 작업 단위로 구현한다. 단계 추적에는 체크박스(`- [ ]`) 문법을 사용한다.

**목표:** Inspectable Kagome search-mode token, source byte span, deterministic matching, support-text masking, shared-service race proof를 포함한 Issue #118 Japanese product-search preparation command를 만든다.

**아키텍처:** Constructor-only `catalogprep.Service`가 Kagome IPA tokenizer 하나, immutable blockword dictionary 하나, prepared catalog 하나를 소유한다. Product preparation은 field-local source span을 보존하고, masked support token을 space-delimited index에서 제외하며, search는 prepared query term으로 call-local `textsearch.Matcher`를 compile한다.

**기술 스택:** Go 1.26, bluetape-go v0.18.0 `textsearch`, `textsearch/japanese`, `testing/concurrency`, 표준 라이브러리 JSON CLI와 table-driven test.

---

## 범위와 파일 지도

- `examples/japanese-search-preparation/internal/catalogprep/service.go` 생성: service contract, validation, preparation, masking, matching, preview, deep-copy helper.
- `examples/japanese-search-preparation/internal/catalogprep/service_test.go` 생성: success, failure, normalization/span, masking, matching, preview, concurrency test.
- `examples/japanese-search-preparation/main.go` 생성: deterministic JSON 명령.
- `examples/japanese-search-preparation/README.md` 생성: English lesson, command, expected output, lifecycle, boundary.
- `examples/japanese-search-preparation/README.ko.md` 생성: Korean locale parity.
- `README.md` 수정: root navigation row와 run section.
- `README.ko.md` 수정: localized root navigation row와 run section.
- Delivery 시점에만 GitHub Issue #34를 갱신한다. 이미 merge된 child #55를 complete로 표시하고, #118은 PR이 merge될 때까지 open으로 둔다.

`go.mod`, workflow, module registration, diagram, Docker fixture, public library package 변경은 계획하지 않는다. 필요해지면 이 계획을 확장하지 말고 중단한 뒤 spec을 다시 연다.

## 예상 위험과 수리 지점

| Risk | Signal | Mitigation | Repair/rerun point |
|---|---|---|---|
| Normalized/index position을 original span으로 착각 | source slicing이 token text와 다름 | field-local source span만 노출하고 모든 selected token을 테스트한다. | Task 2를 중단하고 projection을 고친 뒤 모든 Task 2 test를 다시 실행한다. |
| Masked support term이 search index로 누출 | `偽物`가 `IndexTerms` 또는 query hit에 나타남 | mask match를 먼저 계산하고 겹치는 모든 token을 제외한다. | Task 3 전 중단하고 masking/search test를 다시 실행한다. |
| Shared catalog slice 또는 metadata가 search 사이에서 mutation됨 | deep-copy 또는 race test 실패 | immutable service state, call-local result, recursive copy helper | Task 1/4로 돌아가 focused normal/race test를 다시 실행한다. |
| Matcher boundary semantic이 substring hit를 만듦 | query term이 다른 prepared term 내부에서 match됨 | space-delimited index와 Unicode-boundary negative test | spec에서 index encoding을 다시 열고 Task 2-4를 다시 실행한다. |
| Kagome lifecycle이 반복 작업을 지배 | constructor가 `Search` 또는 stress task 안에 나타남 | `NewService`에서 search-mode tokenizer 하나만 만든다. | 제거될 때까지 performance/stability scan이 delivery를 막는다. |

## Acceptance Traceability

| Spec requirement | Plan task | Proof |
|---|---|---|
| Constructor-only service, validation, deep copy | Task 1 | focused error 및 mutation-isolation test |
| Search-mode tokenization, POS/base form, normalization, byte span | Task 2 | source-slicing 및 metadata test |
| Masking과 index exclusion | Task 2 | 정확한 masked text/match span/index-term assertion |
| 실제 all-term matching과 deterministic ordering | Task 3 | 정확한 query/hit/no-hit test |
| Shared reuse와 race safety | Task 4 | normal/race run에서 bounded exact total |
| Runnable deterministic JSON | Task 4 | preview test와 `go run` JSON parse |
| Bilingual lesson과 root navigation | Task 5 | locale/link/content check |
| Repository quality와 P0/P1 convergence | Task 6 | `make ci`, diff review, CodeGraph/self-review |

### Task 1: Service Contract와 Failure Semantics 고정

**Complexity:** Medium
**Depends on:** approved design spec
**Pattern skills:** `test-driven-development`, `bluetape-go-patterns`
**Files:**
- Create: `examples/japanese-search-preparation/internal/catalogprep/service_test.go`
- Create: `examples/japanese-search-preparation/internal/catalogprep/service.go`

- [x] **Step 1: 실패하는 constructor, zero-value, validation, deep-copy test 작성**

다음 concrete contract를 가진 test를 추가한다.

```go
func TestNewServiceRejectsInvalidProducts(t *testing.T) {
    policy := MaskPolicy{
        Entries: []textsearch.BlockwordEntry{{ID: "unsafe", Text: "偽物"}},
        Mask: "*",
    }
    tests := []ProductInput{
        {Title: "ランニングシューズ", SupportText: "毎日の運動に使えます。"},
        {SKU: "JP-100", SupportText: "毎日の運動に使えます。"},
        {SKU: "JP-100", Title: "ランニングシューズ"},
    }
    for _, product := range tests {
        _, err := NewService([]ProductInput{product}, policy)
        if !errors.Is(err, ErrInvalidProduct) {
            t.Fatalf("NewService(%+v) error = %v, want ErrInvalidProduct", product, err)
        }
    }
}

func TestZeroValueServiceFailsClosed(t *testing.T) {
    var service *Service
    if _, err := service.Search(SearchRequest{Query: "保存 容器"}); !errors.Is(err, ErrInvalidService) {
        t.Fatalf("Search() error = %v, want ErrInvalidService", err)
    }
    if products := service.Products(); products != nil {
        t.Fatalf("Products() = %#v, want nil", products)
    }
}

func TestProductsReturnsDeepCopy(t *testing.T) {
    service := &Service{products: []PreparedProduct{{
        SKU: "JP-COPY",
        IndexTerms: []string{"保存"},
        Tokens: []PreparedToken{{Metadata: map[string]string{japanese.MetadataPOS: "名詞/一般/*/*"}}},
    }}}
    first := service.Products()
    first[0].IndexTerms[0] = "mutated"
    first[0].Tokens[0].Metadata[japanese.MetadataPOS] = "mutated"
    second := service.Products()
    if second[0].IndexTerms[0] == "mutated" || second[0].Tokens[0].Metadata[japanese.MetadataPOS] == "mutated" {
        t.Fatalf("Products() exposed shared state: %#v", second[0])
    }
}
```

Test file에서 `DefaultProducts()`와 `DefaultMaskPolicy()`를 사용해 `newTestService`를 정의한다.

- [x] **Step 2: Focused test 실행 및 RED 기록**

Run:

```bash
go test -count=1 ./examples/japanese-search-preparation/internal/catalogprep
```

기대값: package contract가 없어서 FAIL한다. Undefined symbol 또는 missing package error를 기록한다. Setup/dependency error는 유효한 RED가 아니다.

- [x] **Step 3: 최소 constructor-owned contract 구현**

Spec의 정확한 public shape로 `service.go`를 만든다.

```go
package catalogprep

var (
    ErrInvalidService = errors.New("catalogprep: invalid service")
    ErrInvalidProduct = errors.New("catalogprep: invalid product")
    ErrInvalidQuery   = errors.New("catalogprep: invalid query")
)

type ProductInput struct {
    SKU         string `json:"sku"`
    Title       string `json:"title"`
    SupportText string `json:"support_text"`
}

type MaskPolicy struct {
    Entries []textsearch.BlockwordEntry
    Mask    string
}

type Field string

const (
    FieldTitle       Field = "title"
    FieldSupportText Field = "support_text"
)

type PreparedToken struct {
    Field      Field             `json:"field"`
    Text       string            `json:"text"`
    Normalized string            `json:"normalized"`
    BaseForm   string            `json:"base_form,omitempty"`
    POS        string            `json:"pos"`
    Start      int               `json:"start"`
    End        int               `json:"end"`
    Indexable  bool              `json:"indexable"`
    Metadata   map[string]string `json:"metadata"`
}

type MaskMatch struct {
    ID    string `json:"id"`
    Text  string `json:"text"`
    Start int    `json:"start"`
    End   int    `json:"end"`
}

type PreparedProduct struct {
    SKU               string          `json:"sku"`
    Title             string          `json:"title"`
    SupportText       string          `json:"support_text"`
    MaskedSupportText string          `json:"masked_support_text"`
    MaskMatches       []MaskMatch     `json:"mask_matches"`
    Tokens            []PreparedToken `json:"tokens"`
    IndexTerms        []string        `json:"index_terms"`
    IndexText         string          `json:"index_text"`
}

type SearchRequest struct { Query string `json:"query"` }
type SearchHit struct { SKU string `json:"sku"`; MatchedTerms []string `json:"matched_terms"` }
type SearchResult struct { Query string `json:"query"`; QueryTerms []string `json:"query_terms"`; Hits []SearchHit `json:"hits"` }

type Preview struct {
    Scenario       string            `json:"scenario"`
    Tokenizer      string            `json:"tokenizer"`
    LifecycleNotes []string          `json:"lifecycle_notes"`
    BoundaryNotes  []string          `json:"boundary_notes"`
    Products       []PreparedProduct `json:"products"`
    Searches       []SearchResult    `json:"searches"`
    Commands       []string          `json:"commands"`
}

type Service struct {
    tokenizer  *japanese.Tokenizer
    dictionary *textsearch.BlockwordDictionary
    mask       string
    products   []PreparedProduct
}

func NewService(products []ProductInput, policy MaskPolicy) (*Service, error) {
    if len(products) == 0 {
        return nil, fmt.Errorf("%w: products are required", ErrInvalidProduct)
    }
    tokenizer, err := japanese.NewTokenizer(japanese.WithMode(japanese.Search))
    if err != nil {
        return nil, fmt.Errorf("create search tokenizer: %w", err)
    }
    dictionary, err := textsearch.NewBlockwordDictionary(policy.Entries, textsearch.Config{
        Normalize: textsearch.NormalizeNFC,
        Boundary:  textsearch.BoundaryNone,
    })
    if err != nil {
        return nil, fmt.Errorf("compile mask policy: %w", err)
    }
    if policy.Mask == "" {
        policy.Mask = "*"
    }
    service := &Service{tokenizer: tokenizer, dictionary: dictionary, mask: policy.Mask}
    for _, product := range products {
        if strings.TrimSpace(product.SKU) == "" || strings.TrimSpace(product.Title) == "" || strings.TrimSpace(product.SupportText) == "" {
            return nil, fmt.Errorf("%w: sku, title, and support text are required", ErrInvalidProduct)
        }
        prepared, err := service.prepareProduct(product)
        if err != nil {
            return nil, fmt.Errorf("%w: prepare %q: %w", ErrInvalidProduct, product.SKU, err)
        }
        service.products = append(service.products, prepared)
    }
    return service, nil
}
```

`Products`는 nil/invalid receiver에 대해 `nil`을 반환하고, 그 외에는 모든 slice와 metadata map을 deep-copy한다. 이 임시 preparation boundary를 추가한다. Task 2는 비어 있는 projection을 실제 token과 masking으로 교체한다.

```go
func (s *Service) Products() []PreparedProduct {
    if s == nil || s.tokenizer == nil || s.dictionary == nil { return nil }
    copied := make([]PreparedProduct, len(s.products))
    for i, product := range s.products {
        copied[i] = product
        copied[i].MaskMatches = append([]MaskMatch(nil), product.MaskMatches...)
        copied[i].IndexTerms = append([]string(nil), product.IndexTerms...)
        copied[i].Tokens = make([]PreparedToken, len(product.Tokens))
        for j, token := range product.Tokens {
            copied[i].Tokens[j] = token
            copied[i].Tokens[j].Metadata = maps.Clone(token.Metadata)
        }
    }
    return copied
}

func (s *Service) prepareProduct(input ProductInput) (PreparedProduct, error) {
    for _, text := range []string{input.Title, input.SupportText} {
        if _, err := textsearch.NewTokenizeRequest(text, textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC}); err != nil {
            return PreparedProduct{}, err
        }
    }
    return PreparedProduct{
        SKU: input.SKU, Title: input.Title, SupportText: input.SupportText,
        MaskedSupportText: input.SupportText,
        MaskMatches: []MaskMatch{}, Tokens: []PreparedToken{},
        IndexTerms: []string{}, IndexText: "",
    }, nil
}
```

- [x] **Step 4: Focused test 실행 및 GREEN 기록**

같은 focused command를 실행한다. 기대값은 Task 1 test PASS다. Temporary minimal preparation은 Task 2 전까지만 empty-but-owned projection을 반환할 수 있다. 그래도 upstream request limit을 우회하지 말고 검증해야 한다.

- [x] **Step 5: Commit Task 1**

```bash
git add examples/japanese-search-preparation/internal/catalogprep/service.go examples/japanese-search-preparation/internal/catalogprep/service_test.go
git commit -m "feat: define Japanese catalog preparation contract"
```

**Rollback/rerun point:** Constructor가 다른 dependency를 요구하면 중단하고 이 task를 revert한다. 승인된 dependency boundary가 이를 금지한다.

### Task 2: Token 준비, Span 보존, Masked Term 제외

**Complexity:** High
**Depends on:** Task 1
**Files:**
- Modify: `examples/japanese-search-preparation/internal/catalogprep/service_test.go`
- Modify: `examples/japanese-search-preparation/internal/catalogprep/service.go`

- [x] **Step 1: 실패하는 token, normalization, POS, masking, oversized-input test 추가**

Unsafe noun을 관찰할 수 있는 fixed product를 사용한다.

```go
func TestPrepareProductPreservesSpansAndExcludesMaskedTerms(t *testing.T) {
    service := newTestService(t)
    product := productBySKU(t, service.Products(), "JP-200")
    if product.MaskedSupportText != "電子レンジで温めて使用できます。**に注意してください。" {
        t.Fatalf("MaskedSupportText = %q", product.MaskedSupportText)
    }
    if len(product.MaskMatches) != 1 || product.MaskMatches[0].Text != "偽物" {
        t.Fatalf("MaskMatches = %#v", product.MaskMatches)
    }
    for _, token := range product.Tokens {
        source := product.Title
        if token.Field == FieldSupportText {
            source = product.SupportText
        }
        if got := source[token.Start:token.End]; got != token.Text {
            t.Fatalf("source[%d:%d] = %q, want %q", token.Start, token.End, got, token.Text)
        }
        if token.POS == "" || token.Metadata[japanese.MetadataPOS] == "" {
            t.Fatalf("missing POS metadata: %+v", token)
        }
        if token.Text == "偽物" && token.Indexable {
            t.Fatalf("masked token remained indexable: %+v", token)
        }
    }
    if slices.Contains(product.IndexTerms, "偽物") {
        t.Fatalf("IndexTerms contain masked term: %#v", product.IndexTerms)
    }
}

func TestPrepareProductUsesNFCWithoutMovingSourceSpans(t *testing.T) {
    service, err := NewService([]ProductInput{{
        SKU: "JP-NFC", Title: "ガラス保存容器", SupportText: "食品を安全に保存します。",
    }}, DefaultMaskPolicy())
    if err != nil {
        t.Fatalf("NewService() error = %v", err)
    }
    product := service.Products()[0]
    for _, token := range product.Tokens {
        source := product.Title
        if token.Field == FieldSupportText { source = product.SupportText }
        if source[token.Start:token.End] != token.Text {
            t.Fatalf("invalid source span: %+v", token)
        }
        if token.Normalized != textsearch.NormalizeText(token.Text, textsearch.NormalizeNFC).Normalized {
            t.Fatalf("Normalized = %q for %q", token.Normalized, token.Text)
        }
    }
}
```

`textsearch.MaxTokenizeTextLength`보다 긴 input에 대한 table case를 추가하고, `ErrInvalidProduct`와 `textsearch.ErrTokenizeTextTooLong`을 모두 `errors.Is`로 검사할 수 있는지 검증한다.

- [x] **Step 2: Focused test 실행 및 RED 기록**

기대값: preparation/masking field와 behavior가 없거나 잘못되어 FAIL한다.

- [x] **Step 3: Preparation 및 masking projection 구현**

`prepareField`, `prepareProduct`, `indexTerm`, overlap helper를 구현한다. Core projection은 다음과 같다.

```go
request, err := textsearch.NewTokenizeRequest(input, textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC})
response, err := s.tokenizer.Tokenize(request)
selected := japanese.Filter(response.Tokens, func(token textsearch.Token) bool {
    return japanese.IsNoun(token) || japanese.IsVerb(token)
})
```

Support text에는 `BlockwordOptions{Mask: s.mask}`로 `dictionary.Process`를 호출한다. 모든 blockword match를 `MaskMatch`로 변환하고, 겹치는 selected token은 non-indexable로 표시하며, first-seen `IndexTerms`에서 제외한다. Kagome base form이 비어 있거나 `"*"`가 아니면 이를 사용하고, 선택한 term은 NFC로 normalize한다.

Default는 정확히 한 번 구현한다.

```go
func DefaultMaskPolicy() MaskPolicy {
    return MaskPolicy{
        Entries: []textsearch.BlockwordEntry{{ID: "counterfeit", Text: "偽物", Severity: textsearch.SeverityHigh}},
        Mask: "*",
    }
}

func DefaultProducts() []ProductInput {
    return []ProductInput{
        {SKU: "JP-100", Title: "軽量ランニングシューズ", SupportText: "毎日のランニングを快適にします。"},
        {SKU: "JP-200", Title: "ガラス保存容器", SupportText: "電子レンジで温めて使用できます。偽物に注意してください。"},
        {SKU: "JP-300", Title: "折りたたみ自転車", SupportText: "通勤で便利に走れます。"},
    }
}
```

`DefaultProducts`는 running shoes, glass storage, folding bicycle을 다루는 stable SKU 세 개(`JP-100`, `JP-200`, `JP-300`)를 반환한다. JP-200은 위 assertion에 사용한 정확한 support text를 사용한다.

공유 Task 2 test helper를 추가한다.

```go
func newTestService(t *testing.T) *Service {
    t.Helper()
    service, err := NewService(DefaultProducts(), DefaultMaskPolicy())
    if err != nil { t.Fatalf("NewService() error = %v", err) }
    return service
}

func productBySKU(t *testing.T, products []PreparedProduct, sku string) PreparedProduct {
    t.Helper()
    for _, product := range products {
        if product.SKU == sku { return product }
    }
    t.Fatalf("product %q not found", sku)
    return PreparedProduct{}
}
```

- [x] **Step 4: Focused normal 및 race test 실행**

```bash
go test -count=1 ./examples/japanese-search-preparation/internal/catalogprep
go test -race -count=1 ./examples/japanese-search-preparation/internal/catalogprep
```

기대값: PASS. NFC fixture가 다른 Kagome segmentation을 드러내면 contract assertion(normalization plus original slicing)은 보존하고, source-span rule이 아니라 fixture-specific token lookup만 조정한다.

- [x] **Step 5: Commit Task 2**

```bash
git add examples/japanese-search-preparation/internal/catalogprep
git commit -m "feat: prepare and mask Japanese catalog terms"
```

**Rollback/rerun point:** Original field를 slice할 수 없는 span은 P1이다. Search 작업 전에 중단하고 수리한다.

### Task 3: Deterministic All-Term Catalog Search 추가

**Complexity:** Medium
**Depends on:** Task 2
**Files:**
- Modify: `examples/japanese-search-preparation/internal/catalogprep/service_test.go`
- Modify: `examples/japanese-search-preparation/internal/catalogprep/service.go`

- [x] **Step 1: 실패하는 search success, ordering, no-match, invalid-query test 추가**

```go
func TestSearchRequiresAllPreparedTermsAndSortsBySKU(t *testing.T) {
    service := newTestService(t)
    result, err := service.Search(SearchRequest{Query: "保存 容器"})
    if err != nil { t.Fatalf("Search() error = %v", err) }
    if got := hitSKUs(result.Hits); !reflect.DeepEqual(got, []string{"JP-200"}) {
        t.Fatalf("hit SKUs = %#v", got)
    }
    if !reflect.DeepEqual(result.Hits[0].MatchedTerms, result.QueryTerms) {
        t.Fatalf("MatchedTerms = %#v, QueryTerms = %#v", result.Hits[0].MatchedTerms, result.QueryTerms)
    }
}

func TestSearchReturnsStableEmptyHits(t *testing.T) {
    result, err := newTestService(t).Search(SearchRequest{Query: "宇宙船"})
    if err != nil { t.Fatalf("Search() error = %v", err) }
    if result.Hits == nil || len(result.Hits) != 0 {
        t.Fatalf("Hits = %#v, want non-nil empty slice", result.Hits)
    }
}

func TestSearchRejectsBlankOrUnindexableQuery(t *testing.T) {
    service := newTestService(t)
    for _, query := range []string{" ", "の"} {
        _, err := service.Search(SearchRequest{Query: query})
        if !errors.Is(err, ErrInvalidQuery) {
            t.Fatalf("Search(%q) error = %v", query, err)
        }
    }
}
```

Duplicate-term query를 추가하고 `QueryTerms`가 first-seen unique인지 검증한다.

Search 및 concurrency test에서 사용할 exact result helper를 추가한다.

```go
func hitSKUs(hits []SearchHit) []string {
    result := make([]string, len(hits))
    for i, hit := range hits { result[i] = hit.SKU }
    return result
}
```

- [x] **Step 2: Focused test 실행 및 RED 기록**

기대값: 누락되었거나 불완전한 `Search` 구현 때문에 FAIL한다.

- [x] **Step 3: Query preparation 및 matcher-based search 구현**

같은 NFC/base-form projection으로 query term을 준비한다. Term text를 stable ID로 사용해 pattern을 compile한다.

```go
patterns := make([]textsearch.Pattern, len(terms))
for i, term := range terms {
    patterns[i] = textsearch.Pattern{ID: term, Text: term}
}
matcher, err := textsearch.Compile(patterns, textsearch.Config{
    Normalize: textsearch.NormalizeNFC,
    Boundary: textsearch.BoundaryUnicodeWord,
    Overlap: textsearch.OverlapLeftmostLongest,
})
```

각 product에 `matcher.FindAll(product.IndexText)`를 호출하고 unique matched pattern ID를 수집한다. 모든 query term이 match된 경우에만 hit를 emit한다. Matched term은 query order로 반환하고 hit는 SKU 기준으로 정렬한다. 성공했지만 결과가 비어 있는 경우에도 non-nil empty `QueryTerms`/`Hits` slice를 할당한다.

- [x] **Step 4: Focused normal 및 race test 실행**

기대값: exact hit 및 ordering assertion과 함께 둘 다 PASS한다.

- [x] **Step 5: Commit Task 3**

```bash
git add examples/japanese-search-preparation/internal/catalogprep
git commit -m "feat: search prepared Japanese catalog terms"
```

**Rollback/rerun point:** Unicode boundary가 exact space-delimited term matching을 강제하지 못하면 boundary-focused test를 추가하고, 진행 전에 spec의 prepared index encoding을 수정한다.

### Task 4: Shared Reuse 증명과 Runnable Preview 추가

**Complexity:** Medium
**Depends on:** Tasks 1-3
**Files:**
- Modify: `examples/japanese-search-preparation/internal/catalogprep/service_test.go`
- Modify: `examples/japanese-search-preparation/internal/catalogprep/service.go`
- Create: `examples/japanese-search-preparation/main.go`

- [x] **Step 1: 실패하는 bounded concurrency 및 preview test 추가**

`concurrencytest.NewGoroutineStressTester`를 worker 6개, task 6개, round 3개, 5초 timeout으로 사용한다. Service 하나를 공유한다. 모든 task는 exact hit SKU, JP-200 masked text, 반환된 모든 source span을 검증해야 한다.

```go
func TestServiceSharedReuseUnderBoundedConcurrency(t *testing.T) {
service := newTestService(t)
tasks := make([]concurrencytest.Task, 6)
queries := []struct{ query string; want []string }{
    {"ランニング シューズ", []string{"JP-100"}},
    {"保存 容器", []string{"JP-200"}},
    {"宇宙船", []string{}},
}
for i := range tasks {
    fixture := queries[i%len(queries)]
    tasks[i] = func(ctx context.Context) error {
        if err := ctx.Err(); err != nil { return err }
        result, err := service.Search(SearchRequest{Query: fixture.query})
        if err != nil { return err }
        if !reflect.DeepEqual(hitSKUs(result.Hits), fixture.want) {
            return fmt.Errorf("hits = %#v, want %#v", hitSKUs(result.Hits), fixture.want)
        }
        return validateProducts(service.Products())
    }
}
tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
    Workers: 6, RoundsPerTask: 3, Timeout: 5 * time.Second,
})
report := tester.RunT(t, tasks...)
if report.Completed != 18 || report.MaxConcurrent < 2 {
    t.Fatalf("stress report = %+v", report)
}
}

func validateProducts(products []PreparedProduct) error {
    product := products[1]
    if product.MaskedSupportText != "電子レンジで温めて使用できます。**に注意してください。" {
        return fmt.Errorf("masked support text = %q", product.MaskedSupportText)
    }
    for _, token := range product.Tokens {
        source := product.Title
        if token.Field == FieldSupportText { source = product.SupportText }
        if source[token.Start:token.End] != token.Text {
            return fmt.Errorf("invalid source span: %+v", token)
        }
    }
    return nil
}
```

Product 세 개, search 세 개, masked fixture 하나, lifecycle/boundary note, exact focused normal/race command를 검증하는 `TestNewPreviewDocumentsScenario`를 추가한다.

- [x] **Step 2: Focused normal 및 race test 실행 후 RED 기록**

기대값: `NewPreview` 전에는 preview test가 FAIL한다. Concurrency assertion은 shared-slice defect를 드러낼 수 있다.

- [x] **Step 3: Preview와 CLI 구현**

`NewPreview`는 default로 service 하나를 구성하고 product를 copy한 뒤 fixed query `"ランニング シューズ"`, `"保存 容器"`, `"宇宙船"`를 평가하고 deterministic JSON-friendly slice를 반환한다.

`main.go`를 만든다.

```go
package main

func main() {
    preview, err := catalogprep.NewPreview()
    if err != nil { log.Fatalf("build Japanese search preview: %v", err) }
    encoded, err := json.MarshalIndent(preview, "", "  ")
    if err != nil { log.Fatalf("encode Japanese search preview: %v", err) }
    fmt.Println(string(encoded))
}
```

- [x] **Step 4: Focused proof 실행 및 CLI JSON 검증**

```bash
go test -count=1 ./examples/japanese-search-preparation/...
go test -race -count=1 ./examples/japanese-search-preparation/...
go run ./examples/japanese-search-preparation >/tmp/japanese-search-preparation.json
python3 -m json.tool /tmp/japanese-search-preparation.json >/dev/null
```

기대값: 모든 명령이 PASS한다. Output에는 `JP-100`, `JP-200`, `kagome-ipa-search`, masked `**`가 포함된다.

- [x] **Step 5: Commit Task 4**

```bash
git add examples/japanese-search-preparation
git commit -m "feat: add Japanese search preparation preview"
```

### Task 5: 두 Locale에 Lesson 문서화

**Complexity:** Medium
**Depends on:** generated preview from Task 4
**Files:**
- Create: `examples/japanese-search-preparation/README.md`
- Create: `examples/japanese-search-preparation/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [x] **Step 1: 실제 output 기반 paired example README 작성**

두 파일은 package lesson, three-stage flow, run/test command, representative token/POS/span/index/search/mask output, Normal-vs-Search mode intent, construct-once reuse, IPA dictionary footprint, NFC/source-span semantic, substring masking boundary, non-goal을 포함해야 한다. English public prose가 authoritative이며, Korean content는 모든 factual section을 대응시킨다.

- [x] **Step 2: Root navigation과 run section 추가**

두 root locale에서 다른 textsearch example 옆에 table row 하나와 compact run section 하나를 넣는다. English 및 Korean example file을 명시적으로 link한다.

- [x] **Step 3: Locale parity와 link 검증**

```bash
test -f examples/japanese-search-preparation/README.md
test -f examples/japanese-search-preparation/README.ko.md
rg -n "japanese-search-preparation" README.md README.ko.md
rg -n "go run ./examples/japanese-search-preparation|go test -race" examples/japanese-search-preparation/README.md examples/japanese-search-preparation/README.ko.md
git diff --check
```

기대값: 두 locale file과 두 root file이 example link/command를 포함하고 diff check가 clean이다.

- [x] **Step 4: Commit Task 5**

```bash
git add README.md README.ko.md examples/japanese-search-preparation/README.md examples/japanese-search-preparation/README.ko.md
git commit -m "docs: explain Japanese search preparation"
```

### Task 6: 승인된 Spec 검증, Diff Review, Delivery 준비

**Complexity:** High verification
**Depends on:** Tasks 1-5
**Files:**
- 모든 branch change를 spec 및 plan과 대조해 review한다.
- Type A workflow가 요구하는 대로 evidence 또는 evidence-backed N/A decision을 담은 `docs/lessons/2026-07-12-issue-118-japanese-search-preparation.md`를 만든다.

- [x] **Step 1: Fast static 및 focused gate 실행**

```bash
make fmt-check
make tidy-check
make vet
make lint
go test -count=1 ./examples/japanese-search-preparation/...
go test -race -count=1 ./examples/japanese-search-preparation/...
```

기대값: 모든 명령이 PASS하고 lint 또는 race finding이 없다.

- [x] **Step 2: Repository-wide CI 순차 실행**

```bash
make ci
```

기대값: tidy, format, vet, lint, 모든 normal test, 모든 race test가 PASS한다. 다른 Docker-backed suite를 병렬로 실행하지 않는다.

- [x] **Step 3: Spec/plan acceptance를 line-by-line 검증**

Spec, 이 plan, command JSON, README pair, test, diff를 읽는다. 각 acceptance row를 PASS로 기록하거나 owning task로 돌아간다. `go.mod`가 변경되지 않았고 HTTP, ranking, persistence, new dependency, rune-offset contract가 생기지 않았는지 확인한다.

- [x] **Step 4: 최종 structural 및 severity review 실행**

`performance-stability-scan.md`를 읽고, 명시적 new file로 CodeGraph change detection을 시도한 뒤 six Type A perspective와 main integration을 수행한다. Collaboration spawn schema에 별도 `agent_type` field가 없으면 각 child prompt에 설치된 native role을 명시적으로 주입한다. 이 workflow는 executor, verifier, code-reviewer, writer lane을 사용했다. P0 또는 P1은 delivery를 막는다. 수정하고 영향받은 focused/full gate를 다시 실행한 뒤 해당 lens를 반복한다.

- [x] **Step 5: 필요한 durable lesson commit**

구체적인 decision, normalization/span distinction, masked-token exclusion, verification evidence, observed miss, future guard를 작성한다. 그런 다음:

```bash
git add docs/lessons/2026-07-12-issue-118-japanese-search-preparation.md
git commit -m "docs: record Japanese search preparation lessons"
```

- [x] **Step 6: External delivery boundary에서 중단**

Branch commit, clean status, checklist count, P0=0/P1=0, verification evidence를 제시한다. PR creation, Issue #34 edit, merge, remote branch deletion, local synchronization은 active thread의 명시적 user authorization이 필요하다.

**Rollback/rerun point:** 이 feature는 application-shaped additive change다. Rollback은 branch abandonment다. Spec mismatch는 owning task로 돌아가며, rerun 전까지 이후 verification evidence를 무효화한다.
