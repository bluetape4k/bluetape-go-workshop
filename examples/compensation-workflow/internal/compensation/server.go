// Package compensation exposes a request-scoped compensation workflow over Gin.
package compensation

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/workflow"
	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/gin-gonic/gin"
)

var (
	// ErrInventoryUnavailable reports that fulfillment cannot reserve stock.
	ErrInventoryUnavailable = errors.New("inventory is unavailable")
	// ErrPaymentDeclined reports that payment authorization failed.
	ErrPaymentDeclined = errors.New("payment authorization declined")
	// ErrShipmentProviderUnavailable reports that shipment creation cannot start.
	ErrShipmentProviderUnavailable = errors.New("shipment provider is unavailable")
	// ErrPaymentVoidFailed reports that payment compensation failed.
	ErrPaymentVoidFailed = errors.New("payment void failed")
	// ErrInventoryReleaseFailed reports that inventory compensation failed.
	ErrInventoryReleaseFailed = errors.New("inventory release failed")

	errInvalidOrderID = errors.New("order_id is required")
)

// Options configures the compensation workflow API server.
type Options struct{}

// Server exposes a request-scoped compensation workflow over HTTP.
type Server struct {
	router *gin.Engine
}

// RunRequest describes one fulfillment scenario with optional compensation failures.
type RunRequest struct {
	OrderID                   string `json:"order_id" binding:"required"`
	StockAvailable            bool   `json:"stock_available"`
	PaymentAuthorized         bool   `json:"payment_authorized"`
	ShipmentProviderAvailable bool   `json:"shipment_provider_available"`
	VoidPaymentFails          bool   `json:"void_payment_fails,omitempty"`
	ReleaseInventoryFails     bool   `json:"release_inventory_fails,omitempty"`
}

type runResponse struct {
	OrderID       string     `json:"order_id"`
	Completed     bool       `json:"completed"`
	Compensated   bool       `json:"compensated"`
	OriginalError string     `json:"original_error,omitempty"`
	Effects       effectsDTO `json:"effects"`
	Report        reportNode `json:"report"`
}

type effectsDTO struct {
	InventoryReserved bool `json:"inventory_reserved"`
	PaymentAuthorized bool `json:"payment_authorized"`
	ShipmentCreated   bool `json:"shipment_created"`
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

type compensationRun struct {
	request RunRequest

	inventoryReserved bool
	paymentAuthorized bool
	shipmentCreated   bool

	compensations []workflow.Work
	afterReserve  func()
}

// NewServer creates the compensation workflow API.
func NewServer(Options) (*Server, error) {
	router := gin.New()
	router.Use(gin.Recovery())

	server := &Server{router: router}
	router.GET("/healthz", server.health)
	router.POST("/compensation/fulfillment", server.run)

	return server, nil
}

// ServeHTTP dispatches requests to the Gin router.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) run(c *gin.Context) {
	var request RunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	request.OrderID = strings.TrimSpace(request.OrderID)
	if err := validateRequest(request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	run := &compensationRun{request: request}
	report, compensated, originalErr := run.execute(c.Request.Context())
	status := statusForReport(report)

	response := runResponse{
		OrderID:     request.OrderID,
		Completed:   report.IsSuccess(),
		Compensated: compensated,
		Effects: effectsDTO{
			InventoryReserved: run.inventoryReserved,
			PaymentAuthorized: run.paymentAuthorized,
			ShipmentCreated:   run.shipmentCreated,
		},
		Report: projectReport(report),
	}
	if originalErr != nil {
		response.OriginalError = originalErr.Error()
	}
	c.JSON(status, response)
}

func validateRequest(request RunRequest) error {
	if request.OrderID == "" {
		return errInvalidOrderID
	}
	return nil
}

func (r *compensationRun) execute(ctx context.Context) (workreport.Report, bool, error) {
	forward := r.forwardRunner().Run(ctx)
	originalErr := firstError(forward)
	if forward.IsSuccess() || len(r.compensations) == 0 {
		return parentReport("compensating-fulfillment", forward.Status, originalErr, forward.Reason, []workreport.Report{forward}), false, originalErr
	}

	compensationCtx := ctx
	parentStatus := workreport.StatusFailed
	reason := "forward failure triggered compensation"
	if forward.IsCancelled() {
		compensationCtx = context.WithoutCancel(ctx)
		parentStatus = workreport.StatusCancelled
		reason = "caller cancellation triggered compensation"
	}
	compensationReport := workflow.Sequential(
		"compensation",
		workreport.ContinueOnFailure,
		r.reverseCompensations()...,
	).Run(compensationCtx)

	return parentReport(
		"compensating-fulfillment",
		parentStatus,
		originalErr,
		reason,
		[]workreport.Report{forward, compensationReport},
	), true, originalErr
}

func (r *compensationRun) forwardRunner() workflow.Runner {
	return workflow.Sequential(
		"fulfillment-forward",
		workreport.StopOnFailure,
		r.reserveInventory,
		r.authorizePayment,
		r.createShipment,
	)
}

func (r *compensationRun) reserveInventory(ctx context.Context) workreport.Report {
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

func (r *compensationRun) authorizePayment(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("authorize-payment", err)
	}
	if !r.request.PaymentAuthorized {
		return workreport.Failed("authorize-payment", ErrPaymentDeclined)
	}
	r.paymentAuthorized = true
	r.registerCompensation(r.voidPayment)
	return workreport.Completed("authorize-payment")
}

func (r *compensationRun) createShipment(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("create-shipment", err)
	}
	if !r.request.ShipmentProviderAvailable {
		return workreport.Failed("create-shipment", ErrShipmentProviderUnavailable)
	}
	r.shipmentCreated = true
	return workreport.Completed("create-shipment")
}

func (r *compensationRun) voidPayment(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("void-payment", err)
	}
	if r.request.VoidPaymentFails {
		return workreport.Failed("void-payment", ErrPaymentVoidFailed)
	}
	r.paymentAuthorized = false
	return workreport.Completed("void-payment")
}

func (r *compensationRun) releaseInventory(ctx context.Context) workreport.Report {
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("release-inventory", err)
	}
	if r.request.ReleaseInventoryFails {
		return workreport.Failed("release-inventory", ErrInventoryReleaseFailed)
	}
	r.inventoryReserved = false
	return workreport.Completed("release-inventory")
}

func (r *compensationRun) registerCompensation(work workflow.Work) {
	r.compensations = append(r.compensations, work)
}

func (r *compensationRun) reverseCompensations() []workflow.Work {
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

func writeError(c *gin.Context, status int, code string, err error) {
	c.JSON(status, errorResponse{
		Code:  code,
		Error: err.Error(),
	})
}
