package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/sqlkit"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxAuthorRunes = 128

// Config 는 영속 감사 작성자와 주입 가능한 트랜잭션 시계를 제공한다.
type Config struct {
	Author string
	Now    func() time.Time
}

type transactionRunner func(context.Context, sqlkit.Beginner, *sql.TxOptions, sqlkit.TxFunc) error

// Service 는 명령 검증과 주문, 이력, outbox를 함께 기록하는 원자적 트랜잭션을 책임진다.
type Service struct {
	db      *sql.DB
	history *HistoryStore
	outbox  *sqloutbox.Store
	author  string
	now     func() time.Time
	runTx   transactionRunner
}

// NewService 는 기존 store 위에 감사 가능한 주문 명령 서비스를 구성한다.
func NewService(db *sql.DB, history *HistoryStore, outbox *sqloutbox.Store, config Config) (*Service, error) {
	author := strings.TrimSpace(config.Author)
	if db == nil || history == nil || outbox == nil || author == "" ||
		!utf8.ValidString(author) || utf8.RuneCountInString(author) > maxAuthorRunes {
		return nil, ErrInvalidConfig
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		db: db, history: history, outbox: outbox,
		author: author, now: now, runTx: sqlkit.WithTx,
	}, nil
}

// Create 는 대기 상태 주문을 영속화하거나 표준 replay 프로젝션을 반환한다.
func (s *Service) Create(ctx context.Context, command CreateCommand) (Order, bool, error) {
	if err := s.validate(); err != nil {
		return Order{}, false, err
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return Order{}, false, err
	}
	validated, err := command.validate()
	if err != nil {
		return Order{}, false, err
	}

	var result Order
	var replayed bool
	err = s.runTx(ctx, s.db, nil, func(ctx context.Context, tx *sql.Tx) error {
		if existing, found, findErr := s.history.FindByCommandID(ctx, tx, validated.CommandID); findErr != nil {
			return findErr
		} else if found {
			resolved, resolveErr := resolveCreate(existing, validated)
			if resolveErr != nil {
				return resolveErr
			}
			result, replayed = resolved, true
			return nil
		}

		now := normalizeTimestamp(s.now())
		result = Order{
			OrderID: validated.OrderID, Status: StatusPending,
			Revision: audit.InitialRevision(), UpdatedAt: now,
		}
		if _, execErr := tx.ExecContext(ctx, `
insert into audited_order_workflow_orders (order_id, status, revision, updated_at)
values ($1, $2, $3, $4)`, result.OrderID, result.Status, result.Revision, result.UpdatedAt); execErr != nil {
			return fmt.Errorf("insert order: %w", execErr)
		}
		entry, buildErr := buildCreateEntry(s.author, now, validated, result)
		if buildErr != nil {
			return buildErr
		}
		if insertErr := s.history.Insert(ctx, tx, entry); insertErr != nil {
			return insertErr
		}
		if enqueueErr := s.outbox.Enqueue(ctx, tx, entry); enqueueErr != nil {
			return fmt.Errorf("enqueue outbox: %w", enqueueErr)
		}
		return nil
	})
	if err == nil {
		return result, replayed, nil
	}
	if !isUniqueViolation(err) {
		return Order{}, false, err
	}
	resolved, found, resolveErr := s.resolveCreateByCommandID(ctx, validated)
	if resolveErr != nil {
		return Order{}, false, errors.Join(err, resolveErr)
	}
	if found {
		return resolved, true, nil
	}
	return Order{}, false, fmt.Errorf("%w: create order or command already exists: %w", ErrConflict, err)
}

// Transition 은 하나의 유효한 상태 변경을 적용하거나 표준 replay 프로젝션을 반환한다.
func (s *Service) Transition(ctx context.Context, command TransitionCommand) (Order, bool, error) {
	if err := s.validate(); err != nil {
		return Order{}, false, err
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return Order{}, false, err
	}
	validated, err := command.validate()
	if err != nil {
		return Order{}, false, err
	}

	var result Order
	var replayed bool
	err = s.runTx(ctx, s.db, nil, func(ctx context.Context, tx *sql.Tx) error {
		current, loadErr := loadOrderForUpdate(ctx, tx, validated.OrderID)
		if errors.Is(loadErr, sql.ErrNoRows) {
			if existing, found, findErr := s.history.FindByCommandID(ctx, tx, validated.CommandID); findErr != nil {
				return findErr
			} else if found {
				resolved, resolveErr := resolveTransition(existing, validated)
				if resolveErr != nil {
					return resolveErr
				}
				result, replayed = resolved, true
				return nil
			}
			return fmt.Errorf("%w: order %s", ErrNotFound, validated.OrderID)
		}
		if loadErr != nil {
			return loadErr
		}

		if existing, found, findErr := s.history.FindByCommandID(ctx, tx, validated.CommandID); findErr != nil {
			return findErr
		} else if found {
			resolved, resolveErr := resolveTransition(existing, validated)
			if resolveErr != nil {
				return resolveErr
			}
			result, replayed = resolved, true
			return nil
		}

		nextStatus, stateErr := transitionStatus(current.Status, validated.Action)
		if stateErr != nil {
			return stateErr
		}
		nextRevision, revisionErr := current.Revision.Next()
		if revisionErr != nil {
			return fmt.Errorf("%w: next revision: %w", ErrConflict, revisionErr)
		}
		now := normalizeTimestamp(s.now())
		result = Order{OrderID: current.OrderID, Status: nextStatus, Revision: nextRevision, UpdatedAt: now}
		update, updateErr := tx.ExecContext(ctx, `
update audited_order_workflow_orders
set status = $1, revision = $2, updated_at = $3
where order_id = $4 and revision = $5`, result.Status, result.Revision, result.UpdatedAt, result.OrderID, current.Revision)
		if updateErr != nil {
			return fmt.Errorf("update order: %w", updateErr)
		}
		updated, rowsErr := update.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("updated rows: %w", rowsErr)
		}
		if updated != 1 {
			return fmt.Errorf("%w: stale order revision", ErrConflict)
		}
		entry, buildErr := buildTransitionEntry(s.author, now, validated, result)
		if buildErr != nil {
			return buildErr
		}
		if insertErr := s.history.Insert(ctx, tx, entry); insertErr != nil {
			return insertErr
		}
		if enqueueErr := s.outbox.Enqueue(ctx, tx, entry); enqueueErr != nil {
			return fmt.Errorf("enqueue outbox: %w", enqueueErr)
		}
		return nil
	})
	if err == nil {
		return result, replayed, nil
	}
	if !isUniqueViolation(err) {
		return Order{}, false, err
	}
	resolved, found, resolveErr := s.resolveTransitionByCommandID(ctx, validated)
	if resolveErr != nil {
		return Order{}, false, errors.Join(err, resolveErr)
	}
	if found {
		return resolved, true, nil
	}
	return Order{}, false, fmt.Errorf("%w: transition command already exists: %w", ErrConflict, err)
}

func (s *Service) validate() error {
	if s == nil || s.db == nil || s.history == nil || s.outbox == nil || s.now == nil || s.runTx == nil {
		return ErrInvalidConfig
	}
	return nil
}

func (s *Service) resolveCreateByCommandID(ctx context.Context, command CreateCommand) (Order, bool, error) {
	entry, found, err := s.history.FindByCommandID(ctx, s.db, command.CommandID)
	if err != nil || !found {
		return Order{}, found, err
	}
	order, err := resolveCreate(entry, command)
	return order, true, err
}

func (s *Service) resolveTransitionByCommandID(ctx context.Context, command TransitionCommand) (Order, bool, error) {
	entry, found, err := s.history.FindByCommandID(ctx, s.db, command.CommandID)
	if err != nil || !found {
		return Order{}, found, err
	}
	order, err := resolveTransition(entry, command)
	return order, true, err
}

func resolveCreate(entry audit.Entry, command CreateCommand) (Order, error) {
	payload, err := decodeEventPayload(entry)
	if err != nil {
		return Order{}, err
	}
	if !payload.Intent.matchesCreate(command) {
		return Order{}, fmt.Errorf("%w: command id reused with different create intent", ErrConflict)
	}
	return payload.Order, nil
}

func resolveTransition(entry audit.Entry, command TransitionCommand) (Order, error) {
	payload, err := decodeEventPayload(entry)
	if err != nil {
		return Order{}, err
	}
	if !payload.Intent.matchesTransition(command) {
		return Order{}, fmt.Errorf("%w: command id reused with different transition intent", ErrConflict)
	}
	return payload.Order, nil
}

func loadOrderForUpdate(ctx context.Context, tx *sql.Tx, orderID string) (Order, error) {
	var order Order
	var revision int64
	err := tx.QueryRowContext(ctx, `
select order_id, status, revision, updated_at
from audited_order_workflow_orders
where order_id = $1
for update`, orderID).Scan(&order.OrderID, &order.Status, &revision, &order.UpdatedAt)
	if err != nil {
		return Order{}, err
	}
	if revision < 1 {
		return Order{}, fmt.Errorf("%w: invalid stored revision", ErrInvalidEntry)
	}
	order.Revision = audit.Revision(revision)
	order.UpdatedAt = normalizeTimestamp(order.UpdatedAt)
	return order, nil
}

func transitionStatus(current Status, action Action) (Status, error) {
	switch {
	case current == StatusPending && action == ActionConfirm:
		return StatusConfirmed, nil
	case (current == StatusPending || current == StatusConfirmed) && action == ActionCancel:
		return StatusCancelled, nil
	default:
		return "", fmt.Errorf("%w: cannot %s %s order", ErrConflict, action, current)
	}
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
		return false
	}
	switch postgresError.ConstraintName {
	case "audited_order_workflow_orders_pkey",
		"audited_order_workflow_audit_entries_event_id_key",
		"audited_order_workflow_audit_entries_idempotency_key_key",
		"audited_order_workflow_outbox_records_event_id_key",
		"audited_order_workflow_outbox_records_idempotency_key_key":
		return true
	default:
		return false
	}
}
