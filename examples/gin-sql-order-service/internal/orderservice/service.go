// Package orderservice 는 Gin handler, service-owned SQL
// transaction, sqlkit repository를 작은 order workflow로 통합한다.
package orderservice

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
	"github.com/gin-gonic/gin"
)

const (
	ordersTable           = "gin_sql_order_service_orders"
	itemsTable            = "gin_sql_order_service_items"
	statusEventsTable     = "gin_sql_order_service_status_events"
	defaultRequestTimeout = 3 * time.Second
)

var (
	// ErrInvalidOrder 는 유효하지 않은 API 또는 domain 입력을 나타낸다.
	ErrInvalidOrder = errors.New("orderservice: invalid order")
	// ErrOrderNotFound 는 order row가 존재하지 않음을 나타낸다.
	ErrOrderNotFound = errors.New("orderservice: order not found")
	// ErrItemNotFound 는 order에 해당하는 item row가 존재하지 않음을 나타낸다.
	ErrItemNotFound = errors.New("orderservice: item not found")
	// ErrOrderRejected 는 rollback 증명에 쓰는 결정적인 워크숍 fault다.
	ErrOrderRejected = errors.New("orderservice: order rejected")
)

// Status 는 이 integration 예제에서 사용하는 좁은 order service state다.
type Status string

const (
	// StatusPending 은 order가 존재하며 아직 수정 가능하다는 뜻이다.
	StatusPending Status = "pending"
	// StatusConfirmed 는 order가 확정되었다는 뜻이다.
	StatusConfirmed Status = "confirmed"
	// StatusCancelled 는 order가 취소되었다는 뜻이다.
	StatusCancelled Status = "cancelled"
)

// Order 는 service가 영속화하는 order header다.
type Order struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Status     Status    `json:"status"`
	TotalCents int64     `json:"total_cents"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OrderItem 은 영속화된 order item row 하나다.
type OrderItem struct {
	ID             string `json:"id"`
	OrderID        string `json:"order_id"`
	SKU            string `json:"sku"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	LineTotalCents int64  `json:"line_total_cents"`
}

// StatusEvent 는 order status가 변경된 이유를 기록한다.
type StatusEvent struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	Status    Status    `json:"status"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// OrderView 는 order aggregate용 HTTP response shape다.
type OrderView struct {
	Order         Order         `json:"order"`
	Items         []OrderItem   `json:"items"`
	StatusHistory []StatusEvent `json:"status_history"`
}

// CreateOrderRequest 는 POST /orders 요청 본문이다.
type CreateOrderRequest struct {
	CustomerID       string              `json:"customer_id"`
	Items            []CreateItemRequest `json:"items"`
	RejectAfterItems bool                `json:"reject_after_items,omitempty"`
}

// CreateItemRequest 는 create-order 요청 안의 item 하나다.
type CreateItemRequest struct {
	SKU            string `json:"sku"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

// UpdateItemRequest 는 PATCH /orders/:id/items/:item_id 요청 본문이다.
type UpdateItemRequest struct {
	Quantity int `json:"quantity"`
}

// StatusResponse 는 GET /orders/:id/status 응답 본문이다.
type StatusResponse struct {
	OrderID string        `json:"order_id"`
	Status  Status        `json:"status"`
	History []StatusEvent `json:"history"`
}

// StatementSnapshot 은 검사 가능한 SQL statement와 순서가 있는 argument 묶음이다.
type StatementSnapshot struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// Preview 는 통합 order service lesson을 설명한다.
type Preview struct {
	Scenario        string              `json:"scenario"`
	BuildsOn        []string            `json:"builds_on"`
	HTTPBoundary    []string            `json:"http_boundary"`
	ServiceBoundary []string            `json:"service_boundary"`
	Repositories    []string            `json:"repository_boundary"`
	Endpoints       []string            `json:"endpoints"`
	Statements      []StatementSnapshot `json:"statements"`
	Production      []string            `json:"production_hardening"`
	TestCommand     string              `json:"test_command"`
	RaceTestCommand string              `json:"race_test_command"`
}

// Options 는 Gin SQL order service API를 설정한다.
type Options struct {
	DB             *sql.DB
	Now            func() time.Time
	NewID          func(prefix string) (string, error)
	RequestTimeout time.Duration
}

// Server 는 Gin 위에 order service를 노출한다.
type Server struct {
	router         *gin.Engine
	db             *sql.DB
	service        Service
	requestTimeout time.Duration
}

// Service 는 transaction lifetime을 소유하고 repository들을 조율한다.
type Service struct {
	orders   OrderRepository
	items    ItemRepository
	statuses StatusRepository
	now      func() time.Time
	newID    func(prefix string) (string, error)
}

// OrderRepository 는 order header SQL을 소유한다.
type OrderRepository struct{}

// ItemRepository 는 order item SQL을 소유한다.
type ItemRepository struct{}

// StatusRepository 는 status history SQL을 소유한다.
type StatusRepository struct{}

// NewServer 는 Gin order service API를 만든다.
func NewServer(options Options) (*Server, error) {
	if options.DB == nil {
		return nil, fmt.Errorf("%w: db is required", ErrInvalidOrder)
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
		service:        NewService(options.Now, options.NewID),
		requestTimeout: timeout,
	}
	router.GET("/healthz", server.health)
	router.POST("/orders", server.createOrder)
	router.GET("/orders/:id", server.getOrder)
	router.GET("/orders/:id/status", server.getStatus)
	router.PATCH("/orders/:id/items/:item_id", server.updateItem)

	return server, nil
}

// NewService 는 service-owned transaction boundary를 가진 order service를 반환한다.
func NewService(now func() time.Time, newID func(prefix string) (string, error)) Service {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = randomID
	}
	return Service{
		orders:   OrderRepository{},
		items:    ItemRepository{},
		statuses: StatusRepository{},
		now:      now,
		newID:    newID,
	}
}

// ServeHTTP 는 요청을 Gin으로 전달한다.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Migrate 는 local example schema를 만든다.
func Migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`create table if not exists gin_sql_order_service_orders (
			id text primary key,
			customer_id text not null,
			status text not null,
			total_cents bigint not null check (total_cents >= 0),
			created_at timestamptz not null,
			updated_at timestamptz not null
		)`,
		`create table if not exists gin_sql_order_service_items (
			id text primary key,
			order_id text not null references gin_sql_order_service_orders(id) on delete cascade,
			sku text not null,
			quantity integer not null check (quantity > 0),
			unit_price_cents bigint not null check (unit_price_cents >= 0),
			line_total_cents bigint not null check (line_total_cents >= 0)
		)`,
		`create table if not exists gin_sql_order_service_status_events (
			id text primary key,
			order_id text not null references gin_sql_order_service_orders(id) on delete cascade,
			status text not null,
			reason text not null,
			created_at timestamptz not null
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// NewPreview 는 검사 가능한 API, service, SQL 계약을 반환한다.
func NewPreview() (Preview, error) {
	orders := OrderRepository{}
	items := ItemRepository{}
	statuses := StatusRepository{}
	now := previewTime()

	createOrder, err := orders.CreateStatement(Order{
		ID:         "ord_1001",
		CustomerID: "customer-42",
		Status:     StatusPending,
		TotalCents: 3700,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		return Preview{}, err
	}
	createItem, err := items.CreateStatement(OrderItem{
		ID:             "item_1001",
		OrderID:        "ord_1001",
		SKU:            "sku-coffee",
		Quantity:       2,
		UnitPriceCents: 1200,
		LineTotalCents: 2400,
	})
	if err != nil {
		return Preview{}, err
	}
	updateItem, err := items.UpdateQuantityStatement("ord_1001", "item_1001", 3, 1200)
	if err != nil {
		return Preview{}, err
	}
	updateTotal, err := orders.UpdateTotalStatement("ord_1001", 4900, now)
	if err != nil {
		return Preview{}, err
	}
	appendStatus, err := statuses.AppendStatement(StatusEvent{
		ID:        "evt_1001",
		OrderID:   "ord_1001",
		Status:    StatusPending,
		Reason:    "order accepted",
		CreatedAt: now,
	})
	if err != nil {
		return Preview{}, err
	}

	return Preview{
		Scenario: "Create and edit an order aggregate through Gin while one service boundary owns multi-table SQL transactions.",
		BuildsOn: []string{
			"#62 showed a single order repository.",
			"#63 showed service-owned transactions.",
			"#64 showed a Gin CRUD boundary.",
			"#65 composes those lessons into one order aggregate workflow.",
		},
		HTTPBoundary: []string{
			"Gin parses JSON, path parameters, and public error codes.",
			"Handlers do not build SQL or own transactions.",
			"Each handler creates a request-scoped timeout before calling the service.",
		},
		ServiceBoundary: []string{
			"CreateOrder writes order, items, and status history inside one sqlkit.WithTx call.",
			"UpdateItemQuantity updates one item, recalculates the order total, and appends status history inside one transaction.",
			"A deterministic reject_after_items flag proves rollback without adding inventory, payment, or outbox scope.",
		},
		Repositories: []string{
			"OrderRepository owns order header SQL.",
			"ItemRepository owns item row SQL and aggregate total queries.",
			"StatusRepository owns status event SQL.",
			"Repositories accept context plus sqlkit Execer/Queryer interfaces and do not import Gin.",
		},
		Endpoints: []string{
			"GET /healthz",
			"POST /orders",
			"GET /orders/{id}",
			"GET /orders/{id}/status",
			"PATCH /orders/{id}/items/{item_id}",
		},
		Statements:      []StatementSnapshot{createOrder, createItem, updateItem, updateTotal, appendStatus},
		Production:      productionNotes(),
		TestCommand:     "go test -count=1 ./examples/gin-sql-order-service/...",
		RaceTestCommand: "go test -race -count=1 ./examples/gin-sql-order-service/...",
	}, nil
}

// CreateOrder 는 order, item, status history를 원자적으로 기록한다.
func (s Service) CreateOrder(ctx context.Context, db *sql.DB, request CreateOrderRequest) (OrderView, error) {
	request.CustomerID = strings.TrimSpace(request.CustomerID)
	if err := validateCreateOrderRequest(request); err != nil {
		return OrderView{}, err
	}
	now := s.now().UTC()
	orderID, err := s.newID("ord")
	if err != nil {
		return OrderView{}, err
	}

	items := make([]OrderItem, 0, len(request.Items))
	var total int64
	for _, itemRequest := range request.Items {
		itemID, err := s.newID("item")
		if err != nil {
			return OrderView{}, err
		}
		item := OrderItem{
			ID:             itemID,
			OrderID:        orderID,
			SKU:            strings.TrimSpace(itemRequest.SKU),
			Quantity:       itemRequest.Quantity,
			UnitPriceCents: itemRequest.UnitPriceCents,
			LineTotalCents: int64(itemRequest.Quantity) * itemRequest.UnitPriceCents,
		}
		items = append(items, item)
		total += item.LineTotalCents
	}
	order := Order{
		ID:         orderID,
		CustomerID: request.CustomerID,
		Status:     StatusPending,
		TotalCents: total,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err = sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		if err := s.orders.Create(ctx, tx, order); err != nil {
			return err
		}
		for _, item := range items {
			if err := s.items.Create(ctx, tx, item); err != nil {
				return err
			}
		}
		if request.RejectAfterItems {
			return ErrOrderRejected
		}
		eventID, err := s.newID("evt")
		if err != nil {
			return err
		}
		return s.statuses.Append(ctx, tx, StatusEvent{
			ID:        eventID,
			OrderID:   order.ID,
			Status:    order.Status,
			Reason:    "order accepted",
			CreatedAt: now,
		})
	})
	if err != nil {
		return OrderView{}, err
	}
	return s.GetOrder(ctx, db, orderID)
}

// GetOrder 는 order aggregate를 반환한다.
func (s Service) GetOrder(ctx context.Context, db sqlkit.Queryer, id string) (OrderView, error) {
	order, err := s.orders.FindByID(ctx, db, id)
	if err != nil {
		return OrderView{}, err
	}
	items, err := s.items.ListByOrder(ctx, db, id)
	if err != nil {
		return OrderView{}, err
	}
	statuses, err := s.statuses.ListByOrder(ctx, db, id)
	if err != nil {
		return OrderView{}, err
	}
	return OrderView{Order: order, Items: items, StatusHistory: statuses}, nil
}

// GetStatus 는 current status와 status history를 반환한다.
func (s Service) GetStatus(ctx context.Context, db sqlkit.Queryer, id string) (StatusResponse, error) {
	order, err := s.orders.FindByID(ctx, db, id)
	if err != nil {
		return StatusResponse{}, err
	}
	history, err := s.statuses.ListByOrder(ctx, db, id)
	if err != nil {
		return StatusResponse{}, err
	}
	return StatusResponse{OrderID: order.ID, Status: order.Status, History: history}, nil
}

// UpdateItemQuantity 는 item 하나를 갱신하고 order total을 원자적으로 다시 계산한다.
func (s Service) UpdateItemQuantity(ctx context.Context, db *sql.DB, orderID, itemID string, request UpdateItemRequest) (OrderView, error) {
	if request.Quantity <= 0 {
		return OrderView{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	}
	now := s.now().UTC()
	err := sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := s.orders.FindByID(ctx, tx, orderID); err != nil {
			return err
		}
		item, err := s.items.FindByID(ctx, tx, orderID, itemID)
		if err != nil {
			return err
		}
		if err := s.items.UpdateQuantity(ctx, tx, orderID, itemID, request.Quantity, item.UnitPriceCents); err != nil {
			return err
		}
		total, err := s.items.SumOrderTotal(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if err := s.orders.UpdateTotal(ctx, tx, orderID, total, now); err != nil {
			return err
		}
		eventID, err := s.newID("evt")
		if err != nil {
			return err
		}
		return s.statuses.Append(ctx, tx, StatusEvent{
			ID:        eventID,
			OrderID:   orderID,
			Status:    StatusPending,
			Reason:    fmt.Sprintf("item %s quantity changed to %d", itemID, request.Quantity),
			CreatedAt: now,
		})
	})
	if err != nil {
		return OrderView{}, err
	}
	return s.GetOrder(ctx, db, orderID)
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
	ctx, cancel := s.requestContext(c)
	defer cancel()
	view, err := s.service.CreateOrder(ctx, s.db, request)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (s *Server) getOrder(c *gin.Context) {
	ctx, cancel := s.requestContext(c)
	defer cancel()
	view, err := s.service.GetOrder(ctx, s.db, c.Param("id"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) getStatus(c *gin.Context) {
	ctx, cancel := s.requestContext(c)
	defer cancel()
	status, err := s.service.GetStatus(ctx, s.db, c.Param("id"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) updateItem(c *gin.Context) {
	var request UpdateItemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	ctx, cancel := s.requestContext(c)
	defer cancel()
	view, err := s.service.UpdateItemQuantity(ctx, s.db, c.Param("id"), c.Param("item_id"), request)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (s *Server) requestContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), s.requestTimeout)
}

// Create 는 order header를 삽입한다.
func (OrderRepository) Create(ctx context.Context, db sqlkit.Execer, order Order) error {
	stmt, err := createOrderSQL(order)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// FindByID 는 order header를 반환하거나 ErrOrderNotFound를 반환한다.
func (OrderRepository) FindByID(ctx context.Context, db sqlkit.Queryer, id string) (Order, error) {
	stmt, err := findOrderSQL(id)
	if err != nil {
		return Order{}, err
	}
	order, err := sqlkit.QueryOne(ctx, db, stmt.SQL, scanOrder, stmt.Args...)
	if errors.Is(err, sqlkit.ErrNoRows) {
		return Order{}, fmt.Errorf("%w: %s", ErrOrderNotFound, id)
	}
	return order, err
}

// UpdateTotal 은 order aggregate total을 갱신한다.
func (OrderRepository) UpdateTotal(ctx context.Context, db sqlkit.Execer, orderID string, totalCents int64, updatedAt time.Time) error {
	stmt, err := updateOrderTotalSQL(orderID, totalCents, updatedAt)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(ctx, db)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("%w: %s", ErrOrderNotFound, orderID)
	}
	return nil
}

// CreateStatement 는 preview에 표시되는 create-order SQL을 반환한다.
func (OrderRepository) CreateStatement(order Order) (StatementSnapshot, error) {
	stmt, err := createOrderSQL(order)
	return snapshot("orders.create", stmt, err)
}

// UpdateTotalStatement 는 preview에 표시되는 update-total SQL을 반환한다.
func (OrderRepository) UpdateTotalStatement(orderID string, totalCents int64, updatedAt time.Time) (StatementSnapshot, error) {
	stmt, err := updateOrderTotalSQL(orderID, totalCents, updatedAt)
	return snapshot("orders.update_total", stmt, err)
}

// Create 는 order item을 삽입한다.
func (ItemRepository) Create(ctx context.Context, db sqlkit.Execer, item OrderItem) error {
	stmt, err := createItemSQL(item)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// FindByID 는 order 범위의 item 하나를 반환한다.
func (ItemRepository) FindByID(ctx context.Context, db sqlkit.Queryer, orderID, itemID string) (OrderItem, error) {
	stmt, err := findItemSQL(orderID, itemID)
	if err != nil {
		return OrderItem{}, err
	}
	item, err := sqlkit.QueryOne(ctx, db, stmt.SQL, scanItem, stmt.Args...)
	if errors.Is(err, sqlkit.ErrNoRows) {
		return OrderItem{}, fmt.Errorf("%w: %s", ErrItemNotFound, itemID)
	}
	return item, err
}

// ListByOrder 는 order 하나의 모든 item을 반환한다.
func (ItemRepository) ListByOrder(ctx context.Context, db sqlkit.Queryer, orderID string) ([]OrderItem, error) {
	stmt, err := listItemsSQL(orderID)
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanItem, stmt.Args...)
}

// UpdateQuantity 는 item quantity와 파생 line total을 갱신한다.
func (ItemRepository) UpdateQuantity(ctx context.Context, db sqlkit.Execer, orderID, itemID string, quantity int, unitPriceCents int64) error {
	stmt, err := updateItemQuantitySQL(orderID, itemID, quantity, unitPriceCents)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(ctx, db)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("%w: %s", ErrItemNotFound, itemID)
	}
	return nil
}

// SumOrderTotal 은 item row에서 current aggregate total을 반환한다.
func (ItemRepository) SumOrderTotal(ctx context.Context, db sqlkit.Queryer, orderID string) (int64, error) {
	stmt, err := sumItemsSQL(orderID)
	if err != nil {
		return 0, err
	}
	return sqlkit.QueryOne(ctx, db, stmt.SQL, func(rows *sql.Rows) (int64, error) {
		var total int64
		return total, rows.Scan(&total)
	}, stmt.Args...)
}

// CreateStatement 는 preview에 표시되는 create-item SQL을 반환한다.
func (ItemRepository) CreateStatement(item OrderItem) (StatementSnapshot, error) {
	stmt, err := createItemSQL(item)
	return snapshot("items.create", stmt, err)
}

// UpdateQuantityStatement 는 preview에 표시되는 update-item SQL을 반환한다.
func (ItemRepository) UpdateQuantityStatement(orderID, itemID string, quantity int, unitPriceCents int64) (StatementSnapshot, error) {
	stmt, err := updateItemQuantitySQL(orderID, itemID, quantity, unitPriceCents)
	return snapshot("items.update_quantity", stmt, err)
}

// Append 는 status event를 삽입한다.
func (StatusRepository) Append(ctx context.Context, db sqlkit.Execer, event StatusEvent) error {
	stmt, err := appendStatusSQL(event)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// ListByOrder 는 status event를 시간순으로 반환한다.
func (StatusRepository) ListByOrder(ctx context.Context, db sqlkit.Queryer, orderID string) ([]StatusEvent, error) {
	stmt, err := listStatusesSQL(orderID)
	if err != nil {
		return nil, err
	}
	return sqlkit.QueryAll(ctx, db, stmt.SQL, scanStatus, stmt.Args...)
}

// AppendStatement 는 preview에 표시되는 append-status SQL을 반환한다.
func (StatusRepository) AppendStatement(event StatusEvent) (StatementSnapshot, error) {
	stmt, err := appendStatusSQL(event)
	return snapshot("statuses.append", stmt, err)
}

func createOrderSQL(order Order) (sqlkit.Statement, error) {
	if err := validateOrder(order); err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.InsertInto(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at", "updated_at").
		Values(order.ID, order.CustomerID, string(order.Status), order.TotalCents, order.CreatedAt.UTC(), order.UpdatedAt.UTC()).
		Build()
}

func findOrderSQL(id string) (sqlkit.Statement, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: id is required", ErrInvalidOrder)
	}
	return sqlkit.SelectFrom(ordersTable).
		Columns("id", "customer_id", "status", "total_cents", "created_at", "updated_at").
		Where("id = ?", id).
		Build()
}

func updateOrderTotalSQL(orderID string, totalCents int64, updatedAt time.Time) (sqlkit.Statement, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if totalCents < 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: total_cents must not be negative", ErrInvalidOrder)
	}
	if updatedAt.IsZero() {
		return sqlkit.Statement{}, fmt.Errorf("%w: updated_at is required", ErrInvalidOrder)
	}
	return sqlkit.Update(ordersTable).
		Set("total_cents", totalCents).
		Set("updated_at", updatedAt.UTC()).
		Where("id = ?", orderID).
		Build()
}

func createItemSQL(item OrderItem) (sqlkit.Statement, error) {
	if err := validateItem(item); err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.InsertInto(itemsTable).
		Columns("id", "order_id", "sku", "quantity", "unit_price_cents", "line_total_cents").
		Values(item.ID, item.OrderID, item.SKU, item.Quantity, item.UnitPriceCents, item.LineTotalCents).
		Build()
}

func findItemSQL(orderID, itemID string) (sqlkit.Statement, error) {
	orderID = strings.TrimSpace(orderID)
	itemID = strings.TrimSpace(itemID)
	if orderID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if itemID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: item id is required", ErrInvalidOrder)
	}
	return sqlkit.SelectFrom(itemsTable).
		Columns("id", "order_id", "sku", "quantity", "unit_price_cents", "line_total_cents").
		Where("order_id = ?", orderID).
		Where("id = ?", itemID).
		Build()
}

func listItemsSQL(orderID string) (sqlkit.Statement, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	return sqlkit.SelectFrom(itemsTable).
		Columns("id", "order_id", "sku", "quantity", "unit_price_cents", "line_total_cents").
		Where("order_id = ?", orderID).
		OrderBy("id").
		Build()
}

func updateItemQuantitySQL(orderID, itemID string, quantity int, unitPriceCents int64) (sqlkit.Statement, error) {
	orderID = strings.TrimSpace(orderID)
	itemID = strings.TrimSpace(itemID)
	if orderID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if itemID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: item id is required", ErrInvalidOrder)
	}
	if quantity <= 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	}
	if unitPriceCents < 0 {
		return sqlkit.Statement{}, fmt.Errorf("%w: unit_price_cents must not be negative", ErrInvalidOrder)
	}
	return sqlkit.Update(itemsTable).
		Set("quantity", quantity).
		Set("line_total_cents", int64(quantity)*unitPriceCents).
		Where("order_id = ?", orderID).
		Where("id = ?", itemID).
		Build()
}

func sumItemsSQL(orderID string) (sqlkit.Statement, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	return sqlkit.NewStatement(
		`select coalesce(sum(line_total_cents), 0) from gin_sql_order_service_items where order_id = $1`,
		orderID,
	), nil
}

func appendStatusSQL(event StatusEvent) (sqlkit.Statement, error) {
	if err := validateStatusEvent(event); err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.InsertInto(statusEventsTable).
		Columns("id", "order_id", "status", "reason", "created_at").
		Values(event.ID, event.OrderID, string(event.Status), event.Reason, event.CreatedAt.UTC()).
		Build()
}

func listStatusesSQL(orderID string) (sqlkit.Statement, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return sqlkit.Statement{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	return sqlkit.SelectFrom(statusEventsTable).
		Columns("id", "order_id", "status", "reason", "created_at").
		Where("order_id = ?", orderID).
		OrderBy("created_at", "id").
		Build()
}

func validateCreateOrderRequest(request CreateOrderRequest) error {
	if strings.TrimSpace(request.CustomerID) == "" {
		return fmt.Errorf("%w: customer_id is required", ErrInvalidOrder)
	}
	if len(request.Items) == 0 {
		return fmt.Errorf("%w: at least one item is required", ErrInvalidOrder)
	}
	for i, item := range request.Items {
		if strings.TrimSpace(item.SKU) == "" {
			return fmt.Errorf("%w: item %d sku is required", ErrInvalidOrder, i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("%w: item %d quantity must be positive", ErrInvalidOrder, i)
		}
		if item.UnitPriceCents < 0 {
			return fmt.Errorf("%w: item %d unit_price_cents must not be negative", ErrInvalidOrder, i)
		}
	}
	return nil
}

func validateOrder(order Order) error {
	switch {
	case strings.TrimSpace(order.ID) == "":
		return fmt.Errorf("%w: id is required", ErrInvalidOrder)
	case strings.TrimSpace(order.CustomerID) == "":
		return fmt.Errorf("%w: customer_id is required", ErrInvalidOrder)
	case !order.Status.Valid():
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidOrder, order.Status)
	case order.TotalCents < 0:
		return fmt.Errorf("%w: total_cents must not be negative", ErrInvalidOrder)
	case order.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	case order.UpdatedAt.IsZero():
		return fmt.Errorf("%w: updated_at is required", ErrInvalidOrder)
	default:
		return nil
	}
}

func validateItem(item OrderItem) error {
	switch {
	case strings.TrimSpace(item.ID) == "":
		return fmt.Errorf("%w: item id is required", ErrInvalidOrder)
	case strings.TrimSpace(item.OrderID) == "":
		return fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	case strings.TrimSpace(item.SKU) == "":
		return fmt.Errorf("%w: sku is required", ErrInvalidOrder)
	case item.Quantity <= 0:
		return fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	case item.UnitPriceCents < 0:
		return fmt.Errorf("%w: unit_price_cents must not be negative", ErrInvalidOrder)
	case item.LineTotalCents != int64(item.Quantity)*item.UnitPriceCents:
		return fmt.Errorf("%w: line_total_cents does not match quantity and unit price", ErrInvalidOrder)
	default:
		return nil
	}
}

func validateStatusEvent(event StatusEvent) error {
	switch {
	case strings.TrimSpace(event.ID) == "":
		return fmt.Errorf("%w: event id is required", ErrInvalidOrder)
	case strings.TrimSpace(event.OrderID) == "":
		return fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	case !event.Status.Valid():
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidOrder, event.Status)
	case strings.TrimSpace(event.Reason) == "":
		return fmt.Errorf("%w: reason is required", ErrInvalidOrder)
	case event.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	default:
		return nil
	}
}

// Valid 는 이 예제 service가 status를 허용하는지 보고한다.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusConfirmed, StatusCancelled:
		return true
	default:
		return false
	}
}

func scanOrder(rows *sql.Rows) (Order, error) {
	var order Order
	var status string
	if err := rows.Scan(&order.ID, &order.CustomerID, &status, &order.TotalCents, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return Order{}, err
	}
	order.Status = Status(status)
	order.CreatedAt = order.CreatedAt.UTC()
	order.UpdatedAt = order.UpdatedAt.UTC()
	return order, nil
}

func scanItem(rows *sql.Rows) (OrderItem, error) {
	var item OrderItem
	err := rows.Scan(&item.ID, &item.OrderID, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents)
	return item, err
}

func scanStatus(rows *sql.Rows) (StatusEvent, error) {
	var event StatusEvent
	var status string
	if err := rows.Scan(&event.ID, &event.OrderID, &status, &event.Reason, &event.CreatedAt); err != nil {
		return StatusEvent{}, err
	}
	event.Status = Status(status)
	event.CreatedAt = event.CreatedAt.UTC()
	return event, nil
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidOrder):
		writeError(c, http.StatusBadRequest, "invalid_request", err)
	case errors.Is(err, ErrOrderNotFound):
		writeError(c, http.StatusNotFound, "order_not_found", err)
	case errors.Is(err, ErrItemNotFound):
		writeError(c, http.StatusNotFound, "item_not_found", err)
	case errors.Is(err, ErrOrderRejected):
		writeError(c, http.StatusConflict, "order_rejected", err)
	default:
		writeError(c, http.StatusInternalServerError, "service_error", err)
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

func randomID(prefix string) (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf[:]), nil
}

func previewTime() time.Time {
	return time.Date(2026, 6, 29, 11, 0, 0, 0, time.UTC)
}

func productionNotes() []string {
	return []string{
		"add authentication and authorization before exposing customer order data",
		"move schema changes into a migration owner outside the HTTP process",
		"add optimistic versioning before concurrent item edits",
		"keep external inventory, payment, and outbox work outside this focused example",
		"record metrics for transaction duration, rollback reason, item update count, and status lookup latency",
	}
}
