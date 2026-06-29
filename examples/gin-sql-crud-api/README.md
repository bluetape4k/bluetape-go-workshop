# Gin SQL CRUD API Example

[English](README.md) | [한국어](README.ko.md)

This example puts a public Gin API in front of a small PostgreSQL order
repository. It follows the SQL repository lesson, but changes the reader's
question from "how should the repository look?" to "where should HTTP parsing,
public errors, timeouts, and SQL ownership meet?"

The answer is deliberately narrow. Gin owns request parsing and response
projection. The repository owns visible `sqlkit` statements. `database/sql`
still owns the database session, pooling, driver behavior, and cancellation.

![Gin SQL CRUD API architecture](../../docs/images/readme-diagrams/gin-sql-crud-api-architecture.png)

## Scenario

A small order service needs CRUD endpoints before the larger order-service
integration example combines repositories, transactions, and HTTP behavior.
This example exposes one `orders` resource:

1. `POST /orders` validates JSON and creates a pending order.
2. `GET /orders/{id}` fetches one order or returns a stable not-found error.
3. `GET /orders?customer_id=...&status=...` lists matching orders in a
   deterministic order.
4. `PATCH /orders/{id}/status` updates a narrow status field.
5. `DELETE /orders/{id}` deletes one order and returns `204 No Content`.

Each handler creates a request-scoped timeout before calling the repository.
The repository has no Gin dependency, so later examples can call it from a
service transaction or background worker without pulling HTTP concerns along.

![Gin SQL CRUD API sequence](../../docs/images/readme-diagrams/gin-sql-crud-api-sequence.png)

## Boundary Model

| Layer | Owns | Does not own |
| --- | --- | --- |
| Gin handler | JSON binding, path/query parsing, request timeout, public error code, HTTP status. | SQL strings, row scanning, connection pooling, schema migration policy. |
| `Repository` | Domain validation, `Create`, `FindByID`, `List`, `UpdateStatus`, `Delete`, row mapping. | Gin routing, auth policy, transaction lifetime, retry policy. |
| `sqlkit` | Inspectable `Statement`, PostgreSQL placeholder rewrite, `QueryOne`, `QueryAll`. | ORM identity map, schema metadata, generated DAO layer. |
| `database/sql` | `*sql.DB`, `*sql.Tx`, driver calls, pooling, cancellation propagation. | Domain meaning or HTTP projection. |

## Run

Print the no-database preview:

```bash
go run ./examples/gin-sql-crud-api
```

Run the HTTP service against PostgreSQL:

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable'
go run ./examples/gin-sql-crud-api
```

The service listens on `127.0.0.1:8097` by default. Override it with
`HTTP_ADDR=127.0.0.1:8098`.

The example calls `Migrate` on startup so local runs are easy to try. Treat that
as a workshop convenience, not a production migration strategy. Production
services should move schema changes into a migration owner and run them before
the HTTP process serves traffic.

## Try The API

Create an order:

```bash
curl -s -X POST http://127.0.0.1:8097/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-42","total_cents":2599}' | jq
```

Read the generated order:

```bash
ORDER_ID=$(curl -s -X POST http://127.0.0.1:8097/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-42","total_cents":4800}' | jq -r '.id')

curl -s "http://127.0.0.1:8097/orders/${ORDER_ID}" | jq
```

List customer orders:

```bash
curl -s 'http://127.0.0.1:8097/orders?customer_id=customer-42&status=pending&limit=20' | jq
```

Update status and delete:

```bash
curl -s -X PATCH "http://127.0.0.1:8097/orders/${ORDER_ID}/status" \
  -H 'Content-Type: application/json' \
  -d '{"status":"paid"}' | jq

curl -i -X DELETE "http://127.0.0.1:8097/orders/${ORDER_ID}"
```

Try stable public errors:

```bash
curl -s 'http://127.0.0.1:8097/orders/missing' | jq

curl -s -X PATCH 'http://127.0.0.1:8097/orders/missing/status' \
  -H 'Content-Type: application/json' \
  -d '{"status":"shipped"}' | jq
```

Expected error codes include `invalid_request`, `order_not_found`,
`repository_error`, and `id_generation_failed`.

## Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/healthz` | `200` | Process liveness only. |
| `POST` | `/orders` | `201` | Create a pending order. |
| `GET` | `/orders/{id}` | `200` | Read one order. |
| `GET` | `/orders` | `200` | List by `customer_id`, `status`, or both. |
| `PATCH` | `/orders/{id}/status` | `200` | Update to `pending`, `paid`, or `cancelled`. |
| `DELETE` | `/orders/{id}` | `204` | Delete one order. |

## What The Tests Prove

Run the focused PostgreSQL-backed tests:

```bash
go test -count=1 ./examples/gin-sql-crud-api/...
go test -race -count=1 ./examples/gin-sql-crud-api/...
```

The tests use the bluetape-go PostgreSQL Testcontainers fixture and prove:

- the preview keeps HTTP and SQL boundary statements inspectable;
- the full HTTP CRUD flow persists and reads real PostgreSQL rows;
- public validation and not-found errors remain stable;
- the repository works without Gin;
- context cancellation propagates through repository calls.

## Production Notes

- Add authentication and authorization before exposing customer order data.
- Replace the small `limit` parameter with a stable pagination contract before
  using list endpoints at scale.
- Add optimistic versioning before concurrent status updates.
- Move schema migration ownership outside the HTTP process.
- Record handler latency, SQL latency, validation error counts, and not-found
  rates without logging raw customer data.
