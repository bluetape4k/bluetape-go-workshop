package compensation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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

func TestServerRunSuccessWithoutCompensation(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:                   "order-1001",
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if !body.Completed || body.Compensated || body.OriginalError != "" {
		t.Fatalf("completion flags = completed:%v compensated:%v original:%q", body.Completed, body.Compensated, body.OriginalError)
	}
	if !body.Effects.InventoryReserved || !body.Effects.PaymentAuthorized || !body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want every forward side effect retained", body.Effects)
	}
	assertReport(t, body.Report, "compensating-fulfillment", workreport.StatusCompleted)
	forward := assertChild(t, body.Report, "fulfillment-forward", workreport.StatusCompleted)
	assertChildren(t, forward, []childExpectation{
		{name: "reserve-inventory", status: workreport.StatusCompleted},
		{name: "authorize-payment", status: workreport.StatusCompleted},
		{name: "create-shipment", status: workreport.StatusCompleted},
	})
	assertNoChild(t, body.Report, "compensation")
}

func TestServerRunShipmentFailureCompensatesInReverseOrder(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:                   "order-1002",
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: false,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if body.Completed || !body.Compensated || !strings.Contains(body.OriginalError, ErrShipmentProviderUnavailable.Error()) {
		t.Fatalf("response = %#v, want compensated shipment failure", body)
	}
	if body.Effects.InventoryReserved || body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want inventory and payment undone", body.Effects)
	}
	assertReport(t, body.Report, "compensating-fulfillment", workreport.StatusFailed)
	forward := assertChild(t, body.Report, "fulfillment-forward", workreport.StatusFailed)
	assertChild(t, forward, "create-shipment", workreport.StatusFailed)
	compensation := assertChild(t, body.Report, "compensation", workreport.StatusCompleted)
	assertChildren(t, compensation, []childExpectation{
		{name: "void-payment", status: workreport.StatusCompleted},
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
}

func TestServerRunPaymentFailureReleasesOnlyInventory(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:                   "order-1003",
		StockAvailable:            true,
		PaymentAuthorized:         false,
		ShipmentProviderAvailable: true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if body.Completed || !body.Compensated || !strings.Contains(body.OriginalError, ErrPaymentDeclined.Error()) {
		t.Fatalf("response = %#v, want compensated payment failure", body)
	}
	if body.Effects.InventoryReserved || body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want inventory released and no payment/shipment", body.Effects)
	}
	compensation := assertChild(t, body.Report, "compensation", workreport.StatusCompleted)
	assertChildren(t, compensation, []childExpectation{
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
	assertNoChild(t, compensation, "void-payment")
}

func TestServerRunCompensationFailurePreservesOriginalErrorAndContinues(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:                   "order-1004",
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: false,
		VoidPaymentFails:          true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if body.Completed || !body.Compensated || !strings.Contains(body.OriginalError, ErrShipmentProviderUnavailable.Error()) {
		t.Fatalf("response = %#v, want original shipment failure preserved", body)
	}
	if body.Effects.InventoryReserved || !body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want payment left authorized and inventory released", body.Effects)
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

func TestServerRunInventoryFailureHasNoCompensation(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:                   "order-1005",
		StockAvailable:            false,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if body.Completed || body.Compensated || !strings.Contains(body.OriginalError, ErrInventoryUnavailable.Error()) {
		t.Fatalf("response = %#v, want uncompensated inventory failure", body)
	}
	if body.Effects.InventoryReserved || body.Effects.PaymentAuthorized || body.Effects.ShipmentCreated {
		t.Fatalf("effects = %#v, want no side effects", body.Effects)
	}
	assertNoChild(t, body.Report, "compensation")
}

func TestServerRunCallerCancellationMapsToRequestTimeout(t *testing.T) {
	server := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := postRunWithContext(ctx, t, server, RunRequest{
		OrderID:                   "order-cancelled",
		StockAvailable:            true,
		PaymentAuthorized:         true,
		ShipmentProviderAvailable: true,
	})
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("cancelled run status = %d, want %d: %s", response.Code, http.StatusRequestTimeout, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "compensating-fulfillment", workreport.StatusCancelled)
	if !body.Report.Cancelled || body.Compensated {
		t.Fatalf("cancelled response = %#v, want cancelled without compensation", body)
	}
}

func TestCompensationRunCancellationAfterSideEffectStillCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	run := &compensationRun{
		request: RunRequest{
			OrderID:                   "order-cancel-after-reserve",
			StockAvailable:            true,
			PaymentAuthorized:         true,
			ShipmentProviderAvailable: true,
		},
		afterReserve: cancel,
	}

	report, compensated, originalErr := run.execute(ctx)
	if originalErr == nil || !strings.Contains(originalErr.Error(), context.Canceled.Error()) {
		t.Fatalf("originalErr = %v, want context canceled", originalErr)
	}
	if !compensated {
		t.Fatalf("compensated = false, want true after side-effect cancellation")
	}
	assertReport(t, projectReport(report), "compensating-fulfillment", workreport.StatusCancelled)
	if run.inventoryReserved || run.paymentAuthorized || run.shipmentCreated {
		t.Fatalf("effects = inventory:%v payment:%v shipment:%v, want cleanup after cancellation", run.inventoryReserved, run.paymentAuthorized, run.shipmentCreated)
	}
	compensation := assertChild(t, projectReport(report), "compensation", workreport.StatusCompleted)
	assertChildren(t, compensation, []childExpectation{
		{name: "release-inventory", status: workreport.StatusCompleted},
	})
}

func TestServerRunBadRequests(t *testing.T) {
	server := newTestServer(t)

	tests := []struct {
		name string
		body []byte
	}{
		{name: "malformed", body: []byte("{")},
		{name: "missing order id", body: []byte(`{"stock_available":true,"payment_authorized":true}`)},
		{name: "blank order id", body: []byte(`{"order_id":"   ","stock_available":true,"payment_authorized":true}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(t, server, http.MethodPost, "/compensation/fulfillment", tt.body)
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

func TestCompensationRunParallelRequestsHaveIndependentState(t *testing.T) {
	server := newTestServer(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		response := postRun(t, server, RunRequest{
			OrderID:                   "parallel-success",
			StockAvailable:            true,
			PaymentAuthorized:         true,
			ShipmentProviderAvailable: true,
		})
		if response.Code != http.StatusOK {
			t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
		}
	})
	t.Run("compensated", func(t *testing.T) {
		t.Parallel()
		response := postRun(t, server, RunRequest{
			OrderID:                   "parallel-compensated",
			StockAvailable:            true,
			PaymentAuthorized:         true,
			ShipmentProviderAvailable: false,
		})
		if response.Code != http.StatusConflict {
			t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
		}
		body := decodeResponse[runResponse](t, response)
		if !body.Compensated {
			t.Fatalf("compensated = false, want true")
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

func postRun(t *testing.T, server *Server, request RunRequest) *httptest.ResponseRecorder {
	t.Helper()
	return postRunWithContext(t.Context(), t, server, request)
}

func postRunWithContext(ctx context.Context, t *testing.T, server *Server, request RunRequest) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return performRequestWithContext(ctx, t, server, http.MethodPost, "/compensation/fulfillment", body)
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

func assertNoChild(t *testing.T, parent reportNode, name string) {
	t.Helper()

	for _, child := range parent.Children {
		if child.Name == name {
			t.Fatalf("child %q found unexpectedly in %#v", name, parent.Children)
		}
	}
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
