# Code review: issue #62 SQL order repository

## Scope

- New runnable example: `examples/sql-order-repository`
- New README diagrams for repository ownership and insert/find/filter behavior
- Root README navigation and run instructions

## Findings

P0=0 P1=0

## Evidence

- `go run ./examples/sql-order-repository`
- `go test -count=1 ./examples/sql-order-repository/...`
- `go test -race -count=1 ./examples/sql-order-repository/...`
- `xmllint --noout docs/images/readme-diagrams/sql-order-repository-architecture.svg docs/images/readme-diagrams/sql-order-repository-sequence.svg`
- CairoSVG render for:
  - `docs/images/readme-diagrams/sql-order-repository-architecture.png`
  - `docs/images/readme-diagrams/sql-order-repository-sequence.png`
- PNG inspection for clipping/overlap after render
- README local link/image check: `checked 268 local markdown links/images across 4 files`

## Notes

- The repository keeps transaction ownership outside the example so #63 can
  teach transaction boundaries without rewriting the repository contract.
- Query behavior is asserted through PostgreSQL Testcontainers. SQL snapshots
  exist only as explanatory preview checks, not as the only correctness proof.
- The diagram skill helper scripts referenced by local guidance were not present
  at `references/diagram-geometry-audit.py` and
  `references/diagram-endpoint-audit.py`; SVG XML validation, CairoSVG render,
  and PNG inspection were used as fallback evidence.

## Residual Risk

- Production migration ownership, pagination tokens, optimistic concurrency, and
  observability policy are documented but intentionally outside #62.
