# Gin SQL CRUD API 예제

[English](README.md) | [한국어](README.ko.md)

이 예제는 작은 PostgreSQL 주문 repository 앞에 public Gin API를 둡니다.
앞선 SQL repository 예제의 다음 단계이지만, 독자의 질문은 바뀝니다.
"repository는 어떻게 만들까?"가 아니라 "HTTP parsing, public error, timeout,
SQL ownership은 어디서 만나야 할까?"를 보여줍니다.

답은 의도적으로 좁게 잡았습니다. Gin은 request parsing과 response projection을
소유합니다. Repository는 visible `sqlkit` statement를 소유합니다.
`database/sql`은 여전히 database session, pooling, driver behavior,
cancellation을 소유합니다.

![Gin SQL CRUD API architecture](../../docs/images/readme-diagrams/gin-sql-crud-api-architecture.png)

## 시나리오

더 큰 order-service integration 예제가 repository, transaction, HTTP behavior를
합치기 전에 작은 주문 service에는 CRUD endpoint가 필요합니다. 이 예제는 하나의
`orders` resource를 노출합니다.

1. `POST /orders`는 JSON을 검증하고 pending 주문을 생성합니다.
2. `GET /orders/{id}`는 주문 하나를 읽거나 안정적인 not-found error를 반환합니다.
3. `GET /orders?customer_id=...&status=...`는 조건에 맞는 주문을 deterministic
   order로 나열합니다.
4. `PATCH /orders/{id}/status`는 좁은 status field만 update합니다.
5. `DELETE /orders/{id}`는 주문 하나를 삭제하고 `204 No Content`를 반환합니다.

각 handler는 repository를 호출하기 전에 request-scoped timeout을 만듭니다.
Repository에는 Gin 의존성이 없으므로, 다음 예제에서 service transaction이나
background worker가 HTTP concern 없이 같은 repository를 호출할 수 있습니다.

![Gin SQL CRUD API sequence](../../docs/images/readme-diagrams/gin-sql-crud-api-sequence.png)

## Boundary Model

| Layer | 소유하는 것 | 소유하지 않는 것 |
| --- | --- | --- |
| Gin handler | JSON binding, path/query parsing, request timeout, public error code, HTTP status. | SQL string, row scanning, connection pooling, schema migration policy. |
| `Repository` | Domain validation, `Create`, `FindByID`, `List`, `UpdateStatus`, `Delete`, row mapping. | Gin routing, auth policy, transaction lifetime, retry policy. |
| `sqlkit` | Inspectable `Statement`, PostgreSQL placeholder rewrite, `QueryOne`, `QueryAll`. | ORM identity map, schema metadata, generated DAO layer. |
| `database/sql` | `*sql.DB`, `*sql.Tx`, driver call, pooling, cancellation propagation. | Domain 의미나 HTTP projection. |

## 실행

Database 없이 preview를 출력합니다.

```bash
go run ./examples/gin-sql-crud-api
```

PostgreSQL을 대상으로 HTTP service를 실행합니다.

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable'
go run ./examples/gin-sql-crud-api
```

Service는 기본적으로 `127.0.0.1:8097`에서 listen합니다.
`HTTP_ADDR=127.0.0.1:8098`로 바꿀 수 있습니다.

예제는 local 실행을 쉽게 하기 위해 시작 시 `Migrate`를 호출합니다. 이것은 workshop
편의 기능이지 production migration 전략이 아닙니다. 실제 service에서는 schema
변경을 별도 migration owner로 옮기고 HTTP process가 traffic을 받기 전에 실행해야
합니다.

## API 호출

주문을 생성합니다.

```bash
curl -s -X POST http://127.0.0.1:8097/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-42","total_cents":2599}' | jq
```

생성된 주문을 읽습니다.

```bash
ORDER_ID=$(curl -s -X POST http://127.0.0.1:8097/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-42","total_cents":4800}' | jq -r '.id')

curl -s "http://127.0.0.1:8097/orders/${ORDER_ID}" | jq
```

Customer 주문을 나열합니다.

```bash
curl -s 'http://127.0.0.1:8097/orders?customer_id=customer-42&status=pending&limit=20' | jq
```

Status를 갱신하고 삭제합니다.

```bash
curl -s -X PATCH "http://127.0.0.1:8097/orders/${ORDER_ID}/status" \
  -H 'Content-Type: application/json' \
  -d '{"status":"paid"}' | jq

curl -i -X DELETE "http://127.0.0.1:8097/orders/${ORDER_ID}"
```

안정적인 public error를 재현합니다.

```bash
curl -s 'http://127.0.0.1:8097/orders/missing' | jq

curl -s -X PATCH 'http://127.0.0.1:8097/orders/missing/status' \
  -H 'Content-Type: application/json' \
  -d '{"status":"shipped"}' | jq
```

예상 error code는 `invalid_request`, `order_not_found`, `repository_error`,
`id_generation_failed`입니다.

## Endpoints

| Method | Path | Success | 목적 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | `200` | Process liveness 전용입니다. |
| `POST` | `/orders` | `201` | Pending 주문을 생성합니다. |
| `GET` | `/orders/{id}` | `200` | 주문 하나를 읽습니다. |
| `GET` | `/orders` | `200` | `customer_id`, `status`, 또는 둘 다로 나열합니다. |
| `PATCH` | `/orders/{id}/status` | `200` | `pending`, `paid`, `cancelled`로 갱신합니다. |
| `DELETE` | `/orders/{id}` | `204` | 주문 하나를 삭제합니다. |

## 테스트가 증명하는 것

PostgreSQL-backed focused test를 실행합니다.

```bash
go test -count=1 ./examples/gin-sql-crud-api/...
go test -race -count=1 ./examples/gin-sql-crud-api/...
```

테스트는 bluetape-go PostgreSQL Testcontainers fixture를 사용하며 다음을 증명합니다.

- preview가 HTTP와 SQL boundary statement를 inspectable하게 유지합니다.
- 전체 HTTP CRUD flow가 실제 PostgreSQL row를 persist하고 다시 읽습니다.
- Public validation error와 not-found error가 안정적입니다.
- Repository는 Gin 없이도 동작합니다.
- Context cancellation이 repository call까지 전파됩니다.

## Production Notes

- Customer order data를 노출하기 전에 authentication과 authorization을 추가해야
  합니다.
- List endpoint를 scale 있게 쓰기 전에 작은 `limit` parameter를 stable pagination
  contract로 바꿔야 합니다.
- 동시 status update가 필요해지기 전에 optimistic versioning을 추가합니다.
- Schema migration ownership은 HTTP process 밖으로 옮깁니다.
- Raw customer data를 로그에 남기지 말고 handler latency, SQL latency, validation
  error count, not-found rate를 기록합니다.
