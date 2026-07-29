# SQL access strategy decision 예제

Issue: #117

## 결정

더 큰 SQL HTTP example을 추가하기 전에 작은 local PostgreSQL-backed repository example로
SQL access boundary를 설명한다. 예제는 direct `database/sql`과 `sqlkit`을 비교한 뒤,
generated query layer와 migration tooling이 언제 runtime dependency boundary 밖에 남아야
하는지 문서화한다.

## 이유

reader가 같은 operation을 두 방식으로 볼 수 있을 때 `sqlkit`이 가장 이해하기 쉽다. direct
`database/sql`은 raw SQL string, manual row scanning, manual cardinality check, explicit
transaction lifecycle 같은 ceremony를 드러낸다. `sqlkit`은 SQL과 args를 inspectable하게
유지하면서 반복되는 row/transaction helper code를 줄인다.

예제는 runtime dependency로 sqlc, Jet, Atlas, GORM, Bun, ent, goqu를 의도적으로 피한다.
이들은 유용한 product choice지만, 여기 추가하면 v0.7.0 lesson을 숨긴다. `sqlkit`은 작은
runtime helper이지 ORM, schema metadata system, generator, migration runner가 아니다.

## 검증 형태

- repository test는 PostgreSQL Testcontainers에 대해 direct 방식과 `sqlkit` 방식의
  read/write parity를 assert한다.
- cardinality test는 `sqlkit.ErrNoRows`와 `sqlkit.ErrTooManyRows`를 assert한다.
- transaction test는 `sqlkit.WithTx`가 rollback하고 `ErrAuditRejected`를 보존함을 assert한다.
- cancellation test는 canceled context가 query path까지 도달함을 assert한다.
- README diagram은 architecture decision과 repository sequence를 별도 reader question으로
  보여 boundary를 분명하게 유지한다.
