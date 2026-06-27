# SQL transaction boundary example

## Decision

Add `examples/sql-transaction-boundary` after the SQL order repository example.
The example teaches that transaction lifetime belongs in the service boundary,
not inside individual repositories.

## Rationale

The service calls `sqlkit.WithTx`, locks product rows in sorted product ID order,
debits stock, inserts the order header and lines, and commits only when every
business check succeeds. Repositories accept the provided `*sql.Tx`; they never
start, commit, roll back, retry, or hide transaction scope.

Tests use PostgreSQL Testcontainers because rollback behavior and `select ...
for update` are stronger when proven against a real database. The tests assert
commit state, insufficient-stock rollback, post-debit payment rollback, and
context cancellation.

## Rejected

- A hidden unit-of-work abstraction. It would obscure the lesson that the
  service owns transaction lifetime.
- A Gin API in this issue. Public SQL HTTP examples are tracked separately by
  #64 and #65.
- Calling an external payment service inside the transaction. The example uses
  a deterministic failure flag so it can teach rollback without holding locks
  across network I/O.

## Verification

- `go run ./examples/sql-transaction-boundary`
- `go test -count=1 ./examples/sql-transaction-boundary/...`
- `go test -race -count=1 ./examples/sql-transaction-boundary/...`
- `xmllint --noout` on both SQL transaction boundary diagrams
- CairoSVG render for both SQL transaction boundary PNG diagrams
- README local link/image check
