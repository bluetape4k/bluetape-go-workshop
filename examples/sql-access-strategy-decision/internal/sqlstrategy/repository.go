// Package sqlstrategy 는 직접 database/sql 사용과 sqlkit repository 형태를 비교한다.
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
	// ErrInvalidHold 는 누락되었거나 유효하지 않은 hold 입력을 나타낸다.
	ErrInvalidHold = errors.New("sqlstrategy: invalid hold")
	// ErrAuditRejected 는 테스트에서 트랜잭션 롤백을 증명하기 위해 사용된다.
	ErrAuditRejected = errors.New("sqlstrategy: audit rejected")
	// ErrDirectTooManyRows 는 직접 database/sql 조회의 cardinality 초과를 나타낸다.
	ErrDirectTooManyRows = errors.New("sqlstrategy: direct query returned too many rows")
)

// HoldStatus 는 예제가 사용하는 좁은 주문 hold 수명주기 상태다.
type HoldStatus string

const (
	// StatusPending 은 hold 가 생성됐지만 확정되지 않았음을 의미한다.
	StatusPending HoldStatus = "pending"
	// StatusConfirmed 는 hold 가 트랜잭션 안에서 확정됐음을 의미한다.
	StatusConfirmed HoldStatus = "confirmed"
)

// Hold 는 두 repository 전략이 함께 사용하는 도메인 행이다.
type Hold struct {
	ID       string     `json:"id"`
	Customer string     `json:"customer"`
	Status   HoldStatus `json:"status"`
}

// StatementSnapshot 은 검토 가능한 SQL 문과 인자 목록이다.
type StatementSnapshot struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// StrategyChoice 는 각 SQL 접근 방식을 언제 선택해야 하는지 문서화한다.
type StrategyChoice struct {
	Name     string `json:"name"`
	UseWhen  string `json:"use_when"`
	Boundary string `json:"boundary"`
}

// DecisionReport 는 실행 가능한 예제 출력이다.
type DecisionReport struct {
	Scenario   string              `json:"scenario"`
	Direct     []StatementSnapshot `json:"direct_database_sql"`
	SQLKit     []StatementSnapshot `json:"sqlkit"`
	Choices    []StrategyChoice    `json:"choices"`
	Production []string            `json:"production_hardening"`
}

// DirectRepository 는 호출자가 소유한 SQL 문자열과 database/sql 을 직접 사용한다.
type DirectRepository struct{}

// SQLKitRepository 는 sqlkit 문, 행 helper, 트랜잭션을 사용한다.
type SQLKitRepository struct{}

// NewDecisionReport 는 검토 가능한 SQL과 도구 선택 가이드를 구성한다.
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

// Create 는 직접 database/sql 로 hold 를 삽입한다.
func (DirectRepository) Create(ctx context.Context, db sqlkit.Execer, hold Hold) error {
	stmt, err := (DirectRepository{}).CreateStatement(hold)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, stmt.SQL, stmt.Args...)
	return err
}

// Find 는 직접 database/sql 과 수동 cardinality 검사로 hold 하나만 읽는다.
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

// Confirm 은 직접 SQL로 hold 를 확정 표시하고 감사 이벤트를 기록한다.
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

// CreateStatement 는 직접 SQL insert 형태를 반환한다.
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

// FindStatement 는 직접 SQL lookup 형태를 반환한다.
func (DirectRepository) FindStatement(id string) StatementSnapshot {
	return StatementSnapshot{
		Name: "direct.find_hold",
		SQL:  `select id, customer, status from sql_strategy_holds where id = $1`,
		Args: []any{id},
	}
}

// ConfirmStatement 는 직접 SQL update 형태를 반환한다.
func (DirectRepository) ConfirmStatement(id string) StatementSnapshot {
	return StatementSnapshot{
		Name: "direct.confirm_hold",
		SQL:  `update sql_strategy_holds set status = $1 where id = $2`,
		Args: []any{string(StatusConfirmed), id},
	}
}

// Create 는 sqlkit 으로 hold 를 삽입한다.
func (repo SQLKitRepository) Create(ctx context.Context, db sqlkit.Execer, hold Hold) error {
	stmt, err := repo.createSQL(hold)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ctx, db)
	return err
}

// Find 는 sqlkit.QueryOne 으로 hold 하나만 읽는다.
func (repo SQLKitRepository) Find(ctx context.Context, db sqlkit.Queryer, id string) (Hold, error) {
	stmt, err := repo.findSQL(id)
	if err != nil {
		return Hold{}, err
	}
	return sqlkit.QueryOne(ctx, db, stmt.SQL, scanHold, stmt.Args...)
}

// FindOnePendingByCustomer 는 sqlkit.QueryOptional cardinality 를 보여준다.
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

// ConfirmWithEvent 는 하나의 tx 안에서 hold 를 확정 표시하고 감사 이벤트를 기록한다.
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

// CreateStatement 는 sqlkit insert 형태를 반환한다.
func (repo SQLKitRepository) CreateStatement(hold Hold) (StatementSnapshot, error) {
	stmt, err := repo.createSQL(hold)
	return snapshot("sqlkit.create_hold", stmt, err)
}

// FindStatement 는 sqlkit lookup 형태를 반환한다.
func (repo SQLKitRepository) FindStatement(id string) (StatementSnapshot, error) {
	stmt, err := repo.findSQL(id)
	return snapshot("sqlkit.find_hold", stmt, err)
}

// ConfirmStatement 는 sqlkit update 형태를 반환한다.
func (repo SQLKitRepository) ConfirmStatement(id string) (StatementSnapshot, error) {
	stmt, err := repo.confirmSQL(id)
	return snapshot("sqlkit.confirm_hold", stmt, err)
}

// EventStatement 는 sqlkit 감사 insert 형태를 반환한다.
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
