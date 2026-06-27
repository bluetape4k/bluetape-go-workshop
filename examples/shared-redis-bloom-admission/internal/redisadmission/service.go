// Package redisadmission implements the shared Redis Bloom admission example.
package redisadmission

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	redisbloom "github.com/bluetape4k/bluetape-go/probabilistic/redis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const maxJSONBodySize = 8 << 10

var (
	// ErrInvalidRequest reports invalid request JSON or fields.
	ErrInvalidRequest = errors.New("redisadmission: invalid request")
	// ErrInvalidConfig reports invalid Bloom filter configuration.
	ErrInvalidConfig = errors.New("redisadmission: invalid config")
	// ErrConfigMismatch reports a Redis Bloom namespace initialized with different metadata.
	ErrConfigMismatch = errors.New("redisadmission: redis bloom config mismatch")
	// ErrFilterUnavailable reports Redis-backed filter failures.
	ErrFilterUnavailable = errors.New("redisadmission: redis bloom unavailable")
)

// Decision is the public admission decision.
type Decision string

const (
	// DecisionAdmit means at least one Bloom bit changed, so this value is definitely new to the filter.
	DecisionAdmit Decision = "admit"
	// DecisionProbablySeen means all Bloom bits were already set, so the value may be duplicate or a false positive.
	DecisionProbablySeen Decision = "probably_seen"
)

const (
	// ReasonDefinitelyNew describes the no-false-negative first-insert path.
	ReasonDefinitelyNew = "definitely_new"
	// ReasonMightBeDuplicate describes the duplicate-or-false-positive path.
	ReasonMightBeDuplicate = "might_be_duplicate_or_false_positive"
)

// Config customizes the shared Redis Bloom admission service.
type Config struct {
	Namespace                string
	InstanceID               string
	ScenarioName             string
	ExpectedInsertions       uint64
	FalsePositiveProbability float64
}

// Service owns one application instance using a shared Redis Bloom filter.
type Service struct {
	filter       redisbloom.BloomFilter[string]
	namespace    string
	instanceID   string
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
	Instance string      `json:"instance"`
	Decision Decision    `json:"decision"`
	Reason   string      `json:"reason"`
	Accepted bool        `json:"accepted"`
	Stats    FilterStats `json:"stats"`
}

// FilterStats reports approximate Redis Bloom filter state and reader caveats.
type FilterStats struct {
	Namespace                          string  `json:"namespace"`
	ExpectedInsertions                 uint64  `json:"expected_insertions"`
	TargetFalsePositiveProbability     float64 `json:"target_false_positive_probability"`
	ApproximateElementCount            uint64  `json:"approximate_element_count"`
	ExpectedFalsePositiveProbability   float64 `json:"expected_false_positive_probability"`
	BitCount                           uint64  `json:"bit_count"`
	BitSize                            uint64  `json:"bit_size"`
	HashFunctionCount                  uint64  `json:"hash_function_count"`
	HasherKey                          string  `json:"hasher_key"`
	SharedRedisState                   bool    `json:"shared_redis_state"`
	AuthoritativeStoreRequired         bool    `json:"authoritative_store_required"`
	ProbabilisticFalsePositivePossible bool    `json:"probabilistic_false_positive_possible"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService creates a Redis-backed Bloom admission service.
func NewService(ctx context.Context, client redis.Cmdable, config Config) (*Service, error) {
	cfg, err := bloomConfig(config)
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(config.Namespace)
	if namespace == "" {
		namespace = "shared-redis-bloom-admission:webhooks"
	}
	instanceID := strings.TrimSpace(config.InstanceID)
	if instanceID == "" {
		instanceID = "api-instance"
	}
	scenarioName := strings.TrimSpace(config.ScenarioName)
	if scenarioName == "" {
		scenarioName = "shared-webhook-admission"
	}

	filter, err := redisbloom.NewStringBloomFilter(ctx, client, namespace, cfg)
	if err != nil {
		return nil, mapFilterError(err)
	}
	return &Service{
		filter:       filter,
		namespace:    namespace,
		instanceID:   instanceID,
		scenarioName: scenarioName,
	}, nil
}

// Admit evaluates and inserts one event ID through the shared Bloom prefilter.
func (s *Service) Admit(ctx context.Context, request EventRequest) (AdmitResponse, error) {
	if s == nil || s.filter == nil {
		return AdmitResponse{}, ErrFilterUnavailable
	}
	eventID := strings.TrimSpace(request.EventID)
	if eventID == "" {
		return AdmitResponse{}, fmt.Errorf("%w: event_id is required", ErrInvalidRequest)
	}
	source := strings.TrimSpace(request.Source)

	changed, err := s.filter.Put(ctx, eventID)
	if err != nil {
		return AdmitResponse{}, mapFilterError(err)
	}
	stats, err := s.Stats(ctx)
	if err != nil {
		return AdmitResponse{}, err
	}
	if changed {
		return AdmitResponse{
			EventID:  eventID,
			Source:   source,
			Scenario: s.scenarioName,
			Instance: s.instanceID,
			Decision: DecisionAdmit,
			Reason:   ReasonDefinitelyNew,
			Accepted: true,
			Stats:    stats,
		}, nil
	}
	return AdmitResponse{
		EventID:  eventID,
		Source:   source,
		Scenario: s.scenarioName,
		Instance: s.instanceID,
		Decision: DecisionProbablySeen,
		Reason:   ReasonMightBeDuplicate,
		Accepted: false,
		Stats:    stats,
	}, nil
}

// Stats returns approximate current Bloom filter state.
func (s *Service) Stats(ctx context.Context) (FilterStats, error) {
	if s == nil || s.filter == nil {
		return FilterStats{}, ErrFilterUnavailable
	}
	bitCount, err := s.filter.BitCount(ctx)
	if err != nil {
		return FilterStats{}, mapFilterError(err)
	}
	approximateCount, err := s.filter.ApproximateElementCount(ctx)
	if err != nil {
		return FilterStats{}, mapFilterError(err)
	}
	expectedFPP, err := s.filter.ExpectedFPP(ctx)
	if err != nil {
		return FilterStats{}, mapFilterError(err)
	}
	return FilterStats{
		Namespace:                          s.namespace,
		ExpectedInsertions:                 s.filter.ExpectedInsertions(),
		TargetFalsePositiveProbability:     s.filter.FalsePositiveProbability(),
		ApproximateElementCount:            approximateCount,
		ExpectedFalsePositiveProbability:   expectedFPP,
		BitCount:                           bitCount,
		BitSize:                            s.filter.BitSize(),
		HashFunctionCount:                  s.filter.HashFunctionCount(),
		HasherKey:                          s.filter.HasherKey(),
		SharedRedisState:                   true,
		AuthoritativeStoreRequired:         true,
		ProbabilisticFalsePositivePossible: true,
	}, nil
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
		response, err := service.Admit(c.Request.Context(), request)
		if err != nil {
			writeServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	router.GET("/filters/current", func(c *gin.Context) {
		stats, err := service.Stats(c.Request.Context())
		if err != nil {
			writeServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, stats)
	})

	return router
}

func bloomConfig(config Config) (probabilistic.Config, error) {
	expectedInsertions := config.ExpectedInsertions
	if expectedInsertions == 0 {
		expectedInsertions = 10_000
	}
	falsePositiveProbability := config.FalsePositiveProbability
	if falsePositiveProbability == 0 {
		falsePositiveProbability = 0.01
	}
	cfg, err := probabilistic.NewConfig(expectedInsertions, falsePositiveProbability)
	if err != nil {
		return probabilistic.Config{}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	return cfg, nil
}

func mapFilterError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, redisbloom.ErrConfigMismatch):
		return fmt.Errorf("%w: %w", ErrConfigMismatch, err)
	case errors.Is(err, redisbloom.ErrInvalidOptions), errors.Is(err, probabilistic.ErrInvalidConfig):
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	default:
		return fmt.Errorf("%w: %w", ErrFilterUnavailable, err)
	}
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid event admission request")
	case errors.Is(err, ErrInvalidConfig), errors.Is(err, ErrConfigMismatch):
		writeError(c, http.StatusConflict, "filter_config_mismatch", "redis bloom filter configuration does not match")
	case errors.Is(err, ErrFilterUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusServiceUnavailable, "filter_unavailable", "redis bloom filter unavailable")
	default:
		writeError(c, http.StatusInternalServerError, "filter_error", "filter operation failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
