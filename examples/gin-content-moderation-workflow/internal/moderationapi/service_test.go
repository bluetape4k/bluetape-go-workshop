package moderationapi

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
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
