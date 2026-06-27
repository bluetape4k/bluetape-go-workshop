# Code review: issue #63 SQL transaction boundary

## Scope

- New runnable example: `examples/sql-transaction-boundary`
- New README diagrams for transaction ownership and commit/rollback sequence
- Root README navigation and run instructions

## Findings

P0=0 P1=0

## Evidence

- `go run ./examples/sql-transaction-boundary`
- `go test -count=1 ./examples/sql-transaction-boundary/...`
- `go test -race -count=1 ./examples/sql-transaction-boundary/...`
- `xmllint --noout docs/images/readme-diagrams/sql-transaction-boundary-architecture.svg docs/images/readme-diagrams/sql-transaction-boundary-sequence.svg`
- CairoSVG render for:
  - `docs/images/readme-diagrams/sql-transaction-boundary-architecture.png`
  - `docs/images/readme-diagrams/sql-transaction-boundary-sequence.png`
- PNG inspection for clipping/overlap after render
- README local link/image check: `checked 274 local markdown links/images across 4 files`

## Notes

- `Service.PlaceOrder` is the only owner of `sqlkit.WithTx`; repositories remain
  narrow statement executors over the provided `*sql.Tx`.
- Rollback is proven after a stock conflict and after an injected payment
  failure that occurs after stock debit.
- The diagram skill helper scripts referenced by local guidance were not present
  at `references/diagram-geometry-audit.py` and
  `references/diagram-endpoint-audit.py`; SVG XML validation, CairoSVG render,
  and PNG inspection were used as fallback evidence.

## Residual Risk

- Isolation-level tuning, retry policy, outbox publication, and lock-wait
  metrics are documented production follow-ups rather than #63 implementation
  scope.
