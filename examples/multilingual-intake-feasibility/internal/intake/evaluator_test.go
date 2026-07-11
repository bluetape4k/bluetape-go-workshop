package intake

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

func TestEvaluateReportsSupportedLanguagePaths(t *testing.T) {
	evaluator := newTestEvaluator(t)

	tests := []struct {
		name          string
		message       Message
		wantLanguage  string
		wantTokenizer string
	}{
		{
			name:          "English remains explicitly untokenized",
			message:       Message{ID: "ticket-en", Text: "Please help me check the delivery status for order number 12345."},
			wantLanguage:  "English",
			wantTokenizer: TokenizerUnsupported,
		},
		{
			name:          "Korean remains explicitly untokenized",
			message:       Message{ID: "ticket-ko", Text: "주문 번호 12345의 배송 상태를 확인하고 싶습니다."},
			wantLanguage:  "Korean",
			wantTokenizer: TokenizerUnsupported,
		},
		{
			name:          "Japanese uses Kagome",
			message:       Message{ID: "ticket-ja", Text: "配送状況を確認したいです。注文番号を教えてください。"},
			wantLanguage:  "Japanese",
			wantTokenizer: TokenizerKagomeIPA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := evaluator.Evaluate(tt.message)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if report.Language != tt.wantLanguage {
				t.Fatalf("Language = %q, want %q; report = %+v", report.Language, tt.wantLanguage, report)
			}
			if report.Tokenizer != tt.wantTokenizer {
				t.Fatalf("Tokenizer = %q, want %q", report.Tokenizer, tt.wantTokenizer)
			}
			if report.ManualReview {
				t.Fatalf("ManualReview = true; reasons = %#v", report.ReviewReasons)
			}
			if report.Confidence < DefaultConfig().MinimumConfidence {
				t.Fatalf("Confidence = %f, want >= %f", report.Confidence, DefaultConfig().MinimumConfidence)
			}
		})
	}
}

func TestEvaluatePreservesJapaneseByteSpansAndUsefulPOS(t *testing.T) {
	evaluator := newTestEvaluator(t)
	message := Message{ID: "ticket-ja-spans", Text: "日本語で配送状況を確認します。"}

	report, err := evaluator.Evaluate(message)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(report.SelectedTokens) == 0 {
		t.Fatal("SelectedTokens is empty")
	}
	for _, token := range report.SelectedTokens {
		if token.Start < 0 || token.End > len(message.Text) || token.Start >= token.End {
			t.Fatalf("invalid token span: %+v", token)
		}
		if got := message.Text[token.Start:token.End]; got != token.Text {
			t.Fatalf("text[%d:%d] = %q, want %q", token.Start, token.End, got, token.Text)
		}
		if token.POS == "" {
			t.Fatalf("token POS is blank: %+v", token)
		}
	}
}

func TestEvaluateRoutesMixedShortAndUnknownInputToReview(t *testing.T) {
	evaluator := newTestEvaluator(t)

	tests := []struct {
		name       string
		message    Message
		wantReason string
		wantMixed  bool
	}{
		{
			name:       "mixed scripts",
			message:    Message{ID: "ticket-mixed", Text: "Please check 配送状況を確認してください for order 12345."},
			wantReason: ReviewMixedLanguage,
			wantMixed:  true,
		},
		{
			name:       "short text",
			message:    Message{ID: "ticket-short", Text: "help"},
			wantReason: ReviewTextTooShort,
		},
		{
			name:       "unknown script",
			message:    Message{ID: "ticket-unknown", Text: "12345 67890"},
			wantReason: ReviewLanguageUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := evaluator.Evaluate(tt.message)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if !report.ManualReview {
				t.Fatalf("ManualReview = false; report = %+v", report)
			}
			if report.Mixed != tt.wantMixed {
				t.Fatalf("Mixed = %v, want %v", report.Mixed, tt.wantMixed)
			}
			if !contains(report.ReviewReasons, tt.wantReason) {
				t.Fatalf("ReviewReasons = %#v, want %q", report.ReviewReasons, tt.wantReason)
			}
			if report.Tokenizer != TokenizerNone {
				t.Fatalf("Tokenizer = %q, want %q", report.Tokenizer, TokenizerNone)
			}
		})
	}
}

func TestEvaluateRejectsBlankTextAndMissingID(t *testing.T) {
	evaluator := newTestEvaluator(t)

	_, err := evaluator.Evaluate(Message{ID: "ticket-blank", Text: "  "})
	if !errors.Is(err, language.ErrBlankText) {
		t.Fatalf("blank Evaluate() error = %v, want language.ErrBlankText", err)
	}

	_, err = evaluator.Evaluate(Message{Text: "A sufficiently long English support request."})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("missing ID Evaluate() error = %v, want ErrInvalidMessage", err)
	}
}

func TestNewEvaluatorRejectsInvalidConfig(t *testing.T) {
	tests := []Config{
		{MinimumConfidence: -0.1, MinimumRunes: 8},
		{MinimumConfidence: 1.1, MinimumRunes: 8},
		{MinimumConfidence: 0.7, MinimumRunes: 0},
	}
	for _, config := range tests {
		_, err := NewEvaluator(config)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("NewEvaluator(%+v) error = %v, want ErrInvalidConfig", config, err)
		}
	}
}

func TestDefaultConfigDocumentsLazyLifecycle(t *testing.T) {
	config := DefaultConfig()
	if config.PreloadModels {
		t.Fatal("DefaultConfig().PreloadModels = true, want lazy model loading")
	}
	if !reflect.DeepEqual(config, Config{MinimumConfidence: 0.70, MinimumRunes: 8}) {
		t.Fatalf("DefaultConfig() = %+v", config)
	}
}

func TestEvaluatorSharedReuseUnderBoundedConcurrency(t *testing.T) {
	evaluator := newTestEvaluator(t)
	messages := []struct {
		message      Message
		wantLanguage string
	}{
		{Message{ID: "stress-en", Text: "Please check the current delivery status for order 12345."}, "English"},
		{Message{ID: "stress-ko", Text: "주문 번호 12345의 현재 배송 상태를 확인해 주세요."}, "Korean"},
		{Message{ID: "stress-ja", Text: "注文番号12345の現在の配送状況を確認してください。"}, "Japanese"},
	}
	tasks := make([]concurrencytest.Task, 6)
	for i := range tasks {
		fixture := messages[i%len(messages)]
		tasks[i] = func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			report, err := evaluator.Evaluate(fixture.message)
			if err != nil {
				return err
			}
			if report.Language != fixture.wantLanguage || report.ManualReview {
				return fmt.Errorf("report = %+v, want language %q without review", report, fixture.wantLanguage)
			}
			if fixture.wantLanguage == "Japanese" {
				for _, token := range report.SelectedTokens {
					if fixture.message.Text[token.Start:token.End] != token.Text {
						return fmt.Errorf("invalid Japanese byte span: %+v", token)
					}
				}
			}
			return nil
		}
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       6,
		RoundsPerTask: 3,
		Timeout:       5 * time.Second,
	})
	report := tester.RunT(t, tasks...)
	if report.Completed != 18 {
		t.Fatalf("report = %+v, want 18 completed calls", report)
	}
}

func TestNewPreviewDocumentsScenarioAndBoundaryReports(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.Scenario != "evaluate multilingual support intake without hiding heuristic uncertainty" {
		t.Fatalf("Scenario = %q", preview.Scenario)
	}
	if len(preview.Reports) != 6 {
		t.Fatalf("report count = %d, want 6", len(preview.Reports))
	}
	if len(preview.LifecycleNotes) < 2 || len(preview.HeuristicBoundaries) < 3 {
		t.Fatalf("preview notes are incomplete: %+v", preview)
	}

	japaneseReport := previewReport(t, preview, "preview-ja")
	if japaneseReport.Tokenizer != TokenizerKagomeIPA || len(japaneseReport.SelectedTokens) == 0 {
		t.Fatalf("Japanese report = %+v", japaneseReport)
	}
	for _, id := range []string{"preview-mixed", "preview-short", "preview-unknown"} {
		report := previewReport(t, preview, id)
		if !report.ManualReview || len(report.ReviewReasons) == 0 {
			t.Fatalf("report %q = %+v, want review reasons", id, report)
		}
	}
}

func previewReport(t *testing.T, preview Preview, id string) Report {
	t.Helper()
	for _, report := range preview.Reports {
		if report.ID == id {
			return report
		}
	}
	t.Fatalf("preview report %q not found", id)
	return Report{}
}

func newTestEvaluator(t *testing.T) *Evaluator {
	t.Helper()
	evaluator, err := NewEvaluator(DefaultConfig())
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}
	return evaluator
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
