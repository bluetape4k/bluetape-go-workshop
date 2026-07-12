package moderationapi

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/bluetape4k/bluetape-go/textsearch/japanese"
	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

const defaultSearchLimit = 20

type ServiceOption func(*serviceOptions) error

type serviceOptions struct {
	clock func() time.Time
}

func WithClock(clock func() time.Time) ServiceOption {
	return func(options *serviceOptions) error {
		if clock == nil {
			return fmt.Errorf("%w: clock is nil", ErrInvalidConfig)
		}
		options.clock = clock
		return nil
	}
}

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

func (s *Service) Create(ctx context.Context, _ CreateRequest) (Record, error) {
	if !s.ready() {
		return Record{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	return Record{}, ErrWorkflow
}

func (s *Service) Get(ctx context.Context, _ string) (Record, error) {
	if !s.ready() {
		return Record{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	return Record{}, ErrWorkflow
}

func (s *Service) Search(ctx context.Context, _ SearchRequest) (SearchResponse, error) {
	if !s.ready() {
		return SearchResponse{}, ErrInvalidConfig
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	return SearchResponse{}, ErrWorkflow
}

func (s *Service) ready() bool {
	return s != nil && s.detector != nil && s.japaneseTokenizer != nil && s.simpleTokenizer != nil &&
		s.dictionary != nil && s.clock != nil && s.records != nil
}
