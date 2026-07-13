# gin-audit-query-api

English | [한국어](README.ko.md)

Application-shaped Gin API for querying bluetape-go `audit` history. Search
filters are submitted as a POST JSON body instead of a URL query string, so the
contract does not depend on URL length as revision and time filters grow.

## Package Lesson

| Component | Owns |
|---|---|
| `audit.HistoryReader` | Storage-independent audit entry reads. |
| Query service | Aggregate scope, revision/time filters, limit, ordering, and the next page boundary. |
| Gin adapter | Strict JSON, body limit/close, request timeout, HTTP status, and safe logs. |
| Application | Authentication, authorization, tenant scope, payload/metadata redaction, retention, and rate limits. |

The service queries exactly one aggregate. It gives `audit.Query` `limit + 1`,
returns only the requested count, and uses the first unseen revision as an
inclusive continuation when another entry exists. Pages therefore neither
repeat nor skip a revision.

## Run

```bash
go run ./examples/gin-audit-query-api
```

The default address is `127.0.0.1:8080`.

```bash
curl -sS http://127.0.0.1:8080/healthz
```

## POST JSON Search

Query the two newest entries:

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/search \
  -H 'Content-Type: application/json' \
  --data '{
    "aggregate": {"type": "order", "id": "order-1001"},
    "from_recorded_at": "2026-07-13T09:00:00Z",
    "newest_first": true,
    "limit": 2
  }'
```

`entries` contains the complete `audit.Entry` JSON. This abbreviated response
keeps the page contract visible:

```json
{
  "entries": [
    {"revision": 4, "event": {"event_type": "order.shipped"}},
    {"revision": 3, "event": {"event_type": "order.packed"}}
  ],
  "page": {
    "limit": 2,
    "has_more": true,
    "next": {"to_revision": 2}
  }
}
```

For the next newest-first page, resend the original filters and replace only
`to_revision` with the returned value:

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/search \
  -H 'Content-Type: application/json' \
  --data '{
    "aggregate": {"type": "order", "id": "order-1001"},
    "from_recorded_at": "2026-07-13T09:00:00Z",
    "to_revision": 2,
    "newest_first": true,
    "limit": 2
  }'
```

Ascending searches use `next.from_revision` the same way. Revision and
recorded-at ranges are inclusive. No matches produce `200` with
`"entries": []`, not a 404.

## Event Detail

The current `HistoryReader` has no event-ID lookup, so detail identity uses an
aggregate and revision:

```bash
curl -sS \
  http://127.0.0.1:8080/audit/aggregates/order/order-1001/revisions/3
```

A missing revision returns `404 audit_entry_not_found`.

## Strict Boundary

- Search accepts UTF-8 `application/json` bodies up to 32 KiB.
- Unknown fields, duplicate JSON keys, trailing JSON values, and compressed
  bodies are rejected.
- Aggregate type and ID are restricted to path-safe ASCII identifiers.
- Requests have a two-second deadline; the server also owns header, read,
  write, and idle timeouts.
- Logs contain only route template, method, status, code, and elapsed time.
  Aggregate IDs, payloads, metadata, request bodies, and raw errors are omitted.

## Metadata Scope

Stored event metadata remains in each returned `audit.Entry`. bluetape-go
v0.18.0 `audit.Query` does not provide a metadata predicate. Filtering metadata
in the application after the repository applies its limit would create missing
or incomplete pages, so this example does not offer a metadata filter.

## Production Boundaries

- This server implements no authentication, authorization, or tenant isolation,
  so it binds to loopback by default. Remote binding requires the explicit
  `ALLOW_UNAUTHENTICATED_REMOTE=1` risk opt-in.
- Production code must derive tenant and aggregate scope from authenticated
  server context, never from client JSON as an authorization claim.
- Audit payloads and metadata may contain PII or credentials. Classify,
  minimize, and apply field-level redaction before storage and response.
- `MemoryRepository` loses every entry on process exit. `/healthz` proves only
  process availability, not readiness or durability.
- A real repository must own retention/deletion, encryption, per-entry and total
  response byte budgets, storage-backed pagination, and rate limiting.
- This is audit querying, not event sourcing, recovery, global audit search, or
  exactly-once delivery.

Remote bind example:

```bash
HTTP_ADDR=0.0.0.0:8080 ALLOW_UNAUTHENTICATED_REMOTE=1 \
  go run ./examples/gin-audit-query-api
```

## Test

```bash
go test -count=1 ./examples/gin-audit-query-api/...
go test -race -count=1 ./examples/gin-audit-query-api/...
```

Tests cover filters and both pagination directions, detail not-found, typed
audit errors, cancellation, strict JSON, body closure, timeout and log
redaction, loopback policy, graceful/forced shutdown, and listener joining.
