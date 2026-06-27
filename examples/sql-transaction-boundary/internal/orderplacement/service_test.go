package orderplacement

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPreviewKeepsTransactionBoundaryInspectable(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}

	assertSnapshot(t, preview.Statements, "products.lock_for_update",
		`select id, name, stock, price_cents from sql_transaction_products where id in ($1, $2) order by id for update`,
		[]any{"sku-coffee", "sku-filter"})
	assertSnapshot(t, preview.Statements, "products.debit_stock",
		`update sql_transaction_products set stock = stock - $1 where id = $2`,
		[]any{2, "sku-coffee"})
	assertSnapshot(t, preview.Statements, "orders.create_header",
		`insert into "sql_transaction_orders" ("id", "customer_id", "total_cents", "created_at") values ($1, $2, $3, $4)`,
		[]any{"order-1001", "customer-42", int64(3700), previewTime()})

	if len(preview.RollbackPath) < 2 {
		t.Fatalf("RollbackPath = %#v, want rollback guidance", preview.RollbackPath)
	}
}

func TestPlaceOrderCommitsStockDebitAndOrderRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	seedProducts(ctx, t, db)

	placed, err := NewService().PlaceOrder(ctx, db, PlaceOrderRequest{
		OrderID:    "order-commit",
		CustomerID: "customer-42",
		Lines: []LineRequest{
			{ProductID: "sku-coffee", Quantity: 2},
			{ProductID: "sku-filter", Quantity: 1},
		},
		CreatedAt: previewTime(),
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if placed.TotalCents != 3700 || placed.LineCount != 2 {
		t.Fatalf("placed = %#v, want total 3700 and 2 lines", placed)
	}

	assertStock(ctx, t, db, "sku-coffee", 8)
	assertStock(ctx, t, db, "sku-filter", 4)
	assertOrderCount(ctx, t, db, 1)
	assertLineCount(ctx, t, db, "order-commit", 2)
}

func TestPlaceOrderRollsBackOnInsufficientStock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	seedProducts(ctx, t, db)

	_, err := NewService().PlaceOrder(ctx, db, PlaceOrderRequest{
		OrderID:    "order-stock-conflict",
		CustomerID: "customer-42",
		Lines: []LineRequest{
			{ProductID: "sku-filter", Quantity: 6},
		},
		CreatedAt: previewTime(),
	})
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("PlaceOrder() error = %v, want ErrInsufficientStock", err)
	}

	assertStock(ctx, t, db, "sku-filter", 5)
	assertOrderCount(ctx, t, db, 0)
}

func TestPlaceOrderRollsBackAfterStockDebitOnPaymentFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	seedProducts(ctx, t, db)

	_, err := NewService().PlaceOrder(ctx, db, PlaceOrderRequest{
		OrderID:    "order-payment-failure",
		CustomerID: "customer-42",
		Lines: []LineRequest{
			{ProductID: "sku-coffee", Quantity: 3},
		},
		CreatedAt:     previewTime(),
		RejectPayment: true,
	})
	if !errors.Is(err, ErrPaymentRejected) {
		t.Fatalf("PlaceOrder() error = %v, want ErrPaymentRejected", err)
	}

	assertStock(ctx, t, db, "sku-coffee", 10)
	assertOrderCount(ctx, t, db, 0)
}

func TestPlaceOrderPropagatesContextCancellationBeforeCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	createSchema(ctx, t, db)
	seedProducts(ctx, t, db)

	canceled, stop := context.WithCancel(context.Background())
	stop()

	_, err := NewService().PlaceOrder(canceled, db, PlaceOrderRequest{
		OrderID:    "order-canceled",
		CustomerID: "customer-42",
		Lines: []LineRequest{
			{ProductID: "sku-coffee", Quantity: 1},
		},
		CreatedAt: previewTime(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PlaceOrder() error = %v, want context.Canceled", err)
	}

	assertStock(ctx, t, db, "sku-coffee", 10)
	assertOrderCount(ctx, t, db, 0)
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
		`create table sql_transaction_products (
			id text primary key,
			name text not null,
			stock integer not null check (stock >= 0),
			price_cents bigint not null check (price_cents >= 0)
		)`,
		`create table sql_transaction_orders (
			id text primary key,
			customer_id text not null,
			total_cents bigint not null check (total_cents >= 0),
			created_at timestamptz not null
		)`,
		`create table sql_transaction_order_lines (
			order_id text not null references sql_transaction_orders(id) on delete cascade,
			product_id text not null references sql_transaction_products(id),
			quantity integer not null check (quantity > 0),
			unit_price_cents bigint not null check (unit_price_cents >= 0),
			primary key (order_id, product_id)
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
}

func seedProducts(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	rows := []Product{
		{ID: "sku-coffee", Name: "Coffee beans", Stock: 10, PriceCents: 1200},
		{ID: "sku-filter", Name: "Paper filter", Stock: 5, PriceCents: 1300},
	}
	for _, row := range rows {
		_, err := db.ExecContext(ctx, `insert into sql_transaction_products (id, name, stock, price_cents) values ($1, $2, $3, $4)`, row.ID, row.Name, row.Stock, row.PriceCents)
		if err != nil {
			t.Fatalf("seed product %s: %v", row.ID, err)
		}
	}
}

func assertStock(ctx context.Context, t *testing.T, db *sql.DB, productID string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRowContext(ctx, `select stock from sql_transaction_products where id = $1`, productID).Scan(&got); err != nil {
		t.Fatalf("read stock %s: %v", productID, err)
	}
	if got != want {
		t.Fatalf("%s stock = %d, want %d", productID, got, want)
	}
}

func assertOrderCount(ctx context.Context, t *testing.T, db *sql.DB, want int) {
	t.Helper()

	var got int
	if err := db.QueryRowContext(ctx, `select count(*) from sql_transaction_orders`).Scan(&got); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if got != want {
		t.Fatalf("orders = %d, want %d", got, want)
	}
}

func assertLineCount(ctx context.Context, t *testing.T, db *sql.DB, orderID string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRowContext(ctx, `select count(*) from sql_transaction_order_lines where order_id = $1`, orderID).Scan(&got); err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if got != want {
		t.Fatalf("lines = %d, want %d", got, want)
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
