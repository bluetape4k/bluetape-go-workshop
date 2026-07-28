# Customer Migration Batch Integration 구현 계획

> **agentic worker 대상:** 필수 sub-skill: 이 계획은 task-by-task 구현을 위해 superpowers:subagent-driven-development(권장) 또는 superpowers:executing-plans를 사용한다. 단계 추적에는 checkbox(`- [ ]`) 문법을 사용한다.

**목표:** `examples/customer-migration-batch-integration`을 runnable v0.5.0 workshop 예제로 추가한다. 이 예제는 checkpoint restart, retry/dead-letter behavior, Gin operations API, cancel operation, leader-guarded scheduled tick/loop를 조합하며 PR text는 #42, #43, #75, umbrella #29를 닫을 수 있어야 한다.

**Architecture:** example-local `internal/customermigration` package 하나만 유지한다. batch engine은 source record, checkpoint replay, retry/skip policy, idempotent sink write, fixed chunk size `2`, report projection을 소유한다. service는 `sync.Mutex` 뒤의 run lifecycle, active-run guarding, defensive public snapshot, cancelation, leader gating, scheduler loop orchestration, Gin handler를 소유한다. checkpoint/sink/dead-letter store는 각자 lock을 소유하고 race-safe active-run read를 위한 snapshot API를 노출한다. `main.go`는 local loopback server configuration, demo leader mode, timeout, signal handling, graceful shutdown만 연결한다.

**Tech Stack:** Go, `github.com/bluetape4k/bluetape-go/batch`, `github.com/bluetape4k/bluetape-go/leader`, Gin, standard-library `context`, `errors`, `net/http`, `sync`, `time`, `os/signal`, existing repo Make target. 새 dependency는 없다.

---

## 제약

- 모든 Go code task에는 `$bluetape-go-patterns`를 적용한다: context-first API, sentinel error wrapping, race-safe shared state, table-driven test, `gofmt`, package-level mutable state 금지.
- example-local boundary를 보존한다. sibling example `internal` package를 import하지 않는다.
- 각 behavioral surface마다 implementation 전에 test를 작성한다.
- HTTP DTO는 ID/count 기반으로 유지한다. `/batch/status`와 `/batch/report`는 fixture email value를 노출하면 안 된다.
- server는 기본적으로 local-only다: `127.0.0.1:8095`. `HTTP_ADDR` override는 loopback bind만 허용하고, 기본적으로 non-loopback bind는 거부한다. security decision에 forwarded header를 신뢰하지 않는다.
- repo가 이미 사용하는 bilingual 방식에 맞춰 example README pair를 추가하고 root `README.md` / `README.ko.md`를 갱신한다.
- 이번 pass에서는 diagram, infrastructure service, queue, database, Redis, NATS, 새 scheduler framework, 새 dependency를 추가하지 않는다.

## 계획 파일

- `examples/customer-migration-batch-integration/main.go`
- `examples/customer-migration-batch-integration/main_test.go`
- `examples/customer-migration-batch-integration/README.md`
- `examples/customer-migration-batch-integration/README.ko.md`
- `examples/customer-migration-batch-integration/internal/customermigration/engine.go`
- `examples/customer-migration-batch-integration/internal/customermigration/engine_test.go`
- `examples/customer-migration-batch-integration/internal/customermigration/service.go`
- `examples/customer-migration-batch-integration/internal/customermigration/service_test.go`
- `examples/customer-migration-batch-integration/internal/customermigration/scheduler.go`
- `examples/customer-migration-batch-integration/internal/customermigration/scheduler_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-customer-migration-batch-integration.md`
- `docs/review/2026-06-22-issue-29-batch-integration-code-review.md`

## 구현 작업

- [ ] **A. Engine TDD red test [complexity: medium]**
  - 다음을 다루는 `engine_test.go`를 만든다.
    - `CrashAfterNewWrites: 3`을 사용한 manual run이 `ErrWriterCrash`로 실패한다.
    - failure가 checkpoint를 `NextIndex: 2`에 남긴다.
    - default engine이 fixed `ChunkSize: 2`를 사용한다.
    - partial second chunk가 `cust-1003`을 write한다.
    - restart run이 checkpoint에서 재개되고 replay된 `cust-1003`을 duplicate no-op으로 받아들이며, `cust-1005`를 write하고 `cust-1004`를 dead-letter 처리한 뒤 `NextIndex: 5`에서 완료된다.
    - transient `cust-1003`은 retry count를 증가시키고 성공한다.
    - permanent `cust-1004`는 정확히 하나의 dead letter를 기록하고 skip count를 증가시킨다.
    - invalid checkpoint와 caller cancellation은 stable sentinel/diagnostic behavior를 반환한다.
    - report projection에는 timestamp와 email value가 없다.
  - `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`를 실행하고 예상 compile/fail output을 TDD evidence로 보존한다.

- [ ] **B. Engine implementation [complexity: medium]**
  - default record, `FailureScenario`, sentinel error, `RunOptions`, `RunResponse`, `Summary`, report DTO, in-memory checkpoint store, sink, dead-letter store, reader, processor, writer, `RunBatch`를 구현한다.
  - checkpoint store, sink, dead-letter store는 각자 lock과 defensive snapshot API를 가진다. status/report는 `Service.mu` 아래 immutable latest projection을 읽을 수 있고, active 중 live store state가 필요하면 locked snapshot API를 사용해야 한다.
  - `batch.Step` with `ChunkSize: 2`, `batch.Job`, `batch.CheckpointReader`, `batch.RetryErrors(3, ...)`, `batch.SkipErrors(2, ...)`를 사용한다.
  - writer는 customer ID 기준 idempotent로 만든다. `accepted_ids`, `new_written_ids`, `migrated_ids`, `duplicate_skip_count`를 upstream report `WriteCount`와 별도로 추적한다.
  - `context.Context`를 전파한다. caller cancellation/deadline을 retry하지 않는다.
  - `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`를 실행한다.

- [ ] **C. Service and HTTP TDD red test [complexity: high]**
  - 다음을 다루는 `service_test.go`를 만든다.
    - `GET /healthz`.
    - `POST /batch/start` crash response, stable status code, latest status snapshot.
    - held leadership 아래 `POST /batch/schedule/tick`이 restart 후 완료한다.
    - missing leadership은 checkpoint, sink, dead letter를 mutate하지 않고 `not_leader`를 반환한다.
    - status/report가 stable latest-run projection을 노출하고 email을 생략한다.
    - success, crash, not-leader, malformed JSON, cancellation, no-active cancel, report-not-found path의 모든 public response body가 fixture email string을 생략하고 allowlisted public error message를 사용한다.
    - 두 POST endpoint에서 malformed JSON, blank/invalid run ID, invalid crash counter, oversized body가 stable error code를 반환한다.
    - latest report가 없으면 `report_not_found`를 반환한다.
    - `POST /batch/cancel`은 active work를 cancel하고 `status=\"cancel_requested\"`, `error_code` 없음으로 `202 Accepted`를 반환하며, run이 cancellation을 관찰한 뒤 active state를 정리하고, active run이 없을 때 반복 호출하면 `no_active_run`을 반환한다.
    - request cancellation은 `request_cancelled`로 매핑된다.
    - cancellation은 active-run guard를 해제한다.
    - concurrent manual start, scheduled tick, cancel, status, report request는 active run을 최대 하나만 허용하고 defensive snapshot을 반환한다.
    - bounded mixed-access stress는 최소 8 goroutine과 goroutine당 25 iteration을 사용하고, active run이 최대 하나인지 assert하며, handler-visible checkpoint/sink/dead-letter snapshot path를 normal 및 race test에서 반복한다.
    - deterministic leader case는 held, missing, `leader.ErrAlreadyLeader`, acquisition 전 campaign cancellation, acquisition 뒤 request cancellation, resign timeout, resign failure diagnostic을 다룬다. campaign cancellation은 `context.Canceled`/`context.DeadlineExceeded`를 보존하고, HTTP를 `request_cancelled`로 매핑하며, `not_leader` 및 batch mutation을 피해야 한다.
  - test가 active work를 sleep 없이 overlap하도록 latchable runner hook을 사용한다.
  - `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`를 실행한다.

- [ ] **D. Service, leader gate, Gin implementation [complexity: high]**
  - `mu`, `active`, active-run cancel function, shared locked checkpoint store, locked sink, locked dead-letter store, latest response, latest report, `last_rejection_code`, deterministic test hook을 가진 `Service`를 구현한다.
  - `StartManual`, `RunScheduledTick`, `CancelActiveRun`, `Status`, `Report`, validation helper, defensive-copy helper, allowlisted public error message, stable error mapping을 구현한다.
  - `RunIfLeader`와 `LeaderHeld`를 가진 `LeaderGate`를 구현한다. test용 deterministic in-memory gate와 `leader.Elector` 주변의 production-shaped adapter를 포함한다.
  - `leader.ErrAlreadyLeader`는 newly acquired lease ownership 없이 held leadership으로 취급한다. 이 call이 leadership을 acquire한 경우에만 resign한다.
  - `context.WithoutCancel(ctx)` 또는 동등한 bounded cleanup context 위의 fixed resign cleanup timeout을 사용해 caller cancellation 뒤에도 cleanup을 시도하고 무한 대기하지 않게 한다.
  - `gin.New`, recovery middleware, trusted forwarded header 없음, capped 8 KiB JSON body decoding, shared error response writer, 다음 route를 가진 `NewRouter`를 구현한다.
    - `GET /healthz`
    - `POST /batch/start`
    - `POST /batch/schedule/tick`
    - `POST /batch/cancel`
    - `GET /batch/status`
    - `GET /batch/report`
  - `router.SetTrustedProxies(nil)`를 호출하고 확인한다. handler가 client IP를 노출하면 forwarded-header spoofing regression test를 추가한다.
  - `go test -count=1 ./examples/customer-migration-batch-integration/internal/customermigration`를 실행한다.

- [ ] **E. Scheduler and main entrypoint TDD/implementation [complexity: medium]**
  - injectable ticker loop가 leader-guarded run을 trigger하고 context cancellation에서 멈추며 sleep 없는 manual test tick을 사용하는지 증명하는 `scheduler_test.go`를 만든다.
  - `Service.RunScheduledTick` 주변의 작은 scheduler loop를 구현한다. durable queue semantic은 범위 밖으로 유지한다.
  - `newHTTPServer`가 `Addr`, handler, `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`를 설정하는지 검증하는 `main_test.go`를 만든다.
  - default `127.0.0.1:8095`, loopback `HTTP_ADDR`, invalid `HTTP_ADDR`, rejected non-loopback `HTTP_ADDR`용 bind parsing test를 추가한다.
  - default bind `127.0.0.1:8095`, loopback-only `HTTP_ADDR`, `LEADER_MODE=held|missing`, `http.Server`, SIGINT/SIGTERM handling, bounded graceful shutdown이 있는 `main.go`를 구현한다.
  - `go test -count=1 ./examples/customer-migration-batch-integration/...`를 실행한다.

- [ ] **F. Documentation [complexity: medium]**
  - English/Korean example README를 추가하고 scenario, run command, manual crash/status/report/leader-held scheduled restart/active-run cancel/not-leader rejection/malformed JSON/blank run ID/oversized body용 curl command를 포함한다.
  - checkpoint key, chunk size, endpoint table, process liveness only인 `/healthz`, operator diagnosis/readiness인 `/batch/status`, leader-guarded tick 및 tiny scheduler loop, retry/dead-letter policy, restart duplicate boundary behavior, focused #41/#73/#74 example과의 관계, Ctrl-C shutdown, port collision, loopback-only `HTTP_ADDR`, `LEADER_MODE=held|missing`, process restart reset, production hardening을 문서화한다.
  - root `README.md`와 `README.ko.md` example table 및 v0.5.0 run section을 갱신한다.

- [ ] **G. Focused verification [complexity: medium]**
  - 다음을 실행한다.
    - `go test -count=1 ./examples/customer-migration-batch-integration/...`
    - `go test -race -count=1 ./examples/customer-migration-batch-integration/...`
    - `go run ./examples/customer-migration-batch-integration`, 이후 `/healthz`, manual crash, `/batch/status`, `/batch/report`, `/batch/schedule/tick`, `/batch/cancel`, `LEADER_MODE=missing` not-leader, malformed JSON, blank run ID, oversized body에 대한 README curl smoke flow를 실행한다. 적용 가능한 곳에는 expected HTTP status와 `error_code`를 기록한다.
    - 실행 중인 process를 SIGTERM/SIGINT로 종료하고 configured shutdown timeout plus small margin 안에 exit하는지 대기하며, active run이 cancel되었는지 finish되었는지 기록한다.
  - broader verification 전에 실패를 수정한다.

- [ ] **H. Cleanup pass [complexity: medium]**
  - implementation이 세 파일 넘게 건드릴 것이므로 cleanup 편집 전에 working note에 짧은 cleanup checklist를 쓴다.
  - 새 abstraction보다 deletion 및 consolidation을 선호한다. workshop user가 읽기 쉬운 example code를 유지한다.
  - cleanup 뒤 focused test를 다시 실행한다.

- [ ] **I. Performance/stability scan [complexity: medium]**
  - long sleep, unbounded goroutine, unbounded body read, leaked context, lock-held batch execution, caller-canceled resign cleanup이 없는지 확인한다.
  - concurrency/stress test가 active run 중 status/report snapshot을 exercise하는지 확인한다.
  - cleanup이 service state를 바꾸면 package race test를 다시 실행한다.

- [ ] **J. Full repository verification [complexity: high]**
  - 다음을 실행한다.
    - `go test -p 1 ./...`
    - `make fmt-check`
    - `make tidy-check`
    - `make vet`
    - `make lint`
    - `GOFLAGS=-p=1 make ci`
    - `git diff --check`
  - 실패를 정확히 기록하고 범위 안에서 수정한다.

- [ ] **K. Step 6-R code review and fixes [complexity: high]**
  - `bluetape4k-full-feature` 기준 six-lane code review plus integration review를 실행한다.
  - `docs/review/2026-06-22-issue-29-batch-integration-code-review.md`를 저장한다.
  - 모든 P0/P1 finding을 수정하고 affected review lane 및 relevant test를 다시 실행한다.

- [ ] **L. Lesson, commit, PR [complexity: medium]**
  - pitfall, command, review finding, follow-up risk가 있는 `docs/lessons/2026-06-22-customer-migration-batch-integration.md`를 추가한다.
  - Lore protocol로 commit한다.
  - branch를 push하고 `Closes #29`, `Closes #42`, `Closes #43`, `Closes #75`가 있는 PR을 만든다.
  - GitHub가 허용하면 linked issue set과 맞춰 PR assignee, milestone `0.5.0`, label을 설정한다.
  - PR body는 required `## DoD Status` section으로 끝낸다.

## 인수 조건 매핑

| Spec requirement | Plan coverage |
|---|---|
| checkpoint crash/restart 및 duplicate replay | A, B, C, D, G |
| Gin operations API | C, D, F, G |
| active-run cancel operation | C, D, F, G |
| leader-guarded scheduled tick 및 scheduler loop | C, D, E, F |
| retry/dead-letter policy | A, B, F |
| shared state race safety | C, D, G, I, J |
| local HTTP trust boundary 및 timeout | D, E, F, G |
| error code stability | C, D, F |
| 모든 public HTTP response에서 email 노출 없음 | A, C, D, F |
| bilingual docs 및 root README update | F |
| 새 dependency 없음 | B, D, J |
| full verification 및 review | G, H, I, J, K |

## 위험 가정

- 예제는 process-local 및 in-memory다. process restart는 checkpoint, migrated sink, dead letter를 설계상 reset한다.
- leader adapter는 production-shaped이지만 workshop example은 distributed coordinator를 시작하지 않으므로 test는 deterministic fake를 사용한다.
- repository에는 Testcontainers example이 포함되어 있으므로 `GOFLAGS=-p=1 make ci`는 focused package check보다 오래 걸릴 수 있다. targeted check가 통과한 뒤 실행한다.
- operations API는 unauthenticated이므로 non-loopback HTTP bind는 기본적으로 거부한다. production exposure에는 별도의 auth/trusted-network design이 필요하다.
- 사용자가 review/CI 뒤 merge를 명시적으로 요청하지 않는 한 PR merge는 범위 밖이다.
