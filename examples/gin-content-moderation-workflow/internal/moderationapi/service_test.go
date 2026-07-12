package moderationapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.Service.MinimumConfidence != 0.70 || config.Service.MinimumRunes != 8 {
		t.Fatalf("routing defaults = %+v", config.Service)
	}
	if config.Service.MaximumContentRunes != 8_000 || config.Service.MaximumRecords != 1_000 || config.Service.MaximumSearchResults != 100 {
		t.Fatalf("service limits = %+v", config.Service)
	}
	if config.HTTP.MaximumBodyBytes != 64<<10 || config.HTTP.RequestTimeout != 2*time.Second {
		t.Fatalf("HTTP defaults = %+v", config.HTTP)
	}
}

func TestNewServiceRejectsInvalidConfidence(t *testing.T) {
	for _, confidence := range []float64{-0.01, 1.01, math.NaN(), math.Inf(-1), math.Inf(1)} {
		config := DefaultConfig().Service
		config.MinimumConfidence = confidence
		_, err := NewService(config)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("confidence %v: err = %v, want ErrInvalidConfig", confidence, err)
		}
	}
}

func TestNewServiceRejectsInvalidLimits(t *testing.T) {
	tests := map[string]func(*ServiceConfig){
		"minimum runes":          func(c *ServiceConfig) { c.MinimumRunes = 0 },
		"negative minimum runes": func(c *ServiceConfig) { c.MinimumRunes = -1 },
		"maximum content runes":  func(c *ServiceConfig) { c.MaximumContentRunes = 0 },
		"negative content runes": func(c *ServiceConfig) { c.MaximumContentRunes = -1 },
		"maximum records":        func(c *ServiceConfig) { c.MaximumRecords = 0 },
		"negative records":       func(c *ServiceConfig) { c.MaximumRecords = -1 },
		"zero search results":    func(c *ServiceConfig) { c.MaximumSearchResults = 0 },
		"negative search results": func(c *ServiceConfig) {
			c.MaximumSearchResults = -1
		},
		"maximum search results": func(c *ServiceConfig) { c.MaximumSearchResults = 19 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := DefaultConfig().Service
			mutate(&config)
			_, err := NewService(config)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestNewServiceBuildsSharedComponents(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	service, err := NewService(DefaultConfig().Service, WithClock(func() time.Time { return fixedTime }))
	if err != nil {
		t.Fatal(err)
	}
	if service.detector == nil || service.japaneseTokenizer == nil || service.dictionary == nil || service.clock == nil || service.records == nil {
		t.Fatalf("service not initialized: %+v", service)
	}
	languages := service.detector.Languages()
	for _, want := range []language.Language{language.English, language.Korean, language.Japanese, language.Chinese} {
		if !slices.Contains(languages, want) {
			t.Fatalf("languages = %v, missing %v", languages, want)
		}
	}
	if len(languages) != 4 {
		t.Fatalf("languages = %v, want exactly four", languages)
	}
	if got := service.clock(); !got.Equal(fixedTime) {
		t.Fatalf("clock = %v, want %v", got, fixedTime)
	}

	simpleRequest, err := textsearch.NewTokenizeRequest("Delivery 42!", textsearch.TokenizeOptions{Normalize: textsearch.NormalizeNFC})
	if err != nil {
		t.Fatal(err)
	}
	simpleResponse, err := service.simpleTokenizer.Tokenize(simpleRequest)
	if err != nil {
		t.Fatal(err)
	}
	if got := simpleResponse.Texts(); !reflect.DeepEqual(got, []string{"Delivery", "42", "!"}) {
		t.Fatalf("simple tokens = %#v", got)
	}

	japaneseRequest, err := textsearch.NewTokenizeRequest("関西国際空港に行く", textsearch.TokenizeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	japaneseResponse, err := service.japaneseTokenizer.Tokenize(japaneseRequest)
	if err != nil {
		t.Fatal(err)
	}
	if got := japaneseResponse.Texts(); !reflect.DeepEqual(got, []string{"関西", "国際", "空港", "に", "行く"}) {
		t.Fatalf("Japanese Search tokens = %#v", got)
	}

	blockRequest, err := textsearch.NewBlockwordRequest("bad wolf 욕설", service.blockwordOptions)
	if err != nil {
		t.Fatal(err)
	}
	blockResponse, err := service.dictionary.Process(blockRequest)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockResponse.Matches) != 2 || blockResponse.Matches[0].Entry.ID != "abuse-bad-wolf" || blockResponse.Matches[1].Entry.ID != "ko-abuse" {
		t.Fatalf("blockword matches = %+v", blockResponse.Matches)
	}
	for _, match := range blockResponse.Matches {
		if match.Entry.Severity < textsearch.SeverityMiddle || match.Entry.Metadata["category"] == "" {
			t.Fatalf("blockword policy match = %+v", match)
		}
	}
}

func TestNewServiceRejectsNilClock(t *testing.T) {
	_, err := NewService(DefaultConfig().Service, WithClock(nil))
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestZeroValueServiceFailsClosed(t *testing.T) {
	var service Service
	_, err := service.Create(context.Background(), CreateRequest{ContentID: "record-1", Content: "valid content"})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestServicePreservesCanceledAndDeadlineContexts(t *testing.T) {
	service, err := NewService(DefaultConfig().Service)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer expire()

	for name, test := range map[string]struct {
		ctx  context.Context
		want error
	}{
		"canceled": {ctx: canceled, want: context.Canceled},
		"deadline": {ctx: expired, want: context.DeadlineExceeded},
	} {
		t.Run(name, func(t *testing.T) {
			_, createErr := service.Create(test.ctx, CreateRequest{ContentID: "record-1", Content: "valid content"})
			_, getErr := service.Get(test.ctx, "record-1")
			_, searchErr := service.Search(test.ctx, SearchRequest{Query: "valid"})
			for operation, err := range map[string]error{"create": createErr, "get": getErr, "search": searchErr} {
				if !errors.Is(err, test.want) {
					t.Fatalf("%s err = %v, want %v", operation, err, test.want)
				}
			}
		})
	}
}

func TestCreateRoutesSupportedAndReviewContent(t *testing.T) {
	service := newCreateTestService(t)
	tests := []struct {
		id       string
		content  string
		outcome  Outcome
		language string
		reason   string
	}{
		{id: "en", content: "Please review the delivery status for order 12345.", outcome: OutcomeAllowed, language: "English"},
		{id: "ko", content: "주문 번호 12345의 배송 상태를 확인해 주세요.", outcome: OutcomeAllowed, language: "Korean"},
		{id: "ja", content: "配送状況を確認してください。注文番号は12345です。", outcome: OutcomeAllowed, language: "Japanese"},
		{id: "zh", content: "请确认订单12345的配送状态。", outcome: OutcomeManualReview, language: "Chinese", reason: ReasonUnsupportedLanguage},
		{id: "short", content: "help", outcome: OutcomeManualReview, language: "English", reason: ReasonTextTooShort},
		{id: "unknown", content: "12345 67890", outcome: OutcomeManualReview, language: "unknown", reason: ReasonLanguageUnknown},
		{id: "mixed", content: "Please check 配送状況を確認してください for order 12345.", outcome: OutcomeManualReview, language: "Japanese", reason: ReasonMixedLanguage},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			record, err := service.Create(context.Background(), CreateRequest{ContentID: "  " + test.id + "  ", Content: test.content})
			if err != nil {
				t.Fatal(err)
			}
			if record.ContentID != test.id || record.Content != test.content || record.Outcome != test.outcome || record.Language != test.language {
				t.Fatalf("record = %+v", record)
			}
			if test.reason != "" && !slices.Contains(record.ReviewReasons, test.reason) {
				t.Fatalf("reasons = %v, want %q", record.ReviewReasons, test.reason)
			}
			if test.outcome == OutcomeManualReview && record.DisplayText != ManualReviewDisplayText {
				t.Fatalf("display text = %q", record.DisplayText)
			}
			if record.Confidences == nil || record.Sections == nil || record.ScriptHints == nil || record.ReviewReasons == nil {
				t.Fatalf("nil evidence slice: %+v", record)
			}
		})
	}
}

func TestCreateUsesLowConfidenceAndHanOnlyFallbacks(t *testing.T) {
	config := DefaultConfig().Service
	config.MinimumConfidence = 1.0
	service, err := NewService(config)
	if err != nil {
		t.Fatal(err)
	}
	low, err := service.Create(context.Background(), CreateRequest{ContentID: "low", Content: "support 문의 订单 delivery"})
	if err != nil {
		t.Fatal(err)
	}
	if low.Outcome != OutcomeManualReview || !slices.Contains(low.ReviewReasons, ReasonLowConfidence) {
		t.Fatalf("low confidence record = %+v", low)
	}

	hanService := newCreateTestService(t)
	han, err := hanService.Create(context.Background(), CreateRequest{ContentID: "han", Content: "注文番号配送確認依頼"})
	if err != nil {
		t.Fatal(err)
	}
	if han.Outcome != OutcomeManualReview || (!slices.Contains(han.ReviewReasons, ReasonAmbiguousCJKScript) && !slices.Contains(han.ReviewReasons, ReasonUnsupportedLanguage)) {
		t.Fatalf("Han-only record = %+v", han)
	}
}

func TestCreateMasksKoreanAndPreservesByteSpan(t *testing.T) {
	service := newCreateTestService(t)
	record, err := service.Create(context.Background(), CreateRequest{ContentID: "ko-mask", Content: "배송 욕설 문의입니다"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Outcome != OutcomeMasked || record.DisplayText != "배송 ** 문의입니다" || len(record.Findings) != 1 {
		t.Fatalf("record = %+v", record)
	}
	finding := record.Findings[0]
	if got := record.Content[finding.Start:finding.End]; got != "욕설" {
		t.Fatalf("span %d:%d = %q", finding.Start, finding.End, got)
	}
}

func TestCreatePreparesJapaneseSearchTermsAndSpans(t *testing.T) {
	service := newCreateTestService(t)
	record, err := service.Create(context.Background(), CreateRequest{ContentID: "ja-terms", Content: "配送状況を確認してください。注文番号は12345です。"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(record.Terms, "配送") || !slices.Contains(record.Terms, "確認") || len(record.Tokens) == 0 {
		t.Fatalf("Japanese terms/tokens = %+v / %+v", record.Terms, record.Tokens)
	}
	for _, token := range record.Tokens {
		if got := record.Content[token.Start:token.End]; got != token.Text {
			t.Fatalf("token span %d:%d = %q, want %q", token.Start, token.End, got, token.Text)
		}
	}
}

func TestCreateValidatesInputAndCopiesMetadata(t *testing.T) {
	service := newCreateTestService(t)
	invalid := []CreateRequest{
		{ContentID: "", Content: "Please review valid content."},
		{ContentID: "bad/id", Content: "Please review valid content."},
		{ContentID: strings.Repeat("a", 129), Content: "Please review valid content."},
		{ContentID: "blank", Content: " \t\n"},
		{ContentID: "metadata-empty", Content: "Please review valid content.", Metadata: map[string]string{" ": "value"}},
		{ContentID: "metadata-collision", Content: "Please review valid content.", Metadata: map[string]string{"tenant": "a", " tenant ": "b"}},
	}
	for _, request := range invalid {
		if _, err := service.Create(context.Background(), request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("request %+v: err = %v", request, err)
		}
	}

	metadata := map[string]string{" tenant ": "demo"}
	record, err := service.Create(context.Background(), CreateRequest{ContentID: "metadata-copy", Content: "Please review valid delivery content.", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	metadata[" tenant "] = "mutated"
	record.Metadata["tenant"] = "returned mutation"
	stored := service.records["metadata-copy"]
	if stored.Metadata["tenant"] != "demo" {
		t.Fatalf("stored metadata = %v", stored.Metadata)
	}
}

func TestCreateRejectsDuplicateBeforeCapacity(t *testing.T) {
	config := DefaultConfig().Service
	config.MaximumRecords = 1
	service, err := NewService(config)
	if err != nil {
		t.Fatal(err)
	}
	request := CreateRequest{ContentID: "only", Content: "Please review valid delivery content."}
	first, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), CreateRequest{ContentID: " only ", Content: "This payload must never overwrite the first record."}); !errors.Is(err, ErrDuplicateContentID) {
		t.Fatalf("duplicate err = %v", err)
	}
	if _, err := service.Create(context.Background(), CreateRequest{ContentID: "second", Content: "Please review another valid delivery content."}); !errors.Is(err, ErrStoreCapacity) {
		t.Fatalf("capacity err = %v", err)
	}
	if stored := service.records["only"]; stored.Content != first.Content {
		t.Fatalf("stored record overwritten: %+v", stored)
	}
}

func TestCreateCanceledByClockDoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service, err := NewService(DefaultConfig().Service, WithClock(func() time.Time {
		cancel()
		return time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Create(ctx, CreateRequest{ContentID: "cancel-before-commit", Content: "Please review valid delivery content."})
	if !errors.Is(err, context.Canceled) || len(service.records) != 0 {
		t.Fatalf("err = %v, records = %d", err, len(service.records))
	}
}

func TestCreateChecksCancellationBetweenWorkflowStages(t *testing.T) {
	for _, cancelAt := range []int{3, 4, 5} {
		service := newCreateTestService(t)
		ctx := &countingCanceledContext{cancelAt: cancelAt}
		_, err := service.Create(ctx, CreateRequest{ContentID: fmt.Sprintf("cancel-stage-%d", cancelAt), Content: "Please review valid delivery content."})
		if !errors.Is(err, context.Canceled) || len(service.records) != 0 {
			t.Fatalf("cancelAt %d: err = %v, records = %d, checks = %d", cancelAt, err, len(service.records), ctx.calls)
		}
	}
}

func TestCreateChecksContextAtEveryWorkflowStage(t *testing.T) {
	service := newCreateTestService(t)
	ctx := &countingCanceledContext{cancelAt: 100}
	_, err := service.Create(ctx, CreateRequest{ContentID: "context-checkpoints", Content: "Please review valid delivery content."})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.calls < 9 {
		t.Fatalf("context checks = %d, want at least 9 stage and commit checks", ctx.calls)
	}
}

func TestCreateReturnsDeepCopiesOfEveryRecordLayer(t *testing.T) {
	service := newCreateTestService(t)
	mixed, err := service.Create(context.Background(), CreateRequest{ContentID: "copy-mixed", Content: "Please check 配送状況を確認してください for order 12345."})
	if err != nil {
		t.Fatal(err)
	}
	masked, err := service.Create(context.Background(), CreateRequest{ContentID: "copy-masked", Content: "배송 욕설 문의입니다"})
	if err != nil {
		t.Fatal(err)
	}
	japaneseRecord, err := service.Create(context.Background(), CreateRequest{ContentID: "copy-ja", Content: "配送状況を確認してください。注文番号は12345です。"})
	if err != nil {
		t.Fatal(err)
	}

	mixed.Metadata["new"] = "mutation"
	mixed.Confidences[0].Language = "mutation"
	mixed.Sections[0].Text = "mutation"
	mixed.ScriptHints[0] = "mutation"
	mixed.ReviewReasons[0] = "mutation"
	masked.Findings[0].Text = "mutation"
	masked.Findings[0].Metadata["category"] = "mutation"
	masked.Terms[0] = "mutation"
	japaneseRecord.Tokens[0].Text = "mutation"
	japaneseRecord.Tokens[0].Metadata["language"] = "mutation"
	japaneseRecord.Terms[0] = "mutation"

	storedMixed := service.records["copy-mixed"]
	storedMasked := service.records["copy-masked"]
	storedJapanese := service.records["copy-ja"]
	if storedMixed.Confidences[0].Language == "mutation" || storedMixed.Sections[0].Text == "mutation" || storedMixed.ScriptHints[0] == "mutation" || storedMixed.ReviewReasons[0] == "mutation" || storedMixed.Metadata["new"] == "mutation" {
		t.Fatalf("mixed stored record mutated: %+v", storedMixed)
	}
	if storedMasked.Findings[0].Text == "mutation" || storedMasked.Findings[0].Metadata["category"] == "mutation" || storedMasked.Terms[0] == "mutation" {
		t.Fatalf("masked stored record mutated: %+v", storedMasked)
	}
	if storedJapanese.Tokens[0].Text == "mutation" || storedJapanese.Tokens[0].Metadata["language"] == "mutation" || storedJapanese.Terms[0] == "mutation" {
		t.Fatalf("Japanese stored record mutated: %+v", storedJapanese)
	}
}

func TestTermProjectionNormalizationFilteringAndDeduplication(t *testing.T) {
	service := newCreateTestService(t)
	simple, err := service.prepareSimpleTerms("DELIVERY delivery 42 42 café cafe\u0301 !")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(simple, []string{"delivery", "42", "café"}) {
		t.Fatalf("simple terms = %#v", simple)
	}
	korean, err := service.prepareSimpleTerms("배송 배송 123 123")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(korean, []string{"배송", "123"}) {
		t.Fatalf("Korean terms = %#v", korean)
	}
	tokens, terms, err := service.prepareJapaneseTerms("寿司を食べた。寿司を食べた。")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(terms, []string{"寿司", "食べる"}) {
		t.Fatalf("Japanese terms = %#v", terms)
	}
	for _, token := range tokens {
		if token.Text == "を" || token.Text == "。" {
			t.Fatalf("non noun/verb token selected: %+v", token)
		}
	}
	if got := japaneseIndexTerm(PreparedToken{Text: "カ\u3099"}); got != "ガ" {
		t.Fatalf("decomposed fallback term = %q", got)
	}
	if got := japaneseIndexTerm(PreparedToken{Text: "食べた", BaseForm: "食べる"}); got != "食べる" {
		t.Fatalf("base-form term = %q", got)
	}
}

func TestCreateValidatesUTF8RuneAndIDBoundaries(t *testing.T) {
	config := DefaultConfig().Service
	config.MaximumContentRunes = 8
	service, err := NewService(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []CreateRequest{
		{ContentID: "bad-utf8", Content: string([]byte{0xff})},
		{ContentID: "too-long", Content: "123456789"},
	} {
		if _, err := service.Create(context.Background(), request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("request %+v: err = %v", request, err)
		}
	}
	for _, request := range []CreateRequest{
		{ContentID: "a", Content: "12345678", Metadata: map[string]string{"scope": "  exact value  "}},
		{ContentID: strings.Repeat("z", 128), Content: "abcdefgh"},
	} {
		record, err := service.Create(context.Background(), request)
		if err != nil {
			t.Fatalf("boundary request %+v: %v", request, err)
		}
		if request.Metadata != nil && record.Metadata["scope"] != "  exact value  " {
			t.Fatalf("metadata value = %q", record.Metadata["scope"])
		}
	}
}

type countingCanceledContext struct {
	cancelAt int
	calls    int
	canceled bool
}

func (c *countingCanceledContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *countingCanceledContext) Done() <-chan struct{}       { return nil }
func (c *countingCanceledContext) Value(any) any               { return nil }
func (c *countingCanceledContext) Err() error {
	c.calls++
	if c.canceled || c.calls >= c.cancelAt {
		c.canceled = true
		return context.Canceled
	}
	return nil
}

func newCreateTestService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService(DefaultConfig().Service, WithClock(func() time.Time {
		return time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	}))
	if err != nil {
		t.Fatal(err)
	}
	return service
}
