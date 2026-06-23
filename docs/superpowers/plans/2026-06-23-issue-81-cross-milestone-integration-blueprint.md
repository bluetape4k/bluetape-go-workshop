# Issue #81 Plan: Cross-Milestone Integration Blueprint

## Goal

Define the workshop integration spine that connects the implemented foundation
examples from 0.4.0 through 0.6.0 to the larger planned SQL, AWS, text, audit,
and graph domains from 0.8.0 through 0.12.0.

Issue #81 is planning-only. It should make the future path visible without
prematurely implementing packages that belong to later `bluetape-go`
milestones.

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #81 `[v0.7.0] Add cross milestone workshop integration blueprint`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- Work type: Type E - Planning / Maintenance

## Sources Checked

- GitHub issues:
  - #27 milestone-aligned workshop roadmap epic
  - #31 0.7.0 research/planning parent
  - Foundation integrations: #72, #75, #78
  - Future integrations: #65, #66, #67, #68, #69
  - Planning artifacts: #49, #79, #80
- Current repository planning/spec docs:
  - `docs/superpowers/specs/2026-06-09-issue-72-order-fulfillment-integration-design.md`
  - `docs/superpowers/specs/2026-06-22-issue-29-customer-migration-batch-integration-design.md`
  - `docs/superpowers/specs/2026-06-23-issue-78-checkout-guard-design.md`
  - `docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md`
  - `docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md`
  - `docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md`
- Retrieval:
  - Context-mode search for #81, #49, #80, #27, and integration-spine history.
  - `gno query "bluetape-go-workshop issue 81 cross milestone workshop integration blueprint #31 #49 #79 #80" -c bluetape4k-github --fast --no-rerank`

## Spine Principle

The workshop should read as a widening path:

1. Foundation examples teach local service behavior, batch recovery, and
   portable utility trust boundaries.
2. 0.7.0 planning artifacts define what belongs in later domains and what stays
   out of scope.
3. Future examples reuse the same domain nouns and integration habits without
   turning the repository into one broad demo application.

The spine is not a single runnable app. It is the narrative and sequencing rule
that keeps each milestone useful on its own while making the next milestone
feel like a continuation.

## Foundation Integration Anchors

| Issue | Milestone | Implemented integration anchor | Reusable lesson |
|---|---|---|---|
| #72 | 0.4.0 | Order fulfillment workflow integration | Domain state, workflow steps, work reports, compensation, and failure projection can compose into one service-facing scenario. |
| #75 | 0.5.0 | Customer migration batch integration | Batch checkpoint/restart, operations API, scheduled leader guard, retry, and dead-letter handling can compose into operationally visible recovery behavior. |
| #78 | 0.6.0 | Checkout guard integration | ID/JWT, money/rules, and probabilistic dedupe utilities can compose at an HTTP trust boundary without becoming a generic framework. |

These examples establish the pattern future milestones should reuse:

- focused examples first
- integration example last
- one concrete domain scenario
- deterministic fixtures
- English/Korean README walkthroughs
- local validation evidence before PR merge
- explicit production hardening notes

## Future Integration Path

| Issue | Milestone | Planned integration | Foundation bridge | Primary nouns | Runtime posture |
|---|---|---|---|---|---|
| #65 | 0.8.0 SQL | Gin SQL order service integration | Extends #72 order lifecycle and #78 checkout order IDs into persisted order, item, and status tables. | order, order item, status, transaction | Local first; Postgres/Testcontainers only when transaction or lock semantics require it. |
| #66 | 0.9.0 AWS/Floci | S3-SQS-DynamoDB document workflow integration | Extends #75 operational recovery and #78 request identity into document ingestion, queued processing, and idempotent state. | document, processing event, idempotency key, customer/account | Floci/Testcontainers local emulator; no real AWS credentials. |
| #67 | 0.10.0 text | Gin content moderation workflow integration | Extends #78 trust-boundary checks into text moderation and search records; can feed #69 risk signals later. | content, language, token, moderation record | Local deterministic fixtures; dependency-heavy tokenizers remain bounded. |
| #68 | 0.11.0 audit/outbox | Audited order workflow outbox integration | Extends #72 order state changes and #65 SQL persistence into auditable events and outbox handoff. | order, audit event, outbox record, publisher attempt | Storage-neutral first; SQL/Testcontainers when durable outbox behavior is required. |
| #69 | 0.12.0 graph | Graph risk intelligence integration | Pulls risk signals from #66 documents/events, #67 moderation records, and #68 audit events into graph traversal and scoring. | account, device, document, risk event, cluster | In-memory/domain fixtures first; backend matrices deferred until upstream adapters are stable. |

## Shared Domain Nouns

Use shared nouns deliberately so examples feel connected, but do not force a
global schema.

| Noun | Starts in | Carries forward to | Rule |
|---|---|---|---|
| `order` | #72, #78 | #65, #68 | Keep order identifiers stable and domain-shaped; do not share Go packages across examples. |
| `customer` | #75, #78 | #65, #66, #67 | Use deterministic fixture customers and avoid real personal data. |
| `document` | #66 | #67, #69 | Treat documents as local fixtures or emulator objects; no real cloud data. |
| `account` | #69 | #66, #68, #69 | Use opaque or synthetic account IDs when examples touch risk or graph analysis. |
| `risk event` | #67, #68 | #69 | Keep risk events explainable and deterministic; no model or opaque scoring dependency. |
| `idempotency key` | #75, #78 | #66, #68 | State whether the key is authoritative, probabilistic, or demo-only in each README. |

The shared noun contract is documentation-level. Each example remains
self-contained under its own `examples/<name>` directory.

## Sequencing Rules

1. Do not begin an integration issue until its focused prerequisite issues have
   proven the package contracts.
2. Keep future milestones in parent #31 order: #65, #66, #67, #68, then #69.
3. Within each milestone, prefer the #79 scorecard sequence over ad hoc issue
   selection.
4. Use the #80 template when drafting each integration issue's design, README,
   validation plan, and PR DoD table.
5. Update root README navigation only when a runnable example exists, except
   for the 0.7.0 planning links added by this blueprint PR.
6. Add diagrams only when they clarify lifecycle, data flow, graph shape,
   sequence, or operator behavior.
7. Keep real external services out of workshop DoD. Use local deterministic
   fixtures, Testcontainers, and Floci where package behavior requires service
   semantics.

## Integration Spine By Reader Journey

### Path A: Order And Consistency

1. #72 teaches lifecycle and compensation in one order workflow.
2. #78 introduces checkout trust boundaries and generated request/order IDs.
3. #65 persists order data through repository and transaction contracts.
4. #68 records order changes as audit events and outbox records.
5. #69 can later inspect account/order risk signals as graph inputs.

### Path B: Customer And Operations

1. #75 teaches batch checkpoint/restart, operations visibility, and leader
   guarded scheduling.
2. #66 reuses the operational pattern for queued document processing and
   idempotent state.
3. #67 moderates customer/user content with deterministic text behavior.
4. #69 can connect customer, account, document, and moderation risk signals.

### Path C: Trust Boundary And Risk

1. #78 teaches JWT claims, money/rule decisions, and probabilistic dedupe at a
   service boundary.
2. #67 turns text behavior into moderation decisions.
3. #68 turns domain behavior into auditable facts.
4. #69 turns events and relationships into explainable graph risk intelligence.

## README Roadmap Placement

The root README should point to this integration spine as planning material,
not as a runnable route.

The `Roadmap Planning` section belongs directly after the milestone roadmap
table. It should link:

- #49 roadmap matrix
- #79 example selection scorecard
- #80 integration template/rubric
- #81 cross-milestone integration blueprint

This keeps #27 and README roadmap readers oriented without mixing planned
tracks into the implemented examples table.

## Stop Conditions For Future Work

Future issue authors should stop and split work when an integration issue tries
to add any of these:

- a new shared framework across examples
- a production deployment story
- real cloud credentials or IAM setup
- a multi-backend matrix before a focused example proves the surface
- a broad graph/backend abstraction
- an opaque AI/model dependency
- README rows for examples that do not exist yet

Those topics can become separate research or implementation issues, but they
should not be smuggled into an integration PR.

## DoD Evidence For #81

- Blueprint artifact exists in `docs/superpowers/plans`.
- Blueprint links #72, #75, #78, #65, #66, #67, #68, and #69.
- Shared domain nouns are mapped across foundation and future milestones.
- Sequencing rules point to #31, #79, and #80.
- README roadmap placement explains how #27 and the root README can point to
  this integration spine.
- Root `README.md` and `README.ko.md` include a `Roadmap Planning` section with
  links to #49, #79, #80, and #81 planning artifacts.
