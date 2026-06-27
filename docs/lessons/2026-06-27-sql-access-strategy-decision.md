# SQL access strategy decision example

Issue: #117

## Decision

Use a small local PostgreSQL-backed repository example to teach the SQL access
boundary before adding larger SQL HTTP examples. The example compares direct
`database/sql` with `sqlkit`, then documents when generated query layers and
migration tooling should stay outside the runtime dependency boundary.

## Why

`sqlkit` is easiest to understand when the reader can see the same operation
written both ways. Direct `database/sql` makes the ceremony visible: raw SQL
strings, manual row scanning, manual cardinality checks, and explicit
transaction lifecycle. `sqlkit` keeps SQL and args inspectable while reducing
the repeated row and transaction helper code.

The example intentionally avoids sqlc, Jet, Atlas, GORM, Bun, ent, and goqu as
runtime dependencies. They are useful product choices, but adding them here
would hide the v0.7.0 lesson: `sqlkit` is a small runtime helper, not an ORM,
schema metadata system, generator, or migration runner.

## Verification shape

- Repository tests assert direct and `sqlkit` read/write parity against
  PostgreSQL Testcontainers.
- Cardinality tests assert `sqlkit.ErrNoRows` and `sqlkit.ErrTooManyRows`.
- Transaction tests assert `sqlkit.WithTx` rolls back and preserves
  `ErrAuditRejected`.
- Cancellation tests assert canceled contexts reach the query path.
- README diagrams show the architecture decision and the repository sequence as
  separate reader questions so the boundary remains clear.
