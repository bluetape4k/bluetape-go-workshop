// Package moderation demonstrates deterministic textsearch blockword masking.
package moderation

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/textsearch"
)

const maxReviewTextRunes = 8_000

var (
	// ErrInvalidReview reports a malformed moderation review request.
	ErrInvalidReview = errors.New("moderation: invalid review")
)

// Decision is the public moderation outcome returned to callers.
type Decision string

const (
	// DecisionAllowed means no accepted blockword match remained after policy filtering.
	DecisionAllowed Decision = "allowed"
	// DecisionMasked means at least one accepted blockword match was masked.
	DecisionMasked Decision = "masked"
)

// Policy describes the local blockword and allowlist configuration.
type Policy struct {
	Entries       []textsearch.BlockwordEntry
	AllowPhrases  []AllowPhrase
	MinSeverity   textsearch.Severity
	Mask          string
	NormalizeMode textsearch.NormalizeMode
}

// AllowPhrase marks a configured phrase that should suppress contained matches.
type AllowPhrase struct {
	ID     string
	Text   string
	Reason string
}

// Service evaluates comments using a compiled deterministic policy.
type Service struct {
	dictionary   *textsearch.BlockwordDictionary
	allowMatcher *textsearch.Matcher
	allowPhrases map[string]AllowPhrase
	options      textsearch.BlockwordOptions
}

// ReviewRequest is the caller-owned moderation input.
type ReviewRequest struct {
	ContentID string `json:"content_id"`
	Text      string `json:"text"`
}

// ReviewResult is the stable JSON-friendly output for one moderation review.
type ReviewResult struct {
	ContentID     string         `json:"content_id"`
	Decision      Decision       `json:"decision"`
	OriginalText  string         `json:"original_text"`
	MaskedText    string         `json:"masked_text"`
	Findings      []Finding      `json:"findings"`
	AllowlistHits []AllowlistHit `json:"allowlist_hits"`
	BoundaryNotes []string       `json:"boundary_notes"`
}

// Finding describes one accepted blockword match after allowlist filtering.
type Finding struct {
	ID       string              `json:"id"`
	Text     string              `json:"text"`
	Severity textsearch.Severity `json:"severity"`
	Category string              `json:"category"`
	Start    int                 `json:"start"`
	End      int                 `json:"end"`
}

// AllowlistHit describes one safe phrase span that affected filtering.
type AllowlistHit struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

// Preview is the printable example payload returned by the command.
type Preview struct {
	Scenario string         `json:"scenario"`
	Policy   PreviewPolicy  `json:"policy"`
	Samples  []ReviewResult `json:"samples"`
	Commands []string       `json:"commands"`
}

// PreviewPolicy summarizes the compiled policy in the command output.
type PreviewPolicy struct {
	Boundary      string   `json:"boundary"`
	Normalization string   `json:"normalization"`
	Mask          string   `json:"mask"`
	Blockwords    []string `json:"blockwords"`
	AllowPhrases  []string `json:"allow_phrases"`
}

// DefaultPolicy returns the workshop moderation policy used by the example.
func DefaultPolicy() Policy {
	return Policy{
		Entries: []textsearch.BlockwordEntry{
			{ID: "abuse-bad", Text: "bad", Severity: textsearch.SeverityMiddle, Metadata: map[string]string{"category": "abuse"}},
			{ID: "abuse-bad-wolf", Text: "bad wolf", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"category": "abuse"}},
			{ID: "fraud-scam", Text: "scam", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"category": "fraud"}},
			{ID: "ko-abuse", Text: "욕설", Severity: textsearch.SeverityHigh, Metadata: map[string]string{"category": "abuse"}},
			{ID: "ko-fraud", Text: "무료 돈", Severity: textsearch.SeverityMiddle, Metadata: map[string]string{"category": "fraud"}},
		},
		AllowPhrases: []AllowPhrase{
			{ID: "title-bad-wolf-book-club", Text: "bad wolf book club", Reason: "book title discussion"},
		},
		MinSeverity:   textsearch.SeverityMiddle,
		Mask:          "*",
		NormalizeMode: textsearch.NormalizeNFC,
	}
}

// NewService compiles a Policy into a reusable moderation service.
func NewService(policy Policy) (*Service, error) {
	if policy.Mask == "" {
		policy.Mask = "*"
	}
	dictionary, err := textsearch.NewBlockwordDictionary(policy.Entries, textsearch.Config{
		IgnoreCase: true,
		Normalize:  policy.NormalizeMode,
		Boundary:   textsearch.BoundaryUnicodeWord,
	})
	if err != nil {
		return nil, fmt.Errorf("compile blockword policy: %w", err)
	}
	allowMatcher, allowPhrases, err := compileAllowPhrases(policy.AllowPhrases, policy.NormalizeMode)
	if err != nil {
		return nil, err
	}
	return &Service{
		dictionary:   dictionary,
		allowMatcher: allowMatcher,
		allowPhrases: allowPhrases,
		options: textsearch.BlockwordOptions{
			Mask:        policy.Mask,
			MinSeverity: policy.MinSeverity,
		},
	}, nil
}

// Evaluate reviews one comment and returns the masked projection plus evidence.
func (s *Service) Evaluate(ctx context.Context, request ReviewRequest) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, err
	}
	if strings.TrimSpace(request.ContentID) == "" {
		return ReviewResult{}, fmt.Errorf("%w: content_id is required", ErrInvalidReview)
	}
	if utf8.RuneCountInString(request.Text) > maxReviewTextRunes {
		return ReviewResult{}, fmt.Errorf("%w: text length exceeds %d runes", ErrInvalidReview, maxReviewTextRunes)
	}
	blockRequest, err := textsearch.NewBlockwordRequest(request.Text, s.options)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("%w: %w", ErrInvalidReview, err)
	}
	allowHits := s.detectAllowlist(request.Text)
	matches := s.filterAllowedMatches(s.dictionary.Detect(blockRequest.Text, blockRequest.Options), allowHits)
	findings := findingsFromMatches(matches)
	masked := maskMatches(blockRequest.Text, matches, blockRequest.Options.Mask)
	decision := DecisionAllowed
	if len(findings) > 0 {
		decision = DecisionMasked
	}
	return ReviewResult{
		ContentID:     request.ContentID,
		Decision:      decision,
		OriginalText:  request.Text,
		MaskedText:    masked,
		Findings:      findings,
		AllowlistHits: allowHits,
		BoundaryNotes: []string{
			"BoundaryUnicodeWord prevents substring matches inside words such as badge or badwolf.",
			"Allowlist phrases remove contained blockword matches before masking.",
		},
	}, nil
}

// NewPreview builds the sample payload printed by the example command.
func NewPreview() (Preview, error) {
	service, err := NewService(DefaultPolicy())
	if err != nil {
		return Preview{}, err
	}
	samples := []ReviewRequest{
		{ContentID: "comment-1001", Text: "bad wolf offer와 욕설 포함"},
		{ContentID: "comment-1002", Text: "bad wolf book club discusses scam risk"},
		{ContentID: "comment-1003", Text: "badge bad badwolf bad."},
	}
	results := make([]ReviewResult, 0, len(samples))
	for _, sample := range samples {
		result, err := service.Evaluate(context.Background(), sample)
		if err != nil {
			return Preview{}, err
		}
		results = append(results, result)
	}
	return Preview{
		Scenario: "review marketplace comments with deterministic blockword masking",
		Policy: PreviewPolicy{
			Boundary:      "Unicode word boundary",
			Normalization: "NFC",
			Mask:          "*",
			Blockwords:    []string{"bad", "bad wolf", "scam", "욕설", "무료 돈"},
			AllowPhrases:  []string{"bad wolf book club"},
		},
		Samples: results,
		Commands: []string{
			"go run ./examples/text-moderation-masking",
			"go test -count=1 ./examples/text-moderation-masking/...",
			"go test -race -count=1 ./examples/text-moderation-masking/...",
		},
	}, nil
}

func compileAllowPhrases(phrases []AllowPhrase, mode textsearch.NormalizeMode) (*textsearch.Matcher, map[string]AllowPhrase, error) {
	if len(phrases) == 0 {
		return nil, nil, nil
	}
	patterns := make([]textsearch.Pattern, len(phrases))
	lookup := make(map[string]AllowPhrase, len(phrases))
	for i, phrase := range phrases {
		if strings.TrimSpace(phrase.ID) == "" || strings.TrimSpace(phrase.Text) == "" {
			return nil, nil, fmt.Errorf("%w: allow phrase id and text are required", ErrInvalidReview)
		}
		patterns[i] = textsearch.Pattern{ID: phrase.ID, Text: phrase.Text}
		lookup[phrase.ID] = phrase
	}
	matcher, err := textsearch.Compile(patterns, textsearch.Config{
		IgnoreCase: true,
		Normalize:  mode,
		Boundary:   textsearch.BoundaryUnicodeWord,
		Overlap:    textsearch.OverlapLeftmostLongest,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("compile allow phrases: %w", err)
	}
	return matcher, lookup, nil
}

func (s *Service) detectAllowlist(input string) []AllowlistHit {
	if s.allowMatcher == nil {
		return nil
	}
	matches := s.allowMatcher.FindAll(input)
	hits := make([]AllowlistHit, 0, len(matches))
	for _, match := range matches {
		phrase := s.allowPhrases[match.Pattern.ID]
		hits = append(hits, AllowlistHit{
			ID:     phrase.ID,
			Text:   match.Text,
			Reason: phrase.Reason,
			Start:  match.Start,
			End:    match.End,
		})
	}
	return hits
}

func (s *Service) filterAllowedMatches(matches []textsearch.BlockwordMatch, hits []AllowlistHit) []textsearch.BlockwordMatch {
	if len(matches) == 0 || len(hits) == 0 {
		return matches
	}
	filtered := matches[:0]
	for _, match := range matches {
		if !containedInAllowlist(match.Start, match.End, hits) {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

func containedInAllowlist(start, end int, hits []AllowlistHit) bool {
	for _, hit := range hits {
		if start >= hit.Start && end <= hit.End {
			return true
		}
	}
	return false
}

func findingsFromMatches(matches []textsearch.BlockwordMatch) []Finding {
	findings := make([]Finding, len(matches))
	for i, match := range matches {
		findings[i] = Finding{
			ID:       match.Entry.ID,
			Text:     match.Text,
			Severity: match.Entry.Severity,
			Category: match.Entry.Metadata["category"],
			Start:    match.Start,
			End:      match.End,
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Start != findings[j].Start {
			return findings[i].Start < findings[j].Start
		}
		return findings[i].End > findings[j].End
	})
	return findings
}

func maskMatches(input string, matches []textsearch.BlockwordMatch, mask string) string {
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
