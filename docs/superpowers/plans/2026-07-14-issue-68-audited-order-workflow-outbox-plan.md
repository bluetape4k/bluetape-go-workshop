# Issue #68 Audited Order Workflow with SQL Outbox Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** order state, immutable PostgreSQL audit history, official SQL outbox record를 atomic하게 commit하는 runnable Gin order workflow를 만든다. POST JSON audit query와 asynchronous Redis Streams delivery를 함께 제공한다.

**Architecture:** `orderworkflow.Service`는 validation과 order row, application-owned `HistoryStore`, released `sqloutbox.Store`를 감싸는 `sqlkit.WithTx` boundary 하나를 소유한다. Gin은 strict POST JSON command와 history query를 노출한다. supervised background `sqloutbox.Relay`는 released Redis Streams adapter로 publish하고, readiness는 durable PostgreSQL boundary를 보존한다. `main`은 bounded client, server lifecycle, delivery diagnostic, shutdown을 소유한다.

**Tech Stack:** Go 1.26.3, Gin, pgx v5를 사용하는 `database/sql`, bluetape-go v0.18.0 `audit`, `audit/sqloutbox`, `audit/sqloutbox/sqloutboxtest`, `audit/sqloutbox/redisstreams`, `sqlkit`, go-redis v9, repository PostgreSQL/Redis Testcontainers fixture, SVG/CairoSVG diagram tooling.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/audited-order-workflow-outbox/internal/orderworkflow/model.go` | status value, command, canonical intent, response value, validation, audit entry construction |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/schema.go` | fixed order/history DDL 및 official outbox schema delegation |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/history_store.go` | transactional history insert, command lookup, full `audit.HistoryReader`, safe delivery status query |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/history_store_test.go` | reader semantic, precision/parity, corruption, bounded plan, status diagnostic |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/service.go` | create/transition state machine, row locking, atomic write, idempotent conflict recovery |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/service_test.go` | validation, state transition, rollback, replay, ambiguous commit, concurrency proof |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/handler.go` | strict Gin JSON adapter, error mapping, health/readiness/status, concurrency cap |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/handler_test.go` | POST JSON, cursor, overload, redaction, dependency isolation, timeout proof |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/runtime.go` | bounded client/server configuration, relay observation, readiness state, coordinated shutdown |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/relay_test.go` | retry/dead-letter/duplicate/order-limit, safe signal, cancellation, unexpected-exit proof |
| `examples/audited-order-workflow-outbox/internal/orderworkflow/integration_test.go` | sequential PostgreSQL/Redis, real stream envelope, restart, backlog, pool-limit proof |
| `examples/audited-order-workflow-outbox/main.go` | environment parsing, dependency construction, schema bootstrap, signal lifecycle, server start |
| `examples/audited-order-workflow-outbox/main_test.go` | loopback/configuration, startup failure, server smoke, close/join proof |
| `examples/audited-order-workflow-outbox/smoke_test.go` | checked-in HTTP scenario를 parse하고 sequential real backend에 실행 |
| `examples/audited-order-workflow-outbox/requests.http` | complete create/replay/search/detail/cancel/409 POST JSON scenario |
| `examples/audited-order-workflow-outbox/README.md`, `README.ko.md` | bilingual lesson, curl scenario, delivery semantic, operator runbook |
| `examples/transactional-outbox-publisher/README.md`, `README.ko.md` | current-locale language-switch rendering 교정 |
| `README.md`, `README.ko.md` | root example navigation 및 run command |
| `docs/images/readme-diagrams/audited-order-workflow-outbox-architecture.{svg,png}` | static transaction, history, relay, transport ownership |
| `docs/images/readme-diagrams/audited-order-workflow-outbox-sequence.{svg,png}` | commit-before-response, later delivery, replay, query sequence |
| `docs/review/2026-07-14-issue-68-audited-order-workflow-outbox.md` | final spec/plan/checklist 및 review evidence |
| `docs/lessons/2026-07-14-issue-68-audited-order-workflow-outbox.md` | durable implementation, Docker, relay, diagram lesson |

`go.mod`, `go.sum`, workflow, dependency, public bluetape-go API, changelog,
module registration change는 계획하지 않는다. 이런 diff가 생기면 scope review를 위해 implementation을 중단한다.

## Task 1: domain, validation, audit projection 정의

**Complexity:** Medium. **Depends on:** approved design spec. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `model.go`, `service_test.go`.

- [ ] **Step 1: 모든 domain constraint에 대한 failing table test 작성**

identifier grammar, action, state transition, reason length, metadata key/value/count bound,
nil clock, UTC microsecond normalization, revision overflow, canonical intent equality,
payload round-trip test를 만든다.

```go
func TestNormalizeIdentifier(t *testing.T) {
    tests := []struct{ value string; want string; wantErr bool }{
        {value: " order-1001 ", want: "order-1001"},
        {value: "order:west.1", want: "order:west.1"},
        {value: "order\nforged", wantErr: true},
        {value: "한글", wantErr: true},
        {value: strings.Repeat("a", 129), wantErr: true},
    }
    for _, tt := range tests {
        got, err := normalizeIdentifier(tt.value)
        if (err != nil) != tt.wantErr || got != tt.want {
            t.Fatalf("normalizeIdentifier(%q) = (%q, %v)", tt.value, got, err)
        }
    }
}
```

- [ ] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'Test(Normalize|Validate|BuildEntry|CanonicalIntent)'`

기대값: package와 domain function이 없으므로 FAIL한다.

- [ ] **Step 3: minimal domain value 및 validation 구현**

```go
type Status string
const (
    StatusPending Status = "pending"
    StatusConfirmed Status = "confirmed"
    StatusCancelled Status = "cancelled"
)
type Action string
const (
    ActionConfirm Action = "confirm"
    ActionCancel Action = "cancel"
)
type Order struct {
    OrderID string `json:"order_id"`
    Status Status `json:"status"`
    Revision audit.Revision `json:"revision"`
    UpdatedAt time.Time `json:"updated_at"`
}
type CreateCommand struct { OrderID, CommandID string; Metadata audit.Metadata }
type TransitionCommand struct {
    OrderID, CommandID string
    Action Action
    Reason string
    Metadata audit.Metadata
}
```

`regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)`를 사용한다.
metadata entry는 최대 32개, key는 64 rune, value는 512 rune으로 validate한다.
injected clock value 하나는 `UTC().Truncate(time.Microsecond)`로 normalize한다.

- [ ] **Step 4: accepted command마다 canonical audit entry 하나 생성**

`audit.NewAggregateID`, `audit.NewDomainEvent`, `audit.NewEntry`를 호출한다.
event 및 idempotency identity에는 command ID를 사용하고, resulting revision,
event type `order.created|confirmed|cancelled`, copied metadata,
`intent`와 original resulting `order` projection을 담은 JSON payload를 사용한다.
replay comparison에는 같은 payload를 decode해서 사용한다. raw request JSON은 절대 compare하지 않는다.

- [ ] **Step 5: GREEN, format, commit 실행**

```bash
gofmt -w examples/audited-order-workflow-outbox/internal/orderworkflow/*.go
go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'Test(Normalize|Validate|BuildEntry|CanonicalIntent)'
git diff --check
git add examples/audited-order-workflow-outbox/internal/orderworkflow
git commit -m "feat: define audited order workflow domain"
```

기대값: focused test가 PASS하고 timestamp에 sub-microsecond remainder가 없으며,
commit에는 domain/test file만 포함된다.

## Task 2: durable SQL history store 구현

**Complexity:** High. **Depends on:** Task 1. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `schema.go`, `history_store.go`, `history_store_test.go`.

- [ ] **Step 1: PostgreSQL fixture 하나를 대상으로 reader-contract 및 schema test 작성**

90-second context 아래에서 `postgrestestcontainer.Start`를 한 번만 시작하고 `t.Parallel`은 절대 사용하지 않는다.
idempotent DDL, `Insert`, command lookup, zero/all 및 filtered `audit.Query`, `NewestFirst`,
inclusive bound, limit, `LoadHistory`, `Latest`, 두 snapshot method, absent value, cancellation,
nil session, 1 MiB encoding rejection을 test한다. 일부러 incompatible pre-existing orders/history schema를 만들고,
alter/drop 없이 bootstrap이 fail해야 한다. official outbox table에도 released v0.18.0 table/index contract를 authority로 삼아
같은 incompatibility case를 반복한다. 모든 mismatch는 startup에서 redacted/non-destructive 방식으로 fail해야 한다.
일부러 interrupted partial bootstrap을 retry하고 convergence를 요구한다.

```go
var _ audit.HistoryReader = (*HistoryStore)(nil)

entries, err := store.Find(ctx, audit.Query{
    Aggregate: &aggregate, FromRevision: 1, ToRevision: 2, Limit: 2,
})
if err != nil || len(entries) != 2 || entries[0].Revision != 1 {
    t.Fatalf("Find() = (%v, %v)", entries, err)
}
```

- [ ] **Step 2: RED 실행**

실행: `go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestHistoryStore'`

기대값: `HistoryStore`와 schema function이 없으므로 FAIL한다.

- [ ] **Step 3: fixed idempotent DDL 및 transactional insert 추가**

`audited_order_workflow_orders`, identity-position history table, aggregate/time index를 만든다.
`Table: "audited_order_workflow_outbox_records"`로 configured official store에 delegate한다.
`Insert`는 `sqlkit.Execer`를 받고 entry를 validate하며 `json.Marshal`을 수행하고 1 MiB를 enforce한 뒤,
transaction을 소유하지 않고 scalar guard와 `entry_json`을 insert한다.

idempotent creation 뒤 required column, PostgreSQL type, nullability, primary/unique constraint,
aggregate/time index를 `pg_catalog`로 verify한다. 이는 migration engine이 아니라 fixed single-version compatibility check다.
unexpected shape는 redacted startup error를 반환하고 `ALTER` 또는 `DROP`을 절대 실행하지 않는다.
`Store.CreateSchema` 뒤 같은 read-only catalog check로 official outbox table의 required column, type,
unique identity, primary key, claim index를 verify한다.

```go
func (s *HistoryStore) Insert(ctx context.Context, db sqlkit.Execer, entry audit.Entry) error {
    if s == nil || db == nil { return ErrInvalidConfig }
    if err := entry.Validate(); err != nil { return fmt.Errorf("validate entry: %w", err) }
    encoded, err := json.Marshal(entry)
    if err != nil || len(encoded) > maxEntryBytes { return ErrInvalidEntry }
    _, err = db.ExecContext(ctx, insertHistorySQL,
        entry.Aggregate.Type, entry.Aggregate.ID, entry.Revision,
        entry.Event.EventID, entry.Event.IdempotencyKey,
        entry.Event.EventType, entry.Event.RecordedAt, encoded)
    return err
}
```

- [ ] **Step 4: exact v0.18.0 reader surface 구현**

row 및 row query에는 내부적으로 `sqlkit.Session`을 사용한다. `Find`는 `audit.Query.Validate`로 시작하고,
placeholder만 사용하며, global query는 `position` 기준으로 정렬하고 exact-aggregate query는 revision order와 일관되게 정렬한다.
decode는 `audit.DecodeEntryJSON`으로 수행한다. 반환 전에 microsecond timestamp equality를 포함한 모든 scalar field를 verify한다.
history는 `audit.NewHistory`로 construct한다. missing latest/snapshot에는 `(zero, false, nil)`을 반환한다.

- [ ] **Step 5: corruption, plan, status diagnostic proof 추가**

test SQL로 controlled scalar/JSON mismatch row를 insert하고 closed error를 요구한다.
exact hot aggregate에 최소 5,000 entry, 다른 aggregate에 distractor entry 5,000개를 commit한다.
seeding 뒤 `ANALYZE`를 실행한 다음 canonical revision 및 time search에 대해 `EXPLAIN (FORMAT JSON)`을 실행한다.
plan에 intended primary-key 또는 aggregate/time index와 bounded `Limit`가 있고 sequential scan 또는 unbounded sort가 없는지 assert한다.
pending/retrying/claimed/published/dead-letter count와 oldest-pending seconds만 반환하는 fixed 250 ms outbox status query를 추가한다.

- [ ] **Step 6: GREEN 및 commit 실행**

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestHistoryStore'
go test -race -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestHistoryStore(Find|Corruption|Cancellation)'
git diff --check
git add examples/audited-order-workflow-outbox/internal/orderworkflow
git commit -m "feat: add durable audit history store"
```

기대값: reader contract, corruption, query-plan, race assertion이 PASS한다.

## Task 3: create 및 transition state를 atomic하게 commit

**Complexity:** High. **Depends on:** Tasks 1-2. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `service.go`, `service_test.go`.

- [ ] **Step 1: atomic create 및 transition test를 먼저 작성**

existing PostgreSQL fixture 아래에서 create는 pending revision 1을 만들고 confirm은 revision 2를 만들며,
cancel은 pending/confirmed를 accept하고 invalid transition은 conflict가 되며 missing order는 not found인지 증명한다.
accepted command마다 matching order/history/outbox projection이 정확히 하나씩 남아야 한다.

```go
created, replayed, err := service.Create(ctx, CreateCommand{
    OrderID: "order-1001", CommandID: "cmd-create-1001",
    Metadata: audit.Metadata{"channel": "workshop"},
})
if err != nil || replayed || created.Status != StatusPending || created.Revision != 1 {
    t.Fatalf("Create() = (%v, %v, %v)", created, replayed, err)
}
```

- [ ] **Step 2: implementation 전에 rollback 및 race test 추가**

order write 뒤 history uniqueness failure를, history write 뒤 outbox identity failure를 inject한다.
세 table이 모두 unchanged인지 assert한다. 같은 order transition 두 개를 concurrently 실행하고 valid next revision 하나만 요구한다.
same order에 대한 identical create, 다른 order 간 identical/conflicting command ID도 concurrently 실행한다.
same-order loser는 history identity 전에 order primary key를 먼저 맞을 수 있지만, canonical intent가 match하면 winner를 reload하고 replay해야 한다.
첫 call은 real successful `sqlkit.WithTx`에 delegate한 뒤 synthetic deadline error를 반환하는 package-private transaction runner를 설치한다.
첫 service call은 commit 이후 error를 반환해야 하며, normal runner로 retry하면 새 history/outbox row 없이 original projection을 반환해야 한다.
별도 transaction에서 order row lock을 잡고 100 ms test deadline으로 competing transition을 호출한다.
`context.DeadlineExceeded`, unchanged table count, released pool capacity를 요구한 뒤 lock을 release하고 fresh transition 성공을 요구한다.
이는 2-second HTTP operation deadline의 deterministic fast analogue다.

- [ ] **Step 3: RED 실행**

실행: `go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestService(Create|Transition|Rollback|Concurrent|Ambiguous)'`

기대값: service method가 없으므로 FAIL한다.

- [ ] **Step 4: replay recovery가 있는 create 구현**

`sqlkit.WithTx` 전에 validate한다. 내부에서는 command identity를 check하고 order를 insert한 뒤 entry 하나를 build하고
`HistoryStore.Insert(ctx, tx, entry)`, `outbox.Enqueue(ctx, tx, entry)`를 순서대로 호출한다.
event/idempotency unique violation이 발생하면 `WithTx`가 완전히 roll back하게 두고 fresh database operation에서 winner를 load한다.
canonical intent가 match할 때만 original payload projection을 반환하고, 그렇지 않으면 `ErrConflict`를 반환한다.
named order primary-key 및 event/idempotency constraint에 대한 PostgreSQL SQLSTATE `23505`만 `errors.As`로 detect한다.
다른 database failure는 wrapped storage error로 남긴다. service에는 package-private `runTx` function을 두고 default는 정확히 `sqlkit.WithTx`로 둔다.
test는 real successful delegate call 뒤 error를 반환하는 경우에만 이를 replace할 수 있다.

- [ ] **Step 5: locked transition 구현**

`sqlkit.WithTx` 하나 안에서 `SELECT ... FOR UPDATE`를 실행하고, state validation 전에 command identity를 check한다.
`Revision.Next`를 계산하고 prior revision을 guard로 update한 뒤 history를 insert하고 같은 entry를 enqueue한다.
이 file에서는 Redis call이 금지된다. cancellation을 보존하고 error는 stage와 `%w`로 wrap한다.

- [ ] **Step 6: GREEN, race, commit 실행**

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestService'
go test -race -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestService(Concurrent|Ambiguous|Replay)'
git add examples/audited-order-workflow-outbox/internal/orderworkflow
git commit -m "feat: add atomic audited order workflow"
```

기대값: table-count/parity check가 PASS하고 `-race` 아래에서 duplicate logical revision이 commit되지 않는다.

## Task 4: strict POST JSON command 및 audit query 노출

**Complexity:** High. **Depends on:** Task 3. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `handler.go`, `handler_test.go`.

- [ ] **Step 1: strict decoder 및 route test를 먼저 작성**

`gin-audit-query-api`에서 proven token-walking duplicate-key detector를 가져오되 그 `internal` package는 import하지 않는다.
exact media type, optional UTF-8 charset, identity encoding, 32 KiB limit, invalid UTF-8,
duplicate nested key, unknown field, trailing value, array/scalar, empty body, body closure, 404, 405를 cover한다.
gzip, br, deflate, multiple `Content-Encoding` value는 handler를 호출하지 않고 415로 명시적으로 reject한다.
token 및 typed decoder는 `UseNumber`로 configure한다. float conversion이 precision을 silently lose하지 않도록 exact int64 boundary,
overflow, fraction, exponent, negative/zero revision 및 limit value를 test한다.

Add an explicit registration table and assert each method/path pair:

```text
POST /orders
POST /orders/transitions
POST /audit/history/search
POST /audit/history/detail
GET  /healthz
GET  /readyz
GET  /statusz
```

모든 path에 대해 registered method는 stub을 통해 succeed하고 unsupported method는 405이며 unknown path는 404인지 assert한다.

- [ ] **Step 2: command/response/error test 작성**

POST `/orders`가 201 이후 replay에서는 200을 반환하고 transition은 200을 반환하는지 assert한다.
`replayed`와 `delivery: asynchronous`는 exact해야 한다. invalid transition은 409, missing order는 404,
server deadline은 408, over-limit은 413, media error는 415, storage error는 redacted 500이어야 하며 request ID가 있어야 한다.

- [ ] **Step 3: query/cursor 및 overload test 작성**

canonical search/detail body를 POST한다. revision 1과 2, limit 1일 때 page one은 revision 1/cursor 2를,
page two는 revision 2/null을 반환해야 한다. Redis access에서 panic하는 fake reader로 query가 history만 사용하는지 증명한다.
32-slot semaphore를 채우고 다음 request가 429, `Retry-After: 1`, `Connection: close`, closed body,
service call 없음 을 받는지 assert한다. `GET /statusz`는 Redis/relay state, bounded outbox count,
rounded oldest-pending seconds만 반환해야 한다. 250 ms status-query timeout은 degraded safe response가 된다.
identity, payload, metadata, endpoint, provider marker를 seed하고 HTTP body에 어떤 marker도 없어야 한다.
대표 400/404/408/409/413/415, 429, 500 response의 complete error envelope를 decode한다.
request ID, exact stable code, safe message, `data` 없음, state conflict의 `invalid_transition`,
overload의 `too_many_requests`를 요구한다.

- [ ] **Step 4: RED 실행**

실행: `go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestHTTP'`

기대값: `NewEngine`과 transport type이 없으므로 FAIL한다.

- [ ] **Step 5: minimal Gin adapter 구현**

```go
func NewEngine(service CommandService, reader audit.HistoryReader,
    health HealthReader, cfg HTTPConfig, logger *slog.Logger) (*gin.Engine, error) {
    gin.SetMode(gin.ReleaseMode)
    engine := gin.New()
    engine.HandleMethodNotAllowed = true
    engine.RedirectTrailingSlash = false
    engine.UseRawPath = true
    if err := engine.SetTrustedProxies(nil); err != nil { return nil, err }
    // Install recovery, request ID, concurrency cap, and the seven specified routes.
    return engine, nil
}
```

`http.MaxBytesReader`, `json.Decoder.DisallowUnknownFields`, explicit EOF,
duplicate-key walking, operation context, stable success/error envelope, safe structured field만 사용한다.
GET route는 health/readiness/status로 제한하고 모든 user command 및 query는 POST JSON으로 유지한다.

- [ ] **Step 6: GREEN, race, commit 실행**

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestHTTP'
go test -race -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestHTTP(Concurrent|Timeout|Overload)'
git add examples/audited-order-workflow-outbox/internal/orderworkflow
git commit -m "feat: expose audited workflow HTTP API"
```

## Task 5: continuous relay 및 server lifecycle supervise

**Complexity:** High. **Depends on:** Tasks 3-4. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `runtime.go`, `relay_test.go`, `main.go`, `main_test.go`.

- [ ] **Step 1: deterministic relay 및 signal test 작성**

`sqloutboxtest.RecordingPublisher`, mutable clock, `PublisherFunc`로 success,
정확히 250 ms 뒤 retry 한 번, three-attempt dead-letter, stable identity를 가진 duplicate physical attempt,
lease-recoverable claim을 남기는 caller cancellation, earlier record가 pending/claimed인 동안만 later-revision이 block되는지 증명한다.
publish cancellation 뒤 claimed state를 verify하고 shared clock을 official 30-second lease 뒤로 advance한다.
fresh relay를 시작해 같은 event/idempotency identity를 reclaim/publish하고 lifecycle을 join한 뒤 final published state를 요구한다.

- [ ] **Step 2: runtime code 전에 lifecycle test 작성**

expected cancellation이 clean하게 join되는지 증명한다. unexpected `Relay.Run` failure는 readiness false로 mark하고,
redacted lifecycle transition 하나를 log하며 HTTP를 shutdown하고 failure를 반환해야 한다.
Redis outage는 degraded delivery와 함께 readiness HTTP 200을 유지하고, database failure 또는 stopped relay는 503을 반환한다.
idle relay loop가 log를 emit하지 않는지 assert한다. official `Relay.Run`은 의도적으로 per-batch result를 제공하지 않으므로
current delivery count는 `/statusz`가 소유한다. stalled-header 및 stalled-body client를 가진 real listener로
header/read timeout이 connection을 release하는지 증명한다. 짧은 injected test duration으로 oversized header,
write timeout, idle timeout, graceful-shutdown deadline을 증명한다. PostgreSQL 및 Redis pool을 exhaust하고
configured ceiling과 bounded timeout behavior를 요구한다. IPv4/IPv6 loopback literal은 accept하고,
IPv4/IPv6 wildcard, `localhost`, hostname-only, non-loopback, malformed, missing-port, zone-scoped address는 reject하는 table test를 추가한다.
Redis stream default, blank, invalid UTF-8, 256 byte 초과를 test한다. config, open, ping, schema, relay,
HTTP serve, close stage에 recognizable secret marker를 inject한다. stage와 stable class만 담은 centralized safe projection 하나를 요구하고,
log와 top-level returned error 양쪽에서 marker가 없는지 assert한다.

- [ ] **Step 3: RED 실행**

실행: `go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'Test(Relay|Lifecycle|Readiness|Status)'`

기대값: runtime ownership이 없으므로 FAIL한다.

- [ ] **Step 4: bounded resource 및 runtime configuration 구현**

PostgreSQL은 `MaxOpenConns(8)`, `MaxIdleConns(8)`, 5-minute idle, 30-minute lifetime으로 설정한다.
Redis는 pool size 8, minimum idle 1, pool timeout 2 seconds로 configure한다.
`http.Server`는 2-second header, 5-second read/write, 30-second idle, 16 KiB header,
5-second shutdown limit로 만든다. Relay option은 claim 16, attempts 3, retry 250 ms, idle 50 ms다.

- [ ] **Step 5: supervision 및 safe observation 구현**

lifecycle function 하나가 relay context와 result channel을 소유한다. early `context.Canceled`는 unexpected로 처리하고,
server shutdown 전에 readiness false를 mark하며 independent close error를 join한다.
official `Relay.Run`을 직접 호출하고 injected observer를 통해 delivery-degradation 및 lifecycle transition만 report한다.
record, endpoint, provider error, metadata는 절대 log하지 않는다.

- [ ] **Step 6: loopback-only configuration 및 `main` 구현**

`DATABASE_URL`과 `REDIS_ADDR`는 required다. optional `REDIS_STREAM`과 IP-literal `HTTP_ADDR`를 accept한다.
DNS resolution 없이 `net.SplitHostPort`, 그다음 `net.ParseIP`로 parse하고 nil IP, wildcard, zone,
`!ip.IsLoopback()`은 reject한다. public error는 `safeStageError(stage, class)`에 centralize한다.
wrapped provider/configuration value를 `main`으로 반환하지 않는다. startup order는 parse, open, bounded ping,
schema bootstrap, publisher/relay construction, listener, relay, HTTP다.
shutdown order는 HTTP drain, relay cancel/join, Redis close, database close다.

- [ ] **Step 7: GREEN, race, commit 실행**

```bash
gofmt -w $(rg --files examples/audited-order-workflow-outbox -g '*.go')
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/... -run 'Test(Relay|Lifecycle|HTTP)'
go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestRelayLeaseRecovery'
git add examples/audited-order-workflow-outbox
git commit -m "feat: run supervised audit outbox relay"
```

## Task 6: PostgreSQL 및 Redis integration을 순차 증명

**Complexity:** High. **Depends on:** Task 5. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `integration_test.go` and test helpers only.

- [ ] **Step 1: sequential end-to-end fixture 하나 추가**

PostgreSQL을 먼저 시작하고 schema/command/history/restart assertion을 완료한 뒤 Redis를 시작한다.
`t.Parallel`은 사용하지 않는다. created 및 confirmed event를 `redisstreams.New`로 publish하고
documented 13 field, exact event 및 idempotency identity, valid `entry_json`,
duplicate physical stream entry에 대한 tolerance를 요구한다.

- [ ] **Step 2: restart, outage, lease, backlog case 추가**

같은 database 위에서 service/store/runtime object를 recreate하고 state, history, replay, pending row가 persist하는지 증명한다.
stopped/unavailable Redis는 command를 roll back하면 안 된다. client를 recover하고 writer가 계속되는 동안 multiple 16-record batch를 drain한다.
count/deadline, `db.Stats().MaxOpenConnections == 8`, later record가 earlier pending/claimed record를 bypass하지 않음을 assert한다.
`db.Stats()`를 10 ms마다 sample한다. peak `InUse`는 8을 넘으면 안 되고, 모든 writer는 2-second operation deadline 안에 finish해야 하며,
backlog는 fixed test deadline 안에 drain되어야 한다. relay monopolization이 조용히 pass하지 못하도록 total `WaitDuration`은
`2 seconds * writerCount` 아래여야 한다.

- [ ] **Step 3: sequential package proof 실행**

```bash
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox/... -run 'TestIntegration'
go test -race -count=1 -p 1 ./examples/audited-order-workflow-outbox/... -run 'TestIntegration(Relay|Concurrent)'
```

기대값: PostgreSQL 이후 Redis case가 fresh exit 0으로 PASS하고 parallel container startup이 없다.

- [ ] **Step 4: integration proof commit**

```bash
git add examples/audited-order-workflow-outbox/internal/orderworkflow/integration_test.go
git commit -m "test: prove audited workflow delivery integration"
```

## Task 7: runnable POST JSON documentation 추가

**Complexity:** Medium. **Depends on:** Tasks 4-6. **Pattern skills:** `bluetape-writer`, `bluetape-go-patterns`. **Write scope:** example README pair, `requests.http`, root README pair, issue #57 README pair.

- [ ] **Step 1: `requests.http`를 executable source contract로 작성**

create, confirm, identical replay, two-page search, detail revision 2, reason이 있는 second-order cancel,
rejected post-cancel confirm에 대한 variable 및 complete JSON을 포함한다.
모든 request는 method, URL, content type, body를 포함한다.

```http
### Create order
POST {{baseURL}}/orders
Content-Type: application/json

{"order_id":"order-1001","command_id":"cmd-create-1001","metadata":{"channel":"workshop"}}
```

- [ ] **Step 2: source-equivalent English 및 Korean README 작성**

`English | [한국어](README.ko.md)`와 `[English](README.md) | 한국어`를 사용한다.
architecture, transaction boundary, history versus transport, loopback-only startup, at-least-once 및 reordering,
status/readiness, exact curl body, expected replay/cursor/409/429 result, shutdown을 설명한다.
abbreviated JSON이 아니라 copy-paste 가능한 curl command를 요구한다.

- [ ] **Step 3: operator runbook 및 schema boundary 추가**

safe status inspection, Redis outage/recovery, lease wait, dead-letter diagnosis,
partial-startup restart, explicitly destructive local reset을 document한다.
startup DDL은 migration 또는 production rollback tooling이 아니라 single-version workshop bootstrap이라고 명시한다.

- [ ] **Step 4: navigation 업데이트 및 issue #57 language switch 수정**

두 root README table/run section에 새 example을 추가한다.
issue #57 README pair에서는 current locale이 plain text가 되도록 language switch line만 바꾼다.
다른 issue #57 content는 모두 보존한다.

- [ ] **Step 5: locale 및 request parity 검증 후 commit**

```bash
rg -n '^POST |Content-Type: application/json|replayed|next_from_revision|too_many_requests|invalid_transition' examples/audited-order-workflow-outbox/{requests.http,README.md,README.ko.md}
go test -count=1 ./examples/audited-order-workflow-outbox -run 'TestDocumentationParity'
git diff --check
git add README.md README.ko.md examples/audited-order-workflow-outbox examples/transactional-outbox-publisher/README.md examples/transactional-outbox-publisher/README.ko.md
git commit -m "docs: add audited workflow POST JSON guide"
```

기대값: 모든 scenario marker가 세 artifact 모두에 있고 locale switch에 current-locale self-link가 없다.
`TestDocumentationParity`는 English README, Korean README, `requests.http`에서 normalized scenario name,
method, URL, JSON body, expected status, replay/cursor expectation, warning, runbook step,
unsupported behavior를 extract한다. marker presence만이 아니라 structural equality를 요구한다.

- [ ] **Step 6: checked-in HTTP scenario를 real backend에 실행**

repository fixture로 PostgreSQL을 먼저 시작한 뒤 Redis를 시작하고, loopback application을 시작하며,
`/readyz`를 기다리고 `requests.http`를 parse해 `{{baseURL}}`을 substitute한 다음 checked-in request를 각각 보내는 `smoke_test.go`를 만든다.
scenario annotation은 expected status 및 response assertion을 정의한다. 201 create, 200 confirm,
`replayed: true`를 가진 200 replay, page cursor 2 이후 null, detail revision 2, cancellation,
cancellation 이후 stable 409를 요구한다. HTTP와 container는 항상 bounded cleanup으로 stop한다.

Run:
`go test -count=1 -p 1 ./examples/audited-order-workflow-outbox -run 'TestRequestsHTTPSmoke'`

기대값: actual checked-in request body를 사용해 fresh exit 0으로 PASS한다.

## Task 8: architecture 및 sequence diagram 생성/시각 검증

**Complexity:** High. **Depends on:** Tasks 5-7. **Pattern skills:** `bluetape-diagram`. **Write scope:** canonical image asset 네 개와 README image reference.

- [ ] **Step 1: diagram checklist 및 best-practice reference load**

`bluetape-diagram`을 완전히 따른다. repository visual grammar를 재사용하고 generated SVG를 직접 그린다.
Mermaid는 final artifact가 아니다. Architecture는 route, service, transaction 하나, PostgreSQL table 세 개,
supervised relay, Redis, history-query bypass를 보여줘야 한다.
Sequence는 validate/lock, write 세 번, commit, response, later relay, replay, query를 보여줘야 한다.

- [ ] **Step 2: SVG를 PNG로 render하고 automated diagram audit 실행**

skill-provided rendering/audit command를 사용한다. 두 diagram 모두 connector endpoint, intrusion, crossing,
mixed-corner, sequence-style, clipping, text check가 pass해야 한다.
두 번 re-render하고 deterministic PNG hash를 요구한다.

- [ ] **Step 3: mandatory SVG 및 PNG eye inspection 수행**

네 file 모두 original detail로 연다. SVG-to-PNG conversion 뒤 arrowhead direction,
bend와 card edge에서 arrowhead size/clearance, horizontal routing opportunity,
text/card를 통과하는 connector 없음, clipping 없음, 읽을 수 있는 label, SVG/PNG correspondence를 명시적으로 verify한다.
기술적으로 valid하지만 visually wrong인 render를 accept하지 말고 bend coordinate를 조정한다.

- [ ] **Step 4: 두 locale README에 두 diagram link 후 commit**

```bash
git diff --check
git add docs/images/readme-diagrams/audited-order-workflow-outbox-* examples/audited-order-workflow-outbox/README.md examples/audited-order-workflow-outbox/README.ko.md
git commit -m "docs: diagram audited workflow integration"
```

기대값: 네 asset이 tracked되고 audit이 PASS하며 manual inspection이 기록된다.
두 locale README는 PNG render를 reference한다.

## Task 9: risk gate, full verification, review, lesson 실행

**Complexity:** High. **Depends on:** Tasks 1-8. **Pattern skills:** `verification-before-completion`, `bluetape-full-feature`, `bluetape-go-patterns`. **Write scope:** prior scope 안의 fix, review artifact, lesson artifact.

- [ ] **Step 1: targeted 및 resource-bounded gate를 처음부터 실행**

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/...
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox -run 'TestRequestsHTTPSmoke'
go test -p 1 -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check origin/develop
```

기대값: 모든 command가 fresh observed exit 0을 가진다. full parallel Docker test가 timeout되면 raw evidence를 보존하고
container/resource state를 diagnose하며 isolated failing package를 rerun하고 원인을 repair한다.
isolated retry는 required final fresh `make ci` pass를 대체하지 않는다.

- [ ] **Step 2: triggered performance/stability scan 실행**

request allocation/body bound, query plan, row-lock duration, pool contention, relay polling/backlog,
goroutine ownership, cancellation, lease, Testcontainers cleanup, shutdown을 inspect한다.
P0/P1을 fix하고 affected focused/broad gate를 모두 rerun한다.

- [ ] **Step 3: approved spec 및 plan item 전체 verify**

exact spec, 이 plan, branch diff, test, README, HTTP file, 네 diagram을 대상으로 Step 5 verifier checklist를 사용한다.
traceable PASS를 기록하거나 owning task로 돌아간다. missing item을 optional로 reinterpret하지 않는다.

- [ ] **Step 4: six pre-PR review lens 실행 및 finding integrate**

performance, stability, security, operator/Ops, developer/API, user/caller를 independent하게 review한다.
P0/P1을 fix하고 P2/P3는 rationale과 함께 resolve/defer하며 affected test와 lane을 rerun한다.
latest result가 P0=0/P1=0이 된 뒤에만 `docs/review/2026-07-14-issue-68-audited-order-workflow-outbox.md`를 작성한다.

- [ ] **Step 5: review artifact 및 durable lesson commit**

lesson은 context, chosen history/outbox split, replay 및 readiness decision,
Docker baseline contention, relay/diagram surprise, proof, review miss, future guard를 기록한다.

```bash
git add docs/review/2026-07-14-issue-68-audited-order-workflow-outbox.md docs/lessons/2026-07-14-issue-68-audited-order-workflow-outbox.md
git commit -m "docs: record audited workflow verification lessons"
git status --short
```

기대값: worktree가 clean하고 lesson은 PR creation 전에 commit되어 있다.

## Task 10: PR 생성 후 merge approval gate에서 중지

**Complexity:** Medium. **Depends on:** Task 9. **Pattern skills:** `bluetape-workflow`. **Write scope:** live review가 approved repair를 요구하지 않는 한 repository file 없음. **External effects:** push와 PR은 approved workflow로 authorized되지만 merge는 아니다.

- [ ] **Step 1: issue #68 metadata를 다시 읽고 feature branch push**

assignee `debop`, milestone `0.9.0`, label, dependency, issue state를 confirm한다.
clean feature branch만 push한다.

- [ ] **Step 2: English PR 생성 및 verify**

central template을 사용하고 what보다 why를 먼저 설명한다. complete validation 및 diagram eye-inspection evidence를 포함하고,
#68을 close하며 `## DoD Status`로 끝낸다. `debop`을 assign하고 milestone/label을 mirror한 뒤 `gh pr view`로 live metadata를 verify한다.

- [ ] **Step 3: live PR review 및 CI gate 실행**

actual PR diff를 six perspective 전체로 review하고 thread를 resolve하며 모든 required check가 successful이 될 때까지 bounded interval로 wait한다.
pending, skipped, stale, missing check를 green으로 취급하지 않는다.

- [ ] **Step 4: exact merge-ready state 보고 후 중지**

PR URL, head/base SHA, review convergence, required check X/Y, clean local state, remaining risk를 보고한다.
user가 merge를 명시적으로 approve하기 전까지 merge, worktree delete, `develop` sync를 하지 않는다.

## plan review convergence

| Perspective | Result | Resolved focus |
|---|---|---|
| Performance | P0=0, P1=0 | hot-aggregate plan evidence, pool contention metric, deterministic lock timeout/recovery가 ordered 상태다. |
| Stability | P0=0, P1=0 | same-order create race, post-commit error injection, replay, lease recovery, lifecycle rerun이 explicit하다. |
| Security | P0=0, P1=0 | loopback/stream validation, all-stage redaction, slow-client/pool limit, lossless JSON negative가 implementation보다 앞선다. |
| Operator/Ops | P0=0, P1=0 | schema 세 개, readiness/status, recovery/runbook, non-destructive compatibility failure를 소유한다. |
| Developer/API | P0=0, P1=0 | v0.18.0 `Relay.Run`, reader signature, route 일곱 개, command, task dependency가 implementable하다. |
| User/caller | P0=0, P1=0 | exact envelope, request-file smoke, bilingual structural parity, replay, cursor, overload, 409 path가 executable하다. |

모든 review finding은 이 plan 또는 approved specification의 non-material v0.18.0 observability correction에서 fix했다.
affected lane은 integrated artifact를 대상으로 rerun했고 clear로 돌아왔다. deferred P2/P3 item은 없다.

## 인수 추적성

| spec acceptance | owning task | fresh evidence |
|---|---|---|
| Atomic order/history/outbox commit | Tasks 2-3 | PostgreSQL rollback, parity, concurrency test |
| Durable history independent of transport | Tasks 2, 4 | full reader contract 및 Redis-free query test |
| Official continuous SQL outbox to Redis | Tasks 5-6 | deterministic relay 및 real 13-field stream proof |
| Strict POST JSON for user operations | Task 4 | decoder, route, cursor, detail, status, overload test |
| Complete curl and HTTP request scenario | Task 7 | locale/request marker parity 및 smoke execution |
| Bilingual architecture/sequence diagrams | Task 8 | automated audit, deterministic render, four-file eye check |
| Issue #57 language switch correction | Task 7 | focused two-line diff inspection |
| Targeted/race/integration/full gates | Task 9 | listed command별 fresh exit 0 |
| PR metadata, green CI, merge approval stop | Task 10 | live `gh` evidence 및 merge side effect 없음 |

## 위험 예측 및 rerun 지점

| risk | signal | mitigation | rollback 또는 rerun 지점 |
|---|---|---|---|
| intended atomicity에도 partial dual-write 발생 | injected failure 뒤 table count 또는 identity가 diverge | `sqlkit.WithTx` 하나, 같은 entry value, failure-after-each-write test | Task 3 commit revert 또는 Tasks 2-3을 RED부터 rerun |
| duplicate command race가 wrong state 반환 | replay가 original projection과 다르거나 revision을 추가 | lock-before-state-check, fresh post-rollback lookup, canonical payload intent | Task 3 reopen 후 concurrency/race test rerun |
| PostgreSQL time precision이 parity 손상 | normal entry가 scalar/JSON comparison 실패 | 모든 write 전에 UTC microsecond clock value 하나로 normalize | Tasks 1-2 reopen 후 sub-microsecond test 실행 |
| Redis outage가 outbox availability 저해 | `/readyz`가 Redis만으로 503 반환 | Redis는 degraded delivery로 취급하고 DB/relay는 hard readiness로 유지 | Task 5 readiness/lifecycle test reopen |
| HTTP ready 중 relay 사망 | server stop 없이 relay result 도착 | supervised result channel, readiness transition, bounded shutdown | Task 5 unexpected-exit test reopen |
| backlog/pool contention이 request를 starve | deadline miss, pool 8 초과, backlog drain 실패 | fixed pool, request cap, bounded batch/poll, concurrent-writer test | Tasks 5-6 reopen 후 race/backlog proof rerun |
| container host contention으로 flaky full gate 발생 | package parallelism 아래 PostgreSQL wait strategy timeout | sequential example fixture 및 `go test -p 1`; retry 전 diagnose | isolated package rerun 뒤 resource-bounded suite와 fresh `make ci` |
| PNG render 뒤 diagram arrowhead reverse/collision | eye check가 SVG intent와 다름 | `bluetape-diagram`을 따르고 bend/endpoint coordinate를 조정하며 네 file 모두 inspect | Task 8로 돌아가 rerender 및 두 format re-audit |
| scope가 library/dependency/workflow change로 drift | diff에 go.mod, workflow, bluetape-go API 포함 | stop하고 scope approval 요청 | new task-owned change만 reset하고 prior commit 보존 |

## repository hazard decision

- New module/catalog/BOM/coverage registration: N/A; existing workshop module 안의 package다.
- Dependency 및 `go.mod`/`go.sum`: N/A; required package는 모두 이미 존재한다.
- Workflow/nightly change: N/A; `make ci`와 existing `go test ./...`가 example을 자동으로 discover한다.
- Changelog/release note: N/A; workshop example은 published library changelog가 아니라 README navigation과 issue closure를 사용한다.
- Testcontainers: triggered; PostgreSQL과 Redis는 sequential하게 시작하고 repository-wide resource-bounded lane은 authoritative full gate 전에 실행한다.
- README locale 및 diagram: triggered이며 Tasks 7-8이 소유한다.
