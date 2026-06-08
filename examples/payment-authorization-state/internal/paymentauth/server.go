// Package paymentauth exposes a payment authorization state machine over Gin.
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

// PaymentState is the current lifecycle state for the example payment.
type PaymentState string

const (
	// StateRequested is the initial authorization request state.
	StateRequested PaymentState = "requested"
	// StateAuthorized means the payment authorization hold was approved.
	StateAuthorized PaymentState = "authorized"
	// StateCaptured is a final successful payment state.
	StateCaptured PaymentState = "captured"
	// StateFailed is a final failed payment state.
	StateFailed PaymentState = "failed"
	// StateCancelled is a final cancelled payment state.
	StateCancelled PaymentState = "cancelled"
)

// PaymentEvent is the command accepted by the payment state machine.
type PaymentEvent string

const (
	// EventAuthorize moves a requested payment to authorized.
	EventAuthorize PaymentEvent = "authorize"
	// EventCapture moves an authorized payment to captured.
	EventCapture PaymentEvent = "capture"
	// EventFail moves a requested or authorized payment to failed.
	EventFail PaymentEvent = "fail"
	// EventCancel moves a requested or authorized payment to cancelled.
	EventCancel PaymentEvent = "cancel"
)

var (
	// ErrNonPositiveAmount rejects authorization when the amount is not positive.
	ErrNonPositiveAmount = errors.New("payment amount must be positive before authorization")

	errUnknownEvent         = errors.New("unknown payment event")
	errMissingEvent         = errors.New("event is required")
	errMissingIdempotency   = errors.New("idempotency_key is required")
	errIdempotencyKeyReuse  = errors.New("idempotency key was already used for another event")
	errInvalidPaymentAmount = errors.New("payment amount must not be negative")
)

// Options configures the payment authorization API server.
type Options struct {
	PaymentID   string
	AmountCents int
}

// Server exposes one in-memory payment authorization over HTTP.
type Server struct {
	router *gin.Engine

	mu          sync.Mutex
	paymentID   string
	amountCents int
	machine     *state.Machine[PaymentState, PaymentEvent]
	idempotency *idempotencyStore
}

// PaymentSnapshot is the stable response shape for reading the current payment.
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

// NewServer creates the in-memory payment authorization API.
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

// ServeHTTP dispatches requests to the Gin router.
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
