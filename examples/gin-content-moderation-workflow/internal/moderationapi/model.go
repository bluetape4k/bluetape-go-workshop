package moderationapi

import (
	"errors"
	"time"
)

var (
	ErrInvalidConfig      = errors.New("moderationapi: invalid config")
	ErrInvalidRequest     = errors.New("moderationapi: invalid request")
	ErrDuplicateContentID = errors.New("moderationapi: duplicate content id")
	ErrRecordNotFound     = errors.New("moderationapi: record not found")
	ErrStoreCapacity      = errors.New("moderationapi: store capacity reached")
	ErrWorkflow           = errors.New("moderationapi: workflow failure")
)

type Outcome string

const (
	OutcomeAllowed      Outcome = "allowed"
	OutcomeMasked       Outcome = "masked"
	OutcomeManualReview Outcome = "manual-review"
)

type AppConfig struct {
	Service ServiceConfig
	HTTP    HTTPConfig
}

type ServiceConfig struct {
	MinimumConfidence    float64
	MinimumRunes         int
	MaximumContentRunes  int
	MaximumRecords       int
	MaximumSearchResults int
}

type HTTPConfig struct {
	MaximumBodyBytes int64
	RequestTimeout   time.Duration
}

func DefaultConfig() AppConfig {
	return AppConfig{
		Service: ServiceConfig{
			MinimumConfidence:    0.70,
			MinimumRunes:         8,
			MaximumContentRunes:  8_000,
			MaximumRecords:       1_000,
			MaximumSearchResults: 100,
		},
		HTTP: HTTPConfig{
			MaximumBodyBytes: 64 << 10,
			RequestTimeout:   2 * time.Second,
		},
	}
}

type CreateRequest struct {
	ContentID string
	Content   string
	Metadata  map[string]string
}

type SearchRequest struct {
	Query          string
	Metadata       map[string]string
	Limit          int
	AfterContentID string
}

type Record struct {
	ContentID string
	Content   string
	Outcome   Outcome
}

type SearchResponse struct {
	Hits               []Record
	Truncated          bool
	NextAfterContentID string
}
