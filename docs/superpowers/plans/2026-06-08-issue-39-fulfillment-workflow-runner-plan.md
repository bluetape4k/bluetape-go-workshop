# Issue 39 Fulfillment Workflow Runner 계획

## 범위

- 이슈: #39, `[v0.4.0] Add fulfillment workflow runner example`
- Spec:
  `docs/superpowers/specs/2026-06-08-issue-39-fulfillment-workflow-runner-design.md`
- Research:
  `docs/superpowers/research/2026-06-08-issue-39-fulfillment-workflow-runner-research.md`
- Worktree:
  `.worktrees/feat-issue-39-fulfillment-workflow-runner`
- 브랜치: `feat/issue-39-fulfillment-workflow-runner`

## 순서 제약

- 구현 전에 research, spec, spec review, plan, plan review를 commit한다.
- Go code, test, README example, review gate에는 `bluetape-go-patterns`를
  적용한다.
- README diagram generation과 visual inspection에는 `bluetape4k-diagram`을
  적용한다.
- `bluetape-go v0.5.1`의 기존 `workflow` 및 `workreport` API를 사용한다.
  runtime dependency를 추가하지 않는다.
- 모든 작업은 feature worktree에서 수행한다. root `develop` checkout에 source
  file을 쓰지 않는다.
- 예제는 request-scoped 및 in-memory로 유지한다.

## 작업

### 1. 계획 산출물 커밋

- complexity: low
- skill: `bluetape4k-full-feature`
- expected files:
  - `docs/superpowers/research/2026-06-08-issue-39-fulfillment-workflow-runner-research.md`
  - `docs/superpowers/specs/2026-06-08-issue-39-fulfillment-workflow-runner-design.md`
  - `docs/superpowers/reviews/2026-06-08-issue-39-fulfillment-workflow-runner-spec-review.md`
  - `docs/superpowers/reviews/2026-06-08-issue-39-fulfillment-workflow-runner-plan-review.md`
  - `docs/superpowers/plans/2026-06-08-issue-39-fulfillment-workflow-runner-plan.md`
- action:
  - Step 3-R plan review를 먼저 실행한다.
  - Step 4 전에 모든 planning artifact를 Lore-format trailer로 commit한다.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Fulfillment Workflow Server 구현

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/fulfillment-workflow-runner/internal/fulfillment/server.go`
- action:
  - request DTO, stable report DTO, error response DTO를 정의한다.
  - `NewServer(Options)`와 `ServeHTTP`를 만든다.
  - Gin route를 등록한다.
    - `GET /healthz`
    - `POST /fulfillment/run`
  - per-request runner를 구성한다.
    - sequential root `fulfillment`
    - `validate-order`
    - parallel `risk-checks`
    - `reserve-inventory`
    - `authorize-payment`
    - conditional `shipment-decision`
  - malformed/invalid request field는 `400`으로 매핑한다.
  - failed/partial workflow report는 `409`로 매핑한다.
  - cancelled workflow report는 `408`로 매핑한다.
  - JSON response에서 dynamic timestamp를 생략한다.
- verification:
  - `gofmt -w examples/fulfillment-workflow-runner/internal/fulfillment/server.go`
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`

### 3. Runnable main 추가

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/fulfillment-workflow-runner/main.go`
- action:
  - `ReadHeaderTimeout`이 있는 `http.Server`를 만든다.
  - default `:8084`와 함께 `HTTP_ADDR`를 사용한다.
  - listening address를 log하고 새 fulfillment server를 serve한다.
- verification:
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -run '^$' ./examples/fulfillment-workflow-runner`

### 4. 집중 테스트 추가

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/fulfillment-workflow-runner/internal/fulfillment/server_test.go`
- action:
  - health endpoint를 테스트한다.
  - shipment creation을 포함한 successful fulfillment를 테스트한다.
  - conditional shipment skip이 completed로 남고 shipment creation을 추가하지
    않는지 테스트한다.
  - inventory failure가 `409`로 매핑되고 parallel branch에 보이는지 테스트한다.
  - `StopOnFailure`에서 payment failure가 느린 inventory sibling을 cancel하는지
    테스트한다.
  - caller cancellation이 `408`로 매핑되고 cancelled report를 반환하는지
    테스트한다.
  - malformed JSON 및 invalid request field가 `400`을 반환하는지 테스트한다.
- verification:
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`

### 5. README 다이어그램 생성

- complexity: medium
- skill: `bluetape4k-diagram`
- expected files:
  - `scripts/generate-fulfillment-workflow-diagrams.sh`
  - `docs/images/readme-diagrams/fulfillment-workflow-runner-scenario.svg`
  - `docs/images/readme-diagrams/fulfillment-workflow-runner-scenario.png`
  - `docs/images/readme-diagrams/fulfillment-workflow-runner-architecture.svg`
  - `docs/images/readme-diagrams/fulfillment-workflow-runner-architecture.png`
  - `docs/images/readme-diagrams/fulfillment-workflow-runner-sequence.svg`
  - `docs/images/readme-diagrams/fulfillment-workflow-runner-sequence.png`
  - matching DOT/Plain/Graphviz evidence assets
- action:
  - scenario, Architecture, Sequence Diagram에 대해 deterministic SVG/PNG diagram
    generation을 만든다.
  - bad endpoint angle, bad bend, interior crossing, margin imbalance, title gap
    issue, font fallback이 모두 0인 geometry gate summary를 출력한다.
  - PR 전에 각 rendered PNG를 개별 점검한다.
- verification:
  - `bash scripts/generate-fulfillment-workflow-diagrams.sh`
  - rendered PNG inspection
  - `git diff --check`

### 6. 예제 README pair 작성

- complexity: medium
- skills: `bluetape-go-patterns`, `bluetape4k-diagram`
- expected files:
  - `examples/fulfillment-workflow-runner/README.md`
  - `examples/fulfillment-workflow-runner/README.ko.md`
- action:
  - language switch를 추가한다.
  - example scenario, workflow step boundary, failure policy, cancellation
    propagation, conditional skip behavior, run command, endpoint, test를
    설명한다.
  - PNG diagram만 embed한다.
  - production hardening gap을 명시한다: durable workflow state, retry, outbox,
    external service client, persistence 없음.
- verification:
  - `rg -n "workflow step|Architecture|Sequence Diagram|go test -count=1 ./examples/fulfillment-workflow-runner/..." examples/fulfillment-workflow-runner/README.md examples/fulfillment-workflow-runner/README.ko.md`

### 7. 루트 README pair 갱신

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `README.md`
  - `README.ko.md`
- action:
  - 두 root example table에 `examples/fulfillment-workflow-runner`를 추가한다.
  - 새 example용 root run instruction을 추가한다.
  - 0.4.0 roadmap wording을 state API only에서 state 및 workflow example로
    확장한다.
- verification:
  - `rg -n "fulfillment-workflow-runner|workflow|0.4.0" README.md README.ko.md`

### 8. 검증 실행

- complexity: medium
- skills: `bluetape-go-patterns`, `bluetape4k-diagram`
- expected files: 검증이 결함을 드러내지 않는 한 없음.
- action:
  - diagram generator를 실행한다.
  - targeted test를 먼저 실행한다.
  - 새 example에 대해 race test를 실행한다.
  - full repo test와 local CI를 실행한다.
- verification:
  - `bash scripts/generate-fulfillment-workflow-diagrams.sh`
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -run '^$' ./examples/fulfillment-workflow-runner`
  - `go test -count=1 ./...`
  - `git diff --check`
  - `make ci`

### 9. Review, lesson, commit, PR

- complexity: medium
- skill: `bluetape4k-full-feature`
- expected files:
  - Step 5 verifier checklist under `docs/superpowers/reviews/`
  - Step 6-R review artifact under `docs/superpowers/reviews/`
  - `docs/lessons/2026-06-08-fulfillment-workflow-runner.md`
  - PR body temporary file
- action:
  - 남은 gate를 순서대로 실행한다.
    - Step 4: implementation.
    - Step 4-T: test.
    - Step 5: verifier checklist.
    - Step 6: final checklist.
    - Step 6-R: 7-tier code review, `P0=0 P1=0`.
    - Step 7: lesson file 및 commit.
    - Step 7-P: final section이 `## DoD Status`인 PR open.
    - Step 7-R: PR review/comment gate.
    - Step 8: GitHub CI gate.
    - Step 9: final DoD report.
  - implementation은 Lore-format trailer로 commit한다.
  - #39에 link되고 milestone 0.4.0 및 `debop` assign이 있는 PR을 연다.
- verification:
  - `git log --oneline --decorate -5`
  - `gh pr view --json number,title,url,headRefName,baseRefName,body`
  - `gh pr view --json statusCheckRollup`

## 인수 조건 매핑

| Issue #39 인수 조건 | Plan task |
| --- | --- |
| sequential, parallel, conditional workflow runner example | Tasks 2, 4, 6 |
| success path test | Task 4 |
| step failure test | Task 4 |
| conditional skip test | Task 4 |
| cancellation test | Task 4 |
| README가 workflow step boundary와 failure semantic 문서화 | Task 6 |
| root README가 0.4.0 아래 example link | Task 7 |
| scenario-shaped Gin example | Tasks 2, 3, 6 |
| README scenario, Architecture, Sequence Diagram | Tasks 5, 6 |
| PR gate 전 focused test | Tasks 4, 8 |

## 중단 조건

Step 9가 다음 증거를 가진 뒤에만 중단한다.

- committed planning artifact,
- implemented example,
- generated 및 visually inspected diagram,
- local validation command,
- Step 6-R `P0=0 P1=0`,
- lesson commit,
- PR creation,
- PR body verification,
- PR review/comment gate,
- GitHub CI status.

PR 준비 뒤 사용자가 명시적으로 merge를 요청하지 않는 한 merge는 이 plan의 일부가
아니다.
