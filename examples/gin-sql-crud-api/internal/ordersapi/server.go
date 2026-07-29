// Package ordersapi 는 sqlkit repository 위에 작은 Gin CRUD API를 노출한다.
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
	// ErrInvalidOrder 는 유효하지 않은 domain 또는 API 입력을 나타낸다.
	ErrInvalidOrder = errors.New("ordersapi: invalid order")
	// ErrOrderNotFound 는 order row가 존재하지 않음을 나타낸다.
	ErrOrderNotFound = errors.New("ordersapi: order not found")
)

// Status 는 API가 허용하는 공개 order state다.
type Status string

const (
	// StatusPending 은 주문이 접수되었지만 아직 결제되지 않았다는 뜻이다.
	StatusPending Status = "pending"
	// StatusPaid 는 주문 결제가 완료되었다는 뜻이다.
	StatusPaid Status = "paid"
	// StatusCancelled 는 주문이 취소되었다는 뜻이다.
	StatusCancelled Status = "cancelled"
)

// Order 는 repository가 영속화하고 API가 반환하는 domain row다.
type Order struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Status     Status    `json:"status"`
	TotalCents int64     `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateOrderRequest 는 POST /orders 요청 본문이다.
type CreateOrderRequest struct {
	CustomerID string `json:"customer_id"`
	TotalCents int64  `json:"total_cents"`
}

// UpdateStatusRequest 는 PATCH /orders/:id/status 요청 본문이다.
type UpdateStatusRequest struct {
	Status Status `json:"status"`
}

// ListFilter 는 GET /orders query 범위를 제한한다.
type ListFilter struct {
	CustomerID string
	Status     Status
	Limit      int
}

// StatementSnapshot 은 검사 가능한 SQL statement와 순서가 있는 argument 묶음이다.
type StatementSnapshot struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// Preview 는 DATABASE_URL이 없을 때 출력되는 실행 가능한 API lesson을 설명한다.
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

// Options 는 Gin SQL CRUD API를 설정한다.
type Options struct {
	DB             *sql.DB
	Now            func() time.Time
	NewID          func() (string, error)
	RequestTimeout time.Duration
}

// Server 는 Gin 위에 order CRUD endpoint를 노출한다.
type Server struct {
	router         *gin.Engine
	db             *sql.DB
	repo           Repository
	now            func() time.Time
	newID          func() (string, error)
	requestTimeout time.Duration
}

// Repository 는 order SQL을 소유하고 호출자는 database session을 소유한다.
type Repository struct{}

// NewServer 는 order API를 만든다. Gin은 이 boundary에 머물고 repository code는
// context, sqlkit interface, database/sql만 본다.
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

// ServeHTTP 는 요청을 Gin으로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Migrate 는 예제 table을 만든다. Production system은 migration tool을 사용해야 하며,
// 이 함수는 local demo를 실행 가능하게 유지하기 위한 것이다.
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

// NewPreview 는 DB 없이 API와 repository 계약을 반환한다.
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

// Create 는 제공된 database/sql 실행 boundary를 통해 order를 삽입한다.
func (Repository) Create(ctx context.Context, db sqlkit.Execer, order Order) error {
	stmt, err := createSQL(order)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// FindByID 는 order 하나를 반환하거나 ErrOrderNotFound를 반환한다.
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

// List 는 선택적 customer/status filter와 일치하는 order를 반환한다.
func (Repository) List(ctx context.Context, db sqlkit.Queryer, filter ListFilter) ([]Order, error) {
	stmt, err := listSQL(filter)
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanOrder, stmt.Args...)
}

// UpdateStatus 는 order status 하나를 갱신하고 최신 row를 반환한다.
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

// Delete 는 order 하나를 제거한다.
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

// CreateStatement 는 README와 preview에 표시되는 create SQL을 반환한다.
func (Repository) CreateStatement(order Order) (StatementSnapshot, error) {
	stmt, err := createSQL(order)
	return snapshot("orders.create", stmt, err)
}

// FindStatement 는 README와 preview에 표시되는 find SQL을 반환한다.
func (Repository) FindStatement(id string) (StatementSnapshot, error) {
	stmt, err := findSQL(id)
	return snapshot("orders.find_by_id", stmt, err)
}

// ListStatement 는 README와 preview에 표시되는 list SQL을 반환한다.
func (Repository) ListStatement(filter ListFilter) (StatementSnapshot, error) {
	stmt, err := listSQL(filter)
	return snapshot("orders.list", stmt, err)
}

// UpdateStatusStatement 는 README와 preview에 표시되는 update SQL을 반환한다.
func (Repository) UpdateStatusStatement(id string, status Status) (StatementSnapshot, error) {
	stmt, err := updateStatusSQL(id, status)
	return snapshot("orders.update_status", stmt, err)
}

// DeleteStatement 는 README와 preview에 표시되는 delete SQL을 반환한다.
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

// Valid 는 이 예제 API가 status를 허용하는지 보고한다.
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
