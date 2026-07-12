package catalogprep

import (
	"errors"
	"fmt"
	"maps"
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
	for _, text := range []string{input.Title, input.SupportText} {
		if _, err := textsearch.NewTokenizeRequest(text, textsearch.TokenizeOptions{
			Normalize: textsearch.NormalizeNFC,
		}); err != nil {
			return PreparedProduct{}, err
		}
	}
	return PreparedProduct{
		SKU:               input.SKU,
		Title:             input.Title,
		SupportText:       input.SupportText,
		MaskedSupportText: input.SupportText,
		MaskMatches:       []MaskMatch{},
		Tokens:            []PreparedToken{},
		IndexTerms:        []string{},
		IndexText:         "",
	}, nil
}

func DefaultProducts() []ProductInput {
	return []ProductInput{
		{
			SKU:         "JP-100",
			Title:       "ランニングシューズ",
			SupportText: "毎日の運動に使えます。",
		},
		{
			SKU:         "JP-200",
			Title:       "ガラス保存容器",
			SupportText: "食品を保存できます。偽物にはご注意ください。",
		},
		{
			SKU:         "JP-300",
			Title:       "折りたたみ自転車",
			SupportText: "持ち運びやすい折りたたみ式です。",
		},
	}
}

func DefaultMaskPolicy() MaskPolicy {
	return MaskPolicy{
		Entries: []textsearch.BlockwordEntry{{ID: "unsafe", Text: "偽物"}},
		Mask:    "*",
	}
}
