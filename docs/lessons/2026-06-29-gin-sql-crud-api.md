# Gin SQL CRUD API example

## Decision

Add `examples/gin-sql-crud-api` after the focused SQL repository and
transaction examples. The example teaches where public HTTP concerns meet a
reusable `sqlkit` repository without turning the repository into a Gin package.

## Rationale

The API exposes create, read, list, status update, and delete endpoints for a
small order resource. Gin handlers own JSON binding, path/query parsing,
request-scoped timeouts, HTTP status codes, and public error codes. The
repository owns SQL statement construction, row scanning, and not-found mapping
through `sqlkit` plus `database/sql`.

Tests use PostgreSQL Testcontainers so handler response shapes and repository
behavior are proven against real rows. The repository is also tested directly so
the Gin boundary does not become a hidden dependency of the SQL lesson.

## Rejected

- Importing `examples/sql-order-repository/internal/orderrepo`. The `internal`
  boundary correctly prevents sibling examples from depending on it.
- A full order-service integration. That belongs to #65, where repository,
  transaction, and HTTP service behavior can be composed.
- Auth, pagination tokens, optimistic versioning, and production migration
  orchestration. README documents these as production follow-ups because #64 is
  about the HTTP/SQL boundary.

## Verification

- `go run ./examples/gin-sql-crud-api`
- `go test -count=1 ./examples/gin-sql-crud-api/...`
- `go test -race -count=1 ./examples/gin-sql-crud-api/...`
- `golangci-lint cache clean && make ci`
- `xmllint --noout` on both Gin SQL CRUD API diagrams
- CairoSVG render for both Gin SQL CRUD API PNG diagrams
- PNG inspection for clipping/overlap after render
