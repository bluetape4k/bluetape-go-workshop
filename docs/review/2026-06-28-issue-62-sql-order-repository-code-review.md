# Code review: issue #62 SQL order repository

## 범위

- 새 runnable example: `examples/sql-order-repository`
- repository ownership과 insert/find/filter behavior를 위한 새 README diagram
- root README navigation과 run instruction

## 발견 사항

P0=0 P1=0

## Evidence

- `go run ./examples/sql-order-repository`
- `go test -count=1 ./examples/sql-order-repository/...`
- `go test -race -count=1 ./examples/sql-order-repository/...`
- `xmllint --noout docs/images/readme-diagrams/sql-order-repository-architecture.svg docs/images/readme-diagrams/sql-order-repository-sequence.svg`
- CairoSVG render for:
  - `docs/images/readme-diagrams/sql-order-repository-architecture.png`
  - `docs/images/readme-diagrams/sql-order-repository-sequence.png`
- render 이후 clipping/overlap에 대한 PNG inspection
- README local link/image check: `checked 268 local markdown links/images across 4 files`

## 메모

- repository는 transaction ownership을 예제 밖에 둔다. 그래야 #63이 repository contract를 다시
  쓰지 않고 transaction boundary를 설명할 수 있다.
- query behavior는 PostgreSQL Testcontainers로 assert한다. SQL snapshot은 explanatory preview
  check일 뿐 유일한 correctness proof가 아니다.
- local guidance가 참조한 diagram skill helper script는 `references/diagram-geometry-audit.py`와
  `references/diagram-endpoint-audit.py`에 없었다. fallback evidence로 SVG XML validation,
  CairoSVG render, PNG inspection을 사용했다.

## 잔여 Risk

- production migration ownership, pagination token, optimistic concurrency,
  observability policy는 문서화되어 있지만 #62 범위 밖이다.
