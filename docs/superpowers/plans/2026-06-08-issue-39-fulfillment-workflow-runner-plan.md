# Issue 39 Fulfillment Workflow Runner Plan

## Scope

- Issue: #39, `[v0.4.0] Add fulfillment workflow runner example`
- Spec:
  `docs/superpowers/specs/2026-06-08-issue-39-fulfillment-workflow-runner-design.md`
- Research:
  `docs/superpowers/research/2026-06-08-issue-39-fulfillment-workflow-runner-research.md`
- Worktree:
  `.worktrees/feat-issue-39-fulfillment-workflow-runner`
- Branch: `feat/issue-39-fulfillment-workflow-runner`

## Ordering Constraints

- Commit research, spec, spec review, plan, and plan review before
  implementation.
- Apply `bluetape-go-patterns` to Go code, tests, README examples, and review
  gates.
- Apply `bluetape4k-diagram` to README diagram generation and visual
  inspection.
- Use existing `workflow` and `workreport` APIs from `bluetape-go v0.5.1`; do
  not add runtime dependencies.
- Keep all work in the feature worktree. Do not write source files on the root
  `develop` checkout.
- Keep the example request-scoped and in-memory.

## Tasks

### 1. Commit Planning Artifacts

- complexity: low
- skill: `bluetape4k-full-feature`
- expected files:
  - `docs/superpowers/research/2026-06-08-issue-39-fulfillment-workflow-runner-research.md`
  - `docs/superpowers/specs/2026-06-08-issue-39-fulfillment-workflow-runner-design.md`
  - `docs/superpowers/reviews/2026-06-08-issue-39-fulfillment-workflow-runner-spec-review.md`
  - `docs/superpowers/reviews/2026-06-08-issue-39-fulfillment-workflow-runner-plan-review.md`
  - `docs/superpowers/plans/2026-06-08-issue-39-fulfillment-workflow-runner-plan.md`
- action:
  - Run Step 3-R plan review first.
  - Commit all planning artifacts with Lore-format trailers before Step 4.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Implement Fulfillment Workflow Server

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/fulfillment-workflow-runner/internal/fulfillment/server.go`
- action:
  - Define request DTOs, stable report DTOs, and error response DTOs.
  - Create `NewServer(Options)` and `ServeHTTP`.
  - Register Gin routes:
    - `GET /healthz`
    - `POST /fulfillment/run`
  - Build the per-request runner:
    - sequential root `fulfillment`
    - `validate-order`
    - parallel `risk-checks`
    - `reserve-inventory`
    - `authorize-payment`
    - conditional `shipment-decision`
  - Map malformed/invalid request fields to `400`.
  - Map failed/partial workflow report to `409`.
  - Map cancelled workflow report to `408`.
  - Omit dynamic timestamps from JSON responses.
- verification:
  - `gofmt -w examples/fulfillment-workflow-runner/internal/fulfillment/server.go`
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`

### 3. Add Runnable Main

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/fulfillment-workflow-runner/main.go`
- action:
  - Create an `http.Server` with `ReadHeaderTimeout`.
  - Use `HTTP_ADDR` with default `:8084`.
  - Log the listening address and serve the new fulfillment server.
- verification:
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -run '^$' ./examples/fulfillment-workflow-runner`

### 4. Add Focused Tests

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/fulfillment-workflow-runner/internal/fulfillment/server_test.go`
- action:
  - Test health endpoint.
  - Test successful fulfillment with shipment creation.
  - Test conditional shipment skip remains completed and does not add shipment
    creation.
  - Test inventory failure maps to `409` and is visible in the parallel branch.
  - Test payment failure cancels a slow inventory sibling under
    `StopOnFailure`.
  - Test caller cancellation maps to `408` and returns a cancelled report.
  - Test malformed JSON and invalid request fields return `400`.
- verification:
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`

### 5. Generate README Diagrams

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
  - Create deterministic SVG/PNG diagram generation for scenario,
    Architecture, and Sequence Diagram.
  - Print geometry gate summaries with zero bad endpoint angles, bad bends,
    interior crossings, margin imbalance, title gap issues, and font fallback.
  - Inspect each rendered PNG individually before PR.
- verification:
  - `bash scripts/generate-fulfillment-workflow-diagrams.sh`
  - rendered PNG inspection
  - `git diff --check`

### 6. Write Example README Pair

- complexity: medium
- skills: `bluetape-go-patterns`, `bluetape4k-diagram`
- expected files:
  - `examples/fulfillment-workflow-runner/README.md`
  - `examples/fulfillment-workflow-runner/README.ko.md`
- action:
  - Add language switch.
  - Explain example scenario, workflow step boundaries, failure policy,
    cancellation propagation, conditional skip behavior, run command,
    endpoints, and tests.
  - Embed PNG diagrams only.
  - State production hardening gaps: no durable workflow state, retries,
    outbox, external service clients, or persistence.
- verification:
  - `rg -n "workflow step|Architecture|Sequence Diagram|go test -count=1 ./examples/fulfillment-workflow-runner/..." examples/fulfillment-workflow-runner/README.md examples/fulfillment-workflow-runner/README.ko.md`

### 7. Update Root README Pair

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `README.md`
  - `README.ko.md`
- action:
  - Add `examples/fulfillment-workflow-runner` to both root example tables.
  - Add root run instructions for the new example.
  - Expand 0.4.0 roadmap wording from state API only to state and workflow
    examples.
- verification:
  - `rg -n "fulfillment-workflow-runner|workflow|0.4.0" README.md README.ko.md`

### 8. Run Validation

- complexity: medium
- skills: `bluetape-go-patterns`, `bluetape4k-diagram`
- expected files: none unless validation exposes defects.
- action:
  - Run diagram generator.
  - Run targeted tests first.
  - Run race test for the new example.
  - Run full repo tests and local CI.
- verification:
  - `bash scripts/generate-fulfillment-workflow-diagrams.sh`
  - `go test -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
  - `go test -run '^$' ./examples/fulfillment-workflow-runner`
  - `go test -count=1 ./...`
  - `git diff --check`
  - `make ci`

### 9. Review, Lessons, Commit, PR

- complexity: medium
- skill: `bluetape4k-full-feature`
- expected files:
  - Step 5 verifier checklist under `docs/superpowers/reviews/`
  - Step 6-R review artifact under `docs/superpowers/reviews/`
  - `docs/lessons/2026-06-08-fulfillment-workflow-runner.md`
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
  - Open PR linked to #39, milestone 0.4.0, assigned to `debop`.
- verification:
  - `git log --oneline --decorate -5`
  - `gh pr view --json number,title,url,headRefName,baseRefName,body`
  - `gh pr view --json statusCheckRollup`

## Acceptance Mapping

| Issue #39 Acceptance | Plan Task |
| --- | --- |
| Sequential, parallel, and conditional workflow runner example | Tasks 2, 4, 6 |
| Success path test | Task 4 |
| Step failure test | Task 4 |
| Conditional skip test | Task 4 |
| Cancellation test | Task 4 |
| README documents workflow step boundaries and failure semantics | Task 6 |
| Root README links the example under 0.4.0 | Task 7 |
| Scenario-shaped Gin example | Tasks 2, 3, 6 |
| README scenario, Architecture, and Sequence Diagram | Tasks 5, 6 |
| Focused tests before PR gate | Tasks 4, 8 |

## Stop Condition

Stop only after Step 9 has evidence for:

- committed planning artifacts,
- implemented example,
- generated and visually inspected diagrams,
- local validation commands,
- Step 6-R `P0=0 P1=0`,
- lessons commit,
- PR creation,
- PR body verification,
- PR review/comment gate,
- GitHub CI status.

Merge is not part of this plan unless the user explicitly asks to merge after
the PR is ready.

