package orderworkflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const maxEntryBytes = 1 << 20

// HistoryStore는 불변 감사 엔트리를 영속화하고 audit.HistoryReader를 구현한다.
type HistoryStore struct {
	db *sql.DB
}

// DeliveryStatus는 이벤트 내용을 노출하지 않고 outbox 상태를 집계한다.
type DeliveryStatus struct {
	Pending              int64 `json:"pending"`
	Retrying             int64 `json:"retrying"`
	Claimed              int64 `json:"claimed"`
	Published            int64 `json:"published"`
	DeadLetter           int64 `json:"dead_letter"`
	OldestPendingSeconds int64 `json:"oldest_pending_seconds"`
}

var _ audit.HistoryReader = (*HistoryStore)(nil)

// NewHistoryStore는 영속 이력 조회를 db에 바인딩한다.
func NewHistoryStore(db *sql.DB) (*HistoryStore, error) {
	if db == nil {
		return nil, ErrInvalidConfig
	}
	return &HistoryStore{db: db}, nil
}

// Insert는 호출자가 소유한 트랜잭션 또는 세션을 통해 entry를 기록한다.
func (s *HistoryStore) Insert(ctx context.Context, db sqlkit.Execer, entry audit.Entry) error {
	if s == nil || s.db == nil || db == nil {
		return ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("%w: validate: %w", ErrInvalidEntry, err)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("%w: encode: %w", ErrInvalidEntry, err)
	}
	if len(encoded) > maxEntryBytes {
		return fmt.Errorf("%w: encoded entry exceeds %d bytes", ErrInvalidEntry, maxEntryBytes)
	}
	_, err = db.ExecContext(ctx, `
insert into audited_order_workflow_audit_entries (
    aggregate_type, aggregate_id, revision, event_id, idempotency_key,
    event_type, recorded_at, entry_json
) values ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.Aggregate.Type, entry.Aggregate.ID, entry.Revision,
		entry.Event.EventID, entry.Event.IdempotencyKey,
		entry.Event.EventType, entry.Event.RecordedAt, encoded,
	)
	if err != nil {
		return fmt.Errorf("insert history: %w", err)
	}
	return nil
}

// Find는 안정적인 revision 순서 또는 전역 position 순서로 엔트리를 반환한다.
func (s *HistoryStore) Find(ctx context.Context, query audit.Query) ([]audit.Entry, error) {
	if s == nil || s.db == nil {
		return nil, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalized, err := query.Validate()
	if err != nil {
		return nil, err
	}

	statement, args := buildHistoryFindQuery(normalized)
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	entries, scanErr := scanHistoryRows(rows)
	closeErr := rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close history rows: %w", closeErr)
	}
	return entries, nil
}

// FindByCommandID는 표준 이벤트 ID 또는 idempotency identity에 해당하는 엔트리를 찾는다.
func (s *HistoryStore) FindByCommandID(ctx context.Context, db sqlkit.QueryRower, commandID string) (audit.Entry, bool, error) {
	if s == nil || s.db == nil || db == nil {
		return audit.Entry{}, false, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return audit.Entry{}, false, err
	}
	row := db.QueryRowContext(ctx, `
select aggregate_type, aggregate_id, revision, event_id, idempotency_key,
       event_type, recorded_at, entry_json
from audited_order_workflow_audit_entries
where event_id = $1 or idempotency_key = $1
order by position
limit 1`, commandID)
	entry, err := scanHistoryRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return audit.Entry{}, false, nil
	}
	if err != nil {
		return audit.Entry{}, false, err
	}
	return entry, true, nil
}

// DeliveryStatus는 짧은 제한 시간 안에서 outbox 집계 진단값을 반환한다.
func (s *HistoryStore) DeliveryStatus(ctx context.Context, now time.Time) (DeliveryStatus, error) {
	if s == nil || s.db == nil {
		return DeliveryStatus{}, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	queryCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	var status DeliveryStatus
	err := s.db.QueryRowContext(queryCtx, `
select
    count(*) filter (where status = 'pending' and attempts = 0),
    count(*) filter (where status = 'pending' and attempts > 0),
    count(*) filter (where status = 'claimed'),
    count(*) filter (where status = 'published'),
    count(*) filter (where status = 'dead_letter'),
    greatest(0, coalesce(floor(extract(epoch from (
        $1::timestamptz - min(created_at) filter (where status in ('pending', 'claimed'))
    ))), 0))::bigint
from audited_order_workflow_outbox_records`, normalizeTimestamp(now)).Scan(
		&status.Pending, &status.Retrying, &status.Claimed,
		&status.Published, &status.DeadLetter, &status.OldestPendingSeconds,
	)
	if err != nil {
		return DeliveryStatus{}, fmt.Errorf("query delivery status: %w", err)
	}
	return status, nil
}

// LoadHistory는 aggregate의 검증된 감사 이력을 재구성한다.
func (s *HistoryStore) LoadHistory(ctx context.Context, aggregate audit.AggregateID) (audit.History, bool, error) {
	if err := aggregate.Validate(); err != nil {
		return audit.History{}, false, fmt.Errorf("%w: aggregate: %w", audit.ErrInvalidQuery, err)
	}
	entries, err := s.Find(ctx, audit.Query{Aggregate: &aggregate})
	if err != nil {
		return audit.History{}, false, err
	}
	if len(entries) == 0 {
		return audit.History{}, false, nil
	}
	history, err := audit.NewHistory(entries)
	if err != nil {
		return audit.History{}, false, fmt.Errorf("%w: history: %w", ErrInvalidEntry, err)
	}
	return history, true, nil
}

// Latest는 aggregate에 대해 영속화된 가장 높은 revision 엔트리를 반환한다.
func (s *HistoryStore) Latest(ctx context.Context, aggregate audit.AggregateID) (audit.Entry, bool, error) {
	if err := aggregate.Validate(); err != nil {
		return audit.Entry{}, false, fmt.Errorf("%w: aggregate: %w", audit.ErrInvalidQuery, err)
	}
	entries, err := s.Find(ctx, audit.Query{Aggregate: &aggregate, NewestFirst: true, Limit: 1})
	if err != nil {
		return audit.Entry{}, false, err
	}
	if len(entries) == 0 {
		return audit.Entry{}, false, nil
	}
	return entries[0], true, nil
}

// LatestSnapshot은 스냅샷 엔트리가 있으면 가장 최신 항목을 반환한다.
func (s *HistoryStore) LatestSnapshot(ctx context.Context, aggregate audit.AggregateID) (audit.Entry, bool, error) {
	return s.findSnapshot(ctx, aggregate, 0)
}

// PreviousSnapshot은 제외 상한 revision 이전의 가장 최신 스냅샷을 반환한다.
func (s *HistoryStore) PreviousSnapshot(ctx context.Context, aggregate audit.AggregateID, before audit.Revision) (audit.Entry, bool, error) {
	if err := before.Validate(); err != nil {
		return audit.Entry{}, false, fmt.Errorf("%w: before: %w", audit.ErrInvalidQuery, err)
	}
	return s.findSnapshot(ctx, aggregate, before)
}

func (s *HistoryStore) findSnapshot(ctx context.Context, aggregate audit.AggregateID, before audit.Revision) (audit.Entry, bool, error) {
	if err := aggregate.Validate(); err != nil {
		return audit.Entry{}, false, fmt.Errorf("%w: aggregate: %w", audit.ErrInvalidQuery, err)
	}
	entries, err := s.Find(ctx, audit.Query{Aggregate: &aggregate, NewestFirst: true})
	if err != nil {
		return audit.Entry{}, false, err
	}
	for _, entry := range entries {
		if before != 0 && entry.Revision >= before {
			continue
		}
		if entry.Snapshot != nil {
			return entry, true, nil
		}
	}
	return audit.Entry{}, false, nil
}

func buildHistoryFindQuery(query audit.Query) (string, []any) {
	var statement strings.Builder
	statement.WriteString(`select aggregate_type, aggregate_id, revision, event_id, idempotency_key, event_type, recorded_at, entry_json from audited_order_workflow_audit_entries`)
	conditions := make([]string, 0, 7)
	args := make([]any, 0, 7)
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if query.Aggregate != nil {
		add("aggregate_type = $%d", query.Aggregate.Type)
		add("aggregate_id = $%d", query.Aggregate.ID)
	}
	if query.AggregateType != "" {
		add("aggregate_type = $%d", query.AggregateType)
	}
	if query.FromRevision != 0 {
		add("revision >= $%d", query.FromRevision)
	}
	if query.ToRevision != 0 {
		add("revision <= $%d", query.ToRevision)
	}
	if !query.FromRecordedAt.IsZero() {
		add("recorded_at >= $%d", query.FromRecordedAt)
	}
	if !query.ToRecordedAt.IsZero() {
		add("recorded_at <= $%d", query.ToRecordedAt)
	}
	if len(conditions) > 0 {
		statement.WriteString(" where ")
		statement.WriteString(strings.Join(conditions, " and "))
	}
	if query.Aggregate != nil {
		statement.WriteString(" order by revision")
	} else {
		statement.WriteString(" order by position")
	}
	if query.NewestFirst {
		statement.WriteString(" desc")
	}
	if query.Limit > 0 {
		fmt.Fprintf(&statement, " limit %d", query.Limit)
	}
	return statement.String(), args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHistoryRows(rows *sql.Rows) ([]audit.Entry, error) {
	entries := make([]audit.Entry, 0)
	for rows.Next() {
		entry, err := scanHistoryRow(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history: %w", err)
	}
	return entries, nil
}

func scanHistoryRow(row rowScanner) (audit.Entry, error) {
	var (
		aggregateType  string
		aggregateID    string
		revision       int64
		eventID        string
		idempotencyKey string
		eventType      string
		recordedAt     sql.NullTime
		encoded        []byte
	)
	if err := row.Scan(&aggregateType, &aggregateID, &revision, &eventID, &idempotencyKey, &eventType, &recordedAt, &encoded); err != nil {
		return audit.Entry{}, err
	}
	if revision <= 0 || !recordedAt.Valid || len(encoded) == 0 || len(encoded) > maxEntryBytes {
		return audit.Entry{}, fmt.Errorf("%w: invalid scalar history", ErrInvalidEntry)
	}
	entry, err := audit.DecodeEntryJSON(encoded)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%w: decode: %w", ErrInvalidEntry, err)
	}
	if entry.Aggregate.Type != aggregateType || entry.Aggregate.ID != aggregateID ||
		entry.Revision != audit.Revision(revision) || string(entry.Event.EventID) != eventID ||
		entry.Event.IdempotencyKey != idempotencyKey || string(entry.Event.EventType) != eventType ||
		!entry.Event.RecordedAt.Equal(recordedAt.Time) {
		return audit.Entry{}, fmt.Errorf("%w: scalar json mismatch", ErrInvalidEntry)
	}
	return entry, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
