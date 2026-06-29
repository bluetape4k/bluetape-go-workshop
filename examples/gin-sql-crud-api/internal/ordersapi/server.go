// Package ordersapi exposes a small Gin CRUD API over a sqlkit repository.
package ordersapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
	"github.com/gin-gonic/gin"
)

const (
	ordersTable           = "gin_sql_crud_orders"
	defaultRequestTimeout = 3 * time.Second
)

var (
	// ErrInvalidOrder reports invalid domain or API input.
	ErrInvalidOrder = errors.New("ordersapi: invalid order")
	// ErrOrderNotFound reports that an order row does not exist.
	ErrOrderNotFound = errors.New("ordersapi: order not found")
)

// Status is the public order state accepted by the API.
type Status string

const (
	// StatusPending means the order was accepted but is not paid yet.
	StatusPending Status = "pending"
	// StatusPaid means the order has been paid.
	StatusPaid Status = "paid"
	// StatusCancelled means the order was cancelled.
	StatusCancelled Status = "cancelled"
)

// Order is the domain row persisted by the repository and returned by the API.
type Order struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Status     Status    `json:"status"`
	TotalCents int64     `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateOrderRequest is the POST /orders request body.
type CreateOrderRequest struct {
	CustomerID string `json:"customer_id"`
	TotalCents int64  `json:"total_cents"`
}

// UpdateStatusRequest is the PATCH /orders/:id/status request body.
type UpdateStatusRequest struct {
	Status Status `json:"status"`
}

// ListFilter scopes GET /orders queries.
type ListFilter struct {
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

// Preview describes the runnable API lesson printed when DATABASE_URL is unset.
type Preview struct {
	Scenario        string              `json:"scenario"`
	HTTPBoundary    []string            `json:"http_boundary"`
	Repository      []string            `json:"repository_boundary"`
	Endpoints       []string            `json:"endpoints"`
	Statements      []StatementSnapshot `json:"statements"`
	Setup           []string            `json:"setup"`
	Production      []string            `json:"production_hardening"`
	TestCommand     string              `json:"test_command"`
	RaceTestCommand string              `json:"race_test_command"`
}

// Options configures the Gin SQL CRUD API.
type Options struct {
	DB             *sql.DB
	Now            func() time.Time
	NewID          func() (string, error)
	RequestTimeout time.Duration
}

// Server exposes order CRUD endpoints over Gin.
type Server struct {
	router         *gin.Engine
	db             *sql.DB
	repo           Repository
	now            func() time.Time
	newID          func() (string, error)
	requestTimeout time.Duration
}

// Repository owns order SQL while callers own database sessions.
type Repository struct{}

// NewServer creates the order API. Gin stays at this boundary; repository code
// only sees context, sqlkit interfaces, and database/sql.
func NewServer(options Options) (*Server, error) {
	if options.DB == nil {
		return nil, fmt.Errorf("%w: db is required", ErrInvalidOrder)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = randomOrderID
	}
	timeout := options.RequestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}

	server := &Server{
		router:         router,
		db:             options.DB,
		repo:           Repository{},
		now:            now,
		newID:          newID,
		requestTimeout: timeout,
	}
	router.GET("/healthz", server.health)
	router.POST("/orders", server.createOrder)
	router.GET("/orders", server.listOrders)
	router.GET("/orders/:id", server.getOrder)
	router.PATCH("/orders/:id/status", server.updateStatus)
	router.DELETE("/orders/:id", server.deleteOrder)

	return server, nil
}

// ServeHTTP dispatches requests to Gin.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Migrate creates the example table. Production systems should use a migration
// tool; this keeps the local demo runnable.
func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `create table if not exists gin_sql_crud_orders (
		id text primary key,
		customer_id text not null,
		status text not null,
		total_cents bigint not null check (total_cents >= 0),
		created_at timestamptz not null
	)`)
	return err
}

// NewPreview returns the API and repository contract without requiring a DB.
func NewPreview() (Preview, error) {
	repo := Repository{}
	create, err := repo.CreateStatement(Order{
		ID:         "ord_1001",
		CustomerID: "customer-42",
		Status:     StatusPending,
		TotalCents: 2599,
		CreatedAt:  previewTime(),
	})
	if err != nil {
		return Preview{}, err
	}
	find, err := repo.FindStatement("ord_1001")
	if err != nil {
		return Preview{}, err
	}
	list, err := repo.ListStatement(ListFilter{CustomerID: "customer-42", Status: StatusPending, Limit: 20})
	if err != nil {
		return Preview{}, err
	}
	update, err := repo.UpdateStatusStatement("ord_1001", StatusPaid)
	if err != nil {
		return Preview{}, err
	}
	remove, err := repo.DeleteStatement("ord_1001")
	if err != nil {
		return Preview{}, err
	}

	return Preview{
		Scenario: "Expose order create, read, list, status update, and delete over Gin while keeping SQL in a reusable repository.",
		HTTPBoundary: []string{
			"Gin parses JSON, query strings, path parameters, and public error codes.",
			"Each handler creates a request-scoped context timeout before touching the database.",
			"HTTP status codes map domain outcomes to 201, 200, 204, 400, 404, and 500 responses.",
		},
		Repository: []string{
			"Repository methods accept context and sqlkit Queryer/Execer interfaces, so they work with *sql.DB or *sql.Tx.",
			"sqlkit keeps INSERT, SELECT, UPDATE, and DELETE statements inspectable without becoming an ORM.",
			"The repository has no Gin dependency and can move behind a service boundary later.",
		},
		Endpoints: []string{
			"GET /healthz",
			"POST /orders",
			"GET /orders/{id}",
			"GET /orders?customer_id=customer-42&status=pending&limit=20",
			"PATCH /orders/{id}/status",
			"DELETE /orders/{id}",
		},
		Statements: []StatementSnapshot{create, find, list, update, remove},
		Setup: []string{
			"Run against PostgreSQL by setting DATABASE_URL.",
			"The example creates its table with Migrate for local use; use a migration tool in production.",
			"Focused tests use bluetape-go PostgreSQL Testcontainers fixtures.",
		},
		Production: []string{
			"add authentication and authorization before exposing customer order data",
			"replace offset-like list limits with a stable pagination contract",
			"add optimistic versioning before concurrent status updates",
			"move schema changes into a migration owner outside the HTTP process",
			"emit metrics for handler latency, SQL latency, not-found rates, and validation errors",
		},
		TestCommand:     "go test -count=1 ./examples/gin-sql-crud-api/...",
		RaceTestCommand: "go test -race -count=1 ./examples/gin-sql-crud-api/...",
	}, nil
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) createOrder(c *gin.Context) {
	var request CreateOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	orderID, err := s.newID()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "id_generation_failed", err)
		return
	}
	order := Order{
		ID:         orderID,
		CustomerID: strings.TrimSpace(request.CustomerID),
		Status:     StatusPending,
		TotalCents: request.TotalCents,
		CreatedAt:  s.now().UTC(),
	}

	ctx, cancel := s.requestContext(c)
	defer cancel()
	if err := s.repo.Create(ctx, s.db, order); err != nil {
		writeRepositoryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (s *Server) getOrder(c *gin.Context) {
	ctx, cancel := s.requestContext(c)
	defer cancel()

	order, err := s.repo.FindByID(ctx, s.db, c.Param("id"))
	if err != nil {
		writeRepositoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

func (s *Server) listOrders(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	ctx, cancel := s.requestContext(c)
	defer cancel()

	orders, err := s.repo.List(ctx, s.db, filter)
	if err != nil {
		writeRepositoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders, "count": len(orders)})
}

func (s *Server) updateStatus(c *gin.Context) {
	var request UpdateStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	ctx, cancel := s.requestContext(c)
	defer cancel()

	order, err := s.repo.UpdateStatus(ctx, s.db, c.Param("id"), request.Status)
	if err != nil {
		writeRepositoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

func (s *Server) deleteOrder(c *gin.Context) {
	ctx, cancel := s.requestContext(c)
	defer cancel()

	if err := s.repo.Delete(ctx, s.db, c.Param("id")); err != nil {
		writeRepositoryError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) requestContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), s.requestTimeout)
}

// Create inserts an order through the provided database/sql execution boundary.
func (Repository) Create(ctx context.Context, db sqlkit.Execer, order Order) error {
	stmt, err := createSQL(order)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// FindByID returns one order or ErrOrderNotFound.
func (Repository) FindByID(ctx context.Context, db sqlkit.Queryer, id string) (Order, error) {
	stmt, err := findSQL(id)
	if err != nil {
		return Order{}, err
	}
	order, err := sqlkit.QueryOne(ctx, db, stmt.SQL, scanOrder, stmt.Args...)
	if errors.Is(err, sqlkit.ErrNoRows) {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderNotFound, id)
	}
	return order, err
}

// List returns orders matching optional customer/status filters.
func (Repository) List(ctx context.Context, db sqlkit.Queryer, filter ListFilter) ([]Order, error) {
	stmt, err := listSQL(filter)
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanOrder, stmt.Args...)
}

// UpdateStatus updates one order status and returns the fresh row.
func (repo Repository) UpdateStatus(ctx context.Context, db sqlkit.Execer, id string, status Status) (Order, error) {
	stmt, err := updateStatusSQL(id, status)
	if err != nil {
		return Order{}, err
	}
	result, err := stmt.Exec(ctx, db)
	if err != nil {
		return Order{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderNotFound, id)
	}
	queryer, ok := db.(sqlkit.Queryer)
	if !ok {
		return Order{}, fmt.Errorf("%w: update requires a query-capable database handle", ErrInvalidOrder)
	}
	return repo.FindByID(ctx, queryer, id)
}

// Delete removes one order.
func (Repository) Delete(ctx context.Context, db sqlkit.Execer, id string) error {
	stmt, err := deleteSQL(id)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(ctx, db)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("%w: %s", ErrOrderNotFound, id)
	}
	return nil
}

// CreateStatement returns the create SQL shown by the README and preview.
func (Repository) CreateStatement(order Order) (StatementSnapshot, error) {
	stmt, err := createSQL(order)
	return snapshot("orders.create", stmt, err)
}

// FindStatement returns the find SQL shown by the README and preview.
func (Repository) FindStatement(id string) (StatementSnapshot, error) {
	stmt, err := findSQL(id)
	return snapshot("orders.find_by_id", stmt, err)
}

// ListStatement returns the list SQL shown by the README and preview.
func (Repository) ListStatement(filter ListFilter) (StatementSnapshot, error) {
	stmt, err := listSQL(filter)
	return snapshot("orders.list", stmt, err)
}

// UpdateStatusStatement returns the update SQL shown by the README and preview.
func (Repository) UpdateStatusStatement(id string, status Status) (StatementSnapshot, error) {
	stmt, err := updateStatusSQL(id, status)
	return snapshot("orders.update_status", stmt, err)
}

// DeleteStatement returns the delete SQL shown by the README and preview.
func (Repository) DeleteStatement(id string) (StatementSnapshot, error) {
	stmt, err := deleteSQL(id)
	return snapshot("orders.delete", stmt, err)
}

func createSQL(order Order) (sqlkit.Statement, error) {
	if err := validateOrder(order); err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.InsertInto(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at").
		Values(order.ID, order.CustomerID, string(order.Status), order.TotalCents, order.CreatedAt.UTC()).
		Build()
}

func findSQL(id string) (sqlkit.Statement, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: id is required", ErrInvalidOrder)
	}
	return sqlkit.SelectFrom(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at").
		Where("id = ?", id).
		Build()
}

func listSQL(filter ListFilter) (sqlkit.Statement, error) {
	filter.CustomerID = strings.TrimSpace(filter.CustomerID)
	if filter.Limit < 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: limit must not be negative", ErrInvalidOrder)
	}
	if filter.CustomerID == "" && filter.Status == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: customer_id or status is required", ErrInvalidOrder)
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return sqlkit.Statement{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidOrder, filter.Status)
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

func updateStatusSQL(id string, status Status) (sqlkit.Statement, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: id is required", ErrInvalidOrder)
	}
	if !status.Valid() {
		return sqlkit.Statement{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidOrder, status)
	}
	return sqlkit.Update(ordersTable).
		Set("status", string(status)).
		Where("id = ?", id).
		Build()
}

func deleteSQL(id string) (sqlkit.Statement, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: id is required", ErrInvalidOrder)
	}
	return sqlkit.DeleteFrom(ordersTable).
		Where("id = ?", id).
		Build()
}

func validateOrder(order Order) error {
	order.ID = strings.TrimSpace(order.ID)
	order.CustomerID = strings.TrimSpace(order.CustomerID)
	switch {
	case order.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidOrder)
	case order.CustomerID == "":
		return fmt.Errorf("%w: customer_id is required", ErrInvalidOrder)
	case !order.Status.Valid():
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidOrder, order.Status)
	case order.TotalCents < 0:
		return fmt.Errorf("%w: total_cents must not be negative", ErrInvalidOrder)
	case order.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	default:
		return nil
	}
}

// Valid reports whether the status is accepted by this example API.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusPaid, StatusCancelled:
		return true
	default:
		return false
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

func parseListFilter(c *gin.Context) (ListFilter, error) {
	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, fmt.Errorf("%w: limit must be an integer", ErrInvalidOrder)
		}
		limit = parsed
	}
	if limit <= 0 || limit > 100 {
		return ListFilter{}, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidOrder)
	}
	filter := ListFilter{
		CustomerID: strings.TrimSpace(c.Query("customer_id")),
		Status:     Status(strings.TrimSpace(c.Query("status"))),
		Limit:      limit,
	}
	if filter.CustomerID == "" && filter.Status == "" {
		return ListFilter{}, fmt.Errorf("%w: customer_id or status query is required", ErrInvalidOrder)
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return ListFilter{}, fmt.Errorf("%w: unsupported status %q", ErrInvalidOrder, filter.Status)
	}
	return filter, nil
}

func writeRepositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidOrder):
		writeError(c, http.StatusBadRequest, "invalid_request", err)
	case errors.Is(err, ErrOrderNotFound):
		writeError(c, http.StatusNotFound, "order_not_found", err)
	default:
		writeError(c, http.StatusInternalServerError, "repository_error", err)
	}
}

func writeError(c *gin.Context, status int, code string, err error) {
	c.JSON(status, gin.H{"code": code, "error": err.Error()})
}

func snapshot(name string, stmt sqlkit.Statement, err error) (StatementSnapshot, error) {
	if err != nil {
		return StatementSnapshot{}, err
	}
	return StatementSnapshot{Name: name, SQL: stmt.SQL, Args: stmt.Args}, nil
}

func randomOrderID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "ord_" + hex.EncodeToString(buf[:]), nil
}

func previewTime() time.Time {
	return time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
}
