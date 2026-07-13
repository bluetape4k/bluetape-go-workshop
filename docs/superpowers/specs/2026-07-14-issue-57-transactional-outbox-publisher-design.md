# Issue #57 Transactional Outbox Publisher Design

## Status

User-approved design for Issue #57, planned against bluetape-go v0.18.0.
The Type A specification review converged at P0=0/P1=0 on 2026-07-14.

## Goal

Add a runnable, application-shaped order example that commits an order row and
its audit outbox entry in one PostgreSQL transaction, then publishes the claimed
record through the released Redis Streams adapter. The example must make
at-least-once delivery, stable event identity, bounded retry, cancellation,
shutdown, duplicate handling, and operator replay boundaries visible without
claiming exactly-once behavior.

## Context and Current Evidence

Issue #57 is the third dependency in milestone track #35 after the completed
audit history (#56) and Gin audit query (#58) examples. It precedes the broader
audited order workflow integration (#68). The current stable workshop dependency
is bluetape-go v0.18.0, release commit
`26fe037eb4146f35328c9a549a3adb1207758f50`.

The released `audit/sqloutbox` package provides a caller-session `Store`,
`CreateSchema`, `Enqueue`, claim/retry/completion transitions, and `Relay` with
`RunOnce` and continuous `Run`. `Enqueue` accepts `sqlkit.Execer`, so the
application can pass the same `*sql.Tx` that writes the order. `Relay` documents
at-least-once delivery and preserves caller cancellation without converting it
to retry or dead-letter state.

The released `audit/sqloutbox/sqloutboxtest` package provides a concurrent-safe
`RecordingPublisher`, deterministic per-event failure injection, and
`PublisherFunc`. The released `audit/sqloutbox/redisstreams` package accepts a
caller-owned Redis appender and writes the documented stable envelope with one
`XADD` per publish attempt. Its README explicitly warns that an accepted Redis
write followed by an ambiguous client failure can produce a duplicate entry on
retry.

The existing `sql-transaction-boundary` workshop example is the local source for
`sqlkit.WithTx` ownership, deterministic SQL behavior, rollback tests, and
PostgreSQL fixture usage. This example borrows those boundaries but does not
extend or import its `internal` package. The existing
`order-pipeline-testcontainers` example proves that the released PostgreSQL and
Redis fixtures can be started sequentially in one test.

GNO evidence for bluetape-go issue #533 and PR #574 confirms that the Redis
Streams adapter is intentionally narrow: the SQL outbox remains the durable
source of truth, the Redis client remains caller-owned, retries append duplicate
stream entries, and consumer idempotency remains outside the provider.

## Chosen Approach

Create a new `transactional-outbox-publisher` example with three explicit
boundaries:

1. `orderoutbox.Service` validates one order command, builds one immutable audit
   entry, and uses `sqlkit.WithTx` to insert the order and call
   `Store.Enqueue(ctx, tx, entry)` before commit. It performs no Redis or other
   network publish while the transaction is open.
2. A caller-owned `sqloutbox.Relay` claims and publishes committed records after
   the transaction. Deterministic tests use `RecordingPublisher` and
   `PublisherFunc`; the runnable and integration path use
   `redisstreams.New` directly.
3. `main` owns PostgreSQL and Redis client creation, readiness checks, operation
   timeouts, relay construction, cancellation, and client closure. A bounded
   `RunOnce` invocation publishes the sample and lets the command exit cleanly.
   Continuous `Relay.Run` shutdown is proved separately with a cancellation test
   because an indefinitely running CLI would obscure the workshop output.

The runnable command creates idempotent schemas, places a caller-selected order,
runs one relay batch, reads the resulting Redis stream entry for demonstration,
and prints deterministic-shaped JSON containing the committed order identity,
relay counts, stream key, and stable event/idempotency fields. The timestamp and
order identifiers may be supplied through environment variables for repeatable
documentation; defaults are workshop-safe but a duplicate rerun is reported as
a conflict rather than silently rewritten.

## Alternatives

### Extend `sql-transaction-boundary`

Adding outbox and Redis behavior to the existing transaction example would
reuse code, but it would turn a focused commit/rollback lesson into a
multi-backend delivery application and make its current “outbox comes next”
boundary false. Rejected; source patterns are borrowed into an independent
example instead.

### Split deterministic relay and Redis into separate examples

One example could use `RecordingPublisher` and another could use Redis Streams.
That makes each smaller, but it separates failure semantics from the real
adapter and overlaps the broader integration owned by issue #68. Rejected; one
example uses deterministic publishers as tests and the Redis adapter as the
runnable integration.

### Publish directly with Redis `XADD`

A hand-written mapping would be short and visually direct, but it would duplicate
the released provider, drift from its field contract, and hide the exact
at-least-once behavior issue #57 is meant to teach. Rejected; only
`redisstreams.New` may own Redis field encoding.

## Package and Files

```text
examples/transactional-outbox-publisher/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/orderoutbox/
    model.go
    schema.go
    service.go
    service_test.go
    relay_test.go
    integration_test.go
```

The root `README.md` and `README.ko.md` will link the example. Diagram sources
and renders will use the repository's canonical paths:

```text
docs/images/readme-diagrams/
  transactional-outbox-publisher-architecture.svg
  transactional-outbox-publisher-architecture.png
  transactional-outbox-publisher-sequence.svg
  transactional-outbox-publisher-sequence.png
```

The Type A plan, review, and lesson artifacts remain under the existing `docs`
paths. No new module, dependency, workflow, public bluetape-go API, changelog,
or consumer-group implementation is required.

## Domain and Service Contract

The internal package uses the following teaching surface:

```go
type Config struct {
    Author string
    Now    func() time.Time
}

type PlaceOrderCommand struct {
    OrderID       string
    CustomerID    string
    CommandID     string
    TotalCents    int64
    CreatedAt     time.Time
}

type Order struct {
    OrderID       string
    CustomerID    string
    Status        string
    TotalCents    int64
    CreatedAt     time.Time
}

func NewService(store *sqloutbox.Store, config Config) (*Service, error)
func (s *Service) CreateSchema(context.Context, sqlkit.Execer) error
func (s *Service) Place(context.Context, *sql.DB, PlaceOrderCommand) (Order, error)
```

`NewService` rejects a nil store and an author that is blank, invalid UTF-8, or
longer than 128 runes. A nil clock defaults to `time.Now().UTC`. A zero-value or
nil service fails closed with `ErrInvalidConfig`. `Place` rejects a nil database,
blank or oversized IDs, invalid UTF-8, non-positive totals, and zero timestamps
before opening a transaction. IDs are trimmed and limited to 128 runes; they are
not otherwise normalized or rewritten. A nil context is normalized to
`context.Background`; caller cancellation and deadlines are preserved.

The order table is `transactional_outbox_orders` with `order_id` as its primary
key, a bounded customer ID, `status = 'placed'`, positive total cents, and UTC
creation time. The application constructs the released store with table
`transactional_outbox_records` so it does not share the generic default table
with unrelated examples. `Service.CreateSchema` creates the order table and
delegates outbox schema creation to that store. Schema creation is an idempotent
application startup operation and never runs inside an order transaction; a
partial DDL failure is retried on the next startup rather than hidden.

For every accepted command, `Place` builds:

- aggregate type `order` and aggregate ID equal to `OrderID`;
- revision `audit.InitialRevision()`;
- event ID and idempotency key equal to the caller-owned `CommandID`;
- event type `order.placed`;
- `OccurredAt` from the caller's UTC-normalized `CreatedAt` and `RecordedAt`
  from the service clock;
- a fixed JSON payload with customer ID, status, and total cents;
- the configured author; and
- schema version from `audit.NewEntry`.

Inside one `sqlkit.WithTx`, `Place` inserts the order row and calls
`Store.Enqueue` with the same `*sql.Tx`. The returned `Order` becomes observable
only after `WithTx` commits. SQL uniqueness errors remain inspectable through
`%w`; the example does not translate all PostgreSQL codes into a new public
error taxonomy.

## Relay and Delivery Contract

The application constructs the relay with explicit bounded options:

```go
sqloutbox.RelayOptions{
    ClaimLimit:  1,
    MaxAttempts: 3,
    RetryDelay:  250 * time.Millisecond,
    IdleDelay:   50 * time.Millisecond,
}
```

The runnable path calls `RunOnce` under a bounded operation context. The
deterministic path uses `RecordingPublisher` with one injected failure to prove
that the first attempt is recorded, retry state is persisted, the next eligible
attempt carries the same `EventID` and `IdempotencyKey`, and the second attempt
publishes successfully. Time is injected into the store and relay so the test
advances retry eligibility without sleeping.

The CLI requires the clean single-record batch result
`Claimed=1, Published=1, Failed=0, DeadLettered=0` before printing success. A
pre-existing pending record in the example table is processed according to
outbox order and causes the sample identity check to fail closed rather than
reporting the wrong event as the newly published order. Production relays should
tune a larger claim limit from measured throughput and downstream capacity.

`PublisherFunc` proves cancellation at the publish boundary. If the caller
context is cancelled, `RunOnce` returns the cancellation and does not mark the
record failed or dead-lettered. The record may remain claimed until its lease is
eligible for reclaim; shutdown does not pretend it was successfully delivered.

The Redis path constructs exactly one `redisstreams.Publisher` with a
caller-owned `go-redis` client and trusted application stream configuration.
The integration test reads Redis with bounded `XRangeN` only to verify the
documented provider output; the CLI reads one latest candidate with bounded
`XRevRangeN`. Production publishing never calls `XAdd` outside the released
adapter.

Delivery is at-least-once. A successful Redis append followed by a failed SQL
completion, an expired claim lease, or an ambiguous Redis client failure may
publish the same logical event again. Consumers must deduplicate by `event_id`
or `idempotency_key`; `attempts` is diagnostics, not identity. Ordering is
preserved per aggregate by the SQL outbox claim contract, not globally across
all aggregates.

## Runtime and Shutdown Contract

The command requires `DATABASE_URL` and `REDIS_ADDR`; it does not start Docker
or silently fall back to in-memory infrastructure. Optional `REDIS_STREAM`,
`ORDER_ID`, `CUSTOMER_ID`, `COMMAND_ID`, and `ORDER_CREATED_AT` values remain
bounded application configuration. The created-at value must be RFC3339 when
present and defaults to the service clock. `REDIS_STREAM` must be valid UTF-8,
non-blank, and at most 256 bytes; when omitted, the provider's
`audit:sqloutbox` default is used. Required endpoint values are non-blank;
configuration parsing never echoes their contents in an error.

`main` opens PostgreSQL and Redis clients, verifies both with bounded `Ping`
calls, creates schemas, places the order, runs one bounded relay batch, reads
the demonstration entry, and closes both clients on every exit path. It uses a
signal-derived root context and shorter operation contexts. Raw database URLs,
Redis addresses, credentials, payloads, and provider errors are not printed in
successful output. Failure output names the failed stage and wraps the provider
error without interpolating the supplied endpoint value; operators must still
treat diagnostic stderr as sensitive.

Continuous relay lifecycle is a supported library path but not the default CLI
mode. Its test starts `Relay.Run`, cancels the caller context, waits for the
goroutine, requires `context.Canceled`, and proves no late publish or leaked
goroutine. The application owns restart policy; it does not retry caller
cancellation.

## Redis Stream Evidence

The integration test verifies the released provider's documented fields:

```text
record_id, status, aggregate_type, aggregate_id, revision,
event_id, idempotency_key, event_type, occurred_at, recorded_at,
schema_version, attempts, entry_json
```

It parses `entry_json` as a validated `audit.Entry` and checks identity parity
with the scalar fields. It verifies PostgreSQL and Redis readiness through real
commands after both fixtures start. Container-backed tests do not use
`t.Parallel`, use bounded contexts, register deterministic client cleanup, and
run PostgreSQL plus Redis sequentially.

## Failure Modes

1. Order insert, audit construction, or outbox enqueue failure rolls back both
   order and outbox state. A failed transaction never produces a relay record.
2. A non-cancellation publisher failure persists bounded retry state. After
   `MaxAttempts`, the record becomes dead-lettered and is not reported as
   published. This example documents inspection/replay boundaries but does not
   add an automated dead-letter replay command.
3. Caller cancellation during publish stops the relay without converting the
   record to retry/dead-letter state. A claimed record becomes reclaimable only
   through the store's lease behavior.
4. Redis may accept `XADD` before a client or completion error is observed. A
   later retry can append a duplicate with the same stable identity; consumers
   and operator replay must tolerate it.
5. PostgreSQL or Redis readiness, schema creation, placement, publish, or stream
   verification failure exits the command nonzero after owned resources are
   closed. There is no ambiguous success log.
6. Duplicate order, event ID, or idempotency key conflicts fail closed. The
   example does not silently generate replacement identities or mutate the
   existing order.

## Security and Operations Boundaries

Database and Redis endpoints are trusted deployment configuration, not request
input. The example has no HTTP listener, authentication, authorization,
multi-tenancy, secrets manager, TLS setup, or dynamic stream selection. Real
deployments must obtain credentials securely, enforce TLS/network policy,
separate tenant namespaces, and restrict Redis stream access.

Audit payloads can contain sensitive data. This example uses fixed non-sensitive
fields and warns that payload classification/redaction must happen before
enqueue when SQL and Redis retention or readership differ. Raw provider errors
may contain endpoint details; the CLI wraps failures for stderr but does not
emit configuration or full audit payloads.

The SQL outbox is the durable source of truth. Redis retention, trimming,
consumer groups, pending-entry recovery, consumer acknowledgement, poison
message handling, and replay authorization are explicitly out of scope.
Operators may inspect and requeue failed records only through a separately
approved operational tool; manual SQL mutation is not presented as a safe
workshop procedure.

## Diagrams

The English and Korean README files share two English-label SVG/PNG pairs:

1. A static architecture diagram answers “who owns the transaction, relay,
   clients, durable state, and transport?” It separates application ownership,
   PostgreSQL order/outbox state, relay, and Redis Streams. It uses direct
   horizontal connectors where possible and rounded orthogonal routes only
   where required.
2. A chronological sequence diagram answers “how can one stable event be
   attempted more than once?” It shows transaction commit, claim, first publish
   failure/ambiguous outcome, retry, duplicate-capable append, completion, and
   cancellation as an explicit alternate path.

Both assets follow `bluetape-diagram` architecture/sequence rules. Each SVG is
parsed, rendered with CairoSVG at scale 2, audited for connectors, geometry,
endpoints, mixed corners, marker color/size/direction, and inspected as a
full-size PNG after the final coordinate change. Arrowhead clearance is included
in bend placement; a syntactic `Q` segment alone is not accepted as visual
proof.

## Testing

Focused TDD proves:

- constructor, zero-value, nil database, identifier, total, timestamp, and
  context validation;
- atomic order/outbox commit and rollback on order conflict and an outbox
  identity conflict that occurs after the order insert against real PostgreSQL;
- successful `RunOnce`, transient failure, time-controlled retry, duplicate
  attempts, stable `EventID`/`IdempotencyKey`, max-attempt dead letter, and
  inspectable errors with `RecordingPublisher`;
- `PublisherFunc` caller cancellation without retry/dead-letter conversion;
- continuous `Run` cancellation, joined goroutine, and no late publish;
- PostgreSQL plus Redis readiness, official adapter publish, all documented
  stream fields, valid `entry_json`, and deterministic cleanup; and
- runnable configuration, output shape, error propagation, and resource closure
  without opening a public listener.

Concurrency, retry, and lifecycle tests run under `go test -race`. No test uses
unbounded sleeps; injected clocks and bounded channels coordinate state. The
validation order is focused package tests, focused race tests, the sequential
container integration package, a configured `go run` smoke check when local
services are available, diagram audits, `git diff --check`, and repository-wide
`make ci`.

## README Contract

`README.md` and `README.ko.md` include the language switch, package lesson,
architecture and sequence diagrams, prerequisites, exact run command, expected
JSON and stream fields, focused test commands, and production boundaries. They
state:

- order and outbox commit atomically, but Redis publication is asynchronous;
- delivery is at-least-once, not exactly-once;
- consumers deduplicate with stable event identity;
- replay can duplicate delivery and requires authorization/audit policy;
- dead-letter and poison-message automation are not implemented;
- Redis is a transport, not the audit source of truth; and
- PostgreSQL/Redis clients and relay lifecycle belong to the application.

The prerequisites include pinned PostgreSQL and Redis container commands,
bounded readiness checks, environment configuration, and rerun guidance that
uses a new order/command identity instead of deleting or rewriting a committed
record.

The root README locale pair adds one navigation row and a short lesson section
only if the existing root structure uses per-example detail. It does not copy
the full package README.

## Compatibility, Migration, and Rollback

The example adds only workshop files and uses dependencies already present in
`go.mod`: bluetape-go v0.18.0, pgx, and go-redis. It changes no public library
API, database owned by another example, module registration, or workflow.
Schemas are example-prefixed and created idempotently.

Rollback is deletion of the example, root navigation entries, paired diagrams,
and durable workflow artifacts. No production migration exists. An existing
example database can drop `transactional_outbox_orders` and the configured
outbox table only when it is known to be disposable; the README does not publish
destructive SQL as a normal rollback command.

## Acceptance Criteria and DoD

- Order and outbox entry are committed in one PostgreSQL transaction and roll
  back together on every pre-commit failure.
- The relay proves success, transient failure, retry, duplicate attempt, stable
  event/idempotency identity, cancellation, dead letter, and clean shutdown.
- Redis integration uses `audit/sqloutbox/redisstreams`, verifies every
  documented field plus `entry_json`, and never hand-writes publish mapping.
- PostgreSQL and Redis integration is sequential, bounded, readiness-proved,
  and deterministically cleaned up.
- English and Korean README files explain the run path, expected record,
  at-least-once delivery, consumer deduplication, replay, poison-message limits,
  and ownership boundaries; root navigation links the example.
- Architecture and sequence SVG/PNG pairs pass source, render, audit, marker,
  geometry, endpoint, mixed-corner, exposure, and full-size visual gates.
- Focused tests, race validation, runnable smoke evidence where services are
  available, `make ci`, Type A verification/review, lesson, PR, and CI complete
  with P0=0 and P1=0.
- The workflow stops at a green PR and requests explicit user approval before
  merge. After approval, rebase merge, local sync, and owned-worktree cleanup
  are verified separately.

## Specification Review Record

The installed-role review could not be dispatched because the active subagent
interface does not expose the required `agent_type` field. Following the routing
contract, the main session performed six isolated reviews of this exact spec and
integrated their findings against the v0.18.0 `go doc`, tagged source, live issue,
GNO evidence, and repository rules.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0, P2=1 | Kept the teaching batch at one record, bounded Redis inspection with `XRangeN`/`XRevRangeN`, and deferred production batch tuning to measured capacity. |
| Stability | P0=0, P1=0 after repair | Added an exact successful batch invariant, shared injected clock for retry eligibility, claimed-record cancellation semantics, joined continuous-run shutdown, and fail-closed handling for stale pending rows. |
| Security | P0=0, P1=0, P2=1 | Bounded author, identifiers, stream key, and timestamp parsing; prohibited endpoint interpolation and successful payload/error disclosure; retained a warning that diagnostic stderr is sensitive. |
| Operator/Ops | P0=0, P1=0 | Defined readiness commands, idempotent partial-DDL recovery, durable-source ownership, dead-letter/replay limits, nonzero exits, and disposable rollback boundaries. |
| Developer/API | P0=0, P1=0 after repair | Changed schema creation to service-owned store delegation, fixed the example-specific table, specified nil/zero/error behavior, and separated occurred-at from recorded-at ownership. |
| User/caller | P0=0, P1=0, P2=1 | Added pinned container prerequisites, rerun identity guidance, expected output/field requirements, and explicit exactly-once, poison-message, replay, and consumer-deduplication warnings. |
| Main integration | P0=0, P1=0 | All issue acceptance criteria map to a source-backed component, test, document, diagram gate, or explicit non-goal; no dependency, workflow, public API, or issue #68 scope leak remains. |
