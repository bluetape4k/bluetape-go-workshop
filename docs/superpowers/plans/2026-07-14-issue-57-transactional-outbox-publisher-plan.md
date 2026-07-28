# Issue #57 Transactional Outbox Publisher Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** order와 SQL outbox entry를 atomic하게 commit한 뒤, committed audit record를 released Redis Streams adapter로 publish하는 runnable order example을 만든다. at-least-once, retry, cancellation, shutdown proof를 명시한다.

**Architecture:** `orderoutbox.Service`는 validation과 order insert 및 `sqloutbox.Store.Enqueue`를 감싸는 `sqlkit.WithTx` boundary 하나를 소유한다. caller-owned `sqloutbox.Relay`는 commit 이후 실행된다. deterministic test는 `sqloutboxtest`를 사용하고 runnable integration은 `redisstreams.New`를 사용한다. `main`은 PostgreSQL/Redis readiness, timeout, resource closure, bounded relay batch 하나, JSON output을 소유한다.

**Tech Stack:** Go 1.26.3, bluetape-go v0.18.0 `audit/sqloutbox`, `audit/sqloutbox/sqloutboxtest`, `audit/sqloutbox/redisstreams`, `sqlkit`, pgx v5, go-redis v9, repository PostgreSQL/Redis Testcontainers fixture, CairoSVG를 사용한다.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/transactional-outbox-publisher/internal/orderoutbox/model.go` | domain value, stable error, validation, audit entry construction |
| `examples/transactional-outbox-publisher/internal/orderoutbox/schema.go` | example order schema와 `sqloutbox.Store.CreateSchema` delegation |
| `examples/transactional-outbox-publisher/internal/orderoutbox/service.go` | constructor 및 atomic order/outbox transaction |
| `examples/transactional-outbox-publisher/internal/orderoutbox/service_test.go` | constructor, validation, PostgreSQL atomicity, rollback, cancellation |
| `examples/transactional-outbox-publisher/internal/orderoutbox/relay_test.go` | relay success, retry, duplicate, dead-letter, cancellation, shutdown |
| `examples/transactional-outbox-publisher/internal/orderoutbox/integration_test.go` | sequential PostgreSQL/Redis adapter 및 stream-field proof |
| `examples/transactional-outbox-publisher/main.go` | configuration, client readiness/closure, relay batch, JSON output |
| `examples/transactional-outbox-publisher/main_test.go` | configuration, output, error, runtime proof |
| `examples/transactional-outbox-publisher/README.md`, `README.ko.md` | bilingual lesson 및 operational boundary |
| `README.md`, `README.ko.md` | root navigation 및 run section |
| `docs/images/readme-diagrams/transactional-outbox-publisher-architecture.{svg,png}` | static ownership architecture |
| `docs/images/readme-diagrams/transactional-outbox-publisher-sequence.{svg,png}` | transaction/retry/cancellation sequence |
| `docs/review/2026-07-14-issue-57-transactional-outbox-publisher.md` | final verification/review evidence |
| `docs/lessons/2026-07-14-issue-57-transactional-outbox-publisher.md` | durable implementation/diagram lesson |

`go.mod`, `go.sum`, workflow, module registration, public bluetape-go API, changelog edit는 계획하지 않는다.
이런 diff가 생기면 reapproval을 위해 task를 중단한다.

## Task 1: domain, configuration, schema contract 정의

**Complexity:** Medium. **Depends on:** approved spec. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `model.go`, `schema.go`, `service_test.go`, 그다음 `service.go`.

- [x] **Step 1: constructor 및 validation test를 먼저 작성**

nil store, blank/invalid/oversized author, nil clock, zero-value/nil service, nil database/context,
pre-cancelled context, blank/invalid/oversized ID, non-positive total, zero `CreatedAt`에 대한 table test를 만든다.

```go
func TestNewServiceRejectsInvalidConfiguration(t *testing.T) {
    store, err := sqloutbox.NewStore(sqloutbox.Options{Table: outboxTable})
    if err != nil { t.Fatal(err) }
    tests := []struct {
        name string
        store *sqloutbox.Store
        config Config
    }{
        {name: "nil store", config: Config{Author: "workshop"}},
        {name: "blank author", store: store, config: Config{}},
        {name: "oversized author", store: store, config: Config{Author: strings.Repeat("가", 129)}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service, err := NewService(tt.store, tt.config)
            if service != nil || !errors.Is(err, ErrInvalidConfig) {
                t.Fatalf("NewService() = (%v, %v), want nil ErrInvalidConfig", service, err)
            }
        })
    }
}
```

- [x] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox`

기대값: package와 constructor contract가 없으므로 FAIL한다.

- [x] **Step 3: minimal value 및 constructor 구현**

```go
const (
    aggregateType = "order"
    eventType = audit.EventType("order.placed")
    StatusPlaced = "placed"
    ordersTable = "transactional_outbox_orders"
    outboxTable = "transactional_outbox_records"
    maxIDRunes = 128
)

var (
    ErrInvalidConfig = errors.New("orderoutbox: invalid config")
    ErrInvalidOrder = errors.New("orderoutbox: invalid order")
)

type Config struct { Author string; Now func() time.Time }
type PlaceOrderCommand struct {
    OrderID, CustomerID, CommandID string
    TotalCents int64
    CreatedAt time.Time
}
type Order struct {
    OrderID, CustomerID, Status string
    TotalCents int64
    CreatedAt time.Time
}
```

author를 trim하고 1..128 valid UTF-8 rune으로 bound한다. `Now`는 UTC `time.Now`로 default한다.
caller identifier는 trim 뒤 보존하고 nil/zero-value method는 `%w`-wrapped sentinel로 fail closed하게 만든다.

- [x] **Step 4: schema creation 및 command validation 추가**

`Service.CreateSchema`는 fixed, parameter-free example DDL을 실행하고 configured `sqloutbox.Store`에 delegate한다.

```sql
create table if not exists transactional_outbox_orders (
    order_id text primary key,
    customer_id text not null,
    status text not null check (status = 'placed'),
    total_cents bigint not null check (total_cents > 0),
    created_at timestamptz not null
)
```

nil context는 `context.Background`로 normalize한다. cancellation을 보존하고, ID를 1..128 rune의 valid UTF-8로 validate하며,
positive total을 요구하고, transaction을 열기 전에 `CreatedAt`을 UTC로 normalize한다.

- [x] **Step 5: GREEN 및 format 실행**

```bash
gofmt -w examples/transactional-outbox-publisher/internal/orderoutbox/*.go
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'Test(NewService|ServiceValidation|ServiceCreateSchema)'
```

기대값: PASS하고 `git diff --check`가 clean하다.

## Task 2: order 및 outbox entry를 atomic하게 commit

**Complexity:** High. **Depends on:** Task 1. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `service_test.go`, 그다음 `model.go`와 `service.go`.

- [x] **Step 1: PostgreSQL atomicity test 작성**

90-second context 아래에서 `postgrestestcontainer.Start` instance 하나를 공유하는 sequential subtest를 가진
`TestServicePlacePostgreSQL` 하나를 사용한다. pgx `sql.Open`, real `PingContext`, subtest 간 table reset,
`t.Cleanup`을 사용한다. `t.Parallel`은 절대 사용하지 않는다. successful one-order/one-outbox commit,
duplicate-order rollback, order insert 뒤 outbox identity conflict, pre-cancel rollback, UTC timestamp,
stable event/idempotency identity를 증명한다.

```go
placed, err := service.Place(ctx, db, PlaceOrderCommand{
    OrderID: "order-1001", CustomerID: "customer-42",
    CommandID: "command-1001", TotalCents: 3700,
    CreatedAt: time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC),
})
if err != nil { t.Fatal(err) }
if placed.Status != StatusPlaced { t.Fatalf("status = %q", placed.Status) }
assertTableCount(ctx, t, db, ordersTable, 1)
assertTableCount(ctx, t, db, outboxTable, 1)
```

- [x] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestServicePlace'`

기대값: `Place`와 audit entry construction이 없으므로 FAIL한다.

- [x] **Step 3: released-contract audit entry 하나 생성**

customer ID, status, total cents만 marshal한다. initial revision, event type `order.placed`,
두 identity field에 caller command ID, `OccurredAt=CreatedAt`, `RecordedAt=service clock`을 사용해
`audit.NewAggregateID`, `audit.NewDomainEvent`, `audit.NewEntry`를 호출한다.
모든 error는 `%w`로 wrap한다. caller JSON, metadata, snapshot, generated ID는 추가하지 않는다.

- [x] **Step 4: transaction 하나 구현**

```go
err = sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
    if _, err := tx.ExecContext(ctx,
        `insert into transactional_outbox_orders
         (order_id, customer_id, status, total_cents, created_at)
         values ($1, $2, $3, $4, $5)`,
        command.OrderID, command.CustomerID, StatusPlaced,
        command.TotalCents, command.CreatedAt,
    ); err != nil {
        return fmt.Errorf("insert order: %w", err)
    }
    if err := s.store.Enqueue(ctx, tx, entry); err != nil {
        return fmt.Errorf("enqueue order event: %w", err)
    }
    return nil
})
```

commit 뒤에만 `Order`를 반환한다. transaction 안에서는 Redis, publisher, relay를 절대 호출하지 않는다.

- [x] **Step 5: GREEN 및 focused race 실행**

```bash
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestServicePlace'
go test -race -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestServicePlace'
```

기대값: exact table count와 rollback assertion이 PASS하고 race가 clean하다.

## Task 3: relay retry, duplicate, dead-letter, shutdown 증명

**Complexity:** High. **Depends on:** Task 2. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `relay_test.go`와 shared test helper만 포함한다.

- [x] **Step 1: deterministic success/retry test 작성**

`sqloutbox.Options.Now`와 `RelayOptions.Now`에 같은 mutable clock을 사용한다.
`RecordingPublisher`가 `command-retry`를 한 번 fail하도록 구성한다. 첫 `RunOnce`는 attempt 1로 claimed/failed되고,
pre-retry call은 0개를 claim하며, 정확히 250 ms를 advance하면 두 번째 call이 attempt 2로 publish하는지 assert한다.
두 attempt는 동일한 event/idempotency identity를 보존해야 한다.

```go
publisher := sqloutboxtest.NewRecordingPublisher(
    sqloutboxtest.WithFailures(
        map[audit.EventID]int{"command-retry": 1},
        errors.New("temporary sink failure"),
    ),
)
relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
    ClaimLimit: 1, MaxAttempts: 3, RetryDelay: 250 * time.Millisecond,
    IdleDelay: 50 * time.Millisecond, Now: func() time.Time { return now },
})
```

- [x] **Step 2: dead-letter 및 cancellation test 추가**

eligible attempt 세 개를 fail시키고 final `dead_letter`, attempts 3, published result 없음 을 assert한다.
cancellation에서는 `PublisherFunc`가 caller를 cancel하고 `ctx.Err()`를 반환한다.
`context.Canceled`, status가 `claimed`로 남음, attempts 1, retry/dead-letter write 없음 을 assert한다.

- [x] **Step 3: sleep 없이 continuous `Run` join 증명**

`PublisherFunc`는 `started`를 close하고 `ctx.Done`을 기다린 뒤 context error를 반환한다.
`Relay.Run`을 시작하고 `started`를 기다린 다음 cancel하고 bounded channel로 join한다.
`context.Canceled`, 정확히 한 번의 invocation, late call 없음 을 요구한다.

```go
select {
case err := <-done:
    if !errors.Is(err, context.Canceled) { t.Fatalf("Run() = %v", err) }
case <-time.After(5 * time.Second):
    t.Fatal("relay did not stop after cancellation")
}
```

- [x] **Step 4: bounded concurrent `RunOnce` stress proof 추가**

independent aggregate record 12개를 enqueue하고 closed barrier channel에서 `RunOnce` worker 네 개를 시작한다.
concurrent-safe `RecordingPublisher`를 사용한다. 모든 worker result 전체에서 정확히 12 claimed, 12 published,
failed/dead-letter 0개, unique event ID 12개, SQL row 12개 모두 `published`를 요구한다.
normal test를 10번 반복한다. 어떤 goroutine도 한 call이 return한 뒤 poll하면 안 된다.

- [x] **Step 5: RED 후 GREEN/race 실행**

```bash
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestRelay'
go test -count=10 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRelayConcurrentRunOnce$'
go test -race -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestRelay'
```

기대값: exact result/status/attempt count가 PASS하고 shutdown이 join되며 race가 clean하다.
workshop code에서 relay/store behavior를 재구현하지 않는다.

## Task 4: official Redis Streams publisher 통합

**Complexity:** High. **Depends on:** Tasks 2-3. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** source-backed application gap이 드러나지 않는 한 `integration_test.go`만 포함한다.

- [x] **Step 1: sequential dual-container test 하나 작성**

120-second context 하나 아래에서 released fixture로 PostgreSQL을 먼저 시작한 뒤 Redis를 시작한다.
두 client를 open/ping하고 cleanup을 등록한다. `t.Parallel`은 사용하지 않는다.

```go
postgresURL := postgrestestcontainer.Start(ctx, t)
redisAddr := redistestcontainer.Start(ctx, t)
db, err := sql.Open("pgx", postgresURL)
client := redis.NewClient(&redis.Options{Addr: redisAddr})
if err := db.PingContext(ctx); err != nil { t.Fatal(err) }
if err := client.Ping(ctx).Err(); err != nil { t.Fatal(err) }
```

order 하나를 place하고 `redisstreams.New`만 construct한 뒤 claim limit 1로 `Relay.RunOnce`를 한 번 실행한다.

- [x] **Step 2: documented Redis field 전체 검증**

`XRangeN(ctx, stream, "-", "+", 2)`로 entry를 최대 두 개 읽는다. 정확히 message 하나만 있어야 하며
`record_id`, `status`, `aggregate_type`, `aggregate_id`, `revision`, `event_id`, `idempotency_key`,
`event_type`, `occurred_at`, `recorded_at`, `schema_version`, `attempts`, `entry_json` 값이 정확해야 한다.
`entry_json`은 `audit.DecodeEntryJSON`으로 decode하고 scalar parity를 요구한다. PostgreSQL에서는 `published`와 attempts 1을 query한다.

- [x] **Step 3: integration 및 race를 순차 실행**

```bash
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRedisStreamsIntegration$'
go test -race -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRedisStreamsIntegration$'
```

기대값: command readiness와 함께 두 명령이 모두 PASS한다.
first-fail/retry-pass 결과는 container noise로 넘기지 말고 진단한다.

## Task 5: bounded runnable command 작성

**Complexity:** High. **Depends on:** Task 4. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `main_test.go`, 그다음 `main.go`.

- [x] **Step 1: configuration 및 output test 작성**

unexported `appConfig`, `loadConfig(getenv, now)`, `openDependencies`, `dependencies.Close`,
`execute(ctx, db, redisClient, config, output)`, `run(ctx, config, output)` boundary를 정의한다.
map-backed environment reader를 사용한다. missing endpoint는 variable name만 노출하고, ID는 1..128-rune rule을 따르며,
`REDIS_STREAM`은 provider default를 사용하거나 최대 256 byte의 valid UTF-8이어야 한다. `ORDER_CREATED_AT`은 RFC3339이거나 injected clock을 사용해야 한다.
config error는 값을 echo하지 않고 JSON은 newline-terminated여야 한다. fixture-backed sequential `TestDependenciesClose` 하나로
두 client가 close 전에 ready이고 owned close 뒤 Ping을 거부하는지 증명한다.

```go
type runOutput struct {
    Order          orderoutbox.Order     `json:"order"`
    Relay          sqloutbox.RelayResult `json:"relay"`
    Stream         string                `json:"stream"`
    EventID        string                `json:"event_id"`
    IdempotencyKey string                `json:"idempotency_key"`
}
```

- [x] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/transactional-outbox-publisher -run 'Test(LoadConfig|Execute|Run)'`

기대값: main-package runtime boundary가 없으므로 FAIL한다.

- [x] **Step 3: client ownership, readiness, batch execution 구현**

`run`은 pgx 및 go-redis client를 열고 즉시 close를 defer하며, 두 Ping 모두에 bounded child context를 사용하고,
close error를 return error와 join한다. `main`은 `signal.NotifyContext`를 사용하고 stage-wrapped error 하나를 출력한 뒤,
endpoint 값을 보간하지 않고 nonzero로 종료한다.

`dependencies.Close`는 두 close function을 정확히 한 번씩 호출하고 `errors.Join(postgresErr, redisErr)`를 반환한다.
`run`은 성공적으로 construct한 직후 defer를 설치한다. partial open failure는 return 전에 이미 만든 client를 close한다.

`execute`는 example-table store와 service를 construct하고 schema를 만든 뒤 order를 place하고,
`redisstreams.New`를 construct한 다음 claim을 한 번 실행한다. success는 다음 값일 때만 accept한다.

```go
result == (sqloutbox.RelayResult{Claimed: 1, Published: 1})
```

`XRevRangeN`으로 candidate 하나를 읽고 `event_id`와 `idempotency_key`가 command ID와 같은지 요구한 뒤 JSON을 render한다.
workshop code에서 `XAdd`는 절대 호출하지 않는다.

- [x] **Step 4: GREEN 및 focused race 실행**

```bash
gofmt -w examples/transactional-outbox-publisher/*.go
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
```

기대값: PASS한다. final validation pass는 disposable PostgreSQL 및 Redis endpoint를 대상으로 documented command도 실행하고 JSON exit 0을 요구한다.

## Task 6: architecture diagram 생성 및 시각 검증

**Complexity:** High. **Depends on:** Tasks 2-5. **Pattern skill:** `common.md`와 `architecture.md`를 포함한 `bluetape-diagram`. **Write scope:** architecture SVG/PNG만 포함한다.

- [x] **Step 1: source 및 full-size reference 열기**

best-practice reference
`/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/external-redis-fast-architecture.png`
와 repo-local reference
`docs/images/readme-diagrams/sql-transaction-boundary-architecture.png`.
두 path를 모두 기록한다. reader question은 transaction, client, relay, durable row, Redis transport를 누가 소유하는지다.

- [x] **Step 2: static ownership asset 그리기**

Architects Daughter/Comic Mono, catalog database/Redis icon, 분리된 application/PostgreSQL/Redis region으로
`transactional-outbox-publisher-architecture.svg`를 만든다. 다음을 표시한다.

```text
Command -> Service -> one SQL transaction -> Order row + Outbox row
Application -> Relay -> Outbox claim/update
Relay -> official redisstreams Publisher -> Redis Stream
Application -> caller-owned PostgreSQL/Redis clients and shutdown
```

straight horizontal link를 우선한다. rounded orthogonal bend는 separate port,
최소 `max(8px, rx/2)` corner clearance, 14x14 primary arrowhead를 수용할 terminal segment를 사용한다.
connector가 card에 붙거나 card 안으로 들어가면 안 된다.

- [x] **Step 3: parse, render, audit 후 full size로 inspect**

```bash
xmllint --noout docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
cairosvg docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg -o docs/images/readme-diagrams/transactional-outbox-publisher-architecture.png -s 2
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
```

기대값: meaningful nonzero connector/card/path count가 있고 diagonal, endpoint, intrusion, crossing, mixed-corner failure가 0이다.
final coordinate change 뒤 PNG를 full size로 열어 arrow direction, size/color, bend clearance, label, icon, margin, bottom whitespace를 inspect한다.
PNG evidence가 script보다 우선한다.

## Task 7: retry sequence diagram 생성 및 시각 검증

**Complexity:** High. **Depends on:** Task 6 및 final relay behavior. **Pattern skill:** `common.md`와 `sequence.md`를 포함한 `bluetape-diagram`. **Write scope:** sequence SVG/PNG만 포함한다.

- [x] **Step 1: 두 sequence reference를 full size로 열기**

best-practice reference
`/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-core-sequence-03.png`
와 repo-local reference
`docs/images/readme-diagrams/sql-transaction-boundary-sequence.png`.
reader question은 SQL commit이 atomic하게 유지되는 동안 stable event 하나가 어떻게 두 번 attempt되는지다.

- [x] **Step 2: chronological asset 그리기**

participant는 Application, Service, PostgreSQL, Relay, Redis Publisher, Redis Stream이다.
lifeline, activation bar, visible numbered pill, transparent frame을 추가해 order/outbox commit, claim attempt 1,
ambiguous/failing publish와 retry scheduling, unchanged identity를 가진 attempt 2, append와 published completion,
retry/dead-letter 없는 alternate caller cancellation을 보여준다. explicit per-color 16x16 arrowhead와 continuous message line을 사용한다.

- [x] **Step 3: common 및 sequence-specific proof 실행**

sequence asset에 Task 6의 common command 전체를 실행하고 다음 명령을 추가한다.

```bash
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-sequence.svg
```

기대값: common failure가 0이고 ordered label, participant/lifeline/activation count, transparent frame,
marker color parity가 visible해야 한다. 마지막 coordinate change 뒤 final PNG를 full size로 열어 모든 arrowhead direction,
bend clearance, label/line overlap, frame padding, cancellation branch를 inspect한다.
SVG-only 또는 contact-sheet evidence는 invalid하다.

## Task 8: Write bilingual READMEs and root navigation

**Complexity:** Medium. **Depends on:** Tasks 5-7. **Pattern skills:** `bluetape-writer`, `bluetape-diagram`. **Write scope:** example and root README locale pairs.

- [x] **Step 1: Write the English README from verified behavior**

Include language switch, both diagrams, pinned PostgreSQL/Redis container
prerequisites matching v0.18.0 fixtures, readiness, environment variables,
exact run command, expected
JSON and all Redis fields, focused/race commands, rerun with new identities,
and production boundaries.

State that SQL atomicity ends at commit; Redis is asynchronous; delivery is
at-least-once; consumers deduplicate with stable identity; replay can duplicate;
poison-message replay, retention, trimming, consumer groups, authorization,
TLS, and exactly-once are not implemented.

Use these exact disposable images and commands:

```bash
docker run --rm -d --name workshop-outbox-postgres -e POSTGRES_DB=bluetape -e POSTGRES_USER=bluetape -e POSTGRES_PASSWORD=bluetape -p 5432:5432 postgres:16-alpine
docker run --rm -d --name workshop-outbox-redis -p 6379:6379 redis:7.4-alpine
```

- [x] **Step 2: Produce natural Korean source parity**

Use `bluetape-writer`. Preserve every command, field, warning, diagram embed,
and ownership boundary. Share the English-label assets and add the required
language switch.

- [x] **Step 3: Add root navigation/run parity**

Add an audit/outbox-adjacent row in both root tables with packages
`audit/sqloutbox`, `audit/sqloutbox/redisstreams`, `testcontainers/postgres`,
and `testcontainers/redis`. Add matched English/Korean run/test sections.

- [x] **Step 4: Verify docs and diagram exposure**

```bash
rg -n 'transactional-outbox-publisher|at-least-once|event_id|idempotency_key|go run ./examples/transactional-outbox-publisher' README.md README.ko.md examples/transactional-outbox-publisher/README*.md
rg -n 'transactional-outbox-publisher-(architecture|sequence)\.png' examples/transactional-outbox-publisher/README*.md
test -f docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
test -f docs/images/readme-diagrams/transactional-outbox-publisher-sequence.svg
```

Expected: both locales expose both diagrams and equivalent commands, fields,
delivery caveats, and navigation.

## Task 9: Verify spec/plan, review, and capture lessons

**Complexity:** High. **Depends on:** Tasks 1-8. **Pattern skills:** `verification-before-completion`, `bluetape-go-patterns`, `bluetape-diagram`. **Write scope:** repairs, review artifact, lesson.

- [x] **Step 1: Run fresh proof in order**

```bash
gofmt -w examples/transactional-outbox-publisher/*.go examples/transactional-outbox-publisher/internal/orderoutbox/*.go
git diff --check
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Expected: every command exits 0 with fresh observed output. Lost handles,
truncated output without exit status, and retry-only PASS are not evidence.

- [x] **Step 2: Complete performance/stability and Type A verification**

Inspect unbounded Redis reads, retry/poll loops, DB round trips, context/timer/
goroutine ownership, client closure, readiness, provider error exposure, and
container startup. Map the exact spec and plan to the final diff, tests, locale
pair, navigation, and both PNGs. Repair all P0/P1 and rerun affected proof.

- [x] **Step 3: Converge six review perspectives**

Review performance, stability, security, Ops, developer/API, and user/caller,
then integrate in the main session. Write
`docs/review/2026-07-14-issue-57-transactional-outbox-publisher.md` only at
P0=0/P1=0. Diagram rows record commands, nonzero counts, PNG dimensions,
reference paths, and full-size inspection notes rather than “passed”.

- [x] **Step 4: Commit the durable lesson before PR**

Create `docs/lessons/2026-07-14-issue-57-transactional-outbox-publisher.md`
covering transaction ownership, deterministic retry clock, ambiguous duplicate
delivery, claimed-record cancellation, container proof, arrowhead/bend review
misses, commands, and future guards. Commit only issue #57 files; branch clean.

## Task 10: Create PR, reach green CI, and stop for merge approval

**Complexity:** Medium. **Depends on:** Task 9. **Pattern skills:** `bluetape-workflow`, `finishing-a-development-branch`. **Write scope:** GitHub PR metadata/body only.

- [ ] **Step 1: Push and create the PR**

Push `feat/issue-57-transactional-outbox-publisher`. Create an English PR with
`Closes #57`, assignee `debop`, milestone `0.9.0`, labels
`enhancement,examples`, why/what before validation, diagram evidence, and final
H2 `## DoD Status`.

- [ ] **Step 2: Verify live PR and post-PR review**

Re-read head/base SHA, assignee, milestone, labels, final H2, reviews, comments,
and unresolved threads. Rerun six perspectives against the actual PR diff and
refresh DoD after repairs.

- [ ] **Step 3: Wait for required CI success**

Use the repo CI helper or `gh pr checks --watch`. Required checks must reach
`SUCCESS`; pending, missing, stale, unexplained skipped, or lost output blocks.

- [ ] **Step 4: Stop before merge**

Report PR URL/head SHA, CI conclusions, P0/P1 and review/thread counts, worktree
state, and residual at-least-once risk. Request explicit approval. Do not merge
or delete branch/worktree state.

## Task 11: Merge, sync, and clean only after new explicit approval

**Complexity:** Low. **Depends on:** Task 10 plus explicit merge approval. **Write scope:** GitHub merge and owned git/worktree state.

- [ ] **Step 1: Re-read the hold point**

Verify unchanged head SHA, successful required CI, clean mergeability, no newer
review/thread blocker, and clean main/feature worktrees.

- [ ] **Step 2: Rebase merge with head protection**

Use `gh pr merge --rebase --match-head-commit <verified-sha>`, verify live
`MERGED`, and record the resulting commit.

- [ ] **Step 3: Synchronize and clean owned state**

Fast-forward local `develop`, verify it equals `origin/develop`, remove/prune
the issue worktree, and delete the merged local/remote feature branch. Never
delete dirty or unmerged state.

- [ ] **Step 4: Update roadmap completion metadata**

Verify #57 closed. Update issue #35 and epic #27 only where their live
checklists require #57 completion, leaving #68 open as the next audit/outbox
work. Verify main checkout clean.

## Risk Prediction

| Risk | Signal | Mitigation and rerun point |
|---|---|---|
| Order/outbox atomicity breaks | table counts diverge | Same `*sql.Tx`; rerun outbox-conflict rollback before relay work. |
| Retry relies on sleeps | timing drift or incorrect claim count | Shared injected clock, pre-advance zero claim, exact 250 ms advance; rerun Task 3/race. |
| Cancellation mutates retry state or leaks | claimed status changes or join timeout | Context-returning `PublisherFunc`, channel join; rerun cancellation/race. |
| Duplicate delivery is called exactly-once | docs/tests expect one physical message | Stable logical identity plus explicit duplicate warning; rerun caller/security review. |
| CLI reports stale pending event | stream event ID differs from command | Claim one, compare identity, fail closed, use disposable state/new IDs. |
| Redis read/batch is unbounded | `XRange` or default claim size | `XRangeN`/`XRevRangeN`, claim limit 1, performance review. |
| Containers are flaky/leaking | first fail then retry pass, readiness/close gap | Serialize, real Ping, bounded context/cleanup; diagnose and rerun Task 4. |
| Endpoint/payload leaks | config value or `entry_json` in output | No endpoint interpolation, bounded output; rerun security tests/review. |
| Rendered arrowheads/bends are wrong | backward/crowded head or sharp PNG corner | One-asset loop, fixed sizes, move bends, rerender and full-size inspect. |
| README locales drift | missing field/command/embed | Parity search and writer/diagram exposure rerun. |

## Acceptance Traceability

| Spec requirement | Plan task | Proof |
|---|---|---|
| Service/config/schema contract | 1 | constructor, validation, schema tests |
| Atomic order/outbox commit/rollback | 2 | real PostgreSQL counts and identity |
| Success, retry, duplicate, stable identity, dead letter | 3 | deterministic attempts/status |
| Cancellation and joined shutdown | 3 | `PublisherFunc`, channel join, race |
| Official Redis adapter and all fields | 4 | dual-container integration, decoded entry |
| App-owned clients/timeouts/output/closure | 5 | main tests and configured execution |
| Architecture geometry/ownership | 6 | SVG/PNG audits and full-size ledger |
| Retry/cancellation sequence | 7 | sequence audit and full-size ledger |
| Bilingual delivery/replay docs/navigation | 8 | locale parity/exposure commands |
| Focused/race/static/repository quality | 9 | fresh commands through `make ci` |
| Review convergence and lessons | 9 | review artifact and lesson commit |
| PR metadata and green CI | 10 | live GitHub evidence, pre-merge stop |
| Merge/sync/cleanup | 11 | only after explicit approval |

## Repository Hazard Decisions

| Hazard | Decision and evidence |
|---|---|
| Dependency/version | N/A: all packages exist in current `go.mod`; `tidy-check` must stay clean. |
| Module/BOM/catalog | N/A: one package tree in the existing Go module. |
| Workflow/CI/Nightly/coverage | N/A unless existing fixture execution proves insufficient; workflow edits require reapproval. |
| Public API/KDoc/changelog | N/A: workshop `internal` and main packages only. |
| Production migration | N/A: example-prefixed idempotent DDL; partial startup DDL recovers on rerun. |
| HTTP/auth | N/A: no listener or request input. |
| Benchmark | N/A: no throughput claim; production batch tuning deferred. |
| Diagram | Triggered: architecture and sequence checklists mandatory per asset. |
| Testcontainers | Triggered: PostgreSQL/Redis serialized with readiness/cleanup proof. |

## Plan Review Record

The active subagent interface still lacks the required `agent_type` field, so
the main session performed six isolated plan reviews and integrated them against
the approved spec, Step 3-R checklist, v0.18.0 source/docs, Makefile, issue #57,
and repository rules.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0 after repair | Added exact 12-record/four-worker stress with 10 repetitions, bounded claim/Redis reads, shared-container subtests, and no throughput claim. |
| Stability | P0=0, P1=0 after repair | Added shared-clock retry eligibility, cancellation status assertions, channel-joined `Run`, partial-open cleanup, owned client close proof, sequential readiness, and failure rerun points. |
| Security | P0=0, P1=0 | Tasks bound identifiers/author/stream/time input, use parameterized SQL, avoid endpoint interpolation, keep fixed payloads, and document sensitive diagnostic stderr. |
| Operator/Ops | P0=0, P1=0 | Pinned disposable services, readiness, partial-DDL recovery, dead-letter/replay limits, exact success/nonzero failure, rollback, and pre-merge hold are assigned. |
| Developer/API | P0=0, P1=0 | Every spec API/error/ownership contract maps to an ordered RED/GREEN task; workshop code reuses released store, relay, publisher, and fixtures without new abstractions or dependencies. |
| User/caller | P0=0, P1=0 after repair | Added exact container images, run/output/field evidence, rerun identities, bilingual parity, diagram exposure, and exactly-once/poison/replay warnings. |
| Main integration | P0=0, P1=0 | All acceptance rows map to earlier artifacts and concrete commands; no task depends on later code, #68 scope remains excluded, and merge remains explicitly gated. |
