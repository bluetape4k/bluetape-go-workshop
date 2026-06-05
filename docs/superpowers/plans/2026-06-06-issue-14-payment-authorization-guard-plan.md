# Issue 14 Payment Authorization Guard Plan

## Scope

- Issue: #14, `[v0.2.0] Add payment authorization circuit-breaker and bulkhead example`
- Spec:
  `docs/superpowers/specs/2026-06-06-issue-14-payment-authorization-guard-design.md`
- Worktree:
  `.worktrees/issue-14-payment-authorization-guard`
- Branch: `issue-14-payment-authorization-guard`

## Ordering Constraints

- Commit spec, spec review, and plan before implementation.
- Apply `$bluetape-go-patterns` to every Go code, Go test, and README example task.
- Apply `$bluetape4k-diagram` to the payment-specific diagram task.
- Keep `go.mod` unchanged except for existing module resolution; no new
  dependency may be added.
- Recheck the current `resilience` APIs from the Go module cache before writing
  policy construction code.

## Tasks

### 1. Commit Planning Artifacts

- complexity: low
- skill: `bluetape4k-full-feature`
- expected files:
  - `docs/superpowers/specs/2026-06-06-issue-14-payment-authorization-guard-design.md`
  - `docs/superpowers/reviews/2026-06-06-issue-14-payment-authorization-guard-spec-review.md`
  - `docs/superpowers/reviews/2026-06-06-issue-14-payment-authorization-guard-plan-review.md`
  - `docs/superpowers/plans/2026-06-06-issue-14-payment-authorization-guard-plan.md`
- action:
  - Run Step 3-R plan review first.
  - Commit planning artifacts with a Lore-format commit before Step 4.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. Implement Thin Payment Authorization Guard

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/payment-authorization-guard/internal/paymentguard/authorize.go`
- action:
  - Add `Request`, `Authorization`, `Gateway`, `Options`, and `Authorizer`.
  - Implement zero-value defaults:
    - `FailureThreshold=2`
    - `OpenTimeout=250ms`
    - `MaxConcurrent=1`
  - Reject negative option values.
  - Validate non-empty `MerchantID`, non-empty `OrderID`, positive
    `AmountCents`, non-nil authorizer, and non-nil gateway.
  - Construct `resilience.NewCircuitBreaker[Authorization]` and
    `resilience.NewBulkhead[Authorization]`.
  - Run policies in `breaker, bulkhead` order so an open circuit rejects before
    bulkhead acquisition and before gateway invocation.
- dependency assumptions to recheck:
  - `resilience.CircuitBreakerOptions` required fields.
  - `resilience.BulkheadOptions.Wait=false` immediate rejection behavior.
  - Sentinel errors `resilience.ErrCircuitOpen` and
    `resilience.ErrBulkheadRejected`.
- rollback point:
  - If policy construction cannot stay thin, stop and revise the spec before
    adding helper abstractions.
- verification:
  - `gofmt -w examples/payment-authorization-guard/internal/paymentguard/authorize.go`
  - `go test -count=1 ./examples/payment-authorization-guard/...`

### 3. Add Focused Tests

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/payment-authorization-guard/internal/paymentguard/authorize_test.go`
- action:
  - Test success with zero-value defaults.
  - Test repeated gateway failures open the circuit.
  - Test open circuit rejects before gateway invocation and matches
    `resilience.ErrCircuitOpen`.
  - Test concurrent overflow with a blocked first gateway call and immediate
    second call rejection matching `resilience.ErrBulkheadRejected`.
  - Test synchronous event capture for at least
    `EventCircuitStateTransition`, `EventCircuitRejected`,
    `EventBulkheadAccepted`, and `EventBulkheadRejected`.
  - Test invalid requests, nil gateway, nil authorizer, and invalid negative
    options.
- concurrency note:
  - Kotlin/JUnit helpers such as `MultithreadingTester`,
    `StructuredTaskScopeTester`, and `SuspendedJobTester` do not apply because
    this is a Go module. Use channels, `sync.WaitGroup`, mutex-protected event
    capture, and `go test -race`.
- verification:
  - `gofmt -w examples/payment-authorization-guard/internal/paymentguard/authorize_test.go`
  - `go test -count=1 ./examples/payment-authorization-guard/...`
  - `go test -race -count=1 ./examples/payment-authorization-guard/...`

### 4. Add Payment Authorization Diagram Assets

- complexity: medium
- skill: `bluetape4k-diagram`
- expected files:
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.dot`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.plain`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.svg`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.png`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.svg`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.png`
- action:
  - Build Graphviz DOT/plain evidence for the node/connector layout.
  - Create the final README SVG/PNG pair using `Architects Daughter` for the
    title/prominent labels and `Comic Mono` for detail labels when available.
  - Inspect the rendered PNG and reject overlapping labels, detached connectors,
    cropped text, blank output, or one-color palette drift.
- verification:
  - `dot -Tplain docs/images/readme-diagrams/payment-authorization-guard-flow.dot -o docs/images/readme-diagrams/payment-authorization-guard-flow.plain`
  - `dot -Tsvg docs/images/readme-diagrams/payment-authorization-guard-flow.dot -o docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.svg`
  - `dot -Tpng docs/images/readme-diagrams/payment-authorization-guard-flow.dot -o docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.png`
  - `rsvg-convert docs/images/readme-diagrams/payment-authorization-guard-flow.svg -o docs/images/readme-diagrams/payment-authorization-guard-flow.png`
  - Inspect `docs/images/readme-diagrams/payment-authorization-guard-flow.png`.

### 5. Write English/Korean README Pair

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/payment-authorization-guard/README.md`
  - `examples/payment-authorization-guard/README.ko.md`
- action:
  - Add `English | 한국어` language links.
  - Explain the payment authorization scenario, policy order, open-circuit
    rejection, bulkhead overflow, synchronous events, and test command.
  - Embed the payment authorization diagram PNG.
  - Keep the README scenario-focused rather than a resilience API reference.
- verification:
  - `rg -n "English \\| 한국어|payment-authorization-guard-flow.png|go test -count=1 ./examples/payment-authorization-guard/..." examples/payment-authorization-guard`

### 6. Update Root README Tables

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `README.md`
  - `README.ko.md`
- action:
  - Add the example row to both root tables.
  - Use `English | 한국어` language-link wording.
  - Keep the v0.2.0 resilience example ordering near existing resilience
    examples.
- verification:
  - `rg -n "payment-authorization-guard|English \\| 한국어" README.md README.ko.md`

### 7. Run Validation

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files: none unless validation exposes defects.
- action:
  - Run targeted tests first.
  - Run race tests because bulkhead overflow depends on concurrency.
  - Run full repository tests.
  - Run whitespace diff check.
  - Run configured CI shortcut if present and practical.
- verification:
  - `go test -count=1 ./examples/payment-authorization-guard/...`
  - `go test -race -count=1 ./examples/payment-authorization-guard/...`
  - `go test ./...`
  - `git diff --check`
  - `make ci` when it is defined and available without external credentials.

### 8. Review, Commit, PR

- complexity: medium
- skill: `bluetape4k-full-feature`
- expected files:
  - Step 6-R review artifact under `docs/superpowers/reviews/`
  - PR branch commit(s)
- action:
  - Execute the remaining `bluetape4k-full-feature` gates in order:
    - Step 4: implement code/docs/diagram tasks.
    - Step 4-T: update tests and run targeted validation.
    - Step 5: self-review against the verifier checklist.
    - Step 6: final local validation.
    - Step 6-R: 7-tier code review and convergence to `P0=0 P1=0`.
    - Step 7: commit implementation with Lore-format trailers.
    - Step 7-P: open PR and verify PR body.
    - Step 7-R: PR review/comment gate.
    - Step 8: GitHub CI gate.
    - Step 9: final report with evidence.
  - Commit implementation with Lore-format trailers.
  - Open a PR for issue #14 with a Step DoD table at the end of the PR body.
  - Run PR body verification, PR review/comment gate, and GitHub CI gate.
- verification:
  - `git log --oneline --decorate -3`
  - `gh pr view --json number,title,url,headRefName,baseRefName`
  - `gh pr checks --watch` or equivalent CI status command.

## Acceptance Mapping

| Issue #14 Acceptance | Plan Task |
| --- | --- |
| `go test -count=1 ./examples/payment-authorization-guard/...` | Tasks 2, 3, 7 |
| Repeated gateway failures open circuit | Task 3 |
| Open circuit rejects before gateway call | Tasks 2, 3 |
| Concurrent overflow returns `ErrBulkheadRejected` | Task 3 |
| Circuit transition and rejection events | Task 3 |
| No new dependencies | Tasks 2, 7 |
| Root README and Korean README tables | Task 6 |

## Stop Condition

Stop only after Step 9 has evidence for:

- local validation commands,
- Step 6-R `P0=0 P1=0`,
- PR creation,
- PR body verification,
- PR review/comment gate,
- GitHub CI status.

Merge is not part of this plan unless the user explicitly asks to merge after
the PR is ready.
