// Package dedupe 는 확률적 입장 판단 워크숍 예제를 구현한다.
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
	// ErrInvalidRequest 는 요청 JSON 또는 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("dedupe: invalid request")
	// ErrFilter 는 예상하지 못한 Bloom filter 설정 또는 런타임 실패를 나타낸다.
	ErrFilter = errors.New("dedupe: filter error")
)

// Decision 은 공개 입장 판단 값이다.
type Decision string

const (
	// DecisionAdmit 은 이벤트 ID가 삽입 전에는 확실히 없었음을 의미한다.
	DecisionAdmit Decision = "admit"
	// DecisionProbablySeen 은 이벤트 ID가 이전에 관측됐을 가능성이 있음을 의미한다.
	DecisionProbablySeen Decision = "probably_seen"
)

const (
	// ReasonDefinitelyNew 는 false negative 가 없는 Bloom filter 경로를 설명한다.
	ReasonDefinitelyNew = "definitely_new"
	// ReasonMightBeDuplicate 는 중복 가능성 또는 false positive 경로를 설명한다.
	ReasonMightBeDuplicate = "might_be_duplicate_or_false_positive"
)

// Config 는 데모 서비스를 조정한다.
type Config struct {
	ExpectedInsertions       uint64
	FalsePositiveProbability float64
	ScenarioName             string
}

// Service 는 예제가 사용하는 인메모리 Bloom filter를 소유한다.
type Service struct {
	mu           sync.Mutex
	filter       probabilistic.BloomFilter[string]
	scenarioName string
}

// EventRequest 는 하나의 이벤트 입장을 위한 HTTP 및 서비스 요청이다.
type EventRequest struct {
	EventID string `json:"event_id"`
	Source  string `json:"source"`
}

// AdmitResponse 는 안정적으로 공개되는 입장 판단 응답 표현이다.
type AdmitResponse struct {
	EventID  string      `json:"event_id"`
	Source   string      `json:"source,omitempty"`
	Scenario string      `json:"scenario"`
	Decision Decision    `json:"decision"`
	Reason   string      `json:"reason"`
	Accepted bool        `json:"accepted"`
	Stats    FilterStats `json:"stats"`
}

// FilterStats 는 근사적인 Bloom filter 상태를 보고한다.
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

// ErrorResponse 는 안정적으로 공개되는 오류 응답 형태다.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService 는 결정적인 인메모리 Bloom 입장 판단 서비스를 생성한다.
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

// Admit 은 Bloom prefilter 로 이벤트 ID 하나를 평가하고 삽입한다.
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

// Stats 는 현재 Bloom filter 상태의 근사값을 반환한다.
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

// NewRouter 는 예제용 HTTP 라우터를 생성한다.
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
