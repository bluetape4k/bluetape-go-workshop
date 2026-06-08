// Package fulfillment exposes a request-scoped fulfillment workflow runner over Gin.
package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/workflow"
	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/gin-gonic/gin"
)

const (
	defaultMaxInventoryDelay = 2 * time.Second
)

var (
	// ErrInventoryUnavailable reports that fulfillment cannot reserve stock.
	ErrInventoryUnavailable = errors.New("inventory is unavailable")
	// ErrPaymentDeclined reports that payment authorization failed.
	ErrPaymentDeclined = errors.New("payment authorization declined")

	errInvalidOrderID        = errors.New("order_id is required")
	errInvalidInventoryDelay = errors.New("inventory_delay_ms must be between 0 and the configured maximum")
)

// Options configures the fulfillment workflow API server.
type Options struct {
	MaxInventoryDelay time.Duration
}

// Server exposes a request-scoped fulfillment workflow over HTTP.
type Server struct {
	router            *gin.Engine
	maxInventoryDelay time.Duration
}

// RunRequest describes one fulfillment workflow scenario.
type RunRequest struct {
	OrderID           string `json:"order_id" binding:"required"`
	StockAvailable    bool   `json:"stock_available"`
	PaymentAuthorized bool   `json:"payment_authorized"`
	RequiresShipment  bool   `json:"requires_shipment"`
	InventoryDelayMS  int    `json:"inventory_delay_ms,omitempty"`
}

type runResponse struct {
	OrderID         string     `json:"order_id"`
	ShipmentCreated bool       `json:"shipment_created"`
	ShipmentSkipped bool       `json:"shipment_skipped"`
	Report          reportNode `json:"report"`
}

type reportNode struct {
	Name      string            `json:"name"`
	Status    workreport.Status `json:"status"`
	Error     string            `json:"error,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Success   bool              `json:"success"`
	Failure   bool              `json:"failure"`
	Cancelled bool              `json:"cancelled"`
	Children  []reportNode      `json:"children,omitempty"`
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

type fulfillmentRun struct {
	request          RunRequest
	inventoryStarted chan struct{}
	shipmentCreated  bool
	shipmentSkipped  bool
}

// NewServer creates the fulfillment workflow API.
func NewServer(options Options) (*Server, error) {
	maxInventoryDelay := options.MaxInventoryDelay
	if maxInventoryDelay <= 0 {
		maxInventoryDelay = defaultMaxInventoryDelay
	}

	router := gin.New()
	router.Use(gin.Recovery())

	server := &Server{
		router:            router,
		maxInventoryDelay: maxInventoryDelay,
	}
	router.GET("/healthz", server.health)
	router.POST("/fulfillment/run", server.run)

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
	if err := s.validateRequest(request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	run := &fulfillmentRun{request: request}
	report := run.runner().Run(c.Request.Context())
	status := statusForReport(report)

	c.JSON(status, runResponse{
		OrderID:         request.OrderID,
		ShipmentCreated: run.shipmentCreated,
		ShipmentSkipped: run.shipmentSkipped,
		Report:          projectReport(report),
	})
}

func (s *Server) validateRequest(request RunRequest) error {
	if request.OrderID == "" {
		return errInvalidOrderID
	}
	delay := time.Duration(request.InventoryDelayMS) * time.Millisecond
	if request.InventoryDelayMS < 0 || delay > s.maxInventoryDelay {
		return fmt.Errorf("%w: got %dms", errInvalidInventoryDelay, request.InventoryDelayMS)
	}
	return nil
}

func (r *fulfillmentRun) runner() workflow.Runner {
	return workflow.Sequential(
		"fulfillment",
		workreport.StopOnFailure,
		r.validateOrder,
		r.riskChecks,
		r.shipmentDecision,
	)
}

func (r *fulfillmentRun) validateOrder(context.Context) workreport.Report {
	return workreport.Completed("validate-order")
}

func (r *fulfillmentRun) riskChecks(ctx context.Context) workreport.Report {
	if r.request.InventoryDelayMS > 0 {
		r.inventoryStarted = make(chan struct{})
	}
	return workflow.Parallel(
		"risk-checks",
		workreport.StopOnFailure,
		r.reserveInventory,
		r.authorizePayment,
	).Run(ctx)
}

func (r *fulfillmentRun) reserveInventory(ctx context.Context) workreport.Report {
	r.signalInventoryStarted()
	delay := time.Duration(r.request.InventoryDelayMS) * time.Millisecond
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return workreport.Cancelled("reserve-inventory", ctx.Err())
		case <-timer.C:
		}
	}
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("reserve-inventory", err)
	}
	if !r.request.StockAvailable {
		return workreport.Failed("reserve-inventory", ErrInventoryUnavailable)
	}
	return workreport.Completed("reserve-inventory")
}

func (r *fulfillmentRun) authorizePayment(ctx context.Context) workreport.Report {
	if r.inventoryStarted != nil {
		select {
		case <-ctx.Done():
			return workreport.Cancelled("authorize-payment", ctx.Err())
		case <-r.inventoryStarted:
		}
	}
	if err := ctx.Err(); err != nil {
		return workreport.Cancelled("authorize-payment", err)
	}
	if !r.request.PaymentAuthorized {
		return workreport.Failed("authorize-payment", ErrPaymentDeclined)
	}
	return workreport.Completed("authorize-payment")
}

func (r *fulfillmentRun) signalInventoryStarted() {
	if r.inventoryStarted != nil {
		close(r.inventoryStarted)
	}
}

func (r *fulfillmentRun) shipmentDecision(ctx context.Context) workreport.Report {
	return workflow.Conditional(
		"shipment-decision",
		func(context.Context) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			return r.request.RequiresShipment, nil
		},
		r.createShipment,
		r.skipShipment,
	).Run(ctx)
}

func (r *fulfillmentRun) createShipment(context.Context) workreport.Report {
	r.shipmentCreated = true
	return workreport.Completed("create-shipment")
}

func (r *fulfillmentRun) skipShipment(context.Context) workreport.Report {
	r.shipmentSkipped = true
	return workreport.Completed("shipment-skipped")
}

func statusForReport(report workreport.Report) int {
	switch {
	case report.IsCancelled():
		return http.StatusRequestTimeout
	case report.IsSuccess():
		return http.StatusOK
	case report.IsFailure():
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func projectReport(report workreport.Report) reportNode {
	node := reportNode{
		Name:      report.Name,
		Status:    report.Status,
		Reason:    report.Reason,
		Success:   report.IsSuccess(),
		Failure:   report.IsFailure(),
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
