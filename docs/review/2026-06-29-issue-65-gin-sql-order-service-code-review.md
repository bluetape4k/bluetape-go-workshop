# Code review: issue #65 Gin SQL order service

## Scope

- New runnable example: `examples/gin-sql-order-service`
- New README diagrams for architecture, request sequence, and rollback proof
- Root README navigation and run instructions

## Findings

P0=0 P1=0

## Evidence

- `go run ./examples/gin-sql-order-service`
- `go test -count=1 ./examples/gin-sql-order-service/...`
- `go test -race -count=1 ./examples/gin-sql-order-service/...`
- `golangci-lint cache clean && make ci`
- `xmllint --noout docs/images/readme-diagrams/gin-sql-order-service-architecture.svg docs/images/readme-diagrams/gin-sql-order-service-sequence.svg docs/images/readme-diagrams/gin-sql-order-service-rollback.svg`
- CairoSVG render for:
  - `docs/images/readme-diagrams/gin-sql-order-service-architecture.png`
  - `docs/images/readme-diagrams/gin-sql-order-service-sequence.png`
  - `docs/images/readme-diagrams/gin-sql-order-service-rollback.png`
- PNG inspection for clipping/overlap after render
- README local link/image check across root and example README files

## Notes

- Gin owns HTTP parsing, timeouts, status codes, and stable public errors.
- `Service` owns `sqlkit.WithTx`, repository call order, and aggregate rollback.
- Repositories remain free of Gin and transaction lifetime decisions.
- The diagram skill helper scripts referenced by local guidance were not present
  at `references/diagram-geometry-audit.py` and
  `references/diagram-endpoint-audit.py`; SVG XML validation, CairoSVG render,
  and PNG inspection were used as fallback evidence.

## Residual Risk

- Authentication, authorization, optimistic versioning, external migration
  orchestration, inventory, payment, fulfillment, and outbox publication are
  documented production follow-ups rather than issue #65 implementation scope.
