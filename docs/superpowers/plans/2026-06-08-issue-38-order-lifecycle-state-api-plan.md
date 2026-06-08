# Issue 38 Order Lifecycle State API Plan

## Scope

- Issue: #38, `[v0.4.0] Add Gin order lifecycle state API example`
- Spec:
  `docs/superpowers/specs/2026-06-08-issue-38-order-lifecycle-state-api-design.md`
- Research:
  `docs/superpowers/research/2026-06-08-issue-38-order-lifecycle-state-api-research.md`
- Worktree:
  `.worktrees/feat-issue-38-order-lifecycle-state-api`
- Branch: `feat/issue-38-order-lifecycle-state-api`

## Ordering Constraints

- Commit research, spec, spec review, plan, and plan review before
  implementation.
- Apply `bluetape-go-patterns` to all Go code, Go tests, README examples, and
  review gates.
- Add only Gin as a new dependency; do not add persistence, Testcontainers, or
  unrelated helpers.
- Keep all work in the feature worktree. Do not write source files on the root
  `develop` checkout.
- Keep `workflow` and `workreport` out of this issue.

## Tasks

### 1. Commit Planning Artifacts

- complexity: low
- skill: `bluetape4k-full-feature`
- expected files:
  - `docs/superpowers/research/2026-06-08-issue-38-order-lifecycle-state-api-research.md`
  - `docs/superpowers/specs/2026-06-08-issue-38-order-lifecycle-state-api-design.md`
  - `docs/superpowers/reviews/2026-06-08-issue-38-order-lifecycle-state-api-spec-review.md`
  - `docs/superpowers/reviews/2026-06-08-issue-38-order-lifecycle-state-api-plan-review.md`
  - `docs/superpowers/plans/2026-06-08-issue-38-order-lifecycle-state-api-plan.md`
- action:
  - Run Step 3-R plan review first.
  - Commit all planning artifacts with Lore-format trailers before Step 4.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Add Gin Dependency

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `go.mod`
  - `go.sum`
- action:
  - Run `go get github.com/gin-gonic/gin`.
  - Run `go mod tidy`.
  - Confirm no unrelated direct dependencies were introduced.
- verification:
  - `go list -m github.com/gin-gonic/gin`
  - `git diff -- go.mod go.sum`

### 3. Implement Order State HTTP Server

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/internal/orderstate/server.go`
- action:
  - Define `OrderState`, `OrderEvent`, `OrderSnapshot`, transition request and
    response DTOs.
  - Build `state.NewMachine` with states/events from the spec.
  - Add a positive-total guard for `pay`.
  - Use Gin routes:
    - `GET /healthz`
    - `GET /orders/current`
    - `POST /orders/current/transitions`
    - `GET /orders/current/transitions/:event/can`
  - Use `errors.Is` against `state` sentinel errors for HTTP mapping.
  - Keep response JSON field names stable and documented.
- verification:
  - `gofmt -w examples/order-lifecycle-state-api/internal/orderstate/server.go`
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`

### 4. Add Runnable Main

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/main.go`
- action:
  - Create one in-memory example order with a positive total.
  - Use `http.Server` with `ReadHeaderTimeout`.
  - Allow `HTTP_ADDR` override.
  - Use the new `orderstate.Server` as the handler.
- verification:
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -run '^$' ./examples/order-lifecycle-state-api`

### 5. Add Focused Tests

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/internal/orderstate/server_test.go`
- action:
  - Test health and current-state responses.
  - Test allowed transition path `draft -> submitted -> paid`.
  - Test invalid transition returns `409` and state stays unchanged.
  - Test guard rejection for non-positive total.
  - Test final state rejects further transitions.
  - Test malformed JSON and unknown event return `400`.
  - Test concurrent duplicate transition requests are race-safe and leave a
    valid state.
- verification:
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -race -count=1 ./examples/order-lifecycle-state-api/...`

### 6. Write Example README Pair

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/order-lifecycle-state-api/README.md`
  - `examples/order-lifecycle-state-api/README.ko.md`
- action:
  - Add language switch.
  - Explain scenario, endpoints, state transitions, invalid/final/guard
    behavior, and test commands.
  - Explain when a finite state machine is enough without a workflow runner.
  - State that the example is in-memory and not production persistence.
- verification:
  - `rg -n "finite state machine|workflow runner|go test -count=1 ./examples/order-lifecycle-state-api/..." examples/order-lifecycle-state-api/README.md examples/order-lifecycle-state-api/README.ko.md`

### 7. Update Root README Pair

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `README.md`
  - `README.ko.md`
- action:
  - Add the new example row to both root example tables.
  - Update web framework wording so Gin is default for framework-visible public
    API examples, while net/http/chi remain appropriate for compatibility-
    focused examples.
  - Keep 0.4.0 roadmap wording aligned with the new state/workflow track.
- verification:
  - `rg -n "order-lifecycle-state-api|Gin|0.4.0" README.md README.ko.md`

### 8. Run Validation

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files: none unless validation exposes defects.
- action:
  - Run targeted tests first.
  - Run race test for the new example.
  - Run full repo tests.
  - Run whitespace and local CI gates.
- verification:
  - `go test -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -race -count=1 ./examples/order-lifecycle-state-api/...`
  - `go test -count=1 ./...`
  - `git diff --check`
  - `make ci`

### 9. Review, Lessons, Commit, PR

- complexity: medium
- skill: `bluetape4k-full-feature`
- expected files:
  - Step 6-R review artifact under `docs/superpowers/reviews/`
  - `docs/lessons/2026-06-08-order-lifecycle-state-api.md`
  - PR body temporary file
- action:
  - Execute remaining gates in order:
    - Step 4: implementation.
    - Step 4-T: tests.
    - Step 5: verifier checklist.
    - Step 6: final checklist.
    - Step 6-R: 7-tier code review, `P0=0 P1=0`.
    - Step 7: lessons file and commit.
    - Step 7-P: open PR with final section `## DoD Status`.
    - Step 7-R: PR review/comment gate.
    - Step 8: GitHub CI gate.
    - Step 9: final DoD report.
  - Commit implementation with Lore-format trailers.
  - Open PR linked to #38, milestone 0.4.0, assigned to `debop`.
- verification:
  - `git log --oneline --decorate -5`
  - `gh pr view --json number,title,url,headRefName,baseRefName,body`
  - `gh pr view --json statusCheckRollup`

## Acceptance Mapping

| Issue #38 Acceptance | Plan Task |
| --- | --- |
| Gin routes expose current state and transition commands | Tasks 2, 3, 4 |
| Tests cover allowed transitions | Task 5 |
| Tests cover invalid transitions | Task 5 |
| Tests cover concurrent request safety | Tasks 5, 8 |
| README explains when finite state machine is enough without workflow runner | Task 6 |
| Root README navigation updated | Task 7 |
| Focused tests before PR gate | Tasks 5, 8 |

## Stop Condition

Stop only after Step 9 has evidence for:

- committed planning artifacts,
- implemented example,
- local validation commands,
- Step 6-R `P0=0 P1=0`,
- lessons commit,
- PR creation,
- PR body verification,
- PR review/comment gate,
- GitHub CI status.

Merge is not part of this plan unless the user explicitly asks to merge after
the PR is ready.
