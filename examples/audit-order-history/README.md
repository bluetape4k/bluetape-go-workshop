# audit-order-history

[English](README.md) | [한국어](README.ko.md)

Application-shaped order status history using bluetape-go `audit`.

The example keeps a mutable current order snapshot while appending immutable
create, confirm, and ship entries. Its JSON preview puts both views side by side
and includes a newest-first revision query.

## Package Lesson

| Component | Owns |
|---|---|
| `audit.Repository` | Validated append, contiguous aggregate revisions, duplicate event/idempotency detection, and history queries. |
| Order service | Status transitions, caller-owned command IDs, append-before-mutation consistency, and the current-state projection. |
| Application | Durable transaction/outbox, retention, access control, schema evolution, payload policy, and redaction. |

The service calls `Repository.Append` while holding its teaching lock and only
then changes current state. An append failure or cancellation before/during
append changes neither view. A successful append is the commit point: the
infallible in-memory state assignment completes even if cancellation arrives
immediately afterward. Checking cancellation between those operations would
leave audit history and current state inconsistent.

Command IDs are stable caller-owned identifiers and are used as both event IDs
and idempotency keys. Reusing one is duplicate detection, not a successful
idempotent replay; the error remains compatible with
`audit.ErrRevisionConflict`.

## Run

```bash
go run ./examples/audit-order-history
```

The deterministic output ends with current revision 3 and contrasts append
order with a filtered newest-first query:

```json
{
  "current": {
    "order_id": "order-1001",
    "status": "shipped",
    "revision": 3,
    "updated_at": "2026-07-13T09:10:00Z"
  },
  "history": [
    {"event_id":"command-create-1001","event_type":"order.created","revision":1,"status_after":"pending"},
    {"event_id":"command-confirm-1001","event_type":"order.confirmed","revision":2,"status_before":"pending","status_after":"confirmed"},
    {"event_id":"command-ship-1001","event_type":"order.shipped","revision":3,"status_before":"confirmed","status_after":"shipped"}
  ],
  "recent_history": [
    {"event_id":"command-ship-1001","event_type":"order.shipped","revision":3},
    {"event_id":"command-confirm-1001","event_type":"order.confirmed","revision":2}
  ]
}
```

The abbreviated entries above omit timestamps and authors only to keep the
README compact; the runnable output includes them.

## Test

```bash
go test -count=1 ./examples/audit-order-history/...
go test -count=20 ./examples/audit-order-history/internal/orderhistory -run '^TestServiceConcurrent'
go test -race -count=1 ./examples/audit-order-history/...
```

Tests cover normal and cancelled lifecycles, invalid transitions, duplicate
commands, repository failure, cancellation at the append boundary, absent and
filtered history, defensive copies, and exact concurrent outcomes without
sleeps.

## Audit History Is Not Event Sourcing

The current order map is the teaching source state. The service does not rebuild
orders by replaying audit events, and audit snapshots are not a recovery model.
The history explains what changed and supports inspection; it is not an event
store contract.

## Production Boundaries

- `audit.MemoryRepository` is goroutine-safe but non-durable; process exit loses
  every entry and current snapshot.
- One process-wide mutex intentionally serializes unrelated orders. This is a
  demo-scale invariant, not a throughput or horizontal-scaling design. Durable
  code needs database-owned or per-aggregate concurrency.
- `Find` defaults to 20 and returns at most 100 entries here, but the in-memory
  repository still scans and copies O(total stored entries) before applying the
  limit. A production API needs storage-backed pagination and retention.
- A real application must own retention/deletion, archival, disaster recovery,
  encryption, access control, schema versioning and migration, and payload-size
  policy.
- Event payloads and cancellation reasons must not contain credentials or
  unclassified PII. Classify, minimize, and redact them before storage or logs.
- Coordinating a durable order row with audit delivery requires a caller-owned
  SQL transaction/outbox. This example does not claim exactly-once delivery.

Issue #58 owns the Gin query API lesson. Issues #57 and #68 own durable outbox
and Redis Streams delivery.
