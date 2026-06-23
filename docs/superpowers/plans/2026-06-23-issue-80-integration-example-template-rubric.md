# Issue #80 Plan: Integration Example Template and Acceptance Rubric

## Goal

Define the repeatable template and acceptance rubric for milestone-level
integration examples so 0.8.0+ workshop tracks stay scenario-shaped,
testable, and aligned with the existing foundation integrations.

Issue #80 is planning-only. It standardizes future example PR expectations; it
does not add a runnable example or edit root README navigation in this PR.

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #80 `[v0.7.0] Add integration example template and acceptance rubric`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- Work type: Type E - Planning / Maintenance

## Sources Checked

- GitHub issues:
  - #31 0.7.0 research/planning parent
  - #49 roadmap matrix and README expansion plan
  - #79 example selection scorecard
  - Foundation integration examples: #72, #75, #78
  - Future integration examples: #65, #66, #67, #68, #69
- Current repository planning/spec docs:
  - `docs/superpowers/specs/2026-06-09-issue-72-order-fulfillment-integration-design.md`
  - `docs/superpowers/plans/2026-06-09-issue-72-order-fulfillment-integration-plan.md`
  - `docs/superpowers/specs/2026-06-22-issue-29-customer-migration-batch-integration-design.md`
  - `docs/superpowers/specs/2026-06-23-issue-78-checkout-guard-design.md`
  - `docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md`
  - `docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md`
- Retrieval:
  - Context-mode search for #80, #72, #75, #78, #49, and #79 planning history.
  - `gno query "bluetape-go-workshop issue 80 integration example template acceptance rubric #31 #49 #79" -c bluetape4k-github --fast --no-rerank`

## Integration Example Definition

An integration example is a milestone-level runnable scenario that composes
focused examples from the same umbrella track. It should teach the connection
between package contracts, domain behavior, validation, and operator/user
visibility without becoming a generic application framework.

Required shape:

- It appears after focused examples in the milestone sequence.
- It names the focused prerequisite issues in the design, README, and PR body.
- It owns one concrete domain scenario with stable fixtures.
- It uses real `bluetape-go` package surfaces for the package lesson.
- It keeps framework, persistence, emulator, and diagram work proportional to
  the scenario.
- It records production hardening gaps rather than pretending the workshop
  implementation is production infrastructure.

Non-goals:

- Do not import sibling example `internal` packages across example boundaries.
- Do not add a framework layer just to unify unrelated examples.
- Do not introduce a durable database, queue, cloud service, or graph backend
  unless the package behavior under test requires it.
- Do not hide the package lesson behind a decorative API shell.

## Canonical Integration Issue Set

| Issue | Milestone | Integration role | Template implication |
|---|---|---|---|
| #72 | 0.4.0 | Order fulfillment integration over state, workflow, workreport, and compensation | Gin is appropriate because the lesson is a service-facing workflow; diagrams clarify lifecycle/sequence. |
| #75 | 0.5.0 | Customer migration batch integration over checkpoint, operations API, leader scheduling, retry, and dead-letter handling | Gin is appropriate for start/status/cancel operations; deterministic in-memory stores keep restart behavior visible. |
| #78 | 0.6.0 | Checkout guard integration over ID/JWT, money/rules, and probabilistic admission | Gin is appropriate for a checkout boundary; documentation must state probabilistic dedupe limits. |
| #65 | 0.8.0 | SQL order service integration over repository, transaction, and CRUD examples | Gin is appropriate; Testcontainers should be used only when real transaction/lock semantics are required. |
| #66 | 0.9.0 | S3-SQS-DynamoDB document workflow integration over AWS/Floci examples | Gin is optional; Floci/Testcontainers is required for local AWS behavior and must be serial. |
| #67 | 0.10.0 | Content moderation workflow integration over text search, masking, tokenizer, and language detection | Gin is appropriate; unsupported language/tokenizer cases must be explicit. |
| #68 | 0.11.0 | Audited order workflow/outbox integration over audit history, outbox, and query API | Gin is appropriate for read/query API; durable outbox behavior should be deterministic before broker emulation. |
| #69 | 0.12.0 | Graph risk intelligence integration over graph modeling, import/export, traversal, and scoring | Gin is optional; keep graph package behavior visible and add diagrams only when they clarify the risk workflow. |

## README Template

Every integration example must include `README.md` and `README.ko.md` with the
same structure and equivalent technical content.

### English README Sections

Use these sections in order unless the issue gives a narrower structure:

1. `# <Example Name>`
2. `## Scenario`
   - One paragraph naming the domain workflow.
   - One bullet list naming the focused examples or package lessons composed.
3. `## What This Integrates`
   - Links to focused examples or prerequisite issues.
   - Explicit package coverage, such as `state`, `workflow`, `batch`,
     `leader`, `sql`, `aws`, `text`, `audit`, or `graph`.
4. `## Run`
   - Required command.
   - Default bind address when HTTP is present.
   - Docker/Testcontainers or Floci prerequisite if applicable.
5. `## API` or `## Workflow`
   - Use `API` for HTTP examples.
   - Use `Workflow` for package, worker, local job, or graph examples without a
     public HTTP boundary.
6. `## Validation`
   - Focused test command.
   - Race command when concurrency, worker, HTTP server state, dedupe, retry,
     scheduling, cache, or shared state is part of the contract.
   - Serial Testcontainers note when Docker-backed tests are present.
7. `## Diagrams`
   - Include only when diagrams explain lifecycle, architecture, sequence,
     data flow, graph shape, or operator flow better than prose.
   - Link generated PNG assets from `docs/images/readme-diagrams/` when root
     README assets are involved.
8. `## Production Hardening`
   - Name durability, idempotency, retry, authentication, authorization,
     persistence, cloud, audit, observability, and operational gaps that remain
     outside the workshop scope.

### Korean README Sections

Keep the Korean README synchronized with the English README. Suggested section
labels:

1. `# <Example Name>`
2. `## 시나리오`
3. `## 통합하는 내용`
4. `## 실행`
5. `## API` or `## 워크플로우`
6. `## 검증`
7. `## 다이어그램`
8. `## 운영 환경 보강 지점`

Precise API/runtime terms such as `Gin`, `chi`, `net/http`, `Testcontainers`,
`Floci`, `outbox`, `checkpoint`, `idempotency`, and `race` may remain in
English when that matches the repository vocabulary.

## Implementation Template

Use this structure for future integration example design/spec work:

```markdown
# Issue #<number> Design: <Milestone> Integration Example

## Goal

Add the milestone-level integration example that combines <focused examples>
into one runnable <domain scenario>.

## Non-Goals

- Do not add <durability/cloud/framework scope not required by package behavior>.
- Do not import sibling example internal packages.
- Do not hide <package lessons> behind a generic framework.

## Example

- Path: `examples/<example-name>`
- Package: `internal/<packagename>`
- HTTP framework: `Gin` / `chi` / `net/http` / `None`
- Default address: `127.0.0.1:<port>` when HTTP is present
- Runtime requirement: `Local` / `Docker/Testcontainers` / `Floci`

## Scenario

Describe the domain flow, fixtures, failure paths, and expected output.

## Contracts

- Package contracts
- HTTP contracts, when applicable
- Error and status mapping
- Context/cancellation/resource ownership
- Concurrency/race ownership, when applicable

## Documentation Requirements

- `README.md`
- `README.ko.md`
- Root README navigation updates when the example is runnable
- Diagram assets only when they clarify the workflow

## Test Requirements

- Focused package tests
- HTTP handler tests, when applicable
- Failure and invalid input paths
- Cancellation/timeouts when context is part of the contract
- Race tests for shared state, goroutines, workers, dedupe, scheduling, or HTTP state
- Serial Testcontainers tests when Docker-backed resources are used
```

## Framework And Runtime Rubric

| Choice | Use when | Avoid when | Required evidence |
|---|---|---|---|
| Gin | The example is service-facing, owns public HTTP routes, or existing issue text calls for a Gin API. | The package lesson is repository, worker, graph fixture, or local batch behavior with no useful public boundary. | Route table, status/error mapping, request validation, timeouts when server ownership is in scope. |
| `net/http` | The lesson is standard-library compatibility or the issue explicitly needs standard handler shape. | Gin already matches the public API convention and no compatibility lesson exists. | Handler contract, status mapping, body-size/body-close behavior where applicable. |
| chi | Historical compatibility examples that already use chi, or an explicit router-compatibility issue. | New milestone integration examples without a chi-specific lesson. | Compatibility rationale and README note explaining why Gin is not used. |
| No HTTP | Focused package, worker, import/export, repository, graph, or local job behavior is clearer without an API wrapper. | The issue requires a public operations/query/workflow API. | CLI/workflow command, deterministic fixture, stable output. |
| Testcontainers | Real database, emulator, queue, cloud, or backend behavior is required to prove the package contract. | In-memory fixtures can prove the same workshop contract. | Docker requirement in README, serial test note, local-only credentials, focused command. |
| Floci | AWS-compatible local behavior is required. | Real AWS, IAM, deployment, or cloud-account behavior is being proposed. | No real credentials, endpoint override, serial emulator-backed tests. |
| Diagrams | Lifecycle, sequence, architecture, graph shape, or operator flow would be hard to inspect from text alone. | A diagram only repeats the README prose or requires root asset churn for a planning-only PR. | Generated PNG/SVG assets, generator command, geometry/rendering evidence when applicable. |

## Validation Rubric For Integration PRs

Every integration example PR should provide this evidence before merge:

| Gate | Required evidence |
|---|---|
| Scope | PR body links the umbrella issue, focused prerequisite issues, and integration issue. |
| README pair | `README.md` and `README.ko.md` exist and have equivalent sections. |
| Root navigation | Root `README.md` and `README.ko.md` list the runnable example once it exists. |
| Package contract | Tests prove the package lesson rather than only the HTTP wrapper. |
| Failure behavior | Tests cover at least one domain failure path and one invalid input path. |
| Context/resource behavior | Cancellation, timeout, body/rows/container close, or cleanup behavior is tested when relevant. |
| Race/concurrency | `go test -race` passes for shared state, workers, dedupe, scheduling, HTTP state, or goroutine-owned behavior. |
| Docker/emulator | Testcontainers and Floci tests are serial, local-only, and documented in README. |
| Diagrams | Diagram generation command and visual evidence are recorded when diagrams are changed. |
| Review | Review artifact reports `P0=0 P1=0` before PR review/CI gate. |
| PR body | Final PR body section is `## DoD Status` and includes validation, CI, and merge readiness. |

## Pasteable PR Checklist

Future integration example PRs can paste this into the final `## DoD Status`
section and trim rows that are truly not applicable.

```markdown
## DoD Status

| Check | Status | Evidence |
|---|---|---|
| Integration issue linked | PENDING | Closes #<issue>; umbrella #<umbrella>; prerequisites #<focused issues> |
| Runnable example added | PENDING | `examples/<example-name>` |
| English README | PENDING | `examples/<example-name>/README.md` |
| Korean README | PENDING | `examples/<example-name>/README.ko.md` |
| Root README navigation | PENDING | `README.md`; `README.ko.md` |
| Focused package tests | PENDING | `go test -count=1 ./examples/<example-name>/...` |
| Race gate | PENDING | `go test -race -count=1 ./examples/<example-name>/...` or N/A reason |
| Docker/Testcontainers gate | PENDING | Serial command or N/A reason |
| Diagram gate | PENDING | Generator command and rendered asset evidence or N/A reason |
| Static validation | PENDING | `git diff --check`; formatter/lint command |
| Review P0/P1 | PENDING | Review artifact with `P0=0 P1=0` |
| CI | PENDING | GitHub Actions check URL/result |
```

## README Navigation Plan

#49 already defines the root README behavior:

- The current example table lists runnable examples only.
- After #79, #80, and #81 are complete, the root `## Roadmap` section can gain
  a compact `Roadmap Planning` or planned-track matrix.
- Future integration templates belong in the planning-doc links under that
  roadmap section, not inside the runnable example table.
- When an actual integration example is implemented, update both `README.md`
  and `README.ko.md` in that example PR.

Recommended placement after #81:

```markdown
## Roadmap Planning

- [Workshop roadmap matrix](docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md)
- [Example selection scorecard](docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md)
- [Integration example template and acceptance rubric](docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md)
- [Cross-milestone integration blueprint](docs/superpowers/plans/<issue-81-file>.md)
```

Keep this section separate from implemented examples so future planned tracks do
not look runnable before their example directories exist.

## DoD Evidence For #80

- Template/rubric artifact exists in `docs/superpowers/plans`.
- Rubric references existing foundation integrations #72, #75, and #78.
- Rubric references future integration issues #65, #66, #67, #68, and #69.
- English and Korean README section templates are defined.
- Validation evidence expected in integration PRs is defined.
- Gin, `net/http`, chi, Testcontainers, Floci, and diagram usage rules are
  explicit.
- Pasteable future PR checklist is included.
- README navigation plan explains where this template belongs after #81.
