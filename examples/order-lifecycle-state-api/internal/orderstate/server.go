// Package orderstate 는 주문 수명주기 유한 상태 머신을 Gin으로 노출한다.
package orderstate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bluetape4k/bluetape-go/state"
	"github.com/gin-gonic/gin"
)

// OrderState 는 예제 주문의 현재 수명주기 상태다.
type OrderState string

const (
	// StateDraft 는 처음의 수정 가능한 주문 상태다.
	StateDraft OrderState = "draft"
	// StateSubmitted 는 주문이 결제 가능한 상태임을 의미한다.
	StateSubmitted OrderState = "submitted"
	// StatePaid 는 결제가 승인됐음을 의미한다.
	StatePaid OrderState = "paid"
	// StatePacked 는 이행 단계에서 주문 포장을 마쳤음을 의미한다.
	StatePacked OrderState = "packed"
	// StateShipped 는 성공으로 끝난 최종 주문 상태다.
	StateShipped OrderState = "shipped"
	// StateCancelled 는 취소로 끝난 최종 주문 상태다.
	StateCancelled OrderState = "cancelled"
)

// OrderEvent 는 주문 수명주기 상태 머신이 받는 명령이다.
type OrderEvent string

const (
	// EventSubmit 은 초안 주문을 제출됨 상태로 이동시킨다.
	EventSubmit OrderEvent = "submit"
	// EventPay 는 결제 가드가 허용한 제출 주문을 결제됨 상태로 이동시킨다.
	EventPay OrderEvent = "pay"
	// EventPack 은 결제된 주문을 포장됨 상태로 이동시킨다.
	EventPack OrderEvent = "pack"
	// EventShip 은 포장된 주문을 배송됨 상태로 이동시킨다.
	EventShip OrderEvent = "ship"
	// EventCancel 은 취소 가능한 주문을 취소됨 상태로 이동시킨다.
	EventCancel OrderEvent = "cancel"
)

var (
	// ErrNonPositiveTotal 은 양수 합계가 없는 주문의 결제를 거부한다.
	ErrNonPositiveTotal = errors.New("order total must be positive before payment")

	errUnknownEvent = errors.New("unknown order lifecycle event")
)

// Options 는 주문 수명주기 API 서버를 설정한다.
type Options struct {
	OrderID    string
	TotalCents int
}

// Server 는 하나의 인메모리 주문 수명주기를 HTTP로 노출한다.
type Server struct {
	router *gin.Engine

	orderID    string
	totalCents int
	machine    *state.Machine[OrderState, OrderEvent]
}

// OrderSnapshot 은 현재 주문을 읽기 위한 안정적인 응답 형태다.
type OrderSnapshot struct {
	OrderID       string       `json:"order_id"`
	State         OrderState   `json:"state"`
	AllowedEvents []OrderEvent `json:"allowed_events"`
	TotalCents    int          `json:"total_cents"`
}

type transitionRequest struct {
	Event string `json:"event" binding:"required"`
}

type transitionResponse struct {
	Order    OrderSnapshot `json:"order"`
	Previous OrderState    `json:"previous"`
	Event    OrderEvent    `json:"event"`
	Current  OrderState    `json:"current"`
}

type canTransitionResponse struct {
	Event   OrderEvent    `json:"event"`
	Allowed bool          `json:"allowed"`
	State   OrderState    `json:"state"`
	Error   string        `json:"error,omitempty"`
	Order   OrderSnapshot `json:"order"`
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

// NewServer 는 인메모리 주문 수명주기 API를 생성한다.
func NewServer(options Options) (*Server, error) {
	orderID := options.OrderID
	if orderID == "" {
		orderID = "order-1001"
	}

	machine, err := newMachine(options.TotalCents)
	if err != nil {
		return nil, fmt.Errorf("create order state machine: %w", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	server := &Server{
		router:     router,
		orderID:    orderID,
		totalCents: options.TotalCents,
		machine:    machine,
	}

	router.GET("/healthz", server.health)
	router.GET("/orders/current", server.current)
	router.POST("/orders/current/transitions", server.transition)
	router.GET("/orders/current/transitions/:event/can", server.canTransition)

	return server, nil
}

// ServeHTTP 는 요청을 Gin 라우터로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func newMachine(totalCents int) (*state.Machine[OrderState, OrderEvent], error) {
	return state.NewMachine(
		StateDraft,
		[]state.Transition[OrderState, OrderEvent]{
			{From: StateDraft, Event: EventSubmit, To: StateSubmitted},
			{From: StateSubmitted, Event: EventPay, To: StatePaid, Guard: positiveTotalGuard(totalCents)},
			{From: StatePaid, Event: EventPack, To: StatePacked},
			{From: StatePacked, Event: EventShip, To: StateShipped},
			{From: StateDraft, Event: EventCancel, To: StateCancelled},
			{From: StateSubmitted, Event: EventCancel, To: StateCancelled},
			{From: StatePaid, Event: EventCancel, To: StateCancelled},
		},
		state.WithFinalStates[OrderState, OrderEvent](StateShipped, StateCancelled),
	)
}

func positiveTotalGuard(totalCents int) state.Guard[OrderState, OrderEvent] {
	return func(context.Context, OrderState, OrderEvent) error {
		if totalCents <= 0 {
			return ErrNonPositiveTotal
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

	event, err := parseEvent(request.Event)
	if err != nil {
		writeError(c, http.StatusBadRequest, "unknown_event", err)
		return
	}

	result, err := s.machine.Transition(c.Request.Context(), event)
	if err != nil {
		writeTransitionError(c, err)
		return
	}

	c.JSON(http.StatusOK, transitionResponse{
		Order:    s.snapshot(),
		Previous: result.Previous,
		Event:    result.Event,
		Current:  result.Current,
	})
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
		Order:   snapshot,
	}
	if !allowed {
		response.Error = "event is not available from the current state"
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) snapshot() OrderSnapshot {
	return OrderSnapshot{
		OrderID:       s.orderID,
		State:         s.machine.State(),
		AllowedEvents: s.machine.AllowedEvents(),
		TotalCents:    s.totalCents,
	}
}

func parseEvent(raw string) (OrderEvent, error) {
	switch OrderEvent(strings.ToLower(strings.TrimSpace(raw))) {
	case EventSubmit:
		return EventSubmit, nil
	case EventPay:
		return EventPay, nil
	case EventPack:
		return EventPack, nil
	case EventShip:
		return EventShip, nil
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
	c.JSON(status, errorResponse{
		Code:  code,
		Error: err.Error(),
	})
}
