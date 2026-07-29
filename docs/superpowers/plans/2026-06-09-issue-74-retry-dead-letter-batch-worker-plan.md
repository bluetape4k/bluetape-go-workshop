# Issue #74 계획: Retry and Dead-Letter Batch Worker 예제

## 범위

- 이슈: #74, `[v0.5.0] Add retry and dead letter batch worker example`
- Spec:
  `docs/superpowers/specs/2026-06-09-issue-74-retry-dead-letter-batch-worker-design.md`
- Research:
  `docs/superpowers/research/2026-06-09-issue-74-retry-dead-letter-batch-worker-research.md`
- Worktree:
  `.worktrees/feat-issue-74-retry-dead-letter-batch-worker`
- 브랜치: `feat/issue-74-retry-dead-letter-batch-worker`

## 순서 제약

- 구현 전에 research, spec, spec review, plan, plan review를 commit한다.
- Go code, test, README example, review gate에는 `bluetape-go-patterns`를 적용한다.
- README diagram generation 및 visual inspection에는 `bluetape4k-diagram`을 적용한다.
- `bluetape-go v0.5.1`의 기존 `batch` API를 사용한다. runtime dependency를
  추가하지 않는다.
- 모든 작업은 feature worktree 안에서 수행한다.

## 작업

### 1. 계획 산출물 커밋

- complexity: low
- expected files:
  - research, spec, spec review, plan, plan review under `docs/superpowers/`
- action:
  - `git diff --check`를 실행한다.
  - Step 4 전에 planning artifact를 Lore-format trailer로 commit한다.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Ticket Worker domain 구현

- complexity: high
- expected files:
  - `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`
- action:
  - `Ticket`, `ProcessedTicket`, `DeadLetter`, result DTO, sentinel error,
    deterministic fixture를 정의한다.
  - `TicketReader`, `TicketProcessor`, `TicketWriter`, in-memory processed sink,
    dead-letter store를 구현한다.
  - transient/permanent/duplicate error에는 `errors.Is`-compatible wrapping을
    사용한다.
  - reader, processor, writer, runner path에서 `context.Context`를 확인한다.
  - output은 deterministic 및 timestamp-free로 유지한다.
- verification:
  - `gofmt -w examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`
  - `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`

### 3. Batch runner 및 main 구현

- complexity: medium
- expected files:
  - `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`
  - `examples/retry-dead-letter-batch-worker/main.go`
- action:
  - chunk size `2`로 `batch.Step[Ticket, ProcessedTicket]`를 구성한다.
  - transient error용 `RetryPolicy`를 `MaxAttempts=3`으로 설정한다.
  - permanent error용 `SkipPolicy`를 `MaxSkips=2`로 설정한다.
  - `ticket-batch-worker`라는 `batch.Job`으로 감싼다.
  - `main.go`에서 indented JSON을 출력한다.
- verification:
  - `go run ./examples/retry-dead-letter-batch-worker`
  - `go test -run '^$' ./examples/retry-dead-letter-batch-worker`

### 4. 집중 테스트 추가

- complexity: high
- expected files:
  - `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker_test.go`
- action:
  - completed demo count, processed order, retry count, skip count,
    dead-letter content를 테스트한다.
  - attempt evidence와 함께 retry 뒤 transient success를 테스트한다.
  - permanent failure가 dead-letter 하나를 기록하고 skipped되는지 테스트한다.
  - skip budget exhaustion이 실패하고 `ErrPermanentTicket`을 wrap하는지 테스트한다.
  - duplicate writer failure가 dead-letter로 변환되지 않는지 테스트한다.
  - 작업 전 및 retry 중 cancellation을 테스트한다.
  - report projection이 runtime timestamp를 생략하는지 테스트한다.
- verification:
  - `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`
  - `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`

### 5. README 다이어그램 생성

- complexity: medium
- expected files:
  - `scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.dot`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.plain`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*-graphviz.svg`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*-graphviz.png`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.svg`
  - `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*.png`
- action:
  - scenario, Architecture, Sequence Diagram asset을 만든다.
  - decorated workshop baseline과 semantic route color를 보존한다.
  - concrete margin을 포함한 geometry summary를 출력한다.
  - commit/PR 전에 각 rendered PNG를 점검한다.
- verification:
  - `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
  - `view_image`를 사용한 rendered PNG inspection
  - `git diff --check`

### 6. 예제 README pair 작성

- complexity: medium
- expected files:
  - `examples/retry-dead-letter-batch-worker/README.md`
  - `examples/retry-dead-letter-batch-worker/README.ko.md`
- action:
  - language switch, scenario, run command, policy table, sample output,
    Architecture, Sequence Diagram, production hardening note를 추가한다.
  - nearby concept로 #40 및 #73을 link한다.
  - PNG diagram만 embed한다.
- verification:
  - `rg -n "Example Scenario|Architecture|Sequence Diagram|retry|dead-letter" examples/retry-dead-letter-batch-worker/README.md`
  - `rg -n "예제 시나리오|Architecture|Sequence Diagram|retry|dead-letter" examples/retry-dead-letter-batch-worker/README.ko.md`

### 7. 루트 navigation 갱신

- complexity: low
- expected files:
  - `README.md`
  - `README.ko.md`
  - `docs/images/readme-diagrams/workshop-example-map.*`
- action:
  - 새 example table row와 run command를 추가한다.
  - Graphviz evidence와 final PNG/SVG asset으로 workshop example map을 regenerate
    또는 patch한다.
- verification:
  - `rg -n "retry-dead-letter-batch-worker" README.md README.ko.md`
  - `workshop-example-map.png` visual inspection

### 8. Step 6-R review 및 검증

- complexity: medium
- expected files:
  - `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-code-review.md`
  - `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-verifier-checklist.md`
- action:
  - full diff에 대해 local 7-Tier Go review를 실행한다.
  - `bluetape-go-patterns` P0/P1 gate를 적용한다.
  - PR 전에 모든 P0/P1 finding을 수정한다.
- verification:
  - review artifact가 `P0=0 P1=0`을 보고한다.

### 9. 최종 검증, commit, push, PR

- complexity: medium
- expected files:
  - PR body ending with `## DoD Status`
- action:
  - full validation command set을 실행한다.
  - Lore-format trailer로 commit한다.
  - branch를 push하고 #74에 link된 PR을 만든다.
  - PR body와 GitHub CI를 검증한다.
- verification:
  - `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
  - `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`
  - `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`
  - `go run ./examples/retry-dead-letter-batch-worker`
  - `go test -run '^$' ./examples/retry-dead-letter-batch-worker`
  - `git diff --check`
  - `golangci-lint cache clean && make ci`
  - `gh pr checks`

## 위험과 완화

| 위험 | 완화 |
|---|---|
| dead-letter behavior가 batch skip semantic을 숨김 | test가 domain dead-letter record와 `batch.Report.SkipCount`를 모두 assert한다. |
| retry loop가 실수로 재구현됨 | runner는 반드시 `batch.RetryPolicy`를 사용하고 test는 `RetryCount`를 assert한다. |
| cancellation이 retried 또는 dead-lettered됨 | cancellation test가 `StatusCancelled`, no retry, no DLT record를 assert한다. |
| writer error가 dead letter로 오분류됨 | duplicate writer test는 batch를 실패시키고 DLT를 변경하지 않아야 한다. |
| README diagram이 이전 visual defect 반복 | generator가 concrete margin gate output을 출력하고 PNG를 visually inspect한다. |

## Step 3 체크리스트 완료 보고

| 항목 | 상태 | 메모 |
|---|---|---|
| 모든 spec requirement가 task에 매핑됨 | Done | code, test, docs, diagram, navigation, review, validation. |
| dependency 순서가 올바름 | Done | planning artifact가 implementation보다 앞서고 domain이 test/docs보다 앞선다. |
| concrete verification command 명시 | Done | targeted, race, run, diagram, diff, CI command가 나열되어 있다. |
| public docs 포함 | Done | EN/KO example README 및 root README pair. |
| scope가 approved spec으로 제한됨 | Done | durable queue, DB, Gin, scheduler, new dependency 없음. |
