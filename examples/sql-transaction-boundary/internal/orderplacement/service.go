// Package orderplacement demonstrates an explicit SQL transaction boundary.
package orderplacement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const (
	productsTable = "sql_transaction_products"
	ordersTable   = "sql_transaction_orders"
	linesTable    = "sql_transaction_order_lines"
)

var (
	// ErrInvalidOrder reports invalid order placement input.
	ErrInvalidOrder = errors.New("orderplacement: invalid order")
	// ErrInsufficientStock reports a product stock conflict inside the transaction.
	ErrInsufficientStock = errors.New("orderplacement: insufficient stock")
	// ErrPaymentRejected simulates a downstream failure after stock has been debited.
	ErrPaymentRejected = errors.New("orderplacement: payment rejected")
)

// Product is the stock row read and updated inside the transaction.
type Product struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Stock      int    `json:"stock"`
	PriceCents int64  `json:"price_cents"`
}

// LineRequest is one requested order line.
type LineRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// PlaceOrderRequest is the service input.
type PlaceOrderRequest struct {
	OrderID       string        `json:"order_id"`
	CustomerID    string        `json:"customer_id"`
	Lines         []LineRequest `json:"lines"`
	CreatedAt     time.Time     `json:"created_at"`
	RejectPayment bool          `json:"reject_payment"`
}

// PlacedOrder is returned only after the transaction commits.
type PlacedOrder struct {
	OrderID    string `json:"order_id"`
	LineCount  int    `json:"line_count"`
	TotalCents int64  `json:"total_cents"`
}

// StatementSnapshot is an inspectable SQL statement plus ordered arguments.
type StatementSnapshot struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// Preview documents the runnable transaction lesson.
type Preview struct {
	Scenario     string              `json:"scenario"`
	Boundary     string              `json:"transaction_boundary"`
	Statements   []StatementSnapshot `json:"statements"`
	CommitPath   []string            `json:"commit_path"`
	RollbackPath []string            `json:"rollback_path"`
	Production   []string            `json:"production_hardening"`
	TestCommand  string              `json:"test_command"`
}

// Service owns transaction lifetime and orchestrates narrow repositories.
type Service struct {
	products ProductRepository
	orders   OrderRepository
}

// NewService returns a transaction-bound order placement service.
func NewService() Service {
	return Service{
		products: ProductRepository{},
		orders:   OrderRepository{},
	}
}

// NewPreview builds a readable transaction contract snapshot.
func NewPreview() (Preview, error) {
	products := ProductRepository{}
	orders := OrderRepository{}

	lock, err := products.LockStatement([]string{"sku-coffee", "sku-filter"})
	if err != nil {
		return Preview{}, err
	}
	debit, err := products.DebitStatement("sku-coffee", 2)
	if err != nil {
		return Preview{}, err
	}
	createOrder, err := orders.CreateOrderStatement("order-1001", "customer-42", 3700, previewTime())
	if err != nil {
		return Preview{}, err
	}
	createLine, err := orders.CreateLineStatement("order-1001", LineRequest{ProductID: "sku-coffee", Quantity: 2}, 1200)
	if err != nil {
		return Preview{}, err
	}

	return Preview{
		Scenario:   "Place an order by locking product rows, debiting stock, inserting order rows, and committing once all side effects succeed.",
		Boundary:   "Service.PlaceOrder owns sqlkit.WithTx; repositories only use the *sql.Tx they receive.",
		Statements: []StatementSnapshot{lock, debit, createOrder, createLine},
		CommitPath: []string{
			"validate the order request before opening a transaction",
			"begin sqlkit.WithTx at the service boundary",
			"lock product rows in sorted product_id order",
			"debit stock and insert order header plus lines",
			"commit only after payment and all SQL writes succeed",
		},
		RollbackPath: []string{
			"insufficient stock returns ErrInsufficientStock and rolls back",
			"payment rejection after stock debit returns ErrPaymentRejected and rolls back",
			"context cancellation propagates through BeginTx and query calls",
		},
		Production: []string{
			"keep retry policy outside the transaction body",
			"choose isolation level per workload before adding concurrency",
			"avoid external network calls while holding row locks",
			"record metrics for lock wait, rollback reason, and commit latency",
		},
		TestCommand: "go test -count=1 ./examples/sql-transaction-boundary/...",
	}, nil
}

// PlaceOrder commits stock debit and order rows together or rolls all changes back.
func (svc Service) PlaceOrder(ctx context.Context, db *sql.DB, req PlaceOrderRequest) (PlacedOrder, error) {
	if err := validateRequest(req); err != nil {
		return PlacedOrder{}, err
	}

	var placed PlacedOrder
	err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		productIDs := uniqueProductIDs(req.Lines)
		products, err := svc.products.LockByID(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		byID := make(map[string]Product, len(products))
		for _, product := range products {
			byID[product.ID] = product
		}

		var total int64
		for _, line := range req.Lines {
			product, ok := byID[line.ProductID]
			if !ok {
				return fmt.Errorf("%w: product %s not found", ErrInsufficientStock, line.ProductID)
			}
			if product.Stock < line.Quantity {
				return fmt.Errorf("%w: product %s has %d, need %d", ErrInsufficientStock, line.ProductID, product.Stock, line.Quantity)
			}
			if err := svc.products.Debit(ctx, tx, line.ProductID, line.Quantity); err != nil {
				return err
			}
			total += int64(line.Quantity) * product.PriceCents
		}

		if err := svc.orders.CreateOrder(ctx, tx, req.OrderID, req.CustomerID, total, req.CreatedAt); err != nil {
			return err
		}
		for _, line := range req.Lines {
			product := byID[line.ProductID]
			if err := svc.orders.CreateLine(ctx, tx, req.OrderID, line, product.PriceCents); err != nil {
				return err
			}
		}
		if req.RejectPayment {
			return ErrPaymentRejected
		}
		placed = PlacedOrder{OrderID: req.OrderID, LineCount: len(req.Lines), TotalCents: total}
		return nil
	})
	if err != nil {
		return PlacedOrder{}, err
	}
	return placed, nil
}

// ProductRepository owns product row locking and stock updates.
type ProductRepository struct{}

// LockByID locks product rows in deterministic product_id order.
func (repo ProductRepository) LockByID(ctx context.Context, db sqlkit.Queryer, ids []string) ([]Product, error) {
	stmt, err := repo.lockSQL(ids)
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanProduct, stmt.Args...)
}

// Debit decrements product stock.
func (repo ProductRepository) Debit(ctx context.Context, db sqlkit.Execer, id string, quantity int) error {
	stmt, err := repo.debitSQL(id, quantity)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// LockStatement returns the product lock SQL shown by the preview.
func (repo ProductRepository) LockStatement(ids []string) (StatementSnapshot, error) {
	stmt, err := repo.lockSQL(ids)
	return snapshot("products.lock_for_update", stmt, err)
}

// DebitStatement returns the product debit SQL shown by the preview.
func (repo ProductRepository) DebitStatement(id string, quantity int) (StatementSnapshot, error) {
	stmt, err := repo.debitSQL(id, quantity)
	return snapshot("products.debit_stock", stmt, err)
}

func (ProductRepository) lockSQL(ids []string) (sqlkit.Statement, error) {
	if len(ids) == 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: product ids are required", ErrInvalidOrder)
	}
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	args := make([]any, 0, len(sorted))
	placeholders := make([]string, 0, len(sorted))
	for i, id := range sorted {
		if id == "" {
			return sqlkit.Statement{}, fmt.Errorf("%w: product id is required", ErrInvalidOrder)
		}
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	query := fmt.Sprintf(`select id, name, stock, price_cents from %s where id in (%s) order by id for update`, productsTable, join(placeholders, ", "))
	return sqlkit.NewStatement(query, args...), nil
}

func (ProductRepository) debitSQL(id string, quantity int) (sqlkit.Statement, error) {
	if id == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: product id is required", ErrInvalidOrder)
	}
	if quantity <= 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	}
	return sqlkit.NewStatement(`update sql_transaction_products set stock = stock - $1 where id = $2`, quantity, id), nil
}

// OrderRepository owns order header and line inserts.
type OrderRepository struct{}

// CreateOrder inserts an order header.
func (repo OrderRepository) CreateOrder(ctx context.Context, db sqlkit.Execer, orderID, customerID string, totalCents int64, createdAt time.Time) error {
	stmt, err := repo.createOrderSQL(orderID, customerID, totalCents, createdAt)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// CreateLine inserts an order line.
func (repo OrderRepository) CreateLine(ctx context.Context, db sqlkit.Execer, orderID string, line LineRequest, unitPriceCents int64) error {
	stmt, err := repo.createLineSQL(orderID, line, unitPriceCents)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// CreateOrderStatement returns the order insert SQL shown by the preview.
func (repo OrderRepository) CreateOrderStatement(orderID, customerID string, totalCents int64, createdAt time.Time) (StatementSnapshot, error) {
	stmt, err := repo.createOrderSQL(orderID, customerID, totalCents, createdAt)
	return snapshot("orders.create_header", stmt, err)
}

// CreateLineStatement returns the line insert SQL shown by the preview.
func (repo OrderRepository) CreateLineStatement(orderID string, line LineRequest, unitPriceCents int64) (StatementSnapshot, error) {
	stmt, err := repo.createLineSQL(orderID, line, unitPriceCents)
	return snapshot("orders.create_line", stmt, err)
}

func (OrderRepository) createOrderSQL(orderID, customerID string, totalCents int64, createdAt time.Time) (sqlkit.Statement, error) {
	switch {
	case orderID == "":
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	case customerID == "":
		return sqlkit.Statement{}, fmt.Errorf("%w: customer id is required", ErrInvalidOrder)
	case totalCents < 0:
		return sqlkit.Statement{}, fmt.Errorf("%w: total cents must not be negative", ErrInvalidOrder)
	case createdAt.IsZero():
		return sqlkit.Statement{}, fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	}
	return sqlkit.InsertInto(ordersTable).
		Columns("id", "customer_id", "total_cents", "created_at").
		Values(orderID, customerID, totalCents, createdAt.UTC()).
		Build()
}

func (OrderRepository) createLineSQL(orderID string, line LineRequest, unitPriceCents int64) (sqlkit.Statement, error) {
	switch {
	case orderID == "":
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	case line.ProductID == "":
		return sqlkit.Statement{}, fmt.Errorf("%w: product id is required", ErrInvalidOrder)
	case line.Quantity <= 0:
		return sqlkit.Statement{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	case unitPriceCents < 0:
		return sqlkit.Statement{}, fmt.Errorf("%w: unit price must not be negative", ErrInvalidOrder)
	}
	return sqlkit.InsertInto(linesTable).
		Columns("order_id", "product_id", "quantity", "unit_price_cents").
		Values(orderID, line.ProductID, line.Quantity, unitPriceCents).
		Build()
}

func validateRequest(req PlaceOrderRequest) error {
	switch {
	case req.OrderID == "":
		return fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	case req.CustomerID == "":
		return fmt.Errorf("%w: customer id is required", ErrInvalidOrder)
	case len(req.Lines) == 0:
		return fmt.Errorf("%w: at least one line is required", ErrInvalidOrder)
	case req.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	}
	seen := map[string]struct{}{}
	for _, line := range req.Lines {
		if line.ProductID == "" {
			return fmt.Errorf("%w: product id is required", ErrInvalidOrder)
		}
		if line.Quantity <= 0 {
			return fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
		}
		if _, ok := seen[line.ProductID]; ok {
			return fmt.Errorf("%w: duplicate product %s", ErrInvalidOrder, line.ProductID)
		}
		seen[line.ProductID] = struct{}{}
	}
	return nil
}

func uniqueProductIDs(lines []LineRequest) []string {
	ids := make([]string, 0, len(lines))
	for _, line := range lines {
		ids = append(ids, line.ProductID)
	}
	slices.Sort(ids)
	return ids
}

func scanProduct(rows *sql.Rows) (Product, error) {
	var product Product
	if err := rows.Scan(&product.ID, &product.Name, &product.Stock, &product.PriceCents); err != nil {
		return Product{}, err
	}
	return product, nil
}

func snapshot(name string, stmt sqlkit.Statement, err error) (StatementSnapshot, error) {
	if err != nil {
		return StatementSnapshot{}, err
	}
	return StatementSnapshot{Name: name, SQL: stmt.SQL, Args: stmt.Args}, nil
}

func previewTime() time.Time {
	return time.Date(2026, 6, 28, 10, 30, 0, 0, time.UTC)
}

func join(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += sep + value
	}
	return result
}
