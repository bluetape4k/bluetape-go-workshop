# SQL order repository example

## Decision

Add `examples/sql-order-repository` as the first concrete SQL repository example
in the 0.7.0 SQL track. The earlier `sql-access-strategy-decision` example stays
focused on tool choice; this example teaches the repository contract that later
transaction and HTTP examples can reuse.

## Rationale

The example keeps `database/sql` ownership visible. Application code still
passes a caller-owned `*sql.DB` or `*sql.Tx`, while the repository uses `sqlkit`
for statement construction and row cardinality. This makes the helper useful
without presenting it as an ORM, migration engine, or generated query layer.

Tests use PostgreSQL Testcontainers so insert, find, filter, not-found,
validation, and cancellation behavior are proven against real SQL semantics
instead of SQL string snapshots alone.

## Rejected

- A Gin API in this issue. #64 and #65 cover public HTTP surfaces; #62 should
  stay repository-shaped.
- A generated-query tool such as sqlc or Jet. That would move the lesson from
  bluetape-go `sqlkit` helper usage to generated package ownership.
- SQLite-only tests. PostgreSQL placeholder behavior and the repository's
  intended Testcontainers contract are part of the SQL track foundation.

## Verification

- `go run ./examples/sql-order-repository`
- `go test -count=1 ./examples/sql-order-repository/...`
- `go test -race -count=1 ./examples/sql-order-repository/...`
- `xmllint --noout` on both SQL order repository diagrams
- CairoSVG render for both SQL order repository PNG diagrams
- README local link/image check
