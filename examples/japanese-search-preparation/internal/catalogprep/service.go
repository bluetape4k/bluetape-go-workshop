// Package catalogprep prepares a Japanese product catalog for deterministic search and masking.
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
	// ErrInvalidService reports an uninitialized service receiver.
	ErrInvalidService = errors.New("catalogprep: invalid service")
	// ErrInvalidProduct reports missing or invalid product input.
	ErrInvalidProduct = errors.New("catalogprep: invalid product")
	// ErrInvalidQuery reports a query that cannot produce searchable terms.
	ErrInvalidQuery = errors.New("catalogprep: invalid query")
)

// ProductInput contains the source fields prepared for catalog search.
type ProductInput struct {
	SKU         string `json:"sku"`
	Title       string `json:"title"`
	SupportText string `json:"support_text"`
}

// MaskPolicy configures blockword matching and replacement text.
type MaskPolicy struct {
	Entries []textsearch.BlockwordEntry
	Mask    string
}

// Field identifies the product field that produced a token.
type Field string

const (
	// FieldTitle identifies text from a product title.
	FieldTitle Field = "title"
	// FieldSupportText identifies text from product support content.
	FieldSupportText Field = "support_text"
)

// PreparedToken records a normalized token and its original byte span.
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

// MaskMatch records a blockword match in the original support text.
type MaskMatch struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// PreparedProduct contains source text, masked text, tokens, and index terms.
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

// SearchRequest contains a query to prepare and match against the catalog.
type SearchRequest struct {
	Query string `json:"query"`
}

// SearchHit identifies a matching product and the terms matched for it.
type SearchHit struct {
	SKU          string   `json:"sku"`
	MatchedTerms []string `json:"matched_terms"`
}

// SearchResult contains prepared query terms and matching products.
type SearchResult struct {
	Query      string      `json:"query"`
	QueryTerms []string    `json:"query_terms"`
	Hits       []SearchHit `json:"hits"`
}

// Preview contains the deterministic example payload and operating notes.
type Preview struct {
	Scenario       string            `json:"scenario"`
	Tokenizer      string            `json:"tokenizer"`
	LifecycleNotes []string          `json:"lifecycle_notes"`
	BoundaryNotes  []string          `json:"boundary_notes"`
	Products       []PreparedProduct `json:"products"`
	Searches       []SearchResult    `json:"searches"`
	Commands       []string          `json:"commands"`
}

// Service prepares products and searches a reusable Japanese-tokenized catalog.
type Service struct {
	tokenizer  *japanese.Tokenizer
	dictionary *textsearch.BlockwordDictionary
	mask       string
	products   []PreparedProduct
}

// NewPreview builds the deterministic catalog preparation payload printed by the example command.
func NewPreview() (Preview, error) {
	service, err := NewService(DefaultProducts(), DefaultMaskPolicy())
	if err != nil {
		return Preview{}, fmt.Errorf("build catalog preparation service: %w", err)
	}

	queries := []string{"ランニング シューズ", "保存 容器", "宇宙船"}
	searches := make([]SearchResult, len(queries))
	for i, query := range queries {
		result, err := service.Search(SearchRequest{Query: query})
		if err != nil {
			return Preview{}, fmt.Errorf("search preview query %q: %w", query, err)
		}
		searches[i] = result
	}

	return Preview{
		Scenario:  "prepare a Japanese product catalog for deterministic term search and masking",
		Tokenizer: "kagome-ipa-search",
		LifecycleNotes: []string{
			"Construct one service at application startup and reuse its Kagome tokenizer and compiled blockword dictionary across requests.",
			"The Kagome IPA dictionary has a binary and memory footprint, so tokenizer construction belongs outside the query path.",
		},
		BoundaryNotes: []string{
			"NFC normalization prepares comparison terms while token spans remain UTF-8 byte offsets into each original title or support-text field.",
			"Blockword masking uses substring matching in Japanese support text; it is not a semantic moderation or security boundary.",
			"Search uses Unicode word boundaries over the space-delimited prepared index so a query term does not match inside another term.",
		},
		Products: service.Products(),
		Searches: searches,
		Commands: []string{
			"go test -count=1 ./examples/japanese-search-preparation/...",
			"go test -race -count=1 ./examples/japanese-search-preparation/...",
		},
	}, nil
}

// NewService validates and prepares products with a reusable tokenizer and mask policy.
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

// Products returns a deep copy of the prepared catalog.
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

// Search returns products containing every prepared query term.
func (s *Service) Search(request SearchRequest) (SearchResult, error) {
	if !s.valid() {
		return SearchResult{}, ErrInvalidService
	}
	if strings.TrimSpace(request.Query) == "" {
		return SearchResult{}, fmt.Errorf("%w: query is required", ErrInvalidQuery)
	}

	tokens, err := s.prepareField("", request.Query)
	if err != nil {
		return SearchResult{}, fmt.Errorf("%w: prepare query: %w", ErrInvalidQuery, err)
	}
	queryTerms := uniqueIndexTerms(tokens)
	if len(queryTerms) == 0 {
		return SearchResult{}, fmt.Errorf("%w: query has no indexable terms", ErrInvalidQuery)
	}

	patterns := make([]textsearch.Pattern, len(queryTerms))
	for i, term := range queryTerms {
		patterns[i] = textsearch.Pattern{ID: term, Text: term}
	}
	matcher, err := textsearch.Compile(patterns, textsearch.Config{
		Normalize: textsearch.NormalizeNFC,
		Boundary:  textsearch.BoundaryUnicodeWord,
		Overlap:   textsearch.OverlapLeftmostLongest,
	})
	if err != nil {
		return SearchResult{}, fmt.Errorf("%w: compile query matcher: %w", ErrInvalidQuery, err)
	}

	hits := make([]SearchHit, 0)
	for _, product := range s.products {
		matched := make(map[string]struct{}, len(queryTerms))
		for _, match := range matcher.FindAll(product.IndexText) {
			matched[match.Pattern.ID] = struct{}{}
		}
		if len(matched) != len(queryTerms) {
			continue
		}
		hits = append(hits, SearchHit{
			SKU:          product.SKU,
			MatchedTerms: slices.Clone(queryTerms),
		})
	}
	slices.SortFunc(hits, func(left, right SearchHit) int {
		return strings.Compare(left.SKU, right.SKU)
	})

	return SearchResult{
		Query:      request.Query,
		QueryTerms: queryTerms,
		Hits:       hits,
	}, nil
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
	indexTerms := uniqueIndexTerms(tokens)

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

func uniqueIndexTerms(tokens []PreparedToken) []string {
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
	return indexTerms
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

// DefaultProducts returns the deterministic product fixtures used by the example.
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

// DefaultMaskPolicy returns the blockword policy used by the example.
func DefaultMaskPolicy() MaskPolicy {
	return MaskPolicy{
		Entries: []textsearch.BlockwordEntry{{ID: "counterfeit", Text: "偽物", Severity: textsearch.SeverityHigh}},
		Mask:    "*",
	}
}
