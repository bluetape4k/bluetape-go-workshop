# Issue #29/#75 설계: Customer Migration Batch Integration 예제

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: issue #29는 v0.5.0 batch umbrella이며, 열려 있는 child issue #42, #43,
  #75는 Gin operations API, leader-guarded scheduled execution, milestone-level
  integration 예제를 요구한다.
- 저장소: `bluetape4k/bluetape-go-workshop`.
- 브랜치/워크트리: `feat/issue-29-batch-integration`, 위치는
  `.worktrees/feat-issue-29-batch-integration`.

## 목표

Checkpoint restart, Gin operations API, leader-guarded scheduled execution,
retry/dead-letter 동작, 결정적 status output, 영어/한국어 README walkthrough를
결합한 실행 가능한 v0.5.0 customer migration batch integration 예제를 하나
추가한다.

집중 checkpoint/retry prerequisite인 #41, #73, #74가 이미 구현되어 있으므로,
이 PR은 #42, #43, #75를 닫고 그 다음 #29를 닫을 수 있어야 한다.

## 현재 근거

- `gh issue view 29`는 #29가 checkpoint, restart, report, scheduled execution,
  retry, integration을 포괄하는 umbrella임을 보여준다.
- `gh issue view 42`는 batch policy를 handler 밖에 유지하면서 Gin
  start/status/report handler를 요구한다.
- `gh issue view 43`은 기존 leader primitive로 보호되는 작은 scheduler loop를
  요구하며, held/missing/cancellation 테스트와 긴 sleep 금지를 포함한다.
- `gh issue view 75`는 checkpoint restart, operations API, scheduled execution,
  retry/dead-letter 동작, import fixture를 조합하는 integration 예제를 요구한다.
- 기존 예제는 local pattern을 제공한다.
  - `examples/account-migration-checkpoint-restart`: checkpoint restore.
  - `examples/chunked-csv-import-checkpoint`: failed chunk 이후 replay.
  - `examples/retry-dead-letter-batch-worker`: `batch.RetryPolicy`와
    `batch.SkipPolicy`.
  - `examples/operations-report-policy`와 `examples/order-fulfillment-integration`:
    Gin handler 형태, status mapping, 안정적인 report projection.
  - `examples/leader-coordination-jobs`: leader-owned scheduled work.
- `go doc github.com/bluetape4k/bluetape-go/batch`는 `Step`, `Job`,
  `CheckpointReader`, `CheckpointStore`, `RetryErrors`, `SkipErrors`를 확인한다.
- `go doc github.com/bluetape4k/bluetape-go/leader`는 기존 `leader.Elector`
  계약과 sentinel error를 확인한다.
- `go doc github.com/gin-gonic/gin.Engine`은 저장소 테스트가 사용하는
  `gin.New`, `ServeHTTP`, 표준 `net/http` integration을 확인한다.

## 비목표

- durable queue, Redis, NATS, database, object storage를 추가하지 않는다.
- generic scheduler, queue worker, retry framework, checkpoint storage
  framework를 구현하지 않는다.
- 기존 focused example의 `internal` package를 example boundary 너머로 import하지
  않는다. Go `internal` package visibility가 이를 의도적으로 막는다.
- in-memory checkpoint store, leader fake, dead-letter list가 production
  durable하다고 주장하지 않는다.
- 새 의존성을 추가하지 않는다.

## 검토한 접근

### A. 하나의 통합 Gin 예제 디렉터리

Domain engine, in-memory operations service, leader-gated scheduler, Gin
handler, 테스트, CLI entrypoint, README pair를 소유하는 단일 internal package와
함께 `examples/customer-migration-batch-integration`을 만든다.

이 접근을 선택한다. 같은 시나리오 안에서 #42와 #43을 충족하면서 milestone
예제를 실행 가능하고 review 가능하며 #75에 충실하게 유지한다.

### B. #42, #43, #75를 위한 세 개의 별도 예제 디렉터리

각 child issue를 가장 집중된 형태로 유지할 수 있지만 batch fixture를 중복하고
milestone integration을 늦춘다. 또한 #75를 user-visible milestone example이
아닌 얇은 wrapper로 만든다.

저장소에는 이미 checkpoint와 retry를 위한 focused example이 있고, 남은 유의미한
공백은 integration이므로 기각한다.

### C. Focused example package를 integration 예제로 import

Domain concept 중복을 피할 수 있지만 기존 focused example은 코드를
`examples/<name>/internal/...` 아래에 둔다. Sibling example은 Go의 `internal`
visibility rule을 위반하지 않고 해당 package를 import할 수 없다.

기존 package 이동이나 public API surface 확장보다 example-local boundary 보존이
낫기 때문에 기각한다.

## 예제

- 경로: `examples/customer-migration-batch-integration`
- 패키지: `internal/customermigration`
- 실행 entrypoint: `main.go`
- HTTP framework: Gin
- 기본 주소: `127.0.0.1:8095`
- Package dependency focus: `batch`, `leader`, 표준 라이브러리 `context`,
  `net/http`, `sync`, `time`.
- Chunk size: `2`. Restart demo가 simulated crash 이후 checkpoint를
  `NextIndex=2`에 남기도록 예제에서 고정한다.

## 시나리오

Customer migration service는 operations API와 leader-guarded scheduled trigger를
노출한다. 각 실행은 결정적인 customer record를 chunk 단위로 import하고, 작은
checkpoint cursor를 저장하며, transient customer enrichment failure 하나를
retry하고, permanent customer 하나를 dead-letter 처리한다. 또한 simulated writer
crash 이후 완료된 chunk를 재처리하지 않고 restart할 수 있다.

기본 record:

- `cust-1001`: 성공한다.
- `cust-1002`: 성공한다.
- `cust-1003`: 한 번 transient하게 실패한 뒤 retry에서 성공한다.
- `cust-1004`: permanent validation failure. Dead letter를 기록하고 skip된다.
- `cust-1005`: restart 이후 성공한다.

Demo flow에는 눈에 보이는 두 실행이 있다.

1. `crash_after_new_writes=3`을 사용하는 manual API start는 첫 번째 checkpointed
   chunk와 부분 second chunk 이후 실패한다.
2. Leader guard 아래의 scheduler tick은 같은 checkpoint store와 sink로 restart
   run을 시작한다. Checkpoint를 restore하고 실패한 chunk를 replay하며, 이미
   쓰인 boundary customer를 idempotent no-op으로 처리하고, 남은 작업을 끝낸 뒤
   final checkpoint를 보고한다.

## 도메인 모델

Customer source:

```go
type CustomerRecord struct {
    Index    int
    ID       string
    Email    string
    Segment  string
    Scenario FailureScenario
    Reason   string
}
```

Migrated customer:

```go
type MigratedCustomer struct {
    ID       string `json:"id"`
    Email    string `json:"email"`
    Segment  string `json:"segment"`
    Attempts int    `json:"attempts"`
}
```

Checkpoint:

```go
type Checkpoint struct {
    NextIndex int `json:"next_index"`
}
```

Dead letter:

```go
type DeadLetter struct {
    CustomerID string `json:"customer_id"`
    Reason     string `json:"reason"`
    Attempts   int    `json:"attempts"`
}
```

## Batch Engine 계약

- `RunBatch(ctx, options)`는 `batch.Step[CustomerRecord, MigratedCustomer]`를
  만들고 이를 `batch.Job`으로 wrap한다.
- Reader는 `batch.CheckpointReader`를 구현한다.
- Processor는 record를 검증하고 email/segment를 정규화하며
  `ErrTransientCustomer`만 retry하고 `ErrPermanentCustomer`만 dead-letter 처리한다.
- Writer는 migrated customer를 customer ID 기준으로 idempotent하게 저장하고,
  설정된 new write count 이후 crash를 simulation할 수 있다.
- Replay boundary의 duplicate write는 치명적인 duplicate error가 아니라
  결정적 no-op이다. 응답은 duplicate skip count를 별도로 기록하므로 batch를
  실패시키지 않고 restart 동작을 보여줄 수 있다.
- Checkpoint store는 `batch.CheckpointStore`를 통해 교체 가능하다. 예제는
  in-memory recording store를 사용한다.
- `batch.Step`은 `ChunkSize: 2`로 설정한다. Checkpoint/replay 시나리오가 이
  값에 의존하므로 테스트가 기본값을 증명해야 한다.
- 모든 reader, processor, writer, store, scheduler, handler 경로는
  `context.Context`를 확인하거나 전파한다.
- Caller-owned `context.Canceled`와 `context.DeadlineExceeded`는 retry하지
  않는다.
- Retry policy는 `batch.RetryErrors(3, errors.Is(err, ErrTransientCustomer))`로
  고정한다. 이 워크숍 예제에서는 backoff나 sleep을 사용하지 않는다. Processor
  작업 전 또는 중 cancellation은 `batch.StatusCancelled`를 반환하고 retry하지
  않는다.
- Skip policy는 `batch.SkipErrors(2, errors.Is(err, ErrPermanentCustomer))`로
  고정한다. Skip exhaustion은 run을 실패시키고 안정적인 `permanent_customer`
  diagnostic code를 노출한다.

Sentinel error:

- `ErrInvalidCustomer`
- `ErrTransientCustomer`
- `ErrPermanentCustomer`
- `ErrWriterCrash`
- `ErrDuplicateCustomer`
- `ErrInvalidCheckpoint`
- `ErrRunInProgress`
- `ErrNotLeader`
- `ErrInvalidRunID`
- `ErrInvalidCrashAfter`

Domain logic에서 반환하는 error는 `errors.Is`를 위해 sentinel을 `%w`로 wrap한다.

## Operations Service State 계약

`Service`는 run lifecycle과 모든 public snapshot을 소유한다.

- `mu sync.Mutex`
- `active bool`
- 자체 lock과 snapshot API를 가진 shared checkpoint store
- 자체 lock과 snapshot API를 가진 migrated-customer sink
- 자체 lock과 snapshot API를 가진 dead-letter store
- latest run response
- latest report projection
- run 진행 중의 active run cancel function

Run entrypoint(`StartManual`, `RunScheduledTick`)는 `mu`를 획득하고, `active`가
true이면 거부한다. 그런 다음 run-scoped cancelable context를 만들고
`active=true`로 설정하며 cancel function을 저장한 뒤, batch 실행 동안 lock을
해제한다. `defer`는 다시 `mu`를 획득해 defensive-copy snapshot을 저장하고,
cancel function을 지우며, `active=false`로 설정하고, cancellation 또는 failure
상황에서도 latest report를 기록한다.

Read entrypoint(`Status`, `Report`)는 immutable latest projection과 high-level
active state의 defensive copy를 반환하는 데 필요한 시간 동안만 `mu`를 획득한다.
Store의 locked snapshot API를 사용하지 않는 한, batch가 store를 변경하는 동안
live store internal을 읽으면 안 된다. HTTP DTO는 aliased slice, map, checkpoint
값, dead-letter 값, migrated-customer 값을 절대 노출하지 않는다.

`CancelActiveRun`은 현재 active manual run 또는 scheduled run을 취소하고 안정적인
cancellation response를 반환한다. Active run이 없으면 cancel 요청 전용
`no_active_run` error와 함께 `404 Not Found`를 반환한다.

테스트는 concurrent manual start, scheduled tick, status, report request가
결정적으로 겹치도록 latchable writer 또는 runner hook을 사용해야 한다. 최소
8 goroutine과 goroutine당 25 iteration으로 bounded stress test를 추가해 mixed
manual start, scheduled tick, status, report, cancel access를 반복하고, active
run이 최대 하나임을 증명하며, public snapshot이 유효함을 증명한 뒤 같은 package를
`go test -race`로 다시 실행한다.

## Operations API Contract

### `GET /healthz`

Returns `200 OK` with `{"status":"ok"}`.

### `POST /batch/start`

Request:

```json
{
  "run_id": "manual-001",
  "crash_after_new_writes": 3
}
```

Behavior:

- Validates `run_id` as 1..64 characters of `A-Z`, `a-z`, `0-9`, `_`, `.`, and
  `-` after trimming whitespace.
- `crash_after_new_writes` is optional. Omitted or zero means no crash. Positive
  values must be in `1..len(default records)`. Negative or out-of-range values
  return `400 Bad Request`.
- Rejects a second active run with `409 Conflict`.
- Starts and completes the local batch synchronously for deterministic workshop
  output.
- Returns status `200 OK` for completed runs, `409 Conflict` for failed runs,
  and `408 Request Timeout` for caller cancellation.
- All JSON POST handlers share one capped decoder path. They cap request bodies
  at 8 KiB and return `413 Request Entity Too Large` for oversized requests.

### `POST /batch/schedule/tick`

Request:

```json
{
  "run_id": "scheduled-001"
}
```

Behavior:

- Checks leadership through the configured leader gate before running.
- If leadership is held, starts one batch run and returns the same response
  shape as `/batch/start`.
- If leadership is missing, returns `409 Conflict` with a stable
  `not_leader` error body.
- Does not loop or sleep; tests and README curls trigger exactly one
  deterministic tick.
- Uses the same 8 KiB capped JSON decoder and oversized-body error mapping as
  `/batch/start`.

### `POST /batch/cancel`

Request:

```json
{
  "reason": "operator requested stop"
}
```

Behavior:

- Cancels the active run, if one exists.
- Returns `202 Accepted` as a successful cancellation-request acknowledgement
  with `status="cancel_requested"` and no `error_code`.
- Returns `404 Not Found` with `no_active_run` when no run is active.
- Uses the same 8 KiB capped JSON decoder and oversized-body error mapping as
  the other POST handlers.

### `GET /batch/status`

Returns latest run status, checkpoint, migrated customer IDs, dead letters,
leader state, rejection code, and whether a run is currently active.

### `GET /batch/report`

Returns the latest timestamp-free batch report projection. If no run exists,
returns `404 Not Found`.

## Scheduler and Leader Contract

The example uses a tiny acquisition-oriented `LeaderGate` interface around the
existing `leader.Elector` shape:

```go
type LeaderGate interface {
    RunIfLeader(ctx context.Context, run func(context.Context) (RunResponse, error)) (RunResponse, error)
    LeaderHeld() bool
}
```

The production-shaped adapter can wrap `leader.Elector` by calling
`Campaign(ctx)`, running the callback only while leadership is held, and
resigning with a bounded cleanup context, such as `context.WithTimeout` with a
small fixed timeout over `context.WithoutCancel(ctx)` or an equivalent bounded
cleanup context that survives caller cancellation. `leader.ErrAlreadyLeader` is
treated as held leadership and runs the callback without taking ownership of a
newly acquired leadership lease; the adapter resigns only when this call
successfully acquired leadership. Resign failure is recorded in the response
diagnostics but does not hide the original batch result. Tests use a
deterministic in-memory gate that records campaign and resign counts and reports
held/missing state through `LeaderHeld`.

The scheduler owns a tiny injectable ticker loop for the runnable demo. It
executes one leader-guarded tick per interval until its context is canceled, and
tests use a manual ticker channel to avoid sleeps. The scheduler owns no durable
queue semantics. The explicit `/batch/schedule/tick` endpoint remains for
deterministic curl and test control.

Required tests:

- held leadership runs one tick;
- `leader.ErrAlreadyLeader` is treated as a runnable held-leadership path
  without an extra resign;
- missing leadership returns `ErrNotLeader` without mutating checkpoint, sink,
  or dead-letter state;
- campaign cancellation preserves `context.Canceled` or
  `context.DeadlineExceeded`, maps HTTP to `408 Request Timeout` with
  `request_cancelled`, does not set `not_leader`, and does not run the batch;
- resign cleanup is bounded, cannot hang indefinitely, and records cleanup
  failure without hiding the batch result;
- request cancellation after leadership acquisition still attempts bounded
  resign cleanup;
- scheduler loop stops on context cancellation and does not sleep in tests.

`GET /batch/status` includes `leader_held` and `last_rejection_code` so the
workshop user can distinguish idle, active, not-leader, and failed-run states.
The runnable `main.go` exposes a deterministic `LEADER_MODE=held|missing`
setting so README users can reproduce both successful scheduled runs and
`not_leader` responses.

## HTTP Trust Boundary and Server Contract

This is a local workshop server, not an authenticated operations plane.

- `main.go` binds to `127.0.0.1:8095` by default.
- `HTTP_ADDR` may override the address only to another loopback bind.
  Non-loopback binds such as `:8095` or `0.0.0.0:8095` are rejected by default
  because the operations API is unauthenticated.
- README files must state that widening the bind address requires an explicit
  trusted-network or authentication boundary.
- The Gin router must call and check `SetTrustedProxies(nil)` and must not trust
  forwarded headers for any security decision.
- The runnable `http.Server` must set `ReadHeaderTimeout`, `ReadTimeout`,
  `WriteTimeout`, and `IdleTimeout`.
- `main.go` must handle SIGINT/SIGTERM and call `Shutdown` with a bounded
  context.
- Tests cover request cancellation at handler level; smoke validation covers
  `go run` startup and graceful termination.

## HTTP Status and Response Shape

Successful and failed batch runs return a stable `runResponse`. HTTP responses
also include `error_code`, `error_message`, and `failed_phase` when the run did
not complete.

```json
{
  "run_id": "scheduled-001",
  "trigger": "schedule",
  "status": "completed",
  "checkpoint": {"next_index": 5},
  "read_ids": ["cust-1003", "cust-1004", "cust-1005"],
  "accepted_ids": ["cust-1003", "cust-1005"],
  "new_written_ids": ["cust-1005"],
  "migrated_ids": ["cust-1001", "cust-1002", "cust-1003", "cust-1005"],
  "dead_letters": [
    {"customer_id": "cust-1004", "reason": "blocked customer", "attempts": 1}
  ],
  "summary": {
    "read_count": 3,
    "write_count": 2,
    "retry_count": 1,
    "skip_count": 1,
    "duplicate_skip_count": 1,
    "failure": false
  },
  "report": {"name": "customer-migration-batch", "status": "completed"}
}
```

Report projection must omit runtime timestamps.

`summary.write_count` follows the upstream `batch.Report.WriteCount` for items
accepted by successful writer calls. `new_written_ids` is the smaller domain
delta for newly inserted customers, and `duplicate_skip_count` explains replayed
boundary records that were accepted as idempotent no-ops.

HTTP DTOs expose customer IDs and counts, not customer email values. Email is
kept inside the internal domain sink and CLI demo output only. Tests assert that
all public HTTP responses, including failed runs and error bodies, do not
include fixture email strings. Public `error_message` values are allowlisted
and must not be raw domain errors.

Stable API error codes:

| Condition | HTTP status | Code |
|---|---:|---|
| malformed JSON | 400 | `invalid_request` |
| invalid `run_id` | 400 | `invalid_run_id` |
| invalid `crash_after_new_writes` | 400 | `invalid_crash_after_new_writes` |
| oversized body | 413 | `request_too_large` |
| run already active | 409 | `run_in_progress` |
| scheduled tick without leadership | 409 | `not_leader` |
| simulated writer crash | 409 | `writer_crash` |
| retry exhaustion | 409 | `transient_customer_exhausted` |
| skip exhaustion | 409 | `permanent_customer` |
| no latest report | 404 | `report_not_found` |
| cancel without active run | 404 | `no_active_run` |
| caller cancellation/deadline or canceled run result | 408 | `request_cancelled` |

## Documentation

Add English and Korean README files with:

- example scenario;
- run command and curl smoke commands for manual crash, status, report,
  leader-held scheduled restart, active-run cancel, not-leader rejection,
  malformed JSON, blank run ID, and oversized body;
- checkpoint key and chunk size;
- API endpoint table;
- leader-guarded scheduler explanation;
- retry/dead-letter policy table;
- restart contract and duplicate boundary behavior;
- relationship to focused examples #41, #73, and #74;
- local runbook notes for Ctrl-C shutdown, port collision, and state reset by
  restarting the in-memory demo process;
- `HTTP_ADDR` loopback-only override and `LEADER_MODE=held|missing` demo
  controls;
- `/healthz` as process liveness only, with `/batch/status` as the operator
  diagnosis/readiness surface for active state, leadership state, checkpoint,
  latest failure, and rejection code;
- production hardening notes for durable checkpoint stores, queues, schedulers,
  database upserts, idempotency keys, auth/trusted-network boundaries, metrics,
  structured run lifecycle logs, and dead-letter replay.

Update root `README.md` and `README.ko.md`:

- example table row;
- 0.5.0 run section;
- roadmap wording if needed.

No new diagrams are required for this pass; the current work closes the
functional milestone gap. Existing root map imagery can be refreshed in a
follow-up if the project wants diagram parity for the new example.

## Tests

Focused tests must cover:

- health endpoint;
- manual start fails on simulated writer crash and leaves checkpoint at the
  previous successful chunk;
- default chunk size is `2`;
- scheduled tick under leadership restarts from checkpoint and completes;
- tiny scheduler loop triggers a leader-guarded run and stops on cancellation
  without long sleeps;
- completed first chunk is not reprocessed on restart;
- transient customer retry increments retry count and succeeds;
- permanent customer records exactly one dead letter and increments skip count;
- missing leadership rejects scheduled tick without mutating checkpoint or
  migrated customers;
- status and report endpoints return stable latest-run projections;
- malformed JSON, blank/invalid run IDs, invalid crash counters, and oversized
  request bodies on both POST endpoints return deterministic error responses;
- active-run cancellation endpoint cancels work, clears active state, and maps
  repeated/no-active cancellation to `no_active_run`;
- caller cancellation maps to `408 Request Timeout`;
- cancellation releases the active-run guard;
- concurrent manual start, scheduled tick, cancel, status, and report requests
  allow at most one active run, return defensive snapshots, and never race;
- bounded shared-state stress repeats concurrent manual start, scheduled tick,
  cancel, status, and report access under normal tests and then under the race
  detector;
- all public HTTP response bodies, including status/report, failed runs, and
  error responses, do not expose fixture email values;
- `main.go` constructs an `http.Server` with bounded timeouts and graceful
  shutdown;
- `HTTP_ADDR` accepts loopback binds and rejects non-loopback binds;
- Gin trusted proxies are disabled with `SetTrustedProxies(nil)`;
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`.

## Validation

- `go test -count=1 ./examples/customer-migration-batch-integration/...`
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`
- `go run ./examples/customer-migration-batch-integration`, then execute the
  README curl smoke flow for `/healthz`, manual crash, status, report,
  scheduled restart, cancel, not-leader, malformed JSON, blank run ID, and
  oversized body
- `go test -p 1 ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `GOFLAGS=-p=1 make ci`
- `git diff --check`

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Architecture pre-design ran or skipped | Done | Local pre-design selected one integrated example after comparing three approaches. |
| Step 1-R research incorporated | Done | Issue, GNO, current examples, `go doc`, and root README evidence are listed above. |
| Current-behavior claims cite evidence | Done | Each major dependency and existing pattern cites current files or command evidence. |
| Spec path confirmed inside worktree | Done | This file lives under `.worktrees/feat-issue-29-batch-integration/docs/superpowers/specs/`. |
| Risks/failure modes included | Done | Cancellation, retry/dead-letter, checkpoint replay, leader missing, and active-run conflict are explicit. |
| Approach comparison included | Done | Approaches A/B/C compared and B/C rejected with repository rationale. |
| Brainstorming process | Done | User gave concrete "작업하자" execution direction and AGENTS autonomy forbids permission handoff; material design is captured here instead of stopping for approval. |
| Go pattern compliance | Done | Context, sentinel errors, race/stress, Gin boundaries, and README impact are specified. |
| Open questions resolved | Done | No blocking ambiguity remains; durable infrastructure and diagrams are explicitly out of scope. |
