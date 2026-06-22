package customermigration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestHTTPManualCrashScheduledRestartAndReport(t *testing.T) {
	server := newHTTPTestServer(t, ServiceOptions{LeaderGate: NewStaticLeaderGate(true)})

	crash := postJSON(t, server, "/batch/start", StartRequest{
		RunID:               "manual-001",
		CrashAfterNewWrites: 3,
	})
	if crash.Code != http.StatusConflict {
		t.Fatalf("crash status = %d, want %d: %s", crash.Code, http.StatusConflict, crash.Body.String())
	}
	assertErrorCode(t, crash, ErrorCodeWriterCrash)
	assertNoFixtureEmail(t, crash.Body.String())

	status := get(t, server, "/batch/status")
	if status.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d: %s", status.Code, http.StatusOK, status.Body.String())
	}
	var statusBody StatusResponse
	decodeBody(t, status, &statusBody)
	if !statusBody.Checkpoint.Valid || statusBody.Checkpoint.Value.NextIndex != 2 {
		t.Fatalf("status checkpoint = %#v, want valid NextIndex=2", statusBody.Checkpoint)
	}

	scheduled := postJSON(t, server, "/batch/schedule/tick", ScheduleRequest{RunID: "scheduled-001"})
	if scheduled.Code != http.StatusOK {
		t.Fatalf("scheduled status = %d, want %d: %s", scheduled.Code, http.StatusOK, scheduled.Body.String())
	}
	var scheduledBody RunResponse
	decodeBody(t, scheduled, &scheduledBody)
	if scheduledBody.Status != batch.StatusCompleted {
		t.Fatalf("scheduled batch status = %s, want completed", scheduledBody.Status)
	}
	if scheduledBody.Summary.DuplicateSkipCount != 1 {
		t.Fatalf("duplicate skips = %d, want 1", scheduledBody.Summary.DuplicateSkipCount)
	}
	assertNoFixtureEmail(t, scheduled.Body.String())

	report := get(t, server, "/batch/report")
	if report.Code != http.StatusOK {
		t.Fatalf("report code = %d, want %d: %s", report.Code, http.StatusOK, report.Body.String())
	}
	assertNoFixtureEmail(t, report.Body.String())
}

func TestHTTPLeaderMissingDoesNotMutateState(t *testing.T) {
	service := NewService(ServiceOptions{LeaderGate: NewStaticLeaderGate(false)})
	server := newHTTPTestServerFromService(t, service)

	response := postJSON(t, server, "/batch/schedule/tick", ScheduleRequest{RunID: "scheduled-001"})
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	assertErrorCode(t, response, ErrorCodeNotLeader)

	status := service.Status()
	if status.Checkpoint.Valid {
		t.Fatalf("checkpoint mutated on missing leadership: %#v", status.Checkpoint)
	}
	if len(status.MigratedIDs) != 0 || len(status.DeadLetters) != 0 {
		t.Fatalf("state mutated on missing leadership: %#v", status)
	}
}

func TestHTTPValidationAndBodyLimit(t *testing.T) {
	server := newHTTPTestServer(t, ServiceOptions{LeaderGate: NewStaticLeaderGate(true)})

	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "malformed start", path: "/batch/start", body: "{", wantStatus: http.StatusBadRequest, wantCode: ErrorCodeInvalidRequest},
		{name: "blank run id", path: "/batch/start", body: `{"run_id":"   "}`, wantStatus: http.StatusBadRequest, wantCode: ErrorCodeInvalidRunID},
		{name: "invalid run id", path: "/batch/start", body: `{"run_id":"bad/id"}`, wantStatus: http.StatusBadRequest, wantCode: ErrorCodeInvalidRunID},
		{name: "invalid crash", path: "/batch/start", body: `{"run_id":"manual-001","crash_after_new_writes":99}`, wantStatus: http.StatusBadRequest, wantCode: ErrorCodeInvalidCrashAfter},
		{name: "malformed schedule", path: "/batch/schedule/tick", body: "{", wantStatus: http.StatusBadRequest, wantCode: ErrorCodeInvalidRequest},
		{name: "blank schedule run id", path: "/batch/schedule/tick", body: `{"run_id":"   "}`, wantStatus: http.StatusBadRequest, wantCode: ErrorCodeInvalidRunID},
		{name: "oversized start", path: "/batch/start", body: `{"run_id":"manual-001","padding":"` + strings.Repeat("x", int(maxRequestBodyBytes)) + `"}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: ErrorCodeRequestTooLarge},
		{name: "oversized schedule", path: "/batch/schedule/tick", body: `{"run_id":"scheduled-001","padding":"` + strings.Repeat("x", int(maxRequestBodyBytes)) + `"}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: ErrorCodeRequestTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(t, server, http.MethodPost, tt.path, []byte(tt.body))
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body.String())
			}
			assertErrorCode(t, response, tt.wantCode)
			assertNoFixtureEmail(t, response.Body.String())
		})
	}
}

func TestHTTPReportNotFoundAndCancelNoActiveRun(t *testing.T) {
	server := newHTTPTestServer(t, ServiceOptions{LeaderGate: NewStaticLeaderGate(true)})

	report := get(t, server, "/batch/report")
	if report.Code != http.StatusNotFound {
		t.Fatalf("report status = %d, want %d: %s", report.Code, http.StatusNotFound, report.Body.String())
	}
	assertErrorCode(t, report, ErrorCodeReportNotFound)

	cancel := postJSON(t, server, "/batch/cancel", CancelRequest{Reason: "nothing active"})
	if cancel.Code != http.StatusNotFound {
		t.Fatalf("cancel status = %d, want %d: %s", cancel.Code, http.StatusNotFound, cancel.Body.String())
	}
	assertErrorCode(t, cancel, ErrorCodeNoActiveRun)
}

func TestHTTPCancelActiveRun(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	service := NewService(ServiceOptions{
		LeaderGate: NewStaticLeaderGate(true),
		BeforeRun: func(ctx context.Context) {
			close(started)
			select {
			case <-ctx.Done():
			case <-release:
			}
		},
	})
	server := newHTTPTestServerFromService(t, service)

	var startResponse *httptest.ResponseRecorder
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startResponse = postJSON(t, server, "/batch/start", StartRequest{RunID: "manual-cancel"})
	}()
	<-started

	cancel := postJSON(t, server, "/batch/cancel", CancelRequest{Reason: "operator requested stop"})
	if cancel.Code != http.StatusAccepted {
		t.Fatalf("cancel status = %d, want %d: %s", cancel.Code, http.StatusAccepted, cancel.Body.String())
	}
	var cancelBody CancelResponse
	decodeBody(t, cancel, &cancelBody)
	if cancelBody.Status != "cancel_requested" || cancelBody.ErrorCode != "" {
		t.Fatalf("cancel response = %#v, want non-error cancel_requested", cancelBody)
	}
	close(release)
	wg.Wait()
	if startResponse.Code != http.StatusRequestTimeout {
		t.Fatalf("start status after cancel = %d, want %d: %s", startResponse.Code, http.StatusRequestTimeout, startResponse.Body.String())
	}
	assertErrorCode(t, startResponse, ErrorCodeRequestCancelled)
	if service.Status().Active {
		t.Fatalf("service active after cancellation")
	}
}

func TestLeaderGateCancellationAndAlreadyLeader(t *testing.T) {
	t.Parallel()

	t.Run("campaign cancellation", func(t *testing.T) {
		t.Parallel()
		gate := &scriptedLeaderGate{campaignErr: context.Canceled}
		service := NewService(ServiceOptions{LeaderGate: gate})
		_, err := service.RunScheduledTick(context.Background(), ScheduleRequest{RunID: "scheduled-cancel"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunScheduledTick() error = %v, want context.Canceled", err)
		}
		if gate.runs != 0 {
			t.Fatalf("batch ran after campaign cancellation")
		}
	})

	t.Run("already leader runs without resign", func(t *testing.T) {
		t.Parallel()
		gate := &scriptedLeaderGate{campaignErr: leader.ErrAlreadyLeader, held: true}
		service := NewService(ServiceOptions{LeaderGate: gate})
		response, err := service.RunScheduledTick(context.Background(), ScheduleRequest{RunID: "scheduled-already"})
		if err != nil {
			t.Fatalf("RunScheduledTick() error = %v", err)
		}
		if response.Status != batch.StatusCompleted {
			t.Fatalf("status = %s, want completed", response.Status)
		}
		if gate.resigns != 0 {
			t.Fatalf("resigns = %d, want 0 for already-held leadership", gate.resigns)
		}
	})
}

func TestHTTPConcurrentMixedAccess(t *testing.T) {
	service := NewService(ServiceOptions{LeaderGate: NewStaticLeaderGate(true)})
	server := newHTTPTestServerFromService(t, service)

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 25; round++ {
				_ = get(t, server, "/batch/status")
				_ = get(t, server, "/batch/report")
				_ = postJSON(t, server, "/batch/start", StartRequest{RunID: "manual-mixed"})
				_ = postJSON(t, server, "/batch/schedule/tick", ScheduleRequest{RunID: "scheduled-mixed"})
				if worker%3 == 0 {
					_ = postJSON(t, server, "/batch/cancel", CancelRequest{Reason: "stress"})
				}
			}
		}()
	}
	wg.Wait()
}

type scriptedLeaderGate struct {
	campaignErr error
	held        bool
	runs        int
	resigns     int
}

func (g *scriptedLeaderGate) RunIfLeader(ctx context.Context, run func(context.Context) (RunResponse, error)) (RunResponse, error) {
	if g.campaignErr != nil {
		if errors.Is(g.campaignErr, leader.ErrAlreadyLeader) {
			g.runs++
			return run(ctx)
		}
		return RunResponse{}, g.campaignErr
	}
	if !g.held {
		return RunResponse{}, ErrNotLeader
	}
	g.runs++
	response, err := run(ctx)
	g.resigns++
	return response, err
}

func (g *scriptedLeaderGate) LeaderHeld() bool {
	return g.held || errors.Is(g.campaignErr, leader.ErrAlreadyLeader)
}

func newHTTPTestServer(t *testing.T, options ServiceOptions) http.Handler {
	t.Helper()
	return newHTTPTestServerFromService(t, NewService(options))
}

func newHTTPTestServerFromService(t *testing.T, service *Service) http.Handler {
	t.Helper()
	router, err := NewRouter(service)
	if err != nil {
		t.Fatalf("NewRouter(): %v", err)
	}
	return router
}

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(t, handler, http.MethodGet, path, nil)
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return performRequest(t, handler, http.MethodPost, path, data)
}

func performRequest(t *testing.T, handler http.Handler, method string, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeBody(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		ErrorCode string `json:"error_code"`
		Code      string `json:"code"`
	}
	decodeBody(t, response, &body)
	got := body.ErrorCode
	if got == "" {
		got = body.Code
	}
	if got != want {
		t.Fatalf("error code = %q, want %q: %s", got, want, response.Body.String())
	}
}

func assertNoFixtureEmail(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "@example.test") {
		t.Fatalf("response leaked fixture email: %s", body)
	}
}
