# Gin SQL order service 통합 예제

## 결정

focused SQL repository, SQL transaction boundary, Gin SQL CRUD example 뒤에 milestone-level
composition으로 `examples/gin-sql-order-service`를 추가한다. 이 예제는 Gin HTTP boundary가
하나의 order aggregate를 위해 여러 `sqlkit` repository를 조율하는 service-owned transaction을
호출하는 방식을 설명한다.

## 근거

Issue #65는 #62, #63, #64를 반복하면 안 된다. 새로운 reader question은 이것이다. 같은 order
workflow가 HTTP parsing, multi-table transaction ownership, reusable SQL repository를 동시에
필요로 하면 무엇이 달라지는가?

구현은 그 질문을 계속 보이게 유지한다.

- Gin handler는 JSON, path parameter, request timeout, public error code를 bind한다.
- `Service`는 `sqlkit.WithTx`, ID/time selection, repository call order, commit 또는
  rollback을 소유한다.
- `OrderRepository`, `ItemRepository`, `StatusRepository`는 각각 table SQL을 소유하고
  `sqlkit` execution/query interface를 받는다.
- deterministic `reject_after_items` flag는 payment, inventory, fulfillment, outbox scope를
  끌어오지 않고 rollback을 증명한다.

## 기각한 선택

- 이전 example의 sibling `internal` package를 reuse하는 방식. 해당 boundary는 각 example을
  자체적으로 runnable하고 inspectable하게 유지한다.
- inventory, payment, fulfillment, outbox, auth, pagination 추가. 그러면 issue #65 lesson이
  관련 없는 production concern 뒤에 숨는다.
- repository가 transaction을 열게 하는 방식. #63 service boundary lesson을 되돌리고 aggregate
  rollback 추론을 더 어렵게 만든다.

## 검증

- `go run ./examples/gin-sql-order-service`
- `go test -count=1 ./examples/gin-sql-order-service/...`
- `go test -race -count=1 ./examples/gin-sql-order-service/...`
- `golangci-lint cache clean && make ci`
- `xmllint --noout` on all Gin SQL order service diagrams
- CairoSVG render for all Gin SQL order service PNG diagrams
- PNG inspection for clipping, overlap, marker color, and label readability
