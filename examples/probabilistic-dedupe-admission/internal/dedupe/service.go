// Package dedupe implements the probabilistic admission workshop example.
package dedupe

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/gin-gonic/gin"
)

const maxJSONBodySize = 8 << 10

var (
	// ErrInvalidRequest reports invalid request JSON or fields.
	ErrInvalidRequest = errors.New("dedupe: invalid request")
	// ErrFilter reports unexpected Bloom filter setup or runtime failures.
	ErrFilter = errors.New("dedupe: filter error")
)

// Decision is the public admission decision.
type Decision string

const (
	// DecisionAdmit means the event ID was definitely not present before insertion.
	DecisionAdmit Decision = "admit"
	// DecisionProbablySeen means the event ID might have been seen before.
	DecisionProbablySeen Decision = "probably_seen"
)

const (
	// ReasonDefinitelyNew describes the no-false-negative Bloom filter path.
	ReasonDefinitelyNew = "definitely_new"
	// ReasonMightBeDuplicate describes the possible duplicate or false-positive path.
	ReasonMightBeDuplicate = "might_be_duplicate_or_false_positive"
)

// Config customizes the demo service.
type Config struct {
	ExpectedInsertions       uint64
	FalsePositiveProbability float64
	ScenarioName             string
}

// Service owns the in-memory Bloom filter used by the example.
type Service struct {
	mu           sync.Mutex
	filter       probabilistic.BloomFilter[string]
	scenarioName string
}

// EventRequest is the HTTP and service request for one event admission.
type EventRequest struct {
	EventID string `json:"event_id"`
	Source  string `json:"source"`
}

// AdmitResponse is the stable public admission projection.
type AdmitResponse struct {
	EventID  string      `json:"event_id"`
	Source   string      `json:"source,omitempty"`
	Scenario string      `json:"scenario"`
	Decision Decision    `json:"decision"`
	Reason   string      `json:"reason"`
	Accepted bool        `json:"accepted"`
	Stats    FilterStats `json:"stats"`
}

// FilterStats reports approximate Bloom filter state.
type FilterStats struct {
	ExpectedInsertions                 uint64  `json:"expected_insertions"`
	TargetFalsePositiveProbability     float64 `json:"target_false_positive_probability"`
	ApproximateElementCount            uint64  `json:"approximate_element_count"`
	ExpectedFalsePositiveProbability   float64 `json:"expected_false_positive_probability"`
	BitSize                            uint64  `json:"bit_size"`
	HashFunctionCount                  uint64  `json:"hash_function_count"`
	DurableAuthoritativeStoreRequired  bool    `json:"durable_authoritative_store_required"`
	ProbabilisticFalsePositivePossible bool    `json:"probabilistic_false_positive_possible"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService creates a deterministic in-memory Bloom admission service.
func NewService(config Config) (*Service, error) {
	expectedInsertions := config.ExpectedInsertions
	if expectedInsertions == 0 {
		expectedInsertions = 1_000
	}
	falsePositiveProbability := config.FalsePositiveProbability
	if falsePositiveProbability == 0 {
		falsePositiveProbability = 0.01
	}
	cfg, err := probabilistic.NewConfig(expectedInsertions, falsePositiveProbability)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFilter, err)
	}
	filter, err := probabilistic.NewStringBloomFilter(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFilter, err)
	}
	scenarioName := strings.TrimSpace(config.ScenarioName)
	if scenarioName == "" {
		scenarioName = "webhook-event-dedupe"
	}
	return &Service{
		filter:       filter,
		scenarioName: scenarioName,
	}, nil
}

// Admit evaluates and inserts one event ID through the Bloom prefilter.
func (s *Service) Admit(request EventRequest) (AdmitResponse, error) {
	if s == nil || s.filter == nil {
		return AdmitResponse{}, fmt.Errorf("%w: service is not initialized", ErrFilter)
	}
	eventID := strings.TrimSpace(request.EventID)
	if eventID == "" {
		return AdmitResponse{}, fmt.Errorf("%w: event_id is required", ErrInvalidRequest)
	}
	source := strings.TrimSpace(request.Source)

	s.mu.Lock()
	defer s.mu.Unlock()

	maybeSeen := s.filter.MightContain(eventID)
	if maybeSeen {
		return AdmitResponse{
			EventID:  eventID,
			Source:   source,
			Scenario: s.scenarioName,
			Decision: DecisionProbablySeen,
			Reason:   ReasonMightBeDuplicate,
			Accepted: false,
			Stats:    s.statsLocked(),
		}, nil
	}

	s.filter.Put(eventID)
	return AdmitResponse{
		EventID:  eventID,
		Source:   source,
		Scenario: s.scenarioName,
		Decision: DecisionAdmit,
		Reason:   ReasonDefinitelyNew,
		Accepted: true,
		Stats:    s.statsLocked(),
	}, nil
}

// Stats returns approximate current Bloom filter state.
func (s *Service) Stats() FilterStats {
	if s == nil || s.filter == nil {
		return FilterStats{DurableAuthoritativeStoreRequired: true, ProbabilisticFalsePositivePossible: true}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statsLocked()
}

func (s *Service) statsLocked() FilterStats {
	return FilterStats{
		ExpectedInsertions:                 s.filter.ExpectedInsertions(),
		TargetFalsePositiveProbability:     s.filter.FalsePositiveProbability(),
		ApproximateElementCount:            s.filter.ApproximateElementCount(),
		ExpectedFalsePositiveProbability:   s.filter.ExpectedFPP(),
		BitSize:                            s.filter.BitSize(),
		HashFunctionCount:                  s.filter.HashFunctionCount(),
		DurableAuthoritativeStoreRequired:  true,
		ProbabilisticFalsePositivePossible: true,
	}
}

// NewRouter creates the HTTP router for the example.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/events/admit", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request EventRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid event admission request")
			return
		}
		response, err := service.Admit(request)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	router.GET("/filters/current", func(c *gin.Context) {
		c.JSON(http.StatusOK, service.Stats())
	})

	return router
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid event admission request")
	default:
		writeError(c, http.StatusInternalServerError, "filter_error", "filter operation failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
