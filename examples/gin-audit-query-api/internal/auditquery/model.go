package auditquery

import (
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

var (
	// ErrInvalidConfig reports an unusable service or HTTP configuration.
	ErrInvalidConfig = errors.New("auditquery: invalid config")
	// ErrInvalidRequest reports a caller-provided query that cannot be accepted.
	ErrInvalidRequest = errors.New("auditquery: invalid request")
	// ErrEntryNotFound reports that an exact aggregate revision does not exist.
	ErrEntryNotFound = errors.New("auditquery: entry not found")
)

// ServiceConfig controls history page sizes.
type ServiceConfig struct {
	DefaultLimit int
	MaximumLimit int
}

// DefaultServiceConfig returns conservative pagination limits for the example.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{DefaultLimit: 20, MaximumLimit: 100}
}

// AggregateRequest identifies one audited aggregate.
type AggregateRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// SearchRequest describes an aggregate-scoped history query.
type SearchRequest struct {
	Aggregate      AggregateRequest `json:"aggregate"`
	FromRevision   audit.Revision   `json:"from_revision,omitempty"`
	ToRevision     audit.Revision   `json:"to_revision,omitempty"`
	FromRecordedAt time.Time        `json:"from_recorded_at,omitempty"`
	ToRecordedAt   time.Time        `json:"to_recorded_at,omitempty"`
	NewestFirst    bool             `json:"newest_first,omitempty"`
	Limit          int              `json:"limit,omitempty"`
}

// NextPage contains the exclusive revision boundary for a continuation request.
type NextPage struct {
	FromRevision audit.Revision `json:"from_revision,omitempty"`
	ToRevision   audit.Revision `json:"to_revision,omitempty"`
}

// Page describes the current result size and optional continuation boundary.
type Page struct {
	Limit   int       `json:"limit"`
	HasMore bool      `json:"has_more"`
	Next    *NextPage `json:"next,omitempty"`
}

// SearchResponse returns matching history entries and pagination metadata.
type SearchResponse struct {
	Entries []audit.Entry `json:"entries"`
	Page    Page          `json:"page"`
}
