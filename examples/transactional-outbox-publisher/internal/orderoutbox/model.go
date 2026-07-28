// Package orderoutbox 는 주문과 감사 이벤트를 원자적으로 commit 한다.
package orderoutbox

import (
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

const (
	aggregateType = "order"
	eventType     = audit.EventType("order.placed")
	// StatusPlaced 는 성공적으로 생성된 주문의 저장 상태다.
	StatusPlaced = "placed"
	ordersTable  = "transactional_outbox_orders"
	outboxTable  = "transactional_outbox_records"
	maxIDRunes   = 128
)

var (
	// ErrInvalidConfig 는 사용할 수 없는 서비스 의존성 또는 option 을 나타낸다.
	ErrInvalidConfig = errors.New("orderoutbox: invalid config")
	// ErrInvalidOrder 는 유효하지 않은 주문 생성 명령을 나타낸다.
	ErrInvalidOrder = errors.New("orderoutbox: invalid order")
)

// Config 는 Service 에 감사 작성자와 선택적 UTC clock 을 제공한다.
type Config struct {
	Author string
	Now    func() time.Time
}

// PlaceOrderCommand 는 주문 하나의 durable identity 와 값을 담는다.
type PlaceOrderCommand struct {
	OrderID    string
	CustomerID string
	CommandID  string
	TotalCents int64
	CreatedAt  time.Time
}

// Order 는 주문 행과 outbox 행이 함께 commit 된 뒤에만 반환된다.
type Order struct {
	OrderID    string
	CustomerID string
	Status     string
	TotalCents int64
	CreatedAt  time.Time
}
