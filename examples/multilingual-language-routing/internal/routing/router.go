// Package routing 은 증거 기반 언어 라우팅 정책을 지원 요청에 적용한다.
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
	// RouteModeration 은 지원되는 영어와 한국어 요청을 moderation 경로로 보낸다.
	RouteModeration = "moderation"
	// RouteJapaneseTokenization 은 신뢰도 높은 일본어 요청을 tokenization 경로로 보낸다.
	RouteJapaneseTokenization = "japanese-tokenization"
	// RouteManualReview 는 불확실하거나 지원되지 않는 요청을 검토자에게 보낸다.
	RouteManualReview = "manual-review"

	// ReviewTextTooShort 는 설정된 rune 임계값보다 짧은 입력을 식별한다.
	ReviewTextTooShort = "text-too-short"
	// ReviewLanguageUnknown 은 감지된 언어가 없는 입력을 식별한다.
	ReviewLanguageUnknown = "language-unknown"
	// ReviewLowConfidence 는 설정된 신뢰도보다 낮은 감지 결과를 식별한다.
	ReviewLowConfidence = "low-confidence"
	// ReviewMixedLanguage 는 unknown 이 아닌 감지 언어 구간이 여러 개인 입력을 식별한다.
	ReviewMixedLanguage = "mixed-language"
	// ReviewAmbiguousCJKScript 는 Kana 증거 없이 일본어로 감지된 입력을 식별한다.
	ReviewAmbiguousCJKScript = "ambiguous-cjk-script"
	// ReviewUnsupportedLanguage 는 자동 라우트가 없는 감지 언어를 식별한다.
	ReviewUnsupportedLanguage = "unsupported-language"
)

var (
	// ErrInvalidConfig 는 라우터 생성 또는 사용 정책이 유효하지 않음을 식별한다.
	ErrInvalidConfig = errors.New("routing: invalid config")
	// ErrInvalidRequest 는 유효한 필수 입력이 없는 요청을 식별한다.
	ErrInvalidRequest = errors.New("routing: invalid request")
)

// Config 는 애플리케이션 수준 감지기와 라우팅 임계값을 소유한다.
type Config struct {
	MinimumConfidence float64 `json:"minimum_confidence"`
	MinimumRunes      int     `json:"minimum_runes"`
	PreloadModels     bool    `json:"preload_models"`
}

// Request 는 호출자가 소유한 라우팅 입력을 담는다.
type Request struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Confidence 는 안정적인 언어 메타데이터와 함께 하나의 감지기 신뢰도를 표현한다.
type Confidence struct {
	Language string  `json:"language"`
	Value    float64 `json:"value"`
	ISO6391  string  `json:"iso_639_1"`
	ISO6393  string  `json:"iso_639_3"`
}

// Section 은 원본 UTF-8 바이트 오프셋을 포함하는 하나의 감지기 구간을 표현한다.
type Section struct {
	Language string `json:"language"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
	ISO6391  string `json:"iso_639_1"`
	ISO6393  string `json:"iso_639_3"`
}

// Decision 은 감지기 증거와 그 결과로 선택된 애플리케이션 라우트를 담는다.
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

// Router 는 변경 불가능한 애플리케이션 정책과 하나의 언어 감지기를 재사용한다.
type Router struct {
	detector *language.Detector
	config   Config
}

// DefaultConfig 는 예제가 사용하는 지연 로딩 감지기 정책을 반환한다.
func DefaultConfig() Config {
	return Config{MinimumConfidence: 0.70, MinimumRunes: 8}
}

// NewRouter 는 정책을 검증하고 네 개 언어 감지기를 구성한다.
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

// Route 는 감지기 증거와 그 결과로 선택된 애플리케이션 라우트를 반환한다.
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
