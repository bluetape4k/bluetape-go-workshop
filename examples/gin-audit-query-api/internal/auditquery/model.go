package auditquery

import (
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

var (
	ErrInvalidConfig  = errors.New("auditquery: invalid config")
	ErrInvalidRequest = errors.New("auditquery: invalid request")
	ErrEntryNotFound  = errors.New("auditquery: entry not found")
)

type ServiceConfig struct {
	DefaultLimit int
	MaximumLimit int
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{DefaultLimit: 20, MaximumLimit: 100}
}

type AggregateRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type SearchRequest struct {
	Aggregate      AggregateRequest `json:"aggregate"`
	FromRevision   audit.Revision   `json:"from_revision,omitempty"`
	ToRevision     audit.Revision   `json:"to_revision,omitempty"`
	FromRecordedAt time.Time        `json:"from_recorded_at,omitempty"`
	ToRecordedAt   time.Time        `json:"to_recorded_at,omitempty"`
	NewestFirst    bool             `json:"newest_first,omitempty"`
	Limit          int              `json:"limit,omitempty"`
}

type NextPage struct {
	FromRevision audit.Revision `json:"from_revision,omitempty"`
	ToRevision   audit.Revision `json:"to_revision,omitempty"`
}

type Page struct {
	Limit   int       `json:"limit"`
	HasMore bool      `json:"has_more"`
	Next    *NextPage `json:"next,omitempty"`
}

type SearchResponse struct {
	Entries []audit.Entry `json:"entries"`
	Page    Page          `json:"page"`
}
