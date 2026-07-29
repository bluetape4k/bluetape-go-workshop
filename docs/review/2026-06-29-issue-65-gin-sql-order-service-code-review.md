# Code review: issue #65 Gin SQL order service

## 범위

- 새 runnable example: `examples/gin-sql-order-service`
- architecture, request sequence, rollback proof를 위한 새 README diagram
- root README navigation과 run instruction

## 발견 사항

P0=0 P1=0

## 검증 자료

- `go run ./examples/gin-sql-order-service`
- `go test -count=1 ./examples/gin-sql-order-service/...`
- `go test -race -count=1 ./examples/gin-sql-order-service/...`
- `golangci-lint cache clean && make ci`
- `xmllint --noout docs/images/readme-diagrams/gin-sql-order-service-architecture.svg docs/images/readme-diagrams/gin-sql-order-service-sequence.svg docs/images/readme-diagrams/gin-sql-order-service-rollback.svg`
- CairoSVG render for:
  - `docs/images/readme-diagrams/gin-sql-order-service-architecture.png`
  - `docs/images/readme-diagrams/gin-sql-order-service-sequence.png`
  - `docs/images/readme-diagrams/gin-sql-order-service-rollback.png`
- render 이후 clipping/overlap에 대한 PNG inspection
- root와 example README file 전반의 README local link/image check

## 메모

- Gin은 HTTP parsing, timeout, status code, stable public error를 소유한다.
- `Service`는 `sqlkit.WithTx`, repository call order, aggregate rollback을 소유한다.
- repository는 Gin과 transaction lifetime decision에서 분리되어 있다.
- local guidance가 참조한 diagram skill helper script는 `references/diagram-geometry-audit.py`와
  `references/diagram-endpoint-audit.py`에 없었다. fallback evidence로 SVG XML validation,
  CairoSVG render, PNG inspection을 사용했다.

## 잔여 Risk

- authentication, authorization, optimistic versioning, external migration orchestration,
  inventory, payment, fulfillment, outbox publication은 issue #65 implementation scope가 아니라
  문서화된 production follow-up이다.
