package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const (
	ordersTable  = "audited_order_workflow_orders"
	historyTable = "audited_order_workflow_audit_entries"
	outboxTable  = "audited_order_workflow_outbox_records"
)

const createOrdersTableSQL = `
create table if not exists audited_order_workflow_orders (
    order_id varchar(128) primary key,
    status varchar(16) not null check (status in ('pending', 'confirmed', 'cancelled')),
    revision bigint not null check (revision >= 1),
    updated_at timestamptz not null
)`

const createHistoryTableSQL = `
create table if not exists audited_order_workflow_audit_entries (
    position bigint generated always as identity unique,
    aggregate_type varchar(128) not null,
    aggregate_id varchar(128) not null,
    revision bigint not null check (revision >= 1),
    event_id varchar(128) not null unique,
    idempotency_key varchar(128) not null unique,
    event_type varchar(128) not null,
    recorded_at timestamptz not null,
    entry_json jsonb not null,
    primary key (aggregate_type, aggregate_id, revision)
)`

const createHistoryTimeIndexSQL = `
create index if not exists audited_order_workflow_audit_entries_aggregate_time_idx
on audited_order_workflow_audit_entries
(aggregate_type, aggregate_id, recorded_at, revision)`

// CreateSchema 는 고정된 워크숍 스키마를 준비하고 호환되지 않는 테이블을 거부한다.
func CreateSchema(ctx context.Context, db sqlkit.Session, outbox *sqloutbox.Store) error {
	if db == nil || outbox == nil {
		return ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := verifyExistingSchemaCompatibility(ctx, db); err != nil {
		return err
	}
	for _, statement := range []string{createOrdersTableSQL, createHistoryTableSQL, createHistoryTimeIndexSQL} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create application schema: %w", err)
		}
	}
	if err := outbox.CreateSchema(ctx, db); err != nil {
		return fmt.Errorf("create outbox schema: %w", err)
	}
	if err := verifySchemaCompatibility(ctx, db); err != nil {
		return err
	}
	return nil
}

type expectedColumn struct {
	dataType  string
	nullable  bool
	maxLength int64
}

var expectedSchemas = map[string]map[string]expectedColumn{
	ordersTable: {
		"order_id":   {dataType: "character varying", maxLength: 128},
		"status":     {dataType: "character varying", maxLength: 16},
		"revision":   {dataType: "bigint"},
		"updated_at": {dataType: "timestamp with time zone"},
	},
	historyTable: {
		"position":        {dataType: "bigint"},
		"aggregate_type":  {dataType: "character varying", maxLength: 128},
		"aggregate_id":    {dataType: "character varying", maxLength: 128},
		"revision":        {dataType: "bigint"},
		"event_id":        {dataType: "character varying", maxLength: 128},
		"idempotency_key": {dataType: "character varying", maxLength: 128},
		"event_type":      {dataType: "character varying", maxLength: 128},
		"recorded_at":     {dataType: "timestamp with time zone"},
		"entry_json":      {dataType: "jsonb"},
	},
	outboxTable: {
		"id":               {dataType: "bigint"},
		"status":           {dataType: "text"},
		"aggregate_type":   {dataType: "text"},
		"aggregate_id":     {dataType: "text"},
		"revision":         {dataType: "bigint"},
		"event_id":         {dataType: "text"},
		"idempotency_key":  {dataType: "text"},
		"event_type":       {dataType: "text"},
		"occurred_at":      {dataType: "timestamp with time zone"},
		"recorded_at":      {dataType: "timestamp with time zone"},
		"schema_version":   {dataType: "integer"},
		"entry_json":       {dataType: "jsonb"},
		"attempts":         {dataType: "integer"},
		"available_at":     {dataType: "timestamp with time zone"},
		"claimed_at":       {dataType: "timestamp with time zone", nullable: true},
		"published_at":     {dataType: "timestamp with time zone", nullable: true},
		"dead_lettered_at": {dataType: "timestamp with time zone", nullable: true},
		"last_error":       {dataType: "text"},
		"created_at":       {dataType: "timestamp with time zone"},
		"updated_at":       {dataType: "timestamp with time zone"},
	},
}

var expectedConstraints = map[string]map[string]string{
	ordersTable: {
		"audited_order_workflow_orders_pkey":           "PRIMARY KEY",
		"audited_order_workflow_orders_revision_check": "CHECK",
		"audited_order_workflow_orders_status_check":   "CHECK",
	},
	historyTable: {
		"audited_order_workflow_audit_entries_pkey":                "PRIMARY KEY",
		"audited_order_workflow_audit_entries_position_key":        "UNIQUE",
		"audited_order_workflow_audit_entries_event_id_key":        "UNIQUE",
		"audited_order_workflow_audit_entries_idempotency_key_key": "UNIQUE",
		"audited_order_workflow_audit_entries_revision_check":      "CHECK",
	},
	outboxTable: {
		"audited_order_workflow_outbox_records_pkey":                "PRIMARY KEY",
		"audited_order_workflow_outbox_records_event_id_key":        "UNIQUE",
		"audited_order_workflow_outbox_records_idempotency_key_key": "UNIQUE",
	},
}

func verifyExistingSchemaCompatibility(ctx context.Context, db sqlkit.Session) error {
	for table, expected := range expectedSchemas {
		var relation sql.NullString
		if err := db.QueryRowContext(ctx, `select to_regclass($1)`, "public."+table).Scan(&relation); err != nil {
			return fmt.Errorf("%w: inspect table %s", ErrInvalidConfig, table)
		}
		if !relation.Valid {
			continue
		}
		if err := verifyTableColumns(ctx, db, table, expected); err != nil {
			return fmt.Errorf("%w: incompatible table %s", ErrInvalidConfig, table)
		}
		if err := verifyTableConstraints(ctx, db, table, expectedConstraints[table]); err != nil {
			return fmt.Errorf("%w: incompatible table %s", ErrInvalidConfig, table)
		}
	}
	return nil
}

func verifySchemaCompatibility(ctx context.Context, db sqlkit.Queryer) error {
	for table, expected := range expectedSchemas {
		if err := verifyTableColumns(ctx, db, table, expected); err != nil {
			return fmt.Errorf("%w: incompatible table %s", ErrInvalidConfig, table)
		}
		if err := verifyTableConstraints(ctx, db, table, expectedConstraints[table]); err != nil {
			return fmt.Errorf("%w: incompatible table %s", ErrInvalidConfig, table)
		}
	}
	return nil
}

func verifyTableColumns(ctx context.Context, db sqlkit.Queryer, table string, expected map[string]expectedColumn) (result error) {
	rows, err := db.QueryContext(ctx, `
select column_name, data_type, is_nullable, character_maximum_length
from information_schema.columns
where table_schema = 'public' and table_name = $1`, table)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, rows.Close()) }()
	actual := make(map[string]expectedColumn, len(expected))
	for rows.Next() {
		var name, dataType, nullable string
		var maxLength sql.NullInt64
		if err := rows.Scan(&name, &dataType, &nullable, &maxLength); err != nil {
			return err
		}
		actual[name] = expectedColumn{dataType: dataType, nullable: nullable == "YES", maxLength: maxLength.Int64}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return ErrInvalidConfig
	}
	for name, want := range expected {
		got, ok := actual[name]
		if !ok || got != want {
			return ErrInvalidConfig
		}
	}
	return nil
}

func verifyTableConstraints(ctx context.Context, db sqlkit.Queryer, table string, expected map[string]string) (result error) {
	rows, err := db.QueryContext(ctx, `
select constraint_name, constraint_type
from information_schema.table_constraints
where table_schema = 'public' and table_name = $1`, table)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, rows.Close()) }()
	actual := make(map[string]string, len(expected))
	for rows.Next() {
		var name, constraintType string
		if err := rows.Scan(&name, &constraintType); err != nil {
			return err
		}
		actual[name] = constraintType
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for name, want := range expected {
		if actual[name] != want {
			return ErrInvalidConfig
		}
	}
	return nil
}
