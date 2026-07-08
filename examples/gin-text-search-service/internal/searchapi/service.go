// Package searchapi exposes a thin Gin boundary over deterministic textsearch.
package searchapi

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/gin-gonic/gin"
)

const maxSearchTextRunes = 8_000

var (
	// ErrInvalidRequest reports malformed HTTP or domain input.
	ErrInvalidRequest = errors.New("searchapi: invalid request")
)

// Policy describes the caller-owned text search dictionary.
type Policy struct {
	Patterns      []PolicyPattern
	NormalizeMode textsearch.NormalizeMode
	Boundary      textsearch.BoundaryMode
	Overlap       textsearch.OverlapMode
}

// PolicyPattern is one configured exact-search phrase.
type PolicyPattern struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category"`
}

// Service owns reusable search and masking behavior without a Gin dependency.
type Service struct {
	matcher  *textsearch.Matcher
	patterns map[string]PolicyPattern
}

// Server exposes the search service through Gin.
type Server struct {
	router  *gin.Engine
	service *Service
}

// SearchMaskRequest is the POST /text/search-mask request body.
type SearchMaskRequest struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text"`
	Mask      string `json:"mask,omitempty"`
}

// SearchMaskResult is the stable API response for one search request.
type SearchMaskResult struct {
	RequestID      string        `json:"request_id"`
	OriginalText   string        `json:"original_text"`
	MaskedText     string        `json:"masked_text"`
	Matches        []SearchMatch `json:"matches"`
	Summary        SearchSummary `json:"summary"`
	UnicodeCaveats []string      `json:"unicode_caveats"`
	Commands       []string      `json:"commands,omitempty"`
}

// SearchMatch describes one exact phrase occurrence in the original text.
type SearchMatch struct {
	PatternID string `json:"pattern_id"`
	Pattern   string `json:"pattern"`
	Text      string `json:"text"`
	Category  string `json:"category"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
}

// SearchSummary keeps count fields explicit for HTTP clients.
type SearchSummary struct {
	TotalMatches int `json:"total_matches"`
	PatternsHit  int `json:"patterns_hit"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Preview is printed by the command and documented in the README.
type Preview struct {
	Scenario       string           `json:"scenario"`
	Endpoint       string           `json:"endpoint"`
	HTTPBoundary   []string         `json:"http_boundary"`
	DomainBoundary []string         `json:"domain_boundary"`
	Policy         []PolicyPattern  `json:"policy"`
	Sample         SearchMaskResult `json:"sample"`
	CurlExamples   []string         `json:"curl_examples"`
	UnicodeCaveats []string         `json:"unicode_caveats"`
	TestCommands   []string         `json:"test_commands"`
}

// DefaultPolicy returns the static workshop search policy.
func DefaultPolicy() Policy {
	return Policy{
		Patterns: []PolicyPattern{
			{ID: "promo-coupon", Text: "쿠폰", Category: "promotion"},
			{ID: "promo-sale", Text: "sale", Category: "promotion"},
			{ID: "policy-refund", Text: "refund", Category: "support"},
			{ID: "policy-refund-window", Text: "refund window", Category: "support"},
			{ID: "support-bad-wolf", Text: "bad wolf", Category: "support"},
		},
		NormalizeMode: textsearch.NormalizeNFC,
		Boundary:      textsearch.BoundaryUnicodeWord,
		Overlap:       textsearch.OverlapLeftmostLongest,
	}
}

// NewService compiles a policy into a reusable search service.
func NewService(policy Policy) (*Service, error) {
	patterns := make([]textsearch.Pattern, len(policy.Patterns))
	lookup := make(map[string]PolicyPattern, len(policy.Patterns))
	for i, pattern := range policy.Patterns {
		if strings.TrimSpace(pattern.ID) == "" || strings.TrimSpace(pattern.Text) == "" {
			return nil, fmt.Errorf("%w: pattern id and text are required", ErrInvalidRequest)
		}
		patterns[i] = textsearch.Pattern{ID: pattern.ID, Text: pattern.Text}
		lookup[pattern.ID] = pattern
	}
	matcher, err := textsearch.Compile(patterns, textsearch.Config{
		IgnoreCase: true,
		Normalize:  policy.NormalizeMode,
		Boundary:   policy.Boundary,
		Overlap:    policy.Overlap,
	})
	if err != nil {
		return nil, fmt.Errorf("compile search policy: %w", err)
	}
	return &Service{matcher: matcher, patterns: lookup}, nil
}

// SearchMask searches configured phrases and masks the exact accepted spans.
func (s *Service) SearchMask(request SearchMaskRequest) (SearchMaskResult, error) {
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" {
		return SearchMaskResult{}, fmt.Errorf("%w: request_id is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Text) == "" {
		return SearchMaskResult{}, fmt.Errorf("%w: text is required", ErrInvalidRequest)
	}
	if utf8.RuneCountInString(request.Text) > maxSearchTextRunes {
		return SearchMaskResult{}, fmt.Errorf("%w: text length exceeds %d runes", ErrInvalidRequest, maxSearchTextRunes)
	}
	mask := request.Mask
	if mask == "" {
		mask = "*"
	}
	if utf8.RuneCountInString(mask) != 1 {
		return SearchMaskResult{}, fmt.Errorf("%w: mask must be exactly one rune", ErrInvalidRequest)
	}

	matches := searchMatchesFromTextsearch(s.matcher.FindAll(request.Text), s.patterns)
	return SearchMaskResult{
		RequestID:      request.RequestID,
		OriginalText:   request.Text,
		MaskedText:     maskMatches(request.Text, matches, mask),
		Matches:        matches,
		Summary:        summarize(matches),
		UnicodeCaveats: unicodeCaveats(),
	}, nil
}

// NewServer creates the Gin HTTP boundary for the search service.
func NewServer(service *Service) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("%w: service is required", ErrInvalidRequest)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	server := &Server{router: router, service: service}
	router.GET("/healthz", server.health)
	router.POST("/text/search-mask", server.searchMask)
	return server, nil
}

// ServeHTTP dispatches requests to Gin.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// NewPreview returns the no-server example payload printed by main.
func NewPreview() (Preview, error) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		return Preview{}, err
	}
	sample, err := service.SearchMask(SearchMaskRequest{
		RequestID: "search-preview",
		Text:      "sale 쿠폰 refundable refund window bad wolf",
	})
	if err != nil {
		return Preview{}, err
	}
	sample.Commands = []string{
		"go run ./examples/gin-text-search-service",
		"go test -count=1 ./examples/gin-text-search-service/...",
	}
	return Preview{
		Scenario: "Expose deterministic textsearch matching and masking through a thin Gin API.",
		Endpoint: "POST /text/search-mask",
		HTTPBoundary: []string{
			"Gin owns JSON binding, response status, and public error shape.",
			"Handlers delegate search and masking to the reusable Service.",
		},
		DomainBoundary: []string{
			"Service owns policy compilation, Unicode boundary matching, overlap handling, and exact-span masking.",
			"Service has no Gin dependency and can be reused by later workflow examples.",
		},
		Policy:         DefaultPolicy().Patterns,
		Sample:         sample,
		CurlExamples:   curlExamples(),
		UnicodeCaveats: unicodeCaveats(),
		TestCommands: []string{
			"go test -count=1 ./examples/gin-text-search-service/...",
			"go test -race -count=1 ./examples/gin-text-search-service/...",
		},
	}, nil
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) searchMask(c *gin.Context) {
	var request SearchMaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	result, err := s.service.SearchMask(request)
	if err != nil {
		writeSearchError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func writeSearchError(c *gin.Context, err error) {
	if errors.Is(err, ErrInvalidRequest) {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	writeError(c, http.StatusInternalServerError, "search_failed", err)
}

func writeError(c *gin.Context, status int, code string, err error) {
	c.JSON(status, ErrorResponse{Code: code, Message: err.Error()})
}

func searchMatchesFromTextsearch(matches []textsearch.Match, lookup map[string]PolicyPattern) []SearchMatch {
	results := make([]SearchMatch, len(matches))
	for i, match := range matches {
		pattern := lookup[match.Pattern.ID]
		results[i] = SearchMatch{
			PatternID: pattern.ID,
			Pattern:   pattern.Text,
			Text:      match.Text,
			Category:  pattern.Category,
			Start:     match.Start,
			End:       match.End,
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Start != results[j].Start {
			return results[i].Start < results[j].Start
		}
		return results[i].End > results[j].End
	})
	return results
}

func maskMatches(input string, matches []SearchMatch, mask string) string {
	if len(matches) == 0 {
		return input
	}
	var builder strings.Builder
	builder.Grow(len(input))
	offset := 0
	for _, match := range matches {
		builder.WriteString(input[offset:match.Start])
		builder.WriteString(strings.Repeat(mask, utf8.RuneCountInString(match.Text)))
		offset = match.End
	}
	builder.WriteString(input[offset:])
	return builder.String()
}

func summarize(matches []SearchMatch) SearchSummary {
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		seen[match.PatternID] = struct{}{}
	}
	return SearchSummary{TotalMatches: len(matches), PatternsHit: len(seen)}
}

func unicodeCaveats() []string {
	return []string{
		"BoundaryUnicodeWord prevents substring matches such as sale inside presale or refund inside refundable.",
		"Offsets are byte spans in the original UTF-8 string; UI clients should not treat them as rune indexes.",
		"NFC normalization is enabled, but locale-specific tokenization and language detection are separate examples.",
	}
}

func curlExamples() []string {
	return []string{
		`curl -s -X POST http://127.0.0.1:8098/text/search-mask -H 'Content-Type: application/json' -d '{"request_id":"search-1001","text":"sale 쿠폰 refundable refund window bad wolf"}' | jq`,
		`curl -s -X POST http://127.0.0.1:8098/text/search-mask -H 'Content-Type: application/json' -d '{"request_id":"search-1002","text":"presale refund refundable","mask":"#"}' | jq`,
	}
}
