# Code review: issue #63 SQL transaction boundary

## 범위

- 새 runnable example: `examples/sql-transaction-boundary`
- transaction ownership과 commit/rollback sequence를 위한 새 README diagram
- root README navigation과 run instruction

## 발견 사항

P0=0 P1=0

## Evidence

- `go run ./examples/sql-transaction-boundary`
- `go test -count=1 ./examples/sql-transaction-boundary/...`
- `go test -race -count=1 ./examples/sql-transaction-boundary/...`
- `xmllint --noout docs/images/readme-diagrams/sql-transaction-boundary-architecture.svg docs/images/readme-diagrams/sql-transaction-boundary-sequence.svg`
- CairoSVG render for:
  - `docs/images/readme-diagrams/sql-transaction-boundary-architecture.png`
  - `docs/images/readme-diagrams/sql-transaction-boundary-sequence.png`
- render 이후 clipping/overlap에 대한 PNG inspection
- README local link/image check: `checked 274 local markdown links/images across 4 files`

## 메모

- `Service.PlaceOrder`는 `sqlkit.WithTx`의 유일한 owner다. repository는 제공된 `*sql.Tx` 위에서
  narrow statement executor로 남는다.
- rollback은 stock conflict 이후와 stock debit 이후 발생하는 injected payment failure 뒤에 모두
  증명된다.
- local guidance가 참조한 diagram skill helper script는 `references/diagram-geometry-audit.py`와
  `references/diagram-endpoint-audit.py`에 없었다. fallback evidence로 SVG XML validation,
  CairoSVG render, PNG inspection을 사용했다.

## 잔여 Risk

- isolation-level tuning, retry policy, outbox publication, lock-wait metric은 #63
  implementation scope가 아니라 문서화된 production follow-up이다.
