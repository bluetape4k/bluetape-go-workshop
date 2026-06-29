# Gin SQL Order Service Example

[English](README.md) | [한국어](README.ko.md)

This example composes the three focused SQL lessons into one small order
service. The earlier examples answer separate questions:

- [`sql-order-repository`](../sql-order-repository) shows how a repository can
  keep `sqlkit` statements visible without becoming an ORM.
- [`sql-transaction-boundary`](../sql-transaction-boundary) shows why the
  service, not the repository, should own `sqlkit.WithTx`.
- [`gin-sql-crud-api`](../gin-sql-crud-api) shows how a Gin boundary should
  keep HTTP parsing, timeouts, and public errors away from SQL code.

This module puts those boundaries together. Gin accepts an order workflow,
`Service` owns the multi-table transaction, and three repositories persist the
order header, item rows, and status history.

![Gin SQL order service architecture](../../docs/images/readme-diagrams/gin-sql-order-service-architecture.png)

## Scenario

The service models a realistic but intentionally small order aggregate:

1. `POST /orders` creates one order, one or more items, and the first status
   event inside one transaction.
2. `GET /orders/{id}` reads the aggregate with header, items, and status
   history.
3. `GET /orders/{id}/status` returns the current status plus the event trail.
4. `PATCH /orders/{id}/items/{item_id}` changes one quantity, recalculates the
   order total from item rows, and appends a status event in the same
   transaction.

The example deliberately stops before inventory reservation, payment capture,
fulfillment, outbox publication, or auth. Those concerns would be real in a
production order service, but adding them here would hide the lesson: where the
HTTP boundary, transaction boundary, and repository boundary meet.

![Gin SQL order service request sequence](../../docs/images/readme-diagrams/gin-sql-order-service-sequence.png)

## Boundary Model

| Layer | Owns | Does not own |
| --- | --- | --- |
| Gin handlers | JSON binding, path parameters, request timeout, public error codes, HTTP status. | SQL strings, transaction lifetime, aggregate recalculation. |
| `Service` | Use-case validation, ID/time selection, transaction lifetime, repository call order, commit or rollback decision. | Gin routing, table-specific SQL assembly, external side effects. |
| Repositories | Table contract, visible `sqlkit` statements, row mapping, not-found mapping. | HTTP response shape, opening or committing transactions. |
| PostgreSQL | Constraints, foreign keys, committed rows, rollback behavior. | Domain workflow policy. |

## Transaction Walkthrough

`Service.CreateOrder` validates the request before opening a transaction. Inside
`sqlkit.WithTx`, it inserts:

1. the order header;
2. each order item with a derived `line_total_cents`;
3. the first status event.

Returning nil from the transaction function commits all three table groups.
Returning an error rolls back the whole aggregate. The `reject_after_items`
field is a workshop fault-injection switch: it fails after item inserts but
before the status event so the tests can prove rollback without calling an
external payment, inventory, or fulfillment system.

![Gin SQL order service rollback path](../../docs/images/readme-diagrams/gin-sql-order-service-rollback.png)

## Run

Print the no-database preview:

```bash
go run ./examples/gin-sql-order-service
```

The preview includes the endpoint list, boundary notes, and representative
`sqlkit` statements for order creation, item update, total recalculation, and
status event append.

Run the HTTP service against PostgreSQL:

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable'
go run ./examples/gin-sql-order-service
```

The service listens on `127.0.0.1:8098` by default. Override it with
`HTTP_ADDR=127.0.0.1:8099`.

The example calls `Migrate` on startup so local runs are easy. Treat that as a
workshop convenience. Production services should run schema changes through a
separate migration owner before the HTTP process accepts traffic.

## Try The API

Create an order:

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

Read the aggregate and status trail:

```bash
ORDER_ID=$(curl -s -X POST http://127.0.0.1:8098/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-77","items":[{"sku":"sku-tea","quantity":1,"unit_price_cents":900}]}' \
  | jq -r '.order.id')

curl -s "http://127.0.0.1:8098/orders/${ORDER_ID}" | jq
curl -s "http://127.0.0.1:8098/orders/${ORDER_ID}/status" | jq
```

Update an item quantity and watch the order total change:

```bash
ITEM_ID=$(curl -s "http://127.0.0.1:8098/orders/${ORDER_ID}" | jq -r '.items[0].id')

curl -s -X PATCH "http://127.0.0.1:8098/orders/${ORDER_ID}/items/${ITEM_ID}" \
  -H 'Content-Type: application/json' \
  -d '{"quantity":3}' | jq
```

Prove rollback with the deterministic failure switch:

```bash
curl -s -X POST http://127.0.0.1:8098/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id": "customer-42",
    "items": [{"sku": "sku-coffee", "quantity": 1, "unit_price_cents": 1200}],
    "reject_after_items": true
  }' | jq
```

The expected public error code is `order_rejected`; the tests assert that no
order, item, or status rows remain after this path.

## Endpoints

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/healthz` | `200` | Process liveness only. |
| `POST` | `/orders` | `201` | Create order header, items, and first status event atomically. |
| `GET` | `/orders/{id}` | `200` | Read one order aggregate. |
| `GET` | `/orders/{id}/status` | `200` | Read current status and event history. |
| `PATCH` | `/orders/{id}/items/{item_id}` | `200` | Change item quantity, recalculate total, and append history. |

Stable public error codes include `invalid_request`, `order_not_found`,
`item_not_found`, `order_rejected`, and `service_error`.

## What The Tests Prove

Run the focused PostgreSQL-backed tests:

```bash
go test -count=1 ./examples/gin-sql-order-service/...
go test -race -count=1 ./examples/gin-sql-order-service/...
```

The tests use the bluetape-go PostgreSQL Testcontainers fixture and prove:

- preview statements keep the integrated boundary inspectable;
- the full HTTP workflow creates, reads, checks status, and updates item
  quantity against real PostgreSQL rows;
- the rollback fault leaves all three tables empty;
- validation, not-found, and conflict errors stay stable for callers;
- the service can run without Gin and context cancellation reaches SQL calls.

## Production Notes

- Add authentication and authorization before exposing customer order data.
- Move schema changes into a migration owner outside the HTTP process.
- Add optimistic versioning before concurrent item edits.
- Keep external inventory, payment, fulfillment, and outbox work outside this
  focused example until each boundary is intentionally designed.
- Record metrics for transaction duration, rollback reason, item update count,
  and status lookup latency without logging raw customer data.
