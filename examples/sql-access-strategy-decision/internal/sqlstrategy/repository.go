// Package sqlstrategy compares direct database/sql and sqlkit repository shapes.
package sqlstrategy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const (
	holdsTable  = "sql_strategy_holds"
	eventsTable = "sql_strategy_hold_events"
)

var (
	// ErrInvalidHold reports missing or invalid hold input.
	ErrInvalidHold = errors.New("sqlstrategy: invalid hold")
	// ErrAuditRejected is used by tests to prove transaction rollback.
	ErrAuditRejected = errors.New("sqlstrategy: audit rejected")
	// ErrDirectTooManyRows reports direct database/sql cardinality overflow.
	ErrDirectTooManyRows = errors.New("sqlstrategy: direct query returned too many rows")
)

// HoldStatus is a narrow order-hold lifecycle state used by the example.
type HoldStatus string

const (
	// StatusPending means the hold is created but not confirmed.
	StatusPending HoldStatus = "pending"
	// StatusConfirmed means the hold was confirmed inside a transaction.
	StatusConfirmed HoldStatus = "confirmed"
)

// Hold is the domain row used by both repository strategies.
type Hold struct {
	ID       string     `json:"id"`
	Customer string     `json:"customer"`
	Status   HoldStatus `json:"status"`
}

// StatementSnapshot is an inspectable SQL statement plus argument list.
type StatementSnapshot struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// StrategyChoice documents when each SQL access style should be selected.
type StrategyChoice struct {
	Name     string `json:"name"`
	UseWhen  string `json:"use_when"`
	Boundary string `json:"boundary"`
}

// DecisionReport is the runnable example output.
type DecisionReport struct {
	Scenario   string              `json:"scenario"`
	Direct     []StatementSnapshot `json:"direct_database_sql"`
	SQLKit     []StatementSnapshot `json:"sqlkit"`
	Choices    []StrategyChoice    `json:"choices"`
	Production []string            `json:"production_hardening"`
}

// DirectRepository uses caller-owned SQL strings and database/sql directly.
type DirectRepository struct{}

// SQLKitRepository uses sqlkit statements, row helpers, and transactions.
type SQLKitRepository struct{}

// NewDecisionReport builds inspectable SQL and tool-selection guidance.
func NewDecisionReport(sample Hold) (DecisionReport, error) {
	if err := validateHold(sample); err != nil {
		return DecisionReport{}, err
	}
	direct := DirectRepository{}
	sqlkitRepo := SQLKitRepository{}

	directCreate, err := direct.CreateStatement(sample)
	if err != nil {
		return DecisionReport{}, err
	}
	directFind := direct.FindStatement(sample.ID)
	directConfirm := direct.ConfirmStatement(sample.ID)

	sqlkitCreate, err := sqlkitRepo.CreateStatement(sample)
	if err != nil {
		return DecisionReport{}, err
	}
	sqlkitFind, err := sqlkitRepo.FindStatement(sample.ID)
	if err != nil {
		return DecisionReport{}, err
	}
	sqlkitConfirm, err := sqlkitRepo.ConfirmStatement(sample.ID)
	if err != nil {
		return DecisionReport{}, err
	}
	sqlkitEvent, err := sqlkitRepo.EventStatement(sample.ID, "confirmed")
	if err != nil {
		return DecisionReport{}, err
	}

	return DecisionReport{
		Scenario: "Create a customer order hold, read it with one-row cardinality, and confirm it with an audit event in one transaction.",
		Direct:   []StatementSnapshot{directCreate, directFind, directConfirm},
		SQLKit:   []StatementSnapshot{sqlkitCreate, sqlkitFind, sqlkitConfirm, sqlkitEvent},
		Choices: []StrategyChoice{
			{
				Name:     "direct database/sql",
				UseWhen:  "the query surface is tiny, driver behavior matters, or raw SQL is clearer than helpers",
				Boundary: "caller owns SQL strings, scanning, cardinality checks, and transaction ceremony",
			},
			{
				Name:     "sqlkit",
				UseWhen:  "the application wants visible SQL with shared transaction, row mapping, cardinality, and simple PostgreSQL-first builders",
				Boundary: "runtime helper only; no schema metadata, migrations, generated models, or ORM state",
			},
			{
				Name:     "sqlc or Jet",
				UseWhen:  "stable handwritten SQL or schema-derived builders need generated typed methods",
				Boundary: "application-owned generated package; keep output outside sqlkit and review generated source deliberately",
			},
			{
				Name:     "Atlas",
				UseWhen:  "schema diff, migration planning, linting, or apply workflow is the product need",
				Boundary: "external tool or CI/CD boundary; bluetape-go documents it but does not wrap migration execution",
			},
		},
		Production: []string{
			"choose one migration owner before adding repository code",
			"keep transaction ownership explicit at service boundaries",
			"log SQL shape and decision metadata without leaking argument values",
			"add retry, lock, isolation, and observability policy per database",
		},
	}, nil
}

// Create inserts a hold with direct database/sql.
func (DirectRepository) Create(ctx context.Context, db sqlkit.Execer, hold Hold) error {
	stmt, err := (DirectRepository{}).CreateStatement(hold)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, stmt.SQL, stmt.Args...)
	return err
}

// Find reads exactly one hold with direct database/sql and manual cardinality.
func (DirectRepository) Find(ctx context.Context, db sqlkit.Queryer, id string) (result Hold, err error) {
	stmt := (DirectRepository{}).FindStatement(id)
	rows, err := db.QueryContext(ctx, stmt.SQL, stmt.Args...)
	if err != nil {
		return Hold{}, err
	}
	defer func() {
		if closeErr := rows.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	var found []Hold
	for rows.Next() {
		hold, err := scanHold(rows)
		if err != nil {
			return Hold{}, err
		}
		found = append(found, hold)
		if len(found) > 1 {
			return Hold{}, ErrDirectTooManyRows
		}
	}
	if err := rows.Err(); err != nil {
		return Hold{}, err
	}
	if len(found) == 0 {
		return Hold{}, sql.ErrNoRows
	}
	return found[0], nil
}

// Confirm marks a hold confirmed and writes an audit event with direct SQL.
func (repo DirectRepository) Confirm(ctx context.Context, db *sql.DB, id string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt := repo.ConfirmStatement(id)
	if _, err := tx.ExecContext(ctx, stmt.SQL, stmt.Args...); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into sql_strategy_hold_events (hold_id, kind) values ($1, $2)`, id, "confirmed"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// CreateStatement returns the direct SQL insert shape.
func (DirectRepository) CreateStatement(hold Hold) (StatementSnapshot, error) {
	if err := validateHold(hold); err != nil {
		return StatementSnapshot{}, err
	}
	return StatementSnapshot{
		Name: "direct.create_hold",
		SQL:  `insert into sql_strategy_holds (id, customer, status) values ($1, $2, $3)`,
		Args: []any{hold.ID, hold.Customer, string(hold.Status)},
	}, nil
}

// FindStatement returns the direct SQL lookup shape.
func (DirectRepository) FindStatement(id string) StatementSnapshot {
	return StatementSnapshot{
		Name: "direct.find_hold",
		SQL:  `select id, customer, status from sql_strategy_holds where id = $1`,
		Args: []any{id},
	}
}

// ConfirmStatement returns the direct SQL update shape.
func (DirectRepository) ConfirmStatement(id string) StatementSnapshot {
	return StatementSnapshot{
		Name: "direct.confirm_hold",
		SQL:  `update sql_strategy_holds set status = $1 where id = $2`,
		Args: []any{string(StatusConfirmed), id},
	}
}

// Create inserts a hold with sqlkit.
func (repo SQLKitRepository) Create(ctx context.Context, db sqlkit.Execer, hold Hold) error {
	stmt, err := repo.createSQL(hold)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// Find reads exactly one hold with sqlkit.QueryOne.
func (repo SQLKitRepository) Find(ctx context.Context, db sqlkit.Queryer, id string) (Hold, error) {
	stmt, err := repo.findSQL(id)
	if err != nil {
		return Hold{}, err
	}
	return sqlkit.QueryOne(ctx, db, stmt.SQL, scanHold, stmt.Args...)
}

// FindOnePendingByCustomer demonstrates sqlkit.QueryOptional cardinality.
func (repo SQLKitRepository) FindOnePendingByCustomer(ctx context.Context, db sqlkit.Queryer, customer string) (Hold, bool, error) {
	stmt, err := sqlkit.SelectFrom(holdsTable).
		Columns("id", "customer", "status").
		Where("customer = ?", customer).
		Where("status = ?", string(StatusPending)).
		OrderBy("id").
		Build()
	if err != nil {
		return Hold{}, false, err
	}
	return sqlkit.QueryOptional(ctx, db, stmt.SQL, scanHold, stmt.Args...)
}

// ConfirmWithEvent marks a hold confirmed and records an audit event in one tx.
func (repo SQLKitRepository) ConfirmWithEvent(ctx context.Context, db *sql.DB, id string, rejectAudit bool) error {
	return sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
		confirm, err := repo.confirmSQL(id)
		if err != nil {
			return err
		}
		if _, err := confirm.Exec(ctx, tx); err != nil {
			return err
		}
		if rejectAudit {
			return ErrAuditRejected
		}
		event, err := repo.eventSQL(id, "confirmed")
		if err != nil {
			return err
		}
		_, err = event.Exec(ctx, tx)
		return err
	})
}

// CreateStatement returns the sqlkit insert shape.
func (repo SQLKitRepository) CreateStatement(hold Hold) (StatementSnapshot, error) {
	stmt, err := repo.createSQL(hold)
	return snapshot("sqlkit.create_hold", stmt, err)
}

// FindStatement returns the sqlkit lookup shape.
func (repo SQLKitRepository) FindStatement(id string) (StatementSnapshot, error) {
	stmt, err := repo.findSQL(id)
	return snapshot("sqlkit.find_hold", stmt, err)
}

// ConfirmStatement returns the sqlkit update shape.
func (repo SQLKitRepository) ConfirmStatement(id string) (StatementSnapshot, error) {
	stmt, err := repo.confirmSQL(id)
	return snapshot("sqlkit.confirm_hold", stmt, err)
}

// EventStatement returns the sqlkit audit insert shape.
func (repo SQLKitRepository) EventStatement(id, kind string) (StatementSnapshot, error) {
	stmt, err := repo.eventSQL(id, kind)
	return snapshot("sqlkit.add_event", stmt, err)
}

func (SQLKitRepository) createSQL(hold Hold) (sqlkit.Statement, error) {
	if err := validateHold(hold); err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.InsertInto(holdsTable).
		Columns("id", "customer", "status").
		Values(hold.ID, hold.Customer, string(hold.Status)).
		Build()
}

func (SQLKitRepository) findSQL(id string) (sqlkit.Statement, error) {
	return sqlkit.SelectFrom(holdsTable).
		Columns("id", "customer", "status").
		Where("id = ?", id).
		Build()
}

func (SQLKitRepository) confirmSQL(id string) (sqlkit.Statement, error) {
	return sqlkit.Update(holdsTable).
		Set("status", string(StatusConfirmed)).
		Where("id = ?", id).
		Build()
}

func (SQLKitRepository) eventSQL(id, kind string) (sqlkit.Statement, error) {
	return sqlkit.InsertInto(eventsTable).
		Columns("hold_id", "kind").
		Values(id, kind).
		Build()
}

func validateHold(hold Hold) error {
	switch {
	case hold.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidHold)
	case hold.Customer == "":
		return fmt.Errorf("%w: customer is required", ErrInvalidHold)
	case hold.Status == "":
		return fmt.Errorf("%w: status is required", ErrInvalidHold)
	default:
		return nil
	}
}

func scanHold(rows *sql.Rows) (Hold, error) {
	var hold Hold
	var status string
	if err := rows.Scan(&hold.ID, &hold.Customer, &status); err != nil {
		return Hold{}, err
	}
	hold.Status = HoldStatus(status)
	return hold, nil
}

func snapshot(name string, stmt sqlkit.Statement, err error) (StatementSnapshot, error) {
	if err != nil {
		return StatementSnapshot{}, err
	}
	return StatementSnapshot{Name: name, SQL: stmt.SQL, Args: stmt.Args}, nil
}
