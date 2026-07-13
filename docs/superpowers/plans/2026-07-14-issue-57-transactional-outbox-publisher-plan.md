# Issue #57 Transactional Outbox Publisher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable order example that commits an order and SQL outbox entry atomically, then publishes the committed audit record through the released Redis Streams adapter with explicit at-least-once, retry, cancellation, and shutdown proof.

**Architecture:** `orderoutbox.Service` owns validation and one `sqlkit.WithTx` boundary over an order insert plus `sqloutbox.Store.Enqueue`. A caller-owned `sqloutbox.Relay` runs after commit; deterministic tests use `sqloutboxtest`, while the runnable integration uses `redisstreams.New`. `main` owns PostgreSQL/Redis readiness, timeouts, resource closure, one bounded relay batch, and JSON output.

**Tech Stack:** Go 1.26.3, bluetape-go v0.18.0 `audit/sqloutbox`, `audit/sqloutbox/sqloutboxtest`, `audit/sqloutbox/redisstreams`, `sqlkit`, pgx v5, go-redis v9, repository PostgreSQL/Redis Testcontainers fixtures, CairoSVG.

---

## File Map

| File | Responsibility |
|---|---|
| `examples/transactional-outbox-publisher/internal/orderoutbox/model.go` | Domain values, stable errors, validation, audit entry construction. |
| `examples/transactional-outbox-publisher/internal/orderoutbox/schema.go` | Example order schema and `sqloutbox.Store.CreateSchema` delegation. |
| `examples/transactional-outbox-publisher/internal/orderoutbox/service.go` | Constructor and atomic order/outbox transaction. |
| `examples/transactional-outbox-publisher/internal/orderoutbox/service_test.go` | Constructor, validation, PostgreSQL atomicity, rollback, cancellation. |
| `examples/transactional-outbox-publisher/internal/orderoutbox/relay_test.go` | Relay success, retry, duplicate, dead-letter, cancellation, shutdown. |
| `examples/transactional-outbox-publisher/internal/orderoutbox/integration_test.go` | Sequential PostgreSQL/Redis adapter and stream-field proof. |
| `examples/transactional-outbox-publisher/main.go` | Configuration, client readiness/closure, relay batch, JSON output. |
| `examples/transactional-outbox-publisher/main_test.go` | Configuration, output, error, and runtime proof. |
| `examples/transactional-outbox-publisher/README.md`, `README.ko.md` | Bilingual lesson and operational boundaries. |
| `README.md`, `README.ko.md` | Root navigation and run section. |
| `docs/images/readme-diagrams/transactional-outbox-publisher-architecture.{svg,png}` | Static ownership architecture. |
| `docs/images/readme-diagrams/transactional-outbox-publisher-sequence.{svg,png}` | Transaction/retry/cancellation sequence. |
| `docs/review/2026-07-14-issue-57-transactional-outbox-publisher.md` | Final verification/review evidence. |
| `docs/lessons/2026-07-14-issue-57-transactional-outbox-publisher.md` | Durable implementation/diagram lessons. |

No `go.mod`, `go.sum`, workflow, module registration, public bluetape-go API,
or changelog edit is planned. Any such diff stops the task for reapproval.

## Task 1: Define domain, configuration, and schema contracts

**Complexity:** Medium. **Depends on:** approved spec. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `model.go`, `schema.go`, `service_test.go`, then `service.go`.

- [ ] **Step 1: Write constructor and validation tests first**

Create table tests for nil store, blank/invalid/oversized author, nil clock,
zero-value/nil service, nil database/context, pre-cancelled context,
blank/invalid/oversized IDs, non-positive totals, and zero `CreatedAt`.

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

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox`

Expected: FAIL because the package and constructor contracts do not exist.

- [ ] **Step 3: Implement minimal values and constructor**

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

Trim and bound author to 1..128 valid UTF-8 runes, default `Now` to UTC
`time.Now`, preserve caller identifiers after trimming, and make nil/zero-value
methods fail closed with `%w`-wrapped sentinels.

- [ ] **Step 4: Add schema creation and command validation**

`Service.CreateSchema` executes fixed, parameter-free example DDL and delegates
to its configured `sqloutbox.Store`:

```sql
create table if not exists transactional_outbox_orders (
    order_id text primary key,
    customer_id text not null,
    status text not null check (status = 'placed'),
    total_cents bigint not null check (total_cents > 0),
    created_at timestamptz not null
)
```

Normalize nil context to `context.Background`, preserve cancellation, validate
IDs as valid UTF-8 with 1..128 runes, require positive total, and normalize
`CreatedAt` to UTC before opening a transaction.

- [ ] **Step 5: Run GREEN and format**

```bash
gofmt -w examples/transactional-outbox-publisher/internal/orderoutbox/*.go
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'Test(NewService|ServiceValidation|ServiceCreateSchema)'
```

Expected: PASS and `git diff --check` clean.

## Task 2: Commit the order and outbox entry atomically

**Complexity:** High. **Depends on:** Task 1. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `service_test.go`, then `model.go` and `service.go`.

- [ ] **Step 1: Write PostgreSQL atomicity tests**

Use one `TestServicePlacePostgreSQL` with sequential subtests sharing one
`postgrestestcontainer.Start` instance under a 90-second context, pgx
`sql.Open`, real `PingContext`, table reset between subtests, and `t.Cleanup`.
Never use `t.Parallel`. Prove successful
one-order/one-outbox commit, duplicate-order rollback, an outbox identity
conflict after the order insert, pre-cancel rollback, UTC timestamps, and stable
event/idempotency identity.

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

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestServicePlace'`

Expected: FAIL because `Place` and audit entry construction are absent.

- [ ] **Step 3: Build one released-contract audit entry**

Marshal only customer ID, status, and total cents. Call
`audit.NewAggregateID`, `audit.NewDomainEvent`, and `audit.NewEntry` with initial
revision, event type `order.placed`, caller command ID for both identity fields,
`OccurredAt=CreatedAt`, and `RecordedAt=service clock`. Wrap all errors with
`%w`; do not add caller JSON, metadata, snapshots, or generated IDs.

- [ ] **Step 4: Implement one transaction**

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

Return `Order` only after commit. Never call Redis, publisher, or relay inside
the transaction.

- [ ] **Step 5: Run GREEN and focused race**

```bash
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestServicePlace'
go test -race -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestServicePlace'
```

Expected: exact table counts and rollback assertions PASS; race is clean.

## Task 3: Prove relay retry, duplicate, dead-letter, and shutdown

**Complexity:** High. **Depends on:** Task 2. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `relay_test.go` and shared test helpers only.

- [ ] **Step 1: Write deterministic success/retry tests**

Use the same mutable clock in `sqloutbox.Options.Now` and `RelayOptions.Now`.
Configure `RecordingPublisher` to fail `command-retry` once. Assert first
`RunOnce` is claimed/failed with attempt 1, the pre-retry call claims zero,
advancing exactly 250 ms makes the second call publish attempt 2, and both
attempts retain identical event/idempotency identity.

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

- [ ] **Step 2: Add dead-letter and cancellation tests**

Fail three eligible attempts and assert final `dead_letter`, attempts 3, and no
published result. For cancellation, a `PublisherFunc` cancels the caller and
returns `ctx.Err()`; assert `context.Canceled`, status remains `claimed`,
attempts 1, and no retry/dead-letter write.

- [ ] **Step 3: Prove continuous `Run` joins without sleeps**

A `PublisherFunc` closes `started`, waits on `ctx.Done`, and returns the context
error. Start `Relay.Run`, wait for `started`, cancel, and join through a bounded
channel. Require `context.Canceled`, exactly one invocation, and no late call.

```go
select {
case err := <-done:
    if !errors.Is(err, context.Canceled) { t.Fatalf("Run() = %v", err) }
case <-time.After(5 * time.Second):
    t.Fatal("relay did not stop after cancellation")
}
```

- [ ] **Step 4: Add bounded concurrent `RunOnce` stress proof**

Enqueue 12 independent aggregate records, start four `RunOnce` workers from a
closed barrier channel, and use concurrent-safe `RecordingPublisher`. Across all
worker results require exactly 12 claimed, 12 published, zero failed/dead-letter,
12 unique event IDs, and all 12 SQL rows `published`. Repeat the normal test 10
times; no goroutine may poll after its one call returns.

- [ ] **Step 5: Run RED then GREEN/race**

```bash
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestRelay'
go test -count=10 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRelayConcurrentRunOnce$'
go test -race -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run 'TestRelay'
```

Expected: exact results/status/attempt counts PASS, shutdown joins, race clean.
Do not reimplement relay/store behavior in workshop code.

## Task 4: Integrate the official Redis Streams publisher

**Complexity:** High. **Depends on:** Tasks 2-3. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `integration_test.go` only unless a source-backed application gap appears.

- [ ] **Step 1: Write one sequential dual-container test**

Start PostgreSQL then Redis with released fixtures under one 120-second context.
Open/ping both clients and register cleanup. Do not use `t.Parallel`.

```go
postgresURL := postgrestestcontainer.Start(ctx, t)
redisAddr := redistestcontainer.Start(ctx, t)
db, err := sql.Open("pgx", postgresURL)
client := redis.NewClient(&redis.Options{Addr: redisAddr})
if err := db.PingContext(ctx); err != nil { t.Fatal(err) }
if err := client.Ping(ctx).Err(); err != nil { t.Fatal(err) }
```

Place one order and construct only `redisstreams.New`, then one
`Relay.RunOnce` with claim limit 1.

- [ ] **Step 2: Verify every documented Redis field**

Read at most two entries using `XRangeN(ctx, stream, "-", "+", 2)`. Require
exactly one message and exact values for `record_id`, `status`,
`aggregate_type`, `aggregate_id`, `revision`, `event_id`, `idempotency_key`,
`event_type`, `occurred_at`, `recorded_at`, `schema_version`, `attempts`, and
`entry_json`. Decode `entry_json` with `audit.DecodeEntryJSON` and require scalar
parity. Query PostgreSQL for `published` and attempts 1.

- [ ] **Step 3: Run integration and race sequentially**

```bash
go test -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRedisStreamsIntegration$'
go test -race -count=1 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRedisStreamsIntegration$'
```

Expected: both PASS with command readiness. A first-fail/retry-pass result is
diagnosed, not dismissed as container noise.

## Task 5: Build the bounded runnable command

**Complexity:** High. **Depends on:** Task 4. **Pattern skills:** `bluetape-go-patterns`, `test-driven-development`. **Write scope:** `main_test.go`, then `main.go`.

- [ ] **Step 1: Write configuration and output tests**

Define unexported `appConfig`, `loadConfig(getenv, now)`, `openDependencies`,
`dependencies.Close`, `execute(ctx, db, redisClient, config, output)`, and
`run(ctx, config, output)` boundaries. Use a
map-backed environment reader. Prove missing endpoints name only the variable,
IDs use 1..128-rune rules, `REDIS_STREAM` defaults through the provider or is
valid UTF-8 at most 256 bytes, `ORDER_CREATED_AT` is RFC3339 or the injected
clock, config errors do not echo values, and JSON is newline-terminated. One
sequential fixture-backed `TestDependenciesClose` proves both clients are ready
before close and reject Ping after the owned close.

```go
type runOutput struct {
    Order          orderoutbox.Order     `json:"order"`
    Relay          sqloutbox.RelayResult `json:"relay"`
    Stream         string                `json:"stream"`
    EventID        string                `json:"event_id"`
    IdempotencyKey string                `json:"idempotency_key"`
}
```

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./examples/transactional-outbox-publisher -run 'Test(LoadConfig|Execute|Run)'`

Expected: FAIL because main-package runtime boundaries do not exist.

- [ ] **Step 3: Implement client ownership, readiness, and batch execution**

`run` opens pgx and go-redis clients, immediately defers close, uses bounded
child contexts for both Pings, and joins close errors with the return error.
`main` uses `signal.NotifyContext`, prints one stage-wrapped error, and exits
nonzero without interpolating endpoint values.

`dependencies.Close` invokes both close functions exactly once and returns
`errors.Join(postgresErr, redisErr)`. `run` installs the defer immediately after
successful construction. A partial open failure closes the already-created
client before returning.

`execute` constructs the example-table store and service, creates schema,
places the order, constructs `redisstreams.New`, and runs one claim. Accept
success only for:

```go
result == (sqloutbox.RelayResult{Claimed: 1, Published: 1})
```

Read one candidate with `XRevRangeN`, require its `event_id` and
`idempotency_key` equal the command ID, then render JSON. Never call `XAdd` from
workshop code.

- [ ] **Step 4: Run GREEN and focused race**

```bash
gofmt -w examples/transactional-outbox-publisher/*.go
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
```

Expected: PASS. The final validation pass also runs the documented command
against disposable PostgreSQL and Redis endpoints and requires JSON exit 0.

## Task 6: Create and visually verify the architecture diagram

**Complexity:** High. **Depends on:** Tasks 2-5. **Pattern skill:** `bluetape-diagram` with `common.md` and `architecture.md`. **Write scope:** architecture SVG/PNG only.

- [ ] **Step 1: Open source and full-size references**

Use best-practice reference
`/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/external-redis-fast-architecture.png`
and repo-local reference
`docs/images/readme-diagrams/sql-transaction-boundary-architecture.png`.
Record both paths. The reader question is who owns transaction, clients, relay,
durable rows, and Redis transport.

- [ ] **Step 2: Draw the static ownership asset**

Create `transactional-outbox-publisher-architecture.svg` with Architects
Daughter/Comic Mono, catalog database/Redis icons, and separate application,
PostgreSQL, and Redis regions. Show:

```text
Command -> Service -> one SQL transaction -> Order row + Outbox row
Application -> Relay -> Outbox claim/update
Relay -> official redisstreams Publisher -> Redis Stream
Application -> caller-owned PostgreSQL/Redis clients and shutdown
```

Prefer straight horizontal links. Rounded orthogonal bends use separate ports,
at least `max(8px, rx/2)` corner clearance, and a terminal segment long enough
for a 14x14 primary arrowhead. No connector may hug or enter a card.

- [ ] **Step 3: Parse, render, audit, then inspect at full size**

```bash
xmllint --noout docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
cairosvg docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg -o docs/images/readme-diagrams/transactional-outbox-publisher-architecture.png -s 2
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-architecture.svg
```

Expected: meaningful nonzero connector/card/path counts and zero diagonal,
endpoint, intrusion, crossing, and mixed-corner failures. After the final
coordinate change, open the PNG at full size and inspect arrow direction,
size/color, bend clearance, labels, icons, margins, and bottom whitespace. PNG
evidence overrides scripts.

## Task 7: Create and visually verify the retry sequence diagram

**Complexity:** High. **Depends on:** Task 6 and final relay behavior. **Pattern skill:** `bluetape-diagram` with `common.md` and `sequence.md`. **Write scope:** sequence SVG/PNG only.

- [ ] **Step 1: Open both sequence references at full size**

Use best-practice reference
`/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-core-sequence-03.png`
and repo-local reference
`docs/images/readme-diagrams/sql-transaction-boundary-sequence.png`.
The reader question is how one stable event is attempted twice while the SQL
commit remains atomic.

- [ ] **Step 2: Draw the chronological asset**

Participants are Application, Service, PostgreSQL, Relay, Redis Publisher, and
Redis Stream. Add lifelines, activation bars, visible numbered pills, and
transparent frames for order/outbox commit, claim attempt 1, ambiguous/failing
publish plus retry scheduling, attempt 2 with unchanged identity, append plus
published completion, and alternate caller cancellation without retry/dead
letter. Use explicit per-color 16x16 arrowheads and continuous message lines.

- [ ] **Step 3: Run common and sequence-specific proof**

Run all Task 6 common commands against the sequence asset plus:

```bash
python3 "$HOME/.codex/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/transactional-outbox-publisher-sequence.svg
```

Expected: common failures zero; visible ordered labels, participant/lifeline/
activation counts, transparent frames, and marker color parity. Open the final
PNG at full size after the last coordinate change and inspect every arrowhead
direction, bend clearance, label/line overlap, frame padding, and cancellation
branch. SVG-only or contact-sheet evidence is invalid.

## Task 8: Write bilingual READMEs and root navigation

**Complexity:** Medium. **Depends on:** Tasks 5-7. **Pattern skills:** `bluetape-writer`, `bluetape-diagram`. **Write scope:** example and root README locale pairs.

- [ ] **Step 1: Write the English README from verified behavior**

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

- [ ] **Step 2: Produce natural Korean source parity**

Use `bluetape-writer`. Preserve every command, field, warning, diagram embed,
and ownership boundary. Share the English-label assets and add the required
language switch.

- [ ] **Step 3: Add root navigation/run parity**

Add an audit/outbox-adjacent row in both root tables with packages
`audit/sqloutbox`, `audit/sqloutbox/redisstreams`, `testcontainers/postgres`,
and `testcontainers/redis`. Add matched English/Korean run/test sections.

- [ ] **Step 4: Verify docs and diagram exposure**

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

- [ ] **Step 1: Run fresh proof in order**

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

- [ ] **Step 2: Complete performance/stability and Type A verification**

Inspect unbounded Redis reads, retry/poll loops, DB round trips, context/timer/
goroutine ownership, client closure, readiness, provider error exposure, and
container startup. Map the exact spec and plan to the final diff, tests, locale
pair, navigation, and both PNGs. Repair all P0/P1 and rerun affected proof.

- [ ] **Step 3: Converge six review perspectives**

Review performance, stability, security, Ops, developer/API, and user/caller,
then integrate in the main session. Write
`docs/review/2026-07-14-issue-57-transactional-outbox-publisher.md` only at
P0=0/P1=0. Diagram rows record commands, nonzero counts, PNG dimensions,
reference paths, and full-size inspection notes rather than “passed”.

- [ ] **Step 4: Commit the durable lesson before PR**

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
