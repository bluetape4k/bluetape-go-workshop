package orderoutbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/sqloutboxtest"
)

func TestRelayLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openOrderoutboxPostgres(ctx, t)

	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := newClockedStore(t, func() time.Time { return now })
	service := newClockedService(t, store, func() time.Time { return now })
	if err := service.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}

	t.Run("retry preserves identity and eligibility", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		command := relayCommand("retry")
		placeRelayOrder(ctx, t, service, db, command)
		publisher := sqloutboxtest.NewRecordingPublisher(sqloutboxtest.WithFailures(
			map[audit.EventID]int{audit.EventID(command.CommandID): 1},
			errors.New("temporary sink failure"),
		))
		relay := newTestRelay(t, store, publisher, 1, func() time.Time { return now })

		first, err := relay.RunOnce(ctx, db)
		if err != nil {
			t.Fatalf("first RunOnce() error = %v", err)
		}
		assertRelayResult(t, first, sqloutbox.RelayResult{Claimed: 1, Failed: 1})
		assertOutboxState(ctx, t, db, sqloutbox.StatusPending, 1)

		beforeRetry, err := relay.RunOnce(ctx, db)
		if err != nil {
			t.Fatalf("pre-retry RunOnce() error = %v", err)
		}
		assertRelayResult(t, beforeRetry, sqloutbox.RelayResult{})

		now = now.Add(250 * time.Millisecond)
		second, err := relay.RunOnce(ctx, db)
		if err != nil {
			t.Fatalf("second RunOnce() error = %v", err)
		}
		assertRelayResult(t, second, sqloutbox.RelayResult{Claimed: 1, Published: 1})
		assertOutboxState(ctx, t, db, sqloutbox.StatusPublished, 2)

		records := publisher.Records()
		if len(records) != 2 {
			t.Fatalf("publish attempts = %d, want 2", len(records))
		}
		if records[0].EventID != records[1].EventID || records[0].IdempotencyKey != records[1].IdempotencyKey {
			t.Fatalf("attempt identities differ: (%q, %q) then (%q, %q)",
				records[0].EventID, records[0].IdempotencyKey,
				records[1].EventID, records[1].IdempotencyKey)
		}
		if records[0].Attempts != 1 || records[1].Attempts != 2 {
			t.Fatalf("attempt counters = (%d, %d), want (1, 2)", records[0].Attempts, records[1].Attempts)
		}
	})

	t.Run("third failure dead letters", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		command := relayCommand("dead-letter")
		placeRelayOrder(ctx, t, service, db, command)
		publisher := sqloutboxtest.NewRecordingPublisher(sqloutboxtest.WithFailures(
			map[audit.EventID]int{audit.EventID(command.CommandID): 3},
			errors.New("poison message"),
		))
		relay := newTestRelay(t, store, publisher, 1, func() time.Time { return now })

		for attempt := 1; attempt <= 3; attempt++ {
			result, err := relay.RunOnce(ctx, db)
			if err != nil {
				t.Fatalf("RunOnce() attempt %d error = %v", attempt, err)
			}
			want := sqloutbox.RelayResult{Claimed: 1, Failed: 1}
			if attempt == 3 {
				want = sqloutbox.RelayResult{Claimed: 1, DeadLettered: 1}
			}
			assertRelayResult(t, result, want)
			if attempt < 3 {
				now = now.Add(250 * time.Millisecond)
			}
		}
		assertOutboxState(ctx, t, db, sqloutbox.StatusDeadLetter, 3)
		if publisher.Count() != 3 {
			t.Fatalf("publish attempts = %d, want 3", publisher.Count())
		}
	})

	t.Run("caller cancellation leaves claim for lease recovery", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		placeRelayOrder(ctx, t, service, db, relayCommand("cancel-once"))
		publishCtx, stop := context.WithCancel(ctx)
		calls := 0
		publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, _ sqloutbox.Record) error {
			calls++
			stop()
			return ctx.Err()
		})
		relay := newTestRelay(t, store, publisher, 1, func() time.Time { return now })

		result, err := relay.RunOnce(publishCtx, db)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunOnce() error = %v, want context.Canceled", err)
		}
		assertRelayResult(t, result, sqloutbox.RelayResult{Claimed: 1})
		if calls != 1 {
			t.Fatalf("publish calls = %d, want 1", calls)
		}
		assertOutboxState(ctx, t, db, sqloutbox.StatusClaimed, 1)
	})

	t.Run("continuous run joins after cancellation", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		placeRelayOrder(ctx, t, service, db, relayCommand("continuous"))
		runCtx, stop := context.WithCancel(ctx)
		started := make(chan struct{})
		var calls atomic.Int32
		publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, _ sqloutbox.Record) error {
			if calls.Add(1) == 1 {
				close(started)
			}
			<-ctx.Done()
			return ctx.Err()
		})
		relay := newTestRelay(t, store, publisher, 1, func() time.Time { return now })
		done := make(chan error, 1)
		go func() { done <- relay.Run(runCtx, db) }()

		select {
		case <-started:
			stop()
		case <-time.After(5 * time.Second):
			stop()
			t.Fatal("publisher did not start")
		}
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Run() error = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("relay did not stop after cancellation")
		}
		if calls.Load() != 1 {
			t.Fatalf("publish calls = %d, want 1", calls.Load())
		}
		assertOutboxState(ctx, t, db, sqloutbox.StatusClaimed, 1)
	})
}

func TestRelayConcurrentRunOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openOrderoutboxPostgres(ctx, t)
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	store := newClockedStore(t, func() time.Time { return now })
	service := newClockedService(t, store, func() time.Time { return now })
	if err := service.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}
	for index := 0; index < 12; index++ {
		placeRelayOrder(ctx, t, service, db, relayCommand(fmt.Sprintf("concurrent-%02d", index)))
	}

	publisher := sqloutboxtest.NewRecordingPublisher()
	relay := newTestRelay(t, store, publisher, 3, func() time.Time { return now })
	start := make(chan struct{})
	results := make(chan sqloutbox.RelayResult, 4)
	errorsCh := make(chan error, 4)
	var workers sync.WaitGroup
	workers.Add(4)
	for range 4 {
		go func() {
			defer workers.Done()
			<-start
			result, err := relay.RunOnce(ctx, db)
			results <- result
			errorsCh <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
	}
	var total sqloutbox.RelayResult
	for result := range results {
		total.Claimed += result.Claimed
		total.Published += result.Published
		total.Failed += result.Failed
		total.DeadLettered += result.DeadLettered
	}
	assertRelayResult(t, total, sqloutbox.RelayResult{Claimed: 12, Published: 12})
	ids := publisher.EventIDs()
	if len(ids) != 12 {
		t.Fatalf("published IDs = %d, want 12", len(ids))
	}
	unique := make(map[audit.EventID]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	if len(unique) != 12 {
		t.Fatalf("unique published IDs = %d, want 12", len(unique))
	}
	assertStatusCount(ctx, t, db, sqloutbox.StatusPublished, 12)
}

func newClockedStore(t *testing.T, now func() time.Time) *sqloutbox.Store {
	t.Helper()
	store, err := sqloutbox.NewStore(sqloutbox.Options{Table: outboxTable, Now: now})
	if err != nil {
		t.Fatalf("sqloutbox.NewStore() error = %v", err)
	}
	return store
}

func newClockedService(t *testing.T, store *sqloutbox.Store, now func() time.Time) *Service {
	t.Helper()
	service, err := NewService(store, Config{Author: "workshop", Now: now})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func newTestRelay(t *testing.T, store *sqloutbox.Store, publisher sqloutbox.Publisher, claimLimit int, now func() time.Time) *sqloutbox.Relay {
	t.Helper()
	relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
		ClaimLimit: claimLimit, MaxAttempts: 3,
		RetryDelay: 250 * time.Millisecond, IdleDelay: 50 * time.Millisecond,
		Now: now,
	})
	if err != nil {
		t.Fatalf("sqloutbox.NewRelay() error = %v", err)
	}
	return relay
}

func relayCommand(suffix string) PlaceOrderCommand {
	command := validPlaceOrderCommand()
	command.OrderID = "order-" + suffix
	command.CommandID = "command-" + suffix
	return command
}

func placeRelayOrder(ctx context.Context, t *testing.T, service *Service, db *sql.DB, command PlaceOrderCommand) {
	t.Helper()
	if _, err := service.Place(ctx, db, command); err != nil {
		t.Fatalf("Place(%q) error = %v", command.OrderID, err)
	}
}

func assertRelayResult(t *testing.T, got, want sqloutbox.RelayResult) {
	t.Helper()
	if got != want {
		t.Fatalf("RelayResult = %#v, want %#v", got, want)
	}
}

func assertOutboxState(ctx context.Context, t *testing.T, db *sql.DB, wantStatus sqloutbox.Status, wantAttempts int) {
	t.Helper()
	var status sqloutbox.Status
	var attempts int
	if err := db.QueryRowContext(ctx, `select status, attempts from transactional_outbox_records`).Scan(&status, &attempts); err != nil {
		t.Fatalf("read outbox state: %v", err)
	}
	if status != wantStatus || attempts != wantAttempts {
		t.Fatalf("outbox state = (%s, %d), want (%s, %d)", status, attempts, wantStatus, wantAttempts)
	}
}

func assertStatusCount(ctx context.Context, t *testing.T, db *sql.DB, status sqloutbox.Status, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, `select count(*) from transactional_outbox_records where status = $1`, status).Scan(&got); err != nil {
		t.Fatalf("count status %s: %v", status, err)
	}
	if got != want {
		t.Fatalf("status %s rows = %d, want %d", status, got, want)
	}
}
