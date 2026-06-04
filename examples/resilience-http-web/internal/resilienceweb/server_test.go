package resilienceweb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/resilience-http-web/internal/resilienceweb"
	"github.com/bluetape4k/bluetape-go/resilience"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestResilienceWebRetriesTransientCatalogFailure(t *testing.T) {
	attempts := 0
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path != "/items/book-1" {
			t.Fatalf("catalog path = %s", r.URL.Path)
		}
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		writeCatalogItem(w, "book-1")
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/book-1", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", response.Code, response.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if !strings.Contains(response.Body.String(), `"id":"book-1"`) {
		t.Fatalf("catalog body = %s", response.Body.String())
	}
}

func TestResilienceWebOpensCircuitAfterCatalogFailures(t *testing.T) {
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/book-1", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("failure status = %d, body = %s", response.Code, response.Body.String())
	}

	rejected := httptest.NewRecorder()
	server.ServeHTTP(rejected, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/book-1", nil))
	if rejected.Code != http.StatusServiceUnavailable {
		t.Fatalf("rejected status = %d, body = %s", rejected.Code, rejected.Body.String())
	}
}

func TestResilienceWebCircuitHalfOpenProbeRecovers(t *testing.T) {
	var failCatalog bool
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failCatalog {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		writeCatalogItem(w, "book-1")
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)
	failCatalog = true
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/book-1", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("failure status = %d", response.Code)
	}

	failCatalog = false
	time.Sleep(120 * time.Millisecond)

	recovered := httptest.NewRecorder()
	server.ServeHTTP(recovered, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/book-1", nil))
	if recovered.Code != http.StatusOK {
		t.Fatalf("recovered status = %d, body = %s", recovered.Code, recovered.Body.String())
	}
}

func TestResilienceWebRejectsConcurrentOrdersThroughBulkhead(t *testing.T) {
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeCatalogItem(w, "book-1")
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)

	start := make(chan struct{})
	var wg sync.WaitGroup
	statuses := make(chan int, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/orders?delay=80ms", nil))
			statuses <- response.Code
		}()
	}

	close(start)
	wg.Wait()
	close(statuses)

	var accepted, rejected int
	for status := range statuses {
		switch status {
		case http.StatusAccepted:
			accepted++
		case http.StatusTooManyRequests:
			rejected++
		default:
			t.Fatalf("unexpected status = %d", status)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("accepted=%d rejected=%d", accepted, rejected)
	}
}

func TestResilienceWebExposesPolicyEvents(t *testing.T) {
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/catalog/book-1", nil))

	eventsResponse := httptest.NewRecorder()
	server.ServeHTTP(eventsResponse, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/events", nil))
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("events status = %d", eventsResponse.Code)
	}

	var events []struct {
		PolicyType string `json:"policy_type"`
		Category   string `json:"category"`
	}
	if err := json.NewDecoder(eventsResponse.Body).Decode(&events); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected resilience events")
	}

	seenRetry := false
	for _, event := range events {
		if event.PolicyType == resilience.PolicyTypeRetry && event.Category == string(resilience.EventCategoryRetry) {
			seenRetry = true
		}
	}
	if !seenRetry {
		t.Fatalf("events = %+v", events)
	}
}

func TestResilienceWebBulkheadStressUsesGoroutineStressTester(t *testing.T) {
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeCatalogItem(w, "book-1")
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)
	var accepted atomic.Int32
	var rejected atomic.Int32

	task := func(ctx context.Context) error {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodPost, "/orders?delay=30ms", nil))
		switch response.Code {
		case http.StatusAccepted:
			accepted.Add(1)
		case http.StatusTooManyRequests:
			rejected.Add(1)
		default:
			t.Fatalf("unexpected order status = %d, body = %s", response.Code, response.Body.String())
		}
		return nil
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 4,
		Timeout:       3 * time.Second,
	})
	report := tester.RunT(t, task, task, task, task, task, task, task, task)
	if report.Completed != 32 {
		t.Fatalf("stress report = %+v", report)
	}
	if accepted.Load() == 0 || rejected.Load() == 0 {
		t.Fatalf("accepted=%d rejected=%d", accepted.Load(), rejected.Load())
	}
}

func TestResilienceWebCatalogDeadlineUsesAsyncJobTester(t *testing.T) {
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			writeCatalogItem(w, "book-1")
		}
	}))
	t.Cleanup(catalog.Close)

	server := newTestServer(t, catalog.URL)
	job := func(ctx context.Context) error {
		requestCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()

		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequestWithContext(requestCtx, http.MethodGet, "/catalog/book-1", nil))
		if response.Code != http.StatusBadGateway && response.Code != http.StatusServiceUnavailable {
			t.Fatalf("deadline status = %d, body = %s", response.Code, response.Body.String())
		}
		return nil
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       3,
		RoundsPerTask: 2,
		Timeout:       time.Second,
	})
	report := tester.RunT(t, job, job, job)
	if report.Completed != 6 {
		t.Fatalf("async report = %+v", report)
	}
}

func newTestServer(t *testing.T, catalogURL string) *resilienceweb.Server {
	t.Helper()

	server, err := resilienceweb.NewServer(resilienceweb.Options{CatalogURL: catalogURL})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func writeCatalogItem(w http.ResponseWriter, id string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"` + id + `","name":"Resilient Systems"}`))
}
