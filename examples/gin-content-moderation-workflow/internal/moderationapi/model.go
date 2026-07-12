// Package moderationapi composes bluetape-go text processing into a Gin-ready workflow.
package moderationapi

import (
	"errors"
	"time"
)

var (
	// ErrInvalidConfig reports invalid construction or zero-value service use.
	ErrInvalidConfig = errors.New("moderationapi: invalid config")
	// ErrInvalidRequest reports caller input outside the workflow contract.
	ErrInvalidRequest = errors.New("moderationapi: invalid request")
	// ErrDuplicateContentID reports an existing canonical content ID.
	ErrDuplicateContentID = errors.New("moderationapi: duplicate content id")
	// ErrRecordNotFound reports a missing moderation record.
	ErrRecordNotFound = errors.New("moderationapi: record not found")
	// ErrStoreCapacity reports that the bounded teaching store is full.
	ErrStoreCapacity = errors.New("moderationapi: store capacity reached")
	// ErrWorkflow reports an unexpected detector, tokenizer, or matcher failure.
	ErrWorkflow = errors.New("moderationapi: workflow failure")
)

// Outcome identifies the moderation result stored for one content item.
type Outcome string

const (
	// OutcomeAllowed means no blockword was found in supported content.
	OutcomeAllowed Outcome = "allowed"
	// OutcomeMasked means supported content contains a masked blockword.
	OutcomeMasked Outcome = "masked"
	// OutcomeManualReview means uncertain or unsupported content needs review.
	OutcomeManualReview Outcome = "manual-review"
)

const (
	// ManualReviewDisplayText prevents unreviewed text from appearing in search projections.
	ManualReviewDisplayText = "[pending manual review]"

	// ReasonTextTooShort identifies input below the routing threshold.
	ReasonTextTooShort = "text-too-short"
	// ReasonLanguageUnknown identifies input without a detected language.
	ReasonLanguageUnknown = "language-unknown"
	// ReasonLowConfidence identifies detection below the configured confidence.
	ReasonLowConfidence = "low-confidence"
	// ReasonMixedLanguage identifies multiple detected non-unknown languages.
	ReasonMixedLanguage = "mixed-language"
	// ReasonAmbiguousCJKScript identifies Japanese detection without Kana evidence.
	ReasonAmbiguousCJKScript = "ambiguous-cjk-script"
	// ReasonUnsupportedLanguage identifies a detected language without automation.
	ReasonUnsupportedLanguage = "unsupported-language"
)

// AppConfig groups domain and HTTP boundary configuration.
type AppConfig struct {
	Service ServiceConfig
	HTTP    HTTPConfig
}

// ServiceConfig owns routing thresholds and bounded store limits.
type ServiceConfig struct {
	MinimumConfidence    float64
	MinimumRunes         int
	MaximumContentRunes  int
	MaximumRecords       int
	MaximumSearchResults int
}

// HTTPConfig owns request body and workflow deadline limits.
type HTTPConfig struct {
	MaximumBodyBytes int64
	RequestTimeout   time.Duration
}

// DefaultConfig returns the bounded workshop defaults.
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

// CreateRequest contains caller-owned content and metadata.
type CreateRequest struct {
	ContentID string            `json:"content_id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SearchRequest contains a query, exact metadata filters, and cursor limits.
type SearchRequest struct {
	Query          string            `json:"query"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	AfterContentID string            `json:"after_content_id,omitempty"`
}

// Confidence projects one language confidence with stable ISO codes.
type Confidence struct {
	Language string  `json:"language"`
	ISO6391  string  `json:"iso_639_1"`
	ISO6393  string  `json:"iso_639_3"`
	Value    float64 `json:"value"`
}

// Section projects a detected language section over original UTF-8 bytes.
type Section struct {
	Language string `json:"language"`
	ISO6391  string `json:"iso_639_1"`
	ISO6393  string `json:"iso_639_3"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Text     string `json:"text"`
}

// Finding projects one blockword match over original UTF-8 bytes.
type Finding struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Start    int               `json:"start"`
	End      int               `json:"end"`
	Severity string            `json:"severity"`
	Metadata map[string]string `json:"metadata"`
}

// PreparedToken projects one Japanese search token and its original span.
type PreparedToken struct {
	Text       string            `json:"text"`
	Normalized string            `json:"normalized"`
	BaseForm   string            `json:"base_form,omitempty"`
	POS        string            `json:"pos"`
	Start      int               `json:"start"`
	End        int               `json:"end"`
	Metadata   map[string]string `json:"metadata"`
}

// Record is the immutable caller-owned moderation record projection.
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

// SearchHit omits original content and raw moderation evidence.
type SearchHit struct {
	ContentID   string            `json:"content_id"`
	Outcome     Outcome           `json:"outcome"`
	DisplayText string            `json:"display_text"`
	Metadata    map[string]string `json:"metadata"`
	Language    string            `json:"language"`
}

// SearchResponse contains one bounded cursor page.
type SearchResponse struct {
	Hits               []SearchHit `json:"hits"`
	Truncated          bool        `json:"truncated"`
	NextAfterContentID string      `json:"next_after_content_id,omitempty"`
}
