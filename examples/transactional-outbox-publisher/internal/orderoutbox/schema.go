package orderoutbox

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const createOrdersTableSQL = `
create table if not exists transactional_outbox_orders (
	order_id text primary key,
	customer_id text not null,
	status text not null check (status = 'placed'),
	total_cents bigint not null check (total_cents > 0),
	created_at timestamptz not null
)`

// CreateSchema creates the workshop order table followed by the SQL outbox schema.
func (s *Service) CreateSchema(ctx context.Context, execer sqlkit.Execer) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("%w: initialized service is required", ErrInvalidConfig)
	}
	if isNilExecer(execer) {
		return fmt.Errorf("%w: database executor is required", ErrInvalidConfig)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("create order schema: %w", err)
	}
	if _, err := execer.ExecContext(ctx, createOrdersTableSQL); err != nil {
		return fmt.Errorf("create order schema: %w", err)
	}
	if err := s.store.CreateSchema(ctx, execer); err != nil {
		return fmt.Errorf("create outbox schema: %w", err)
	}
	return nil
}

func isNilExecer(execer sqlkit.Execer) bool {
	if execer == nil {
		return true
	}
	value := reflect.ValueOf(execer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
