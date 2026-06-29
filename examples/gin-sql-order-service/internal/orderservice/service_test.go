package orderservice

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPreviewDocumentsIntegratedBoundaries(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}

	assertSnapshot(t, preview.Statements, "orders.create",
		`insert into "gin_sql_order_service_orders" ("id", "customer_id", "status", "total_cents", "created_at", "updated_at") values ($1, $2, $3, $4, $5, $6)`,
		[]any{"ord_1001", "customer-42", string(StatusPending), int64(3700), previewTime(), previewTime()})
	assertSnapshot(t, preview.Statements, "items.update_quantity",
		`update "gin_sql_order_service_items" set "quantity" = $1, "line_total_cents" = $2 where order_id = $3 and id = $4`,
		[]any{3, int64(3600), "ord_1001", "item_1001"})
	assertSnapshot(t, preview.Statements, "statuses.append",
		`insert into "gin_sql_order_service_status_events" ("id", "order_id", "status", "reason", "created_at") values ($1, $2, $3, $4, $5)`,
		[]any{"evt_1001", "ord_1001", string(StatusPending), "order accepted", previewTime()})

	if len(preview.BuildsOn) != 4 || len(preview.ServiceBoundary) < 2 {
		t.Fatalf("preview = %#v, want previous-example and service boundary notes", preview)
	}
}

func TestHTTPOrderWorkflowCreatesReadsStatusAndUpdatesItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	api := newTestServer(t, db)

	create := performJSON(api, http.MethodPost, "/orders", CreateOrderRequest{
		CustomerID: "customer-42",
		Items: []CreateItemRequest{
			{SKU: "sku-coffee", Quantity: 2, UnitPriceCents: 1200},
			{SKU: "sku-filter", Quantity: 1, UnitPriceCents: 1300},
		},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST /orders status = %d body = %s", create.Code, create.Body.String())
	}
	var created OrderView
	decodeJSON(t, create, &created)
	if created.Order.ID != "ord_1001" || created.Order.TotalCents != 3700 || len(created.Items) != 2 || len(created.StatusHistory) != 1 {
		t.Fatalf("created view = %#v", created)
	}
	if gotIDs := itemIDs(created.Items); !reflect.DeepEqual(gotIDs, []string{"item_1001", "item_1002"}) {
		t.Fatalf("item IDs = %#v", gotIDs)
	}

	get := perform(api, http.MethodGet, "/orders/ord_1001")
	if get.Code != http.StatusOK {
		t.Fatalf("GET /orders/ord_1001 status = %d body = %s", get.Code, get.Body.String())
	}
	var found OrderView
	decodeJSON(t, get, &found)
	if found.Order.TotalCents != 3700 || len(found.Items) != 2 {
		t.Fatalf("found view = %#v", found)
	}

	status := perform(api, http.MethodGet, "/orders/ord_1001/status")
	if status.Code != http.StatusOK {
		t.Fatalf("GET /orders/ord_1001/status status = %d body = %s", status.Code, status.Body.String())
	}
	var statusBody StatusResponse
	decodeJSON(t, status, &statusBody)
	if statusBody.Status != StatusPending || len(statusBody.History) != 1 {
		t.Fatalf("status body = %#v", statusBody)
	}

	update := performJSON(api, http.MethodPatch, "/orders/ord_1001/items/item_1001", UpdateItemRequest{Quantity: 3})
	if update.Code != http.StatusOK {
		t.Fatalf("PATCH item status = %d body = %s", update.Code, update.Body.String())
	}
	var updated OrderView
	decodeJSON(t, update, &updated)
	if updated.Order.TotalCents != 4900 || len(updated.StatusHistory) != 2 {
		t.Fatalf("updated view = %#v", updated)
	}
	if updated.Items[0].Quantity != 3 || updated.Items[0].LineTotalCents != 3600 {
		t.Fatalf("updated first item = %#v", updated.Items[0])
	}
}

func TestHTTPRollbackFailureLeavesNoRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	api := newTestServer(t, db)

	response := performJSON(api, http.MethodPost, "/orders", CreateOrderRequest{
		CustomerID: "customer-42",
		Items: []CreateItemRequest{
			{SKU: "sku-coffee", Quantity: 2, UnitPriceCents: 1200},
		},
		RejectAfterItems: true,
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("POST rejected status = %d body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	decodeJSON(t, response, &body)
	if body.Code != "order_rejected" {
		t.Fatalf("error code = %q, want order_rejected", body.Code)
	}

	assertTableCount(ctx, t, db, ordersTable, 0)
	assertTableCount(ctx, t, db, itemsTable, 0)
	assertTableCount(ctx, t, db, statusEventsTable, 0)
}

func TestHTTPValidationAndNotFoundErrorsAreStable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	api := newTestServer(t, db)

	created := performJSON(api, http.MethodPost, "/orders", CreateOrderRequest{
		CustomerID: "customer-42",
		Items: []CreateItemRequest{
			{SKU: "sku-coffee", Quantity: 1, UnitPriceCents: 1200},
		},
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("POST /orders status = %d body = %s", created.Code, created.Body.String())
	}

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name:       "create with no items",
			method:     http.MethodPost,
			path:       "/orders",
			body:       CreateOrderRequest{CustomerID: "customer-42"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "missing order",
			method:     http.MethodGet,
			path:       "/orders/missing",
			wantStatus: http.StatusNotFound,
			wantCode:   "order_not_found",
		},
		{
			name:       "bad item quantity",
			method:     http.MethodPatch,
			path:       "/orders/missing/items/item-missing",
			body:       UpdateItemRequest{Quantity: 0},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "missing item",
			method:     http.MethodPatch,
			path:       "/orders/ord_1001/items/item-missing",
			body:       UpdateItemRequest{Quantity: 2},
			wantStatus: http.StatusNotFound,
			wantCode:   "item_not_found",
		},
		{
			name:       "missing order during item update",
			method:     http.MethodPatch,
			path:       "/orders/missing/items/item-missing",
			body:       UpdateItemRequest{Quantity: 2},
			wantStatus: http.StatusNotFound,
			wantCode:   "order_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performMaybeJSON(api, tt.method, tt.path, tt.body)
			if response.Code != tt.wantStatus {
				t.Fatalf("%s %s status = %d body = %s", tt.method, tt.path, response.Code, response.Body.String())
			}
			var body struct {
				Code string `json:"code"`
			}
			decodeJSON(t, response, &body)
			if body.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}

func TestServiceWorksWithoutGinAndPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	service := newTestService(t)

	view, err := service.CreateOrder(ctx, db, CreateOrderRequest{
		CustomerID: "customer-77",
		Items: []CreateItemRequest{
			{SKU: "sku-tea", Quantity: 1, UnitPriceCents: 900},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if view.Order.ID != "ord_1001" || view.Order.TotalCents != 900 {
		t.Fatalf("view = %#v", view)
	}

	canceled, stop := context.WithCancel(context.Background())
	stop()
	_, err = service.GetOrder(canceled, db, "ord_1001")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetOrder(canceled) error = %v, want context.Canceled", err)
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
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestServer(t *testing.T, db *sql.DB) *Server {
	t.Helper()

	api, err := NewServer(Options{
		DB:    db,
		Now:   fixedTime,
		NewID: sequentialIDs(),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return api
}

func newTestService(t *testing.T) Service {
	t.Helper()
	return NewService(fixedTime, sequentialIDs())
}

func sequentialIDs() func(prefix string) (string, error) {
	next := map[string]int{
		"evt":  1000,
		"item": 1000,
		"ord":  1000,
	}
	return func(prefix string) (string, error) {
		next[prefix]++
		return prefix + "_" + strconv4(next[prefix]), nil
	}
}

func strconv4(value int) string {
	return fmt.Sprintf("%04d", value)
}

func perform(api http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(context.Background(), method, path, http.NoBody)
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	return response
}

func performJSON(api http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	return performMaybeJSON(api, method, path, body)
}

func performMaybeJSON(api http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request := httptest.NewRequestWithContext(context.Background(), method, path, requestBody)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode JSON %q: %v", response.Body.String(), err)
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

func assertTableCount(ctx context.Context, t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRowContext(ctx, "select count(*) from "+table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("count %s = %d, want %d", table, got, want)
	}
}

func itemIDs(items []OrderItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func fixedTime() time.Time {
	return time.Date(2026, 6, 29, 11, 30, 0, 0, time.UTC)
}
