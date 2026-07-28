# SQL order repository 예제

## 결정

0.7.0 SQL track의 첫 concrete SQL repository example로 `examples/sql-order-repository`를
추가한다. 앞선 `sql-access-strategy-decision` example은 tool choice에 집중하고, 이 예제는
이후 transaction 및 HTTP example이 reuse할 수 있는 repository contract를 설명한다.

## 근거

예제는 `database/sql` ownership을 보이게 유지한다. application code는 여전히 caller-owned
`*sql.DB` 또는 `*sql.Tx`를 전달하고, repository는 statement construction과 row cardinality에
`sqlkit`을 사용한다. 이 방식은 helper를 ORM, migration engine, generated query layer처럼
보이게 하지 않으면서 유용하게 만든다.

test는 PostgreSQL Testcontainers를 사용하므로 insert, find, filter, not-found, validation,
cancellation behavior가 SQL string snapshot만이 아니라 real SQL semantics를 통해 증명된다.

## 기각한 선택

- 이 issue에 Gin API를 넣는 방식. #64와 #65가 public HTTP surface를 다루므로 #62는
  repository-shaped로 남아야 한다.
- sqlc 또는 Jet 같은 generated-query tool. 그러면 lesson이 bluetape-go `sqlkit` helper
  usage에서 generated package ownership으로 이동한다.
- SQLite-only test. PostgreSQL placeholder behavior와 repository가 의도한 Testcontainers
  contract는 SQL track foundation의 일부다.

## 검증

- `go run ./examples/sql-order-repository`
- `go test -count=1 ./examples/sql-order-repository/...`
- `go test -race -count=1 ./examples/sql-order-repository/...`
- `xmllint --noout` on both SQL order repository diagrams
- CairoSVG render for both SQL order repository PNG diagrams
- README local link/image check
