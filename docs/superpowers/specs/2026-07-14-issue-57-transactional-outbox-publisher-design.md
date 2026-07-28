# Issue #57 Transactional Outbox Publisher 설계

## Status

Issue #57에 대한 user-approved design이며 bluetape-go v0.18.0 기준으로 계획되었다. Type A
specification review는 2026-07-14에 P0=0/P1=0으로 수렴했다.

## Goal

하나의 PostgreSQL transaction에서 order row와 audit outbox entry를 commit한 뒤, claimed record를
released Redis Streams adapter로 publish하는 실행 가능한 application-shaped order example을 추가한다.
이 예제는 exactly-once behavior를 주장하지 않으면서 at-least-once delivery, stable event identity,
bounded retry, cancellation, shutdown, duplicate handling, operator replay boundary를 보이게 해야 한다.

## Context and Current Evidence

Issue #57은 완료된 audit history (#56)와 Gin audit query (#58) example 뒤에 오는 milestone track
#35의 세 번째 dependency다. 더 넓은 audited order workflow integration (#68)보다 앞선다. 현재 stable
workshop dependency는 bluetape-go v0.18.0이며 release commit은
`26fe037eb4146f35328c9a549a3adb1207758f50`이다.

released `audit/sqloutbox` package는 caller-session `Store`, `CreateSchema`, `Enqueue`,
claim/retry/completion transition, `RunOnce`와 continuous `Run`을 가진 `Relay`를 제공한다.
`Enqueue`는 `sqlkit.Execer`를 받으므로 application은 order를 write하는 동일한 `*sql.Tx`를 넘길 수
있다. `Relay`는 at-least-once delivery를 문서화하고 caller cancellation을 retry 또는 dead-letter state로
변환하지 않고 보존한다.

released `audit/sqloutbox/sqloutboxtest` package는 concurrent-safe `RecordingPublisher`,
deterministic per-event failure injection, `PublisherFunc`를 제공한다. released
`audit/sqloutbox/redisstreams` package는 caller-owned Redis appender를 받고 publish attempt마다 하나의
`XADD`로 문서화된 stable envelope를 write한다. README는 accepted Redis write 이후 ambiguous client
failure가 발생하면 retry에서 duplicate entry가 생길 수 있다고 명시적으로 경고한다.

기존 `sql-transaction-boundary` workshop example은 `sqlkit.WithTx` ownership, deterministic SQL
behavior, rollback test, PostgreSQL fixture usage의 local source다. 이 예제는 해당 boundary를
빌리지만 그 `internal` package를 확장하거나 import하지 않는다. 기존 `order-pipeline-testcontainers`
example은 released PostgreSQL 및 Redis fixture를 하나의 test에서 순차적으로 시작할 수 있음을 증명한다.

bluetape-go issue #533 및 PR #574에 대한 GNO evidence는 Redis Streams adapter가 의도적으로 좁다는 점을
확인한다. SQL outbox는 durable source of truth로 남고, Redis client는 caller-owned로 남으며, retry는
duplicate stream entry를 append하고, consumer idempotency는 provider 밖에 남는다.

## Chosen Approach

세 explicit boundary를 가진 새 `transactional-outbox-publisher` example을 만든다.

1. `orderoutbox.Service`는 하나의 order command를 검증하고 하나의 immutable audit entry를 만든다.
   그런 뒤 `sqlkit.WithTx`로 order를 insert하고 commit 전에 `Store.Enqueue(ctx, tx, entry)`를
   호출한다. transaction이 열려 있는 동안 Redis 또는 다른 network publish를 수행하지 않는다.
2. caller-owned `sqloutbox.Relay`는 transaction 이후 committed record를 claim하고 publish한다.
   deterministic test는 `RecordingPublisher`와 `PublisherFunc`를 사용한다. runnable 및 integration
   path는 `redisstreams.New`를 직접 사용한다.
3. `main`은 PostgreSQL 및 Redis client creation, readiness check, operation timeout, relay
   construction, cancellation, client closure를 소유한다. bounded `RunOnce` invocation은 sample을
   publish하고 command가 clean하게 exit하도록 한다. indefinitely running CLI는 workshop output을 흐리게
   하므로 continuous `Relay.Run` shutdown은 cancellation test로 별도 증명한다.

runnable command는 idempotent schema를 만들고 caller-selected order를 place하며, 하나의 relay batch를
실행하고 demonstration을 위해 resulting Redis stream entry를 읽는다. 그리고 committed order identity,
relay count, stream key, stable event/idempotency field를 포함한 deterministic-shaped JSON을 출력한다.
timestamp와 order identifier는 repeatable documentation을 위해 environment variable로 제공할 수 있다.
default는 workshop-safe지만 duplicate rerun은 조용히 rewrite되지 않고 conflict로 보고된다.

## Alternatives

### Extend `sql-transaction-boundary`

기존 transaction example에 outbox와 Redis behavior를 추가하면 code를 reuse할 수 있다. 하지만 focused
commit/rollback lesson을 multi-backend delivery application으로 바꾸고 현재의 “outbox comes next”
boundary를 거짓으로 만든다. 거부한다. 대신 source pattern을 독립 example로 빌린다.

### Split deterministic relay and Redis into separate examples

한 example은 `RecordingPublisher`를 쓰고 다른 example은 Redis Streams를 쓸 수도 있다. 각각은 작아지지만
failure semantic을 real adapter에서 분리하고 issue #68이 소유하는 더 넓은 integration과 겹친다. 거부한다.
하나의 example이 deterministic publisher는 test로, Redis adapter는 runnable integration으로 사용한다.

### Publish directly with Redis `XADD`

hand-written mapping은 짧고 시각적으로 직접적일 수 있지만 released provider를 중복하고 field contract에서
drift하며, issue #57이 가르치려는 정확한 at-least-once behavior를 숨긴다. 거부한다. Redis field
encoding은 `redisstreams.New`만 소유할 수 있다.

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

root `README.md`와 `README.ko.md`는 example을 link한다. diagram source와 render는 repository의
canonical path를 사용한다.

```text
docs/images/readme-diagrams/
  transactional-outbox-publisher-architecture.svg
  transactional-outbox-publisher-architecture.png
  transactional-outbox-publisher-sequence.svg
  transactional-outbox-publisher-sequence.png
```

Type A plan, review, lesson artifact는 기존 `docs` path 아래에 남는다. 새 module, dependency, workflow,
public bluetape-go API, changelog, consumer-group implementation은 필요하지 않다.

## Domain and Service Contract

internal package는 다음 teaching surface를 사용한다.

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

`NewService`는 nil store와 blank, invalid UTF-8, 128 rune 초과 author를 거부한다. nil clock은
`time.Now().UTC`를 기본값으로 사용한다. zero-value 또는 nil service는 `ErrInvalidConfig`로 fail
closed한다. `Place`는 transaction을 열기 전에 nil database, blank 또는 oversized ID, invalid UTF-8,
non-positive total, zero timestamp를 거부한다. ID는 trim되고 128 rune으로 제한되며 그 밖의 normalization
또는 rewrite는 하지 않는다. nil context는 `context.Background`로 정규화한다. caller cancellation과
deadline은 보존한다.

order table은 `transactional_outbox_orders`이며 `order_id`를 primary key로 사용하고 bounded customer
ID, `status = 'placed'`, positive total cents, UTC creation time을 가진다. application은
unrelated example과 generic default table을 공유하지 않도록 table `transactional_outbox_records`로
released store를 구성한다. `Service.CreateSchema`는 order table을 만들고 outbox schema creation을 해당
store에 위임한다. schema creation은 idempotent application startup operation이며 order transaction 안에서
실행되지 않는다. partial DDL failure는 숨겨지지 않고 다음 startup에서 retry된다.

accepted command마다 `Place`는 다음을 만든다.

- aggregate type `order`와 `OrderID`와 같은 aggregate ID
- revision `audit.InitialRevision()`
- caller-owned `CommandID`와 같은 event ID 및 idempotency key
- event type `order.placed`
- caller의 UTC-normalized `CreatedAt`에서 온 `OccurredAt`과 service clock에서 온 `RecordedAt`
- customer ID, status, total cents를 가진 fixed JSON payload
- configured author
- `audit.NewEntry`에서 온 schema version

하나의 `sqlkit.WithTx` 안에서 `Place`는 order row를 insert하고 같은 `*sql.Tx`로 `Store.Enqueue`를
호출한다. 반환된 `Order`는 `WithTx` commit 이후에만 observable해진다. SQL uniqueness error는 `%w`를
통해 inspect 가능하게 남는다. 예제는 모든 PostgreSQL code를 새 public error taxonomy로 번역하지 않는다.

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
