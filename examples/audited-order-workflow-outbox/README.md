# Audited Order Workflow with SQL Outbox

English | [한국어](README.ko.md)

This runnable Gin service keeps mutable order state, immutable audit history,
and an official SQL outbox record in one PostgreSQL transaction. A supervised
`sqloutbox.Relay` publishes committed records to Redis Streams later. Command
responses therefore confirm durable PostgreSQL commit, not Redis delivery.

![Audited order workflow architecture](../../docs/images/readme-diagrams/audited-order-workflow-outbox-architecture.png)

## Package lesson

| Component | Responsibility |
| --- | --- |
| Gin adapter | Strict POST JSON, 32 KiB body limit, two-second operation deadline, stable errors, and 32 in-flight requests. |
| `orderworkflow.Service` | Validation, state transitions, row locking, idempotent replay, and one `sqlkit.WithTx`. |
| Order/history/outbox tables | Current projection, immutable reader source, and durable transport backlog. |
| `sqloutbox.Relay` | Bounded claim, retry, dead-letter, and continuous lifecycle. |
| Redis Streams adapter | At-least-once transport after PostgreSQL commit. |

![Audited order workflow sequence](../../docs/images/readme-diagrams/audited-order-workflow-outbox-sequence.png)

Create starts at `pending` revision 1. `confirm` moves pending to `confirmed`;
`cancel` accepts pending or confirmed and produces `cancelled`. A terminal order
rejects later transitions with `409 invalid_transition`.

The command ID is both `event_id` and `idempotency_key`. Retrying the same
canonical command returns the original committed order with `replayed: true`
and creates no history or outbox row. Reusing it for a different order, action,
reason, or metadata returns 409.

When all 32 request slots are occupied, the next request is rejected before
JSON decoding with `429 too_many_requests`, `Retry-After: 1`, and
`Connection: close`. Reusing the same command ID makes that retry safe.

## Run

Start pinned local dependencies:

```bash
docker run --rm -d --name workshop-audited-postgres \
  -e POSTGRES_DB=bluetape -e POSTGRES_USER=bluetape -e POSTGRES_PASSWORD=bluetape \
  -p 5432:5432 postgres:16-alpine
docker run --rm -d --name workshop-audited-redis \
  -p 6379:6379 redis:7.4-alpine
until docker exec workshop-audited-postgres pg_isready -U bluetape -d bluetape; do sleep 1; done
until docker exec workshop-audited-redis redis-cli ping | rg -q PONG; do sleep 1; done
```

The server has no authentication, so `HTTP_ADDR` accepts an IPv4 or IPv6
loopback literal only. Wildcards, hostnames, zone-scoped addresses, and remote
addresses fail closed.

```bash
export DATABASE_URL='postgres://bluetape:bluetape@127.0.0.1:5432/bluetape?sslmode=disable'
export REDIS_ADDR='127.0.0.1:6379'
export REDIS_STREAM='workshop:audited-orders'
export HTTP_ADDR='127.0.0.1:8080'
go run ./examples/audited-order-workflow-outbox
```

Check liveness, durable readiness, and bounded delivery diagnostics:

```bash
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/readyz
curl -sS http://127.0.0.1:8080/statusz
```

Redis failure keeps `/readyz` at 200 with `"delivery":"degraded"`; database
failure or a stopped relay returns 503. `/statusz` contains only Redis/relay
state, delivery counts, and whole oldest-pending seconds. It never exposes
event identity, payload, metadata, endpoint, or raw provider errors.

## POST JSON scenario

Create an order with metadata. A new command returns 201 and
`delivery: "asynchronous"`.

```bash
curl -sS -X POST http://127.0.0.1:8080/orders \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-1001","command_id":"cmd-create-1001","metadata":{"channel":"workshop"}}'
```

Confirm it:

```bash
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-1001","command_id":"cmd-confirm-1001","action":"confirm","metadata":{"operator":"demo"}}'
```

Send that exact confirmation again. It returns 200 with `replayed: true`, the
original revision 2 projection, and no new audit or outbox record:

```bash
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-1001","command_id":"cmd-confirm-1001","action":"confirm","metadata":{"operator":"demo"}}'
```

Search page one. All bounds are inclusive; `limit` is required from 1 through
100. With revisions 1 and 2 this returns revision 1 and
`next_from_revision: 2`.

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/search \
  -H 'Content-Type: application/json' \
  --data '{"aggregate":{"type":"order","id":"order-1001"},"from_revision":1,"to_revision":null,"from_recorded_at":null,"to_recorded_at":null,"limit":1}'
```

Use that inclusive cursor for page two. It returns revision 2 and
`next_from_revision: null`.

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/search \
  -H 'Content-Type: application/json' \
  --data '{"aggregate":{"type":"order","id":"order-1001"},"from_revision":2,"to_revision":null,"from_recorded_at":null,"to_recorded_at":null,"limit":1}'
```

Load revision 2 without depending on Redis:

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/detail \
  -H 'Content-Type: application/json' \
  --data '{"aggregate":{"type":"order","id":"order-1001"},"revision":2}'
```

Create and cancel a second order:

```bash
curl -sS -X POST http://127.0.0.1:8080/orders \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-2001","command_id":"cmd-create-2001","metadata":{"channel":"workshop"}}'
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-2001","command_id":"cmd-cancel-2001","action":"cancel","reason":"customer request","metadata":{"operator":"demo"}}'
```

A fresh confirm after cancellation is rejected with stable
`409 invalid_transition`:

```bash
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-2001","command_id":"cmd-confirm-2001","action":"confirm","metadata":{"operator":"demo"}}'
```

The checked-in [`requests.http`](requests.http) contains the same complete
bodies and expected statuses for JetBrains and VS Code REST clients.

## Failure and recovery runbook

- Redis outage: commands still commit to PostgreSQL. Confirm `delivery:
  "degraded"` and increasing pending/retrying counts, restore Redis, and watch
  the backlog drain.
- Interrupted publish: a claimed row remains recoverable after the official
  30-second lease. Do not mutate the row to force an early retry.
- Dead letter: inspect aggregate counts through `/statusz`; investigate the
  publisher/consumer outside the request path. This example provides no
  unaudited replay endpoint.
- Partial startup: fix the dependency or incompatible schema and restart. The
  idempotent single-version bootstrap converges after an interrupted create but
  never alters an incompatible table.
- Local destructive reset only:

```bash
docker exec workshop-audited-postgres psql -U bluetape -d bluetape -c \
  'TRUNCATE audited_order_workflow_outbox_records, audited_order_workflow_audit_entries, audited_order_workflow_orders RESTART IDENTITY;'
```

This is workshop reset tooling, not a production migration or rollback
procedure.

## Delivery boundary

Delivery is at-least-once. An accepted Redis append can be repeated when the
client outcome is ambiguous or a claim lease expires. Consumers must
deduplicate by `event_id` or `idempotency_key` and tolerate duplicates and
reordering. The SQL history is the immutable query source; Redis Streams is
transport, not the recovery ledger. The example does not implement consumer
groups, retention, stream trimming, dead-letter replay, authentication, tenant
authorization, or exactly-once processing.

On shutdown the application drains HTTP, cancels and joins the relay, then
closes Redis and PostgreSQL. A publish canceled during shutdown remains
lease-recoverable.

## Test

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/...
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox/... -run TestIntegration
```
