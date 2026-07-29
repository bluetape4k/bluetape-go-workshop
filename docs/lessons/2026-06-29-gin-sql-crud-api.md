# Gin SQL CRUD API 예제

## 결정

focused SQL repository와 transaction example 뒤에 `examples/gin-sql-crud-api`를 추가한다.
이 예제는 repository를 Gin package로 바꾸지 않으면서 public HTTP concern이 reusable
`sqlkit` repository와 만나는 지점을 설명한다.

## 근거

API는 작은 order resource에 대한 create, read, list, status update, delete endpoint를
노출한다. Gin handler는 JSON binding, path/query parsing, request-scoped timeout, HTTP status
code, public error code를 소유한다. repository는 `sqlkit`과 `database/sql`을 통해 SQL statement
construction, row scanning, not-found mapping을 소유한다.

test는 PostgreSQL Testcontainers를 사용하므로 handler response shape와 repository behavior가
real row에 대해 증명된다. repository도 직접 test하므로 Gin boundary가 SQL lesson의 hidden
dependency가 되지 않는다.

## 기각한 선택

- `examples/sql-order-repository/internal/orderrepo`를 import하는 방식. `internal` boundary는
  sibling example이 여기에 의존하지 못하게 하는 것이 맞다.
- full order-service integration. repository, transaction, HTTP service behavior를
  조합하는 #65에 속한다.
- 인증, 페이지네이션 토큰, 낙관적 버전 관리, production 마이그레이션 오케스트레이션.
  #64는 HTTP/SQL boundary가 주제이므로 README가 이를 production 후속 과제로 문서화한다.

## 검증

- `go run ./examples/gin-sql-crud-api`
- `go test -count=1 ./examples/gin-sql-crud-api/...`
- `go test -race -count=1 ./examples/gin-sql-crud-api/...`
- `golangci-lint cache clean && make ci`
- `xmllint --noout` on both Gin SQL CRUD API diagrams
- CairoSVG render for both Gin SQL CRUD API PNG diagrams
- PNG inspection for clipping/overlap after render
