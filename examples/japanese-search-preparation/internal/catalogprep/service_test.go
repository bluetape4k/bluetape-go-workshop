package catalogprep

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
)

func TestExcludeOverlappingTokensSweepsOrderedMatches(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []PreparedToken
		matches []MaskMatch
		want    []bool
	}{
		{
			name:   "no matches",
			tokens: []PreparedToken{{Start: 0, End: 5, Indexable: true}},
			want:   []bool{true},
		},
		{
			name: "before between after boundaries overlaps and advances",
			tokens: []PreparedToken{
				{Start: 0, End: 5, Indexable: true},
				{Start: 5, End: 10, Indexable: true},
				{Start: 10, End: 12, Indexable: true},
				{Start: 20, End: 25, Indexable: true},
				{Start: 25, End: 30, Indexable: true},
				{Start: 30, End: 35, Indexable: true},
				{Start: 40, End: 45, Indexable: true},
			},
			matches: []MaskMatch{
				{Start: 10, End: 20},
				{Start: 30, End: 40},
			},
			want: []bool{true, true, false, true, true, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludeOverlappingTokens(tt.tokens, tt.matches)
			for i, token := range tt.tokens {
				if token.Indexable != tt.want[i] {
					t.Fatalf("token %d span %d:%d Indexable = %t, want %t", i, token.Start, token.End, token.Indexable, tt.want[i])
				}
			}
		})
	}
}

func TestNewServicePreparesAndMasksDefaultCatalog(t *testing.T) {
	product := productBySKU(t, newTestService(t).Products(), "JP-200")
	if product.MaskedSupportText != "電子レンジで温めて使用できます。**に注意してください。" {
		t.Fatalf("MaskedSupportText = %q", product.MaskedSupportText)
	}
	if len(product.MaskMatches) != 1 || product.MaskMatches[0].Text != "偽物" {
		t.Fatalf("MaskMatches = %#v, want one 偽物 match", product.MaskMatches)
	}

	foundMaskedToken := false
	for _, token := range product.Tokens {
		source := product.Title
		if token.Field == FieldSupportText {
			source = product.SupportText
		}
		if token.Start < 0 || token.End > len(source) || token.Start >= token.End {
			t.Fatalf("token %q has invalid %s span %d:%d", token.Text, token.Field, token.Start, token.End)
		}
		if got := source[token.Start:token.End]; got != token.Text {
			t.Fatalf("token %q span slices %q from %s", token.Text, got, token.Field)
		}
		if token.POS == "" || token.Metadata[japanese.MetadataPOS] == "" {
			t.Fatalf("token %q is missing POS data: %#v", token.Text, token)
		}
		if token.Text == "偽物" {
			foundMaskedToken = true
			if token.Indexable {
				t.Fatalf("masked token is indexable: %#v", token)
			}
		}
	}
	if !foundMaskedToken {
		t.Fatal("selected tokens do not contain 偽物")
	}
	for _, term := range product.IndexTerms {
		if term == "偽物" {
			t.Fatalf("IndexTerms contains masked term: %#v", product.IndexTerms)
		}
	}
	if product.IndexText != strings.Join(product.IndexTerms, " ") {
		t.Fatalf("IndexText = %q, want joined terms %#v", product.IndexText, product.IndexTerms)
	}
}

func TestNewServicePreservesOriginalSpansForNFCNormalization(t *testing.T) {
	input := ProductInput{
		SKU:         "JP-NFC",
		Title:       "ガラス保存容器",
		SupportText: "食品を安全に保存します。",
	}
	service, err := NewService([]ProductInput{input}, DefaultMaskPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	product := productBySKU(t, service.Products(), input.SKU)
	if len(product.Tokens) == 0 {
		t.Fatal("prepared product has no selected tokens")
	}
	for _, token := range product.Tokens {
		source := input.Title
		if token.Field == FieldSupportText {
			source = input.SupportText
		}
		if token.Start < 0 || token.End > len(source) || token.Start >= token.End {
			t.Fatalf("token %q has invalid %s span %d:%d", token.Text, token.Field, token.Start, token.End)
		}
		if got := source[token.Start:token.End]; got != token.Text {
			t.Fatalf("token %q span slices %q from %s", token.Text, got, token.Field)
		}
		wantNormalized := textsearch.NormalizeText(token.Text, textsearch.NormalizeNFC).Normalized
		if token.Normalized != wantNormalized {
			t.Fatalf("token %q Normalized = %q, want %q", token.Text, token.Normalized, wantNormalized)
		}
	}
}

func TestNewServicePreservesOversizedTokenizeErrors(t *testing.T) {
	tooLong := strings.Repeat("あ", textsearch.MaxTokenizeTextLength+1)
	tests := []ProductInput{
		{SKU: "JP-LONG-TITLE", Title: tooLong, SupportText: "食品を保存します。"},
		{SKU: "JP-LONG-SUPPORT", Title: "保存容器", SupportText: tooLong},
	}
	for _, product := range tests {
		_, err := NewService([]ProductInput{product}, DefaultMaskPolicy())
		if !errors.Is(err, ErrInvalidProduct) || !errors.Is(err, textsearch.ErrTokenizeTextTooLong) {
			t.Fatalf("NewService(%q) error = %v, want ErrInvalidProduct and ErrTokenizeTextTooLong", product.SKU, err)
		}
	}
}

func TestDefaultsDescribeJapaneseCatalogPreparationScenario(t *testing.T) {
	products := DefaultProducts()
	want := []ProductInput{
		{SKU: "JP-100", Title: "軽量ランニングシューズ", SupportText: "毎日のランニングを快適にします。"},
		{SKU: "JP-200", Title: "ガラス保存容器", SupportText: "電子レンジで温めて使用できます。偽物に注意してください。"},
		{SKU: "JP-300", Title: "折りたたみ自転車", SupportText: "通勤で便利に走れます。"},
	}
	if len(products) != len(want) {
		t.Fatalf("DefaultProducts() length = %d, want %d", len(products), len(want))
	}
	for i := range want {
		if products[i] != want[i] {
			t.Fatalf("DefaultProducts()[%d] = %#v, want %#v", i, products[i], want[i])
		}
	}

	policy := DefaultMaskPolicy()
	if policy.Mask != "*" || len(policy.Entries) != 1 {
		t.Fatalf("DefaultMaskPolicy() = %#v", policy)
	}
	entry := policy.Entries[0]
	if entry.ID != "counterfeit" || entry.Text != "偽物" || entry.Severity != textsearch.SeverityHigh {
		t.Fatalf("DefaultMaskPolicy() entry = %#v", entry)
	}
}

func TestNewServiceRejectsInvalidProducts(t *testing.T) {
	policy := MaskPolicy{
		Entries: []textsearch.BlockwordEntry{{ID: "unsafe", Text: "偽物"}},
		Mask:    "*",
	}
	tests := []ProductInput{
		{SKU: " \t", Title: "ランニングシューズ", SupportText: "毎日の運動に使えます。"},
		{SKU: "JP-100", Title: " \t", SupportText: "毎日の運動に使えます。"},
		{SKU: "JP-100", Title: "ランニングシューズ", SupportText: " \t"},
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

func TestSearchRequiresEveryPreparedQueryTerm(t *testing.T) {
	result, err := newTestService(t).Search(SearchRequest{Query: "保存 容器"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	wantTerms := []string{"保存", "容器"}
	if !slices.Equal(result.QueryTerms, wantTerms) {
		t.Fatalf("QueryTerms = %#v, want %#v", result.QueryTerms, wantTerms)
	}
	if got, want := hitSKUs(result.Hits), []string{"JP-200"}; !slices.Equal(got, want) {
		t.Fatalf("hit SKUs = %#v, want %#v", got, want)
	}
	if !slices.Equal(result.Hits[0].MatchedTerms, result.QueryTerms) {
		t.Fatalf("MatchedTerms = %#v, want QueryTerms %#v", result.Hits[0].MatchedTerms, result.QueryTerms)
	}
}

func TestSearchReturnsOwnedEmptyHitsForStableNonMatch(t *testing.T) {
	result, err := newTestService(t).Search(SearchRequest{Query: "宇宙船"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.QueryTerms == nil {
		t.Fatal("QueryTerms = nil, want owned non-nil slice")
	}
	if result.Hits == nil || len(result.Hits) != 0 {
		t.Fatalf("Hits = %#v, want owned empty slice", result.Hits)
	}
}

func TestSearchRejectsQueriesWithoutIndexableTerms(t *testing.T) {
	for _, query := range []string{" ", "の"} {
		t.Run(query, func(t *testing.T) {
			_, err := newTestService(t).Search(SearchRequest{Query: query})
			if !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("Search(%q) error = %v, want ErrInvalidQuery", query, err)
			}
		})
	}
}

func TestSearchPreservesQueryPreparationErrors(t *testing.T) {
	query := strings.Repeat("あ", textsearch.MaxTokenizeTextLength+1)
	_, err := newTestService(t).Search(SearchRequest{Query: query})
	if !errors.Is(err, ErrInvalidQuery) || !errors.Is(err, textsearch.ErrTokenizeTextTooLong) {
		t.Fatalf("Search() error = %v, want ErrInvalidQuery and ErrTokenizeTextTooLong", err)
	}
}

func TestSearchKeepsUniqueQueryTermsInFirstSeenOrder(t *testing.T) {
	result, err := newTestService(t).Search(SearchRequest{Query: "保存 保存 容器 保存"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	want := []string{"保存", "容器"}
	if !slices.Equal(result.QueryTerms, want) {
		t.Fatalf("QueryTerms = %#v, want %#v", result.QueryTerms, want)
	}
}

func TestSearchSortsMultipleHitsBySKU(t *testing.T) {
	products := []ProductInput{
		{SKU: "JP-Z", Title: "ガラス保存容器", SupportText: "食品を保存できます。"},
		{SKU: "JP-A", Title: "密閉保存容器", SupportText: "食品を保存できます。"},
	}
	service, err := NewService(products, DefaultMaskPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Search(SearchRequest{Query: "保存 容器"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got, want := hitSKUs(result.Hits), []string{"JP-A", "JP-Z"}; !slices.Equal(got, want) {
		t.Fatalf("hit SKUs = %#v, want %#v", got, want)
	}
}

func TestSearchDoesNotMatchInsideSpaceDelimitedIndexTerm(t *testing.T) {
	service := newTestService(t)
	service.products = []PreparedProduct{{SKU: "JP-INSIDE", IndexText: "長期保存"}}

	result, err := service.Search(SearchRequest{Query: "保存"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := hitSKUs(result.Hits); len(got) != 0 {
		t.Fatalf("hit SKUs = %#v, want no substring match", got)
	}
}

func TestSearchKeepsMatcherStateLocalToEachCall(t *testing.T) {
	service := newTestService(t)
	tests := []struct {
		query string
		want  []string
	}{
		{query: "保存 容器", want: []string{"JP-200"}},
		{query: "自転車", want: []string{"JP-300"}},
		{query: "宇宙船", want: []string{}},
	}

	var wait sync.WaitGroup
	for _, tt := range tests {
		tt := tt
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Search(SearchRequest{Query: tt.query})
			if err != nil {
				t.Errorf("Search(%q) error = %v", tt.query, err)
				return
			}
			if got := hitSKUs(result.Hits); !slices.Equal(got, tt.want) {
				t.Errorf("Search(%q) hit SKUs = %#v, want %#v", tt.query, got, tt.want)
			}
		}()
	}
	wait.Wait()
}

func TestProductsPreservesOwnedEmptySlices(t *testing.T) {
	products := newTestService(t).Products()
	for _, product := range products {
		if product.MaskMatches == nil || product.Tokens == nil || product.IndexTerms == nil {
			t.Fatalf("Products() slices for %q = mask matches %#v, tokens %#v, index terms %#v; want non-nil empties",
				product.SKU, product.MaskMatches, product.Tokens, product.IndexTerms)
		}
	}
}

func TestProductsReturnsDeepCopy(t *testing.T) {
	service := newTestService(t)
	service.products = []PreparedProduct{{
		SKU:         "JP-COPY",
		MaskMatches: []MaskMatch{{ID: "unsafe", Text: "偽物", Start: 0, End: 6}},
		IndexTerms:  []string{"保存"},
		Tokens: []PreparedToken{{
			Text:     "保存",
			Metadata: map[string]string{japanese.MetadataPOS: "名詞/一般/*/*"},
		}},
	}}

	first := service.Products()
	first[0].MaskMatches[0].Text = "mutated"
	first[0].IndexTerms[0] = "mutated"
	first[0].Tokens[0].Text = "mutated"
	first[0].Tokens[0].Metadata[japanese.MetadataPOS] = "mutated"

	second := service.Products()
	if second[0].MaskMatches[0].Text == "mutated" ||
		second[0].IndexTerms[0] == "mutated" ||
		second[0].Tokens[0].Text == "mutated" ||
		second[0].Tokens[0].Metadata[japanese.MetadataPOS] == "mutated" {
		t.Fatalf("Products() exposed shared state: %#v", second[0])
	}
}

func TestServiceSharedReuseUnderBoundedConcurrency(t *testing.T) {
	service := newTestService(t)
	fixtures := []struct {
		query string
		want  []string
	}{
		{query: "ランニング シューズ", want: []string{"JP-100"}},
		{query: "保存 容器", want: []string{"JP-200"}},
		{query: "宇宙船", want: []string{}},
	}
	tasks := make([]concurrencytest.Task, 6)
	var arrivalMu sync.Mutex
	arrived := 0
	release := make(chan struct{})
	for i := range tasks {
		fixture := fixtures[i%len(fixtures)]
		arrival := &sync.Once{}
		tasks[i] = func(ctx context.Context) error {
			arrival.Do(func() {
				arrivalMu.Lock()
				defer arrivalMu.Unlock()
				arrived++
				if arrived == len(tasks) {
					close(release)
				}
			})
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-release:
			}
			result, err := service.Search(SearchRequest{Query: fixture.query})
			if err != nil {
				return err
			}
			if got := hitSKUs(result.Hits); !slices.Equal(got, fixture.want) {
				return fmt.Errorf("Search(%q) hit SKUs = %#v, want %#v", fixture.query, got, fixture.want)
			}
			return validateProducts(service.Products())
		}
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       6,
		RoundsPerTask: 3,
		Timeout:       5 * time.Second,
	})
	report := tester.RunT(t, tasks...)
	if report.Completed != 18 || report.MaxConcurrent < 2 {
		t.Fatalf("stress report = %+v, want 18 completions and concurrent execution", report)
	}
	t.Logf("stress report: Completed=%d MaxConcurrent=%d", report.Completed, report.MaxConcurrent)
}

func TestNewPreviewDocumentsScenario(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.Tokenizer != "kagome-ipa-search" {
		t.Fatalf("Tokenizer = %q, want kagome-ipa-search", preview.Tokenizer)
	}
	if len(preview.Products) != 3 || len(preview.Searches) != 3 {
		t.Fatalf("preview counts = %d products and %d searches, want 3 and 3", len(preview.Products), len(preview.Searches))
	}
	wantQueries := []string{"ランニング シューズ", "保存 容器", "宇宙船"}
	wantHits := [][]string{{"JP-100"}, {"JP-200"}, {}}
	for i, search := range preview.Searches {
		if search.Query != wantQueries[i] || !slices.Equal(hitSKUs(search.Hits), wantHits[i]) {
			t.Fatalf("Searches[%d] = %+v, want query %q and hits %#v", i, search, wantQueries[i], wantHits[i])
		}
		if search.QueryTerms == nil || search.Hits == nil {
			t.Fatalf("Searches[%d] has nil JSON slices: %+v", i, search)
		}
	}
	maskedFixtures := 0
	for _, product := range preview.Products {
		if product.MaskedSupportText != product.SupportText {
			maskedFixtures++
		}
		if product.MaskMatches == nil || product.Tokens == nil || product.IndexTerms == nil {
			t.Fatalf("product %q has nil JSON slices: %+v", product.SKU, product)
		}
	}
	if maskedFixtures != 1 {
		t.Fatalf("masked fixture count = %d, want 1", maskedFixtures)
	}
	if got := productBySKU(t, preview.Products, "JP-200").MaskedSupportText; got != "電子レンジで温めて使用できます。**に注意してください。" {
		t.Fatalf("JP-200 MaskedSupportText = %q", got)
	}
	if preview.LifecycleNotes == nil || preview.BoundaryNotes == nil {
		t.Fatalf("preview note slices must be non-nil: %+v", preview)
	}
	if len(preview.LifecycleNotes) == 0 || len(preview.BoundaryNotes) == 0 {
		t.Fatalf("preview notes are incomplete: %+v", preview)
	}
	wantCommands := []string{
		"go test -count=1 ./examples/japanese-search-preparation/...",
		"go test -race -count=1 ./examples/japanese-search-preparation/...",
	}
	if !slices.Equal(preview.Commands, wantCommands) {
		t.Fatalf("Commands = %#v, want %#v", preview.Commands, wantCommands)
	}
}

func validateProducts(products []PreparedProduct) error {
	if len(products) != 3 {
		return fmt.Errorf("product count = %d, want 3", len(products))
	}
	if products[1].SKU != "JP-200" || products[1].MaskedSupportText != "電子レンジで温めて使用できます。**に注意してください。" {
		return fmt.Errorf("JP-200 product = %+v, want exact masked support text", products[1])
	}
	for _, product := range products {
		for _, token := range product.Tokens {
			var source string
			switch token.Field {
			case FieldTitle:
				source = product.Title
			case FieldSupportText:
				source = product.SupportText
			default:
				return fmt.Errorf("token %q has unexpected field %q", token.Text, token.Field)
			}
			if token.Start < 0 || token.End > len(source) || token.Start >= token.End {
				return fmt.Errorf("token %q has invalid %s source span %d:%d", token.Text, token.Field, token.Start, token.End)
			}
			if got := source[token.Start:token.End]; got != token.Text {
				return fmt.Errorf("token %q span slices %q from %s", token.Text, got, token.Field)
			}
		}
	}
	return nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService(DefaultProducts(), DefaultMaskPolicy())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func productBySKU(t *testing.T, products []PreparedProduct, sku string) PreparedProduct {
	t.Helper()
	for _, product := range products {
		if product.SKU == sku {
			return product
		}
	}
	t.Fatalf("product %q not found in %#v", sku, products)
	return PreparedProduct{}
}

func hitSKUs(hits []SearchHit) []string {
	skus := make([]string, len(hits))
	for i, hit := range hits {
		skus[i] = hit.SKU
	}
	return skus
}
