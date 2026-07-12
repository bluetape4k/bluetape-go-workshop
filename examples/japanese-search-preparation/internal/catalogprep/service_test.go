package catalogprep

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
)

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
