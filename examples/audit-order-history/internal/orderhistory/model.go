// Package orderhistory demonstrates append-before-mutation audit history for orders.
package orderhistory

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
)

var (
	// ErrInvalidConfig reports invalid service construction or zero-value use.
	ErrInvalidConfig = errors.New("invalid order history configuration")
	// ErrInvalidCommand reports an invalid command identifier or field.
	ErrInvalidCommand = errors.New("invalid order history command")
	// ErrOrderExists reports duplicate order creation.
	ErrOrderExists = errors.New("order already exists")
	// ErrOrderNotFound reports a command for an absent order.
	ErrOrderNotFound = errors.New("order not found")
	// ErrInvalidTransition reports a transition disallowed by the state machine.
	ErrInvalidTransition = errors.New("invalid order status transition")
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Status is the current lifecycle state of an order.
type Status string

const (
	// StatusPending begins a newly created order lifecycle.
	StatusPending Status = "pending"
	// StatusConfirmed is ready for shipment.
	StatusConfirmed Status = "confirmed"
	// StatusShipped is a terminal fulfilled state.
	StatusShipped Status = "shipped"
	// StatusCancelled is a terminal stopped state.
	StatusCancelled Status = "cancelled"
)

// Order is the mutable current-state projection recorded by the example.
type Order struct {
	OrderID   string         `json:"order_id"`
	Status    Status         `json:"status"`
	Revision  audit.Revision `json:"revision"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CreateCommand starts an order and owns its stable command identity.
type CreateCommand struct {
	OrderID   string
	CommandID string
}

// TransitionCommand confirms or ships an existing order.
type TransitionCommand struct {
	OrderID   string
	CommandID string
}

// CancelCommand cancels an order with optional bounded audit context.
type CancelCommand struct {
	OrderID   string
	CommandID string
	Reason    string
}

func normalizeIdentifier(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if !identifierPattern.MatchString(value) {
		return "", fmt.Errorf("%w: %s is invalid", ErrInvalidCommand, name)
	}
	return value, nil
}

func normalizeReason(reason string) (string, error) {
	if !utf8.ValidString(reason) {
		return "", fmt.Errorf("%w: reason must be valid UTF-8", ErrInvalidCommand)
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 256 {
		return "", fmt.Errorf("%w: reason exceeds 256 runes", ErrInvalidCommand)
	}
	return reason, nil
}
