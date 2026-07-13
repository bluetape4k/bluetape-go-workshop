package auditquery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()
	if config.DefaultLimit != 20 || config.MaximumLimit != 100 {
		t.Fatalf("DefaultServiceConfig() = %+v", config)
	}
}

func TestService_SearchPaginatesInBothDirections(t *testing.T) {
	service := newTestService(t, "order-1", 4)

	first, err := service.Search(context.Background(), SearchRequest{
		Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRevisions(t, first.Entries, 1, 2)
	if !first.Page.HasMore || first.Page.Next == nil || first.Page.Next.FromRevision != 3 || first.Page.Next.ToRevision != 0 {
		t.Fatalf("first page = %+v", first.Page)
	}
	second, err := service.Search(context.Background(), SearchRequest{
		Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, FromRevision: first.Page.Next.FromRevision, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRevisions(t, second.Entries, 3, 4)
	if second.Page.HasMore || second.Page.Next != nil {
		t.Fatalf("second page = %+v", second.Page)
	}

	newest, err := service.Search(context.Background(), SearchRequest{
		Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, NewestFirst: true, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRevisions(t, newest.Entries, 4, 3)
	if !newest.Page.HasMore || newest.Page.Next == nil || newest.Page.Next.ToRevision != 2 || newest.Page.Next.FromRevision != 0 {
		t.Fatalf("newest page = %+v", newest.Page)
	}
	oldest, err := service.Search(context.Background(), SearchRequest{
		Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, ToRevision: newest.Page.Next.ToRevision, NewestFirst: true, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRevisions(t, oldest.Entries, 2, 1)
}

func TestService_SearchFiltersAndReturnsEmptySlice(t *testing.T) {
	service := newTestService(t, "order-1", 4)
	base := testBaseTime()
	response, err := service.Search(context.Background(), SearchRequest{
		Aggregate:      AggregateRequest{Type: "order", ID: "order-1"},
		FromRevision:   2,
		ToRevision:     4,
		FromRecordedAt: base.Add(2 * time.Minute),
		ToRecordedAt:   base.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRevisions(t, response.Entries, 3, 4)
	if response.Page.Limit != 20 {
		t.Fatalf("page limit = %d", response.Page.Limit)
	}

	empty, err := service.Search(context.Background(), SearchRequest{
		Aggregate: AggregateRequest{Type: "order", ID: "missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Entries == nil || len(empty.Entries) != 0 || empty.Page.HasMore {
		t.Fatalf("empty response = %+v", empty)
	}
}

func TestService_SearchRejectsInvalidQuery(t *testing.T) {
	service := newTestService(t, "order-1", 1)
	tests := []SearchRequest{
		{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, Limit: -1},
		{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, Limit: 101},
		{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, FromRevision: 3, ToRevision: 2},
		{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, FromRecordedAt: testBaseTime().Add(time.Hour), ToRecordedAt: testBaseTime()},
	}
	for _, request := range tests {
		_, err := service.Search(context.Background(), request)
		if !errors.Is(err, ErrInvalidRequest) || !errors.Is(err, audit.ErrInvalidQuery) {
			t.Fatalf("Search(%+v) err = %v", request, err)
		}
	}
}

func TestService_Get(t *testing.T) {
	service := newTestService(t, "order-1", 3)
	entry, err := service.Get(context.Background(), AggregateRequest{Type: "order", ID: "order-1"}, 2)
	if err != nil || entry.Revision != 2 {
		t.Fatalf("Get() entry = %+v, err = %v", entry, err)
	}
	_, err = service.Get(context.Background(), AggregateRequest{Type: "order", ID: "order-1"}, 4)
	if !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("Get() missing err = %v", err)
	}
	_, err = service.Get(context.Background(), AggregateRequest{Type: "order", ID: "order-1"}, 0)
	if !errors.Is(err, ErrInvalidRequest) || !errors.Is(err, audit.ErrInvalidRevision) {
		t.Fatalf("Get() zero revision err = %v", err)
	}
}

func TestService_PreservesReaderErrorsAndDoesNotRetry(t *testing.T) {
	opaque := errors.New("reader unavailable")
	validation := audit.ValidationError{Kind: audit.ErrInvalidQuery, Field: "limit", Value: 999}
	for _, want := range []error{context.Canceled, context.DeadlineExceeded, validation, opaque} {
		t.Run(want.Error(), func(t *testing.T) {
			var calls atomic.Int32
			reader := &findReader{find: func(context.Context, audit.Query) ([]audit.Entry, error) {
				calls.Add(1)
				return nil, want
			}}
			service, err := NewService(reader, DefaultServiceConfig())
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Search(context.Background(), SearchRequest{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}})
			if !errors.Is(err, want) || calls.Load() != 1 {
				t.Fatalf("Search() err = %v, calls = %d", err, calls.Load())
			}
			var gotValidation audit.ValidationError
			if errors.As(want, &gotValidation) && !errors.As(err, &gotValidation) {
				t.Fatalf("Search() lost ValidationError: %v", err)
			}
		})
	}
}

func TestService_ZeroValueAndCancellation(t *testing.T) {
	var zero Service
	if _, err := zero.Search(context.Background(), SearchRequest{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero Search() err = %v", err)
	}
	if _, err := zero.Get(context.Background(), AggregateRequest{}, 1); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero Get() err = %v", err)
	}

	service := newTestService(t, "order-1", 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Search(ctx, SearchRequest{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Search() err = %v", err)
	}
	if _, err := service.Get(ctx, AggregateRequest{Type: "order", ID: "order-1"}, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Get() err = %v", err)
	}
}

func TestService_ConcurrentReads(t *testing.T) {
	service := newTestService(t, "order-1", 4)
	const workers = 24
	start := make(chan struct{})
	var wait sync.WaitGroup
	var failures atomic.Int32
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			if index%2 == 0 {
				response, err := service.Search(context.Background(), SearchRequest{Aggregate: AggregateRequest{Type: "order", ID: "order-1"}, Limit: 2})
				if err != nil || len(response.Entries) != 2 {
					failures.Add(1)
				}
				return
			}
			entry, err := service.Get(context.Background(), AggregateRequest{Type: "order", ID: "order-1"}, 3)
			if err != nil || entry.Revision != 3 {
				failures.Add(1)
			}
		}(i)
	}
	close(start)
	wait.Wait()
	if failures.Load() != 0 {
		t.Fatalf("concurrent failures = %d", failures.Load())
	}
}

type findReader struct {
	audit.HistoryReader
	find func(context.Context, audit.Query) ([]audit.Entry, error)
}

func (r *findReader) Find(ctx context.Context, query audit.Query) ([]audit.Entry, error) {
	return r.find(ctx, query)
}

func newTestService(t *testing.T, aggregateID string, revisions int) *Service {
	t.Helper()
	repository := audit.NewMemoryRepository()
	entries := make([]audit.Entry, revisions)
	for index := range entries {
		entries[index] = mustTestEntry(t, aggregateID, audit.Revision(index+1))
	}
	if err := repository.Append(context.Background(), entries...); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repository, DefaultServiceConfig())
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func mustTestEntry(t *testing.T, aggregateID string, revision audit.Revision) audit.Entry {
	t.Helper()
	aggregate, err := audit.NewAggregateID("order", aggregateID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"revision": revision})
	if err != nil {
		t.Fatal(err)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        audit.EventID(fmt.Sprintf("%s-event-%d", aggregateID, revision)),
		EventType:      audit.EventType("order.updated"),
		AggregateID:    aggregate,
		Revision:       revision,
		OccurredAt:     testBaseTime().Add(time.Duration(revision-1) * time.Minute),
		RecordedAt:     testBaseTime().Add(time.Duration(revision-1) * time.Minute),
		IdempotencyKey: fmt.Sprintf("%s-command-%d", aggregateID, revision),
		Payload:        payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: "test", Event: event})
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func testBaseTime() time.Time {
	return time.Date(2026, time.July, 13, 0, 0, 0, 0, time.UTC)
}

func assertRevisions(t *testing.T, entries []audit.Entry, revisions ...audit.Revision) {
	t.Helper()
	if len(entries) != len(revisions) {
		t.Fatalf("entry count = %d, want %d", len(entries), len(revisions))
	}
	for index, revision := range revisions {
		if entries[index].Revision != revision {
			t.Fatalf("entry[%d].Revision = %d, want %d", index, entries[index].Revision, revision)
		}
	}
}

func TestNewServiceRejectsInvalidConfig(t *testing.T) {
	repository := audit.NewMemoryRepository()
	tests := []struct {
		name   string
		reader audit.HistoryReader
		config ServiceConfig
	}{
		{name: "nil reader", reader: nil, config: DefaultServiceConfig()},
		{name: "zero default", reader: repository, config: ServiceConfig{MaximumLimit: 100}},
		{name: "negative default", reader: repository, config: ServiceConfig{DefaultLimit: -1, MaximumLimit: 100}},
		{name: "zero maximum", reader: repository, config: ServiceConfig{DefaultLimit: 20}},
		{name: "default above maximum", reader: repository, config: ServiceConfig{DefaultLimit: 21, MaximumLimit: 20}},
		{name: "maximum above hard bound", reader: repository, config: ServiceConfig{DefaultLimit: 20, MaximumLimit: 101}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(test.reader, test.config)
			if service != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewService() service = %v, err = %v", service, err)
			}
		})
	}
}

func TestNewServiceRejectsTypedNilReader(t *testing.T) {
	var repository *audit.MemoryRepository
	service, err := NewService(repository, DefaultServiceConfig())
	if service != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewService() service = %v, err = %v", service, err)
	}
}

func TestNormalizeAggregate(t *testing.T) {
	tests := []struct {
		name    string
		request AggregateRequest
		want    audit.AggregateID
		wantErr error
	}{
		{name: "valid", request: AggregateRequest{Type: " order ", ID: " order-1 "}, want: audit.AggregateID{Type: "order", ID: "order-1"}},
		{name: "blank type", request: AggregateRequest{ID: "order-1"}, wantErr: ErrInvalidRequest},
		{name: "blank id", request: AggregateRequest{Type: "order"}, wantErr: ErrInvalidRequest},
		{name: "path separator", request: AggregateRequest{Type: "order", ID: "tenant/order-1"}, wantErr: ErrInvalidRequest},
		{name: "percent escape", request: AggregateRequest{Type: "order", ID: "order%2F1"}, wantErr: ErrInvalidRequest},
		{name: "too long", request: AggregateRequest{Type: "order", ID: "a" + string(make([]byte, 128))}, wantErr: ErrInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeAggregate(test.request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("normalizeAggregate() err = %v, want %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("normalizeAggregate() = %+v, want %+v", got, test.want)
			}
		})
	}
}
