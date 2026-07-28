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

## Task 5: Supervise the continuous relay and server lifecycle

**Complexity:** High. **Depends on:** Tasks 3-4. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `runtime.go`, `relay_test.go`, `main.go`, `main_test.go`.

- [ ] **Step 1: Write deterministic relay and signal tests**

Use `sqloutboxtest.RecordingPublisher`, a mutable clock, and
`PublisherFunc` to prove success, one retry after exactly 250 ms, three-attempt
dead-letter, duplicate physical attempts with stable identity, caller
cancellation leaving a lease-recoverable claim, and later-revision blocking only
while an earlier record is pending/claimed. After publish cancellation, verify
claimed state, advance the shared clock beyond the official 30-second lease,
start a fresh relay, reclaim the same event/idempotency identity, publish it,
join lifecycle, and require final published state.

- [ ] **Step 2: Write lifecycle tests before runtime code**

Prove expected cancellation joins cleanly; unexpected `Relay.Run` failure
marks readiness false, logs one redacted lifecycle transition, shuts down HTTP,
and returns failure. Redis outage keeps readiness HTTP 200 with degraded
delivery, while database failure or a stopped relay returns 503. Assert idle
relay loops do not emit logs; official `Relay.Run` intentionally provides no
per-batch result, so `/statusz` owns current delivery counts. Use a
real listener with stalled-header and stalled-body clients to prove header/read
timeouts release connections. With short injected test durations, prove
oversized headers, write timeout, idle timeout, and graceful-shutdown deadline.
Exhaust PostgreSQL and Redis pools and require their configured ceilings and
bounded timeout behavior. Add table tests accepting IPv4/IPv6 loopback literals
and rejecting IPv4/IPv6 wildcards, `localhost`, hostname-only, non-loopback,
malformed, missing-port, and zone-scoped addresses. Test Redis stream default,
blank, invalid UTF-8, and more than 256 bytes. Inject recognizable secret
markers at config, open, ping, schema, relay, HTTP serve, and close stages;
require one centralized safe projection with stage and stable class only, and
assert markers are absent from both logs and top-level returned errors.

- [ ] **Step 3: Run RED**

Run: `go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'Test(Relay|Lifecycle|Readiness|Status)'`

Expected: FAIL because runtime ownership is absent.

- [ ] **Step 4: Implement bounded resources and runtime configuration**

Set PostgreSQL `MaxOpenConns(8)`, `MaxIdleConns(8)`, 5-minute idle and
30-minute lifetime. Configure Redis pool size 8, minimum idle 1, pool timeout
2 seconds. Build `http.Server` with 2-second header, 5-second read/write,
30-second idle, 16 KiB header, and 5-second shutdown limits. Relay options are
claim 16, attempts 3, retry 250 ms, idle 50 ms.

- [ ] **Step 5: Implement supervision and safe observations**

Own the relay context and result channel in one lifecycle function. Make early
`context.Canceled` unexpected, mark readiness false before server shutdown, and
join independent close errors. Call the official `Relay.Run` directly and
report only delivery-degradation and lifecycle transitions through an injected
observer; never log records, endpoints, provider errors, or metadata.

- [ ] **Step 6: Implement loopback-only configuration and `main`**

Require `DATABASE_URL` and `REDIS_ADDR`; accept optional `REDIS_STREAM` and an
IP-literal `HTTP_ADDR`. Parse with `net.SplitHostPort`, then `net.ParseIP`
without DNS resolution, and reject nil IP, wildcard, zone, or
`!ip.IsLoopback()`. Centralize public errors in `safeStageError(stage, class)`;
never return a wrapped provider/configuration value to `main`. Startup order is
parse, open, bounded ping, schema bootstrap, publisher/relay construction,
listener, relay, HTTP. Shutdown order is HTTP drain, relay cancel/join, Redis
close, database close.

- [ ] **Step 7: Run GREEN, race, and commit**

```bash
gofmt -w $(rg --files examples/audited-order-workflow-outbox -g '*.go')
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/... -run 'Test(Relay|Lifecycle|HTTP)'
go test -count=1 ./examples/audited-order-workflow-outbox/internal/orderworkflow -run 'TestRelayLeaseRecovery'
git add examples/audited-order-workflow-outbox
git commit -m "feat: run supervised audit outbox relay"
```

## Task 6: Prove PostgreSQL and Redis integration sequentially

**Complexity:** High. **Depends on:** Task 5. **Pattern skills:** `test-driven-development`, `bluetape-go-patterns`. **Write scope:** `integration_test.go` and test helpers only.

- [ ] **Step 1: Add one sequential end-to-end fixture**

Start PostgreSQL first, complete schema/command/history/restart assertions, then
start Redis. Do not use `t.Parallel`. Publish created and confirmed events via
`redisstreams.New` and require the documented 13 fields, exact event and
idempotency identity, valid `entry_json`, and tolerance of duplicate physical
stream entries.

- [ ] **Step 2: Add restart, outage, lease, and backlog cases**

Recreate service/store/runtime objects over the same databases and prove state,
history, replay, and pending rows persist. Stop/unavailable Redis must not roll
back commands. Recover the client, drain multiple 16-record batches while
writers continue, assert counts/deadline, `db.Stats().MaxOpenConnections == 8`,
and no later record bypasses an earlier pending/claimed record. Sample
`db.Stats()` every 10 ms: peak `InUse` must not exceed 8, every writer must
finish within its 2-second operation deadline, the backlog must drain within the
fixed test deadline, and total `WaitDuration` must stay below
`2 seconds * writerCount` so relay monopolization cannot pass silently.

- [ ] **Step 3: Run the sequential package proof**

```bash
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox/... -run 'TestIntegration'
go test -race -count=1 -p 1 ./examples/audited-order-workflow-outbox/... -run 'TestIntegration(Relay|Concurrent)'
```

Expected: PostgreSQL then Redis cases PASS with fresh exit 0 and no parallel
container startup.

- [ ] **Step 4: Commit integration proof**

```bash
git add examples/audited-order-workflow-outbox/internal/orderworkflow/integration_test.go
git commit -m "test: prove audited workflow delivery integration"
```

## Task 7: Add runnable POST JSON documentation

**Complexity:** Medium. **Depends on:** Tasks 4-6. **Pattern skills:** `bluetape-writer`, `bluetape-go-patterns`. **Write scope:** example README pair, `requests.http`, root README pair, issue #57 README pair.

- [ ] **Step 1: Write `requests.http` as the executable source contract**

Include variables and complete JSON for create, confirm, identical replay,
two-page search, detail revision 2, second-order cancel with reason, and rejected
post-cancel confirm. Every request includes method, URL, content type, and body.

```http
### Create order
POST {{baseURL}}/orders
Content-Type: application/json

{"order_id":"order-1001","command_id":"cmd-create-1001","metadata":{"channel":"workshop"}}
```

- [ ] **Step 2: Write source-equivalent English and Korean READMEs**

Use `English | [한국어](README.ko.md)` and
`[English](README.md) | 한국어`. Explain architecture, transaction boundary,
history versus transport, loopback-only startup, at-least-once and reordering,
status/readiness, exact curl bodies, expected replay/cursor/409/429 results, and
shutdown. Require copy-paste curl commands, not abbreviated JSON.

- [ ] **Step 3: Add the operator runbook and schema boundary**

Document safe status inspection, Redis outage/recovery, lease wait, dead-letter
diagnosis, partial-startup restart, and an explicitly destructive local reset.
State that startup DDL is a single-version workshop bootstrap, not migration or
production rollback tooling.

- [ ] **Step 4: Update navigation and fix issue #57 language switches**

Add the new example to both root README tables/run sections. Change only the
language switch lines in the issue #57 README pair so the current locale is
plain text. Preserve all other issue #57 content.

- [ ] **Step 5: Validate locale and request parity, then commit**

```bash
rg -n '^POST |Content-Type: application/json|replayed|next_from_revision|too_many_requests|invalid_transition' examples/audited-order-workflow-outbox/{requests.http,README.md,README.ko.md}
go test -count=1 ./examples/audited-order-workflow-outbox -run 'TestDocumentationParity'
git diff --check
git add README.md README.ko.md examples/audited-order-workflow-outbox examples/transactional-outbox-publisher/README.md examples/transactional-outbox-publisher/README.ko.md
git commit -m "docs: add audited workflow POST JSON guide"
```

Expected: every scenario marker exists in all three artifacts and locale
switches have no current-locale self-link. `TestDocumentationParity` extracts
normalized scenario names, method, URL, JSON body, expected status, replay/cursor
expectations, warnings, runbook steps, and unsupported behavior from the English
README, Korean README, and `requests.http`; it requires structural equality, not
marker presence alone.

- [ ] **Step 6: Execute the checked-in HTTP scenario against real backends**

Create `smoke_test.go` that starts PostgreSQL then Redis through the repository
fixtures, starts the loopback application, waits for `/readyz`, parses
`requests.http`, substitutes `{{baseURL}}`, and sends each checked-in request.
Scenario annotations define expected status and response assertions. Require
201 create, 200 confirm, 200 replay with `replayed: true`, page cursor 2 then
null, detail revision 2, cancellation, and stable 409 after cancellation. Always
stop HTTP and containers through bounded cleanup.

Run:
`go test -count=1 -p 1 ./examples/audited-order-workflow-outbox -run 'TestRequestsHTTPSmoke'`

Expected: PASS using the actual checked-in request bodies and fresh exit 0.

## Task 8: Create and visually verify architecture and sequence diagrams

**Complexity:** High. **Depends on:** Tasks 5-7. **Pattern skills:** `bluetape-diagram`. **Write scope:** four canonical image assets and README image references.

- [ ] **Step 1: Load the diagram checklist and best-practice references**

Follow `bluetape-diagram` completely. Reuse repository visual grammar and draw
generated SVG directly; Mermaid is not a final artifact. Architecture must show
routes, service, one transaction, three PostgreSQL tables, supervised relay,
Redis, and the history-query bypass. Sequence must show validate/lock, three
writes, commit, response, later relay, replay, and query.

- [ ] **Step 2: Render SVG to PNG and run automated diagram audits**

Use the skill-provided rendering/audit commands. Require connector endpoint,
intrusion, crossing, mixed-corner, sequence-style, clipping, and text checks to
pass for both diagrams. Re-render twice and require deterministic PNG hashes.

- [ ] **Step 3: Perform mandatory SVG and PNG eye inspection**

Open all four files at original detail. Explicitly verify arrowhead direction
after SVG-to-PNG conversion, arrowhead size/clearance at bends and card edges,
horizontal routing opportunities, no connector through text/cards, no clipping,
legible labels, and SVG/PNG correspondence. Adjust bend coordinates rather than
accepting a technically valid but visually wrong render.

- [ ] **Step 4: Link both diagrams in both locale READMEs and commit**

```bash
git diff --check
git add docs/images/readme-diagrams/audited-order-workflow-outbox-* examples/audited-order-workflow-outbox/README.md examples/audited-order-workflow-outbox/README.ko.md
git commit -m "docs: diagram audited workflow integration"
```

Expected: four assets are tracked, audits PASS, manual inspection is recorded,
and both locale READMEs reference the PNG renders.

## Task 9: Run risk gates, full verification, review, and lessons

**Complexity:** High. **Depends on:** Tasks 1-8. **Pattern skills:** `verification-before-completion`, `bluetape-full-feature`, `bluetape-go-patterns`. **Write scope:** fixes within prior scopes, review artifact, lesson artifact.

- [ ] **Step 1: Run targeted and resource-bounded gates from scratch**

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

Expected: every command has a fresh observed exit 0. If full parallel Docker
tests time out, preserve raw evidence, diagnose container/resource state, rerun
the isolated failing package, and repair the cause; an isolated retry does not
replace the required final fresh `make ci` pass.

- [ ] **Step 2: Execute the triggered performance/stability scan**

Inspect request allocation/body bounds, query plans, row-lock duration, pool
contention, relay polling/backlog, goroutine ownership, cancellation, leases,
Testcontainers cleanup, and shutdown. Fix P0/P1 and rerun every affected focused
and broad gate.

- [ ] **Step 3: Verify every approved spec and plan item**

Use the Step 5 verifier checklist against the exact spec, this plan, branch
diff, tests, READMEs, HTTP file, and four diagrams. Record a traceable PASS or
return to the owning task; do not reinterpret a missing item as optional.

- [ ] **Step 4: Run six pre-PR review lenses and integrate findings**

Review performance, stability, security, operator/Ops, developer/API, and
user/caller independently. Fix P0/P1, resolve/defer P2/P3 with rationale, rerun
affected tests and lanes, and write
`docs/review/2026-07-14-issue-68-audited-order-workflow-outbox.md` only after the
latest result is P0=0/P1=0.

- [ ] **Step 5: Commit the review artifact and durable lesson**

The lesson records context, chosen history/outbox split, replay and readiness
decisions, Docker baseline contention, relay/diagram surprises, proof, review
misses, and future guards.

```bash
git add docs/review/2026-07-14-issue-68-audited-order-workflow-outbox.md docs/lessons/2026-07-14-issue-68-audited-order-workflow-outbox.md
git commit -m "docs: record audited workflow verification lessons"
git status --short
```

Expected: worktree clean and the lesson is committed before PR creation.

## Task 10: Create the PR and stop at the merge approval gate

**Complexity:** Medium. **Depends on:** Task 9. **Pattern skills:** `bluetape-workflow`. **Write scope:** no repository files unless live review requires an approved repair. **External effects:** push and PR are authorized by the approved workflow; merge is not.

- [ ] **Step 1: Re-read issue #68 metadata and push the feature branch**

Confirm assignee `debop`, milestone `0.9.0`, labels, dependencies, and issue
state. Push only the clean feature branch.

- [ ] **Step 2: Create and verify the English PR**

Use the central template, explain why before what, include complete validation
and diagram eye-inspection evidence, close #68, and end with `## DoD Status`.
Assign `debop`, mirror milestone and labels, then verify live metadata with
`gh pr view`.

- [ ] **Step 3: Run the live PR review and CI gate**

Review the actual PR diff through all six perspectives, resolve threads, and
wait in bounded intervals until every required check is successful. Never treat
pending, skipped, stale, or missing checks as green.

- [ ] **Step 4: Report the exact merge-ready state and stop**

Report PR URL, head/base SHAs, review convergence, required checks X/Y, clean
local state, and remaining risks. Do not merge, delete the worktree, or sync
`develop` until the user explicitly approves merge.

## Plan Review Convergence

| Perspective | Result | Resolved focus |
|---|---|---|
| Performance | P0=0, P1=0 | Hot-aggregate plan evidence, pool contention metrics, and deterministic lock timeout/recovery are ordered. |
| Stability | P0=0, P1=0 | Same-order create races, post-commit error injection, replay, lease recovery, and lifecycle reruns are explicit. |
| Security | P0=0, P1=0 | Loopback/stream validation, all-stage redaction, slow-client/pool limits, and lossless JSON negatives precede implementation. |
| Operator/Ops | P0=0, P1=0 | All three schemas, readiness/status, recovery/runbook, and non-destructive compatibility failure are owned. |
| Developer/API | P0=0, P1=0 | v0.18.0 `Relay.Run`, reader signatures, seven routes, commands, and task dependencies are implementable. |
| User/caller | P0=0, P1=0 | Exact envelopes, request-file smoke, bilingual structural parity, replay, cursor, overload, and 409 paths are executable. |

Every review finding was fixed in this plan or the non-material v0.18.0
observability correction in the approved specification. Each affected lane was
rerun against the integrated artifacts and returned clear; no P2/P3 item is
deferred.

## Acceptance Traceability

| Spec acceptance | Owning task | Fresh evidence |
|---|---|---|
| Atomic order/history/outbox commit | Tasks 2-3 | PostgreSQL rollback, parity, and concurrency tests |
| Durable history independent of transport | Tasks 2, 4 | Full reader contract and Redis-free query tests |
| Official continuous SQL outbox to Redis | Tasks 5-6 | Deterministic relay plus real 13-field stream proof |
| Strict POST JSON for user operations | Task 4 | Decoder, route, cursor, detail, status, and overload tests |
| Complete curl and HTTP request scenario | Task 7 | Locale/request marker parity and smoke execution |
| Bilingual architecture/sequence diagrams | Task 8 | Automated audits, deterministic renders, four-file eye check |
| Issue #57 language switch correction | Task 7 | Focused two-line diff inspection |
| Targeted/race/integration/full gates | Task 9 | Fresh exit 0 for every listed command |
| PR metadata, green CI, merge approval stop | Task 10 | Live `gh` evidence and no merge side effect |

## Risk Prediction and Rerun Points

| Risk | Signal | Mitigation | Rollback or rerun point |
|---|---|---|---|
| Partial dual-write despite intended atomicity | Table counts or identities diverge after injected failure | One `sqlkit.WithTx`, same entry value, failure-after-each-write tests | Revert Task 3 commit or rerun Tasks 2-3 from RED |
| Duplicate command race returns wrong state | Replay differs from original projection or adds revision | Lock-before-state-check, fresh post-rollback lookup, canonical payload intent | Reopen Task 3 and rerun concurrency/race tests |
| PostgreSQL time precision corrupts parity | Normal entry fails scalar/JSON comparison | Normalize one UTC microsecond clock value before all writes | Reopen Tasks 1-2 and run sub-microsecond test |
| Redis outage defeats outbox availability | `/readyz` returns 503 only for Redis | Treat Redis as degraded delivery; DB/relay remain hard readiness | Reopen Task 5 readiness/lifecycle tests |
| Relay dies while HTTP stays ready | Relay result arrives without server stop | Supervised result channel, readiness transition, bounded shutdown | Reopen Task 5 unexpected-exit test |
| Backlog/pool contention starves requests | Deadline miss, pool exceeds 8, backlog fails to drain | Fixed pools, request cap, bounded batch/poll, concurrent-writer test | Reopen Tasks 5-6 and rerun race/backlog proof |
| Container host contention produces flaky full gate | PostgreSQL wait strategy times out under package parallelism | Sequential example fixture and `go test -p 1`; diagnose before retry | Rerun isolated package, then resource-bounded suite and fresh `make ci` |
| Diagram arrowheads reverse or collide after PNG render | Eye check differs from SVG intent | Follow `bluetape-diagram`, adjust bend/endpoint coordinates, inspect all four | Return to Task 8; rerender and re-audit both formats |
| Scope drifts into library/dependency/workflow changes | Diff includes go.mod, workflow, or bluetape-go API | Stop and request scope approval | Reset only new task-owned changes; preserve prior commits |

## Repository Hazard Decisions

- New module/catalog/BOM/coverage registration: N/A; this is a package inside
  the existing workshop module.
- Dependency and `go.mod`/`go.sum`: N/A; every required package already exists.
- Workflow/nightly changes: N/A; `make ci` and existing `go test ./...` discover
  the example automatically.
- Changelog/release note: N/A; workshop examples use README navigation and issue
  closure rather than a published library changelog.
- Testcontainers: triggered; PostgreSQL and Redis start sequentially and the
  repository-wide resource-bounded lane runs before the authoritative full gate.
- README locales and diagrams: triggered and owned by Tasks 7-8.
