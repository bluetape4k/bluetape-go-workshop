package auditquery

import (
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

var (
	// ErrInvalidConfig 는 사용할 수 없는 service 또는 HTTP configuration을 나타낸다.
	ErrInvalidConfig = errors.New("auditquery: invalid config")
	// ErrInvalidRequest 는 수락할 수 없는 호출자 제공 query를 나타낸다.
	ErrInvalidRequest = errors.New("auditquery: invalid request")
	// ErrEntryNotFound 는 정확한 aggregate revision이 존재하지 않음을 나타낸다.
	ErrEntryNotFound = errors.New("auditquery: entry not found")
)

// ServiceConfig 는 history page 크기를 제어한다.
type ServiceConfig struct {
	DefaultLimit int
	MaximumLimit int
}

// DefaultServiceConfig 는 예제용 보수적인 pagination limit을 반환한다.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{DefaultLimit: 20, MaximumLimit: 100}
}

// AggregateRequest 는 감사 대상 aggregate 하나를 식별한다.
type AggregateRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// SearchRequest 는 aggregate 범위의 history query를 설명한다.
type SearchRequest struct {
	Aggregate      AggregateRequest `json:"aggregate"`
	FromRevision   audit.Revision   `json:"from_revision,omitempty"`
	ToRevision     audit.Revision   `json:"to_revision,omitempty"`
	FromRecordedAt time.Time        `json:"from_recorded_at,omitempty"`
	ToRecordedAt   time.Time        `json:"to_recorded_at,omitempty"`
	NewestFirst    bool             `json:"newest_first,omitempty"`
	Limit          int              `json:"limit,omitempty"`
}

// NextPage 는 continuation 요청에 사용할 배타적 revision boundary를 담는다.
type NextPage struct {
	FromRevision audit.Revision `json:"from_revision,omitempty"`
	ToRevision   audit.Revision `json:"to_revision,omitempty"`
}

// Page 는 현재 결과 크기와 선택적 continuation boundary를 설명한다.
type Page struct {
	Limit   int       `json:"limit"`
	HasMore bool      `json:"has_more"`
	Next    *NextPage `json:"next,omitempty"`
}

// SearchResponse 는 일치하는 history entry와 pagination metadata를 반환한다.
type SearchResponse struct {
	Entries []audit.Entry `json:"entries"`
	Page    Page          `json:"page"`
}
