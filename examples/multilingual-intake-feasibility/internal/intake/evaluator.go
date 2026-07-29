// Package intake 는 신뢰도 기반 다국어 지원 접수 흐름을 보여준다.
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
	// TokenizerNone 은 검토 정책 때문에 토큰화가 수행되지 않았음을 의미한다.
	TokenizerNone = "none"
	// TokenizerUnsupported 는 언어는 수용됐지만 토크나이저 학습 대상이 아님을 의미한다.
	TokenizerUnsupported = "unsupported"
	// TokenizerKagomeIPA 는 일본어 Kagome IPA 경로를 식별한다.
	TokenizerKagomeIPA = "kagome-ipa"

	// ReviewTextTooShort 는 설정된 rune 임계값보다 짧은 입력을 식별한다.
	ReviewTextTooShort = "text_too_short"
	// ReviewLanguageUnknown 은 지원 언어 판단을 만들 수 없는 입력을 식별한다.
	ReviewLanguageUnknown = "language_unknown"
	// ReviewLowConfidence 는 설정된 임계값보다 낮은 감지 결과를 식별한다.
	ReviewLowConfidence = "low_confidence"
	// ReviewMixedLanguage 는 여러 지원 문자 계열이 섞인 입력을 식별한다.
	ReviewMixedLanguage = "mixed_language"
)

var (
	// ErrInvalidConfig 는 평가기 생성 정책이 유효하지 않음을 식별한다.
	ErrInvalidConfig = errors.New("intake: invalid config")
	// ErrInvalidMessage 는 필수 메타데이터가 없는 접수 메시지를 식별한다.
	ErrInvalidMessage = errors.New("intake: invalid message")
)

// Config 는 애플리케이션 수준 신뢰도와 감지기 수명주기 정책을 소유한다.
type Config struct {
	MinimumConfidence float64 `json:"minimum_confidence"`
	MinimumRunes      int     `json:"minimum_runes"`
	PreloadModels     bool    `json:"preload_models"`
}

// Message 는 하나의 지원 접수 항목이다.
type Message struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// SelectedToken 은 원본 바이트 범위를 포함하는 유용한 일본어 명사 또는 동사다.
type SelectedToken struct {
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	POS   string `json:"pos"`
}

// LanguageSection 은 감지기가 보고한 연속 언어 영역이다.
type LanguageSection struct {
	Language string `json:"language"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
}

// Report 는 휴리스틱 판단과 결정적 평가기 결정을 명시적으로 보여준다.
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

// Evaluator 는 하나의 언어 감지기와 하나의 일본어 토크나이저를 재사용한다.
type Evaluator struct {
	detector          *language.Detector
	japaneseTokenizer *japanese.Tokenizer
	config            Config
}

// Preview 는 예제 명령이 출력하는 결정적 JSON 페이로드다.
type Preview struct {
	Scenario            string   `json:"scenario"`
	Config              Config   `json:"config"`
	LifecycleNotes      []string `json:"lifecycle_notes"`
	HeuristicBoundaries []string `json:"heuristic_boundaries"`
	Reports             []Report `json:"reports"`
	TestCommands        []string `json:"test_commands"`
}

// DefaultConfig 는 지연 로딩 감지기 모델과 보수적인 검토 임계값을 사용한다.
func DefaultConfig() Config {
	return Config{MinimumConfidence: 0.70, MinimumRunes: 8}
}

// NewEvaluator 는 재사용 가능한 감지기와 토크나이저 쌍을 구성한다.
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

// NewPreview 는 명령과 문서에 사용할 고정 fixture를 평가한다.
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

// Evaluate 는 신뢰도 기반 지원 접수 리포트 하나를 반환한다.
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
