package sqlstrategy

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDecisionReportKeepsSQLInspectable(t *testing.T) {
	report, err := NewDecisionReport(Hold{ID: "hold-1001", Customer: "customer-42", Status: StatusPending})
	if err != nil {
		t.Fatalf("NewDecisionReport() error = %v", err)
	}

	assertSnapshot(t, report.Direct, "direct.create_hold",
		`insert into sql_strategy_holds (id, customer, status) values ($1, $2, $3)`,
		[]any{"hold-1001", "customer-42", string(StatusPending)})
	assertSnapshot(t, report.SQLKit, "sqlkit.create_hold",
		`insert into "sql_strategy_holds" ("id", "customer", "status") values ($1, $2, $3)`,
		[]any{"hold-1001", "customer-42", string(StatusPending)})
	assertSnapshot(t, report.SQLKit, "sqlkit.find_hold",
		`select "id", "customer", "status" from "sql_strategy_holds" where id = $1`,
		[]any{"hold-1001"})

	if len(report.Choices) < 4 {
		t.Fatalf("choices = %#v, want direct/sqlkit/sqlc-or-jet/Atlas guidance", report.Choices)
	}
}

func TestDirectAndSQLKitRepositoriesShareReadWriteFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)

	direct := DirectRepository{}
	sqlkitRepo := SQLKitRepository{}

	if err := direct.Create(ctx, db, Hold{ID: "hold-direct", Customer: "customer-direct", Status: StatusPending}); err != nil {
		t.Fatalf("direct create: %v", err)
	}
	directHold, err := direct.Find(ctx, db, "hold-direct")
	if err != nil {
		t.Fatalf("direct find: %v", err)
	}
	if directHold.Status != StatusPending {
		t.Fatalf("direct status = %s, want pending", directHold.Status)
	}

	if err := sqlkitRepo.Create(ctx, db, Hold{ID: "hold-sqlkit", Customer: "customer-sqlkit", Status: StatusPending}); err != nil {
		t.Fatalf("sqlkit create: %v", err)
	}
	sqlkitHold, err := sqlkitRepo.Find(ctx, db, "hold-sqlkit")
	if err != nil {
		t.Fatalf("sqlkit find: %v", err)
	}
	if sqlkitHold.Status != StatusPending {
		t.Fatalf("sqlkit status = %s, want pending", sqlkitHold.Status)
	}

	if err := direct.Confirm(ctx, db, "hold-direct"); err != nil {
		t.Fatalf("direct confirm: %v", err)
	}
	if err := sqlkitRepo.ConfirmWithEvent(ctx, db, "hold-sqlkit", false); err != nil {
		t.Fatalf("sqlkit confirm: %v", err)
	}

	for _, id := range []string{"hold-direct", "hold-sqlkit"} {
		hold, err := sqlkitRepo.Find(ctx, db, id)
		if err != nil {
			t.Fatalf("find confirmed %s: %v", id, err)
		}
		if hold.Status != StatusConfirmed {
			t.Fatalf("%s status = %s, want confirmed", id, hold.Status)
		}
	}
}

func TestSQLKitCardinalityNoRowsAndTooManyRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	repo := SQLKitRepository{}

	_, err := repo.Find(ctx, db, "missing")
	if !errors.Is(err, sqlkit.ErrNoRows) {
		t.Fatalf("Find() error = %v, want sqlkit.ErrNoRows", err)
	}

	for _, hold := range []Hold{
		{ID: "hold-1", Customer: "duplicate-customer", Status: StatusPending},
		{ID: "hold-2", Customer: "duplicate-customer", Status: StatusPending},
	} {
		if err := repo.Create(ctx, db, hold); err != nil {
			t.Fatalf("create duplicate setup: %v", err)
		}
	}

	_, _, err = repo.FindOnePendingByCustomer(ctx, db, "duplicate-customer")
	if !errors.Is(err, sqlkit.ErrTooManyRows) {
		t.Fatalf("FindOnePendingByCustomer() error = %v, want sqlkit.ErrTooManyRows", err)
	}
}

func TestSQLKitTransactionRollbackPreservesOriginalError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	repo := SQLKitRepository{}

	if err := repo.Create(ctx, db, Hold{ID: "hold-rollback", Customer: "customer-rollback", Status: StatusPending}); err != nil {
		t.Fatalf("create rollback setup: %v", err)
	}

	err := repo.ConfirmWithEvent(ctx, db, "hold-rollback", true)
	if !errors.Is(err, ErrAuditRejected) {
		t.Fatalf("ConfirmWithEvent() error = %v, want ErrAuditRejected", err)
	}

	hold, err := repo.Find(ctx, db, "hold-rollback")
	if err != nil {
		t.Fatalf("find rolled-back hold: %v", err)
	}
	if hold.Status != StatusPending {
		t.Fatalf("rolled-back status = %s, want pending", hold.Status)
	}

	var events int
	if err := db.QueryRowContext(ctx, `select count(*) from sql_strategy_hold_events where hold_id = $1`, "hold-rollback").Scan(&events); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != 0 {
		t.Fatalf("events = %d, want 0 after rollback", events)
	}
}

func TestSQLKitPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	repo := SQLKitRepository{}

	canceled, stop := context.WithCancel(context.Background())
	stop()

	_, err := repo.Find(canceled, db, "hold-canceled")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Find() error = %v, want context.Canceled", err)
	}
}

func openPostgresDB(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	dsn := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close postgres: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return db
}

func createSchema(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		`create table sql_strategy_holds (
			id text primary key,
			customer text not null,
			status text not null
		)`,
		`create table sql_strategy_hold_events (
			id bigserial primary key,
			hold_id text not null references sql_strategy_holds(id) on delete cascade,
			kind text not null
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
}

func assertSnapshot(t *testing.T, snapshots []StatementSnapshot, name, wantSQL string, wantArgs []any) {
	t.Helper()

	for _, snapshot := range snapshots {
		if snapshot.Name != name {
			continue
		}
		if snapshot.SQL != wantSQL {
			t.Fatalf("%s SQL = %q, want %q", name, snapshot.SQL, wantSQL)
		}
		if !reflect.DeepEqual(snapshot.Args, wantArgs) {
			t.Fatalf("%s Args = %#v, want %#v", name, snapshot.Args, wantArgs)
		}
		return
	}
	t.Fatalf("snapshot %q not found in %#v", name, snapshots)
}
