package routing

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

func TestDefaultConfigAndSelectedLanguages(t *testing.T) {
	wantConfig := Config{MinimumConfidence: 0.70, MinimumRunes: 8, PreloadModels: false}
	if got := DefaultConfig(); got != wantConfig {
		t.Fatalf("DefaultConfig() = %+v, want %+v", got, wantConfig)
	}

	router, err := NewRouter(DefaultConfig())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	wantLanguages := []language.Language{
		language.English,
		language.Korean,
		language.Japanese,
		language.Chinese,
	}
	if got := router.detector.Languages(); !slices.Equal(got, wantLanguages) {
		t.Fatalf("detector languages = %#v, want %#v", got, wantLanguages)
	}
}

func TestNewRouterRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "negative confidence", config: Config{MinimumConfidence: -0.01, MinimumRunes: 8}},
		{name: "confidence above one", config: Config{MinimumConfidence: 1.01, MinimumRunes: 8}},
		{name: "non-positive minimum runes", config: Config{MinimumConfidence: 0.70, MinimumRunes: 0}},
		{name: "NaN confidence", config: Config{MinimumConfidence: math.NaN(), MinimumRunes: 8}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRouter(tt.config)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewRouter(%+v) error = %v, want ErrInvalidConfig", tt.config, err)
			}
		})
	}
}

func TestRouteRejectsInvalidRouterAndRequest(t *testing.T) {
	var nilRouter *Router
	if _, err := nilRouter.Route(Request{ID: "request-1", Text: "Please review the delivery status."}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil Router.Route() error = %v, want ErrInvalidConfig", err)
	}

	router, err := NewRouter(DefaultConfig())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	if _, err := router.Route(Request{ID: " \t", Text: "Please review the delivery status."}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Route(blank ID) error = %v, want ErrInvalidRequest", err)
	}

	_, err = router.Route(Request{ID: "request-blank", Text: " \t\n"})
	if !errors.Is(err, language.ErrBlankText) || errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Route(blank text) error = %v, want only language.ErrBlankText", err)
	}

	tooLong := strings.Repeat("a", language.MaxTextLength+1)
	_, err = router.Route(Request{ID: "request-long", Text: tooLong})
	if !errors.Is(err, language.ErrTextTooLong) || errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Route(oversized text) error = %v, want only language.ErrTextTooLong", err)
	}
}

func TestRoutePolicy(t *testing.T) {
	router, err := NewRouter(DefaultConfig())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	tests := []struct {
		id       string
		text     string
		language string
		route    string
		review   bool
		reasons  []string
	}{
		{id: "en", text: "Please review the delivery status for order 12345.", language: "English", route: RouteModeration, reasons: []string{}},
		{id: "ko", text: "주문 번호 12345의 배송 상태를 확인해 주세요.", language: "Korean", route: RouteModeration, reasons: []string{}},
		{id: "ja", text: "配送状況を確認してください。注文番号は12345です。", language: "Japanese", route: RouteJapaneseTokenization, reasons: []string{}},
		{id: "zh", text: "请确认订单12345的配送状态。", language: "Chinese", route: RouteManualReview, review: true, reasons: []string{ReviewUnsupportedLanguage}},
		{id: "short", text: "help", language: "English", route: RouteManualReview, review: true, reasons: []string{ReviewTextTooShort}},
		{id: "unknown", text: "12345 67890", language: "unknown", route: RouteManualReview, review: true, reasons: []string{ReviewLanguageUnknown}},
		{id: "mixed", text: "Please check 配送状況を確認してください for order 12345.", language: "Japanese", route: RouteManualReview, review: true, reasons: []string{ReviewMixedLanguage}},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got, routeErr := router.Route(Request{ID: "  " + tt.id + "  ", Text: tt.text})
			if routeErr != nil {
				t.Fatalf("Route() error = %v", routeErr)
			}
			if got.ID != tt.id || got.Text != tt.text || got.Language != tt.language || got.Route != tt.route || got.ManualReview != tt.review {
				t.Fatalf("Route() = %#v", got)
			}
			if tt.language == "unknown" && (got.ISO6391 != "" || got.ISO6393 != "") {
				t.Fatalf("unknown ISO codes = %q/%q", got.ISO6391, got.ISO6393)
			}
			if !reflect.DeepEqual(got.ReviewReasons, tt.reasons) {
				t.Fatalf("ReviewReasons = %v, want %v", got.ReviewReasons, tt.reasons)
			}
			if got.Confidences == nil || got.Sections == nil || got.ScriptHints == nil || got.ReviewReasons == nil {
				t.Fatalf("successful slices must be non-nil: %#v", got)
			}
		})
	}
}

func TestLowConfidenceFallbackUsesExplicitThreshold(t *testing.T) {
	config := DefaultConfig()
	config.MinimumConfidence = 1.0
	router, err := NewRouter(config)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	got, err := router.Route(Request{ID: "low", Text: "support 문의 订单 delivery"})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if got.Language != "Korean" || got.Confidence >= config.MinimumConfidence {
		t.Fatalf("low-confidence evidence = %#v", got)
	}
	if got.Route != RouteManualReview || !got.ManualReview || !reflect.DeepEqual(got.ReviewReasons, []string{ReviewLowConfidence}) {
		t.Fatalf("low-confidence decision = %#v", got)
	}
}

func TestRouteProjectsMixedEvidence(t *testing.T) {
	router, err := NewRouter(DefaultConfig())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	text := "Please check 配送状況を確認してください for order 12345."
	got, err := router.Route(Request{ID: "mixed", Text: text})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if len(got.Confidences) != 4 {
		t.Fatalf("len(Confidences) = %d, want 4", len(got.Confidences))
	}
	for i := 1; i < len(got.Confidences); i++ {
		if got.Confidences[i-1].Value < got.Confidences[i].Value {
			t.Fatalf("Confidences are not descending: %v", got.Confidences)
		}
	}
	if !reflect.DeepEqual(got.ScriptHints, []string{"latin", "kana", "han"}) {
		t.Fatalf("ScriptHints = %v", got.ScriptHints)
	}
	languages := make(map[string]struct{}, 2)
	for _, section := range got.Sections {
		if section.Start < 0 || section.End > len(text) || section.Start > section.End || text[section.Start:section.End] != section.Text {
			t.Fatalf("invalid section: %#v", section)
		}
		if section.Language != language.Unknown.String() {
			languages[section.Language] = struct{}{}
		}
	}
	if len(languages) < 2 {
		t.Fatalf("non-unknown section languages = %v", languages)
	}
}

func TestJapaneseWithoutKanaFailsClosed(t *testing.T) {
	router, err := NewRouter(DefaultConfig())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	got, err := router.Route(Request{ID: "han", Text: "注文番号配送確認依頼"})
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if got.Route != RouteManualReview || !got.ManualReview {
		t.Fatalf("Han-only decision = %#v, want manual review", got)
	}
	switch got.Language {
	case "Japanese":
		if !reflect.DeepEqual(got.ReviewReasons, []string{ReviewAmbiguousCJKScript}) {
			t.Fatalf("Japanese Han-only reasons = %v", got.ReviewReasons)
		}
	case "Chinese":
		if !reflect.DeepEqual(got.ReviewReasons, []string{ReviewUnsupportedLanguage}) {
			t.Fatalf("Chinese Han-only reasons = %v", got.ReviewReasons)
		}
	default:
		t.Fatalf("Han-only Language = %q, want Japanese or Chinese", got.Language)
	}
}

func TestRouteReturnsCallerOwnedEvidence(t *testing.T) {
	router, err := NewRouter(DefaultConfig())
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	request := Request{ID: "mixed", Text: "Please check 配送状況を確認してください for order 12345."}
	first, err := router.Route(request)
	if err != nil {
		t.Fatalf("first Route() error = %v", err)
	}
	first.Confidences[0].Language = "mutated"
	first.Sections[0].Text = "mutated"
	first.ScriptHints[0] = "mutated"
	first.ReviewReasons[0] = "mutated"

	second, err := router.Route(request)
	if err != nil {
		t.Fatalf("second Route() error = %v", err)
	}
	if second.Confidences[0].Language == "mutated" || second.Sections[0].Text == "mutated" ||
		second.ScriptHints[0] == "mutated" || second.ReviewReasons[0] == "mutated" {
		t.Fatalf("Route() exposed shared evidence: %#v", second)
	}
}
