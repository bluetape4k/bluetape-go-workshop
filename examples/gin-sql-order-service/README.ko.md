# Gin SQL Order Service 예제

[English](README.md) | [한국어](README.ko.md)

이 예제는 앞선 세 SQL 예제를 하나의 작은 주문 service로 합칩니다. 앞선 예제들은
각각 다른 질문에 답했습니다.

- [`sql-order-repository`](../sql-order-repository)는 `sqlkit` statement를
  눈에 보이게 유지하면서 repository가 ORM처럼 커지지 않게 만드는 방법을 보여줍니다.
- [`sql-transaction-boundary`](../sql-transaction-boundary)는 repository가 아니라
  service가 `sqlkit.WithTx`를 소유해야 하는 이유를 보여줍니다.
- [`gin-sql-crud-api`](../gin-sql-crud-api)는 Gin boundary가 HTTP parsing, timeout,
  public error를 SQL code와 분리하는 방법을 보여줍니다.

이 module은 그 boundary들을 함께 배치합니다. Gin은 주문 workflow를 받아들이고,
`Service`는 multi-table transaction을 소유하며, 세 repository는 order header,
item row, status history를 저장합니다.

![Gin SQL order service architecture](../../docs/images/readme-diagrams/gin-sql-order-service-architecture.png)

## 시나리오

서비스는 현실적인 모양의 주문 aggregate를 다루되 범위는 작게 유지합니다.

1. `POST /orders`는 주문 하나, item 하나 이상, 첫 status event를 한 transaction
   안에서 생성합니다.
2. `GET /orders/{id}`는 header, items, status history를 포함한 aggregate를 읽습니다.
3. `GET /orders/{id}/status`는 현재 status와 event trail을 반환합니다.
4. `PATCH /orders/{id}/items/{item_id}`는 item quantity를 바꾸고, item row에서 주문
   total을 다시 계산한 뒤, 같은 transaction 안에서 status event를 추가합니다.

이 예제는 inventory reservation, payment capture, fulfillment, outbox publication,
auth 앞에서 멈춥니다. Production order service라면 모두 중요한 주제입니다. 하지만
여기서까지 넣으면 핵심이 흐려집니다. 이 예제의 질문은 HTTP boundary, transaction
boundary, repository boundary가 어디에서 만나는가입니다.

![Gin SQL order service request sequence](../../docs/images/readme-diagrams/gin-sql-order-service-sequence.png)

## Boundary Model

| Layer | 소유하는 것 | 소유하지 않는 것 |
| --- | --- | --- |
| Gin handlers | JSON binding, path parameter, request timeout, public error code, HTTP status. | SQL string, transaction lifetime, aggregate recalculation. |
| `Service` | Use-case validation, ID/time 선택, transaction lifetime, repository 호출 순서, commit/rollback 결정. | Gin routing, table-specific SQL assembly, external side effect. |
| Repositories | Table contract, visible `sqlkit` statement, row mapping, not-found mapping. | HTTP response shape, transaction open/commit. |
| PostgreSQL | Constraint, foreign key, committed row, rollback behavior. | Domain workflow policy. |

## Transaction Walkthrough

`Service.CreateOrder`는 transaction을 열기 전에 request를 검증합니다.
`sqlkit.WithTx` 안에서는 다음 순서로 insert합니다.

1. order header;
2. derived `line_total_cents`를 가진 order item들;
3. 첫 status event.

Transaction function이 nil을 반환하면 세 table group이 함께 commit됩니다. Error를
반환하면 aggregate 전체가 rollback됩니다. `reject_after_items`는 workshop용
fault-injection switch입니다. Item insert 뒤, status event insert 전에 실패시켜서
external payment, inventory, fulfillment system 없이도 rollback을 증명합니다.

![Gin SQL order service rollback path](../../docs/images/readme-diagrams/gin-sql-order-service-rollback.png)

## 실행

Database 없이 preview를 출력합니다.

```bash
go run ./examples/gin-sql-order-service
```

Preview에는 endpoint 목록, boundary note, order creation, item update, total
recalculation, status event append에 대한 대표 `sqlkit` statement가 들어 있습니다.

PostgreSQL을 대상으로 HTTP service를 실행합니다.

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable'
go run ./examples/gin-sql-order-service
```

Service는 기본적으로 `127.0.0.1:8098`에서 listen합니다.
`HTTP_ADDR=127.0.0.1:8099`로 바꿀 수 있습니다.

예제는 local 실행을 쉽게 하기 위해 시작 시 `Migrate`를 호출합니다. 이것은 workshop
편의 기능입니다. Production service에서는 schema 변경을 별도 migration owner가
맡고, HTTP process가 traffic을 받기 전에 완료해야 합니다.

## API 호출

주문을 생성합니다.

```bash
curl -s -X POST http://127.0.0.1:8098/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id": "customer-42",
    "items": [
      {"sku": "sku-coffee", "quantity": 2, "unit_price_cents": 1200},
      {"sku": "sku-filter", "quantity": 1, "unit_price_cents": 1300}
    ]
  }' | jq
```

Aggregate와 status trail을 읽습니다.

```bash
ORDER_ID=$(curl -s -X POST http://127.0.0.1:8098/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-77","items":[{"sku":"sku-tea","quantity":1,"unit_price_cents":900}]}' \
  | jq -r '.order.id')

curl -s "http://127.0.0.1:8098/orders/${ORDER_ID}" | jq
curl -s "http://127.0.0.1:8098/orders/${ORDER_ID}/status" | jq
```

Item quantity를 바꾸고 order total이 바뀌는지 확인합니다.

```bash
ITEM_ID=$(curl -s "http://127.0.0.1:8098/orders/${ORDER_ID}" | jq -r '.items[0].id')

curl -s -X PATCH "http://127.0.0.1:8098/orders/${ORDER_ID}/items/${ITEM_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"quantity":3}' | jq
```

Deterministic failure switch로 rollback을 증명합니다.

```bash
curl -s -X POST http://127.0.0.1:8098/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id": "customer-42",
    "items": [{"sku": "sku-coffee", "quantity": 1, "unit_price_cents": 1200}],
    "reject_after_items": true
  }' | jq
```

예상 public error code는 `order_rejected`입니다. 테스트는 이 경로 뒤에 order,
item, status row가 하나도 남지 않는지 확인합니다.

## Endpoints

| Method | Path | Success | 목적 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | `200` | Process liveness 전용입니다. |
| `POST` | `/orders` | `201` | Order header, items, 첫 status event를 atomic하게 생성합니다. |
| `GET` | `/orders/{id}` | `200` | 주문 aggregate 하나를 읽습니다. |
| `GET` | `/orders/{id}/status` | `200` | 현재 status와 event history를 읽습니다. |
| `PATCH` | `/orders/{id}/items/{item_id}` | `200` | Item quantity 변경, total 재계산, history append를 수행합니다. |

안정적인 public error code는 `invalid_request`, `order_not_found`,
`item_not_found`, `order_rejected`, `service_error`입니다.

## 테스트가 증명하는 것

PostgreSQL-backed focused test를 실행합니다.

```bash
go test -count=1 ./examples/gin-sql-order-service/...
go test -race -count=1 ./examples/gin-sql-order-service/...
```

테스트는 bluetape-go PostgreSQL Testcontainers fixture를 사용하며 다음을 증명합니다.

- preview statement가 통합 boundary를 inspectable하게 유지합니다.
- 전체 HTTP workflow가 실제 PostgreSQL row에 대해 create, read, status 조회,
  item quantity update를 수행합니다.
- Rollback fault 뒤에 세 table이 모두 비어 있습니다.
- Validation, not-found, conflict error가 caller에게 안정적으로 보입니다.
- Service는 Gin 없이도 동작하고, context cancellation은 SQL call까지 전파됩니다.

## Production Notes

- Customer order data를 노출하기 전에 authentication과 authorization을 추가해야
  합니다.
- Schema 변경은 HTTP process 밖의 migration owner로 옮겨야 합니다.
- 동시 item edit가 필요해지기 전에 optimistic versioning을 추가합니다.
- 각 boundary를 의도적으로 설계하기 전까지 external inventory, payment,
  fulfillment, outbox 작업은 이 focused example 밖에 둡니다.
- Raw customer data를 로그에 남기지 말고 transaction duration, rollback reason,
  item update count, status lookup latency를 기록합니다.
