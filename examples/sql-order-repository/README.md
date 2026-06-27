# SQL Order Repository Example

This example turns the SQL access strategy lesson into a concrete order
repository. It uses `database/sql` for the real database session and
`github.com/bluetape4k/bluetape-go/sqlkit` for the small pieces that are easy to
repeat incorrectly: statement construction, row mapping, and one-row
cardinality.

The goal is not to hide SQL. The repository still exposes inspectable SQL text
and ordered arguments. `sqlkit` only removes mechanical boilerplate while the
application keeps ownership of context deadlines, `*sql.DB`, `*sql.Tx`,
migrations, and transaction boundaries.

![SQL order repository architecture](../../docs/images/readme-diagrams/sql-order-repository-architecture.png)

## Scenario

An order intake service needs a durable repository before the later HTTP and
transaction examples can reuse it. This example keeps the service boundary
small:

1. `Create` validates an order and inserts it into PostgreSQL.
2. `FindByID` reads exactly one order and reports `sqlkit.ErrNoRows` for a
   missing primary key.
3. `List` returns customer orders filtered by customer and status, ordered by
   `created_at` and `id`.

The tests run against PostgreSQL through Testcontainers. They assert repository
behavior instead of relying on brittle string-only SQL snapshots.

![SQL order repository sequence](../../docs/images/readme-diagrams/sql-order-repository-sequence.png)

## Repository Boundary

| Layer | Owns | Does not own |
| --- | --- | --- |
| Use case | Context, cancellation, service transaction choice, input scenario. | SQL string assembly or row scan boilerplate. |
| `Repository` | Domain validation, table contract, `Create`, `FindByID`, `List`, row mapping. | Connection pooling, migrations, HTTP routing, background workers. |
| `sqlkit` | Visible `Statement`, PostgreSQL placeholder rewrite, `QueryOne`, `QueryAll`. | ORM state, schema metadata, generated code, retry policy. |
| `database/sql` | Driver calls, `*sql.DB`, `*sql.Tx`, pooling, cancellation propagation. | Domain semantics. |

## Direct `database/sql` Contrast

Plain `database/sql` is still the right baseline. A direct implementation would:

- keep handwritten `insert`, `select`, and filter SQL strings near repository
  methods;
- call `QueryContext`, loop `rows.Next`, scan columns, close rows, and check
  `rows.Err`;
- manually decide what "zero rows" and "more than one row" mean for each
  method.

This example chooses `sqlkit` because the repository still shows the SQL, but
the repetitive cardinality and statement mechanics are shared. If a query
depends on driver-specific behavior, needs generated typed accessors, or grows
into a product-wide schema workflow, keep that decision outside this small
runtime helper.

## Run

Print the repository preview:

```bash
go run ./examples/sql-order-repository
```

The output is JSON. It includes the insert, find, and filtered-list SQL shapes,
the behavior contract, and production hardening notes:

```json
{
  "scenario": "Persist a customer order, read it by primary key, and list customer orders by status.",
  "repository": "Repository methods accept context plus caller-owned *sql.DB or *sql.Tx through sqlkit interfaces.",
  "behavior": [
    "Create validates the domain order and executes an inspectable INSERT.",
    "FindByID uses sqlkit.QueryOne, so missing rows return sqlkit.ErrNoRows.",
    "List uses sqlkit.QueryAll and returns domain rows ordered by created_at and id.",
    "The tests assert behavior through a real PostgreSQL database, not string-only snapshots."
  ]
}
```

Run the PostgreSQL contract tests:

```bash
go test -count=1 ./examples/sql-order-repository/...
```

Run the race gate for this example:

```bash
go test -race -count=1 ./examples/sql-order-repository/...
```

## What The Tests Prove

- Inserted rows can be read back with the same domain values.
- Customer and status filters return the expected order set in deterministic
  order.
- Missing primary keys return `sqlkit.ErrNoRows`.
- Invalid orders and empty filters fail before SQL execution.
- Context cancellation is propagated through the database call.

## Production Notes

- Choose a migration owner before deploying the repository. `sqlkit` does not
  create or migrate schemas.
- Keep transaction ownership at the service boundary. The next SQL transaction
  example can call the same repository with `*sql.Tx`.
- Add optimistic versioning before concurrent status updates.
- Add pagination contracts before this list method becomes a public API.
- Log SQL shape and correlation IDs without logging customer identifiers or
  raw argument values.
