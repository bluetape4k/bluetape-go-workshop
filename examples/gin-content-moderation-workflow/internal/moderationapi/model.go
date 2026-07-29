// Package moderationapi 는 bluetape-go text processing을 Gin-ready workflow로 조합한다.
package moderationapi

import (
	"errors"
	"time"
)

var (
	// ErrInvalidConfig 는 잘못된 생성 또는 zero-value service 사용을 나타낸다.
	ErrInvalidConfig = errors.New("moderationapi: invalid config")
	// ErrInvalidRequest 는 workflow 계약을 벗어난 호출자 입력을 나타낸다.
	ErrInvalidRequest = errors.New("moderationapi: invalid request")
	// ErrDuplicateContentID 는 이미 존재하는 표준 content ID를 나타낸다.
	ErrDuplicateContentID = errors.New("moderationapi: duplicate content id")
	// ErrRecordNotFound 는 누락된 moderation record를 나타낸다.
	ErrRecordNotFound = errors.New("moderationapi: record not found")
	// ErrStoreCapacity 는 제한된 교육용 store가 가득 찼음을 나타낸다.
	ErrStoreCapacity = errors.New("moderationapi: store capacity reached")
	// ErrWorkflow 는 예상하지 못한 detector, tokenizer, matcher 실패를 나타낸다.
	ErrWorkflow = errors.New("moderationapi: workflow failure")
)

// Outcome 은 content item 하나에 저장되는 moderation 결과를 식별한다.
type Outcome string

const (
	// OutcomeAllowed 는 지원되는 content에서 blockword가 발견되지 않았다는 뜻이다.
	OutcomeAllowed Outcome = "allowed"
	// OutcomeMasked 는 지원되는 content에 masking된 blockword가 포함된다는 뜻이다.
	OutcomeMasked Outcome = "masked"
	// OutcomeManualReview 는 불확실하거나 지원되지 않는 content에 review가 필요하다는 뜻이다.
	OutcomeManualReview Outcome = "manual-review"
)

const (
	// ManualReviewDisplayText 는 review되지 않은 text가 search projection에 나타나지 않도록 한다.
	ManualReviewDisplayText = "[pending manual review]"

	// ReasonTextTooShort 는 routing threshold보다 짧은 입력을 식별한다.
	ReasonTextTooShort = "text-too-short"
	// ReasonLanguageUnknown 은 감지된 언어가 없는 입력을 식별한다.
	ReasonLanguageUnknown = "language-unknown"
	// ReasonLowConfidence 는 설정된 confidence보다 낮은 감지 결과를 식별한다.
	ReasonLowConfidence = "low-confidence"
	// ReasonMixedLanguage 는 unknown이 아닌 언어가 여러 개 감지된 입력을 식별한다.
	ReasonMixedLanguage = "mixed-language"
	// ReasonAmbiguousCJKScript 는 Kana 근거 없이 Japanese로 감지된 입력을 식별한다.
	ReasonAmbiguousCJKScript = "ambiguous-cjk-script"
	// ReasonUnsupportedLanguage 는 자동 처리 대상이 아닌 감지 언어를 식별한다.
	ReasonUnsupportedLanguage = "unsupported-language"
)

// AppConfig 는 domain 설정과 HTTP boundary 설정을 묶는다.
type AppConfig struct {
	Service ServiceConfig
	HTTP    HTTPConfig
}

// ServiceConfig 는 routing threshold와 제한된 store limit을 소유한다.
type ServiceConfig struct {
	MinimumConfidence    float64
	MinimumRunes         int
	MaximumContentRunes  int
	MaximumRecords       int
	MaximumSearchResults int
}

// HTTPConfig 는 request body와 workflow deadline limit을 소유한다.
type HTTPConfig struct {
	MaximumBodyBytes int64
	RequestTimeout   time.Duration
}

// DefaultConfig 는 제한된 워크숍 기본값을 반환한다.
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

// CreateRequest 는 호출자 소유 content와 metadata를 담는다.
type CreateRequest struct {
	ContentID string            `json:"content_id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SearchRequest 는 query, 정확한 metadata filter, cursor limit을 담는다.
type SearchRequest struct {
	Query          string            `json:"query"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	AfterContentID string            `json:"after_content_id,omitempty"`
}

// Confidence 는 안정적인 ISO code와 함께 언어 confidence 하나를 projection한다.
type Confidence struct {
	Language string  `json:"language"`
	ISO6391  string  `json:"iso_639_1"`
	ISO6393  string  `json:"iso_639_3"`
	Value    float64 `json:"value"`
}

// Section 은 원본 UTF-8 byte 범위 위에 감지된 language section을 projection한다.
type Section struct {
	Language string `json:"language"`
	ISO6391  string `json:"iso_639_1"`
	ISO6393  string `json:"iso_639_3"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
}

// Finding 은 원본 UTF-8 byte 범위 위에 blockword match 하나를 projection한다.
type Finding struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Start    int               `json:"start"`
	End      int               `json:"end"`
	Severity string            `json:"severity"`
	Metadata map[string]string `json:"metadata"`
}

// PreparedToken 은 Japanese search token 하나와 원본 span을 projection한다.
type PreparedToken struct {
	Text       string            `json:"text"`
	Normalized string            `json:"normalized"`
	BaseForm   string            `json:"base_form,omitempty"`
	POS        string            `json:"pos"`
	Start      int               `json:"start"`
	End        int               `json:"end"`
	Metadata   map[string]string `json:"metadata"`
}

// Record 는 호출자가 소유하는 불변 moderation record projection이다.
type Record struct {
	ContentID     string            `json:"content_id"`
	Content       string            `json:"content"`
	DisplayText   string            `json:"display_text"`
	Metadata      map[string]string `json:"metadata"`
	Outcome       Outcome           `json:"outcome"`
	Language      string            `json:"language"`
	ISO6391       string            `json:"iso_639_1"`
	ISO6393       string            `json:"iso_639_3"`
	Confidence    float64           `json:"confidence"`
	Confidences   []Confidence      `json:"confidences"`
	Sections      []Section         `json:"sections"`
	ScriptHints   []string          `json:"script_hints"`
	ReviewReasons []string          `json:"review_reasons"`
	Findings      []Finding         `json:"findings"`
	Tokens        []PreparedToken   `json:"tokens"`
	Terms         []string          `json:"terms"`
	CreatedAt     time.Time         `json:"created_at"`
}

// SearchHit 은 원본 content와 raw moderation evidence를 생략한다.
type SearchHit struct {
	ContentID   string            `json:"content_id"`
	Outcome     Outcome           `json:"outcome"`
	DisplayText string            `json:"display_text"`
	Metadata    map[string]string `json:"metadata"`
	Language    string            `json:"language"`
}

// SearchResponse 는 제한된 cursor page 하나를 담는다.
type SearchResponse struct {
	Hits               []SearchHit `json:"hits"`
	Truncated          bool        `json:"truncated"`
	NextAfterContentID string      `json:"next_after_content_id,omitempty"`
}
