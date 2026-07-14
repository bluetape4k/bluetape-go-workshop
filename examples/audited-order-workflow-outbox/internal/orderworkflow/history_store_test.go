package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNewHistoryStoreRejectsNilDatabase(t *testing.T) {
	store, err := NewHistoryStore(nil)
	if store != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewHistoryStore(nil) = (%v, %v)", store, err)
	}
}

func TestBuildHistoryFindQueryUsesAggregateRevisionOrder(t *testing.T) {
	aggregate, _ := audit.NewAggregateID(orderAggregateType, "order-1001")
	statement, _ := buildHistoryFindQuery(audit.Query{Aggregate: &aggregate, Limit: 20})
	if !strings.Contains(statement, "order by revision") || strings.Contains(statement, "order by position") {
		t.Fatalf("exact aggregate query order = %q", statement)
	}
	statement, _ = buildHistoryFindQuery(audit.Query{AggregateType: orderAggregateType, Limit: 20})
	if !strings.Contains(statement, "order by position") {
		t.Fatalf("cross-aggregate query order = %q", statement)
	}
}

func TestHistoryStorePostgreSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	db := openWorkflowPostgres(ctx, t)
	outbox := newWorkflowOutbox(t)
	if err := CreateSchema(ctx, db, outbox); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}
	if err := CreateSchema(ctx, db, outbox); err != nil {
		t.Fatalf("second CreateSchema() error = %v", err)
	}
	store, err := NewHistoryStore(db)
	if err != nil {
		t.Fatalf("NewHistoryStore() error = %v", err)
	}
	var _ audit.HistoryReader = store

	t.Run("insert find load latest and command lookup", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		first, second := workflowTestEntries(t)
		if err := store.Insert(ctx, db, first); err != nil {
			t.Fatalf("Insert(first) error = %v", err)
		}
		if err := store.Insert(ctx, db, second); err != nil {
			t.Fatalf("Insert(second) error = %v", err)
		}

		aggregate := first.Aggregate
		entries, err := store.Find(ctx, audit.Query{
			Aggregate: &aggregate, FromRevision: 1, ToRevision: 2, Limit: 2,
		})
		if err != nil || len(entries) != 2 || entries[0].Revision != 1 || entries[1].Revision != 2 {
			t.Fatalf("Find() = (%#v, %v)", entries, err)
		}
		newest, err := store.Find(ctx, audit.Query{Aggregate: &aggregate, NewestFirst: true, Limit: 1})
		if err != nil || len(newest) != 1 || newest[0].Revision != 2 {
			t.Fatalf("Find(newest) = (%#v, %v)", newest, err)
		}
		history, found, err := store.LoadHistory(ctx, aggregate)
		if err != nil || !found || history.HeadRevision() != 2 {
			t.Fatalf("LoadHistory() = (%#v, %v, %v)", history, found, err)
		}
		latest, found, err := store.Latest(ctx, aggregate)
		if err != nil || !found || latest.Revision != 2 {
			t.Fatalf("Latest() = (%#v, %v, %v)", latest, found, err)
		}
		byCommand, found, err := store.FindByCommandID(ctx, db, string(second.Event.EventID))
		if err != nil || !found || byCommand.Event.EventID != second.Event.EventID {
			t.Fatalf("FindByCommandID() = (%#v, %v, %v)", byCommand, found, err)
		}
		if _, found, err := store.LatestSnapshot(ctx, aggregate); err != nil || found {
			t.Fatalf("LatestSnapshot() = found %v, error %v", found, err)
		}
		if _, found, err := store.PreviousSnapshot(ctx, aggregate, 2); err != nil || found {
			t.Fatalf("PreviousSnapshot() = found %v, error %v", found, err)
		}
	})

	t.Run("inclusive recorded bounds and aggregate type", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		first, second := workflowTestEntries(t)
		if err := store.Insert(ctx, db, first); err != nil {
			t.Fatal(err)
		}
		if err := store.Insert(ctx, db, second); err != nil {
			t.Fatal(err)
		}
		entries, err := store.Find(ctx, audit.Query{
			AggregateType:  orderAggregateType,
			FromRecordedAt: second.Event.RecordedAt,
			ToRecordedAt:   second.Event.RecordedAt,
		})
		if err != nil || len(entries) != 1 || entries[0].Revision != 2 {
			t.Fatalf("Find(recorded bounds) = (%#v, %v)", entries, err)
		}
	})

	t.Run("missing and invalid inputs", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		aggregate, _ := audit.NewAggregateID(orderAggregateType, "missing")
		if _, found, err := store.LoadHistory(ctx, aggregate); err != nil || found {
			t.Fatalf("LoadHistory(missing) = found %v, error %v", found, err)
		}
		if _, found, err := store.Latest(ctx, aggregate); err != nil || found {
			t.Fatalf("Latest(missing) = found %v, error %v", found, err)
		}
		if _, err := store.Find(ctx, audit.Query{Limit: -1}); !errors.Is(err, audit.ErrInvalidQuery) {
			t.Fatalf("Find(invalid) error = %v", err)
		}
		if err := store.Insert(ctx, nil, audit.Entry{}); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("Insert(nil) error = %v", err)
		}
	})

	t.Run("preserves cancellation", func(t *testing.T) {
		cancelled, stop := context.WithCancel(context.Background())
		stop()
		if _, err := store.Find(cancelled, audit.Query{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Find(cancelled) error = %v", err)
		}
	})

	t.Run("fails closed on scalar json mismatch", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		first, _ := workflowTestEntries(t)
		if err := store.Insert(ctx, db, first); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `update audited_order_workflow_audit_entries set event_type = 'order.corrupt'`); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Find(ctx, audit.Query{}); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("Find(corrupt) error = %v", err)
		}
	})

	t.Run("reports bounded delivery status without identities", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		entries := make([]audit.Entry, 5)
		base := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
		for index := range entries {
			orderID := "status-order-" + string(rune('a'+index))
			command := CreateCommand{OrderID: orderID, CommandID: "status-command-" + string(rune('a'+index))}
			order := Order{OrderID: orderID, Status: StatusPending, Revision: 1, UpdatedAt: base}
			entry, err := buildCreateEntry("workshop", base, command, order)
			if err != nil {
				t.Fatal(err)
			}
			entries[index] = entry
		}
		if err := outbox.Enqueue(ctx, db, entries...); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		if _, err := db.ExecContext(ctx, `
update audited_order_workflow_outbox_records
set status = case event_id
    when 'status-command-a' then 'pending'
    when 'status-command-b' then 'pending'
    when 'status-command-c' then 'claimed'
    when 'status-command-d' then 'published'
    when 'status-command-e' then 'dead_letter'
end,
attempts = case when event_id = 'status-command-a' then 0 else 1 end,
created_at = $1`, base.Add(-5*time.Second)); err != nil {
			t.Fatal(err)
		}
		status, err := store.DeliveryStatus(ctx, base)
		if err != nil {
			t.Fatalf("DeliveryStatus() error = %v", err)
		}
		if status.Pending != 1 || status.Retrying != 1 || status.Claimed != 1 ||
			status.Published != 1 || status.DeadLetter != 1 || status.OldestPendingSeconds != 5 {
			t.Fatalf("DeliveryStatus() = %#v", status)
		}
	})

	t.Run("uses bounded indexes for populated history searches", func(t *testing.T) {
		resetWorkflowTables(ctx, t, db)
		base := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
		if _, err := db.ExecContext(ctx, `
insert into audited_order_workflow_audit_entries (
    aggregate_type, aggregate_id, revision, event_id, idempotency_key,
    event_type, recorded_at, entry_json
)
select
    'order',
    case when sequence <= 5000 then 'order-hot' else 'order-distractor-' || sequence::text end,
    case when sequence <= 5000 then sequence else 1 end,
    'seed-event-' || sequence::text,
    'seed-command-' || sequence::text,
    'order.seeded',
    $1::timestamptz + sequence * interval '1 second',
    jsonb_build_object('seed', sequence)
from generate_series(1, 10000) as sequence`, base); err != nil {
			t.Fatalf("seed populated history: %v", err)
		}
		if _, err := db.ExecContext(ctx, `analyze audited_order_workflow_audit_entries`); err != nil {
			t.Fatalf("analyze history: %v", err)
		}
		aggregate, _ := audit.NewAggregateID(orderAggregateType, "order-hot")
		revisionQuery, revisionArgs := buildHistoryFindQuery(audit.Query{
			Aggregate: &aggregate, FromRevision: 4900, Limit: 20,
		})
		assertExplainUsesIndex(ctx, t, db, revisionQuery, revisionArgs,
			"audited_order_workflow_audit_entries_pkey")
		timeQuery, timeArgs := buildHistoryFindQuery(audit.Query{
			Aggregate:      &aggregate,
			FromRecordedAt: base.Add(4990 * time.Second),
			ToRecordedAt:   base.Add(5000 * time.Second),
			Limit:          20,
		})
		assertExplainUsesIndex(ctx, t, db, timeQuery, timeArgs,
			"audited_order_workflow_audit_entries_aggregate_time_idx")
	})

	t.Run("rejects incompatible existing schemas without altering them", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, `
drop table if exists audited_order_workflow_outbox_records cascade;
drop table if exists audited_order_workflow_audit_entries cascade;
drop table if exists audited_order_workflow_orders cascade;
create table audited_order_workflow_orders (order_id text primary key)`); err != nil {
			t.Fatalf("create incompatible orders table: %v", err)
		}
		if err := CreateSchema(ctx, db, outbox); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("CreateSchema(incompatible orders) error = %v", err)
		}
		var statusColumns int
		if err := db.QueryRowContext(ctx, `
select count(*) from information_schema.columns
where table_schema = 'public'
  and table_name = 'audited_order_workflow_orders'
  and column_name = 'status'`).Scan(&statusColumns); err != nil {
			t.Fatal(err)
		}
		if statusColumns != 0 {
			t.Fatal("CreateSchema altered incompatible orders table")
		}

		if _, err := db.ExecContext(ctx, `drop table audited_order_workflow_orders cascade`); err != nil {
			t.Fatal(err)
		}
		if err := CreateSchema(ctx, db, outbox); err != nil {
			t.Fatalf("CreateSchema(recover partial) error = %v", err)
		}
		if _, err := db.ExecContext(ctx, `
alter table audited_order_workflow_outbox_records
drop constraint audited_order_workflow_outbox_records_event_id_key`); err != nil {
			t.Fatalf("drop outbox identity constraint: %v", err)
		}
		if err := CreateSchema(ctx, db, outbox); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("CreateSchema(missing outbox identity constraint) error = %v", err)
		}
		if _, err := db.ExecContext(ctx, `
drop table audited_order_workflow_outbox_records cascade;
create table audited_order_workflow_outbox_records (id bigserial primary key)`); err != nil {
			t.Fatalf("create incompatible outbox table: %v", err)
		}
		if err := CreateSchema(ctx, db, outbox); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("CreateSchema(incompatible outbox) error = %v", err)
		}
	})
}

func assertExplainUsesIndex(ctx context.Context, t *testing.T, db *sql.DB, query string, args []any, index string) {
	t.Helper()
	var plan []byte
	if err := db.QueryRowContext(ctx, "explain (format json) "+query, args...).Scan(&plan); err != nil {
		t.Fatalf("explain query: %v", err)
	}
	text := string(plan)
	if !strings.Contains(text, index) || strings.Contains(text, `"Node Type": "Seq Scan"`) {
		t.Fatalf("query plan does not use %s without sequential scan: %s", index, text)
	}
}

func openWorkflowPostgres(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()
	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("database close: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PingContext() error = %v", err)
	}
	return db
}

func newWorkflowOutbox(t *testing.T) *sqloutbox.Store {
	t.Helper()
	store, err := sqloutbox.NewStore(sqloutbox.Options{Table: outboxTable})
	if err != nil {
		t.Fatalf("sqloutbox.NewStore() error = %v", err)
	}
	return store
}

func resetWorkflowTables(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `truncate table audited_order_workflow_outbox_records, audited_order_workflow_audit_entries, audited_order_workflow_orders restart identity`); err != nil {
		t.Fatalf("reset tables: %v", err)
	}
}

func workflowTestEntries(t *testing.T) (audit.Entry, audit.Entry) {
	t.Helper()
	firstTime := time.Date(2026, 7, 14, 3, 0, 0, 123456000, time.UTC)
	create := CreateCommand{OrderID: "order-1001", CommandID: "cmd-create-1001", Metadata: audit.Metadata{"source": "test"}}
	firstOrder := Order{OrderID: create.OrderID, Status: StatusPending, Revision: 1, UpdatedAt: firstTime}
	first, err := buildCreateEntry("workshop", firstTime, create, firstOrder)
	if err != nil {
		t.Fatalf("buildCreateEntry() error = %v", err)
	}
	secondTime := firstTime.Add(time.Minute)
	transition := TransitionCommand{OrderID: create.OrderID, CommandID: "cmd-confirm-1001", Action: ActionConfirm}
	secondOrder := Order{OrderID: create.OrderID, Status: StatusConfirmed, Revision: 2, UpdatedAt: secondTime}
	second, err := buildTransitionEntry("workshop", secondTime, transition, secondOrder)
	if err != nil {
		t.Fatalf("buildTransitionEntry() error = %v", err)
	}
	return first, second
}
