package paymentauth

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

	current := performRequest(t, server, http.MethodGet, "/payments/current", nil)
	if current.Code != http.StatusOK {
		t.Fatalf("current status = %d, want %d", current.Code, http.StatusOK)
	}

	snapshot := decodeResponse[PaymentSnapshot](t, current)
	if snapshot.PaymentID != "pay-test" {
		t.Fatalf("payment id = %q, want pay-test", snapshot.PaymentID)
	}
	if snapshot.State != StateRequested {
		t.Fatalf("state = %q, want %q", snapshot.State, StateRequested)
	}
	assertEvents(t, snapshot.AllowedEvents, []PaymentEvent{EventAuthorize, EventFail, EventCancel})
}

func TestServerAuthorizeAndCapturePath(t *testing.T) {
	server := newTestServer(t, 12_900)

	authorized := postTransition(t, server, EventAuthorize, "auth-1")
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want %d: %s", authorized.Code, http.StatusOK, authorized.Body.String())
	}
	authBody := decodeResponse[transitionResponse](t, authorized)
	if authBody.Previous != StateRequested || authBody.Current != StateAuthorized || authBody.IdempotentReplay {
		t.Fatalf("authorize response = %#v", authBody)
	}
	assertEvents(t, authBody.Payment.AllowedEvents, []PaymentEvent{EventCapture, EventFail, EventCancel})

	canCapture := performRequest(t, server, http.MethodGet, "/payments/current/transitions/capture/can", nil)
	if canCapture.Code != http.StatusOK {
		t.Fatalf("can capture status = %d, want %d", canCapture.Code, http.StatusOK)
	}
	canCaptureBody := decodeResponse[canTransitionResponse](t, canCapture)
	if !canCaptureBody.Allowed || canCaptureBody.State != StateAuthorized {
		t.Fatalf("can capture response = %#v, want allowed from authorized", canCaptureBody)
	}

	captured := postTransition(t, server, EventCapture, "capture-1")
	if captured.Code != http.StatusOK {
		t.Fatalf("capture status = %d, want %d: %s", captured.Code, http.StatusOK, captured.Body.String())
	}
	captureBody := decodeResponse[transitionResponse](t, captured)
	if captureBody.Previous != StateAuthorized || captureBody.Current != StateCaptured {
		t.Fatalf("capture response = %#v", captureBody)
	}
	if len(captureBody.Payment.AllowedEvents) != 0 {
		t.Fatalf("allowed events after capture = %#v, want empty", captureBody.Payment.AllowedEvents)
	}
}

func TestServerFailureAndCancelReachFinalStates(t *testing.T) {
	tests := []struct {
		name      string
		event     PaymentEvent
		wantState PaymentState
		before    []PaymentEvent
	}{
		{name: "fail requested", event: EventFail, wantState: StateFailed},
		{name: "fail authorized", event: EventFail, wantState: StateFailed, before: []PaymentEvent{EventAuthorize}},
		{name: "cancel requested", event: EventCancel, wantState: StateCancelled},
		{name: "cancel authorized", event: EventCancel, wantState: StateCancelled, before: []PaymentEvent{EventAuthorize}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTestServer(t, 12_900)
			for i, event := range tt.before {
				response := postTransition(t, server, event, tt.name+"-before-"+string(rune('0'+i)))
				if response.Code != http.StatusOK {
					t.Fatalf("%s status = %d, want %d: %s", event, response.Code, http.StatusOK, response.Body.String())
				}
			}

			response := postTransition(t, server, tt.event, tt.name+"-final")
			if response.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want %d: %s", tt.event, response.Code, http.StatusOK, response.Body.String())
			}
			body := decodeResponse[transitionResponse](t, response)
			if body.Current != tt.wantState || body.Payment.State != tt.wantState {
				t.Fatalf("final response = %#v, want %q", body, tt.wantState)
			}

			next := postTransition(t, server, EventCapture, tt.name+"-after")
			if next.Code != http.StatusConflict {
				t.Fatalf("transition after final status = %d, want %d", next.Code, http.StatusConflict)
			}
			errBody := decodeResponse[errorResponse](t, next)
			if errBody.Code != "final_state" {
				t.Fatalf("error code = %q, want final_state", errBody.Code)
			}
		})
	}
}

func TestServerInvalidTransitionReturnsConflictAndKeepsState(t *testing.T) {
	server := newTestServer(t, 12_900)

	response := postTransition(t, server, EventCapture, "capture-before-authorize")
	if response.Code != http.StatusConflict {
		t.Fatalf("capture from requested status = %d, want %d", response.Code, http.StatusConflict)
	}
	body := decodeResponse[errorResponse](t, response)
	if body.Code != "invalid_transition" {
		t.Fatalf("error code = %q, want invalid_transition", body.Code)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateRequested {
		t.Fatalf("state after invalid transition = %q, want %q", snapshot.State, StateRequested)
	}
}

func TestServerGuardRejectionReturnsConflictAndKeepsState(t *testing.T) {
	server := newTestServer(t, 0)

	response := postTransition(t, server, EventAuthorize, "auth-zero")
	if response.Code != http.StatusConflict {
		t.Fatalf("authorize zero status = %d, want %d", response.Code, http.StatusConflict)
	}
	body := decodeResponse[errorResponse](t, response)
	if body.Code != "guard_rejected" {
		t.Fatalf("error code = %q, want guard_rejected", body.Code)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateRequested {
		t.Fatalf("state after guard rejection = %q, want %q", snapshot.State, StateRequested)
	}

	canAuthorize := performRequest(t, server, http.MethodGet, "/payments/current/transitions/authorize/can", nil)
	if canAuthorize.Code != http.StatusConflict {
		t.Fatalf("can authorize status = %d, want %d", canAuthorize.Code, http.StatusConflict)
	}
}

func TestServerCanTransitionEndpointReportsUnavailableWithoutMutating(t *testing.T) {
	server := newTestServer(t, 12_900)

	capture := performRequest(t, server, http.MethodGet, "/payments/current/transitions/capture/can", nil)
	if capture.Code != http.StatusOK {
		t.Fatalf("can capture status = %d, want %d", capture.Code, http.StatusOK)
	}
	body := decodeResponse[canTransitionResponse](t, capture)
	if body.Allowed {
		t.Fatalf("capture allowed from requested = true, want false")
	}
	if body.Error == "" {
		t.Fatalf("unavailable error is empty")
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateRequested {
		t.Fatalf("state after can capture = %q, want %q", snapshot.State, StateRequested)
	}
}

func TestServerIdempotentReplayDoesNotMutateState(t *testing.T) {
	server := newTestServer(t, 12_900)

	first := postTransition(t, server, EventAuthorize, "auth-retry")
	if first.Code != http.StatusOK {
		t.Fatalf("first authorize status = %d, want %d", first.Code, http.StatusOK)
	}
	firstBody := decodeResponse[transitionResponse](t, first)
	if firstBody.IdempotentReplay {
		t.Fatalf("first response should not be replay: %#v", firstBody)
	}

	retry := postTransition(t, server, EventAuthorize, "auth-retry")
	if retry.Code != http.StatusOK {
		t.Fatalf("retry authorize status = %d, want %d: %s", retry.Code, http.StatusOK, retry.Body.String())
	}
	retryBody := decodeResponse[transitionResponse](t, retry)
	if !retryBody.IdempotentReplay {
		t.Fatalf("retry response should be replay: %#v", retryBody)
	}
	if retryBody.Previous != firstBody.Previous || retryBody.Current != firstBody.Current {
		t.Fatalf("retry response = %#v, want original transition %#v", retryBody, firstBody)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateAuthorized {
		t.Fatalf("state after idempotent replay = %q, want %q", snapshot.State, StateAuthorized)
	}
}

func TestServerIdempotencyKeyConflict(t *testing.T) {
	server := newTestServer(t, 12_900)

	first := postTransition(t, server, EventAuthorize, "payment-key-1")
	if first.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want %d", first.Code, http.StatusOK)
	}

	conflict := postTransition(t, server, EventCapture, "payment-key-1")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("idempotency conflict status = %d, want %d", conflict.Code, http.StatusConflict)
	}
	body := decodeResponse[errorResponse](t, conflict)
	if body.Code != "idempotency_conflict" {
		t.Fatalf("error code = %q, want idempotency_conflict", body.Code)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateAuthorized {
		t.Fatalf("state after idempotency conflict = %q, want %q", snapshot.State, StateAuthorized)
	}
}

func TestServerDoesNotStoreFailedTransitionsAsReplay(t *testing.T) {
	server := newTestServer(t, 12_900)

	invalid := postTransition(t, server, EventCapture, "capture-invalid")
	if invalid.Code != http.StatusConflict {
		t.Fatalf("invalid capture status = %d, want %d", invalid.Code, http.StatusConflict)
	}

	authorize := postTransition(t, server, EventAuthorize, "auth-after-invalid")
	if authorize.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want %d", authorize.Code, http.StatusOK)
	}

	capture := postTransition(t, server, EventCapture, "capture-invalid")
	if capture.Code != http.StatusOK {
		t.Fatalf("capture with previously failed key status = %d, want %d: %s", capture.Code, http.StatusOK, capture.Body.String())
	}
	body := decodeResponse[transitionResponse](t, capture)
	if body.IdempotentReplay {
		t.Fatalf("successful capture after failed first use should not be replay: %#v", body)
	}
}

func TestServerBadRequests(t *testing.T) {
	server := newTestServer(t, 12_900)

	tests := []struct {
		name string
		body []byte
	}{
		{name: "malformed", body: []byte("{")},
		{name: "missing event", body: []byte(`{"idempotency_key":"key-1"}`)},
		{name: "blank event", body: []byte(`{"event":" ","idempotency_key":"key-1"}`)},
		{name: "unknown event", body: []byte(`{"event":"refund","idempotency_key":"key-1"}`)},
		{name: "missing idempotency key", body: []byte(`{"event":"authorize"}`)},
		{name: "blank idempotency key", body: []byte(`{"event":"authorize","idempotency_key":" "}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(t, server, http.MethodPost, "/payments/current/transitions", tt.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			body := decodeResponse[errorResponse](t, response)
			if body.Code != "invalid_request" {
				t.Fatalf("error code = %q, want invalid_request", body.Code)
			}
		})
	}
}

func TestServerRejectsNegativeAmountAtConstruction(t *testing.T) {
	_, err := NewServer(Options{PaymentID: "pay-test", AmountCents: -1})
	if err == nil {
		t.Fatalf("NewServer negative amount error = nil, want error")
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
		key := "auth-concurrent-" + string(rune('a'+i))
		go func() {
			defer wg.Done()
			<-start
			results <- postTransition(t, server, EventAuthorize, key).Code
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
		t.Fatalf("successful authorize requests = %d, want 1", successes)
	}
	if conflicts != requests-1 {
		t.Fatalf("conflicting authorize requests = %d, want %d", conflicts, requests-1)
	}

	snapshot := currentSnapshot(t, server)
	if snapshot.State != StateAuthorized {
		t.Fatalf("state after concurrent authorize = %q, want %q", snapshot.State, StateAuthorized)
	}
}

func newTestServer(t *testing.T, amountCents int) *Server {
	t.Helper()

	server, err := NewServer(Options{
		PaymentID:   "pay-test",
		AmountCents: amountCents,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func postTransition(t *testing.T, server *Server, event PaymentEvent, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{"event":"` + string(event) + `","idempotency_key":"` + idempotencyKey + `"}`)
	return performRequest(t, server, http.MethodPost, "/payments/current/transitions", body)
}

func currentSnapshot(t *testing.T, server *Server) PaymentSnapshot {
	t.Helper()

	response := performRequest(t, server, http.MethodGet, "/payments/current", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("current status = %d, want %d", response.Code, http.StatusOK)
	}
	return decodeResponse[PaymentSnapshot](t, response)
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

func assertEvents(t *testing.T, got []PaymentEvent, want []PaymentEvent) {
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
