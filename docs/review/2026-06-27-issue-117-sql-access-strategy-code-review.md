# Code review: issue #117 SQL access strategy decision

## Scope

- New runnable example: `examples/sql-access-strategy-decision`
- New README diagrams for architecture and repository sequence
- Root README navigation and focused run instructions
- Lesson note for SQL access strategy boundary selection

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Direct `database/sql` path owns raw SQL, manual scan/cardinality behavior, and
  explicit `BeginTx` / `Commit` transaction ceremony.
- `sqlkit` path uses statement builders, `QueryOne`, `QueryOptional`, and
  `WithTx` without hiding SQL text or argument ordering.
- Generated query tools and migration tooling are documented as boundaries, not
  added as runtime dependencies.
- Testcontainers PostgreSQL tests cover no-row, too-many-row, rollback, context
  cancellation, and statement snapshot behavior.
- README diagrams render as PNG and keep architecture choice separate from the
  runtime repository sequence.

## Validation evidence

- `go run ./examples/sql-access-strategy-decision`
- `go test -count=1 ./examples/sql-access-strategy-decision/...`
- `go test -race -count=1 ./examples/sql-access-strategy-decision/...`
- `make ci`
- SVG XML parse plus CairoSVG render for
  `sql-access-strategy-decision-architecture.svg` and
  `sql-access-strategy-decision-sequence.svg`
- Local link/image existence check for root README files and the new example
  README pair
- `git diff --check`

## Residual risk

The example does not model service-layer HTTP behavior, isolation-level
selection, retry policy, migration execution, or generated query packages. Those
are documented as downstream choices so this example stays focused on the SQL
access decision.

The optional diagram geometry and endpoint audit helper scripts referenced by
the local diagram skill were not present at the installed path, so the diagram
gate used XML parsing, CairoSVG rendering, marker inspection, and full-size PNG
inspection instead.

## P0/P1 Gate

P0=0 P1=0
