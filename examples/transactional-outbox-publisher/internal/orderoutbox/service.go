package orderoutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

// Service 는 주문 저장과 감사 이벤트 enqueue 를 함께 수행하는 트랜잭션을 소유한다.
type Service struct {
	store  *sqloutbox.Store
	author string
	now    func() time.Time
}

// NewService 는 의존성을 검증하고 주문 outbox 서비스를 반환한다.
func NewService(store *sqloutbox.Store, config Config) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: outbox store is required", ErrInvalidConfig)
	}
	author := strings.TrimSpace(config.Author)
	if !utf8.ValidString(author) {
		return nil, fmt.Errorf("%w: author must be valid UTF-8", ErrInvalidConfig)
	}
	if runes := utf8.RuneCountInString(author); runes == 0 || runes > maxIDRunes {
		return nil, fmt.Errorf("%w: author must contain 1..%d runes", ErrInvalidConfig, maxIDRunes)
	}

	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{store: store, author: author, now: now}, nil
}

// Place 는 하나의 SQL 트랜잭션에서 주문과 outbox 항목을 함께 저장한다.
func (s *Service) Place(ctx context.Context, db *sql.DB, command PlaceOrderCommand) (Order, error) {
	if s == nil || s.store == nil || s.now == nil || s.author == "" {
		return Order{}, fmt.Errorf("%w: initialized service is required", ErrInvalidConfig)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Order{}, fmt.Errorf("place order: %w", err)
	}

	command, err := validatePlaceOrderCommand(command)
	if err != nil {
		return Order{}, err
	}
	if db == nil {
		return Order{}, fmt.Errorf("%w: database is required", ErrInvalidConfig)
	}
	entry, err := s.buildEntry(command)
	if err != nil {
		return Order{}, err
	}

	err = sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into transactional_outbox_orders
				(order_id, customer_id, status, total_cents, created_at)
			values ($1, $2, $3, $4, $5)`,
			command.OrderID, command.CustomerID, StatusPlaced,
			command.TotalCents, command.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert order: %w", err)
		}
		if err := s.store.Enqueue(ctx, tx, entry); err != nil {
			return fmt.Errorf("enqueue order event: %w", err)
		}
		return nil
	})
	if err != nil {
		return Order{}, fmt.Errorf("place order transaction: %w", err)
	}

	return Order{
		OrderID: command.OrderID, CustomerID: command.CustomerID,
		Status: StatusPlaced, TotalCents: command.TotalCents,
		CreatedAt: command.CreatedAt,
	}, nil
}

func (s *Service) buildEntry(command PlaceOrderCommand) (audit.Entry, error) {
	aggregate, err := audit.NewAggregateID(aggregateType, command.OrderID)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("build aggregate identity: %w", err)
	}
	payload, err := json.Marshal(struct {
		CustomerID string `json:"customer_id"`
		Status     string `json:"status"`
		TotalCents int64  `json:"total_cents"`
	}{
		CustomerID: command.CustomerID,
		Status:     StatusPlaced,
		TotalCents: command.TotalCents,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("encode order event payload: %w", err)
	}
	recordedAt := s.now().UTC()
	if recordedAt.IsZero() {
		return audit.Entry{}, fmt.Errorf("%w: clock returned zero time", ErrInvalidConfig)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        audit.EventID(command.CommandID),
		EventType:      eventType,
		AggregateID:    aggregate,
		Revision:       audit.InitialRevision(),
		OccurredAt:     command.CreatedAt,
		RecordedAt:     recordedAt,
		IdempotencyKey: command.CommandID,
		Payload:        payload,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("build order event: %w", err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: s.author, Event: event})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("build order audit entry: %w", err)
	}
	return entry, nil
}

func validatePlaceOrderCommand(command PlaceOrderCommand) (PlaceOrderCommand, error) {
	var err error
	if command.OrderID, err = validateIdentifier("order_id", command.OrderID); err != nil {
		return PlaceOrderCommand{}, err
	}
	if command.CustomerID, err = validateIdentifier("customer_id", command.CustomerID); err != nil {
		return PlaceOrderCommand{}, err
	}
	if command.CommandID, err = validateIdentifier("command_id", command.CommandID); err != nil {
		return PlaceOrderCommand{}, err
	}
	if command.TotalCents <= 0 {
		return PlaceOrderCommand{}, fmt.Errorf("%w: total_cents must be positive", ErrInvalidOrder)
	}
	if command.CreatedAt.IsZero() {
		return PlaceOrderCommand{}, fmt.Errorf("%w: created_at is required", ErrInvalidOrder)
	}
	command.CreatedAt = command.CreatedAt.UTC()
	return command, nil
}

func validateIdentifier(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%w: %s must be valid UTF-8", ErrInvalidOrder, field)
	}
	if runes := utf8.RuneCountInString(value); runes == 0 || runes > maxIDRunes {
		return "", fmt.Errorf("%w: %s must contain 1..%d runes", ErrInvalidOrder, field, maxIDRunes)
	}
	return value, nil
}
