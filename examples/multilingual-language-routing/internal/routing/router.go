// Package routing applies evidence-backed language routing policy to support requests.
package routing

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

const (
	// RouteModeration sends supported English and Korean requests to moderation.
	RouteModeration = "moderation"
	// RouteJapaneseTokenization sends confident Japanese requests to tokenization.
	RouteJapaneseTokenization = "japanese-tokenization"
	// RouteManualReview sends uncertain or unsupported requests to a reviewer.
	RouteManualReview = "manual-review"

	// ReviewTextTooShort identifies input below the configured rune threshold.
	ReviewTextTooShort = "text-too-short"
	// ReviewLanguageUnknown identifies input without a detected language.
	ReviewLanguageUnknown = "language-unknown"
	// ReviewLowConfidence identifies a detection below the configured confidence.
	ReviewLowConfidence = "low-confidence"
	// ReviewMixedLanguage identifies multiple detected non-unknown language sections.
	ReviewMixedLanguage = "mixed-language"
	// ReviewAmbiguousCJKScript identifies Japanese detection without Kana evidence.
	ReviewAmbiguousCJKScript = "ambiguous-cjk-script"
	// ReviewUnsupportedLanguage identifies a detected language without an automated route.
	ReviewUnsupportedLanguage = "unsupported-language"
)

var (
	// ErrInvalidConfig identifies invalid router construction or use.
	ErrInvalidConfig = errors.New("routing: invalid config")
	// ErrInvalidRequest identifies a request without valid required input.
	ErrInvalidRequest = errors.New("routing: invalid request")
)

// Config owns application-level detector and routing thresholds.
type Config struct {
	MinimumConfidence float64 `json:"minimum_confidence"`
	MinimumRunes      int     `json:"minimum_runes"`
	PreloadModels     bool    `json:"preload_models"`
}

// Request contains caller-owned routing input.
type Request struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Confidence projects one detector confidence with stable language metadata.
type Confidence struct {
	Language string  `json:"language"`
	Value    float64 `json:"value"`
	ISO6391  string  `json:"iso_639_1"`
	ISO6393  string  `json:"iso_639_3"`
}

// Section projects one detector section with original UTF-8 byte offsets.
type Section struct {
	Language string `json:"language"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
	ISO6391  string `json:"iso_639_1"`
	ISO6393  string `json:"iso_639_3"`
}

// Decision contains detector evidence and the resulting application route.
type Decision struct {
	ID            string       `json:"id"`
	Text          string       `json:"text"`
	Language      string       `json:"language"`
	ISO6391       string       `json:"iso_639_1"`
	ISO6393       string       `json:"iso_639_3"`
	Confidence    float64      `json:"confidence"`
	Confidences   []Confidence `json:"confidences"`
	Sections      []Section    `json:"sections"`
	ScriptHints   []string     `json:"script_hints"`
	Route         string       `json:"route"`
	ManualReview  bool         `json:"manual_review"`
	ReviewReasons []string     `json:"review_reasons"`
}

// Router reuses one language detector with immutable application policy.
type Router struct {
	detector *language.Detector
	config   Config
}

// DefaultConfig returns the lazy detector policy used by the example.
func DefaultConfig() Config {
	return Config{MinimumConfidence: 0.70, MinimumRunes: 8}
}

// NewRouter validates policy and constructs a four-language detector.
func NewRouter(config Config) (*Router, error) {
	if math.IsNaN(config.MinimumConfidence) || config.MinimumConfidence < 0 || config.MinimumConfidence > 1 || config.MinimumRunes <= 0 {
		return nil, fmt.Errorf("%w: confidence must be in [0,1] and minimum runes must be positive", ErrInvalidConfig)
	}

	options := make([]language.Option, 0, 1)
	if config.PreloadModels {
		options = append(options, language.WithPreloadedLanguageModels())
	}
	detector, err := language.NewDetector([]language.Language{
		language.English,
		language.Korean,
		language.Japanese,
		language.Chinese,
	}, options...)
	if err != nil {
		return nil, fmt.Errorf("create language router detector: %w", err)
	}
	return &Router{detector: detector, config: config}, nil
}

// Route returns detector evidence and the resulting application route.
func (r *Router) Route(request Request) (Decision, error) {
	if r == nil || r.detector == nil {
		return Decision{}, fmt.Errorf("%w: router is nil", ErrInvalidConfig)
	}
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return Decision{}, fmt.Errorf("%w: id is required", ErrInvalidRequest)
	}
	detected, err := r.detector.Detect(request.Text)
	if err != nil {
		return Decision{}, fmt.Errorf("detect %q language: %w", id, err)
	}
	rawConfidences, err := r.detector.Confidences(request.Text)
	if err != nil {
		return Decision{}, fmt.Errorf("compute %q confidences: %w", id, err)
	}
	rawSections, err := r.detector.DetectMultiple(request.Text)
	if err != nil {
		return Decision{}, fmt.Errorf("detect %q language sections: %w", id, err)
	}

	decision := Decision{
		ID:            id,
		Text:          request.Text,
		Language:      detected.Language.String(),
		ISO6391:       detected.ISO6391,
		ISO6393:       detected.ISO6393,
		Confidence:    detected.Confidence,
		Confidences:   projectConfidences(rawConfidences),
		Sections:      projectSections(rawSections),
		ScriptHints:   scriptHints(request.Text),
		Route:         RouteManualReview,
		ReviewReasons: []string{},
	}
	if !detected.Detected {
		decision.Language = "unknown"
	}
	if utf8.RuneCountInString(request.Text) < r.config.MinimumRunes {
		decision.ReviewReasons = append(decision.ReviewReasons, ReviewTextTooShort)
	}
	if !detected.Detected || detected.Language == language.Unknown {
		decision.ReviewReasons = append(decision.ReviewReasons, ReviewLanguageUnknown)
	} else if detected.Confidence < r.config.MinimumConfidence {
		decision.ReviewReasons = append(decision.ReviewReasons, ReviewLowConfidence)
	}
	if hasMultipleLanguages(rawSections) {
		decision.ReviewReasons = append(decision.ReviewReasons, ReviewMixedLanguage)
	}
	if detected.Language == language.Japanese && !language.ContainsJapanese(request.Text) {
		decision.ReviewReasons = append(decision.ReviewReasons, ReviewAmbiguousCJKScript)
	}
	if detected.Language == language.Chinese {
		decision.ReviewReasons = append(decision.ReviewReasons, ReviewUnsupportedLanguage)
	}

	decision.ManualReview = len(decision.ReviewReasons) > 0
	if !decision.ManualReview {
		switch detected.Language {
		case language.English, language.Korean:
			decision.Route = RouteModeration
		case language.Japanese:
			decision.Route = RouteJapaneseTokenization
		default:
			decision.Route = RouteManualReview
			decision.ManualReview = true
			decision.ReviewReasons = append(decision.ReviewReasons, ReviewUnsupportedLanguage)
		}
	}
	return decision, nil
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

func projectConfidences(values []language.Confidence) []Confidence {
	result := make([]Confidence, len(values))
	for i, value := range values {
		result[i] = Confidence{
			Language: value.Language.String(),
			Value:    value.Value,
			ISO6391:  value.ISO6391,
			ISO6393:  value.ISO6393,
		}
	}
	return result
}

func projectSections(values []language.Section) []Section {
	result := make([]Section, len(values))
	for i, value := range values {
		result[i] = Section{
			Language: value.Language.String(),
			Start:    value.Start,
			End:      value.End,
			Text:     value.Text,
			ISO6391:  value.ISO6391,
			ISO6393:  value.ISO6393,
		}
	}
	return result
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
