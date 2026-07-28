// Package orderfulfillment 는 통합 주문 이행 워크플로를 Gin으로 노출한다.
package orderfulfillment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/state"
	"github.com/bluetape4k/bluetape-go/workflow"
	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/gin-gonic/gin"
)

// OrderState 는 하나의 이행 실행에 대한 현재 수명주기 상태다.
type OrderState string

const (
	// StateDraft 는 제출 전 초기 주문 상태다.
	StateDraft OrderState = "draft"
	// StateSubmitted 는 재고와 결제 작업을 시작할 수 있음을 의미한다.
	StateSubmitted OrderState = "submitted"
	// StatePaid 는 결제 승인이 성공했음을 의미한다.
	StatePaid OrderState = "paid"
	// StatePacked 는 주문이 배송 준비를 마쳤음을 의미한다.
	StatePacked OrderState = "packed"
	// StateShipped 는 성공적인 종료 상태다.
	StateShipped OrderState = "shipped"
	// StateCancelled 는 실패 종료 상태다.
	StateCancelled OrderState = "cancelled"
)

// OrderEvent 는 상태 머신이 받는 수명주기 명령이다.
type OrderEvent string

const (
	// EventSubmit 은 초안 주문을 제출됨 상태로 이동시킨다.
	EventSubmit OrderEvent = "submit"
	// EventPay 는 제출된 주문을 결제됨 상태로 이동시킨다.
	EventPay OrderEvent = "pay"
	// EventPack 은 결제된 주문을 포장됨 상태로 이동시킨다.
	EventPack OrderEvent = "pack"
	// EventShip 은 포장된 주문을 배송됨 상태로 이동시킨다.
	EventShip OrderEvent = "ship"
	// EventCancel 은 취소 가능한 주문을 취소됨 상태로 이동시킨다.
	EventCancel OrderEvent = "cancel"
)

var (
	// ErrInventoryUnavailable 은 재고 예약을 진행할 수 없음을 나타낸다.
	ErrInventoryUnavailable = errors.New("inventory is unavailable")
	// ErrPaymentDeclined 는 결제 승인이 실패했음을 나타낸다.
	ErrPaymentDeclined = errors.New("payment authorization declined")
	// ErrShipmentProviderUnavailable 은 배송 생성 작업을 진행할 수 없음을 나타낸다.
	ErrShipmentProviderUnavailable = errors.New("shipment provider is unavailable")
	// ErrPaymentVoidFailed 는 결제 보상 작업이 실패했음을 나타낸다.
	ErrPaymentVoidFailed = errors.New("payment void failed")
	// ErrInventoryReleaseFailed 는 재고 보상 작업이 실패했음을 나타낸다.
	ErrInventoryReleaseFailed = errors.New("inventory release failed")

	errInvalidOrderID = errors.New("order_id is required")
	errInvalidTotal   = errors.New("total_cents must be positive")
)

// Options 는 주문 이행 통합 API 서버를 설정한다.
type Options struct{}

// Server 는 주문 이행 통합 흐름을 HTTP로 노출한다.
type Server struct {
	router *gin.Engine
}

// FulfillmentRequest 는 하나의 통합 주문 이행 시나리오를 설명한다.
type FulfillmentRequest struct {
	OrderID                   string `json:"order_id" binding:"required"`
	TotalCents                int    `json:"total_cents"`
	StockAvailable            bool   `json:"stock_available"`
	PaymentAuthorized         bool   `json:"payment_authorized"`
	ShipmentProviderAvailable bool   `json:"shipment_provider_available"`
	ForceInvalidTransition    bool   `json:"force_invalid_transition,omitempty"`
	VoidPaymentFails          bool   `json:"void_payment_fails,omitempty"`
	ReleaseInventoryFails     bool   `json:"release_inventory_fails,omitempty"`
}

type fulfillmentResponse struct {
	OrderID       string       `json:"order_id"`
	State         OrderState   `json:"state"`
	StateHistory  []OrderState `json:"state_history"`
	Completed     bool         `json:"completed"`
	Compensated   bool         `json:"compensated"`
	OriginalError string       `json:"original_error,omitempty"`
	Effects       effectsDTO   `json:"effects"`
	Summary       summaryDTO   `json:"summary"`
	Report        reportNode   `json:"report"`
}

type effectsDTO struct {
	InventoryReserved bool `json:"inventory_reserved"`
	PaymentAuthorized bool `json:"payment_authorized"`
	ShipmentCreated   bool `json:"shipment_created"`
}

type summaryDTO struct {
	Completed int  `json:"completed"`
	Failed    int  `json:"failed"`
	Partial   int  `json:"partial"`
	Aborted   int  `json:"aborted"`
	Cancelled int  `json:"cancelled"`
	Total     int  `json:"total"`
	Terminal  bool `json:"terminal"`
	Success   bool `json:"success"`
	Failure   bool `json:"failure"`
}

type reportNode struct {
	Name      string            `json:"name"`
	Status    workreport.Status `json:"status"`
	Error     string            `json:"error,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Success   bool              `json:"success"`
	Failure   bool              `json:"failure"`
	Partial   bool              `json:"partial"`
	Cancelled bool              `json:"cancelled"`
	Children  []reportNode      `json:"children,omitempty"`
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

type orderRun struct {
	request FulfillmentRequest
	machine *state.Machine[OrderState, OrderEvent]
	history []OrderState

	inventoryReserved bool
	paymentAuthorized bool
	shipmentCreated   bool

	compensations []workflow.Work
	afterReserve  func()
}

// NewServer 는 주문 이행 통합 API를 생성한다.
func NewServer(Options) (*Server, error) {
	router := gin.New()
	router.Use(gin.Recovery())

	server := &Server{router: router}
	router.GET("/healthz", server.health)
	router.POST("/orders/fulfillment", server.fulfill)

	return server, nil
}

// ServeHTTP 는 요청을 Gin 라우터로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) fulfill(c *gin.Context) {
	var request FulfillmentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	request.OrderID = strings.TrimSpace(request.OrderID)
	if err := validateRequest(request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	run, err := newOrderRun(request)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "machine_init_failed", err)
		return
	}

	report, compensated, originalErr := run.execute(c.Request.Context())
	response := fulfillmentResponse{
		OrderID:      request.OrderID,
		State:        run.machine.State(),
		StateHistory: append([]OrderState(nil), run.history...),
		Completed:    report.IsSuccess(),
		Compensated:  compensated,
		Effects: effectsDTO{
			InventoryReserved: run.inventoryReserved,
			PaymentAuthorized: run.paymentAuthorized,
			ShipmentCreated:   run.shipmentCreated,
		},
		Summary: summarizeReport(report),
		Report:  projectReport(report),
	}
	if originalErr != nil {
		response.OriginalError = originalErr.Error()
	}

	c.JSON(statusForReport(report), response)
}

func validateRequest(request FulfillmentRequest) error {
	if request.OrderID == "" {
		return errInvalidOrderID
	}
	if request.TotalCents <= 0 {
		return errInvalidTotal
	}
	return nil
}

func newOrderRun(request FulfillmentRequest) (*orderRun, error) {
	machine, err := state.NewMachine(
		StateDraft,
		[]state.Transition[OrderState, OrderEvent]{
			{From: StateDraft, Event: EventSubmit, To: StateSubmitted},
			{From: StateSubmitted, Event: EventPay, To: StatePaid},
			{From: StatePaid, Event: EventPack, To: StatePacked},
			{From: StatePacked, Event: EventShip, To: StateShipped},
			{From: StateDraft, Event: EventCancel, To: StateCancelled},
			{From: StateSubmitted, Event: EventCancel, To: StateCancelled},
			{From: StatePaid, Event: EventCancel, To: StateCancelled},
			{From: StatePacked, Event: EventCancel, To: StateCancelled},
		},
		state.WithFinalStates[OrderState, OrderEvent](StateShipped, StateCancelled),
	)
	if err != nil {
		return nil, fmt.Errorf("create order state machine: %w", err)
	}

	return &orderRun{
		request: request,
		machine: machine,
		history: []OrderState{StateDraft},
	}, nil
}

func (r *orderRun) execute(ctx context.Context) (workreport.Report, bool, error) {
	forward := r.forwardRunner().Run(ctx)
	originalErr := firstError(forward)
	if forward.IsSuccess() || len(r.compensations) == 0 {
		return forward, false, originalErr
	}

	compensationCtx := context.WithoutCancel(ctx)
	parentStatus := workreport.StatusFailed
	reason := "forward failure triggered compensation"
	if forward.IsCancelled() {
		parentStatus = workreport.StatusCancelled
		reason = "caller cancellation triggered compensation"
	}

	compensationReport := workflow.Sequential(
		"compensation",
		workreport.ContinueOnFailure,
		r.reverseCompensations()...,
	).Run(compensationCtx)
	r.cancelOrder(compensationCtx)

	if parentStatus == workreport.StatusFailed && compensationReport.IsPartial() {
		parentStatus = workreport.StatusPartial
	}

	return parentReport(
		"order-fulfillment-run",
		parentStatus,
		originalErr,
		reason,
		[]workreport.Report{forward, compensationReport},
	), true, originalErr
}

func (r *orderRun) forwardRunner() workflow.Runner {
	return workflow.Sequential(
		"order-fulfillment",
		workreport.StopOnFailure,
		r.submitOrder,
		r.reserveInventory,
		r.authorizePayment,
		r.packOrder,
		r.createShipment,
	)
}

func (r *orderRun) submitOrder(ctx context.Context) workreport.Report {
	return r.transition(ctx, "submit-order", EventSubmit)
}

func (r *orderRun) reserveInventory(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("reserve-inventory", err)
	}
	if !r.request.StockAvailable {
		return workreport.Failed("reserve-inventory", ErrInventoryUnavailable)
	}
	r.inventoryReserved = true
	r.registerCompensation(r.releaseInventory)
	if r.afterReserve != nil {
		r.afterReserve()
	}
	return workreport.Completed("reserve-inventory")
}

func (r *orderRun) authorizePayment(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("authorize-payment", err)
	}
	if !r.request.PaymentAuthorized {
		return workreport.Failed("authorize-payment", ErrPaymentDeclined)
	}
	if report := r.transition(ctx, "authorize-payment", EventPay); report.Status != workreport.StatusCompleted {
		return report
	}
	r.paymentAuthorized = true
	r.registerCompensation(r.voidPayment)
	return workreport.Completed("authorize-payment")
}

func (r *orderRun) packOrder(ctx context.Context) workreport.Report {
	if r.request.ForceInvalidTransition {
		return r.transition(ctx, "pack-order", EventShip)
	}
	return r.transition(ctx, "pack-order", EventPack)
}

func (r *orderRun) createShipment(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("create-shipment", err)
	}
	if !r.request.ShipmentProviderAvailable {
		return workreport.Failed("create-shipment", ErrShipmentProviderUnavailable)
	}
	if report := r.transition(ctx, "create-shipment", EventShip); report.Status != workreport.StatusCompleted {
		return report
	}
	r.shipmentCreated = true
	return workreport.Completed("create-shipment")
}

func (r *orderRun) voidPayment(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("void-payment", err)
	}
	if r.request.VoidPaymentFails {
		return workreport.Failed("void-payment", ErrPaymentVoidFailed)
	}
	r.paymentAuthorized = false
	return workreport.Completed("void-payment")
}

func (r *orderRun) releaseInventory(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("release-inventory", err)
	}
	if r.request.ReleaseInventoryFails {
		return workreport.Failed("release-inventory", ErrInventoryReleaseFailed)
	}
	r.inventoryReserved = false
	return workreport.Completed("release-inventory")
}

func (r *orderRun) cancelOrder(ctx context.Context) {
	if r.machine.State() == StateCancelled || r.machine.State() == StateShipped {
		return
	}
	if result, err := r.machine.Transition(ctx, EventCancel); err == nil {
		r.history = append(r.history, result.Current)
	}
}

func (r *orderRun) transition(ctx context.Context, name string, event OrderEvent) workreport.Report {
	result, err := r.machine.Transition(ctx, event)
	if err != nil {
		return workreport.Failed(name, err)
	}
	r.history = append(r.history, result.Current)
	return workreport.Completed(name)
}

func (r *orderRun) registerCompensation(work workflow.Work) {
	r.compensations = append(r.compensations, work)
}

func (r *orderRun) reverseCompensations() []workflow.Work {
	works := make([]workflow.Work, 0, len(r.compensations))
	for i := len(r.compensations) - 1; i >= 0; i-- {
		works = append(works, r.compensations[i])
	}
	return works
}

func statusForReport(report workreport.Report) int {
	switch {
	case report.IsCancelled():
		return http.StatusRequestTimeout
	case report.IsSuccess():
		return http.StatusOK
	case report.IsFailure() || report.IsPartial():
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func parentReport(name string, status workreport.Status, err error, reason string, children []workreport.Report) workreport.Report {
	now := time.Now()
	return workreport.Report{
		Name:      name,
		Status:    status,
		Err:       err,
		Reason:    reason,
		StartedAt: now,
		EndedAt:   now,
		Children:  append([]workreport.Report(nil), children...),
	}
}

func firstError(report workreport.Report) error {
	if report.Err != nil {
		return report.Err
	}
	for _, child := range report.Children {
		if err := firstError(child); err != nil {
			return err
		}
	}
	return nil
}

func projectReport(report workreport.Report) reportNode {
	node := reportNode{
		Name:      report.Name,
		Status:    report.Status,
		Reason:    report.Reason,
		Success:   report.IsSuccess(),
		Failure:   report.IsFailure(),
		Partial:   report.IsPartial(),
		Cancelled: report.IsCancelled(),
	}
	if report.Err != nil {
		node.Error = report.Err.Error()
	}
	if len(report.Children) > 0 {
		node.Children = make([]reportNode, 0, len(report.Children))
		for _, child := range report.Children {
			node.Children = append(node.Children, projectReport(child))
		}
	}
	return node
}

func summarizeReport(report workreport.Report) summaryDTO {
	summary := summaryDTO{
		Terminal: report.IsTerminal(),
		Success:  report.IsSuccess(),
		Failure:  report.IsFailure(),
	}
	walkReports(report, func(current workreport.Report) {
		summary.Total++
		switch current.Status {
		case workreport.StatusCompleted:
			summary.Completed++
		case workreport.StatusFailed:
			summary.Failed++
		case workreport.StatusPartial:
			summary.Partial++
		case workreport.StatusAborted:
			summary.Aborted++
		case workreport.StatusCancelled:
			summary.Cancelled++
		}
	})
	return summary
}

func walkReports(report workreport.Report, visit func(workreport.Report)) {
	visit(report)
	for _, child := range report.Children {
		walkReports(child, visit)
	}
}

func writeError(c *gin.Context, status int, code string, err error) {
	c.JSON(status, errorResponse{Code: code, Error: err.Error()})
}
