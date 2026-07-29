// Package orderhistory 는 order에 대해 mutation 전에 audit history를 append하는 방식을 보여준다.
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
	// ErrInvalidConfig 는 invalid service construction이나 zero-value use를 나타낸다.
	ErrInvalidConfig = errors.New("invalid order history configuration")
	// ErrInvalidCommand 는 invalid command identifier나 field를 나타낸다.
	ErrInvalidCommand = errors.New("invalid order history command")
	// ErrOrderExists 는 duplicate order creation을 나타낸다.
	ErrOrderExists = errors.New("order already exists")
	// ErrOrderNotFound 는 존재하지 않는 order에 대한 command를 나타낸다.
	ErrOrderNotFound = errors.New("order not found")
	// ErrInvalidTransition 은 state machine이 허용하지 않는 transition을 나타낸다.
	ErrInvalidTransition = errors.New("invalid order status transition")
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Status 는 order의 현재 lifecycle state다.
type Status string

const (
	// StatusPending 은 새로 생성된 order lifecycle을 시작한다.
	StatusPending Status = "pending"
	// StatusConfirmed 는 shipment 준비가 된 상태다.
	StatusConfirmed Status = "confirmed"
	// StatusShipped 는 fulfilled terminal state다.
	StatusShipped Status = "shipped"
	// StatusCancelled 는 stopped terminal state다.
	StatusCancelled Status = "cancelled"
)

// Order 는 example이 기록하는 mutable current-state projection이다.
type Order struct {
	OrderID   string         `json:"order_id"`
	Status    Status         `json:"status"`
	Revision  audit.Revision `json:"revision"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CreateCommand 는 order를 시작하고 stable command identity를 소유한다.
type CreateCommand struct {
	OrderID   string
	CommandID string
}

// TransitionCommand 는 기존 order를 confirm하거나 ship한다.
type TransitionCommand struct {
	OrderID   string
	CommandID string
}

// CancelCommand 는 optional bounded audit context와 함께 order를 cancel한다.
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
