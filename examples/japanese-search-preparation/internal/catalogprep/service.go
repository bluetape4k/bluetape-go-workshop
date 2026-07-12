package catalogprep

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
)

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

type SearchRequest struct {
	Query string `json:"query"`
}

type SearchHit struct {
	SKU          string   `json:"sku"`
	MatchedTerms []string `json:"matched_terms"`
}

type SearchResult struct {
	Query      string      `json:"query"`
	QueryTerms []string    `json:"query_terms"`
	Hits       []SearchHit `json:"hits"`
}

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

	service := &Service{
		tokenizer:  tokenizer,
		dictionary: dictionary,
		mask:       policy.Mask,
		products:   make([]PreparedProduct, 0, len(products)),
	}
	for _, product := range products {
		if strings.TrimSpace(product.SKU) == "" ||
			strings.TrimSpace(product.Title) == "" ||
			strings.TrimSpace(product.SupportText) == "" {
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

func (s *Service) Products() []PreparedProduct {
	if !s.valid() {
		return nil
	}

	copied := make([]PreparedProduct, len(s.products))
	for i, product := range s.products {
		copied[i] = product
		copied[i].MaskMatches = slices.Clone(product.MaskMatches)
		copied[i].IndexTerms = slices.Clone(product.IndexTerms)
		copied[i].Tokens = slices.Clone(product.Tokens)
		for j, token := range product.Tokens {
			copied[i].Tokens[j].Metadata = maps.Clone(token.Metadata)
		}
	}
	return copied
}

func (s *Service) Search(SearchRequest) (SearchResult, error) {
	if !s.valid() {
		return SearchResult{}, ErrInvalidService
	}
	return SearchResult{}, nil
}

func (s *Service) valid() bool {
	return s != nil && s.tokenizer != nil && s.dictionary != nil
}

func (s *Service) prepareProduct(input ProductInput) (PreparedProduct, error) {
	titleTokens, err := s.prepareField(FieldTitle, input.Title)
	if err != nil {
		return PreparedProduct{}, err
	}
	supportTokens, err := s.prepareField(FieldSupportText, input.SupportText)
	if err != nil {
		return PreparedProduct{}, err
	}

	maskResponse, err := s.dictionary.Process(textsearch.BlockwordRequest{
		Text:    input.SupportText,
		Options: textsearch.BlockwordOptions{Mask: s.mask},
	})
	if err != nil {
		return PreparedProduct{}, err
	}
	maskMatches := make([]MaskMatch, len(maskResponse.Matches))
	for i, match := range maskResponse.Matches {
		maskMatches[i] = MaskMatch{
			ID:    match.Entry.ID,
			Text:  match.Text,
			Start: match.Start,
			End:   match.End,
		}
	}
	excludeOverlappingTokens(supportTokens, maskMatches)

	tokens := make([]PreparedToken, 0, len(titleTokens)+len(supportTokens))
	tokens = append(tokens, titleTokens...)
	tokens = append(tokens, supportTokens...)
	indexTerms := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if !token.Indexable {
			continue
		}
		term := indexTerm(token)
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		indexTerms = append(indexTerms, term)
	}

	return PreparedProduct{
		SKU:               input.SKU,
		Title:             input.Title,
		SupportText:       input.SupportText,
		MaskedSupportText: maskResponse.MaskedText,
		MaskMatches:       maskMatches,
		Tokens:            tokens,
		IndexTerms:        indexTerms,
		IndexText:         strings.Join(indexTerms, " "),
	}, nil
}

func (s *Service) prepareField(field Field, input string) ([]PreparedToken, error) {
	request, err := textsearch.NewTokenizeRequest(input, textsearch.TokenizeOptions{
		Normalize: textsearch.NormalizeNFC,
	})
	if err != nil {
		return nil, err
	}
	response, err := s.tokenizer.Tokenize(request)
	if err != nil {
		return nil, err
	}
	selected := japanese.Filter(response.Tokens, func(token textsearch.Token) bool {
		return japanese.IsNoun(token) || japanese.IsVerb(token)
	})
	prepared := make([]PreparedToken, len(selected))
	for i, token := range selected {
		prepared[i] = PreparedToken{
			Field:      field,
			Text:       token.Text,
			Normalized: token.Normalized,
			BaseForm:   token.Metadata[japanese.MetadataBaseForm],
			POS:        string(token.POS),
			Start:      token.Span.Start,
			End:        token.Span.End,
			Indexable:  true,
			Metadata:   maps.Clone(token.Metadata),
		}
	}
	return prepared, nil
}

func indexTerm(token PreparedToken) string {
	term := token.BaseForm
	if term == "" || term == "*" {
		term = token.Text
	}
	return textsearch.NormalizeText(term, textsearch.NormalizeNFC).Normalized
}

func byteSpansOverlap(firstStart, firstEnd, secondStart, secondEnd int) bool {
	return firstStart < secondEnd && secondStart < firstEnd
}

// excludeOverlappingTokens sweeps tokens and matches ordered by byte position.
func excludeOverlappingTokens(tokens []PreparedToken, matches []MaskMatch) {
	matchIndex := 0
	for i := range tokens {
		for matchIndex < len(matches) && matches[matchIndex].End <= tokens[i].Start {
			matchIndex++
		}
		if matchIndex == len(matches) {
			return
		}
		match := matches[matchIndex]
		if byteSpansOverlap(tokens[i].Start, tokens[i].End, match.Start, match.End) {
			tokens[i].Indexable = false
		}
	}
}

func DefaultProducts() []ProductInput {
	return []ProductInput{
		{
			SKU:         "JP-100",
			Title:       "軽量ランニングシューズ",
			SupportText: "毎日のランニングを快適にします。",
		},
		{
			SKU:         "JP-200",
			Title:       "ガラス保存容器",
			SupportText: "電子レンジで温めて使用できます。偽物に注意してください。",
		},
		{
			SKU:         "JP-300",
			Title:       "折りたたみ自転車",
			SupportText: "通勤で便利に走れます。",
		},
	}
}

func DefaultMaskPolicy() MaskPolicy {
	return MaskPolicy{
		Entries: []textsearch.BlockwordEntry{{ID: "counterfeit", Text: "偽物", Severity: textsearch.SeverityHigh}},
		Mask:    "*",
	}
}
