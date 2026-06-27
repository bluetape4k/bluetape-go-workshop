// Package orderrepo demonstrates a small sqlkit-backed order repository.
package orderrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const ordersTable = "sql_order_repository_orders"

// ErrInvalidOrder reports missing or invalid order input.
var ErrInvalidOrder = errors.New("orderrepo: invalid order")

// Status is the narrow order lifecycle state used by this repository example.
type Status string

const (
	// StatusPending means the order was accepted but not paid.
	StatusPending Status = "pending"
	// StatusPaid means payment authorization succeeded.
	StatusPaid Status = "paid"
	// StatusCancelled means the order was cancelled.
	StatusCancelled Status = "cancelled"
)

// Order is the domain object persisted by Repository.
type Order struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Status     Status    `json:"status"`
	TotalCents int64     `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

// Filter scopes repository list queries without hiding SQL semantics.
type Filter struct {
	CustomerID string
	Status     Status
	Limit      int
}

// StatementSnapshot is an inspectable SQL statement plus ordered arguments.
type StatementSnapshot struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// Preview describes the runnable repository lesson printed by main.
type Preview struct {
	Scenario    string              `json:"scenario"`
	Repository  string              `json:"repository"`
	Statements  []StatementSnapshot `json:"statements"`
	Behavior    []string            `json:"behavior"`
	DirectSQL   []string            `json:"direct_database_sql_boundary"`
	Production  []string            `json:"production_hardening"`
	TestCommand string              `json:"test_command"`
}

// Repository owns order SQL while the application owns the database session.
type Repository struct{}

// NewPreview builds a human-readable snapshot of the repository contract.
func NewPreview(sample Order) (Preview, error) {
	repo := Repository{}
	create, err := repo.CreateStatement(sample)
	if err != nil {
		return Preview{}, err
	}
	find, err := repo.FindStatement(sample.ID)
	if err != nil {
		return Preview{}, err
	}
	list, err := repo.ListStatement(Filter{CustomerID: sample.CustomerID, Status: sample.Status, Limit: 10})
	if err != nil {
		return Preview{}, err
	}

	return Preview{
		Scenario:   "Persist a customer order, read it by primary key, and list customer orders by status.",
		Repository: "Repository methods accept context plus caller-owned *sql.DB or *sql.Tx through sqlkit interfaces.",
		Statements: []StatementSnapshot{create, find, list},
		Behavior: []string{
			"Create validates the domain order and executes an inspectable INSERT.",
			"FindByID uses sqlkit.QueryOne, so missing rows return sqlkit.ErrNoRows.",
			"List uses sqlkit.QueryAll and returns domain rows ordered by created_at and id.",
			"The tests assert behavior through a real PostgreSQL database, not string-only snapshots.",
		},
		DirectSQL: []string{
			"database/sql still owns connections, pooling, context cancellation, and transactions.",
			"sqlkit only builds visible SQL and maps row cardinality; it is not an ORM or migration tool.",
			"Direct database/sql remains a good fit for one-off queries, driver-specific behavior, or explicit scanning lessons.",
		},
		Production: []string{
			"pick a migration owner before repository deployment",
			"keep transaction ownership at the service boundary",
			"add optimistic versioning before concurrent status updates",
			"log SQL shape and correlation IDs without leaking customer data",
			"add pagination contracts before exposing list endpoints",
		},
		TestCommand: "go test -count=1 ./examples/sql-order-repository/...",
	}, nil
}

// Create inserts an order through the provided database/sql execution boundary.
func (repo Repository) Create(ctx context.Context, db sqlkit.Execer, order Order) error {
	stmt, err := repo.createSQL(order)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// FindByID returns exactly one order or sqlkit.ErrNoRows.
func (repo Repository) FindByID(ctx context.Context, db sqlkit.Queryer, id string) (Order, error) {
	stmt, err := repo.findSQL(id)
	if err != nil {
		return Order{}, err
	}
	return sqlkit.QueryOne(ctx, db, stmt.SQL, scanOrder, stmt.Args...)
}

// List returns orders that match the optional customer/status filter.
func (repo Repository) List(ctx context.Context, db sqlkit.Queryer, filter Filter) ([]Order, error) {
	stmt, err := repo.listSQL(filter)
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanOrder, stmt.Args...)
}

// CreateStatement returns the create SQL shown by the README and preview.
func (repo Repository) CreateStatement(order Order) (StatementSnapshot, error) {
	stmt, err := repo.createSQL(order)
	return snapshot("orders.create", stmt, err)
}

// FindStatement returns the find SQL shown by the README and preview.
func (repo Repository) FindStatement(id string) (StatementSnapshot, error) {
	stmt, err := repo.findSQL(id)
	return snapshot("orders.find_by_id", stmt, err)
}

// ListStatement returns the list SQL shown by the README and preview.
func (repo Repository) ListStatement(filter Filter) (StatementSnapshot, error) {
	stmt, err := repo.listSQL(filter)
	return snapshot("orders.list", stmt, err)
}

func (Repository) createSQL(order Order) (sqlkit.Statement, error) {
	if err := validateOrder(order); err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.InsertInto(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at").
		Values(order.ID, order.CustomerID, string(order.Status), order.TotalCents, order.CreatedAt.UTC()).
		Build()
}

func (Repository) findSQL(id string) (sqlkit.Statement, error) {
	if id == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: id is required", ErrInvalidOrder)
	}
	return sqlkit.SelectFrom(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at").
		Where("id = ?", id).
		Build()
}

func (Repository) listSQL(filter Filter) (sqlkit.Statement, error) {
	if filter.Limit < 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: limit must not be negative", ErrInvalidOrder)
	}
	if filter.CustomerID == "" && filter.Status == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: customer_id or status filter is required", ErrInvalidOrder)
	}

	builder := sqlkit.SelectFrom(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at").
		OrderBy("created_at", "id")
	if filter.CustomerID != "" {
		builder.Where("customer_id = ?", filter.CustomerID)
	}
	if filter.Status != "" {
		builder.Where("status = ?", string(filter.Status))
	}
	if filter.Limit > 0 {
		builder.Limit(filter.Limit)
	}
	return builder.Build()
}

func validateOrder(order Order) error {
	switch {
	case order.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidOrder)
	case order.CustomerID == "":
		return fmt.Errorf("%w: customer_id is required", ErrInvalidOrder)
	case order.Status == "":
		return fmt.Errorf("%w: status is required", ErrInvalidOrder)
	case order.TotalCents < 0:
		return fmt.Errorf("%w: total_cents must not be negative", ErrInvalidOrder)
	case order.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	default:
		return nil
	}
}

func scanOrder(rows *sql.Rows) (Order, error) {
	var order Order
	var status string
	if err := rows.Scan(&order.ID, &order.CustomerID, &status, &order.TotalCents, &order.CreatedAt); err != nil {
		return Order{}, err
	}
	order.Status = Status(status)
	order.CreatedAt = order.CreatedAt.UTC()
	return order, nil
}

func snapshot(name string, stmt sqlkit.Statement, err error) (StatementSnapshot, error) {
	if err != nil {
		return StatementSnapshot{}, err
	}
	return StatementSnapshot{Name: name, SQL: stmt.SQL, Args: stmt.Args}, nil
}
