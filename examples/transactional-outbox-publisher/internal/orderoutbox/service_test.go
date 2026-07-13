package orderoutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNewServiceRejectsInvalidConfiguration(t *testing.T) {
	store := newTestStore(t)
	tests := []struct {
		name   string
		store  *sqloutbox.Store
		author string
	}{
		{name: "nil store", author: "workshop"},
		{name: "blank author", store: store, author: " \t\n "},
		{name: "invalid UTF-8 author", store: store, author: string([]byte{0xff})},
		{name: "oversized author", store: store, author: strings.Repeat("가", maxIDRunes+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(tt.store, Config{Author: tt.author})
			if service != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewService() = (%v, %v), want (nil, ErrInvalidConfig)", service, err)
			}
		})
	}
}

func TestNewServiceDefaultsClockAndTrimsAuthor(t *testing.T) {
	service, err := NewService(newTestStore(t), Config{Author: "  workshop  "})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service.author != "workshop" {
		t.Fatalf("author = %q, want workshop", service.author)
	}
	if service.now == nil {
		t.Fatal("now is nil, want default UTC clock")
	}
	if got := service.now(); got.Location() != time.UTC {
		t.Fatalf("now location = %v, want UTC", got.Location())
	}
}

func TestServiceCreateSchema(t *testing.T) {
	service, err := NewService(newTestStore(t), Config{Author: "workshop"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	execer := &recordingExecer{}
	var noContext context.Context
	if err := service.CreateSchema(noContext, execer); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}
	if len(execer.queries) != 3 {
		t.Fatalf("query count = %d, want 3", len(execer.queries))
	}
	if !strings.Contains(execer.queries[0], ordersTable) {
		t.Fatalf("first query does not create %s: %s", ordersTable, execer.queries[0])
	}
	if !strings.Contains(execer.queries[1], outboxTable) {
		t.Fatalf("second query does not create %s: %s", outboxTable, execer.queries[1])
	}
	if !strings.Contains(execer.queries[2], outboxTable) {
		t.Fatalf("third query does not index %s: %s", outboxTable, execer.queries[2])
	}
}

func TestServiceCreateSchemaFailsClosed(t *testing.T) {
	service, err := NewService(newTestStore(t), Config{Author: "workshop"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	var nilService *Service
	tests := []struct {
		name    string
		service *Service
		execer  *recordingExecer
	}{
		{name: "nil service", service: nilService, execer: &recordingExecer{}},
		{name: "zero service", service: &Service{}, execer: &recordingExecer{}},
		{name: "nil execer", service: service},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.service.CreateSchema(context.Background(), tt.execer)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("CreateSchema() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestServiceCreateSchemaPreservesCancellation(t *testing.T) {
	service, err := NewService(newTestStore(t), Config{Author: "workshop"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.CreateSchema(ctx, &recordingExecer{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateSchema() error = %v, want context.Canceled", err)
	}
}

func TestServiceValidation(t *testing.T) {
	service, err := NewService(newTestStore(t), Config{Author: "workshop"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	valid := validPlaceOrderCommand()

	tests := []struct {
		name    string
		service *Service
		command PlaceOrderCommand
		want    error
	}{
		{name: "nil service", command: valid, want: ErrInvalidConfig},
		{name: "zero service", service: &Service{}, command: valid, want: ErrInvalidConfig},
		{name: "nil database", service: service, command: valid, want: ErrInvalidConfig},
		{name: "blank order id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.OrderID = " \t " }), want: ErrInvalidOrder},
		{name: "invalid order id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.OrderID = string([]byte{0xff}) }), want: ErrInvalidOrder},
		{name: "oversized order id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.OrderID = strings.Repeat("주", maxIDRunes+1) }), want: ErrInvalidOrder},
		{name: "blank customer id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CustomerID = "" }), want: ErrInvalidOrder},
		{name: "invalid customer id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CustomerID = string([]byte{0xff}) }), want: ErrInvalidOrder},
		{name: "oversized customer id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CustomerID = strings.Repeat("객", maxIDRunes+1) }), want: ErrInvalidOrder},
		{name: "blank command id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CommandID = "" }), want: ErrInvalidOrder},
		{name: "invalid command id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CommandID = string([]byte{0xff}) }), want: ErrInvalidOrder},
		{name: "oversized command id", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CommandID = strings.Repeat("명", maxIDRunes+1) }), want: ErrInvalidOrder},
		{name: "zero total", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.TotalCents = 0 }), want: ErrInvalidOrder},
		{name: "negative total", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.TotalCents = -1 }), want: ErrInvalidOrder},
		{name: "zero created at", service: service, command: commandWith(valid, func(c *PlaceOrderCommand) { c.CreatedAt = time.Time{} }), want: ErrInvalidOrder},
	}
	var noContext context.Context

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.service.Place(noContext, nil, tt.command)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Place() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestServicePlacePostgreSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openOrderoutboxPostgres(ctx, t)
	fixedNow := time.Date(2026, 7, 14, 12, 30, 0, 123000000, time.FixedZone("KST", 9*60*60))
	store, err := sqloutbox.NewStore(sqloutbox.Options{
		Table: outboxTable,
		Now:   func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("sqloutbox.NewStore() error = %v", err)
	}
	service, err := NewService(store, Config{
		Author: "workshop",
		Now:    func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}

	t.Run("commits one order and one outbox entry", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		command := validPlaceOrderCommand()
		var noContext context.Context
		placed, err := service.Place(noContext, db, command)
		if err != nil {
			t.Fatalf("Place() error = %v", err)
		}
		want := Order{
			OrderID: command.OrderID, CustomerID: command.CustomerID,
			Status: StatusPlaced, TotalCents: command.TotalCents,
			CreatedAt: command.CreatedAt.UTC(),
		}
		if placed != want {
			t.Fatalf("Place() = %#v, want %#v", placed, want)
		}
		assertTableCount(ctx, t, db, ordersTable, 1)
		assertTableCount(ctx, t, db, outboxTable, 1)
		assertPersistedEntry(ctx, t, db, command, fixedNow.UTC())
	})

	t.Run("duplicate order rolls back new outbox entry", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		first := validPlaceOrderCommand()
		if _, err := service.Place(ctx, db, first); err != nil {
			t.Fatalf("first Place() error = %v", err)
		}
		second := first
		second.CommandID = "command-duplicate-order"
		if _, err := service.Place(ctx, db, second); err == nil {
			t.Fatal("second Place() error = nil, want duplicate order error")
		}
		assertTableCount(ctx, t, db, ordersTable, 1)
		assertTableCount(ctx, t, db, outboxTable, 1)
	})

	t.Run("outbox identity conflict rolls back new order", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		seed := validPlaceOrderCommand()
		if _, err := service.Place(ctx, db, seed); err != nil {
			t.Fatalf("seed Place() error = %v", err)
		}
		conflict := seed
		conflict.OrderID = "order-identity-conflict"
		if _, err := service.Place(ctx, db, conflict); err == nil {
			t.Fatal("conflicting Place() error = nil, want outbox identity error")
		}
		assertTableCount(ctx, t, db, ordersTable, 1)
		assertTableCount(ctx, t, db, outboxTable, 1)
		assertOrderMissing(ctx, t, db, conflict.OrderID)
	})

	t.Run("pre-cancel rolls back both writes", func(t *testing.T) {
		resetOrderoutboxTables(ctx, t, db)
		canceled, stop := context.WithCancel(context.Background())
		stop()
		if _, err := service.Place(canceled, db, validPlaceOrderCommand()); !errors.Is(err, context.Canceled) {
			t.Fatalf("Place() error = %v, want context.Canceled", err)
		}
		assertTableCount(ctx, t, db, ordersTable, 0)
		assertTableCount(ctx, t, db, outboxTable, 0)
	})
}

func validPlaceOrderCommand() PlaceOrderCommand {
	return PlaceOrderCommand{
		OrderID: "order-1001", CustomerID: "customer-42", CommandID: "command-1001",
		TotalCents: 3700,
		CreatedAt:  time.Date(2026, 7, 14, 9, 0, 0, 0, time.FixedZone("CEST", 2*60*60)),
	}
}

func commandWith(command PlaceOrderCommand, change func(*PlaceOrderCommand)) PlaceOrderCommand {
	change(&command)
	return command
}

func openOrderoutboxPostgres(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()
	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close postgres: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return db
}

func resetOrderoutboxTables(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `truncate table transactional_outbox_orders, transactional_outbox_records restart identity`); err != nil {
		t.Fatalf("reset tables: %v", err)
	}
}

func assertTableCount(ctx context.Context, t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	query := `select count(*) from ` + table
	if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s rows = %d, want %d", table, got, want)
	}
}

func assertOrderMissing(ctx context.Context, t *testing.T, db *sql.DB, orderID string) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `select count(*) from transactional_outbox_orders where order_id = $1`, orderID).Scan(&count); err != nil {
		t.Fatalf("count order %q: %v", orderID, err)
	}
	if count != 0 {
		t.Fatalf("order %q rows = %d, want 0", orderID, count)
	}
}

func assertPersistedEntry(ctx context.Context, t *testing.T, db *sql.DB, command PlaceOrderCommand, recordedAt time.Time) {
	t.Helper()
	var (
		aggregateTypeValue string
		aggregateID        string
		revision           uint64
		eventID            string
		idempotencyKey     string
		eventTypeValue     string
		occurredAt         time.Time
		persistedRecorded  time.Time
		entryJSON          []byte
	)
	err := db.QueryRowContext(ctx, `
		select aggregate_type, aggregate_id, revision, event_id, idempotency_key,
		       event_type, occurred_at, recorded_at, entry_json
		from transactional_outbox_records`).Scan(
		&aggregateTypeValue, &aggregateID, &revision, &eventID, &idempotencyKey,
		&eventTypeValue, &occurredAt, &persistedRecorded, &entryJSON,
	)
	if err != nil {
		t.Fatalf("read outbox entry: %v", err)
	}
	if aggregateTypeValue != aggregateType || aggregateID != command.OrderID {
		t.Fatalf("aggregate = %s:%s, want %s:%s", aggregateTypeValue, aggregateID, aggregateType, command.OrderID)
	}
	if revision != uint64(audit.InitialRevision()) {
		t.Fatalf("revision = %d, want %d", revision, audit.InitialRevision())
	}
	if eventID != command.CommandID || idempotencyKey != command.CommandID {
		t.Fatalf("identity = (%q, %q), want %q", eventID, idempotencyKey, command.CommandID)
	}
	if eventTypeValue != string(eventType) {
		t.Fatalf("event type = %q, want %q", eventTypeValue, eventType)
	}
	if !occurredAt.Equal(command.CreatedAt.UTC()) || !persistedRecorded.Equal(recordedAt) {
		t.Fatalf("timestamps = (%s, %s), want (%s, %s)", occurredAt, persistedRecorded, command.CreatedAt.UTC(), recordedAt)
	}
	entry, err := audit.DecodeEntryJSON(entryJSON)
	if err != nil {
		t.Fatalf("audit.DecodeEntryJSON() error = %v", err)
	}
	if entry.Event.EventID != audit.EventID(command.CommandID) || entry.Event.IdempotencyKey != command.CommandID {
		t.Fatalf("decoded identity = (%q, %q), want %q", entry.Event.EventID, entry.Event.IdempotencyKey, command.CommandID)
	}
	var payload struct {
		CustomerID string `json:"customer_id"`
		Status     string `json:"status"`
		TotalCents int64  `json:"total_cents"`
	}
	if err := json.Unmarshal(entry.Event.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.CustomerID != command.CustomerID || payload.Status != StatusPlaced || payload.TotalCents != command.TotalCents {
		t.Fatalf("payload = %#v, want customer/status/total from command", payload)
	}
}

func newTestStore(t *testing.T) *sqloutbox.Store {
	t.Helper()
	store, err := sqloutbox.NewStore(sqloutbox.Options{Table: outboxTable})
	if err != nil {
		t.Fatalf("sqloutbox.NewStore() error = %v", err)
	}
	return store
}

type recordingExecer struct {
	queries []string
}

func (e *recordingExecer) ExecContext(ctx context.Context, query string, _ ...any) (sql.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.queries = append(e.queries, query)
	return execResult(0), nil
}

type execResult int64

func (r execResult) LastInsertId() (int64, error) { return int64(r), nil }
func (r execResult) RowsAffected() (int64, error) { return int64(r), nil }
