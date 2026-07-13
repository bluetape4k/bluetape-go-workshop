package orderhistory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestNewServiceValidatesConfiguration(t *testing.T) {
	t.Parallel()

	service, err := NewService(nil, Options{Author: "workshop"})
	if service != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewService(nil) = (%v, %v), want nil ErrInvalidConfig", service, err)
	}
	service, err = NewService(audit.NewMemoryRepository(), Options{})
	if service != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewService(empty author) = (%v, %v), want nil ErrInvalidConfig", service, err)
	}
	service, err = NewService(audit.NewMemoryRepository(), Options{Author: " workshop "})
	if err != nil || service == nil {
		t.Fatalf("NewService(valid) = (%v, %v), want service nil", service, err)
	}
}

func TestZeroValueServiceRejectsBeforeCommandValidation(t *testing.T) {
	t.Parallel()

	var service Service
	if _, err := service.Create(context.Background(), CreateCommand{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero-value Create() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := service.Confirm(context.Background(), TransitionCommand{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero-value Confirm() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := service.Ship(context.Background(), TransitionCommand{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero-value Ship() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := service.Cancel(context.Background(), CancelCommand{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("zero-value Cancel() error = %v, want ErrInvalidConfig", err)
	}
	var nilService *Service
	if _, err := nilService.Create(context.Background(), CreateCommand{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil Create() error = %v, want ErrInvalidConfig", err)
	}
}

func TestServiceDuplicateCommandPreservesAuditConflict(t *testing.T) {
	t.Parallel()

	service := newTestService(t, audit.NewMemoryRepository())
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"}); err != nil {
		t.Fatal(err)
	}
	_, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
	if !errors.Is(err, audit.ErrRevisionConflict) {
		t.Fatalf("duplicate Create() error = %v, want audit.ErrRevisionConflict", err)
	}
	var validationErr audit.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "event_id" {
		t.Fatalf("duplicate Create() error = %v, want audit.ValidationError field=event_id", err)
	}
	current, ok := service.Current("order-1")
	if !ok || current.Revision != 1 || current.Status != StatusPending {
		t.Fatalf("Current() = (%+v, %v), want unchanged revision 1", current, ok)
	}
}

func TestServiceAppendFailureDoesNotMutateCurrentState(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("repository unavailable")
	repo := &appendRepository{
		Repository: audit.NewMemoryRepository(),
		appendFn:   func(context.Context, ...audit.Entry) error { return wantErr },
	}
	service := newTestService(t, repo)
	_, err := service.Create(context.Background(), CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Create() error = %v, want %v", err, wantErr)
	}
	if _, ok := service.Current("order-1"); ok {
		t.Fatal("current state mutated after append failure")
	}
}

func TestServiceSuccessfulAppendIsCommitPoint(t *testing.T) {
	t.Parallel()

	base := audit.NewMemoryRepository()
	ctx, cancel := context.WithCancel(context.Background())
	repo := &appendRepository{Repository: base}
	repo.appendFn = func(ctx context.Context, entries ...audit.Entry) error {
		if err := base.Append(ctx, entries...); err != nil {
			return err
		}
		cancel()
		return nil
	}
	service := newTestService(t, repo)
	created, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
	if err != nil || created.Revision != 1 {
		t.Fatalf("Create() = (%+v, %v)", created, err)
	}
	current, ok := service.Current("order-1")
	if !ok || current.Revision != 1 {
		t.Fatalf("Current() = (%+v, %v), want committed", current, ok)
	}
}

func TestServiceRejectsInvalidTransitions(t *testing.T) {
	t.Parallel()

	service := newTestService(t, audit.NewMemoryRepository())
	ctx := context.Background()
	if _, err := service.Confirm(ctx, TransitionCommand{OrderID: "missing", CommandID: "cmd-1"}); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("Confirm(missing) error = %v", err)
	}
	if _, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Ship(ctx, TransitionCommand{OrderID: "order-1", CommandID: "cmd-2"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Ship(pending) error = %v", err)
	}
	if _, err := service.Cancel(ctx, CancelCommand{OrderID: "order-1", CommandID: "cmd-3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Confirm(ctx, TransitionCommand{OrderID: "order-1", CommandID: "cmd-4"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Confirm(cancelled) error = %v", err)
	}
}

func TestServiceConcurrentUnrelatedOrders(t *testing.T) {
	t.Parallel()

	service, err := NewService(audit.NewMemoryRepository(), Options{Author: "workshop"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if history, ok, err := service.History(ctx, "missing"); err != nil || ok || len(history.Entries()) != 0 {
		t.Fatalf("History(missing) = (%+v, %v, %v)", history, ok, err)
	}
	empty, err := service.Find(ctx, audit.Query{FromRevision: 99})
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("Find(empty) = (%+v, %v)", empty, err)
	}
	if _, err := service.Find(ctx, audit.Query{Limit: 101}); !errors.Is(err, audit.ErrInvalidQuery) {
		t.Fatalf("Find(limit) error = %v", err)
	}

	const count = 16
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := service.Create(ctx, CreateCommand{OrderID: fmt.Sprintf("order-%02d", i), CommandID: fmt.Sprintf("cmd-%02d", i)})
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent Create() error = %v", err)
		}
	}
	for i := 0; i < count; i++ {
		orderID := fmt.Sprintf("order-%02d", i)
		current, ok := service.Current(orderID)
		if !ok || current.Status != StatusPending || current.Revision != 1 {
			t.Fatalf("Current(%q) = (%+v, %v)", orderID, current, ok)
		}
		history, ok, err := service.History(ctx, orderID)
		if err != nil || !ok || history.HeadRevision() != 1 || len(history.Entries()) != 1 {
			t.Fatalf("History(%q) = (%+v, %v, %v)", orderID, history, ok, err)
		}
	}
}

func TestServiceFindRejectsCrossDomainAggregate(t *testing.T) {
	t.Parallel()

	service := newTestService(t, audit.NewMemoryRepository())
	foreign, err := audit.NewAggregateID("customer", "customer-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Find(context.Background(), audit.Query{Aggregate: &foreign})
	if !errors.Is(err, audit.ErrInvalidQuery) {
		t.Fatalf("Find(foreign aggregate) error = %v, want audit.ErrInvalidQuery", err)
	}
}

func TestServiceFindDefaultsToTwentyResults(t *testing.T) {
	t.Parallel()

	service, err := NewService(audit.NewMemoryRepository(), Options{Author: "workshop"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 25; i++ {
		if _, err := service.Create(ctx, CreateCommand{
			OrderID: fmt.Sprintf("order-%02d", i), CommandID: fmt.Sprintf("cmd-%02d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := service.Find(ctx, audit.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 20 || entries[0].Aggregate.ID != "order-00" || entries[19].Aggregate.ID != "order-19" {
		t.Fatalf("Find() returned %d entries [%q..%q]", len(entries), entries[0].Aggregate.ID, entries[len(entries)-1].Aggregate.ID)
	}
}

func TestServiceConcurrentTransitionHasOneWinner(t *testing.T) {
	t.Parallel()

	service, err := NewService(audit.NewMemoryRepository(), Options{Author: "workshop"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-create"}); err != nil {
		t.Fatal(err)
	}

	const count = 16
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := service.Confirm(ctx, TransitionCommand{OrderID: "order-1", CommandID: fmt.Sprintf("cmd-confirm-%02d", i)})
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	winners, rejected := 0, 0
	for err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrInvalidTransition):
			rejected++
		default:
			t.Fatalf("unexpected Confirm() error = %v", err)
		}
	}
	if winners != 1 || rejected != 15 {
		t.Fatalf("winners=%d rejected=%d, want 1/15", winners, rejected)
	}
	current, ok := service.Current("order-1")
	if !ok || current.Status != StatusConfirmed || current.Revision != 2 {
		t.Fatalf("Current() = (%+v, %v), want confirmed revision 2", current, ok)
	}
	history, ok, err := service.History(ctx, "order-1")
	if err != nil || !ok || history.HeadRevision() != 2 || len(history.Entries()) != 2 {
		t.Fatalf("History() = (%+v, %v, %v)", history, ok, err)
	}
}

func TestServiceCancellationBeforeAppendDoesNotMutate(t *testing.T) {
	t.Parallel()

	service := newTestService(t, audit.NewMemoryRepository())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Create(canceled) error = %v", err)
	}
	if _, ok := service.Current("order-1"); ok {
		t.Fatal("current state mutated for canceled command")
	}
}

func TestServiceCancellationDuringAppendDoesNotMutate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	repo := &appendRepository{Repository: audit.NewMemoryRepository()}
	repo.appendFn = func(ctx context.Context, _ ...audit.Entry) error {
		cancel()
		return ctx.Err()
	}
	service := newTestService(t, repo)
	_, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Create(cancel during append) error = %v", err)
	}
	if _, ok := service.Current("order-1"); ok {
		t.Fatal("current state mutated after canceled append")
	}
	if history, ok, err := service.History(context.Background(), "order-1"); err != nil || ok || len(history.Entries()) != 0 {
		t.Fatalf("History() = (%+v, %v, %v), want absent", history, ok, err)
	}
}

func TestServiceHistoryReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	service := newTestService(t, audit.NewMemoryRepository())
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"}); err != nil {
		t.Fatal(err)
	}
	history, ok, err := service.History(ctx, "order-1")
	if err != nil || !ok {
		t.Fatalf("History() = (%+v, %v, %v)", history, ok, err)
	}
	entries := history.Entries()
	entries[0].Event.Payload[0] = '['
	entries[0].Change.Attributes["status_after"] = "tampered"

	again, ok, err := service.History(ctx, "order-1")
	if err != nil || !ok {
		t.Fatalf("History() again = (%+v, %v, %v)", again, ok, err)
	}
	entry := again.Entries()[0]
	if entry.Event.Payload[0] != '{' || entry.Change.Attributes["status_after"] != "pending" {
		t.Fatalf("stored entry was mutated: %+v", entry)
	}
}

func TestServiceRejectsInvalidIdentifiersAndReason(t *testing.T) {
	t.Parallel()

	service := newTestService(t, audit.NewMemoryRepository())
	if _, err := service.Create(context.Background(), CreateCommand{OrderID: "bad/id", CommandID: "cmd-1"}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Create(invalid id) error = %v", err)
	}
	if _, err := service.Create(context.Background(), CreateCommand{OrderID: "order-1", CommandID: "cmd-1"}); err != nil {
		t.Fatal(err)
	}
	invalidUTF8 := string([]byte{0xff})
	if _, err := service.Cancel(context.Background(), CancelCommand{OrderID: "order-1", CommandID: "cmd-2", Reason: invalidUTF8}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Cancel(invalid UTF-8) error = %v", err)
	}
	if _, err := service.Cancel(context.Background(), CancelCommand{OrderID: "order-1", CommandID: "cmd-3", Reason: strings.Repeat("가", 257)}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Cancel(long reason) error = %v", err)
	}
}

type appendRepository struct {
	audit.Repository
	appendFn func(context.Context, ...audit.Entry) error
}

func (r *appendRepository) Append(ctx context.Context, entries ...audit.Entry) error {
	return r.appendFn(ctx, entries...)
}

func TestServiceLifecycleWritesAuditBeforeCurrentState(t *testing.T) {
	t.Parallel()

	repo := audit.NewMemoryRepository()
	service := newTestService(t, repo)
	ctx := context.Background()

	created, err := service.Create(ctx, CreateCommand{OrderID: " order-1 ", CommandID: "cmd-1"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.OrderID != "order-1" || created.Status != StatusPending || created.Revision != 1 {
		t.Fatalf("Create() = %+v", created)
	}
	confirmed, err := service.Confirm(ctx, TransitionCommand{OrderID: "order-1", CommandID: "cmd-2"})
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	shipped, err := service.Ship(ctx, TransitionCommand{OrderID: "order-1", CommandID: "cmd-3"})
	if err != nil {
		t.Fatalf("Ship() error = %v", err)
	}
	if confirmed.Status != StatusConfirmed || confirmed.Revision != 2 || shipped.Status != StatusShipped || shipped.Revision != 3 {
		t.Fatalf("unexpected transitions: confirmed=%+v shipped=%+v", confirmed, shipped)
	}

	history, ok, err := service.History(ctx, "order-1")
	if err != nil || !ok {
		t.Fatalf("History() = (%+v, %v, %v)", history, ok, err)
	}
	entries := history.Entries()
	if len(entries) != 3 {
		t.Fatalf("len(history.Entries()) = %d, want 3", len(entries))
	}
	wantTypes := []audit.EventType{"order.created", "order.confirmed", "order.shipped"}
	for i, entry := range entries {
		wantRevision := audit.Revision(i + 1)
		if entry.Event.EventType != wantTypes[i] || entry.Revision != wantRevision {
			t.Fatalf("entry[%d] = %+v", i, entry)
		}
		if entry.Event.EventID != audit.EventID("cmd-"+string(rune('1'+i))) || entry.Event.IdempotencyKey != "cmd-"+string(rune('1'+i)) {
			t.Fatalf("entry[%d] identity = (%q, %q)", i, entry.Event.EventID, entry.Event.IdempotencyKey)
		}
		if entry.Author != "workshop" || entry.Change == nil {
			t.Fatalf("entry[%d] metadata = %+v", i, entry)
		}
	}
}

func TestServiceCancelTransitions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		confirm bool
	}{
		{name: "pending"},
		{name: "confirmed", confirm: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := newTestService(t, audit.NewMemoryRepository())
			ctx := context.Background()
			if _, err := service.Create(ctx, CreateCommand{OrderID: "order-1", CommandID: "cmd-1"}); err != nil {
				t.Fatal(err)
			}
			if test.confirm {
				if _, err := service.Confirm(ctx, TransitionCommand{OrderID: "order-1", CommandID: "cmd-2"}); err != nil {
					t.Fatal(err)
				}
			}
			cancelled, err := service.Cancel(ctx, CancelCommand{OrderID: "order-1", CommandID: "cmd-cancel", Reason: " customer request "})
			if err != nil || cancelled.Status != StatusCancelled {
				t.Fatalf("Cancel() = (%+v, %v)", cancelled, err)
			}
		})
	}
}

func newTestService(t *testing.T, repo audit.Repository) *Service {
	t.Helper()
	times := []time.Time{
		time.Date(2026, 7, 13, 1, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 13, 1, 1, 0, 0, time.UTC),
		time.Date(2026, 7, 13, 1, 2, 0, 0, time.UTC),
		time.Date(2026, 7, 13, 1, 3, 0, 0, time.UTC),
	}
	next := 0
	service, err := NewService(repo, Options{
		Author: "workshop",
		Now: func() time.Time {
			value := times[next%len(times)]
			next++
			return value
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
