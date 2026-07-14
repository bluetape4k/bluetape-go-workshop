package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/sqloutboxtest"
)

func TestRelayLifecyclePostgreSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openWorkflowPostgres(ctx, t)
	now := normalizeTimestamp(testWorkflowNow)
	store := newClockedWorkflowOutbox(t, func() time.Time { return now })
	history, err := NewHistoryStore(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, history, store, Config{Author: "workshop", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateSchema(ctx, db, store); err != nil {
		t.Fatal(err)
	}

	t.Run("retries after exactly 250ms with stable identity", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		command := CreateCommand{OrderID: "order-retry", CommandID: "cmd-retry"}
		if _, _, err := service.Create(ctx, command); err != nil {
			t.Fatal(err)
		}
		publisher := sqloutboxtest.NewRecordingPublisher(sqloutboxtest.WithFailures(
			map[audit.EventID]int{audit.EventID(command.CommandID): 1}, errors.New("temporary"),
		))
		relay := newWorkflowRelay(t, store, publisher, func() time.Time { return now })
		first, err := relay.RunOnce(ctx, db)
		if err != nil || first != (sqloutbox.RelayResult{Claimed: 1, Failed: 1}) {
			t.Fatalf("first RunOnce() = (%+v, %v)", first, err)
		}
		before, err := relay.RunOnce(ctx, db)
		if err != nil || before != (sqloutbox.RelayResult{}) {
			t.Fatalf("early RunOnce() = (%+v, %v)", before, err)
		}
		now = now.Add(250 * time.Millisecond)
		second, err := relay.RunOnce(ctx, db)
		if err != nil || second != (sqloutbox.RelayResult{Claimed: 1, Published: 1}) {
			t.Fatalf("second RunOnce() = (%+v, %v)", second, err)
		}
		records := publisher.Records()
		if len(records) != 2 || records[0].EventID != records[1].EventID ||
			records[0].IdempotencyKey != records[1].IdempotencyKey || records[0].Attempts != 1 || records[1].Attempts != 2 {
			t.Fatalf("publish attempts = %#v", records)
		}
	})

	t.Run("third failure dead letters", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		command := CreateCommand{OrderID: "order-dead", CommandID: "cmd-dead"}
		if _, _, err := service.Create(ctx, command); err != nil {
			t.Fatal(err)
		}
		publisher := sqloutboxtest.NewRecordingPublisher(sqloutboxtest.WithFailures(
			map[audit.EventID]int{audit.EventID(command.CommandID): 3}, errors.New("poison"),
		))
		relay := newWorkflowRelay(t, store, publisher, func() time.Time { return now })
		for attempt := 1; attempt <= 3; attempt++ {
			result, err := relay.RunOnce(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			if attempt < 3 {
				if result.Failed != 1 {
					t.Fatalf("attempt %d = %+v", attempt, result)
				}
				now = now.Add(250 * time.Millisecond)
			} else if result.DeadLettered != 1 {
				t.Fatalf("attempt 3 = %+v", result)
			}
		}
		assertWorkflowOutboxState(ctx, t, db, sqloutbox.StatusDeadLetter, 3)
	})

	t.Run("canceled publish is reclaimed after lease", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		command := CreateCommand{OrderID: "order-lease", CommandID: "cmd-lease"}
		if _, _, err := service.Create(ctx, command); err != nil {
			t.Fatal(err)
		}
		publishCtx, stop := context.WithCancel(ctx)
		publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, _ sqloutbox.Record) error {
			stop()
			return ctx.Err()
		})
		relay := newWorkflowRelay(t, store, publisher, func() time.Time { return now })
		result, err := relay.RunOnce(publishCtx, db)
		if !errors.Is(err, context.Canceled) || result.Claimed != 1 {
			t.Fatalf("canceled RunOnce() = (%+v, %v)", result, err)
		}
		assertWorkflowOutboxState(ctx, t, db, sqloutbox.StatusClaimed, 1)

		now = now.Add(31 * time.Second)
		recorder := sqloutboxtest.NewRecordingPublisher()
		recovery := newWorkflowRelay(t, store, recorder, func() time.Time { return now })
		recovered, err := recovery.RunOnce(ctx, db)
		if err != nil || recovered.Published != 1 || recorder.Count() != 1 {
			t.Fatalf("recovery RunOnce() = (%+v, %v), records=%d", recovered, err, recorder.Count())
		}
		if records := recorder.Records(); records[0].EventID != audit.EventID(command.CommandID) || records[0].IdempotencyKey != command.CommandID {
			t.Fatalf("recovered identity = %#v", records[0])
		}
	})

	t.Run("continuous run joins after expected cancellation", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		if _, _, err := service.Create(ctx, CreateCommand{OrderID: "order-run", CommandID: "cmd-run"}); err != nil {
			t.Fatal(err)
		}
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
		relay := newWorkflowRelay(t, store, publisher, func() time.Time { return now })
		done := make(chan error, 1)
		go func() { done <- relay.Run(runCtx, db) }()
		select {
		case <-started:
			stop()
		case <-time.After(5 * time.Second):
			stop()
			t.Fatal("relay did not publish")
		}
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Relay.Run() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("relay did not join")
		}
	})
}

func newClockedWorkflowOutbox(t *testing.T, now func() time.Time) *sqloutbox.Store {
	t.Helper()
	store, err := sqloutbox.NewStore(sqloutbox.Options{Table: outboxTable, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func newWorkflowRelay(t *testing.T, store *sqloutbox.Store, publisher sqloutbox.Publisher, now func() time.Time) *sqloutbox.Relay {
	t.Helper()
	relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
		ClaimLimit: 16, MaxAttempts: 3, RetryDelay: 250 * time.Millisecond, IdleDelay: 50 * time.Millisecond, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return relay
}

func assertWorkflowOutboxState(ctx context.Context, t *testing.T, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, want sqloutbox.Status, attempts int) {
	t.Helper()
	var gotStatus sqloutbox.Status
	var gotAttempts int
	if err := db.QueryRowContext(ctx, `select status, attempts from audited_order_workflow_outbox_records`).Scan(&gotStatus, &gotAttempts); err != nil {
		t.Fatal(err)
	}
	if gotStatus != want || gotAttempts != attempts {
		t.Fatalf("outbox state = (%s, %d), want (%s, %d)", gotStatus, gotAttempts, want, attempts)
	}
}
