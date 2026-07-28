// Package paymentauth 는 결제 승인 상태 머신을 Gin으로 노출한다.
package paymentauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/bluetape4k/bluetape-go/state"
	"github.com/gin-gonic/gin"
)

// PaymentState 는 예제 결제의 현재 수명주기 상태다.
type PaymentState string

const (
	// StateRequested 는 최초 승인 요청 상태다.
	StateRequested PaymentState = "requested"
	// StateAuthorized 는 결제 승인 hold 가 승인됐음을 의미한다.
	StateAuthorized PaymentState = "authorized"
	// StateCaptured 는 성공으로 끝난 최종 결제 상태다.
	StateCaptured PaymentState = "captured"
	// StateFailed 는 실패로 끝난 최종 결제 상태다.
	StateFailed PaymentState = "failed"
	// StateCancelled 는 취소로 끝난 최종 결제 상태다.
	StateCancelled PaymentState = "cancelled"
)

// PaymentEvent 는 결제 상태 머신이 받는 명령이다.
type PaymentEvent string

const (
	// EventAuthorize 는 요청된 결제를 승인됨 상태로 이동시킨다.
	EventAuthorize PaymentEvent = "authorize"
	// EventCapture 는 승인된 결제를 캡처됨 상태로 이동시킨다.
	EventCapture PaymentEvent = "capture"
	// EventFail 은 요청됨 또는 승인됨 결제를 실패 상태로 이동시킨다.
	EventFail PaymentEvent = "fail"
	// EventCancel 은 요청됨 또는 승인됨 결제를 취소 상태로 이동시킨다.
	EventCancel PaymentEvent = "cancel"
)

var (
	// ErrNonPositiveAmount 는 금액이 양수가 아닐 때 승인을 거부한다.
	ErrNonPositiveAmount = errors.New("payment amount must be positive before authorization")

	errUnknownEvent         = errors.New("unknown payment event")
	errMissingEvent         = errors.New("event is required")
	errMissingIdempotency   = errors.New("idempotency_key is required")
	errIdempotencyKeyReuse  = errors.New("idempotency key was already used for another event")
	errInvalidPaymentAmount = errors.New("payment amount must not be negative")
)

// Options 는 결제 승인 API 서버를 설정한다.
type Options struct {
	PaymentID   string
	AmountCents int
}

// Server 는 하나의 인메모리 결제 승인 흐름을 HTTP로 노출한다.
type Server struct {
	router *gin.Engine

	mu          sync.Mutex
	paymentID   string
	amountCents int
	machine     *state.Machine[PaymentState, PaymentEvent]
	idempotency *idempotencyStore
}

// PaymentSnapshot 은 현재 결제를 읽기 위한 안정적인 응답 형태다.
type PaymentSnapshot struct {
	PaymentID     string         `json:"payment_id"`
	State         PaymentState   `json:"state"`
	AllowedEvents []PaymentEvent `json:"allowed_events"`
	AmountCents   int            `json:"amount_cents"`
}

type transitionRequest struct {
	Event          string `json:"event" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

type transitionResponse struct {
	Payment          PaymentSnapshot `json:"payment"`
	Previous         PaymentState    `json:"previous"`
	Event            PaymentEvent    `json:"event"`
	Current          PaymentState    `json:"current"`
	IdempotentReplay bool            `json:"idempotent_replay"`
}

type canTransitionResponse struct {
	Event   PaymentEvent    `json:"event"`
	Allowed bool            `json:"allowed"`
	State   PaymentState    `json:"state"`
	Error   string          `json:"error,omitempty"`
	Payment PaymentSnapshot `json:"payment"`
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

type idempotencyStore struct {
	entries map[string]idempotencyEntry
}

type idempotencyEntry struct {
	event    PaymentEvent
	response transitionResponse
}

// NewServer 는 인메모리 결제 승인 API를 생성한다.
func NewServer(options Options) (*Server, error) {
	paymentID := strings.TrimSpace(options.PaymentID)
	if paymentID == "" {
		paymentID = "pay-1001"
	}
	if options.AmountCents < 0 {
		return nil, errInvalidPaymentAmount
	}

	machine, err := newMachine(options.AmountCents)
	if err != nil {
		return nil, fmt.Errorf("create payment state machine: %w", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	server := &Server{
		router:      router,
		paymentID:   paymentID,
		amountCents: options.AmountCents,
		machine:     machine,
		idempotency: &idempotencyStore{entries: make(map[string]idempotencyEntry)},
	}

	router.GET("/healthz", server.health)
	router.GET("/payments/current", server.current)
	router.POST("/payments/current/transitions", server.transition)
	router.GET("/payments/current/transitions/:event/can", server.canTransition)

	return server, nil
}

// ServeHTTP 는 요청을 Gin 라우터로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func newMachine(amountCents int) (*state.Machine[PaymentState, PaymentEvent], error) {
	return state.NewMachine(
		StateRequested,
		[]state.Transition[PaymentState, PaymentEvent]{
			{From: StateRequested, Event: EventAuthorize, To: StateAuthorized, Guard: positiveAmountGuard(amountCents)},
			{From: StateAuthorized, Event: EventCapture, To: StateCaptured},
			{From: StateRequested, Event: EventFail, To: StateFailed},
			{From: StateAuthorized, Event: EventFail, To: StateFailed},
			{From: StateRequested, Event: EventCancel, To: StateCancelled},
			{From: StateAuthorized, Event: EventCancel, To: StateCancelled},
		},
		state.WithFinalStates[PaymentState, PaymentEvent](StateCaptured, StateFailed, StateCancelled),
	)
}

func positiveAmountGuard(amountCents int) state.Guard[PaymentState, PaymentEvent] {
	return func(context.Context, PaymentState, PaymentEvent) error {
		if amountCents <= 0 {
			return ErrNonPositiveAmount
		}
		return nil
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) current(c *gin.Context) {
	c.JSON(http.StatusOK, s.snapshot())
}

func (s *Server) transition(c *gin.Context) {
	var request transitionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	event, key, err := parseTransitionRequest(request)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	response, err := s.transitionWithIdempotency(c.Request.Context(), event, key)
	if err != nil {
		writeTransitionError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) transitionWithIdempotency(ctx context.Context, event PaymentEvent, key string) (transitionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if replay, ok, err := s.idempotency.replay(key, event); ok || err != nil {
		return replay, err
	}

	result, err := s.machine.Transition(ctx, event)
	if err != nil {
		return transitionResponse{}, err
	}

	response := transitionResponse{
		Payment:  s.snapshotLocked(),
		Previous: result.Previous,
		Event:    result.Event,
		Current:  result.Current,
	}
	s.idempotency.store(key, event, response)
	return response, nil
}

func (s *Server) canTransition(c *gin.Context) {
	event, err := parseEvent(c.Param("event"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "unknown_event", err)
		return
	}

	allowed, err := s.machine.CanTransition(c.Request.Context(), event)
	if err != nil {
		writeTransitionError(c, err)
		return
	}

	snapshot := s.snapshot()
	response := canTransitionResponse{
		Event:   event,
		Allowed: allowed,
		State:   snapshot.State,
		Payment: snapshot,
	}
	if !allowed {
		response.Error = "event is not available from the current state"
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) snapshot() PaymentSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Server) snapshotLocked() PaymentSnapshot {
	return PaymentSnapshot{
		PaymentID:     s.paymentID,
		State:         s.machine.State(),
		AllowedEvents: s.machine.AllowedEvents(),
		AmountCents:   s.amountCents,
	}
}

func (s *idempotencyStore) replay(key string, event PaymentEvent) (transitionResponse, bool, error) {
	entry, ok := s.entries[key]
	if !ok {
		return transitionResponse{}, false, nil
	}
	if entry.event != event {
		return transitionResponse{}, false, fmt.Errorf("%w: key=%q previous_event=%q event=%q", errIdempotencyKeyReuse, key, entry.event, event)
	}
	response := entry.response
	response.IdempotentReplay = true
	return response, true, nil
}

func (s *idempotencyStore) store(key string, event PaymentEvent, response transitionResponse) {
	s.entries[key] = idempotencyEntry{event: event, response: response}
}

func parseTransitionRequest(request transitionRequest) (PaymentEvent, string, error) {
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" {
		return "", "", errMissingIdempotency
	}
	event, err := parseEvent(request.Event)
	if err != nil {
		if strings.TrimSpace(request.Event) == "" {
			return "", "", errMissingEvent
		}
		return "", "", err
	}
	return event, key, nil
}

func parseEvent(raw string) (PaymentEvent, error) {
	switch PaymentEvent(strings.ToLower(strings.TrimSpace(raw))) {
	case EventAuthorize:
		return EventAuthorize, nil
	case EventCapture:
		return EventCapture, nil
	case EventFail:
		return EventFail, nil
	case EventCancel:
		return EventCancel, nil
	default:
		return "", fmt.Errorf("%w: %q", errUnknownEvent, raw)
	}
}

func writeTransitionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusRequestTimeout, "request_timeout", err)
	case errors.Is(err, errIdempotencyKeyReuse):
		writeError(c, http.StatusConflict, "idempotency_conflict", err)
	case errors.Is(err, state.ErrInvalidTransition):
		writeError(c, http.StatusConflict, "invalid_transition", err)
	case errors.Is(err, state.ErrGuardRejected):
		writeError(c, http.StatusConflict, "guard_rejected", err)
	case errors.Is(err, state.ErrFinalState):
		writeError(c, http.StatusConflict, "final_state", err)
	case errors.Is(err, state.ErrConcurrentTransition):
		writeError(c, http.StatusConflict, "concurrent_transition", err)
	default:
		writeError(c, http.StatusInternalServerError, "transition_failed", err)
	}
}

func writeError(c *gin.Context, status int, code string, err error) {
	c.JSON(status, errorResponse{Code: code, Error: err.Error()})
}
