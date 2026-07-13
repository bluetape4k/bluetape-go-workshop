// Package orderoutbox commits orders and their audit events atomically.
package orderoutbox

import (
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

const (
	aggregateType = "order"
	eventType     = audit.EventType("order.placed")
	// StatusPlaced is the persisted status of a successfully placed order.
	StatusPlaced = "placed"
	ordersTable  = "transactional_outbox_orders"
	outboxTable  = "transactional_outbox_records"
	maxIDRunes   = 128
)

var (
	// ErrInvalidConfig reports an unusable service dependency or option.
	ErrInvalidConfig = errors.New("orderoutbox: invalid config")
	// ErrInvalidOrder reports an invalid order placement command.
	ErrInvalidOrder = errors.New("orderoutbox: invalid order")
)

// Config supplies the audit author and optional UTC clock for a Service.
type Config struct {
	Author string
	Now    func() time.Time
}

// PlaceOrderCommand contains the durable identities and values for one order.
type PlaceOrderCommand struct {
	OrderID    string
	CustomerID string
	CommandID  string
	TotalCents int64
	CreatedAt  time.Time
}

// Order is returned only after the order and outbox rows commit together.
type Order struct {
	OrderID    string
	CustomerID string
	Status     string
	TotalCents int64
	CreatedAt  time.Time
}
