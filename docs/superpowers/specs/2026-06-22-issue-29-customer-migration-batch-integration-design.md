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

## Operations API 계약

### `GET /healthz`

`{"status":"ok"}`와 함께 `200 OK`를 반환한다.

### `POST /batch/start`

요청:

```json
{
  "run_id": "manual-001",
  "crash_after_new_writes": 3
}
```

동작:

- 공백 trim 이후 `run_id`를 `A-Z`, `a-z`, `0-9`, `_`, `.`, `-`로 이루어진
  1..64자 값으로 검증한다.
- `crash_after_new_writes`는 선택 사항이다. 생략하거나 0이면 crash가 없다.
  양수 값은 `1..len(default records)` 안에 있어야 한다. 음수 또는 범위 밖 값은
  `400 Bad Request`를 반환한다.
- 두 번째 active run은 `409 Conflict`로 거부한다.
- 결정적인 워크숍 출력을 위해 local batch를 동기적으로 시작하고 완료한다.
- Completed run은 `200 OK`, failed run은 `409 Conflict`, caller cancellation은
  `408 Request Timeout` status를 반환한다.
- 모든 JSON POST handler는 하나의 capped decoder path를 공유한다. Request body는
  8 KiB로 제한하며, 초과 요청은 `413 Request Entity Too Large`를 반환한다.

### `POST /batch/schedule/tick`

요청:

```json
{
  "run_id": "scheduled-001"
}
```

동작:

- 실행 전에 설정된 leader gate를 통해 leadership을 확인한다.
- Leadership이 held 상태이면 batch run 하나를 시작하고 `/batch/start`와 같은
  response shape를 반환한다.
- Leadership이 없으면 안정적인 `not_leader` error body와 함께 `409 Conflict`를
  반환한다.
- Loop 또는 sleep하지 않는다. 테스트와 README curl은 정확히 하나의 deterministic
  tick을 trigger한다.
- `/batch/start`와 같은 8 KiB capped JSON decoder 및 oversized-body error
  mapping을 사용한다.

### `POST /batch/cancel`

요청:

```json
{
  "reason": "operator requested stop"
}
```

동작:

- Active run이 있으면 취소한다.
- 성공적인 cancellation-request acknowledgement로 `status="cancel_requested"`와
  `error_code` 없이 `202 Accepted`를 반환한다.
- Active run이 없으면 `no_active_run`과 함께 `404 Not Found`를 반환한다.
- 다른 POST handler와 같은 8 KiB capped JSON decoder 및 oversized-body error
  mapping을 사용한다.

### `GET /batch/status`

Latest run status, checkpoint, migrated customer ID, dead letter, leader state,
rejection code, 현재 active run 여부를 반환한다.

### `GET /batch/report`

Latest timestamp-free batch report projection을 반환한다. Run이 없으면
`404 Not Found`를 반환한다.

## Scheduler 및 Leader 계약

예제는 기존 `leader.Elector` 형태 주변에 작은 acquisition-oriented `LeaderGate`
interface를 사용한다.

```go
type LeaderGate interface {
    RunIfLeader(ctx context.Context, run func(context.Context) (RunResponse, error)) (RunResponse, error)
    LeaderHeld() bool
}
```

Production-shaped adapter는 `Campaign(ctx)`를 호출하고, leadership이 held인 동안만
callback을 실행하며, caller cancellation 이후에도 살아남는 bounded cleanup
context로 resign함으로써 `leader.Elector`를 wrap할 수 있다. 예를 들어
`context.WithoutCancel(ctx)` 위에 작은 고정 timeout의 `context.WithTimeout`을
사용하거나 동등한 bounded cleanup context를 사용할 수 있다.
`leader.ErrAlreadyLeader`는 held leadership으로 취급하고 새로 획득한 leadership
lease ownership 없이 callback을 실행한다. Adapter는 이 호출이 leadership을
성공적으로 획득했을 때만 resign한다. Resign failure는 response diagnostic에
기록하지만 원래 batch result를 숨기지 않는다. 테스트는 campaign 및 resign count를
기록하고 `LeaderHeld`로 held/missing state를 보고하는 결정적 in-memory gate를
사용한다.

Scheduler는 실행 가능한 demo를 위한 작은 injectable ticker loop를 소유한다.
Context가 취소될 때까지 interval마다 leader-guarded tick 하나를 실행하며, 테스트는
sleep을 피하기 위해 manual ticker channel을 사용한다. Scheduler는 durable queue
semantics를 소유하지 않는다. 명시적인 `/batch/schedule/tick` endpoint는 결정적
curl과 테스트 제어를 위해 유지한다.

필수 테스트:

- held leadership이 tick 하나를 실행한다.
- `leader.ErrAlreadyLeader`는 추가 resign 없는 runnable held-leadership 경로로
  취급된다.
- missing leadership은 checkpoint, sink, dead-letter state를 변경하지 않고
  `ErrNotLeader`를 반환한다.
- campaign cancellation은 `context.Canceled` 또는 `context.DeadlineExceeded`를
  보존하고, HTTP를 `request_cancelled`와 함께 `408 Request Timeout`으로 mapping하며,
  `not_leader`를 설정하지 않고 batch를 실행하지 않는다.
- resign cleanup은 bounded이고 무한히 hang될 수 없으며, batch result를 숨기지
  않고 cleanup failure를 기록한다.
- leadership acquisition 이후 request cancellation이 발생해도 bounded resign
  cleanup을 시도한다.
- scheduler loop는 context cancellation에서 멈추고 테스트에서 sleep하지 않는다.

`GET /batch/status`는 `leader_held`와 `last_rejection_code`를 포함하므로 워크숍
사용자는 idle, active, not-leader, failed-run 상태를 구분할 수 있다. 실행 가능한
`main.go`는 결정적인 `LEADER_MODE=held|missing` 설정을 노출해 README 사용자가
성공한 scheduled run과 `not_leader` 응답을 모두 재현할 수 있게 한다.

## HTTP Trust Boundary 및 Server 계약

이것은 local workshop server이며 authenticated operations plane이 아니다.

- `main.go`는 기본적으로 `127.0.0.1:8095`에 bind한다.
- `HTTP_ADDR`는 다른 loopback bind로만 주소를 override할 수 있다.
  Operations API가 unauthenticated이므로 `:8095` 또는 `0.0.0.0:8095` 같은
  non-loopback bind는 기본적으로 거부한다.
- README 파일은 bind address 확장에 명시적인 trusted-network 또는 authentication
  boundary가 필요하다고 설명해야 한다.
- Gin router는 `SetTrustedProxies(nil)`을 호출하고 확인해야 하며, 어떤 security
  decision에서도 forwarded header를 신뢰하면 안 된다.
- 실행 가능한 `http.Server`는 `ReadHeaderTimeout`, `ReadTimeout`,
  `WriteTimeout`, `IdleTimeout`을 설정해야 한다.
- `main.go`는 SIGINT/SIGTERM을 처리하고 bounded context로 `Shutdown`을 호출해야
  한다.
- 테스트는 handler level request cancellation을 다룬다. Smoke validation은
  `go run` startup과 graceful termination을 다룬다.

## HTTP Status 및 Response Shape

Successful 및 failed batch run은 안정적인 `runResponse`를 반환한다. Run이 완료되지
않았을 때 HTTP response에는 `error_code`, `error_message`, `failed_phase`도
포함된다.

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

Report projection은 runtime timestamp를 생략해야 한다.

`summary.write_count`는 성공한 writer call이 accept한 item에 대한 upstream
`batch.Report.WriteCount`를 따른다. `new_written_ids`는 새로 insert된 customer에
대한 더 작은 domain delta이며, `duplicate_skip_count`는 idempotent no-op으로
accept된 replayed boundary record를 설명한다.

HTTP DTO는 customer email 값이 아니라 customer ID와 count를 노출한다. Email은
internal domain sink와 CLI demo output 안에만 유지한다. 테스트는 failed run과
error body를 포함한 모든 public HTTP response가 fixture email string을 포함하지
않음을 assert한다. Public `error_message` 값은 allowlist되어야 하며 raw domain
error이면 안 된다.

안정적인 API error code:

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

## 문서

영어 및 한국어 README 파일에 다음을 추가한다.

- example scenario
- manual crash, status, report, leader-held scheduled restart, active-run
  cancel, not-leader rejection, malformed JSON, blank run ID, oversized body를
  위한 run command 및 curl smoke command
- checkpoint key와 chunk size
- API endpoint table
- leader-guarded scheduler 설명
- retry/dead-letter policy table
- restart contract 및 duplicate boundary 동작
- focused example #41, #73, #74와의 관계
- Ctrl-C shutdown, port collision, in-memory demo process 재시작을 통한 state
  reset에 대한 local runbook note
- `HTTP_ADDR` loopback-only override 및 `LEADER_MODE=held|missing` demo control
- `/healthz`는 process liveness 전용이며, `/batch/status`는 active state,
  leadership state, checkpoint, latest failure, rejection code에 대한 operator
  diagnosis/readiness surface라는 설명
- durable checkpoint store, queue, scheduler, database upsert, idempotency key,
  auth/trusted-network boundary, metric, structured run lifecycle log,
  dead-letter replay에 대한 production hardening note

루트 `README.md`와 `README.ko.md`를 갱신한다.

- example table row
- 0.5.0 run section
- 필요한 경우 roadmap wording

이번 pass에는 새 다이어그램이 필요하지 않다. 현재 작업은 functional milestone
gap을 닫는다. Project가 새 예제에 대한 diagram parity를 원하면 기존 root map
이미지는 follow-up에서 refresh할 수 있다.

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- health endpoint
- manual start가 simulated writer crash에서 실패하고 checkpoint를 이전 성공
  chunk에 남기는지
- 기본 chunk size가 `2`인지
- leadership 아래 scheduled tick이 checkpoint에서 restart하고 완료되는지
- 작은 scheduler loop가 leader-guarded run을 trigger하고 긴 sleep 없이
  cancellation에서 멈추는지
- 완료된 첫 chunk가 restart에서 다시 처리되지 않는지
- transient customer retry가 retry count를 증가시키고 성공하는지
- permanent customer가 dead letter 하나를 정확히 기록하고 skip count를
  증가시키는지
- missing leadership이 checkpoint 또는 migrated customer를 변경하지 않고
  scheduled tick을 거부하는지
- status 및 report endpoint가 안정적인 latest-run projection을 반환하는지
- 두 POST endpoint에서 malformed JSON, blank/invalid run ID, invalid crash
  counter, oversized request body가 결정적 error response를 반환하는지
- active-run cancellation endpoint가 work를 취소하고 active state를 지우며,
  repeated/no-active cancellation을 `no_active_run`으로 mapping하는지
- caller cancellation이 `408 Request Timeout`으로 mapping되는지
- cancellation이 active-run guard를 release하는지
- concurrent manual start, scheduled tick, cancel, status, report request가
  active run을 최대 하나만 허용하고 defensive snapshot을 반환하며 race하지
  않는지
- bounded shared-state stress가 normal test와 race detector 아래에서 concurrent
  manual start, scheduled tick, cancel, status, report access를 반복하는지
- status/report, failed run, error response를 포함한 모든 public HTTP response
  body가 fixture email 값을 노출하지 않는지
- `main.go`가 bounded timeout과 graceful shutdown을 갖춘 `http.Server`를
  구성하는지
- `HTTP_ADDR`가 loopback bind를 허용하고 non-loopback bind를 거부하는지
- Gin trusted proxy가 `SetTrustedProxies(nil)`로 비활성화되는지
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`.

## 검증

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

## Step 2 Checklist 완료 보고

| 항목 | 상태 | 메모 |
|---|---|---|
| Architecture pre-design ran or skipped | 완료 | 세 가지 접근을 비교한 뒤 local pre-design에서 하나의 integrated example을 선택했다. |
| Step 1-R research incorporated | 완료 | Issue, GNO, current example, `go doc`, root README 근거를 위에 나열했다. |
| Current-behavior claims cite evidence | 완료 | 각 주요 dependency와 기존 pattern은 current file 또는 command evidence를 인용한다. |
| Spec path confirmed inside worktree | 완료 | 이 파일은 `.worktrees/feat-issue-29-batch-integration/docs/superpowers/specs/` 아래에 있다. |
| Risks/failure modes included | 완료 | Cancellation, retry/dead-letter, checkpoint replay, leader missing, active-run conflict를 명시했다. |
| Approach comparison included | 완료 | Approach A/B/C를 비교했고 B/C는 repository rationale로 기각했다. |
| Brainstorming process | 완료 | 사용자가 구체적인 "작업하자" 실행 방향을 주었고 AGENTS autonomy가 permission handoff를 금지하므로, 승인을 기다리는 대신 material design을 여기에 기록했다. |
| Go pattern compliance | 완료 | Context, sentinel error, race/stress, Gin boundary, README impact를 명시했다. |
| Open questions resolved | 완료 | Blocking ambiguity는 남아 있지 않으며 durable infrastructure와 diagram은 명시적으로 scope 밖이다. |
