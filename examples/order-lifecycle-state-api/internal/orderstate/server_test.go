package orderstate

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestServerHealthAndCurrentState(t *testing.T) {
	server := newTestServer(t, 12_900)

	health := performRequest(t, server, http.MethodGet, "/healthz", nil)
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", health.Code, http.StatusOK)
	}

	current := performRequest(t, server, http.MethodGet, "/orders/current", nil)
	if current.Code != http.StatusOK {
		t.Fatalf("current status = %d, want %d", current.Code, http.StatusOK)
	}

	snapshot := decodeResponse[OrderSnapshot](t, current)
	if snapshot.OrderID != "order-test" {
		t.Fatalf("order id = %q, want order-test", snapshot.OrderID)
	}
	if snapshot.State != StateDraft {
		t.Fatalf("state = %q, want %q", snapshot.State, StateDraft)
	}
	assertEvents(t, snapshot.AllowedEvents, []OrderEvent{EventSubmit, EventCancel})
}

func TestServerAllowedTransitionPath(t *testing.T) {
	server := newTestServer(t, 12_900)

	submitted := postTransition(t, server, EventSubmit)
	if submitted.Code != http.StatusOK {
		t.Fatalf("submit status = %d, want %d", submitted.Code, http.StatusOK)
	}
	submitResponse := decodeResponse[transitionResponse](t, submitted)
	if submitResponse.Previous != StateDraft || submitResponse.Current != StateSubmitted {
		t.Fatalf("submit transition = %q -> %q, want %q -> %q", submitResponse.Previous, submitResponse.Current, StateDraft, StateSubmitted)
	}

	canPay := performRequest(t, server, http.MethodGet, "/orders/current/transitions/pay/can", nil)
	if canPay.Code != http.StatusOK {
		t.Fatalf("can pay status = %d, want %d", canPay.Code, http.StatusOK)
	}
	canPayResponse := decodeResponse[canTransitionResponse](t, canPay)
	if !canPayResponse.Allowed {
		t.Fatalf("pay allowed = false, want true: %#v", canPayResponse)
	}

	paid := postTransition(t, server, EventPay)
	if paid.Code != http.StatusOK {
		t.Fatalf("pay status = %d, want %d", paid.Code, http.StatusOK)
	}
	payResponse := decodeResponse[transitionResponse](t, paid)
	if payResponse.Previous != StateSubmitted || payResponse.Current != StatePaid {
		t.Fatalf("pay transition = %q -> %q, want %q -> %q", payResponse.Previous, payResponse.Current, StateSubmitted, StatePaid)
	}
	assertEvents(t, payResponse.Order.AllowedEvents, []OrderEvent{EventPack, EventCancel})
}

func TestServerInvalidTransitionReturnsConflictAndKeepsState(t *testing.T) {
	server := newTestServer(t, 12_900)

	response := postTransition(t, server, EventPay)
	if response.Code != http.StatusConflict {
		t.Fatalf("pay from draft status = %d, want %d", response.Code, http.StatusConflict)
	}
	errBody := decodeResponse[errorResponse](t, response)
	if errBody.Code != "invalid_transition" {
		t.Fatalf("error code = %q, want invalid_transition", errBody.Code)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateDraft {
		t.Fatalf("state after invalid transition = %q, want %q", snapshot.State, StateDraft)
	}
}

func TestServerGuardRejectionReturnsConflictAndKeepsState(t *testing.T) {
	server := newTestServer(t, 0)

	submit := postTransition(t, server, EventSubmit)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit status = %d, want %d", submit.Code, http.StatusOK)
	}

	pay := postTransition(t, server, EventPay)
	if pay.Code != http.StatusConflict {
		t.Fatalf("pay status = %d, want %d", pay.Code, http.StatusConflict)
	}
	errBody := decodeResponse[errorResponse](t, pay)
	if errBody.Code != "guard_rejected" {
		t.Fatalf("error code = %q, want guard_rejected", errBody.Code)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateSubmitted {
		t.Fatalf("state after guard rejection = %q, want %q", snapshot.State, StateSubmitted)
	}
}

func TestServerFinalStateRejectsFurtherTransitions(t *testing.T) {
	server := newTestServer(t, 12_900)

	for _, event := range []OrderEvent{EventSubmit, EventPay, EventPack, EventShip} {
		response := postTransition(t, server, event)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", event, response.Code, http.StatusOK)
		}
	}

	cancel := postTransition(t, server, EventCancel)
	if cancel.Code != http.StatusConflict {
		t.Fatalf("cancel after shipped status = %d, want %d", cancel.Code, http.StatusConflict)
	}
	errBody := decodeResponse[errorResponse](t, cancel)
	if errBody.Code != "final_state" {
		t.Fatalf("error code = %q, want final_state", errBody.Code)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateShipped {
		t.Fatalf("state after final-state rejection = %q, want %q", snapshot.State, StateShipped)
	}
	if len(snapshot.AllowedEvents) != 0 {
		t.Fatalf("allowed events in final state = %#v, want empty", snapshot.AllowedEvents)
	}
}

func TestServerBadRequests(t *testing.T) {
	server := newTestServer(t, 12_900)

	malformed := performRequest(t, server, http.MethodPost, "/orders/current/transitions", []byte("{"))
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed status = %d, want %d", malformed.Code, http.StatusBadRequest)
	}
	malformedBody := decodeResponse[errorResponse](t, malformed)
	if malformedBody.Code != "invalid_request" {
		t.Fatalf("malformed error code = %q, want invalid_request", malformedBody.Code)
	}

	unknown := performRequest(t, server, http.MethodPost, "/orders/current/transitions", []byte(`{"event":"refund"}`))
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown event status = %d, want %d", unknown.Code, http.StatusBadRequest)
	}
	unknownBody := decodeResponse[errorResponse](t, unknown)
	if unknownBody.Code != "unknown_event" {
		t.Fatalf("unknown event code = %q, want unknown_event", unknownBody.Code)
	}
}

func TestServerConcurrentDuplicateTransitionSafety(t *testing.T) {
	server := newTestServer(t, 12_900)

	const requests = 24
	start := make(chan struct{})
	results := make(chan int, requests)

	var wg sync.WaitGroup
	wg.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer wg.Done()
			<-start
			results <- postTransition(t, server, EventSubmit).Code
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for status := range results {
		switch status {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent status = %d", status)
		}
	}

	if successes != 1 {
		t.Fatalf("successful submit requests = %d, want 1", successes)
	}
	if conflicts != requests-1 {
		t.Fatalf("conflicting submit requests = %d, want %d", conflicts, requests-1)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateSubmitted {
		t.Fatalf("state after concurrent submit = %q, want %q", snapshot.State, StateSubmitted)
	}
}

func newTestServer(t *testing.T, totalCents int) *Server {
	t.Helper()

	server, err := NewServer(Options{
		OrderID:    "order-test",
		TotalCents: totalCents,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func postTransition(t *testing.T, server *Server, event OrderEvent) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{"event":"` + string(event) + `"}`)
	return performRequest(t, server, http.MethodPost, "/orders/current/transitions", body)
}

func currentSnapshot(t *testing.T, server *Server) OrderSnapshot {
	t.Helper()

	response := performRequest(t, server, http.MethodGet, "/orders/current", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("current status = %d, want %d", response.Code, http.StatusOK)
	}
	return decodeResponse[OrderSnapshot](t, response)
}

func performRequest(t *testing.T, handler http.Handler, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(body))
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

func assertEvents(t *testing.T, got []OrderEvent, want []OrderEvent) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %#v, want %#v", got, want)
		}
	}
}
