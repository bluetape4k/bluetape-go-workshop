package orderhistory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
)

const aggregateType = "order"

// Options configures the audit author and event clock.
type Options struct {
	Author string
	Now    func() time.Time
}

// Service appends immutable audit entries before updating current order state.
type Service struct {
	mu     sync.Mutex
	repo   audit.Repository
	author string
	now    func() time.Time
	orders map[string]Order
	used   map[string]struct{}
}

// NewService creates an audit-backed order history service.
func NewService(repo audit.Repository, options Options) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrInvalidConfig)
	}
	author := strings.TrimSpace(options.Author)
	if author == "" || !utf8.ValidString(author) || utf8.RuneCountInString(author) > 128 {
		return nil, fmt.Errorf("%w: author must be valid UTF-8 and contain 1..128 runes", ErrInvalidConfig)
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		repo: repo, author: author, now: now,
		orders: make(map[string]Order), used: make(map[string]struct{}),
	}, nil
}

// Create records a pending order.
func (s *Service) Create(ctx context.Context, command CreateCommand) (Order, error) {
	if !s.ready() {
		return Order{}, ErrInvalidConfig
	}
	orderID, commandID, err := normalizeCommand(command.OrderID, command.CommandID)
	if err != nil {
		return Order{}, err
	}
	return s.apply(ctx, orderID, commandID, "order.created", "", StatusPending, "")
}

// Confirm moves a pending order to confirmed.
func (s *Service) Confirm(ctx context.Context, command TransitionCommand) (Order, error) {
	if !s.ready() {
		return Order{}, ErrInvalidConfig
	}
	orderID, commandID, err := normalizeCommand(command.OrderID, command.CommandID)
	if err != nil {
		return Order{}, err
	}
	return s.apply(ctx, orderID, commandID, "order.confirmed", StatusPending, StatusConfirmed, "")
}

// Ship moves a confirmed order to shipped.
func (s *Service) Ship(ctx context.Context, command TransitionCommand) (Order, error) {
	if !s.ready() {
		return Order{}, ErrInvalidConfig
	}
	orderID, commandID, err := normalizeCommand(command.OrderID, command.CommandID)
	if err != nil {
		return Order{}, err
	}
	return s.apply(ctx, orderID, commandID, "order.shipped", StatusConfirmed, StatusShipped, "")
}

// Cancel moves a pending or confirmed order to cancelled.
func (s *Service) Cancel(ctx context.Context, command CancelCommand) (Order, error) {
	if !s.ready() {
		return Order{}, ErrInvalidConfig
	}
	orderID, commandID, err := normalizeCommand(command.OrderID, command.CommandID)
	if err != nil {
		return Order{}, err
	}
	reason, err := normalizeReason(command.Reason)
	if err != nil {
		return Order{}, err
	}
	return s.apply(ctx, orderID, commandID, "order.cancelled", "", StatusCancelled, reason)
}

// Current returns the current-state projection for one order.
func (s *Service) Current(orderID string) (Order, bool) {
	if !s.ready() {
		return Order{}, false
	}
	orderID = strings.TrimSpace(orderID)
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[orderID]
	return order, ok
}

// History returns the complete demo history for one order aggregate.
func (s *Service) History(ctx context.Context, orderID string) (audit.History, bool, error) {
	if !s.ready() {
		return audit.History{}, false, ErrInvalidConfig
	}
	orderID, err := normalizeIdentifier("order_id", orderID)
	if err != nil {
		return audit.History{}, false, err
	}
	aggregate, err := audit.NewAggregateID(aggregateType, orderID)
	if err != nil {
		return audit.History{}, false, fmt.Errorf("create aggregate id: %w", err)
	}
	history, ok, err := s.repo.LoadHistory(normalizeContext(ctx), aggregate)
	if err != nil {
		return audit.History{}, false, fmt.Errorf("load order history: %w", err)
	}
	return history, ok, nil
}

// Find returns a bounded order-only audit query result.
func (s *Service) Find(ctx context.Context, query audit.Query) ([]audit.Entry, error) {
	if !s.ready() {
		return nil, ErrInvalidConfig
	}
	if query.AggregateType != "" && strings.TrimSpace(query.AggregateType) != aggregateType {
		return nil, fmt.Errorf("%w: aggregate_type must be order", audit.ErrInvalidQuery)
	}
	if query.Aggregate != nil && strings.TrimSpace(query.Aggregate.Type) != aggregateType {
		return nil, fmt.Errorf("%w: aggregate must have type order", audit.ErrInvalidQuery)
	}
	query.AggregateType = aggregateType
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		return nil, fmt.Errorf("%w: limit exceeds 100", audit.ErrInvalidQuery)
	}
	entries, err := s.repo.Find(normalizeContext(ctx), query)
	if err != nil {
		return nil, fmt.Errorf("find order history: %w", err)
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	return entries, nil
}

func (s *Service) apply(ctx context.Context, orderID, commandID string, eventType audit.EventType, required, target Status, reason string) (Order, error) {
	if !s.ready() {
		return Order{}, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return Order{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Order{}, err
	}
	if _, duplicate := s.used[commandID]; duplicate {
		return Order{}, fmt.Errorf("duplicate command: %w", audit.ValidationError{
			Kind: audit.ErrRevisionConflict, Field: "event_id", Value: commandID,
		})
	}

	current, exists := s.orders[orderID]
	if eventType == "order.created" {
		if exists {
			return Order{}, fmt.Errorf("%w: %s", ErrOrderExists, orderID)
		}
	} else {
		if !exists {
			return Order{}, fmt.Errorf("%w: %s", ErrOrderNotFound, orderID)
		}
		if eventType == "order.cancelled" {
			if current.Status != StatusPending && current.Status != StatusConfirmed {
				return Order{}, fmt.Errorf("%w: cannot cancel %s", ErrInvalidTransition, current.Status)
			}
		} else if current.Status != required {
			return Order{}, fmt.Errorf("%w: %s to %s", ErrInvalidTransition, current.Status, target)
		}
	}

	revision := audit.InitialRevision()
	before := Status("")
	if exists {
		before = current.Status
		var err error
		revision, err = current.Revision.Next()
		if err != nil {
			return Order{}, fmt.Errorf("next order revision: %w", err)
		}
	}
	now := s.now().UTC()
	entry, err := s.newEntry(orderID, commandID, eventType, revision, now, before, target, reason)
	if err != nil {
		return Order{}, err
	}
	if err := s.repo.Append(ctx, entry); err != nil {
		return Order{}, fmt.Errorf("append order audit entry: %w", err)
	}
	updated := Order{OrderID: orderID, Status: target, Revision: revision, UpdatedAt: now}
	s.orders[orderID] = updated
	s.used[commandID] = struct{}{}
	return updated, nil
}

func (s *Service) newEntry(orderID, commandID string, eventType audit.EventType, revision audit.Revision, now time.Time, before, after Status, reason string) (audit.Entry, error) {
	aggregate, err := audit.NewAggregateID(aggregateType, orderID)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("create aggregate id: %w", err)
	}
	payload, err := json.Marshal(struct {
		StatusBefore Status `json:"status_before,omitempty"`
		StatusAfter  Status `json:"status_after"`
		Reason       string `json:"reason,omitempty"`
	}{StatusBefore: before, StatusAfter: after, Reason: reason})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("marshal order event: %w", err)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID: audit.EventID(commandID), EventType: eventType, AggregateID: aggregate,
		Revision: revision, OccurredAt: now, RecordedAt: now, IdempotencyKey: commandID,
		Metadata: audit.Metadata{"source": "audit-order-history"}, Payload: payload,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("create order event: %w", err)
	}
	change, err := audit.NewChangeMetadata([]string{"status"}, "order status changed", audit.Metadata{
		"status_before": string(before), "status_after": string(after),
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("create order change metadata: %w", err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: s.author, Event: event, Change: &change})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("create order audit entry: %w", err)
	}
	return entry, nil
}

func (s *Service) ready() bool {
	return s != nil && s.repo != nil && s.now != nil && s.orders != nil && s.used != nil
}

func normalizeCommand(orderID, commandID string) (string, string, error) {
	orderID, err := normalizeIdentifier("order_id", orderID)
	if err != nil {
		return "", "", err
	}
	commandID, err = normalizeIdentifier("command_id", commandID)
	if err != nil {
		return "", "", err
	}
	return orderID, commandID, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
