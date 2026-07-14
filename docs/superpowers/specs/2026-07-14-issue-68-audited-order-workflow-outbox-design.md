# Issue #68 Audited Order Workflow with SQL Outbox Design

## Status

The high-level design was approved by the user on 2026-07-14. The detailed Type
A specification review converged at P0=0/P1=0 and final user review was approved
on 2026-07-14. It targets the current workshop dependency, bluetape-go v0.18.0.

## Goal

Add one runnable Gin application that demonstrates a durable order workflow,
immutable audit history, and asynchronous Redis Streams delivery. Every
accepted command must commit the order state, one queryable audit entry, and
one SQL outbox record in the same PostgreSQL transaction. Operators and callers
must be able to distinguish committed history from asynchronous delivery, and
the example must not claim exactly-once delivery.

The application exposes commands and audit queries as POST JSON endpoints so
large or extensible query criteria do not depend on URL length. Both README
locales include complete `curl` examples, and a checked-in `requests.http` file
provides the same runnable scenario with optional metadata.

## Context and Current Evidence

Issue #68 is the integration example in milestone track #35. Its completed
prerequisites are the audit history example (#56), Gin audit query example
(#58), and transactional outbox publisher example (#57). The workshop resolves
`github.com/bluetape4k/bluetape-go` v0.18.0, which is also the latest stable
release observed on 2026-07-14.

The released `audit` package defines immutable entries, JSON encoding, history,
and the `HistoryReader` query contract. Its bundled `MemoryRepository` is not a
durable application repository. The released `audit/sqloutbox` package provides
the caller-session `Store`, schema creation, transactional `Enqueue`, claim and
completion transitions, and a continuous `Relay`. The released
`audit/sqloutbox/redisstreams` adapter publishes the stable audit envelope to a
caller-owned Redis client with at-least-once semantics.

The SQL outbox is a transport ledger rather than an audit read model. Querying
its `entry_json` would couple business history retention and pagination to
delivery cleanup, and the outbox store does not implement `audit.HistoryReader`.
The application therefore owns a separate immutable PostgreSQL history table
while using the official SQL outbox unchanged for delivery.

The repository baseline full `make ci` run reached the container-backed test
suite but one existing PostgreSQL fixture timed out while many packages started
Docker resources in parallel. The same existing package passed alone in 4.477
seconds, and the immediately preceding change passed the full gate at the same
base revision. This is treated as host resource contention, not a source
failure. The new PostgreSQL and Redis integration tests must start containers
sequentially, and completion still requires a fresh, observed `make ci` exit 0.

## Chosen Approach

Create `examples/audited-order-workflow-outbox` with four application
boundaries:

1. An order service validates create and transition commands. Inside one
   `sqlkit.WithTx`, it writes the order row, inserts one immutable audit entry
   into the application history store, and calls `sqloutbox.Store.Enqueue` with
   the same `*sql.Tx` and the same validated `audit.Entry` value.
2. A PostgreSQL history store implements `audit.HistoryReader` independently of
   outbox delivery state and retention. HTTP audit searches and details use only
   this store.
3. A background `sqloutbox.Relay.Run` claims committed outbox records and uses
   the released Redis Streams publisher. Redis never participates in a command
   transaction.
4. A Gin server owns strict POST JSON transport, bounded timeouts, readiness,
   signal-driven shutdown, and redacted error responses.

The application has no Redis consumer. The runnable scenario demonstrates the
producer-side contract and shows that committed audit queries remain available
when delivery is delayed or retried.

## Rejected Alternatives

### Query the SQL outbox as audit history

This would avoid a table but make history disappear when published records are
cleaned up, expose delivery state through a domain query boundary, and require
application-specific queries over an internal transport schema. Rejected
because transport retention is not business history retention.

### Use `audit.MemoryRepository` with a SQL outbox

This would reuse the full reader contract but lose history on restart and allow
the database transaction to commit while the in-memory insert fails, or vice
versa. Rejected because the integration lesson is durable atomic state.

### Publish directly to Redis in the order transaction

This would shorten the code path but create a dual-write failure window and
hold database locks across a network call. Rejected because the official SQL
outbox is the feature being taught.

## Package and Files

```text
examples/audited-order-workflow-outbox/
  main.go
  main_test.go
  requests.http
  README.md
  README.ko.md
  internal/orderworkflow/
    config.go
    model.go
    schema.go
    history_store.go
    history_store_test.go
    service.go
    service_test.go
    handler.go
    handler_test.go
    relay_test.go
    integration_test.go
```

The root README pair links the new example. Diagrams use the canonical generated
asset paths:

```text
docs/images/readme-diagrams/
  audited-order-workflow-outbox-architecture.svg
  audited-order-workflow-outbox-architecture.png
  audited-order-workflow-outbox-sequence.svg
  audited-order-workflow-outbox-sequence.png
```

No new Go module, third-party dependency, workflow, consumer group, generic
library abstraction, or bluetape-go API is required.

## Domain Model

An order contains:

```go
type Order struct {
    OrderID    string    `json:"order_id"`
    Status     Status    `json:"status"`
    Revision   int64     `json:"revision"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

Allowed states are `pending`, `confirmed`, and `cancelled`. Create produces
`pending` at revision 1. Confirm accepts only `pending` and produces
`confirmed`. Cancel accepts `pending` or `confirmed` and produces `cancelled`.
No transition leaves `cancelled`, and no transition confirms an already
confirmed order. Each accepted transition increments revision exactly once.

Create commands carry `order_id`, `command_id`, and optional metadata.
Transition commands carry `order_id`, `command_id`, `action`, optional bounded
`reason`, and optional metadata. Identifiers are trimmed ASCII and must match
`[A-Za-z0-9][A-Za-z0-9._:-]{0,127}`. This keeps database keys, response values,
and structured log attributes free of control characters without normalizing
case. Reasons are valid UTF-8 and at most 500 runes. Metadata is a JSON object
with at most 32 keys; keys are 1 to 64 runes, values are JSON strings at most
512 runes, and the encoded request is still subject to the global body limit.

The command ID becomes both `EventID` and `IdempotencyKey`. Audit entries use
aggregate type `order`, the order ID as aggregate ID, the resulting order
revision, the event types `order.created`, `order.confirmed`, or
`order.cancelled`, and a UTC timestamp from the application clock. Payloads
contain the resulting status and only command fields needed to explain the
transition. Metadata is copied into the entry's structured metadata after
validation. The configured application author is used for every entry. The
service normalizes one clock value with `UTC().Truncate(time.Microsecond)`
before constructing an order and event, then uses that same value in SQL scalar
columns and `entry_json`. This matches PostgreSQL timestamp precision and makes
parity checks deterministic.

## PostgreSQL Schema

The application owns three tables:

```text
audited_order_workflow_orders
  order_id       varchar(128) primary key
  status         varchar(16) not null
  revision       bigint not null check (revision >= 1)
  updated_at     timestamptz not null

audited_order_workflow_audit_entries
  position       bigint generated always as identity unique
  aggregate_type varchar(128) not null
  aggregate_id   varchar(128) not null
  revision       bigint not null check (revision >= 1)
  event_id       varchar(128) not null unique
  idempotency_key varchar(128) not null unique
  event_type     varchar(128) not null
  recorded_at    timestamptz not null
  entry_json     jsonb not null
  primary key (aggregate_type, aggregate_id, revision)

audited_order_workflow_outbox_records
  schema owned by audit/sqloutbox.Store
```

The history table has an index on `(aggregate_type, aggregate_id, recorded_at,
revision)` for bounded time queries. Its generated `position` preserves the
released query contract's cross-aggregate append order without becoming domain
identity. Scalar columns are routing and uniqueness guards; `entry_json` is the
canonical decoded entry. On reads, decoded aggregate identity, revision, event
ID, idempotency key, event type, and recorded time must match the scalar columns.
A mismatch is an internal data-integrity error, not a partial response.

Schema creation is idempotent startup work. It creates the order and history
tables, indexes, and the official outbox schema before the server reports ready.
DDL never runs in an order command transaction. Startup fails closed if any
schema step fails.

## Atomic Command Contract

Create validates the complete command before opening a transaction. Inside one
`sqlkit.WithTx`, it checks command identity, then inserts the pending order,
constructs and validates the revision-1 `audit.Entry`, inserts that entry
through `HistoryStore.Insert`, and calls `Store.Enqueue(ctx, tx, entry)`. The
command returns only after the transaction commits. A duplicate order with no
matching command intent is a conflict.

Transition validates transport-independent fields before opening a transaction.
Inside one `sqlkit.WithTx`, it reads the order with `SELECT ... FOR UPDATE`,
checks command identity and canonical intent before validating current state,
then validates the requested state change, computes the next revision,
constructs one immutable entry, updates the order with the locked prior revision
as a guard, inserts the history entry, and enqueues the same entry. Any failure
rolls back all three writes.

The service never writes Redis. A cancelled or timed-out caller receives an
error even when commit outcome is ambiguous; the same command ID can be queried
or retried safely. Database uniqueness on command identity prevents a retry
from producing a second logical event. Before mutation, the service looks up an
existing command identity and compares its canonical intent. A transition does
this while holding the order lock and before state validation. If either command
later loses a concurrent event-ID or idempotency-key uniqueness race, the entire
transaction rolls back before the service reloads the winner in a fresh database
operation. Matching intent returns the original committed order projection from
the audit payload as an idempotent replay; it does not return a later order
state. Reuse for a different order, action, reason, or metadata is a conflict.
The comparison uses the canonical validated command projection stored in the
audit entry, not raw JSON byte equality. Tests cover ordinary retry,
ambiguous-commit retry, concurrent identical intent, and concurrent conflicting
reuse across orders.

## History Store Contract

`HistoryStore` exposes an application write boundary plus the released reader
contract:

```go
func (s *HistoryStore) Insert(context.Context, sqlkit.Execer, audit.Entry) error
func (s *HistoryStore) Find(context.Context, audit.Query) ([]audit.Entry, error)
func (s *HistoryStore) LoadHistory(context.Context, audit.AggregateID) (audit.History, bool, error)
func (s *HistoryStore) Latest(context.Context, audit.AggregateID) (audit.Entry, bool, error)
func (s *HistoryStore) LatestSnapshot(context.Context, audit.AggregateID) (audit.Entry, bool, error)
func (s *HistoryStore) PreviousSnapshot(context.Context, audit.AggregateID, audit.Revision) (audit.Entry, bool, error)
```

The constructor rejects a nil database and invalid table configuration. The
store uses placeholders and trusted fixed identifiers; request data never
becomes SQL syntax. `Insert` accepts the caller's transaction through
`sqlkit.Execer`, calls `Entry.Validate`, encodes with `encoding/json`, enforces
the same 1 MiB maximum entry size as the official store, and does not begin,
commit, or roll back a transaction itself.

`Find` calls `audit.Query.Validate` and preserves its semantics: optional exact
aggregate or aggregate-type filters, inclusive revision and recorded-time
bounds, stable append order through `position`, `NewestFirst`, and `Limit`.
Exact-aggregate results are consequently in revision order for this service.
The HTTP layer always supplies an exact aggregate and requests `limit + 1`
records, returns at most `limit`, and derives a next-revision cursor without an
unbounded count. `LoadHistory`, `Latest`, and snapshot queries are bounded by
aggregate identity. Snapshot queries filter JSON entries that contain a
non-null snapshot and still validate decoded scalar parity. The order workflow
does not create snapshots, so snapshot absence is normal and returns
`found=false` with the released reader semantics.

## HTTP API

The server uses Gin without debug mode and exposes:

```text
POST /orders
POST /orders/transitions
POST /audit/history/search
POST /audit/history/detail
GET  /healthz
GET  /readyz
GET  /statusz
```

All command and query bodies require `Content-Type: application/json`, are
limited to 32 KiB before decoding, and reject compressed bodies, invalid UTF-8,
unknown fields, duplicate object keys, trailing JSON values, non-object
top-level values, and empty bodies. The decoder preserves integer precision.
Metadata does not weaken these rules.

`POST /orders` accepts:

```json
{
  "order_id": "order-1001",
  "command_id": "cmd-create-1001",
  "metadata": {"channel": "workshop"}
}
```

`POST /orders/transitions` accepts:

```json
{
  "order_id": "order-1001",
  "command_id": "cmd-confirm-1001",
  "action": "confirm",
  "metadata": {"operator": "demo"}
}
```

The create response is HTTP 201 for a new order and HTTP 200 for an idempotent
replay. A new or replayed transition returns HTTP 200. Successful responses use
this complete shape:

```json
{
  "request_id": "req-...",
  "data": {
    "order": {
      "order_id": "order-1001",
      "status": "confirmed",
      "revision": 2,
      "updated_at": "2026-07-14T12:00:00Z"
    },
    "event_id": "cmd-confirm-1001",
    "idempotency_key": "cmd-confirm-1001",
    "delivery": "asynchronous",
    "replayed": false
  }
}
```

An identical retry sets `replayed` to true and returns the original committed
order projection even if the aggregate has since advanced. It creates no new
revision or outbox record. The response confirms durable commit, not Redis
publish.

`POST /audit/history/search` accepts this canonical shape:

```json
{
  "aggregate": {"type": "order", "id": "order-1001"},
  "from_revision": 1,
  "to_revision": null,
  "from_recorded_at": null,
  "to_recorded_at": null,
  "limit": 20
}
```

The four bounds may be omitted or null; present revisions must be positive and
present times must be RFC3339 with an offset. Lower and upper bounds are
inclusive, lower bounds must not exceed upper bounds, and `limit` is required
from 1 through 100. A successful response contains `entries` in ascending
revision order and nullable `next_from_revision`.

The cursor is the first revision fetched but not returned. For revisions 1 and
2 with `limit: 1`, page one supplies `from_revision: 1`, returns revision 1 and
`next_from_revision: 2`; page two supplies `from_revision: 2`, returns revision
2 and a null cursor. Because the next cursor identifies the first unreturned
entry and the lower bound is inclusive, neither revision repeats or skips.

`POST /audit/history/detail` accepts:

```json
{
  "aggregate": {"type": "order", "id": "order-1001"},
  "revision": 2
}
```

Revision must be positive. Both audit endpoints read PostgreSQL history only
and remain correct when the relay is stopped.

Transport errors use
`{"request_id":"req-...","error":{"code":"...","message":"..."}}` with a
non-sensitive code and message. Invalid JSON and validation failures return 400,
oversized bodies return 413, and unsupported media type or content encoding
returns 415. Missing orders or audit revisions return 404. Duplicate order
identities, conflicting command reuse, and invalid transitions return 409. A
server-side operation deadline returns 408 when caused by the request context.
Concurrency-cap rejection returns 429 with code `too_many_requests`,
`Retry-After: 1`, `Connection: close`, and a closed request body; retry is safe
when the caller reuses the same command ID because no handler ran. Unexpected
storage errors return 500 without endpoint values, SQL text, credentials, entry
payloads, or raw provider messages. Publisher failures are asynchronous and
appear only through relay retry, dead-letter state, delivery status, and safe
logs; they have no per-request HTTP mapping.

## Runtime, Relay, and Shutdown

Required configuration is `DATABASE_URL` and `REDIS_ADDR`. Optional values are
`REDIS_STREAM` and `HTTP_ADDR`. `HTTP_ADDR` defaults to `127.0.0.1:8080` and
must contain a loopback IP; wildcard, hostname-only, and non-loopback binds fail
closed because the example has no authentication. There is no unsafe remote
opt-in. `REDIS_STREAM` is trusted application configuration but is still
required to be valid UTF-8, non-blank, and at most 256 bytes.

The application owns the PostgreSQL and Redis clients. Startup parses redacted
configuration, opens both clients, performs bounded pings, creates schemas,
constructs one released Redis Streams publisher, starts one background relay,
then serves HTTP. Readiness is false until all startup work succeeds. Startup,
request, relay, and shutdown logs contain only stage, stable error class,
request ID, and grammar-validated identity where needed; raw provider errors,
endpoint values, and caller metadata are never logged.

PostgreSQL is configured with 8 maximum open and idle connections, 5-minute
maximum idle time, and 30-minute maximum lifetime. Redis uses a pool of 8 with
one minimum idle connection and a 2-second pool timeout. The server allows at
most 32 in-flight requests and rejects excess work with 429 before decoding a
body. It uses a 2-second request operation deadline, 2-second header timeout,
5-second read and write timeouts, 30-second idle timeout, 16 KiB maximum header
size, and 5-second graceful-shutdown deadline. These teaching defaults are
explicit and are not production sizing advice.

The relay uses explicit bounded options:

```go
sqloutbox.RelayOptions{
    ClaimLimit:  16,
    MaxAttempts: 3,
    RetryDelay:  250 * time.Millisecond,
    IdleDelay:   50 * time.Millisecond,
}
```

`/healthz` reports only process liveness. `/readyz` requires a short bounded
PostgreSQL check and a running supervised relay. Redis failure is reported in
the 200 response as `"delivery":"degraded"` but does not remove the durable
command and history service from traffic; the application intentionally accepts
a local-demo backlog rather than defeating the outbox availability boundary.
Database failure or a stopped relay returns 503. Outbox emptiness is not a
readiness condition.

`/statusz` exposes only bounded delivery diagnostics: Redis status, relay state,
pending/retrying/claimed/published/dead-letter counts, and oldest-pending age
rounded to seconds.
It never returns entry identity, payload, metadata, endpoints, or provider
errors. A read-only status query uses the fixed official table and its status
index under a 250-millisecond deadline; timeout returns a degraded status rather
than blocking readiness. The official v0.18.0 `Relay.Run` exposes only its
terminal error, not per-batch results, so the application does not replace it
with a custom polling loop merely to log batch counts. Delivery degradation and
lifecycle transitions are logged once per transition to avoid idle-loop noise;
current counts remain available through `/statusz`. The relay goroutine reports
its terminal result to the lifecycle owner.
An unexpected `Relay.Run` error first makes readiness false, then initiates the
same bounded server shutdown and causes process failure. `context.Canceled` is
successful only after the lifecycle owner requested shutdown; an early
cancellation is unexpected.

Shutdown first stops accepting HTTP work and waits for bounded in-flight
requests, then cancels the relay context, joins the relay goroutine, and closes
Redis and PostgreSQL clients. Caller cancellation is preserved. A relay publish
cancelled during shutdown is not marked published or converted to a retry; its
claim becomes recoverable under the released lease contract. Shutdown joins
independent errors without exposing secrets and has a hard deadline.

Delivery is at-least-once. An accepted Redis append followed by an ambiguous
client failure, failed SQL completion, or expired claim can append the same
logical event more than once. The official store blocks a later pending
revision while an earlier revision is pending or claimed, but dead-letter,
replay, and ambiguous publish outcomes mean the application does not promise
consumer-observed ordering. Consumers must deduplicate by `event_id` or
`idempotency_key` and tolerate reordering. Outbox cleanup, operator replay,
Redis consumer groups, and consumer idempotency are production follow-up
concerns rather than hidden behavior in this example.

## Runnable Documentation Contract

Both README locales explain prerequisites, environment variables, startup,
the order state machine, atomic transaction boundary, history-versus-outbox
separation, at-least-once behavior, failure recovery, overload response, and
shutdown. Schema creation is explicitly a single-version workshop bootstrap,
not a migration framework; an incompatible existing schema fails startup and
is never altered or dropped automatically. The application owns all three
example tables. The runbook shows safe outbox status inspection, Redis outage
and recovery, pending/claimed lease recovery, dead-letter diagnosis, restart
after partial startup, and an explicitly destructive local-only reset. It does
not present table deletion or dead-letter mutation as a production rollback.
The English language switch is `English | [한국어](README.ko.md)` and the Korean
switch is `[English](README.md) | 한국어`; the current locale is plain text. The
same deferred language-switch defect is corrected in the issue #57 README pair.

The documented scenario uses complete `curl` POST JSON commands for:

1. creating an order with metadata;
2. confirming it with metadata;
3. retrying the identical confirm command and observing `replayed: true` with no
   new revision;
4. searching its audit history, including the two-page cursor example;
5. loading revision 2 in detail; and
6. optionally creating and cancelling a second order with a reason, then showing
   a rejected post-cancellation transition and stable 409 code.

Every request includes its URL, method, content type, and body. The checked-in
`requests.http` file repeats the scenario with variables and valid JSON bodies,
so JetBrains and VS Code REST clients can run it without reconstructing a
request from prose. The README pairs and HTTP file use source-equivalent bodies.
Expected status, revision, replay marker, asynchronous-delivery marker, history
ordering, 429 retry rule, invalid-transition code, and duplicate-delivery caveat
are shown.

The README architecture diagram shows the Gin routes, order service,
`sqlkit.WithTx`, order/history/outbox tables, background relay, and Redis
Streams. The sequence diagram shows command validation, row locking, all three
transactional writes, commit-before-response, later relay delivery, and a
history query that bypasses Redis. SVG is the source artifact and PNG is the
GitHub-facing render. Both diagrams follow `bluetape-diagram`, including
checklist validation and visual inspection of SVG and PNG arrowhead direction,
arrowhead clearance at bends and card boundaries, clipping, text legibility,
and correspondence between the two formats.

## Failure Modes and Safety Boundaries

- A history insert or outbox enqueue failure rolls back the order write.
- Redis unavailability never rolls back an already committed command; readiness
  remains available with degraded delivery status and the relay applies its
  bounded retry/dead-letter policy.
- A duplicate command with the same canonical intent is an idempotent replay; a
  mismatched intent is a 409 conflict.
- Concurrent transitions serialize through the locked order row. Exactly one
  valid next revision commits, and losing or now-invalid transitions conflict.
- Query pagination is bounded and stable per aggregate. Entries committed after
  a page may appear on a later page but cannot rewrite an existing revision.
- Corrupt or scalar-mismatched history JSON fails the request closed and is
  logged by stage and safe identity only.
- Request and shutdown logs never include database URLs, Redis addresses,
  credentials, arbitrary metadata, audit payloads, or raw request bodies.
- The example is unauthenticated and loopback-only. Remote exposure is rejected
  and remains outside this demonstration.
- PostgreSQL and Redis Testcontainers start sequentially to reduce constrained
  host contention. A missing process handle or unobserved exit code is not test
  evidence.

## Test Strategy

Unit and handler tests prove:

- identifier, action, reason, metadata, content type, body size, duplicate key,
  unknown field, trailing value, compression, and UTF-8 rejection;
- state-machine transitions, revision increments, entry identity, payload and
  metadata projection, idempotent replay, and conflicting command reuse;
- 201/200/400/404/408/409/413/415/429/500 response mapping, replay markers,
  `Retry-After`, request-body closure, and redaction;
- search bounds, stable ordering, `limit + 1` cursor derivation, detail lookup,
  and no Redis dependency in query handlers;
- zero-value, nil dependency, cancellation, timeout, close, slow-header/body,
  concurrency-cap, pool-exhaustion, and graceful shutdown behavior.

PostgreSQL integration tests prove with the released fixture:

- order, history, and outbox commit together on create and transition;
- failures injected after each write roll back every write;
- two concurrent transitions cannot commit the same revision;
- the SQL history store implements every `audit.HistoryReader` method and fails
  closed on corrupted scalar/JSON parity;
- a clock containing sub-microsecond nanoseconds is normalized once so SQL
  scalar parity, history reads, and relay claims remain valid;
- restart preserves order state, audit history, idempotent replay, and pending
  outbox records; and
- a seeded large-history query plan uses the intended aggregate/revision or
  aggregate/time index and applies `limit + 1` without an unbounded scan or
  sort, verified with `EXPLAIN (FORMAT JSON)`.

Relay tests use `sqloutboxtest.RecordingPublisher` and `PublisherFunc` to prove
success, one injected retry, duplicate logical identity across attempts,
dead-letter bounds, caller cancellation, lease recovery, and continuous
`Run` shutdown without a leaked goroutine. A real Redis integration test starts
after PostgreSQL and verifies the released provider's documented 13-field
envelope and stable event/idempotency identity for created and confirmed events.
It accepts multiple physical stream entries for one logical event. A bounded
backlog test drains multiple claim batches while commands continue writing,
records claimed and published counts, observes the configured connection-pool
ceiling, and completes before a fixed deadline. The claim size and polling delay
are conservative teaching values validated by this test, not benchmark-derived
production tuning. A lifecycle test proves that an unexpected relay exit makes
readiness false and terminates serving. Observability tests prove readiness
transitions across Redis outage and recovery, accurate bounded status counts and
oldest-pending age, lifecycle summaries, idle-loop log suppression, and
the absence of payloads, metadata, endpoints, credentials, and provider errors.

Validation runs targeted package tests, race tests, sequential container-backed
integration tests, a resource-bounded `go test -p 1 -count=1 ./...` lane, a
runnable server smoke scenario using the checked-in POST JSON requests, diagram
render and checklist validation, then a fresh `make ci`. Every final gate must
have a newly observed exit code.

## Acceptance Criteria

- The example is application-shaped and compiles against bluetape-go v0.18.0.
- Create and transition commands atomically commit order, durable history, and
  official SQL outbox state.
- Audit search and detail use the SQL history store, not Redis or outbox rows.
- The background official relay publishes through the official Redis Streams
  adapter with explicit at-least-once semantics.
- All user operations use strict POST JSON; health and readiness remain GET.
- Both README locales and `requests.http` provide complete runnable requests.
- Architecture and sequence SVG/PNG pairs pass automated checks and manual
  visual inspection, including arrowhead direction and bend clearance.
- The issue #57 README language switches show the current locale as plain text.
- Targeted, race, integration, smoke, and full `make ci` gates pass with fresh
  observed exit codes.
- The PR is assigned, labeled, attached to milestone 0.9.0, links issue #68, and
  reaches green CI. Merge remains a separate user approval gate.

## Specification Review Convergence

| Perspective | Result | Resolved focus |
|---|---|---|
| Performance | P0=0, P1=0 | Pool limits, indexed bounded history queries, and backlog contention evidence are explicit. |
| Stability | P0=0, P1=0 | Race-safe replay, ambiguous commit, relay supervision, delivery ordering limits, and resource-bounded tests are explicit. |
| Security | P0=0, P1=0 | Loopback-only binding, strict JSON, resource caps, safe identifiers, and all-stage redaction are explicit. |
| Operator/Ops | P0=0, P1=0 | Redis degradation, readiness, relay/backlog signals, schema ownership, and recovery runbooks are explicit. |
| Developer/API | P0=0, P1=0 | v0.18.0 reader signatures, timestamp precision, and asynchronous publisher error ownership are explicit. |
| User/caller | P0=0, P1=0 | Complete POST JSON bodies, cursor continuation, replay, overload, cancellation, and 409 examples are explicit. |

All review findings were repaired in this specification. The affected lanes
were rerun against the integrated artifact and returned clear; no P2/P3 item is
deferred.

## Non-Goals

- Adding a reusable SQL audit repository to bluetape-go.
- Exactly-once Redis delivery or global event ordering.
- A Redis consumer, consumer group, consumer-side projection, or deduplication
  store.
- Authentication, authorization, TLS termination, or production deployment.
- Automated outbox or audit-history retention and operator replay tooling.
- General workflow engines, arbitrary order states, or cross-aggregate
  transactions.
