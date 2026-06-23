# Issue #79 Plan: Example Selection Scorecard

## Goal

Create the 0.7.0 planning scorecard that turns the #47, #48, and #49
research into an actionable sequence for the larger post-utility workshop
tracks.

Issue #79 is planning-only. It does not add runnable examples or root README
navigation in this PR. The README expansion remains gated by the #49 plan:
add planning links after #79, #80, and #81 are complete.

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #79 `[v0.7.0] Add bluetape4k-workshop example selection scorecard`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- Work type: Type E - Planning / Maintenance

## Sources Checked

- GitHub issues:
  - #31 0.7.0 research/planning parent
  - #32 SQL umbrella
  - #33 AWS/Floci umbrella
  - #34 text umbrella
  - #35 audit/outbox umbrella
  - #36 graph umbrella
  - #47 graph/text candidate research
  - #48 audit/AWS/SQL candidate research
  - #49 roadmap matrix and README expansion plan
  - #50-#69 accepted or gated candidate issues
- Current repository planning docs:
  - `docs/superpowers/research/2026-06-23-issue-47-graph-text-candidates-research.md`
  - `docs/superpowers/research/2026-06-23-issue-48-audit-aws-sql-candidates-research.md`
  - `docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md`
- Retrieval:
  - Context-mode search for #79, #47, #48, and #49 planning history.
  - `gno query "bluetape-go-workshop issue 79 scorecard #32 #33 #34 #35 #36" -c bluetape4k-github --fast --no-rerank`

## Scoring Rubric

Scores are 1 to 5, where 5 is strongest for the workshop path.

| Dimension | Meaning |
|---|---|
| Package maturity | How ready the upstream or expected `bluetape-go` package surface is for a stable example. |
| Workshop value | How clearly the example teaches a domain-shaped lesson rather than a thin API tour. |
| Dependency lightness | How small the runtime/test dependency cost is. A local deterministic example scores higher than a Docker/cloud-heavy one. |
| Testability | How easily the behavior can be verified with deterministic tests and fixtures. |
| Source similarity | How directly the example maps to proven `bluetape4k-workshop` source examples or package research. |

Decision rules:

- `Accept first`: lead example for the umbrella track.
- `Accept after`: valuable and in scope, but depends on a focused predecessor.
- `Accept bounded`: in scope only with the stated reduced contract.
- `Defer`: do not implement until the named upstream or dependency condition is met.
- `Reject`: keep out of the workshop path unless a future issue changes the constraints.

The total score is a prioritization signal, not an automatic schedule. Umbrella
milestone order, prerequisite issues, and dependency readiness still control the
actual implementation sequence.

## Candidate Scorecard

### SQL Track - #32

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #62 | SQL order repository | 4 | 5 | 3 | 4 | 4 | 20 | Accept first. It is the smallest repository-shaped SQL lesson and keeps SQL visible. |
| #63 | SQL transaction boundary | 4 | 5 | 2 | 4 | 4 | 19 | Accept after #62. It needs the repository model before teaching rollback, lock, and service-owned transaction behavior. |
| #64 | Gin SQL CRUD API | 4 | 4 | 3 | 4 | 3 | 18 | Accept after #62. Keep Gin at the HTTP boundary and reuse the repository contract. |
| #65 | Gin SQL order service integration | 3 | 5 | 2 | 4 | 4 | 18 | Accept after #62, #63, and #64. This is the 0.8.0 integration route, not the first SQL lesson. |

SQL notes:

- #62 anchors the track because it can prove package helper usage without
  inventing a Go ORM.
- #63 and #65 may require Postgres/Testcontainers if lock or rollback semantics
  are in scope.
- #64 should stay handler-focused once repository behavior is already proven.

### AWS / Floci Track - #33

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #59 | S3 Floci storage | 3 | 5 | 2 | 3 | 5 | 18 | Accept first. It establishes the local AWS emulator and credential story. |
| #60 | SQS Floci worker | 3 | 4 | 2 | 3 | 4 | 16 | Accept after #59. It should reuse the Floci setup and keep delivery/idempotency boundaries explicit. |
| #61 | DynamoDB conditional repository | 3 | 5 | 2 | 4 | 3 | 17 | Accept after #59, independently from #60. It adds deterministic conditional-write and conflict behavior. |
| #66 | S3-SQS-DynamoDB document workflow integration | 2 | 5 | 1 | 3 | 4 | 15 | Accept after #59, #60, and #61. Keep it as the 0.9.0 integration example and run emulator-backed tests serially. |

AWS notes:

- The lower dependency-lightness scores are intentional. Local AWS behavior
  requires Docker and Floci/Testcontainers, but real AWS credentials remain out
  of scope.
- #66 should not add IAM, deployment, or real cloud setup.

### Text Track - #34

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #53 | Text moderation masking | 4 | 5 | 5 | 5 | 4 | 23 | Accept first. It is deterministic, local, and maps directly to text search and masking behavior. |
| #54 | Gin text search service | 4 | 4 | 5 | 5 | 3 | 21 | Accept after #53. It exposes the text behavior through a public API without making HTTP the lesson. |
| #55 | Tokenizer / language detection feasibility | 2 | 4 | 3 | 3 | 4 | 16 | Accept bounded. Keep it feasibility-only until dependency, dictionary, license, and binary-size risks are resolved. |
| #67 | Gin content moderation workflow integration | 3 | 5 | 4 | 4 | 4 | 20 | Accept after #53, #54, and #55. It composes moderation, search, and language handling into one deterministic workflow. |

Text notes:

- #53 is the strongest first candidate because it can be fully local and
  observable.
- #55 must not become a production tokenizer dependency decision by accident.
- #67 should link the focused examples instead of repeating every primitive.

### Audit / Outbox Track - #35

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #56 | Audit order history | 3 | 5 | 5 | 5 | 4 | 22 | Accept first. It teaches immutable audit history without claiming event-sourcing semantics. |
| #57 | Transactional outbox publisher | 2 | 5 | 2 | 3 | 5 | 17 | Accept after #56 or once upstream outbox primitives are stable. It owns the dual-write lesson. |
| #58 | Gin audit query API | 3 | 4 | 4 | 4 | 3 | 18 | Accept after #56. Keep query/storage behavior framework-independent. |
| #68 | Audited order workflow outbox integration | 2 | 5 | 2 | 3 | 4 | 16 | Accept after #56, #57, and #58. It is the 0.11.0 integration route. |

Audit notes:

- #56 keeps the first pass storage-neutral if upstream package contracts allow
  it.
- #57 may need durable SQL/Testcontainers, but Kafka-like broker emulation
  should stay deferred until storage/outbox claim behavior is stable.
- #68 should prove audit rows and outbox rows stay consistent.

### Graph Track - #36

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #50 | Graph abuse cluster | 3 | 5 | 4 | 5 | 5 | 22 | Accept first. It is concrete, security-shaped, and maps directly to the Kotlin abuser-detection example. |
| #51 | Graph recommendation | 3 | 4 | 4 | 5 | 5 | 21 | Accept after #50. It adds ranking and tie-breaking without needing a backend matrix. |
| #52 | Graph import/export fixtures | 2 | 4 | 3 | 4 | 4 | 17 | Defer until upstream graph I/O helpers are stable, then accept as the fixture path for later graph examples. |
| #69 | Graph risk intelligence integration | 2 | 5 | 3 | 4 | 4 | 18 | Accept after #50, #51, and #52. It is the 0.12.0 integration route. |

Graph notes:

- #50 and #51 should start with small deterministic fixtures and no backend
  abstraction layer.
- #52 is useful only if it uses upstream graph I/O, not a workshop-only loader.
- #69 should keep scoring explainable and package behavior visible.

## Rejected Or Deferred Patterns

| Track | Pattern | Decision | Reason |
|---|---|---|---|
| SQL - #32 | Kotlin Exposed clone | Reject | Go examples should keep SQL and runtime contracts visible instead of recreating a Kotlin ORM shape. |
| SQL - #32 | Mandatory code generation path | Reject for now | The current research does not prove codegen is the smallest safe workshop path. |
| SQL - #32 | R2DBC/coroutine-style port | Reject | Go examples should use `context.Context` and database contracts, not Kotlin coroutine architecture. |
| AWS - #33 | Broad AWS SDK wrapper | Reject | Helpers should exist only where repeated-service value is clear. |
| AWS - #33 | Real AWS integration tests | Reject | Workshop DoD must stay local and deterministic with Floci/Testcontainers. |
| AWS - #33 | IAM, STS, Lambda, or deployment examples | Defer | They introduce credential and cloud-account scope outside #33. |
| Text - #34 | Spring Data Elasticsearch port | Reject | It is external-service and Spring-shaped, while #34 starts with local text behavior. |
| Text - #34 | Full production Korean/Japanese tokenizer example | Defer | Dependency, dictionary, license, and binary-size decisions are unresolved. |
| Text - #34 | Generic text framework scaffolding | Reject | The workshop needs scenario-shaped observable behavior. |
| Audit - #35 | JaVers clone | Reject | The Go track should port audit concepts, not JaVers internals. |
| Audit - #35 | Generic application logging example | Reject | #35 requires audit/event semantics, not log aggregation. |
| Audit - #35 | Kafka-backed publisher tests | Defer | Storage/outbox claim behavior should be stable before broker emulation is added. |
| Audit - #35 | Event-sourcing claims | Defer | Audit history can be append-only without owning aggregate reconstruction semantics. |
| Graph - #36 | Broad backend abstraction | Reject | It would hide backend-specific graph behavior too early. |
| Graph - #36 | Multi-backend Testcontainers matrix | Defer | Useful for upstream confidence, too heavy for first workshop graph examples. |
| Graph - #36 | Standalone knowledge-graph example | Defer | Valuable, but broader than the first abuse-cluster and recommendation pass. |
| Graph - #36 | Bare traversal snippets | Reject | #36 asks for domain graph scenarios, not isolated API demonstrations. |

## Recommended Sequencing

Use parent #31 milestone order first:

1. 0.8.0 SQL - #32: #62, #63, #64, then #65.
2. 0.9.0 AWS/Floci - #33: #59, then #60 and #61, then #66.
3. 0.10.0 text - #34: #53, #54, bounded #55, then #67.
4. 0.11.0 audit/outbox - #35: #56, #57 and #58, then #68.
5. 0.12.0 graph - #36: #50, #51, deferred #52, then #69.

When implementation begins, each umbrella issue should keep the same pattern:
focused examples first, integration issue last, and README navigation only
after runnable examples exist.

## DoD Evidence For #79

- Scorecard covers graph, text, audit, AWS, and SQL candidates.
- Every candidate issue #50-#69 has an explicit decision.
- Decisions link back to umbrella issues #32, #33, #34, #35, and #36.
- Accepted, bounded, deferred, and rejected examples are separated with reasons.
- README navigation is intentionally deferred until #79-#81 are complete, as
  planned by #49.
