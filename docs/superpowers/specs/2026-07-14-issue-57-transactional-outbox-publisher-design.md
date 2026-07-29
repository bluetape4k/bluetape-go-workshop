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

focused TDD는 다음을 증명한다.

- constructor, zero-value, nil database, identifier, total, timestamp, context validation
- real PostgreSQL에서 order insert 이후 발생하는 order conflict 및 outbox identity conflict에 대한 atomic
  order/outbox commit과 rollback
- `RecordingPublisher`를 사용한 successful `RunOnce`, transient failure, time-controlled retry,
  duplicate attempt, stable `EventID`/`IdempotencyKey`, max-attempt dead letter, inspectable error
- retry/dead-letter conversion 없는 `PublisherFunc` caller cancellation
- continuous `Run` cancellation, joined goroutine, late publish 없음
- PostgreSQL plus Redis readiness, official adapter publish, 모든 documented stream field, valid
  `entry_json`, deterministic cleanup
- public listener를 열지 않는 runnable configuration, output shape, error propagation, resource closure

concurrency, retry, lifecycle test는 `go test -race` 아래에서 실행한다. 어떤 test도 unbounded sleep을
사용하지 않는다. injected clock과 bounded channel이 state를 조율한다. validation order는 focused package
test, focused race test, sequential container integration package, local service를 사용할 수 있을 때의
configured `go run` smoke check, diagram audit, `git diff --check`, repository-wide `make ci`다.

## README Contract

`README.md`와 `README.ko.md`는 language switch, package lesson, architecture 및 sequence diagram,
prerequisite, exact run command, expected JSON 및 stream field, focused test command, production
boundary를 포함한다. 이들은 다음을 명시한다.

- order와 outbox는 atomically commit되지만 Redis publication은 asynchronous다.
- delivery는 exactly-once가 아니라 at-least-once다.
- consumer는 stable event identity로 deduplicate한다.
- replay는 delivery를 duplicate할 수 있으며 authorization/audit policy가 필요하다.
- dead-letter 및 poison-message automation은 구현하지 않는다.
- Redis는 audit source of truth가 아니라 transport다.
- PostgreSQL/Redis client와 relay lifecycle은 application에 속한다.

prerequisite에는 pinned PostgreSQL 및 Redis container command, bounded readiness check, environment
configuration, committed record를 삭제하거나 rewrite하는 대신 새 order/command identity를 사용하는 rerun
guidance가 포함된다.

root README locale pair는 기존 root structure가 per-example detail을 사용할 때만 navigation row 하나와 짧은
lesson section을 추가한다. full package README를 복사하지 않는다.

## Compatibility, Migration, and Rollback

example은 workshop file만 추가하고 `go.mod`에 이미 있는 dependency인 bluetape-go v0.18.0, pgx,
go-redis를 사용한다. public library API, 다른 example이 소유한 database, module registration, workflow를
변경하지 않는다. schema는 example-prefixed이며 idempotently 생성된다.

rollback은 example, root navigation entry, paired diagram, durable workflow artifact를 삭제하는 것이다.
production migration은 없다. 기존 example database가 disposable임이 알려진 경우에만
`transactional_outbox_orders`와 configured outbox table을 drop할 수 있다. README는 destructive SQL을
normal rollback command로 게시하지 않는다.

## Acceptance Criteria and DoD

- order와 outbox entry는 하나의 PostgreSQL transaction에서 commit되고 모든 pre-commit failure에서 함께
  rollback된다.
- relay는 success, transient failure, retry, duplicate attempt, stable event/idempotency identity,
  cancellation, dead letter, clean shutdown을 증명한다.
- Redis integration은 `audit/sqloutbox/redisstreams`를 사용하고 모든 documented field와 `entry_json`을
  검증하며 publish mapping을 hand-write하지 않는다.
- PostgreSQL 및 Redis integration은 sequential, bounded, readiness-proved이며 deterministic하게 cleanup된다.
- English 및 Korean README file은 run path, expected record, at-least-once delivery, consumer
  deduplication, replay, poison-message limit, ownership boundary를 설명한다. root navigation은 example을
  link한다.
- architecture 및 sequence SVG/PNG pair는 source, render, audit, marker, geometry, endpoint,
  mixed-corner, exposure, full-size visual gate를 통과한다.
- focused test, race validation, service가 available할 때의 runnable smoke evidence, `make ci`,
  Type A verification/review, lesson, PR, CI가 P0=0/P1=0으로 완료된다.
- workflow는 green PR에서 멈추고 merge 전에 explicit user approval을 요청한다. approval 이후 rebase merge,
  local sync, owned-worktree cleanup은 별도로 검증한다.

## Specification Review Record

active subagent interface가 required `agent_type` field를 노출하지 않아 installed-role review를 dispatch할
수 없었다. routing contract에 따라 main session이 이 exact spec을 여섯 번 isolated review하고, v0.18.0
`go doc`, tagged source, live issue, GNO evidence, repository rule에 맞춰 finding을 통합했다.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0, P2=1 | teaching batch를 record 하나로 유지하고, Redis inspection을 `XRangeN`/`XRevRangeN`으로 제한했으며, production batch tuning은 measured capacity로 미뤘다. |
| Stability | P0=0, P1=0 after repair | exact successful batch invariant, retry eligibility용 shared injected clock, claimed-record cancellation semantic, joined continuous-run shutdown, stale pending row에 대한 fail-closed handling을 추가했다. |
| Security | P0=0, P1=0, P2=1 | author, identifier, stream key, timestamp parsing을 bounded로 제한했다. endpoint interpolation과 successful payload/error disclosure를 금지했고 diagnostic stderr가 sensitive라는 warning을 유지했다. |
| Operator/Ops | P0=0, P1=0 | readiness command, idempotent partial-DDL recovery, durable-source ownership, dead-letter/replay limit, nonzero exit, disposable rollback boundary를 정의했다. |
| Developer/API | P0=0, P1=0 after repair | schema creation을 service-owned store delegation으로 바꾸고, example-specific table을 고정했으며, nil/zero/error behavior를 지정하고 occurred-at ownership과 recorded-at ownership을 분리했다. |
| User/caller | P0=0, P1=0, P2=1 | pinned container prerequisite, rerun identity guidance, expected output/field requirement, 명시적인 exactly-once, poison-message, replay, consumer-deduplication warning을 추가했다. |
| Main integration | P0=0, P1=0 | 모든 issue acceptance criterion이 source-backed component, test, document, diagram gate 또는 explicit non-goal에 mapping된다. dependency, workflow, public API, issue #68 scope leak는 남지 않았다. |
