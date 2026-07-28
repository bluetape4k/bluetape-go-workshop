// Package searchapi 는 deterministic textsearch 위에 얇은 Gin boundary를 노출한다.
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
	// ErrInvalidRequest 는 형식이 잘못된 HTTP 또는 domain 입력을 나타낸다.
	ErrInvalidRequest = errors.New("searchapi: invalid request")
)

// Policy 는 호출자가 소유하는 text search dictionary를 설명한다.
type Policy struct {
	Patterns      []PolicyPattern
	NormalizeMode textsearch.NormalizeMode
	Boundary      textsearch.BoundaryMode
	Overlap       textsearch.OverlapMode
}

// PolicyPattern 은 설정된 exact-search phrase 하나다.
type PolicyPattern struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category"`
}

// Service 는 Gin 의존성 없이 재사용 가능한 search와 masking 동작을 소유한다.
type Service struct {
	matcher  *textsearch.Matcher
	patterns map[string]PolicyPattern
}

// Server 는 Gin을 통해 search service를 노출한다.
type Server struct {
	router  *gin.Engine
	service *Service
}

// SearchMaskRequest 는 POST /text/search-mask 요청 본문이다.
type SearchMaskRequest struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text"`
	Mask      string `json:"mask,omitempty"`
}

// SearchMaskResult 는 search 요청 하나에 대한 안정적인 API 응답이다.
type SearchMaskResult struct {
	RequestID      string        `json:"request_id"`
	OriginalText   string        `json:"original_text"`
	MaskedText     string        `json:"masked_text"`
	Matches        []SearchMatch `json:"matches"`
	Summary        SearchSummary `json:"summary"`
	UnicodeCaveats []string      `json:"unicode_caveats"`
	Commands       []string      `json:"commands,omitempty"`
}

// SearchMatch 는 원본 text에서 exact phrase occurrence 하나를 설명한다.
type SearchMatch struct {
	PatternID string `json:"pattern_id"`
	Pattern   string `json:"pattern"`
	Text      string `json:"text"`
	Category  string `json:"category"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
}

// SearchSummary 는 HTTP client가 count field를 명확히 읽도록 유지한다.
type SearchSummary struct {
	TotalMatches int `json:"total_matches"`
	PatternsHit  int `json:"patterns_hit"`
}

// ErrorResponse 는 안정적인 공개 오류 형식이다.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Preview 는 command가 출력하고 README에 문서화되는 값이다.
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

// DefaultPolicy 는 정적 워크숍 search policy를 반환한다.
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

// NewService 는 policy를 재사용 가능한 search service로 compile한다.
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

// SearchMask 는 설정된 phrase를 검색하고 정확히 허용된 span을 masking한다.
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

// NewServer 는 search service용 Gin HTTP boundary를 만든다.
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

// ServeHTTP 는 요청을 Gin으로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// NewPreview 는 main이 출력하는 no-server example payload를 반환한다.
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
