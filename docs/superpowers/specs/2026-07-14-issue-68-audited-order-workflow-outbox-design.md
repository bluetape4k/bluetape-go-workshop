# Issue #68 SQL Outbox를 사용하는 Audited Order Workflow 설계

## Status

high-level design은 2026-07-14에 user가 승인했다. detailed Type A specification review는
P0=0/P1=0으로 수렴했고 final user review도 2026-07-14에 승인되었다. 대상은 현재 workshop
dependency인 bluetape-go v0.18.0이다.

## Goal

durable order workflow, immutable audit history, asynchronous Redis Streams delivery를 보여주는 실행
가능한 Gin application 하나를 추가한다. accepted command마다 order state, query 가능한 audit entry 하나,
SQL outbox record 하나를 같은 PostgreSQL transaction에서 commit해야 한다. operator와 caller는 committed
history와 asynchronous delivery를 구분할 수 있어야 하며, 예제는 exactly-once delivery를 주장하면 안 된다.

application은 command와 audit query를 POST JSON endpoint로 노출해 크거나 확장 가능한 query criteria가 URL
길이에 의존하지 않게 한다. 두 README locale은 완전한 `curl` example을 포함하고, checked-in
`requests.http` file은 optional metadata를 포함한 동일 runnable scenario를 제공한다.

## Context and Current Evidence

Issue #68은 milestone track #35의 integration example이다. 완료된 prerequisite은 audit history example
(#56), Gin audit query example (#58), transactional outbox publisher example (#57)이다. workshop은
`github.com/bluetape4k/bluetape-go` v0.18.0을 resolve하며, 이는 2026-07-14에 관찰된 latest stable
release이기도 하다.

released `audit` package는 immutable entry, JSON encoding, history, `HistoryReader` query contract를
정의한다. bundled `MemoryRepository`는 durable application repository가 아니다. released
`audit/sqloutbox` package는 caller-session `Store`, schema creation, transactional `Enqueue`, claim 및
completion transition, continuous `Relay`를 제공한다. released `audit/sqloutbox/redisstreams` adapter는
stable audit envelope를 caller-owned Redis client에 at-least-once semantic으로 publish한다.

SQL outbox는 audit read model이 아니라 transport ledger다. 그 `entry_json`을 query하면 business history
retention 및 pagination이 delivery cleanup에 결합되며, outbox store는 `audit.HistoryReader`를 구현하지
않는다. 따라서 application은 별도의 immutable PostgreSQL history table을 소유하고, delivery에는 official
SQL outbox를 변경 없이 사용한다.

repository baseline full `make ci` run은 container-backed test suite까지 도달했지만, 많은 package가
Docker resource를 parallel로 시작하는 동안 기존 PostgreSQL fixture 하나가 timeout되었다. 같은 기존
package는 단독 실행에서 4.477초에 통과했고, 바로 이전 change는 같은 base revision에서 full gate를
통과했다. 이는 source failure가 아니라 host resource contention으로 취급한다. 새 PostgreSQL 및 Redis
integration test는 container를 sequentially 시작해야 하며, completion에는 여전히 fresh observed
`make ci` exit 0이 필요하다.

## Chosen Approach

`examples/audited-order-workflow-outbox`를 네 application boundary로 만든다.

1. order service는 create 및 transition command를 검증한다. 하나의 `sqlkit.WithTx` 안에서 order row를
   write하고 application history store에 immutable audit entry 하나를 insert하며, 같은 `*sql.Tx`와 같은
   validated `audit.Entry` value로 `sqloutbox.Store.Enqueue`를 호출한다.
2. PostgreSQL history store는 outbox delivery state 및 retention과 독립적으로 `audit.HistoryReader`를
   구현한다. HTTP audit search와 detail은 이 store만 사용한다.
3. background `sqloutbox.Relay.Run`은 committed outbox record를 claim하고 released Redis Streams
   publisher를 사용한다. Redis는 command transaction에 절대 참여하지 않는다.
4. Gin server는 strict POST JSON transport, bounded timeout, readiness, signal-driven shutdown,
   redacted error response를 소유한다.

application에는 Redis consumer가 없다. runnable scenario는 producer-side contract를 보여주고, delivery가
delayed 또는 retried되어도 committed audit query가 계속 available함을 보여준다.

## Rejected Alternatives

### Query the SQL outbox as audit history

table 하나를 피할 수 있지만, published record가 cleanup될 때 history가 사라지고 domain query boundary를
통해 delivery state가 노출되며 internal transport schema 위에 application-specific query가 필요해진다.
transport retention은 business history retention이 아니므로 거부한다.

### Use `audit.MemoryRepository` with a SQL outbox

full reader contract를 reuse할 수 있지만 restart 시 history를 잃고 database transaction은 commit됐는데
in-memory insert가 실패하거나 그 반대가 발생할 수 있다. integration lesson은 durable atomic state이므로
거부한다.

### Publish directly to Redis in the order transaction

code path는 짧아지지만 dual-write failure window를 만들고 network call 동안 database lock을 잡게 된다.
official SQL outbox가 가르치려는 feature이므로 거부한다.

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

root README pair는 새 example을 link한다. diagram은 canonical generated asset path를 사용한다.

```text
docs/images/readme-diagrams/
  audited-order-workflow-outbox-architecture.svg
  audited-order-workflow-outbox-architecture.png
  audited-order-workflow-outbox-sequence.svg
  audited-order-workflow-outbox-sequence.png
```

새 Go module, third-party dependency, workflow, consumer group, generic library abstraction,
bluetape-go API는 필요하지 않다.

## Domain Model

order는 다음을 포함한다.

```go
type Order struct {
    OrderID    string    `json:"order_id"`
    Status     Status    `json:"status"`
    Revision   int64     `json:"revision"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

allowed state는 `pending`, `confirmed`, `cancelled`이다. Create는 revision 1의 `pending`을 만든다.
Confirm은 `pending`만 받아 `confirmed`를 만든다. Cancel은 `pending` 또는 `confirmed`를 받아
`cancelled`를 만든다. 어떤 transition도 `cancelled`를 벗어나지 않으며 이미 confirmed된 order를 다시
confirm하지 않는다. accepted transition마다 revision은 정확히 한 번 증가한다.

Create command는 `order_id`, `command_id`, optional metadata를 가진다. Transition command는
`order_id`, `command_id`, `action`, optional bounded `reason`, optional metadata를 가진다.
identifier는 trimmed ASCII이며 `[A-Za-z0-9][A-Za-z0-9._:-]{0,127}`과 일치해야 한다. 이는 case를
normalize하지 않으면서 database key, response value, structured log attribute에 control character가
들어가지 않게 한다. reason은 valid UTF-8이며 최대 500 rune이다. metadata는 최대 32 key를 가진 JSON
object다. key는 1~64 rune이고 value는 최대 512 rune의 JSON string이며, encoded request는 여전히 global
body limit을 따른다.

command ID는 `EventID`와 `IdempotencyKey`가 모두 된다. audit entry는 aggregate type `order`, aggregate
ID로 order ID, resulting order revision, event type `order.created`, `order.confirmed`,
`order.cancelled`, application clock에서 온 UTC timestamp를 사용한다. payload는 resulting status와
transition 설명에 필요한 command field만 포함한다. metadata는 validation 이후 entry의 structured
metadata로 복사된다. configured application author는 모든 entry에 사용된다. service는 order와 event를
구성하기 전에 clock value 하나를 `UTC().Truncate(time.Microsecond)`로 normalize하고, 같은 값을 SQL scalar
column과 `entry_json`에 사용한다. 이는 PostgreSQL timestamp precision과 맞으며 parity check를
deterministic하게 만든다.

## PostgreSQL Schema

application은 세 table을 소유한다.

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

history table은 bounded time query를 위해 `(aggregate_type, aggregate_id, recorded_at, revision)`
index를 가진다. generated `position`은 domain identity가 되지 않으면서 released query contract의
cross-aggregate append order를 보존한다. scalar column은 routing 및 uniqueness guard다. `entry_json`은
canonical decoded entry다. read 시 decoded aggregate identity, revision, event ID, idempotency key,
event type, recorded time은 scalar column과 일치해야 한다. mismatch는 partial response가 아니라 internal
data-integrity error다.

schema creation은 idempotent startup work다. server가 ready를 보고하기 전에 order table, history table,
index, official outbox schema를 만든다. DDL은 order command transaction 안에서 실행되지 않는다. schema
step이 하나라도 실패하면 startup은 fail closed한다.

## Atomic Command Contract

Create는 transaction을 열기 전에 complete command를 검증한다. 하나의 `sqlkit.WithTx` 안에서 command
identity를 확인하고 pending order를 insert한 뒤, revision-1 `audit.Entry`를 구성하고 검증한다. 이어서 그
entry를 `HistoryStore.Insert`로 insert하고 `Store.Enqueue(ctx, tx, entry)`를 호출한다. command는
transaction commit 이후에만 반환된다. matching command intent가 없는 duplicate order는 conflict다.

Transition은 transaction을 열기 전에 transport-independent field를 검증한다. 하나의 `sqlkit.WithTx` 안에서
`SELECT ... FOR UPDATE`로 order를 읽고, current state를 검증하기 전에 command identity와 canonical intent를
확인한다. 그다음 requested state change를 검증하고 next revision을 계산하며, immutable entry 하나를
구성하고, locked prior revision을 guard로 삼아 order를 update하고, history entry를 insert하며, 같은 entry를
enqueue한다. 실패하면 세 write가 모두 rollback된다.

service는 Redis에 쓰지 않는다. commit outcome이 ambiguous해도 cancelled 또는 timed-out caller는 error를
받는다. 같은 command ID는 안전하게 query하거나 retry할 수 있다. command identity에 대한 database
uniqueness는 retry가 두 번째 logical event를 만들지 못하게 한다. mutation 전에 service는 기존 command
identity를 조회하고 그 canonical intent를 비교한다. transition은 order lock을 잡고 state validation 전에
이를 수행한다. 이후 어떤 command가 concurrent event-ID 또는 idempotency-key uniqueness race에서 지면,
service가 fresh database operation으로 winner를 reload하기 전에 전체 transaction이 rollback된다. matching
intent는 idempotent replay로 audit payload의 original committed order projection을 반환한다. later order
state를 반환하지 않는다. 다른 order, action, reason, metadata에 재사용하면 conflict다. 비교는 raw JSON byte
equality가 아니라 audit entry에 저장된 canonical validated command projection을 사용한다. test는 ordinary
retry, ambiguous-commit retry, concurrent identical intent, order 간 concurrent conflicting reuse를 다룬다.

## History Store Contract

`HistoryStore`는 application write boundary와 released reader contract를 함께 노출한다.

```go
func (s *HistoryStore) Insert(context.Context, sqlkit.Execer, audit.Entry) error
func (s *HistoryStore) Find(context.Context, audit.Query) ([]audit.Entry, error)
func (s *HistoryStore) LoadHistory(context.Context, audit.AggregateID) (audit.History, bool, error)
func (s *HistoryStore) Latest(context.Context, audit.AggregateID) (audit.Entry, bool, error)
func (s *HistoryStore) LatestSnapshot(context.Context, audit.AggregateID) (audit.Entry, bool, error)
func (s *HistoryStore) PreviousSnapshot(context.Context, audit.AggregateID, audit.Revision) (audit.Entry, bool, error)
```

constructor는 nil database와 invalid table configuration을 거부한다. store는 placeholder와 trusted fixed
identifier를 사용한다. request data는 SQL syntax가 되지 않는다. `Insert`는 caller의 transaction을
`sqlkit.Execer`로 받고, `Entry.Validate`를 호출하며, `encoding/json`으로 encode하고, official store와 같은
1 MiB maximum entry size를 강제한다. 그리고 transaction을 직접 begin, commit, rollback하지 않는다.

`Find`는 `audit.Query.Validate`를 호출하고 그 semantic을 보존한다. 이는 optional exact aggregate 또는
aggregate-type filter, inclusive revision 및 recorded-time bound, `position`을 통한 stable append order,
`NewestFirst`, `Limit`이다. 따라서 exact-aggregate result는 이 service에서 revision order가 된다. HTTP
layer는 항상 exact aggregate를 제공하고 `limit + 1` record를 요청하며, 최대 `limit`까지만 반환하고
unbounded count 없이 next-revision cursor를 파생한다. `LoadHistory`, `Latest`, snapshot query는 aggregate
identity로 bounded된다. snapshot query는 non-null snapshot을 포함하는 JSON entry를 filter하면서 decoded
scalar parity도 계속 검증한다. order workflow는 snapshot을 만들지 않으므로 snapshot absence는 정상이며
released reader semantic에 따라 `found=false`를 반환한다.

## HTTP API

server는 debug mode 없이 Gin을 사용하고 다음을 노출한다.

```text
POST /orders
POST /orders/transitions
POST /audit/history/search
POST /audit/history/detail
GET  /healthz
GET  /readyz
GET  /statusz
```

모든 command 및 query body는 `Content-Type: application/json`을 요구하며 decoding 전에 32 KiB로 제한된다.
compressed body, invalid UTF-8, unknown field, duplicate object key, trailing JSON value, non-object
top-level value, empty body는 거부한다. decoder는 integer precision을 보존한다. metadata는 이 rule을 약화하지
않는다.

`POST /orders`는 다음을 받는다.

```json
{
  "order_id": "order-1001",
  "command_id": "cmd-create-1001",
  "metadata": {"channel": "workshop"}
}
```

`POST /orders/transitions`는 다음을 받는다.

```json
{
  "order_id": "order-1001",
  "command_id": "cmd-confirm-1001",
  "action": "confirm",
  "metadata": {"operator": "demo"}
}
```

create response는 new order일 때 HTTP 201이고 idempotent replay일 때 HTTP 200이다. new 또는 replayed
transition은 HTTP 200을 반환한다. successful response는 다음 complete shape를 사용한다.

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

identical retry는 `replayed`를 true로 설정하고, aggregate가 이후 advance되었더라도 original committed
order projection을 반환한다. 새 revision이나 outbox record를 만들지 않는다. response는 Redis publish가 아니라
durable commit을 확인한다.

`POST /audit/history/search`는 다음 canonical shape를 받는다.

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

네 bound는 생략하거나 null일 수 있다. 존재하는 revision은 positive여야 하고, 존재하는 time은 offset이 있는
RFC3339여야 한다. lower 및 upper bound는 inclusive이고, lower bound는 upper bound를 초과할 수 없으며,
`limit`은 1부터 100까지의 값으로 필수다. successful response는 ascending revision order의 `entries`와
nullable `next_from_revision`을 포함한다.

cursor는 fetch되었지만 반환되지 않은 첫 revision이다. revision 1과 2가 있고 `limit: 1`이면 page one은
`from_revision: 1`을 제공하고 revision 1과 `next_from_revision: 2`를 반환한다. page two는
`from_revision: 2`를 제공하고 revision 2와 null cursor를 반환한다. next cursor가 첫 unreturned entry를
가리키고 lower bound가 inclusive이므로 revision은 반복되거나 건너뛰지 않는다.

`POST /audit/history/detail`은 다음을 받는다.

```json
{
  "aggregate": {"type": "order", "id": "order-1001"},
  "revision": 2
}
```

revision은 positive여야 한다. 두 audit endpoint는 PostgreSQL history만 읽으며 relay가 stopped 상태여도
correct하게 동작한다.

transport error는 non-sensitive code와 message를 가진
`{"request_id":"req-...","error":{"code":"...","message":"..."}}`를 사용한다. invalid JSON과 validation
failure는 400을 반환하고, oversized body는 413을 반환하며, unsupported media type 또는 content encoding은
415를 반환한다. missing order 또는 audit revision은 404를 반환한다. duplicate order identity, conflicting
command reuse, invalid transition은 409를 반환한다. server-side operation deadline이 request context 때문에
발생하면 408을 반환한다. concurrency-cap rejection은 code `too_many_requests`, `Retry-After: 1`,
`Connection: close`, closed request body와 함께 429를 반환한다. handler가 실행되지 않았으므로 caller가 같은
command ID를 재사용하면 retry가 안전하다. unexpected storage error는 endpoint value, SQL text, credential,
entry payload, raw provider message 없이 500을 반환한다. publisher failure는 asynchronous이며 relay retry,
dead-letter state, delivery status, safe log로만 나타난다. per-request HTTP mapping은 없다.

## Runtime, Relay, and Shutdown

required configuration은 `DATABASE_URL`과 `REDIS_ADDR`다. optional value는 `REDIS_STREAM`과
`HTTP_ADDR`다. `HTTP_ADDR`는 기본값이 `127.0.0.1:8080`이고 loopback IP를 포함해야 한다. 예제에는
authentication이 없으므로 wildcard, hostname-only, non-loopback bind는 fail closed한다. unsafe remote
opt-in은 없다. `REDIS_STREAM`은 trusted application configuration이지만 여전히 valid UTF-8, non-blank,
최대 256 byte여야 한다.

application은 PostgreSQL 및 Redis client를 소유한다. startup은 redacted configuration을 parse하고, 두
client를 열고, bounded ping을 수행하고, schema를 만들고, released Redis Streams publisher 하나를 구성하고,
background relay 하나를 시작한 뒤 HTTP를 serve한다. 모든 startup work가 성공하기 전까지 readiness는 false다.
startup, request, relay, shutdown log는 stage, stable error class, request ID, 필요한 경우
grammar-validated identity만 포함한다. raw provider error, endpoint value, caller metadata는 절대 log하지 않는다.

PostgreSQL은 maximum open/idle connection 8개, maximum idle time 5분, maximum lifetime 30분으로 설정한다.
Redis는 pool 8개, minimum idle connection 1개, pool timeout 2초를 사용한다. server는 최대 32개의
in-flight request를 허용하고 body를 decode하기 전에 초과 작업을 429로 거부한다. request operation deadline
2초, header timeout 2초, read/write timeout 5초, idle timeout 30초, maximum header size 16 KiB,
graceful-shutdown deadline 5초를 사용한다. 이 teaching default는 명시적이며 production sizing advice가 아니다.

relay는 명시적인 bounded option을 사용한다.

```go
sqloutbox.RelayOptions{
    ClaimLimit:  16,
    MaxAttempts: 3,
    RetryDelay:  250 * time.Millisecond,
    IdleDelay:   50 * time.Millisecond,
}
```

`/healthz`는 process liveness만 보고한다. `/readyz`는 짧고 bounded한 PostgreSQL check와 실행 중인
supervised relay를 요구한다. Redis failure는 200 response 안에서 `"delivery":"degraded"`로 보고되지만,
durable command 및 history service를 traffic에서 제거하지 않는다. application은 outbox availability
boundary를 깨는 대신 local-demo backlog를 의도적으로 받아들인다. database failure 또는 stopped relay는
503을 반환한다. outbox emptiness는 readiness condition이 아니다.

`/statusz`는 bounded delivery diagnostic만 노출한다. 여기에는 Redis status, relay state,
pending/retrying/claimed/published/dead-letter count, 초 단위로 반올림한 oldest-pending age가 포함된다.
entry identity, payload, metadata, endpoint, provider error는 절대 반환하지 않는다. read-only status query는
250밀리초 deadline 아래에서 fixed official table과 status index를 사용한다. timeout은 readiness를 막지 않고
degraded status를 반환한다. official v0.18.0 `Relay.Run`은 per-batch result가 아니라 terminal error만
노출하므로, application은 batch count를 log하려고 custom polling loop로 대체하지 않는다. delivery degradation과
lifecycle transition은 idle-loop noise를 피하기 위해 transition마다 한 번만 log한다. current count는
`/statusz`로 계속 available하다. relay goroutine은 terminal result를 lifecycle owner에게 보고한다.
unexpected `Relay.Run` error는 먼저 readiness를 false로 만들고, 같은 bounded server shutdown을 시작한 뒤
process failure를 유발한다. `context.Canceled`는 lifecycle owner가 shutdown을 요청한 뒤에만 successful이다.
early cancellation은 unexpected다.

shutdown은 먼저 HTTP work 수락을 중지하고 bounded in-flight request를 기다린다. 그런 다음 relay context를
cancel하고 relay goroutine을 join하며 Redis 및 PostgreSQL client를 닫는다. caller cancellation은 보존한다.
shutdown 중 cancel된 relay publish는 published로 표시되지 않고 retry로 변환되지도 않는다. 그 claim은 released
lease contract 아래에서 recoverable해진다. shutdown은 secret을 노출하지 않고 independent error를 join하며 hard
deadline을 가진다.

delivery는 at-least-once다. accepted Redis append 뒤에 ambiguous client failure, failed SQL completion,
expired claim이 발생하면 같은 logical event가 한 번 넘게 append될 수 있다. official store는 earlier revision이
pending 또는 claimed인 동안 later pending revision을 막지만, dead-letter, replay, ambiguous publish outcome 때문에
application은 consumer-observed ordering을 promise하지 않는다. consumer는 `event_id` 또는 `idempotency_key`로
deduplicate하고 reordering을 tolerate해야 한다. outbox cleanup, operator replay, Redis consumer group, consumer
idempotency는 이 예제에 숨은 behavior가 아니라 production follow-up concern이다.

## Runnable Documentation Contract

두 README locale은 prerequisite, environment variable, startup, order state machine, atomic
transaction boundary, history-versus-outbox separation, at-least-once behavior, failure recovery,
overload response, shutdown을 설명한다. schema creation은 migration framework가 아니라 single-version
workshop bootstrap임을 명시한다. incompatible existing schema는 startup을 실패시키며 자동으로 변경하거나
drop하지 않는다. application은 세 example table을 모두 소유한다. runbook은 safe outbox status inspection,
Redis outage와 recovery, pending/claimed lease recovery, dead-letter diagnosis, partial startup 이후
restart, 명시적으로 destructive한 local-only reset을 보여준다. table deletion 또는 dead-letter mutation을
production rollback으로 제시하지 않는다. English language switch는 `English | [한국어](README.ko.md)`이고
Korean switch는 `[English](README.md) | 한국어`다. current locale은 plain text다. 같은 deferred
language-switch defect는 issue #57 README pair에서도 수정한다.

문서화된 scenario는 다음에 대해 완전한 `curl` POST JSON command를 사용한다.

1. metadata와 함께 order 생성
2. metadata와 함께 order confirm
3. identical confirm command를 retry하고 새 revision 없이 `replayed: true` 관찰
4. two-page cursor example을 포함한 audit history search
5. revision 2 detail loading
6. 선택적으로 reason과 함께 두 번째 order를 만들고 cancel한 뒤, post-cancellation transition 거부와 stable
   409 code 표시

모든 request는 URL, method, content type, body를 포함한다. checked-in `requests.http` file은 variable과
valid JSON body로 같은 scenario를 반복하므로 JetBrains 및 VS Code REST client가 prose에서 request를
재구성하지 않고 실행할 수 있다. README pair와 HTTP file은 source-equivalent body를 사용한다. expected status,
revision, replay marker, asynchronous-delivery marker, history ordering, 429 retry rule,
invalid-transition code, duplicate-delivery caveat를 보여준다.

README architecture diagram은 Gin route, order service, `sqlkit.WithTx`, order/history/outbox table,
background relay, Redis Streams를 보여준다. sequence diagram은 command validation, row locking, 세
transactional write, commit-before-response, 이후 relay delivery, Redis를 우회하는 history query를 보여준다.
SVG는 source artifact이고 PNG는 GitHub-facing render다. 두 diagram은 `bluetape-diagram`을 따르며 checklist
validation과 SVG/PNG arrowhead direction, bend 및 card boundary의 arrowhead clearance, clipping, text
legibility, 두 format 사이의 correspondence에 대한 visual inspection을 포함한다.

## Failure Modes and Safety Boundaries

- history insert 또는 outbox enqueue failure는 order write를 rollback한다.
- Redis unavailability는 이미 committed command를 rollback하지 않는다. readiness는 degraded delivery
  status와 함께 available하게 남고 relay는 bounded retry/dead-letter policy를 적용한다.
- 같은 canonical intent를 가진 duplicate command는 idempotent replay다. mismatched intent는 409 conflict다.
- concurrent transition은 locked order row를 통해 serialize된다. 정확히 하나의 valid next revision만
  commit되고, 진 쪽 또는 이후 invalid가 된 transition은 conflict된다.
- query pagination은 aggregate별로 bounded이고 stable하다. page 이후 committed entry는 later page에 나타날
  수 있지만 기존 revision을 rewrite할 수 없다.
- corrupt 또는 scalar-mismatched history JSON은 request를 fail closed하고 stage 및 safe identity만 log한다.
- request 및 shutdown log는 database URL, Redis address, credential, arbitrary metadata, audit payload,
  raw request body를 절대 포함하지 않는다.
- 예제는 unauthenticated 및 loopback-only다. remote exposure는 거부되며 이 demonstration 밖에 남는다.
- PostgreSQL 및 Redis Testcontainers는 constrained host contention을 줄이기 위해 sequentially 시작한다. missing
  process handle 또는 unobserved exit code는 test evidence가 아니다.

## Test Strategy

unit 및 handler test는 다음을 증명한다.

- identifier, action, reason, metadata, content type, body size, duplicate key, unknown field,
  trailing value, compression, UTF-8 rejection
- state-machine transition, revision increment, entry identity, payload 및 metadata projection,
  idempotent replay, conflicting command reuse
- 201/200/400/404/408/409/413/415/429/500 response mapping, replay marker, `Retry-After`,
  request-body closure, redaction
- search bound, stable ordering, `limit + 1` cursor derivation, detail lookup, query handler에 Redis
  dependency 없음
- zero-value, nil dependency, cancellation, timeout, close, slow-header/body, concurrency-cap,
  pool-exhaustion, graceful shutdown behavior

PostgreSQL integration test는 released fixture로 다음을 증명한다.

- create 및 transition에서 order, history, outbox가 함께 commit된다.
- 각 write 이후 주입된 failure는 모든 write를 rollback한다.
- 두 concurrent transition은 같은 revision을 commit할 수 없다.
- SQL history store는 모든 `audit.HistoryReader` method를 구현하고 corrupted scalar/JSON parity에서 fail
  closed한다.
- sub-microsecond nanosecond를 포함한 clock은 한 번 normalize되어 SQL scalar parity, history read, relay
  claim이 계속 valid하게 남는다.
- restart는 order state, audit history, idempotent replay, pending outbox record를 보존한다.
- seeded large-history query plan은 intended aggregate/revision 또는 aggregate/time index를 사용하고
  unbounded scan 또는 sort 없이 `limit + 1`을 적용한다. 이는 `EXPLAIN (FORMAT JSON)`으로 검증한다.

relay test는 `sqloutboxtest.RecordingPublisher`와 `PublisherFunc`를 사용해 success, 한 번의 injected
retry, attempt 간 duplicate logical identity, dead-letter bound, caller cancellation, lease recovery,
leaked goroutine 없는 continuous `Run` shutdown을 증명한다. real Redis integration test는 PostgreSQL 이후
시작되고 released provider의 documented 13-field envelope와 created/confirmed event의 stable
event/idempotency identity를 검증한다. 하나의 logical event에 대한 여러 physical stream entry를 허용한다.
bounded backlog test는 command가 계속 write되는 동안 여러 claim batch를 drain하고, claimed/published count를
기록하며, configured connection-pool ceiling을 관찰하고 fixed deadline 전에 완료한다. claim size와 polling
delay는 이 test로 검증된 conservative teaching value이지 benchmark-derived production tuning이 아니다.
lifecycle test는 unexpected relay exit가 readiness를 false로 만들고 serving을 terminate함을 증명한다.
observability test는 Redis outage 및 recovery 사이의 readiness transition, accurate bounded status count와
oldest-pending age, lifecycle summary, idle-loop log suppression, payload/metadata/endpoint/credential/provider
error 부재를 증명한다.

validation은 targeted package test, race test, sequential container-backed integration test,
resource-bounded `go test -p 1 -count=1 ./...` lane, checked-in POST JSON request를 사용하는 runnable server
smoke scenario, diagram render 및 checklist validation, fresh `make ci` 순서로 실행한다. 모든 final gate는
새로 관찰한 exit code를 가져야 한다.

## Acceptance Criteria

- example은 application-shaped이며 bluetape-go v0.18.0 기준으로 compile된다.
- create 및 transition command는 order, durable history, official SQL outbox state를 atomically commit한다.
- audit search와 detail은 Redis 또는 outbox row가 아니라 SQL history store를 사용한다.
- background official relay는 explicit at-least-once semantic으로 official Redis Streams adapter를 통해
  publish한다.
- 모든 user operation은 strict POST JSON을 사용한다. health와 readiness는 GET으로 남는다.
- 두 README locale과 `requests.http`는 완전한 runnable request를 제공한다.
- architecture 및 sequence SVG/PNG pair는 arrowhead direction과 bend clearance를 포함한 automated check와
  manual visual inspection을 통과한다.
- issue #57 README language switch는 current locale을 plain text로 보여준다.
- targeted, race, integration, smoke, full `make ci` gate는 fresh observed exit code와 함께 통과한다.
- PR은 assigned, labeled, milestone 0.9.0 attached 상태이고 issue #68을 link하며 green CI에 도달한다.
  merge는 별도 user approval gate로 남는다.

## Specification Review Convergence

| Perspective | Result | Resolved focus |
|---|---|---|
| Performance | P0=0, P1=0 | pool limit, indexed bounded history query, backlog contention evidence를 명시했다. |
| Stability | P0=0, P1=0 | race-safe replay, ambiguous commit, relay supervision, delivery ordering limit, resource-bounded test를 명시했다. |
| Security | P0=0, P1=0 | loopback-only binding, strict JSON, resource cap, safe identifier, all-stage redaction을 명시했다. |
| Operator/Ops | P0=0, P1=0 | Redis degradation, readiness, relay/backlog signal, schema ownership, recovery runbook을 명시했다. |
| Developer/API | P0=0, P1=0 | v0.18.0 reader signature, timestamp precision, asynchronous publisher error ownership을 명시했다. |
| User/caller | P0=0, P1=0 | complete POST JSON body, cursor continuation, replay, overload, cancellation, 409 example을 명시했다. |

모든 review finding은 이 specification에서 수정되었다. affected lane은 integrated artifact 대상으로 다시
실행되어 clear를 반환했다. deferred P2/P3 item은 없다.

## Non-Goals

- bluetape-go에 reusable SQL audit repository 추가
- exactly-once Redis delivery 또는 global event ordering
- Redis consumer, consumer group, consumer-side projection, deduplication store
- authentication, authorization, TLS termination, production deployment
- automated outbox 또는 audit-history retention과 operator replay tooling
- general workflow engine, arbitrary order state, cross-aggregate transaction
