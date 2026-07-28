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

application은 명시적인 bounded option으로 relay를 구성한다.

```go
sqloutbox.RelayOptions{
    ClaimLimit:  1,
    MaxAttempts: 3,
    RetryDelay:  250 * time.Millisecond,
    IdleDelay:   50 * time.Millisecond,
}
```

runnable path는 bounded operation context 아래에서 `RunOnce`를 호출한다. deterministic path는 한 번의
injected failure를 가진 `RecordingPublisher`를 사용해 첫 attempt가 기록되고 retry state가 persist되며,
다음 eligible attempt가 같은 `EventID`와 `IdempotencyKey`를 가지고, 두 번째 attempt가 성공적으로
publish됨을 증명한다. test가 sleep 없이 retry eligibility를 진행할 수 있도록 time을 store와 relay에 주입한다.

CLI는 success를 출력하기 전에 clean single-record batch result인
`Claimed=1, Published=1, Failed=0, DeadLettered=0`을 요구한다. example table에 이미 존재하는
pending record는 outbox order에 따라 처리되며, 잘못된 event를 newly published order로 보고하는 대신 sample
identity check를 fail closed하게 만든다. production relay는 measured throughput과 downstream capacity에 맞춰 더
큰 claim limit을 조정해야 한다.

`PublisherFunc`는 publish boundary에서 cancellation을 증명한다. caller context가 cancelled되면
`RunOnce`는 cancellation을 반환하고 record를 failed 또는 dead-lettered로 표시하지 않는다. record는 lease가
reclaim eligible이 될 때까지 claimed 상태로 남을 수 있다. shutdown은 성공적으로 delivered된 것처럼 가장하지
않는다.

Redis path는 caller-owned `go-redis` client와 trusted application stream configuration으로 정확히 하나의
`redisstreams.Publisher`를 구성한다. integration test는 documented provider output을 검증하기 위해서만
bounded `XRangeN`으로 Redis를 읽는다. CLI는 bounded `XRevRangeN`으로 latest candidate 하나를 읽는다.
production publishing은 released adapter 밖에서 `XAdd`를 호출하지 않는다.

delivery는 at-least-once다. 성공한 Redis append 뒤에 SQL completion 실패, expired claim lease,
ambiguous Redis client failure가 이어지면 같은 logical event가 다시 publish될 수 있다. consumer는
`event_id` 또는 `idempotency_key`로 deduplicate해야 한다. `attempts`는 diagnostic이지 identity가 아니다.
ordering은 모든 aggregate 전체가 아니라 SQL outbox claim contract에 의해 aggregate별로 보존된다.

## Runtime and Shutdown Contract

command는 `DATABASE_URL`과 `REDIS_ADDR`를 요구한다. Docker를 시작하지 않고 in-memory infrastructure로
조용히 fallback하지 않는다. optional `REDIS_STREAM`, `ORDER_ID`, `CUSTOMER_ID`, `COMMAND_ID`,
`ORDER_CREATED_AT` 값은 bounded application configuration으로 남는다. created-at 값은 존재하면 RFC3339여야
하며 service clock을 기본값으로 사용한다. `REDIS_STREAM`은 valid UTF-8, non-blank, 최대 256 byte여야 한다.
생략하면 provider의 `audit:sqloutbox` default를 사용한다. required endpoint value는 non-blank다.
configuration parsing은 error 안에 그 내용을 echo하지 않는다.

`main`은 PostgreSQL 및 Redis client를 열고 bounded `Ping` 호출로 둘 다 검증하며, schema를 만들고
order를 place하고 하나의 bounded relay batch를 실행한 뒤 demonstration entry를 읽는다. 모든 exit path에서
두 client를 닫는다. signal-derived root context와 더 짧은 operation context를 사용한다. successful output에는
raw database URL, Redis address, credential, payload, provider error를 출력하지 않는다. failure output은
failed stage 이름을 표시하고 supplied endpoint value를 보간하지 않은 채 provider error를 wrap한다. operator는
그래도 diagnostic stderr를 sensitive로 취급해야 한다.

continuous relay lifecycle은 supported library path지만 default CLI mode는 아니다. 해당 test는
`Relay.Run`을 시작하고 caller context를 cancel하며 goroutine을 기다리고 `context.Canceled`를 요구한다.
또한 late publish 또는 leaked goroutine이 없음을 증명한다. application은 restart policy를 소유하며 caller
cancellation을 retry하지 않는다.

## Redis Stream Evidence

integration test는 released provider의 documented field를 검증한다.

```text
record_id, status, aggregate_type, aggregate_id, revision,
event_id, idempotency_key, event_type, occurred_at, recorded_at,
schema_version, attempts, entry_json
```

`entry_json`을 validated `audit.Entry`로 parse하고 scalar field와 identity parity를 확인한다. 두 fixture가
시작된 뒤 real command로 PostgreSQL 및 Redis readiness를 검증한다. container-backed test는 `t.Parallel`을
사용하지 않고, bounded context를 사용하며, deterministic client cleanup을 등록하고 PostgreSQL과 Redis를
순차적으로 실행한다.

## Failure Modes

1. order insert, audit construction, outbox enqueue failure는 order와 outbox state를 모두 rollback한다.
   실패한 transaction은 relay record를 만들지 않는다.
2. non-cancellation publisher failure는 bounded retry state를 persist한다. `MaxAttempts` 이후 record는
   dead-lettered가 되며 published로 보고되지 않는다. 이 예제는 inspection/replay boundary를 문서화하지만
   automated dead-letter replay command를 추가하지 않는다.
3. publish 중 caller cancellation은 record를 retry/dead-letter state로 변환하지 않고 relay를 중단한다.
   claimed record는 store의 lease behavior를 통해서만 reclaimable해진다.
4. client 또는 completion error가 관찰되기 전에 Redis가 `XADD`를 accept할 수 있다. 이후 retry는 같은 stable
   identity로 duplicate를 append할 수 있다. consumer와 operator replay는 이를 tolerate해야 한다.
5. PostgreSQL 또는 Redis readiness, schema creation, placement, publish, stream verification failure는
   owned resource가 닫힌 뒤 command를 nonzero로 종료한다. ambiguous success log는 없다.
6. duplicate order, event ID, idempotency key conflict는 fail closed한다. 예제는 replacement identity를
   조용히 생성하거나 기존 order를 mutate하지 않는다.

## Security and Operations Boundaries

database 및 Redis endpoint는 request input이 아니라 trusted deployment configuration이다. 예제에는 HTTP
listener, authentication, authorization, multi-tenancy, secrets manager, TLS setup, dynamic stream
selection이 없다. real deployment는 credential을 안전하게 얻고, TLS/network policy를 강제하며, tenant
namespace를 분리하고, Redis stream access를 제한해야 한다.

audit payload는 sensitive data를 포함할 수 있다. 이 예제는 fixed non-sensitive field를 사용하고, SQL과
Redis retention 또는 readership가 다를 때 payload classification/redaction이 enqueue 전에 일어나야 한다고
경고한다. raw provider error는 endpoint detail을 포함할 수 있다. CLI는 stderr용 failure를 wrap하지만
configuration 또는 full audit payload를 emit하지 않는다.

SQL outbox가 durable source of truth다. Redis retention, trimming, consumer group,
pending-entry recovery, consumer acknowledgement, poison message handling, replay authorization은
명시적으로 scope 밖이다. operator는 별도로 승인된 operational tool을 통해서만 failed record를 inspect하고
requeue할 수 있다. manual SQL mutation은 safe workshop procedure로 제시하지 않는다.

## Diagrams

English 및 Korean README file은 두 쌍의 English-label SVG/PNG를 공유한다.

1. static architecture diagram은 “transaction, relay, client, durable state, transport는 누가
   소유하는가?”에 답한다. application ownership, PostgreSQL order/outbox state, relay, Redis
   Streams를 분리한다. 가능한 곳에서는 direct horizontal connector를 사용하고 필요한 곳에서만 rounded
   orthogonal route를 사용한다.
2. chronological sequence diagram은 “하나의 stable event가 어떻게 한 번 넘게 attempt될 수 있는가?”에
   답한다. transaction commit, claim, first publish failure/ambiguous outcome, retry,
   duplicate-capable append, completion, cancellation을 explicit alternate path로 보여준다.

두 asset은 `bluetape-diagram` architecture/sequence rule을 따른다. 각 SVG는 parse되고 CairoSVG scale 2로
render되며 connector, geometry, endpoint, mixed corner, marker color/size/direction을 audit한다. 최종
coordinate 변경 후 full-size PNG로 inspect한다. arrowhead clearance는 bend placement에 포함된다. syntactic
`Q` segment만으로는 visual proof로 인정하지 않는다.

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
