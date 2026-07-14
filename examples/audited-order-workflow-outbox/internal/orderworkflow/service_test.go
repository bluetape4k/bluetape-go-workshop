package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestNormalizeIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "trim", value: " order-1001 ", want: "order-1001"},
		{name: "allowed separators", value: "order:west.1_test", want: "order:west.1_test"},
		{name: "blank", value: " ", wantErr: true},
		{name: "leading separator", value: "-order", wantErr: true},
		{name: "newline", value: "order\nforged", wantErr: true},
		{name: "non ascii", value: "주문", wantErr: true},
		{name: "oversized", value: strings.Repeat("a", 129), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeIdentifier(tt.value)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCommand) {
					t.Fatalf("normalizeIdentifier(%q) error = %v", tt.value, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("normalizeIdentifier(%q) = (%q, %v), want (%q, nil)", tt.value, got, err, tt.want)
			}
		})
	}
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata audit.Metadata
		wantErr  bool
	}{
		{name: "nil"},
		{name: "valid", metadata: audit.Metadata{"channel": "workshop"}},
		{name: "blank key", metadata: audit.Metadata{" ": "value"}, wantErr: true},
		{name: "oversized key", metadata: audit.Metadata{strings.Repeat("k", 65): "value"}, wantErr: true},
		{name: "oversized value", metadata: audit.Metadata{"key": strings.Repeat("v", 513)}, wantErr: true},
		{name: "too many", metadata: metadataWithEntries(33), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateMetadata(tt.metadata)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCommand) {
					t.Fatalf("validateMetadata() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateMetadata() error = %v", err)
			}
			if len(got) != len(tt.metadata) {
				t.Fatalf("validateMetadata() length = %d, want %d", len(got), len(tt.metadata))
			}
		})
	}
}

func TestValidateCommands(t *testing.T) {
	create, err := (CreateCommand{
		OrderID:   " order-1001 ",
		CommandID: " cmd-create-1001 ",
		Metadata:  audit.Metadata{"channel": "workshop"},
	}).validate()
	if err != nil {
		t.Fatalf("CreateCommand.validate() error = %v", err)
	}
	if create.OrderID != "order-1001" || create.CommandID != "cmd-create-1001" {
		t.Fatalf("CreateCommand.validate() = %#v", create)
	}

	transition, err := (TransitionCommand{
		OrderID:   "order-1001",
		CommandID: "cmd-cancel-1001",
		Action:    ActionCancel,
		Reason:    " customer request ",
	}).validate()
	if err != nil {
		t.Fatalf("TransitionCommand.validate() error = %v", err)
	}
	if transition.Reason != "customer request" {
		t.Fatalf("TransitionCommand.validate() reason = %q", transition.Reason)
	}

	invalid := []TransitionCommand{
		{OrderID: "order-1001", CommandID: "cmd-1", Action: "ship"},
		{OrderID: "order-1001", CommandID: "cmd-1", Action: ActionConfirm, Reason: "not allowed"},
		{OrderID: "order-1001", CommandID: "cmd-1", Action: ActionCancel, Reason: strings.Repeat("가", 501)},
	}
	for _, command := range invalid {
		if _, err := command.validate(); !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("TransitionCommand.validate(%#v) error = %v", command, err)
		}
	}
}

func TestNormalizeTimestampUsesUTCPostgreSQLPrecision(t *testing.T) {
	value := time.Date(2026, 7, 14, 12, 0, 0, 123456789, time.FixedZone("KST", 9*60*60))
	got := normalizeTimestamp(value)
	want := time.Date(2026, 7, 14, 3, 0, 0, 123456000, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("normalizeTimestamp() = %v, want %v", got, want)
	}
}

func TestBuildEntryPreservesCanonicalIntentAndOrder(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 0, 0, 123456000, time.UTC)
	command := TransitionCommand{
		OrderID:   "order-1001",
		CommandID: "cmd-confirm-1001",
		Action:    ActionConfirm,
		Metadata:  audit.Metadata{"operator": "demo"},
	}
	order := Order{OrderID: command.OrderID, Status: StatusConfirmed, Revision: 2, UpdatedAt: now}

	entry, err := buildTransitionEntry("workshop", now, command, order)
	if err != nil {
		t.Fatalf("buildTransitionEntry() error = %v", err)
	}
	if entry.Aggregate != (audit.AggregateID{Type: orderAggregateType, ID: command.OrderID}) {
		t.Fatalf("entry aggregate = %#v", entry.Aggregate)
	}
	if entry.Event.EventID != audit.EventID(command.CommandID) || entry.Event.IdempotencyKey != command.CommandID {
		t.Fatalf("entry identity = (%q, %q)", entry.Event.EventID, entry.Event.IdempotencyKey)
	}
	if entry.Event.EventType != audit.EventType("order.confirmed") || entry.Revision != 2 {
		t.Fatalf("entry type/revision = (%q, %d)", entry.Event.EventType, entry.Revision)
	}

	payload, err := decodeEventPayload(entry)
	if err != nil {
		t.Fatalf("decodeEventPayload() error = %v", err)
	}
	if !payload.Intent.matchesTransition(command) || payload.Order != order {
		t.Fatalf("payload = %#v", payload)
	}

	command.Metadata["operator"] = "mutated"
	if entry.Event.Metadata["operator"] != "demo" {
		t.Fatalf("entry metadata changed with caller map: %#v", entry.Event.Metadata)
	}
}

func metadataWithEntries(count int) audit.Metadata {
	metadata := make(audit.Metadata, count)
	for index := range count {
		metadata["key"+string(rune('A'+index))] = "value"
	}
	return metadata
}

func TestNewServiceRejectsInvalidConfiguration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openWorkflowPostgres(ctx, t)
	history, err := NewHistoryStore(db)
	if err != nil {
		t.Fatal(err)
	}
	outbox := newWorkflowOutbox(t)
	tests := []struct {
		name    string
		db      *sql.DB
		history *HistoryStore
		outbox  *sqloutbox.Store
		config  Config
	}{
		{name: "nil database", history: history, outbox: outbox, config: Config{Author: "workshop"}},
		{name: "nil history", db: db, outbox: outbox, config: Config{Author: "workshop"}},
		{name: "nil outbox", db: db, history: history, config: Config{Author: "workshop"}},
		{name: "blank author", db: db, history: history, outbox: outbox},
		{name: "oversized author", db: db, history: history, outbox: outbox, config: Config{Author: strings.Repeat("a", 129)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(tt.db, tt.history, tt.outbox, tt.config)
			if service != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewService() = (%v, %v)", service, err)
			}
		})
	}
}

func TestServicePostgreSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openWorkflowPostgres(ctx, t)
	history, outbox, service := newPostgresWorkflowService(ctx, t, db)

	t.Run("creates confirms and replays original projections", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		create := CreateCommand{OrderID: "order-1001", CommandID: "cmd-create-1001", Metadata: audit.Metadata{"channel": "workshop"}}
		created, replayed, err := service.Create(ctx, create)
		if err != nil || replayed || created.Status != StatusPending || created.Revision != 1 {
			t.Fatalf("Create() = (%#v, %v, %v)", created, replayed, err)
		}
		confirm := TransitionCommand{OrderID: create.OrderID, CommandID: "cmd-confirm-1001", Action: ActionConfirm}
		confirmed, replayed, err := service.Transition(ctx, confirm)
		if err != nil || replayed || confirmed.Status != StatusConfirmed || confirmed.Revision != 2 {
			t.Fatalf("Transition() = (%#v, %v, %v)", confirmed, replayed, err)
		}
		replayedOrder, replayed, err := service.Transition(ctx, confirm)
		if err != nil || !replayed || replayedOrder != confirmed {
			t.Fatalf("Transition(replay) = (%#v, %v, %v)", replayedOrder, replayed, err)
		}
		assertWorkflowTableCount(ctx, t, db, historyTable, 2)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 2)
	})

	t.Run("cancels pending or confirmed and rejects terminal transitions", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		_, _, err := service.Create(ctx, CreateCommand{OrderID: "order-cancel", CommandID: "cmd-create-cancel"})
		if err != nil {
			t.Fatal(err)
		}
		cancelled, replayed, err := service.Transition(ctx, TransitionCommand{
			OrderID: "order-cancel", CommandID: "cmd-cancel", Action: ActionCancel, Reason: "customer request",
		})
		if err != nil || replayed || cancelled.Status != StatusCancelled || cancelled.Revision != 2 {
			t.Fatalf("cancel Transition() = (%#v, %v, %v)", cancelled, replayed, err)
		}
		_, _, err = service.Transition(ctx, TransitionCommand{OrderID: "order-cancel", CommandID: "cmd-confirm-late", Action: ActionConfirm})
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("terminal Transition() error = %v", err)
		}
	})

	t.Run("rolls back history conflict after order write", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		seedCommand := CreateCommand{OrderID: "order-history-conflict", CommandID: "seed-history-conflict"}
		seedOrder := Order{OrderID: seedCommand.OrderID, Status: StatusPending, Revision: 1, UpdatedAt: testWorkflowNow}
		seed, err := buildCreateEntry("workshop", testWorkflowNow, seedCommand, seedOrder)
		if err != nil {
			t.Fatal(err)
		}
		if err := history.Insert(ctx, db, seed); err != nil {
			t.Fatal(err)
		}
		_, _, err = service.Create(ctx, CreateCommand{OrderID: seedCommand.OrderID, CommandID: "new-command"})
		if err == nil {
			t.Fatal("Create() error = nil")
		}
		assertWorkflowTableCount(ctx, t, db, ordersTable, 0)
		assertWorkflowTableCount(ctx, t, db, historyTable, 1)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 0)
	})

	t.Run("rolls back outbox conflict after history write", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		command := CreateCommand{OrderID: "order-outbox-conflict", CommandID: "shared-command"}
		seedCommand := CreateCommand{OrderID: "seed-order", CommandID: command.CommandID}
		seedOrder := Order{OrderID: seedCommand.OrderID, Status: StatusPending, Revision: 1, UpdatedAt: testWorkflowNow}
		seed, err := buildCreateEntry("workshop", testWorkflowNow, seedCommand, seedOrder)
		if err != nil {
			t.Fatal(err)
		}
		if err := outbox.Enqueue(ctx, db, seed); err != nil {
			t.Fatal(err)
		}
		_, _, err = service.Create(ctx, command)
		if err == nil {
			t.Fatal("Create() error = nil")
		}
		assertWorkflowTableCount(ctx, t, db, ordersTable, 0)
		assertWorkflowTableCount(ctx, t, db, historyTable, 0)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 1)
	})

	t.Run("recovers ambiguous commit on retry", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		normalRunner := service.runTx
		service.runTx = func(ctx context.Context, beginner sqlkit.Beginner, options *sql.TxOptions, fn sqlkit.TxFunc) error {
			service.runTx = normalRunner
			if err := normalRunner(ctx, beginner, options, fn); err != nil {
				return err
			}
			return context.DeadlineExceeded
		}
		command := CreateCommand{OrderID: "order-ambiguous", CommandID: "cmd-ambiguous"}
		if _, _, err := service.Create(ctx, command); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Create(ambiguous) error = %v", err)
		}
		order, replayed, err := service.Create(ctx, command)
		if err != nil || !replayed || order.Revision != 1 {
			t.Fatalf("Create(retry) = (%#v, %v, %v)", order, replayed, err)
		}
		assertWorkflowTableCount(ctx, t, db, ordersTable, 1)
		assertWorkflowTableCount(ctx, t, db, historyTable, 1)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 1)
	})
}

func TestServiceConcurrentPostgreSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openWorkflowPostgres(ctx, t)
	_, _, service := newPostgresWorkflowService(ctx, t, db)

	t.Run("identical concurrent create commits once and replays once", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		command := CreateCommand{OrderID: "order-concurrent", CommandID: "cmd-concurrent"}
		type result struct {
			order    Order
			replayed bool
			err      error
		}
		results := make(chan result, 2)
		var start sync.WaitGroup
		start.Add(1)
		for range 2 {
			go func() {
				start.Wait()
				order, replayed, err := service.Create(ctx, command)
				results <- result{order: order, replayed: replayed, err: err}
			}()
		}
		start.Done()
		first, second := <-results, <-results
		if first.err != nil || second.err != nil || first.order != second.order || first.replayed == second.replayed {
			t.Fatalf("concurrent results = %#v, %#v", first, second)
		}
		assertWorkflowTableCount(ctx, t, db, ordersTable, 1)
		assertWorkflowTableCount(ctx, t, db, historyTable, 1)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 1)
	})

	t.Run("same command for different orders commits once and conflicts once", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		commands := []CreateCommand{
			{OrderID: "order-command-a", CommandID: "cmd-shared"},
			{OrderID: "order-command-b", CommandID: "cmd-shared"},
		}
		errorsCh := make(chan error, len(commands))
		var start sync.WaitGroup
		start.Add(1)
		for _, command := range commands {
			go func(command CreateCommand) {
				start.Wait()
				_, _, err := service.Create(ctx, command)
				errorsCh <- err
			}(command)
		}
		start.Done()
		first, second := <-errorsCh, <-errorsCh
		if (first == nil) == (second == nil) {
			t.Fatalf("create errors = (%v, %v), want one success", first, second)
		}
		failure := first
		if failure == nil {
			failure = second
		}
		if !errors.Is(failure, ErrConflict) {
			t.Fatalf("losing create error = %v", failure)
		}
		assertWorkflowTableCount(ctx, t, db, ordersTable, 1)
		assertWorkflowTableCount(ctx, t, db, historyTable, 1)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 1)
	})

	t.Run("same order with different commands commits once and conflicts once", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		commands := []CreateCommand{
			{OrderID: "order-shared", CommandID: "cmd-order-a"},
			{OrderID: "order-shared", CommandID: "cmd-order-b"},
		}
		errorsCh := make(chan error, len(commands))
		var start sync.WaitGroup
		start.Add(1)
		for _, command := range commands {
			go func(command CreateCommand) {
				start.Wait()
				_, _, err := service.Create(ctx, command)
				errorsCh <- err
			}(command)
		}
		start.Done()
		first, second := <-errorsCh, <-errorsCh
		if (first == nil) == (second == nil) {
			t.Fatalf("create errors = (%v, %v), want one success", first, second)
		}
		failure := first
		if failure == nil {
			failure = second
		}
		if !errors.Is(failure, ErrConflict) {
			t.Fatalf("losing create error = %v", failure)
		}
		assertWorkflowTableCount(ctx, t, db, ordersTable, 1)
		assertWorkflowTableCount(ctx, t, db, historyTable, 1)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 1)
	})

	t.Run("two confirms commit one next revision", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		if _, _, err := service.Create(ctx, CreateCommand{OrderID: "order-transition", CommandID: "cmd-create-transition"}); err != nil {
			t.Fatal(err)
		}
		errorsCh := make(chan error, 2)
		for _, commandID := range []string{"cmd-confirm-a", "cmd-confirm-b"} {
			go func(commandID string) {
				_, _, err := service.Transition(ctx, TransitionCommand{OrderID: "order-transition", CommandID: commandID, Action: ActionConfirm})
				errorsCh <- err
			}(commandID)
		}
		first, second := <-errorsCh, <-errorsCh
		if (first == nil) == (second == nil) {
			t.Fatalf("transition errors = (%v, %v), want one success", first, second)
		}
		failure := first
		if failure == nil {
			failure = second
		}
		if !errors.Is(failure, ErrConflict) {
			t.Fatalf("losing transition error = %v", failure)
		}
		assertWorkflowTableCount(ctx, t, db, historyTable, 2)
		assertWorkflowTableCount(ctx, t, db, outboxTable, 2)
	})

	t.Run("lock timeout writes nothing and releases capacity", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		if _, _, err := service.Create(ctx, CreateCommand{OrderID: "order-locked", CommandID: "cmd-create-locked"}); err != nil {
			t.Fatal(err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `select 1 from audited_order_workflow_orders where order_id = $1 for update`, "order-locked"); err != nil {
			t.Fatal(err)
		}
		waitCtx, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, _, transitionErr := service.Transition(waitCtx, TransitionCommand{OrderID: "order-locked", CommandID: "cmd-confirm-locked", Action: ActionConfirm})
		stop()
		if !errors.Is(transitionErr, context.DeadlineExceeded) {
			t.Fatalf("Transition(lock timeout) error = %v", transitionErr)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.Transition(ctx, TransitionCommand{OrderID: "order-locked", CommandID: "cmd-confirm-after-lock", Action: ActionConfirm}); err != nil {
			t.Fatalf("Transition(after lock) error = %v", err)
		}
		assertWorkflowTableCount(ctx, t, db, historyTable, 2)
	})
}

var testWorkflowNow = time.Date(2026, 7, 14, 3, 0, 0, 123456789, time.UTC)

func newPostgresWorkflowService(ctx context.Context, t *testing.T, db *sql.DB) (*HistoryStore, *sqloutbox.Store, *Service) {
	t.Helper()
	history, err := NewHistoryStore(db)
	if err != nil {
		t.Fatal(err)
	}
	outbox := newWorkflowOutbox(t)
	if err := CreateSchema(ctx, db, outbox); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, history, outbox, Config{Author: "workshop", Now: func() time.Time { return testWorkflowNow }})
	if err != nil {
		t.Fatal(err)
	}
	return history, outbox, service
}

func assertWorkflowTableCount(ctx context.Context, t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, `select count(*) from `+table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("count %s = %d, want %d", table, got, want)
	}
}
