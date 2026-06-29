package ordersapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPreviewKeepsHTTPAndSQLBoundariesVisible(t *testing.T) {
	preview, err := NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}

	assertSnapshot(t, preview.Statements, "orders.create",
		`insert into "gin_sql_crud_orders" ("id", "customer_id", "status", "total_cents", "created_at") values ($1, $2, $3, $4, $5)`,
		[]any{"ord_1001", "customer-42", string(StatusPending), int64(2599), previewTime()})
	assertSnapshot(t, preview.Statements, "orders.list",
		`select "id", "customer_id", "status", "total_cents", "created_at" from "gin_sql_crud_orders" where customer_id = $1 and status = $2 order by "created_at", "id" limit 20`,
		[]any{"customer-42", string(StatusPending)})
	assertSnapshot(t, preview.Statements, "orders.update_status",
		`update "gin_sql_crud_orders" set "status" = $1 where id = $2`,
		[]any{string(StatusPaid), "ord_1001"})
	assertSnapshot(t, preview.Statements, "orders.delete",
		`delete from "gin_sql_crud_orders" where id = $1`,
		[]any{"ord_1001"})

	if len(preview.HTTPBoundary) < 2 || len(preview.Repository) < 2 {
		t.Fatalf("preview = %#v, want HTTP and repository boundary notes", preview)
	}
}

func TestHTTPCRUDFlowUsesPostgreSQLRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	api := newTestServer(t, db, []string{"ord_1001", "ord_1002"})

	create := performJSON(api, http.MethodPost, "/orders", CreateOrderRequest{
		CustomerID: "customer-42",
		TotalCents: 2599,
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST /orders status = %d body = %s", create.Code, create.Body.String())
	}
	var created Order
	decodeJSON(t, create, &created)
	assertOrder(t, created, Order{
		ID:         "ord_1001",
		CustomerID: "customer-42",
		Status:     StatusPending,
		TotalCents: 2599,
		CreatedAt:  fixedTime(),
	})

	second := performJSON(api, http.MethodPost, "/orders", CreateOrderRequest{
		CustomerID: "customer-42",
		TotalCents: 4800,
	})
	if second.Code != http.StatusCreated {
		t.Fatalf("second POST /orders status = %d body = %s", second.Code, second.Body.String())
	}

	get := perform(api, http.MethodGet, "/orders/ord_1001")
	if get.Code != http.StatusOK {
		t.Fatalf("GET /orders/ord_1001 status = %d body = %s", get.Code, get.Body.String())
	}
	var found Order
	decodeJSON(t, get, &found)
	assertOrder(t, found, created)

	list := perform(api, http.MethodGet, "/orders?customer_id=customer-42&status=pending&limit=10")
	if list.Code != http.StatusOK {
		t.Fatalf("GET /orders status = %d body = %s", list.Code, list.Body.String())
	}
	var listed struct {
		Orders []Order `json:"orders"`
		Count  int     `json:"count"`
	}
	decodeJSON(t, list, &listed)
	if gotIDs := orderIDs(listed.Orders); !reflect.DeepEqual(gotIDs, []string{"ord_1001", "ord_1002"}) {
		t.Fatalf("listed IDs = %#v", gotIDs)
	}
	if listed.Count != 2 {
		t.Fatalf("listed count = %d, want 2", listed.Count)
	}

	update := performJSON(api, http.MethodPatch, "/orders/ord_1001/status", UpdateStatusRequest{Status: StatusPaid})
	if update.Code != http.StatusOK {
		t.Fatalf("PATCH /orders/ord_1001/status status = %d body = %s", update.Code, update.Body.String())
	}
	var paid Order
	decodeJSON(t, update, &paid)
	if paid.Status != StatusPaid {
		t.Fatalf("updated status = %q, want %q", paid.Status, StatusPaid)
	}

	remove := perform(api, http.MethodDelete, "/orders/ord_1001")
	if remove.Code != http.StatusNoContent {
		t.Fatalf("DELETE /orders/ord_1001 status = %d body = %s", remove.Code, remove.Body.String())
	}
	missing := perform(api, http.MethodGet, "/orders/ord_1001")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET deleted status = %d body = %s", missing.Code, missing.Body.String())
	}
}

func TestHTTPValidationAndNotFoundErrorsAreStable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	api := newTestServer(t, db, []string{"ord_2001"})

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name:       "bad create",
			method:     http.MethodPost,
			path:       "/orders",
			body:       CreateOrderRequest{CustomerID: "", TotalCents: -1},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "empty list filter",
			method:     http.MethodGet,
			path:       "/orders",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "unsupported status",
			method:     http.MethodPatch,
			path:       "/orders/missing/status",
			body:       UpdateStatusRequest{Status: "shipped"},
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

func TestRepositoryBehaviorDoesNotDependOnGin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	repo := Repository{}
	order := Order{
		ID:         "ord_repo",
		CustomerID: "customer-77",
		Status:     StatusPending,
		TotalCents: 9900,
		CreatedAt:  fixedTime(),
	}
	if err := repo.Create(ctx, db, order); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	found, err := repo.FindByID(ctx, db, order.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	assertOrder(t, found, order)

	updated, err := repo.UpdateStatus(ctx, db, order.ID, StatusCancelled)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Status != StatusCancelled {
		t.Fatalf("updated status = %q, want %q", updated.Status, StatusCancelled)
	}

	if err := repo.Delete(ctx, db, order.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = repo.FindByID(ctx, db, order.ID)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("FindByID(deleted) error = %v, want ErrOrderNotFound", err)
	}
}

func TestRepositoryPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	db := openPostgresDB(ctx, t)
	repo := Repository{}
	canceled, stop := context.WithCancel(context.Background())
	stop()

	_, err := repo.FindByID(canceled, db, "ord_canceled")
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
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestServer(t *testing.T, db *sql.DB, ids []string) *Server {
	t.Helper()

	index := 0
	api, err := NewServer(Options{
		DB:  db,
		Now: fixedTime,
		NewID: func() (string, error) {
			if index >= len(ids) {
				return "", errors.New("no test IDs left")
			}
			id := ids[index]
			index++
			return id, nil
		},
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return api
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

func fixedTime() time.Time {
	return time.Date(2026, 6, 29, 10, 30, 0, 0, time.UTC)
}
