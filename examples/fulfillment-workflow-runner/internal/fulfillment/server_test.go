package fulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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

func TestServerRunSuccessCreatesShipment(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:           "order-1001",
		StockAvailable:    true,
		PaymentAuthorized: true,
		RequiresShipment:  true,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if !body.ShipmentCreated || body.ShipmentSkipped {
		t.Fatalf("shipment flags = created:%v skipped:%v, want created only", body.ShipmentCreated, body.ShipmentSkipped)
	}
	assertReport(t, body.Report, "fulfillment", workreport.StatusCompleted)
	assertChild(t, body.Report, "validate-order", workreport.StatusCompleted)
	riskChecks := assertChild(t, body.Report, "risk-checks", workreport.StatusCompleted)
	assertChild(t, riskChecks, "reserve-inventory", workreport.StatusCompleted)
	assertChild(t, riskChecks, "authorize-payment", workreport.StatusCompleted)
	shipment := assertChild(t, body.Report, "shipment-decision", workreport.StatusCompleted)
	assertChild(t, shipment, "create-shipment", workreport.StatusCompleted)
}

func TestServerRunConditionalSkipCompletesWithoutShipment(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:           "pickup-1001",
		StockAvailable:    true,
		PaymentAuthorized: true,
		RequiresShipment:  false,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	if body.ShipmentCreated || !body.ShipmentSkipped {
		t.Fatalf("shipment flags = created:%v skipped:%v, want skipped only", body.ShipmentCreated, body.ShipmentSkipped)
	}
	shipment := assertChild(t, body.Report, "shipment-decision", workreport.StatusCompleted)
	assertChild(t, shipment, "shipment-skipped", workreport.StatusCompleted)
}

func TestServerRunInventoryFailureReturnsConflict(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:           "order-1002",
		StockAvailable:    false,
		PaymentAuthorized: true,
		RequiresShipment:  true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "fulfillment", workreport.StatusFailed)
	riskChecks := assertChild(t, body.Report, "risk-checks", workreport.StatusFailed)
	inventory := assertChild(t, riskChecks, "reserve-inventory", workreport.StatusFailed)
	if inventory.Error == "" {
		t.Fatalf("inventory failure should expose error: %#v", inventory)
	}
}

func TestServerRunPaymentFailureCancelsSlowInventorySibling(t *testing.T) {
	server := newTestServer(t)

	response := postRun(t, server, RunRequest{
		OrderID:           "order-1003",
		StockAvailable:    true,
		PaymentAuthorized: false,
		RequiresShipment:  true,
		InventoryDelayMS:  50,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("run status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	riskChecks := assertChild(t, body.Report, "risk-checks", workreport.StatusFailed)
	assertChild(t, riskChecks, "authorize-payment", workreport.StatusFailed)
	inventory := assertChild(t, riskChecks, "reserve-inventory", workreport.StatusCancelled)
	if !inventory.Cancelled {
		t.Fatalf("inventory should be cancelled after payment failure: %#v", inventory)
	}
	if body.ShipmentCreated || body.ShipmentSkipped {
		t.Fatalf("shipment should not run after risk failure: created:%v skipped:%v", body.ShipmentCreated, body.ShipmentSkipped)
	}
}

func TestServerRunCallerCancellationMapsToRequestTimeout(t *testing.T) {
	server := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := postRunWithContext(ctx, t, server, RunRequest{
		OrderID:           "order-cancelled",
		StockAvailable:    true,
		PaymentAuthorized: true,
		RequiresShipment:  true,
	})
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("cancelled run status = %d, want %d: %s", response.Code, http.StatusRequestTimeout, response.Body.String())
	}

	body := decodeResponse[runResponse](t, response)
	assertReport(t, body.Report, "fulfillment", workreport.StatusCancelled)
	if !body.Report.Cancelled {
		t.Fatalf("report should be cancelled: %#v", body.Report)
	}
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
		{name: "negative delay", body: []byte(`{"order_id":"order-1","inventory_delay_ms":-1}`)},
		{name: "excessive delay", body: []byte(`{"order_id":"order-1","inventory_delay_ms":250}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(t, server, http.MethodPost, "/fulfillment/run", tt.body)
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

func newTestServer(t *testing.T) *Server {
	t.Helper()

	server, err := NewServer(Options{MaxInventoryDelay: 100 * time.Millisecond})
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
	return performRequestWithContext(ctx, t, server, http.MethodPost, "/fulfillment/run", body)
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
