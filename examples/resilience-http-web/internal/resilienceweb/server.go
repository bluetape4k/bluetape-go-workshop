package resilienceweb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server exposes an HTTP resilience workshop API.
type Server struct {
	router chi.Router

	client  *http.Client
	catalog string

	events *eventLog

	breaker  *resilience.CircuitBreakerPolicy[*http.Response]
	bulkhead *resilience.BulkheadPolicy[struct{}]
}

// Options configures the resilience workshop server.
type Options struct {
	CatalogURL string
	Client     *http.Client
}

// NewServer creates a server that protects outbound and inbound HTTP paths.
func NewServer(options Options) (*Server, error) {
	catalogURL, err := normalizeCatalogURL(options.CatalogURL)
	if err != nil {
		return nil, err
	}

	events := &eventLog{}
	onEvent := events.append

	retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{ //nolint:bodyclose
		Name:        "catalog-retry",
		MaxAttempts: 3,
		Backoff:     resilience.NoBackoff(),
		OnEvent:     onEvent,
	})
	if err != nil {
		return nil, err
	}
	timeout, err := resilience.NewTimeout[*http.Response](resilience.TimeoutOptions{ //nolint:bodyclose
		Name:    "catalog-timeout",
		Timeout: 500 * time.Millisecond,
		OnEvent: onEvent,
	})
	if err != nil {
		return nil, err
	}
	breaker, err := resilience.NewCircuitBreaker[*http.Response](resilience.CircuitBreakerOptions{ //nolint:bodyclose
		Name:                  "catalog-breaker",
		FailureThreshold:      2,
		SuccessThreshold:      1,
		OpenTimeout:           100 * time.Millisecond,
		HalfOpenMaxConcurrent: 1,
		OnEvent:               onEvent,
	})
	if err != nil {
		return nil, err
	}
	bulkhead, err := resilience.NewBulkhead[struct{}](resilience.BulkheadOptions{
		Name:          "order-handler",
		MaxConcurrent: 1,
		Wait:          false,
		OnEvent:       onEvent,
	})
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport
	if options.Client != nil && options.Client.Transport != nil {
		transport = options.Client.Transport
	}
	client := &http.Client{
		Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
			Transport:       transport,
			Policies:        []resilience.Policy[*http.Response]{retry, timeout, breaker},
			RetryableStatus: resilience.RetryableServerError,
		}),
		Timeout: 2 * time.Second,
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.CleanPath)

	server := &Server{
		router:   router,
		client:   client,
		catalog:  catalogURL,
		events:   events,
		breaker:  breaker,
		bulkhead: bulkhead,
	}

	router.Get("/healthz", server.health)
	router.Get("/catalog/{id}", server.catalogItem)
	router.Post("/orders", resilience.NewHandler(http.HandlerFunc(server.createOrder), resilience.HandlerOptions{
		Policies: []resilience.Policy[struct{}]{bulkhead},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			if errors.Is(err, resilience.ErrBulkheadRejected) {
				writeError(w, http.StatusTooManyRequests, err)
				return
			}
			writeError(w, http.StatusServiceUnavailable, err)
		},
	}).ServeHTTP)
	router.Get("/events", server.eventsSnapshot)

	return server, nil
}

// ServeHTTP dispatches requests to the workshop API.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) catalogItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.catalog+"/items/"+url.PathEscape(itemID), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	response, err := s.client.Do(request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, resilience.ErrCircuitOpen) {
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, err)
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	delay := parseDelay(r.URL.Query().Get("delay"), 25*time.Millisecond)

	select {
	case <-r.Context().Done():
		writeError(w, http.StatusRequestTimeout, r.Context().Err())
	case <-time.After(delay):
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":           "accepted",
			"handler_inflight": s.bulkhead.InFlight(),
		})
	}
}

func (s *Server) eventsSnapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.events.snapshot())
}

func normalizeCatalogURL(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("catalog url must not be empty")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("catalog url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("catalog url must be absolute")
	}
	return strings.TrimRight(value, "/"), nil
}

func parseDelay(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return fallback
	}
	return delay
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

type eventLog struct {
	mu     sync.Mutex
	nextID atomic.Uint64
	events []eventRecord
}

type eventRecord struct {
	ID            uint64                   `json:"id"`
	PolicyName    string                   `json:"policy_name"`
	PolicyType    string                   `json:"policy_type"`
	Kind          resilience.EventKind     `json:"kind"`
	Category      resilience.EventCategory `json:"category"`
	Attempt       int                      `json:"attempt,omitempty"`
	ErrorCategory resilience.ErrorCategory `json:"error_category,omitempty"`
	State         resilience.CircuitState  `json:"state,omitempty"`
	PreviousState resilience.CircuitState  `json:"previous_state,omitempty"`
	InFlight      int                      `json:"in_flight,omitempty"`
}

func (l *eventLog) append(_ context.Context, event resilience.Event) {
	record := eventRecord{
		ID:            l.nextID.Add(1),
		PolicyName:    event.PolicyName,
		PolicyType:    event.PolicyType,
		Kind:          event.Kind,
		Category:      event.Category,
		Attempt:       event.Attempt,
		ErrorCategory: event.ErrorCategory,
		State:         event.State,
		PreviousState: event.PreviousState,
		InFlight:      event.InFlight,
	}

	l.mu.Lock()
	l.events = append(l.events, record)
	l.mu.Unlock()
}

func (l *eventLog) snapshot() []eventRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]eventRecord(nil), l.events...)
}
