package catalogprep

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
)

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
