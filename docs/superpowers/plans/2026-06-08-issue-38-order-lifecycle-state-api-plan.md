# Issue 38 Order Lifecycle State API 계획

## 범위

- 이슈: #38, `[v0.4.0] Add Gin order lifecycle state API example`
- Spec:
  `docs/superpowers/specs/2026-06-08-issue-38-order-lifecycle-state-api-design.md`
- Research:
  `docs/superpowers/research/2026-06-08-issue-38-order-lifecycle-state-api-research.md`
- Worktree:
  `.worktrees/feat-issue-38-order-lifecycle-state-api`
- 브랜치: `feat/issue-38-order-lifecycle-state-api`

## 순서 제약

- 구현 전에 research, spec, spec review, plan, plan review를 commit한다.
- 모든 Go code, Go test, README example, review gate에는
  `bluetape-go-patterns`를 적용한다.
- 새 dependency는 Gin만 추가한다. persistence, Testcontainers, unrelated helper는
  추가하지 않는다.
- 모든 작업은 feature worktree 안에서 수행한다. 루트 `develop` checkout에 source
  file을 쓰지 않는다.
- 이 이슈에서는 `workflow`와 `workreport`를 제외한다.

## 작업

### 1. 계획 산출물 커밋

- complexity: low
- skill: `bluetape4k-full-feature`
- expected files:
  - `docs/superpowers/research/2026-06-08-issue-38-order-lifecycle-state-api-research.md`
  - `docs/superpowers/specs/2026-06-08-issue-38-order-lifecycle-state-api-design.md`
  - `docs/superpowers/reviews/2026-06-08-issue-38-order-lifecycle-state-api-spec-review.md`
  - `docs/superpowers/reviews/2026-06-08-issue-38-order-lifecycle-state-api-plan-review.md`
  - `docs/superpowers/plans/2026-06-08-issue-38-order-lifecycle-state-api-plan.md`
- action:
  - Step 3-R plan review를 먼저 실행한다.
  - Step 4 전에 모든 planning artifact를 Lore-format trailer로 commit한다.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Gin dependency 추가

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `go.mod`
  - `go.sum`
- action:
  - `go get github.com/gin-gonic/gin`을 실행한다.
  - `go mod tidy`를 실행한다.
  - unrelated direct dependency가 들어오지 않았는지 확인한다.
- verification:
  - `go list -m github.com/gin-gonic/gin`
  - `git diff -- go.mod go.sum`

### 3. Order State HTTP Server 구현

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/internal/orderstate/server.go`
- action:
  - `OrderState`, `OrderEvent`, `OrderSnapshot`, transition request/response
    DTO를 정의한다.
  - spec의 state/event로 `state.NewMachine`을 구성한다.
  - `pay`에 positive-total guard를 추가한다.
  - Gin route를 사용한다.
    - `GET /healthz`
    - `GET /orders/current`
    - `POST /orders/current/transitions`
    - `GET /orders/current/transitions/:event/can`
  - HTTP mapping에는 `state` sentinel error에 대한 `errors.Is`를 사용한다.
  - response JSON field name을 stable하고 documented 상태로 유지한다.
- verification:
  - `gofmt -w examples/order-lifecycle-state-api/internal/orderstate/server.go`
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`

### 4. Runnable main 추가

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/main.go`
- action:
  - positive total을 가진 in-memory example order 하나를 만든다.
  - `ReadHeaderTimeout`이 있는 `http.Server`를 사용한다.
  - `HTTP_ADDR` override를 허용한다.
  - 새 `orderstate.Server`를 handler로 사용한다.
- verification:
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -run '^$' ./examples/order-lifecycle-state-api`

### 5. 집중 테스트 추가

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/internal/orderstate/server_test.go`
- action:
  - health 및 current-state response를 테스트한다.
  - 허용 transition path `draft -> submitted -> paid`를 테스트한다.
  - invalid transition이 `409`를 반환하고 state가 바뀌지 않는지 테스트한다.
  - non-positive total에 대한 guard rejection을 테스트한다.
  - final state가 추가 transition을 거부하는지 테스트한다.
  - malformed JSON 및 unknown event가 `400`을 반환하는지 테스트한다.
  - concurrent duplicate transition request가 race-safe이고 valid state를 남기는지
    테스트한다.
- verification:
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -race -count=1 ./examples/order-lifecycle-state-api/...`

### 6. 예제 README pair 작성

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/README.md`
  - `examples/order-lifecycle-state-api/README.ko.md`
- action:
  - language switch를 추가한다.
  - scenario, endpoint, state transition, invalid/final/guard behavior,
    test command를 설명한다.
  - workflow runner 없이 finite state machine으로 충분한 경우를 설명한다.
  - 예제가 in-memory이며 production persistence가 아님을 명시한다.
- verification:
  - `rg -n "finite state machine|workflow runner|go test -count=1 ./examples/order-lifecycle-state-api/..." examples/order-lifecycle-state-api/README.md examples/order-lifecycle-state-api/README.ko.md`

### 7. 루트 README pair 갱신

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `README.md`
  - `README.ko.md`
- action:
  - 두 root example table에 새 example row를 추가한다.
  - framework-visible public API example에서는 Gin이 default이고, compatibility-
    focused example에서는 net/http/chi가 여전히 적절하다는 web framework wording으로
    갱신한다.
  - 0.4.0 roadmap wording을 새 state/workflow track과 맞춘다.
- verification:
  - `rg -n "order-lifecycle-state-api|Gin|0.4.0" README.md README.ko.md`

### 8. 검증 실행

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files: 검증이 결함을 드러내지 않는 한 없음.
- action:
  - targeted test를 먼저 실행한다.
  - 새 example에 대해 race test를 실행한다.
  - full repo test를 실행한다.
  - whitespace 및 local CI gate를 실행한다.
- verification:
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -race -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -count=1 ./...`
  - `git diff --check`
  - `make ci`

### 9. Review, lesson, commit, PR

- complexity: medium
- skill: `bluetape4k-full-feature`
- expected files:
  - Step 6-R review artifact under `docs/superpowers/reviews/`
  - `docs/lessons/2026-06-08-order-lifecycle-state-api.md`
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
  - #38에 link되고 milestone 0.4.0 및 `debop` assign이 있는 PR을 연다.
- verification:
  - `git log --oneline --decorate -5`
  - `gh pr view --json number,title,url,headRefName,baseRefName,body`
  - `gh pr view --json statusCheckRollup`

## 인수 조건 매핑

| Issue #38 인수 조건 | Plan task |
| --- | --- |
| Gin route가 current state 및 transition command 노출 | Tasks 2, 3, 4 |
| allowed transition test coverage | Task 5 |
| invalid transition test coverage | Task 5 |
| concurrent request safety test coverage | Tasks 5, 8 |
| finite state machine이 workflow runner 없이 충분한 경우를 README가 설명 | Task 6 |
| root README navigation 갱신 | Task 7 |
| PR gate 전 focused test | Tasks 5, 8 |

## 중단 조건

Step 9가 다음 증거를 가진 뒤에만 중단한다.

- committed planning artifact,
- implemented example,
- local validation command,
- Step 6-R `P0=0 P1=0`,
- lesson commit,
- PR creation,
- PR body verification,
- PR review/comment gate,
- GitHub CI status.

PR 준비 뒤 사용자가 명시적으로 merge를 요청하지 않는 한 merge는 이 plan의 일부가
아니다.
