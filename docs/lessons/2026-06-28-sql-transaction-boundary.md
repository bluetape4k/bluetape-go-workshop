# SQL transaction boundary 예제

## 결정

SQL order repository example 뒤에 `examples/sql-transaction-boundary`를 추가한다. 이 예제는
transaction lifetime이 individual repository 내부가 아니라 service boundary에 속한다는 점을
설명한다.

## 근거

service는 `sqlkit.WithTx`를 호출하고, 정렬된 product ID 순서로 product row를 lock하고, stock을
debit하고, order header와 line을 insert하며, 모든 business check가 성공할 때만 commit한다.
repository는 제공된 `*sql.Tx`를 받는다. repository가 transaction을 start, commit, rollback,
retry하거나 scope를 숨기지 않는다.

rollback behavior와 `select ... for update`는 real database에서 증명할 때 더 강하므로 test는
PostgreSQL Testcontainers를 사용한다. test는 commit state, insufficient-stock rollback,
post-debit payment rollback, context cancellation을 assert한다.

## 기각한 선택

- hidden unit-of-work abstraction. service가 transaction lifetime을 소유한다는 lesson을 흐린다.
- 이 issue에 Gin API를 넣는 방식. public SQL HTTP example은 #64와 #65에서 따로 추적한다.
- transaction 안에서 external payment service를 호출하는 방식. 예제는 deterministic failure
  flag를 사용해 network I/O 동안 lock을 잡지 않고 rollback을 설명한다.

## 검증

- `go run ./examples/sql-transaction-boundary`
- `go test -count=1 ./examples/sql-transaction-boundary/...`
- `go test -race -count=1 ./examples/sql-transaction-boundary/...`
- `xmllint --noout` on both SQL transaction boundary diagrams
- CairoSVG render for both SQL transaction boundary PNG diagrams
- README local link/image check
