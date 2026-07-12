// Package intake demonstrates confidence-aware multilingual support intake.
package intake

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

const (
	// TokenizerNone means review policy prevented tokenization.
	TokenizerNone = "none"
	// TokenizerUnsupported means the language was accepted without a tokenizer lesson.
	TokenizerUnsupported = "unsupported"
	// TokenizerKagomeIPA identifies the Japanese Kagome IPA path.
	TokenizerKagomeIPA = "kagome-ipa"

	// ReviewTextTooShort identifies input below the configured rune threshold.
	ReviewTextTooShort = "text_too_short"
	// ReviewLanguageUnknown identifies input without a supported language decision.
	ReviewLanguageUnknown = "language_unknown"
	// ReviewLowConfidence identifies a detector result below the configured threshold.
	ReviewLowConfidence = "low_confidence"
	// ReviewMixedLanguage identifies input containing multiple supported script families.
	ReviewMixedLanguage = "mixed_language"
)

var (
	// ErrInvalidConfig identifies invalid evaluator construction policy.
	ErrInvalidConfig = errors.New("intake: invalid config")
	// ErrInvalidMessage identifies an intake message without required metadata.
	ErrInvalidMessage = errors.New("intake: invalid message")
)

// Config owns application-level confidence and detector lifecycle policy.
type Config struct {
	MinimumConfidence float64 `json:"minimum_confidence"`
	MinimumRunes      int     `json:"minimum_runes"`
	PreloadModels     bool    `json:"preload_models"`
}

// Message is one support-intake item.
type Message struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// SelectedToken is a useful Japanese noun or verb with an original byte span.
type SelectedToken struct {
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	POS   string `json:"pos"`
}

// LanguageSection is a detector-reported contiguous language region.
type LanguageSection struct {
	Language string `json:"language"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
}

// Report makes heuristic and deterministic evaluator decisions explicit.
type Report struct {
	ID             string            `json:"id"`
	Text           string            `json:"text"`
	Language       string            `json:"language"`
	Confidence     float64           `json:"confidence"`
	Mixed          bool              `json:"mixed"`
	ScriptHints    []string          `json:"script_hints"`
	Sections       []LanguageSection `json:"sections"`
	Tokenizer      string            `json:"tokenizer"`
	SelectedTokens []SelectedToken   `json:"selected_tokens"`
	ManualReview   bool              `json:"manual_review"`
	ReviewReasons  []string          `json:"review_reasons"`
}

// Evaluator reuses one language detector and one Japanese tokenizer.
type Evaluator struct {
	detector          *language.Detector
	japaneseTokenizer *japanese.Tokenizer
	config            Config
}

// Preview is the deterministic JSON payload printed by the example command.
type Preview struct {
	Scenario            string   `json:"scenario"`
	Config              Config   `json:"config"`
	LifecycleNotes      []string `json:"lifecycle_notes"`
	HeuristicBoundaries []string `json:"heuristic_boundaries"`
	Reports             []Report `json:"reports"`
	TestCommands        []string `json:"test_commands"`
}

// DefaultConfig uses lazy detector models and conservative review thresholds.
func DefaultConfig() Config {
	return Config{MinimumConfidence: 0.70, MinimumRunes: 8}
}

// NewEvaluator constructs the reusable detector and tokenizer pair.
func NewEvaluator(config Config) (*Evaluator, error) {
	if config.MinimumConfidence < 0 || config.MinimumConfidence > 1 {
		return nil, fmt.Errorf("%w: minimum confidence must be between 0 and 1", ErrInvalidConfig)
	}
	if config.MinimumRunes <= 0 {
		return nil, fmt.Errorf("%w: minimum runes must be positive", ErrInvalidConfig)
	}

	options := make([]language.Option, 0, 1)
	if config.PreloadModels {
		options = append(options, language.WithPreloadedLanguageModels())
	}
	detector, err := language.NewDetector([]language.Language{
		language.English,
		language.Korean,
		language.Japanese,
	}, options...)
	if err != nil {
		return nil, fmt.Errorf("create language detector: %w", err)
	}
	tokenizer, err := japanese.NewTokenizer()
	if err != nil {
		return nil, fmt.Errorf("create Japanese tokenizer: %w", err)
	}
	return &Evaluator{detector: detector, japaneseTokenizer: tokenizer, config: config}, nil
}

// NewPreview evaluates fixed fixtures for the command and documentation.
func NewPreview() (Preview, error) {
	config := DefaultConfig()
	evaluator, err := NewEvaluator(config)
	if err != nil {
		return Preview{}, err
	}
	messages := []Message{
		{ID: "preview-en", Text: "Please help me check the delivery status for order number 12345."},
		{ID: "preview-ko", Text: "주문 번호 12345의 배송 상태를 확인하고 싶습니다."},
		{ID: "preview-ja", Text: "配送状況を確認したいです。注文番号を教えてください。"},
		{ID: "preview-mixed", Text: "Please check 配送状況を確認してください for order 12345."},
		{ID: "preview-short", Text: "help"},
		{ID: "preview-unknown", Text: "12345 67890"},
	}
	reports := make([]Report, len(messages))
	for i, message := range messages {
		report, err := evaluator.Evaluate(message)
		if err != nil {
			return Preview{}, fmt.Errorf("evaluate preview message %q: %w", message.ID, err)
		}
		reports[i] = report
	}
	return Preview{
		Scenario: "evaluate multilingual support intake without hiding heuristic uncertainty",
		Config:   config,
		LifecycleNotes: []string{
			"The command constructs one reusable detector and one reusable Kagome tokenizer.",
			"Lingua models load lazily by default; preloading is an explicit application startup tradeoff.",
			"The Kagome IPA dictionary increases binary and memory footprint even when only Japanese input is tokenized.",
		},
		HeuristicBoundaries: []string{
			"Language and confidence are heuristic evidence, not security or compliance decisions.",
			"Short, unknown, low-confidence, and mixed-script input is routed to manual review.",
			"Only confident non-mixed Japanese input is tokenized; English and Korean remain explicitly unsupported.",
			"Japanese token spans are UTF-8 byte offsets into the original text.",
		},
		Reports: reports,
		TestCommands: []string{
			"go test -count=1 ./examples/multilingual-intake-feasibility/...",
			"go test -race -count=1 ./examples/multilingual-intake-feasibility/...",
		},
	}, nil
}

// Evaluate returns one confidence-aware support-intake report.
func (e *Evaluator) Evaluate(message Message) (Report, error) {
	if e == nil || e.detector == nil || e.japaneseTokenizer == nil {
		return Report{}, fmt.Errorf("%w: evaluator is nil", ErrInvalidConfig)
	}
	message.ID = strings.TrimSpace(message.ID)
	if message.ID == "" {
		return Report{}, fmt.Errorf("%w: id is required", ErrInvalidMessage)
	}

	detected, err := e.detector.Detect(message.Text)
	if err != nil {
		return Report{}, fmt.Errorf("detect %q language: %w", message.ID, err)
	}
	sections, err := e.detector.DetectMultiple(message.Text)
	if err != nil {
		return Report{}, fmt.Errorf("detect %q language sections: %w", message.ID, err)
	}

	report := Report{
		ID:             message.ID,
		Text:           message.Text,
		Language:       detected.Language.String(),
		Confidence:     detected.Confidence,
		ScriptHints:    scriptHints(message.Text),
		Sections:       languageSections(sections),
		Tokenizer:      TokenizerNone,
		SelectedTokens: []SelectedToken{},
		ReviewReasons:  []string{},
	}
	report.Mixed = len(report.ScriptHints) > 1

	if utf8.RuneCountInString(message.Text) < e.config.MinimumRunes {
		report.ReviewReasons = append(report.ReviewReasons, ReviewTextTooShort)
	}
	if !detected.Detected || detected.Language == language.Unknown {
		report.ReviewReasons = append(report.ReviewReasons, ReviewLanguageUnknown)
	} else if detected.Confidence < e.config.MinimumConfidence {
		report.ReviewReasons = append(report.ReviewReasons, ReviewLowConfidence)
	}
	if report.Mixed {
		report.ReviewReasons = append(report.ReviewReasons, ReviewMixedLanguage)
	}
	report.ManualReview = len(report.ReviewReasons) > 0

	if report.ManualReview {
		return report, nil
	}
	if detected.Language != language.Japanese {
		report.Tokenizer = TokenizerUnsupported
		return report, nil
	}

	request, err := textsearch.NewTokenizeRequest(message.Text, textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC})
	if err != nil {
		return Report{}, fmt.Errorf("create %q tokenization request: %w", message.ID, err)
	}
	response, err := e.japaneseTokenizer.Tokenize(request)
	if err != nil {
		return Report{}, fmt.Errorf("tokenize %q Japanese text: %w", message.ID, err)
	}
	report.Tokenizer = TokenizerKagomeIPA
	report.SelectedTokens = selectedJapaneseTokens(response.Tokens)
	return report, nil
}

func scriptHints(text string) []string {
	hints := make([]string, 0, 3)
	if language.ContainsJapanese(text) {
		hints = append(hints, "Japanese")
	}
	if language.ContainsKorean(text) {
		hints = append(hints, "Korean")
	}
	if language.ContainsLatin(text) {
		hints = append(hints, "Latin")
	}
	return hints
}

func languageSections(sections []language.Section) []LanguageSection {
	result := make([]LanguageSection, len(sections))
	for i, section := range sections {
		result[i] = LanguageSection{
			Language: section.Language.String(),
			Start:    section.Start,
			End:      section.End,
			Text:     section.Text,
		}
	}
	return result
}

func selectedJapaneseTokens(tokens []textsearch.Token) []SelectedToken {
	selected := japanese.Filter(tokens, func(token textsearch.Token) bool {
		return japanese.IsNoun(token) || japanese.IsVerb(token)
	})
	result := make([]SelectedToken, len(selected))
	for i, token := range selected {
		result[i] = SelectedToken{
			Text:  token.Text,
			Start: token.Span.Start,
			End:   token.Span.End,
			POS:   token.Metadata[japanese.MetadataPOS],
		}
	}
	return result
}
