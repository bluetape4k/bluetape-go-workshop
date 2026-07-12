package moderationapi

import (
	"context"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

const defaultSearchLimit = 20

var contentIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ServiceOption customizes service-owned infrastructure.
type ServiceOption func(*serviceOptions) error

type serviceOptions struct {
	clock func() time.Time
}

// WithClock injects the non-blocking clock used while committing records.
func WithClock(clock func() time.Time) ServiceOption {
	return func(options *serviceOptions) error {
		if clock == nil {
			return fmt.Errorf("%w: clock is nil", ErrInvalidConfig)
		}
		options.clock = clock
		return nil
	}
}

// Service reuses immutable text processors and owns a bounded in-memory store.
type Service struct {
	config            ServiceConfig
	detector          *language.Detector
	japaneseTokenizer *japanese.Tokenizer
	simpleTokenizer   textsearch.Tokenizer
	dictionary        *textsearch.BlockwordDictionary
	blockwordOptions  textsearch.BlockwordOptions
	clock             func() time.Time

	mu      sync.RWMutex
	records map[string]*Record
}

// NewService validates configuration and builds application-owned processors.
func NewService(config ServiceConfig, options ...ServiceOption) (*Service, error) {
	if err := validateServiceConfig(config); err != nil {
		return nil, err
	}

	configured := serviceOptions{clock: time.Now}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: service option is nil", ErrInvalidConfig)
		}
		if err := option(&configured); err != nil {
			return nil, err
		}
	}

	detector, err := language.NewDetector([]language.Language{
		language.English,
		language.Korean,
		language.Japanese,
		language.Chinese,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: construct language detector: %w", ErrWorkflow, err)
	}
	japaneseTokenizer, err := japanese.NewTokenizer(japanese.WithMode(japanese.Search))
	if err != nil {
		return nil, fmt.Errorf("%w: construct Japanese tokenizer: %w", ErrWorkflow, err)
	}
	dictionary, err := textsearch.NewBlockwordDictionary(defaultBlockwordEntries(), textsearch.Config{
		IgnoreCase: true,
		Normalize:  textsearch.NormalizeNFC,
		Boundary:   textsearch.BoundaryUnicodeWord,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: construct moderation dictionary: %w", ErrWorkflow, err)
	}

	return &Service{
		config:            config,
		detector:          detector,
		japaneseTokenizer: japaneseTokenizer,
		simpleTokenizer:   textsearch.NewSimpleTokenizer(),
		dictionary:        dictionary,
		blockwordOptions: textsearch.BlockwordOptions{
			Mask:        "*",
			MinSeverity: textsearch.SeverityMiddle,
		},
		clock:   configured.clock,
		records: make(map[string]*Record, config.MaximumRecords),
	}, nil
}

func validateServiceConfig(config ServiceConfig) error {
	if math.IsNaN(config.MinimumConfidence) || math.IsInf(config.MinimumConfidence, 0) ||
		config.MinimumConfidence < 0 || config.MinimumConfidence > 1 {
		return fmt.Errorf("%w: minimum confidence must be finite and between zero and one", ErrInvalidConfig)
	}
	if config.MinimumRunes <= 0 || config.MaximumContentRunes <= 0 || config.MaximumRecords <= 0 {
		return fmt.Errorf("%w: service limits must be positive", ErrInvalidConfig)
	}
	if config.MaximumSearchResults < defaultSearchLimit {
		return fmt.Errorf("%w: maximum search results must be at least %d", ErrInvalidConfig, defaultSearchLimit)
	}
	return nil
}

func defaultBlockwordEntries() []textsearch.BlockwordEntry {
	return []textsearch.BlockwordEntry{
		{ID: "abuse-bad", Text: "bad", Severity: textsearch.SeverityMiddle, Metadata: map[string]string{"category": "abuse"}},
		{ID: "abuse-bad-wolf", Text: "bad wolf", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"category": "abuse"}},
		{ID: "fraud-scam", Text: "scam", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"category": "fraud"}},
		{ID: "ko-abuse", Text: "욕설", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"category": "abuse"}},
		{ID: "ko-fraud", Text: "무료 돈", Severity: textsearch.SeverityMiddle, Metadata: map[string]string{"category": "fraud"}},
	}
}

// Create routes, moderates, prepares, and atomically stores one content record.
func (s *Service) Create(ctx context.Context, request CreateRequest) (Record, error) {
	if !s.ready() {
		return Record{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	contentID, metadata, err := s.validateCreateRequest(request)
	if err != nil {
		return Record{}, err
	}
	record, detectedLanguage, manualReview, err := s.detectAndRoute(ctx, contentID, request.Content, metadata)
	if err != nil {
		return Record{}, err
	}
	if manualReview {
		record.DisplayText = ManualReviewDisplayText
	} else {
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		blockRequest, requestErr := textsearch.NewBlockwordRequest(request.Content, s.blockwordOptions)
		if requestErr != nil {
			return Record{}, fmt.Errorf("%w: prepare moderation request: %w", ErrWorkflow, requestErr)
		}
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		blockResponse, processErr := s.dictionary.Process(blockRequest)
		if processErr != nil {
			return Record{}, fmt.Errorf("%w: moderate content: %w", ErrWorkflow, processErr)
		}
		record.DisplayText = blockResponse.MaskedText
		record.Findings = projectFindings(blockResponse.Matches)
		if len(record.Findings) > 0 {
			record.Outcome = OutcomeMasked
		} else {
			record.Outcome = OutcomeAllowed
		}
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		if detectedLanguage == language.Japanese {
			record.Tokens, record.Terms, err = s.prepareJapaneseTerms(request.Content)
		} else {
			record.Terms, err = s.prepareSimpleTerms(request.Content)
		}
		if err != nil {
			return Record{}, fmt.Errorf("%w: prepare search terms: %w", ErrWorkflow, err)
		}
	}

	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	if _, exists := s.records[contentID]; exists {
		return Record{}, fmt.Errorf("%w: %s", ErrDuplicateContentID, contentID)
	}
	if len(s.records) >= s.config.MaximumRecords {
		return Record{}, ErrStoreCapacity
	}
	record.CreatedAt = s.clock().UTC()
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	stored := cloneRecord(record)
	s.records[contentID] = &stored
	return cloneRecord(stored), nil
}

// Get returns one record by canonical content ID.
func (s *Service) Get(ctx context.Context, contentID string) (Record, error) {
	if !s.ready() {
		return Record{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	contentID = strings.TrimSpace(contentID)
	if !contentIDPattern.MatchString(contentID) {
		return Record{}, fmt.Errorf("%w: content_id is not path safe", ErrInvalidRequest)
	}
	s.mu.RLock()
	record, exists := s.records[contentID]
	if !exists {
		s.mu.RUnlock()
		return Record{}, fmt.Errorf("%w: %s", ErrRecordNotFound, contentID)
	}
	result := cloneRecord(*record)
	s.mu.RUnlock()
	return result, nil
}

// Search returns accepted records matching all prepared terms and metadata filters.
func (s *Service) Search(ctx context.Context, request SearchRequest) (SearchResponse, error) {
	if !s.ready() {
		return SearchResponse{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	query, metadata, limit, afterContentID, err := s.validateSearchRequest(request)
	if err != nil {
		return SearchResponse{}, err
	}

	var terms []string
	if language.ContainsJapanese(query) {
		_, terms, err = s.prepareJapaneseTerms(query)
	} else {
		terms, err = s.prepareSimpleTerms(query)
	}
	if err != nil {
		return SearchResponse{}, fmt.Errorf("%w: prepare query terms: %w", ErrWorkflow, err)
	}
	if len(terms) == 0 {
		return SearchResponse{}, fmt.Errorf("%w: query has no searchable terms", ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}

	s.mu.RLock()
	snapshot := make([]*Record, 0, len(s.records))
	for _, record := range s.records {
		snapshot = append(snapshot, record)
	}
	s.mu.RUnlock()

	hits := make([]SearchHit, 0, min(limit+1, len(snapshot)))
	for i, record := range snapshot {
		if i%32 == 0 {
			if err := ctx.Err(); err != nil {
				return SearchResponse{}, err
			}
		}
		if record.ContentID <= afterContentID || (record.Outcome != OutcomeAllowed && record.Outcome != OutcomeMasked) {
			continue
		}
		if !metadataMatches(record.Metadata, metadata) || !containsAllTerms(record.Terms, terms) {
			continue
		}
		hits = append(hits, SearchHit{
			ContentID:   record.ContentID,
			Outcome:     record.Outcome,
			DisplayText: record.DisplayText,
			Metadata:    maps.Clone(record.Metadata),
			Language:    record.Language,
		})
	}
	slices.SortFunc(hits, func(left, right SearchHit) int {
		return strings.Compare(left.ContentID, right.ContentID)
	})

	response := SearchResponse{Hits: hits}
	if len(response.Hits) > limit {
		response.Hits = response.Hits[:limit]
		response.Truncated = true
		response.NextAfterContentID = response.Hits[len(response.Hits)-1].ContentID
	}
	return response, nil
}

func (s *Service) ready() bool {
	return s != nil && s.detector != nil && s.japaneseTokenizer != nil && s.simpleTokenizer != nil &&
		s.dictionary != nil && s.clock != nil && s.records != nil
}

func (s *Service) validateCreateRequest(request CreateRequest) (string, map[string]string, error) {
	contentID := strings.TrimSpace(request.ContentID)
	if !contentIDPattern.MatchString(contentID) {
		return "", nil, fmt.Errorf("%w: content_id is not path safe", ErrInvalidRequest)
	}
	if !utf8.ValidString(request.Content) || strings.TrimSpace(request.Content) == "" {
		return "", nil, fmt.Errorf("%w: content must be nonblank UTF-8", ErrInvalidRequest)
	}
	if utf8.RuneCountInString(request.Content) > s.config.MaximumContentRunes {
		return "", nil, fmt.Errorf("%w: content exceeds %d runes", ErrInvalidRequest, s.config.MaximumContentRunes)
	}
	metadata, err := canonicalMetadata(request.Metadata)
	if err != nil {
		return "", nil, err
	}
	return contentID, metadata, nil
}

func (s *Service) validateSearchRequest(request SearchRequest) (string, map[string]string, int, string, error) {
	if !utf8.ValidString(request.Query) || strings.TrimSpace(request.Query) == "" {
		return "", nil, 0, "", fmt.Errorf("%w: query must be nonblank UTF-8", ErrInvalidRequest)
	}
	if utf8.RuneCountInString(request.Query) > s.config.MaximumContentRunes {
		return "", nil, 0, "", fmt.Errorf("%w: query exceeds %d runes", ErrInvalidRequest, s.config.MaximumContentRunes)
	}
	metadata, err := canonicalMetadata(request.Metadata)
	if err != nil {
		return "", nil, 0, "", err
	}
	limit := request.Limit
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if limit < 0 || limit > s.config.MaximumSearchResults {
		return "", nil, 0, "", fmt.Errorf("%w: limit must be between one and %d", ErrInvalidRequest, s.config.MaximumSearchResults)
	}
	afterContentID := strings.TrimSpace(request.AfterContentID)
	if afterContentID != "" && !contentIDPattern.MatchString(afterContentID) {
		return "", nil, 0, "", fmt.Errorf("%w: after_content_id is not path safe", ErrInvalidRequest)
	}
	return request.Query, metadata, limit, afterContentID, nil
}

func metadataMatches(record, filters map[string]string) bool {
	for key, value := range filters {
		if record[key] != value {
			return false
		}
	}
	return true
}

func containsAllTerms(recordTerms, queryTerms []string) bool {
	available := make(map[string]struct{}, len(recordTerms))
	for _, term := range recordTerms {
		available[term] = struct{}{}
	}
	for _, term := range queryTerms {
		if _, exists := available[term]; !exists {
			return false
		}
	}
	return true
}

func canonicalMetadata(input map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(input))
	for key, value := range input {
		canonicalKey := strings.TrimSpace(key)
		if canonicalKey == "" {
			return nil, fmt.Errorf("%w: metadata key is blank", ErrInvalidRequest)
		}
		if _, exists := result[canonicalKey]; exists {
			return nil, fmt.Errorf("%w: metadata keys collide after trimming", ErrInvalidRequest)
		}
		result[canonicalKey] = value
	}
	return result, nil
}

func (s *Service) detectAndRoute(ctx context.Context, contentID, content string, metadata map[string]string) (Record, language.Language, bool, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, language.Unknown, false, err
	}
	detected, err := s.detector.Detect(content)
	if err != nil {
		return Record{}, language.Unknown, false, fmt.Errorf("%w: detect language: %w", ErrWorkflow, err)
	}
	if err := ctx.Err(); err != nil {
		return Record{}, language.Unknown, false, err
	}
	confidences, err := s.detector.Confidences(content)
	if err != nil {
		return Record{}, language.Unknown, false, fmt.Errorf("%w: compute confidences: %w", ErrWorkflow, err)
	}
	if err := ctx.Err(); err != nil {
		return Record{}, language.Unknown, false, err
	}
	sections, err := s.detector.DetectMultiple(content)
	if err != nil {
		return Record{}, language.Unknown, false, fmt.Errorf("%w: detect sections: %w", ErrWorkflow, err)
	}

	record := Record{
		ContentID:     contentID,
		Content:       content,
		Metadata:      maps.Clone(metadata),
		Outcome:       OutcomeManualReview,
		Language:      detected.Language.String(),
		ISO6391:       detected.ISO6391,
		ISO6393:       detected.ISO6393,
		Confidence:    detected.Confidence,
		Confidences:   projectConfidences(confidences),
		Sections:      projectSections(sections),
		ScriptHints:   scriptHints(content),
		ReviewReasons: make([]string, 0, 3),
		Findings:      []Finding{},
		Tokens:        []PreparedToken{},
		Terms:         []string{},
	}
	if !detected.Detected {
		record.Language = "unknown"
	}
	if utf8.RuneCountInString(content) < s.config.MinimumRunes {
		record.ReviewReasons = append(record.ReviewReasons, ReasonTextTooShort)
	}
	if !detected.Detected || detected.Language == language.Unknown {
		record.ReviewReasons = append(record.ReviewReasons, ReasonLanguageUnknown)
	} else if detected.Confidence < s.config.MinimumConfidence {
		record.ReviewReasons = append(record.ReviewReasons, ReasonLowConfidence)
	}
	if hasMultipleLanguages(sections) {
		record.ReviewReasons = append(record.ReviewReasons, ReasonMixedLanguage)
	}
	if detected.Language == language.Japanese && !language.ContainsJapanese(content) {
		record.ReviewReasons = append(record.ReviewReasons, ReasonAmbiguousCJKScript)
	}
	if detected.Language == language.Chinese {
		record.ReviewReasons = append(record.ReviewReasons, ReasonUnsupportedLanguage)
	}
	manualReview := len(record.ReviewReasons) > 0
	if !manualReview && detected.Language != language.English && detected.Language != language.Korean && detected.Language != language.Japanese {
		record.ReviewReasons = append(record.ReviewReasons, ReasonUnsupportedLanguage)
		manualReview = true
	}
	return record, detected.Language, manualReview, nil
}

func projectConfidences(values []language.Confidence) []Confidence {
	result := make([]Confidence, len(values))
	for i, value := range values {
		result[i] = Confidence{Language: value.Language.String(), ISO6391: value.ISO6391, ISO6393: value.ISO6393, Value: value.Value}
	}
	return result
}

func projectSections(values []language.Section) []Section {
	result := make([]Section, len(values))
	for i, value := range values {
		result[i] = Section{Language: value.Language.String(), ISO6391: value.ISO6391, ISO6393: value.ISO6393, Start: value.Start, End: value.End, Text: value.Text}
	}
	return result
}

func scriptHints(text string) []string {
	hints := make([]string, 0, 4)
	if language.ContainsLatin(text) {
		hints = append(hints, "latin")
	}
	if language.ContainsKorean(text) {
		hints = append(hints, "hangul")
	}
	if language.ContainsJapanese(text) {
		hints = append(hints, "kana")
	}
	if language.ContainsChinese(text) {
		hints = append(hints, "han")
	}
	return hints
}

func hasMultipleLanguages(values []language.Section) bool {
	seen := make(map[language.Language]struct{}, 2)
	for _, value := range values {
		if value.Language == language.Unknown {
			continue
		}
		seen[value.Language] = struct{}{}
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

func projectFindings(matches []textsearch.BlockwordMatch) []Finding {
	findings := make([]Finding, len(matches))
	for i, match := range matches {
		findings[i] = Finding{
			ID:       match.Entry.ID,
			Text:     match.Text,
			Start:    match.Start,
			End:      match.End,
			Severity: severityName(match.Entry.Severity),
			Metadata: maps.Clone(match.Entry.Metadata),
		}
	}
	return findings
}

func severityName(value textsearch.Severity) string {
	switch value {
	case textsearch.SeverityHigh:
		return "high"
	case textsearch.SeverityMiddle:
		return "middle"
	default:
		return "low"
	}
}

func (s *Service) prepareSimpleTerms(content string) ([]string, error) {
	request, err := textsearch.NewTokenizeRequest(content, textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC})
	if err != nil {
		return nil, err
	}
	response, err := s.simpleTokenizer.Tokenize(request)
	if err != nil {
		return nil, err
	}
	terms := make([]string, 0, len(response.Tokens))
	seen := make(map[string]struct{}, len(response.Tokens))
	for _, token := range response.Tokens {
		if token.POS != textsearch.POSWord && token.POS != textsearch.POSNumber {
			continue
		}
		term := strings.ToLower(token.Normalized)
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms, nil
}

func (s *Service) prepareJapaneseTerms(content string) ([]PreparedToken, []string, error) {
	request, err := textsearch.NewTokenizeRequest(content, textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC})
	if err != nil {
		return nil, nil, err
	}
	response, err := s.japaneseTokenizer.Tokenize(request)
	if err != nil {
		return nil, nil, err
	}
	selected := japanese.Filter(response.Tokens, func(token textsearch.Token) bool {
		return japanese.IsNoun(token) || japanese.IsVerb(token)
	})
	tokens := make([]PreparedToken, len(selected))
	terms := make([]string, 0, len(selected))
	seen := make(map[string]struct{}, len(selected))
	for i, token := range selected {
		baseForm := token.Metadata[japanese.MetadataBaseForm]
		tokens[i] = PreparedToken{
			Text:       token.Text,
			Normalized: token.Normalized,
			BaseForm:   baseForm,
			POS:        string(token.POS),
			Start:      token.Span.Start,
			End:        token.Span.End,
			Metadata:   maps.Clone(token.Metadata),
		}
		term := japaneseIndexTerm(tokens[i])
		if term == "" {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return tokens, terms, nil
}

func japaneseIndexTerm(token PreparedToken) string {
	term := token.BaseForm
	if term == "" || term == "*" {
		term = token.Text
	}
	return textsearch.NormalizeText(term, textsearch.NormalizeNFC).Normalized
}

func cloneRecord(input Record) Record {
	result := input
	result.Metadata = maps.Clone(input.Metadata)
	result.Confidences = slices.Clone(input.Confidences)
	result.Sections = slices.Clone(input.Sections)
	result.ScriptHints = slices.Clone(input.ScriptHints)
	result.ReviewReasons = slices.Clone(input.ReviewReasons)
	result.Findings = make([]Finding, len(input.Findings))
	for i, finding := range input.Findings {
		result.Findings[i] = finding
		result.Findings[i].Metadata = maps.Clone(finding.Metadata)
	}
	result.Tokens = make([]PreparedToken, len(input.Tokens))
	for i, token := range input.Tokens {
		result.Tokens[i] = token
		result.Tokens[i].Metadata = maps.Clone(token.Metadata)
	}
	result.Terms = slices.Clone(input.Terms)
	return result
}
