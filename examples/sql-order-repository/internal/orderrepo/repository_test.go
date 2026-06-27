package orderrepo

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

func TestPreviewKeepsRepositorySQLInspectable(t *testing.T) {
	sample := sampleOrder("order-1001", "customer-42", StatusPending, 2599, 0)
	preview, err := NewPreview(sample)
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}

	assertSnapshot(t, preview.Statements, "orders.create",
		`insert into "sql_order_repository_orders" ("id", "customer_id", "status", "total_cents", "created_at") values ($1, $2, $3, $4, $5)`,
		[]any{"order-1001", "customer-42", string(StatusPending), int64(2599), sample.CreatedAt.UTC()})
	assertSnapshot(t, preview.Statements, "orders.find_by_id",
		`select "id", "customer_id", "status", "total_cents", "created_at" from "sql_order_repository_orders" where id = $1`,
		[]any{"order-1001"})
	assertSnapshot(t, preview.Statements, "orders.list",
		`select "id", "customer_id", "status", "total_cents", "created_at" from "sql_order_repository_orders" where customer_id = $1 and status = $2 order by "created_at", "id" limit 10`,
		[]any{"customer-42", string(StatusPending)})

	if len(preview.DirectSQL) < 2 {
		t.Fatalf("DirectSQL = %#v, want database/sql boundary guidance", preview.DirectSQL)
	}
}

func TestRepositoryInsertFindAndFilter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	repo := Repository{}

	orders := []Order{
		sampleOrder("order-1001", "customer-42", StatusPending, 2599, 0),
		sampleOrder("order-1002", "customer-42", StatusPaid, 4800, time.Minute),
		sampleOrder("order-1003", "customer-77", StatusPending, 1500, 2*time.Minute),
	}
	for _, order := range orders {
		if err := repo.Create(ctx, db, order); err != nil {
			t.Fatalf("Create(%s) error = %v", order.ID, err)
		}
	}

	found, err := repo.FindByID(ctx, db, "order-1002")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	assertOrder(t, found, orders[1])

	customerOrders, err := repo.List(ctx, db, Filter{CustomerID: "customer-42", Limit: 10})
	if err != nil {
		t.Fatalf("List(customer) error = %v", err)
	}
	if gotIDs := orderIDs(customerOrders); !reflect.DeepEqual(gotIDs, []string{"order-1001", "order-1002"}) {
		t.Fatalf("customer order IDs = %#v", gotIDs)
	}

	pendingOrders, err := repo.List(ctx, db, Filter{CustomerID: "customer-42", Status: StatusPending, Limit: 10})
	if err != nil {
		t.Fatalf("List(customer,status) error = %v", err)
	}
	if gotIDs := orderIDs(pendingOrders); !reflect.DeepEqual(gotIDs, []string{"order-1001"}) {
		t.Fatalf("pending customer order IDs = %#v", gotIDs)
	}
}

func TestRepositoryNotFoundAndValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	repo := Repository{}

	_, err := repo.FindByID(ctx, db, "missing")
	if !errors.Is(err, sqlkit.ErrNoRows) {
		t.Fatalf("FindByID() error = %v, want sqlkit.ErrNoRows", err)
	}

	if err := repo.Create(ctx, db, Order{ID: "broken", CustomerID: "customer-42", Status: StatusPending, TotalCents: -1, CreatedAt: time.Now()}); !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("Create(invalid) error = %v, want ErrInvalidOrder", err)
	}

	_, err = repo.List(ctx, db, Filter{})
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("List(empty filter) error = %v, want ErrInvalidOrder", err)
	}
}

func TestRepositoryPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	repo := Repository{}

	canceled, stop := context.WithCancel(context.Background())
	stop()

	_, err := repo.FindByID(canceled, db, "order-canceled")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FindByID() error = %v, want context.Canceled", err)
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

	statement := `create table sql_order_repository_orders (
		id text primary key,
		customer_id text not null,
		status text not null,
		total_cents bigint not null check (total_cents >= 0),
		created_at timestamptz not null
	)`
	if _, err := db.ExecContext(ctx, statement); err != nil {
		t.Fatalf("create schema: %v", err)
	}
}

func sampleOrder(id, customerID string, status Status, totalCents int64, offset time.Duration) Order {
	return Order{
		ID:         id,
		CustomerID: customerID,
		Status:     status,
		TotalCents: totalCents,
		CreatedAt:  time.Date(2026, 6, 28, 9, 30, 0, 0, time.UTC).Add(offset),
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

func assertOrder(t *testing.T, got, want Order) {
	t.Helper()

	if got.ID != want.ID || got.CustomerID != want.CustomerID || got.Status != want.Status || got.TotalCents != want.TotalCents || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func orderIDs(orders []Order) []string {
	ids := make([]string, 0, len(orders))
	for _, order := range orders {
		ids = append(ids, order.ID)
	}
	return ids
}
