package orderfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/state"
	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestServerHealth(t *testing.T) {
	server := newTestServer(t)

	response := performRequest(t, server, http.MethodGet, "/healthz", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestServerFulfillmentSuccess(t *testing.T) {
	server := newTestServer(t)

	response := postFulfillment(t, server, FulfillmentRequest{
		OrderID:                   " order-1001 ",
		TotalCents:                2599,
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("fulfillment status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := decodeResponse[fulfillmentResponse](t, response)
	if body.OrderID != "order-1001" || body.State != StateShipped || !body.Completed || body.Compensated || body.OriginalError != "" {
		t.Fatalf("response = %#v, want shipped completion without compensation", body)
	}
	assertStateHistory(t, body.StateHistory, StateDraft, StateSubmitted, StatePaid, StatePacked, StateShipped)
	if !body.Effects.InventoryReserved || !body.Effects.PaymentAuthorized || !body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want all successful side effects retained", body.Effects)
	}
	if !body.Summary.Success || body.Summary.Failure || body.Summary.Total != 6 || body.Summary.Completed != 6 {
		t.Fatalf("summary = %#v, want successful root plus five steps", body.Summary)
	}
	assertReport(t, body.Report, "order-fulfillment", workreport.StatusCompleted)
	assertChildren(t, body.Report, []childExpectation{
		{name: "submit-order", status: workreport.StatusCompleted},
		{name: "reserve-inventory", status: workreport.StatusCompleted},
		{name: "authorize-payment", status: workreport.StatusCompleted},
		{name: "pack-order", status: workreport.StatusCompleted},
		{name: "create-shipment", status: workreport.StatusCompleted},
	})
}

func TestServerFulfillmentInvalidTransitionCompensates(t *testing.T) {
	server := newTestServer(t)

	response := postFulfillment(t, server, FulfillmentRequest{
		OrderID:                   "order-1002",
		TotalCents:                2599,
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
		ForceInvalidTransition:    true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("fulfillment status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[fulfillmentResponse](t, response)
	if body.Completed || !body.Compensated || body.State != StateCancelled {
		t.Fatalf("response = %#v, want compensated cancelled invalid transition", body)
	}
	if !strings.Contains(body.OriginalError, state.ErrInvalidTransition.Error()) {
		t.Fatalf("original_error = %q, want invalid transition", body.OriginalError)
	}
	assertStateHistory(t, body.StateHistory, StateDraft, StateSubmitted, StatePaid, StateCancelled)
	if body.Effects.InventoryReserved || body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want inventory and payment cleaned up", body.Effects)
	}
	forward := assertChild(t, body.Report, "order-fulfillment", workreport.StatusFailed)
	assertChild(t, forward, "pack-order", workreport.StatusFailed)
	compensation := assertChild(t, body.Report, "compensation", workreport.StatusCompleted)
	assertChildren(t, compensation, []childExpectation{
		{name: "void-payment", status: workreport.StatusCompleted},
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
}

func TestServerFulfillmentShipmentFailureCompensatesAndCancels(t *testing.T) {
	server := newTestServer(t)

	response := postFulfillment(t, server, FulfillmentRequest{
		OrderID:                   "order-1003",
		TotalCents:                2599,
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: false,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("fulfillment status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[fulfillmentResponse](t, response)
	if body.Completed || !body.Compensated || body.State != StateCancelled {
		t.Fatalf("response = %#v, want compensated shipment failure", body)
	}
	if !strings.Contains(body.OriginalError, ErrShipmentProviderUnavailable.Error()) {
		t.Fatalf("original_error = %q, want shipment failure", body.OriginalError)
	}
	assertStateHistory(t, body.StateHistory, StateDraft, StateSubmitted, StatePaid, StatePacked, StateCancelled)
	if body.Effects.InventoryReserved || body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want inventory and payment undone", body.Effects)
	}
	compensation := assertChild(t, body.Report, "compensation", workreport.StatusCompleted)
	assertChildren(t, compensation, []childExpectation{
		{name: "void-payment", status: workreport.StatusCompleted},
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
}

func TestServerFulfillmentCompensationFailurePreservesOriginalError(t *testing.T) {
	server := newTestServer(t)

	response := postFulfillment(t, server, FulfillmentRequest{
		OrderID:                   "order-1004",
		TotalCents:                2599,
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: false,
		VoidPaymentFails:          true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("fulfillment status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[fulfillmentResponse](t, response)
	if body.Completed || !body.Compensated || body.State != StateCancelled {
		t.Fatalf("response = %#v, want partial compensation with cancelled lifecycle", body)
	}
	if !strings.Contains(body.OriginalError, ErrShipmentProviderUnavailable.Error()) {
		t.Fatalf("original_error = %q, want original shipment failure", body.OriginalError)
	}
	if body.Effects.InventoryReserved || !body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want payment still authorized and inventory released", body.Effects)
	}
	compensation := assertChild(t, body.Report, "compensation", workreport.StatusPartial)
	assertChildren(t, compensation, []childExpectation{
		{name: "void-payment", status: workreport.StatusFailed},
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
	voidPayment := assertChild(t, compensation, "void-payment", workreport.StatusFailed)
	if !strings.Contains(voidPayment.Error, ErrPaymentVoidFailed.Error()) {
		t.Fatalf("void-payment error = %q, want %q", voidPayment.Error, ErrPaymentVoidFailed.Error())
	}
}

func TestServerFulfillmentCallerCancellationBeforeSideEffect(t *testing.T) {
	server := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response := postFulfillmentWithContext(ctx, t, server, FulfillmentRequest{
		OrderID:                   "order-cancelled",
		TotalCents:                2599,
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
	})
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("cancelled status = %d, want %d: %s", response.Code, http.StatusRequestTimeout, response.Body.String())
	}

	body := decodeResponse[fulfillmentResponse](t, response)
	if body.Compensated || body.State != StateDraft {
		t.Fatalf("response = %#v, want cancellation before side effects", body)
	}
	if !body.Report.Cancelled {
		t.Fatalf("report = %#v, want cancelled report", body.Report)
	}
}

func TestOrderRunCancellationAfterSideEffectStillCompensates(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	run := newTestRun(t, FulfillmentRequest{
		OrderID:                   "order-cancel-after-reserve",
		TotalCents:                2599,
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
	})
	run.afterReserve = cancel

	report, compensated, originalErr := run.execute(ctx)
	if originalErr == nil || !strings.Contains(originalErr.Error(), context.Canceled.Error()) {
		t.Fatalf("originalErr = %v, want context canceled", originalErr)
	}
	if !compensated {
		t.Fatalf("compensated = false, want true after inventory side effect")
	}
	if run.machine.State() != StateCancelled {
		t.Fatalf("state = %q, want %q", run.machine.State(), StateCancelled)
	}
	assertStateHistory(t, run.history, StateDraft, StateSubmitted, StateCancelled)
	if run.inventoryReserved || run.paymentAuthorized || run.shipmentCreated {
		t.Fatalf("effects = inventory:%v payment:%v shipment:%v, want cleanup", run.inventoryReserved, run.paymentAuthorized, run.shipmentCreated)
	}
	projected := projectReport(report)
	assertReport(t, projected, "order-fulfillment-run", workreport.StatusCancelled)
	compensation := assertChild(t, projected, "compensation", workreport.StatusCompleted)
	assertChildren(t, compensation, []childExpectation{
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
}

func TestServerFulfillmentBadRequests(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		name string
		body []byte
	}{
		{name: "malformed", body: []byte("{")},
		{name: "missing order id", body: []byte(`{"total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":true}`)},
		{name: "blank order id", body: []byte(`{"order_id":"   ","total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":true}`)},
		{name: "non-positive total", body: []byte(`{"order_id":"order-1005","total_cents":0,"stock_available":true,"payment_authorized":true,"shipment_provider_available":true}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(t, server, http.MethodPost, "/orders/fulfillment", tt.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			body := decodeResponse[errorResponse](t, response)
			if body.Code != "invalid_request" {
				t.Fatalf("code = %q, want invalid_request", body.Code)
			}
		})
	}
}

func TestServerFulfillmentParallelRequestsHaveIndependentState(t *testing.T) {
	server := newTestServer(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		response := postFulfillment(t, server, FulfillmentRequest{
			OrderID:                   "parallel-success",
			TotalCents:                2599,
			StockAvailable:            true,
			PaymentAuthorized:         true,
			ShipmentProviderAvailable: true,
		})
		if response.Code != http.StatusOK {
			t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
		}
		body := decodeResponse[fulfillmentResponse](t, response)
		if body.State != StateShipped {
			t.Fatalf("state = %q, want shipped", body.State)
		}
	})
	t.Run("compensated", func(t *testing.T) {
		t.Parallel()
		response := postFulfillment(t, server, FulfillmentRequest{
			OrderID:                   "parallel-compensated",
			TotalCents:                2599,
			StockAvailable:            true,
			PaymentAuthorized:         true,
			ShipmentProviderAvailable: false,
		})
		if response.Code != http.StatusConflict {
			t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
		}
		body := decodeResponse[fulfillmentResponse](t, response)
		if !body.Compensated || body.State != StateCancelled {
			t.Fatalf("response = %#v, want compensated cancelled run", body)
		}
	})
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	server, err := NewServer(Options{})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func newTestRun(t *testing.T, request FulfillmentRequest) *orderRun {
	t.Helper()

	run, err := newOrderRun(request)
	if err != nil {
		t.Fatalf("new run: %v", err)
	}
	return run
}

func postFulfillment(t *testing.T, server *Server, request FulfillmentRequest) *httptest.ResponseRecorder {
	t.Helper()
	return postFulfillmentWithContext(t.Context(), t, server, request)
}

func postFulfillmentWithContext(ctx context.Context, t *testing.T, server *Server, request FulfillmentRequest) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return performRequestWithContext(ctx, t, server, http.MethodPost, "/orders/fulfillment", body)
}

func performRequest(t *testing.T, handler http.Handler, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return performRequestWithContext(t.Context(), t, handler, method, path, body)
}

func performRequestWithContext(ctx context.Context, t *testing.T, handler http.Handler, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
	return value
}

type childExpectation struct {
	name   string
	status workreport.Status
}

func assertReport(t *testing.T, report reportNode, name string, status workreport.Status) {
	t.Helper()

	if report.Name != name || report.Status != status {
		t.Fatalf("report = %#v, want name=%q status=%q", report, name, status)
	}
}

func assertChild(t *testing.T, parent reportNode, name string, status workreport.Status) reportNode {
	t.Helper()

	for _, child := range parent.Children {
		if child.Name == name {
			assertReport(t, child, name, status)
			return child
		}
	}
	t.Fatalf("child %q not found in %#v", name, parent.Children)
	return reportNode{}
}

func assertChildren(t *testing.T, parent reportNode, expectations []childExpectation) {
	t.Helper()

	if len(parent.Children) != len(expectations) {
		t.Fatalf("children = %#v, want %d children", parent.Children, len(expectations))
	}
	for i, expectation := range expectations {
		assertReport(t, parent.Children[i], expectation.name, expectation.status)
	}
}

func assertStateHistory(t *testing.T, actual []OrderState, expected ...OrderState) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("state history = %#v, want %#v", actual, expected)
	}
	for i, want := range expected {
		if actual[i] != want {
			t.Fatalf("state history = %#v, want %#v", actual, expected)
		}
	}
}
