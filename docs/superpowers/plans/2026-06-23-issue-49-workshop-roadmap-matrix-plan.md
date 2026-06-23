# Issue #49 Plan: Workshop Roadmap Matrix and README Expansion

## Goal

Create the planning artifact that keeps the larger `bluetape-go-workshop`
example set navigable before 0.8.0+ implementation starts.

Issue #49 asks for docs/superpowers plan material, not a root README rewrite in
this PR. The root README work below is an execution plan for later example PRs
and for the remaining 0.7.0 planning issues.

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #49 `[v0.7.0] Add workshop roadmap matrix and README expansion plan`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- Work type: Type E - Planning / Maintenance

## Sources Checked

- GitHub issues:
  - #27 epic roadmap
  - #31 0.7.0 research/planning parent
  - #47 graph/text candidate research
  - #48 audit/AWS/SQL candidate research
  - #28, #29, #30 completed 0.4.0-0.6.0 umbrella tracks
  - #32, #33, #34, #35, #36 future 0.8.0-0.12.0 umbrella tracks
  - #38-#46, #50-#69, #70-#81 child issues
- Current repository files:
  - `README.md`
  - `README.ko.md`
  - `docs/superpowers/research/2026-06-23-issue-47-graph-text-candidates-research.md`
  - `docs/superpowers/research/2026-06-23-issue-48-audit-aws-sql-candidates-research.md`
  - `docs/superpowers/plans/2026-06-22-issue-29-customer-migration-batch-integration-plan.md`
- Retrieval:
  - Context-mode search for #49 roadmap matrix and HTTP framework decisions.
  - `gno query "bluetape-go-workshop issue 49 roadmap matrix README expansion #27 #31" -c bluetape4k-github --fast --no-rerank`
  - `gno query "bluetape-go-workshop roadmap matrix README expansion examples milestones" -c bluetape4k-docs --no-rerank`

## Roadmap Matrix

| Milestone | Track / issue | Examples and issue links | Package coverage | HTTP framework choice | Docker/Testcontainers requirement | README / diagram work |
|---|---|---|---|---|---|---|
| 0.4.0 | State and workflow, #28 | #38 order lifecycle state API, #39 fulfillment workflow runner, #40 operations report policy, #70 payment authorization state, #71 compensation workflow, #72 order fulfillment integration | `state`, `workflow`, `workreport` | Gin for every public HTTP API in this milestone | No Docker required; examples are in-memory/service-local | Already in root README. Keep milestone group and example map current when diagrams are regenerated. |
| 0.5.0 | Batch checkpoint/restart, #29 | #41 account migration checkpoint restart, #73 chunked CSV import checkpoint, #74 retry/dead-letter worker, #42/#43/#75 merged into customer migration batch integration | `batch`, `leader` for scheduled integration | Local jobs use no HTTP framework. #75 uses Gin for operations API and scheduled tick endpoints. | No Docker required in current examples; leader gate is deterministic/local in #75 | Already in root README. Document #42/#43 as integrated into #75 to avoid duplicate README entries. |
| 0.6.0 | Portable utilities, #30 | #44 ID/JWT boundary, #76 token refresh claims, #45 money pricing, #77 multi-currency invoice, #46 probabilistic dedupe, #78 checkout guard integration | `id`, `jwt`, `money`, `probabilistic` | Gin for public utility boundary APIs | No Docker required; examples are local HTTP services | Already in root README. Keep the checkout guard as the milestone integration route. |
| 0.7.0 | Research and planning, #31 | #47 graph/text research, #48 audit/AWS/SQL research, #49 roadmap matrix, #79 scorecard, #80 integration template/rubric, #81 cross-milestone blueprint | Planning over SQL, AWS, text, audit, graph tracks | N/A; planning artifacts only | N/A; no runnable examples in this milestone unless upstream packages become available | Add planning links under a future "Roadmap Planning" section only after #79-#81 are complete. |
| 0.8.0 | SQL DSL/repository, #32 | #62 SQL order repository, #63 SQL transaction boundary, #64 Gin SQL CRUD API, #65 Gin SQL order service integration | SQL DSL/repository helpers, transactions, repository boundaries | #62/#63 use no public HTTP boundary. #64/#65 use Gin. | Medium to high. Use Postgres/Testcontainers when real SQL transaction/lock behavior is required; otherwise keep fast local tests for handler-only coverage. | Add SQL milestone group with focused examples first, then #65 as integration. Add DB setup notes and Docker badge/legend. |
| 0.9.0 | AWS/Floci, #33 | #59 S3 Floci storage, #60 SQS Floci worker, #61 DynamoDB conditional repository, #66 S3-SQS-DynamoDB document workflow integration | AWS SDK for Go v2, Floci/Testcontainers, S3, SQS, DynamoDB | #59/#60/#61 default to package/worker examples without public HTTP. #66 may use Gin if it exposes a document workflow API. | High to very high. Floci/Testcontainers is required for local AWS behavior; no real AWS credentials. | Add AWS/Floci milestone group with a Docker-required marker and explicit local emulator setup links. |
| 0.10.0 | Text search/tokenizer, #34 | #53 text moderation masking, #54 Gin text search service, #55 tokenizer/language detection feasibility, #67 Gin content moderation workflow integration | Text search, masking, tokenizer/language feasibility | #53/#55 use no public HTTP boundary. #54/#67 use Gin. | Low. Keep examples local and deterministic; avoid network/model dependencies. | Add text milestone group with Unicode/language caveat notes and #67 integration route. |
| 0.11.0 | Audit/event/outbox, #35 | #56 audit order history, #57 transactional outbox publisher, #58 Gin audit query API, #68 audited order workflow outbox integration | Audit/event model, outbox, query API | #56/#57 use no public HTTP boundary. #58/#68 use Gin. | Low to high. Start storage-neutral where possible; use SQL/Testcontainers only when durable outbox behavior is being proved. | Add audit/outbox group with "audit is not event sourcing" and operator replay caveats. |
| 0.12.0 | Graph domain examples, #36 | #50 graph abuse cluster, #51 graph recommendation, #52 graph import/export, #69 graph risk intelligence integration | Graph abstraction, graph I/O, traversal, scoring | #50/#51/#52 use no public HTTP boundary by default. #69 should stay package-first unless it deliberately exposes a Gin risk API. | Low to high. Start with in-memory/domain fixtures; defer multi-backend Testcontainers until upstream adapters are stable. | Add graph milestone group with domain-scenario ordering and import/export fixture links. |

## HTTP Framework Decision Table

| Example / issue | Decision | Reason |
|---|---|---|
| Existing 0.4.0 state/workflow APIs | Gin | Public HTTP APIs are part of the lesson. |
| Existing 0.5.0 local batch jobs | None | The lesson is checkpoint/retry behavior, not routing. |
| Existing #75 customer migration integration | Gin | It owns the batch operations API and scheduled tick endpoints. |
| Existing 0.6.0 utility APIs | Gin | They are public HTTP trust-boundary examples. |
| Existing leader/resilience compatibility examples before 0.4.0 | chi | Keep historical compatibility-focused examples unchanged. |
| Future #62, #63, #53, #55, #56, #57, #59, #60, #61, #50, #51, #52 | None by default | These teach repository, worker, text, audit, AWS, or graph package behavior directly. Add HTTP only if the issue asks for an API. |
| Future #64, #65, #54, #67, #58, #68 | Gin | The issue explicitly asks for a public API or milestone integration API. |
| Future #66 | Gin if an API is exposed; otherwise none | The scenario is a document workflow. Do not add HTTP unless it clarifies upload/status behavior. |
| Future #69 | Gin only if the integration exposes a risk API | Keep graph package behavior visible; avoid a decorative service wrapper. |
| net/http | Use only for compatibility lessons | No current 0.8.0-0.12.0 issue requires a plain `net/http` boundary. |

## Root README Expansion Plan

### 1. Keep The Current Example Table As The Source Of Implemented Routes

The current root README table should continue to list only runnable examples
that exist under `examples/`. Do not add future issue rows as if they were
implemented.

Required per implemented example:

- English and Korean README links.
- One-sentence scenario purpose.
- bluetape-go package coverage.
- Public HTTP framework only when it matters to the lesson.

### 2. Add A Milestone Roadmap Section For Planned Tracks

After #79-#81 are complete, expand the existing `## Roadmap` section into a
compact planned-track matrix that mirrors the matrix in this document:

- Milestone and umbrella issue.
- Focused examples.
- Integration example.
- Runtime requirement: `Local`, `Docker/Testcontainers`, or `TBD`.
- HTTP boundary: `Gin`, `chi`, `net/http`, or `None`.
- Documentation status: `planned`, `implemented`, or `needs diagram refresh`.

This keeps future work navigable without mixing planned examples into the
implemented examples table.

### 3. Add A Runtime Legend

Add a short legend before the roadmap matrix:

- `Local`: runs without Docker or external service.
- `Docker/Testcontainers`: requires Docker for integration proof.
- `Floci`: local AWS emulator; no real AWS credentials.
- `Gin`: framework-visible public API example.
- `None`: package, worker, or local job example with no HTTP boundary.

### 4. Keep README.md And README.ko.md Synchronized

Every README navigation change must update both files in the same PR. The
Korean README may keep domain terms such as `Gin`, `Testcontainers`, `Floci`,
`outbox`, and `checkpoint` in English when those are the precise API/runtime
terms used by the examples.

### 5. Diagram Update Policy

Do not regenerate the root example map for every planning-only issue. Update
the root diagram when either:

- a milestone integration example is implemented, or
- #81 finalizes the cross-milestone integration blueprint and changes the
  learning route.

Individual example diagrams remain owned by each example README.

## Execution Guidance For Later Issues

- Each future milestone should add focused examples first and the integration
  example last.
- Integration READMEs should link their focused prerequisites.
- Testcontainers-backed examples must be called out in the root README and must
  run serially when they share Docker resources.
- Public HTTP APIs should default to Gin. Use `chi` or `net/http` only when the
  issue explicitly needs compatibility or standard-library handler shape.
- 0.7.0 should remain planning-only unless upstream package availability makes a
  runnable example low-risk and explicitly scoped.

## DoD Evidence For #49

- Matrix covers 0.4.0 through 0.12.0.
- Gin vs `net/http` / `chi` / no-HTTP choice is explicit per current and planned
  HTTP example category.
- Root README expansion plan is included.
- Docker/Testcontainers requirements are visible per milestone.
- The plan references #47 and #48 research instead of reopening candidate
  selection.
