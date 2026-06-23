# Issue #48 Research: Audit, AWS, and SQL Workshop Candidates

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #48 `[v0.7.0] Research audit AWS and SQL workshop candidates`
- Parent track: #31 `[v0.7.0] Plan post-utility workshop tracks`
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- Work type: Type E - Research / Maintenance

## Sources Checked

- GitHub issue #48, parent #31, and follow-up workshop issues:
  - SQL: #32, #62, #63, #64, #65
  - AWS: #33, #59, #60, #61, #66
  - Audit/outbox: #35, #56, #57, #58, #68
- Upstream bluetape-go research:
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.8.0-sql-research.md`
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.9.0-aws-research.md`
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.11.0-audit-javers-research.md`
- Kotlin workshop examples:
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/exposed/mvc-jdbc/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/exposed/webflux-r2dbc/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/exposed/javers-audit/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/messaging/transactional-outbox/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/aws/s3-spring-cloud/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/aws/storage-abstraction/README.md`
- Upstream package references:
  - `/Users/debop/work/bluetape4k/bluetape4k-exposed/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-aws/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-javers/README.md`
- Retrieval:
  - Context-mode search for #48 / audit / AWS / SQL workshop history.
  - `gno query "bluetape-go-workshop issue 48 audit AWS SQL candidates #32 #33 #35" -c bluetape4k-github --fast --no-rerank`
  - `gno query "bluetape-go 0.8.0 SQL 0.9.0 AWS 0.11.0 audit research" -c bluetape4k-docs --no-rerank`

## Current Evidence

Issue #48 asks for candidate selection before implementation. Parent #31 now
maps the relevant tracks as:

- 0.8.0 SQL DSL and repository helpers (#32)
- 0.9.0 AWS helper packages and Floci-backed examples (#33)
- 0.11.0 audit/event packages and outbox-style workflows (#35)

The upstream bluetape-go research already sets the guardrails:

- SQL should keep SQL visible, avoid a full ORM layer, avoid a Kotlin Exposed
  clone, and focus on `context.Context`, transactions, explicit errors, and safe
  query construction.
- AWS should stay helper/example driven, avoid wrapping AWS SDK for Go v2
  without repeated-service evidence, and use Floci/Testcontainers for local AWS
  behavior.
- Audit should port concepts, not JaVers internals, and start with a
  storage-neutral audit/event model before durable publisher adapters.

Some upstream research issue references are stale because the workshop issue
map has since changed. This note uses the current workshop issues from #31,
#32, #33, and #35 as the source of truth.

## Accepted SQL Candidates

### #62 SQL Order Repository

Accept as the first SQL workshop example.

Reasons:

- It maps cleanly to Kotlin `exposed/mvc-jdbc`, where repositories own table
  access and services keep transaction boundaries visible.
- It can teach insert, find, filtered list, and not-found behavior without
  forcing a broad ORM abstraction.
- It fits the upstream Go direction: package-first SQL helpers, visible SQL,
  explicit errors, and context ownership.

Docker/Testcontainers cost:

- Medium. Use Postgres/Testcontainers only if the upstream SQL helper requires
  real database semantics. If sqlite or a package-provided in-memory fixture can
  prove the same repository contract, keep the first pass Docker-free.

Implementation boundary:

- Do not assert behavior only through SQL string snapshots.
- Keep direct `database/sql` comparison in README, but put package helper usage
  in the example code.

### #63 SQL Transaction Boundary

Accept after #62.

Reasons:

- It maps to the order placement flow in `exposed/mvc-jdbc` and
  `exposed/webflux-r2dbc`: order header, lines, product stock, lock ordering,
  and rollback on stock conflict.
- It teaches the most important production contract for SQL helpers:
  transaction ownership belongs in the service boundary.
- It gives the track a concrete failure case instead of CRUD-only behavior.

Docker/Testcontainers cost:

- Medium to high. Rollback and lock behavior are stronger with a real database.
  If row locking is in scope, Postgres/Testcontainers should be used and tests
  should run serially.

Implementation boundary:

- Keep transaction lifetime explicit.
- Include context cancellation notes in README.
- Do not invent a heavy unit-of-work or ORM layer.

### #64 Gin SQL CRUD API

Accept as the HTTP wrapper after #62.

Reasons:

- It gives the SQL repository a public API shape while keeping Gin isolated at
  the HTTP boundary.
- It can reuse the same order repository behavior instead of creating another
  persistence lesson.
- It aligns with the workshop convention that public HTTP examples use Gin
  unless the issue requires `net/http`.

Docker/Testcontainers cost:

- Medium if it reuses the repository's real database tests. Keep handler-only
  tests fast by using package-level interfaces or local test stores when the
  repository contract is already proven elsewhere.

Implementation boundary:

- Handlers should validate and project responses; repositories must remain
  usable without Gin.
- README should include migration/setup notes and curl examples.

### #65 Gin SQL Order Service Integration

Accept as the 0.8.0 integration example after #62, #63, and #64.

Reasons:

- It combines repository modeling, transaction boundaries, and HTTP API without
  duplicating each focused lesson.
- It is scenario-shaped: order creation, status lookup, item updates, and
  rollback/failure behavior.
- It should explain how it differs from the smaller SQL examples.

Docker/Testcontainers cost:

- Medium to high. If the integrated example covers multi-table writes and
  rollback, prefer a real database-backed test. Keep the suite bounded and run
  Testcontainers serially.

Implementation boundary:

- Do not expand into inventory, fulfillment, payment, or outbox behavior.
- Link README prerequisites to #62, #63, and #64.

## Deferred or Rejected SQL Candidates

- Reject a Kotlin Exposed clone. The upstream Go SQL research explicitly says
  to avoid a full ORM layer and keep Go APIs runtime-first.
- Reject mandatory code generation for the workshop path until upstream SQL
  research proves it is the smallest safe path.
- Defer R2DBC/coroutine-style ports. Go examples should use Go context and
  database contracts, not mirror Kotlin coroutine architecture.

## Accepted AWS Candidates

### #59 S3 Floci Storage

Accept as the first AWS workshop example.

Reasons:

- It maps directly to Kotlin `aws/s3-spring-cloud` and the S3 profile in
  `aws/storage-abstraction`.
- S3 object lifecycle is the smallest useful AWS behavior: bucket/object setup,
  upload, download, list, delete, and optionally pre-signed URL behavior.
- It keeps AWS credentials local-only through Floci and SDK endpoint overrides.

Docker/Testcontainers cost:

- High but intentional. #59 requires Floci/Testcontainers-only tests. The README
  must state Docker is required, no real AWS credentials are used, and real AWS
  differences are outside the local proof.

Implementation boundary:

- Use caller-owned AWS SDK clients.
- Keep the storage abstraction small and avoid wrapping the whole AWS SDK.

### #60 SQS Floci Worker

Accept after #59 proves the Floci fixture.

Reasons:

- It maps to the AWS helper track and gives a concrete producer/consumer lesson.
- It can teach message round trip, handler success, retry-visible failure, and
  idempotency requirements.
- It should stay independent from S3/DynamoDB until the integration issue.

Docker/Testcontainers cost:

- High. SQS behavior should be verified through Floci/Testcontainers. Tests
  must be serial when sharing the emulator.

Implementation boundary:

- README must state delivery semantics and idempotency requirements.
- Dead-letter or parking-lot behavior should be documented only if the upstream
  helper/emulator support is clear.

### #61 DynamoDB Conditional Repository

Accept as the AWS persistence-focused example.

Reasons:

- It targets a bluetape-specific pain point: conditional writes, optimistic
  updates, partition-key query behavior, and conflict mapping.
- It complements S3 and SQS without creating one oversized demo.
- It gives #66 a deterministic idempotency state store.

Docker/Testcontainers cost:

- High. Conditional writes need emulator-backed behavior; use Floci/local
  emulator tests and avoid real credentials.

Implementation boundary:

- Keep key design and consistency caveats in README.
- Do not combine this with S3/SQS before #66.

### #66 S3-SQS-DynamoDB Document Workflow Integration

Accept as the 0.9.0 integration example after #59, #60, and #61.

Reasons:

- It composes the focused AWS examples into one realistic local workflow:
  upload object, enqueue processing, and record idempotent state.
- It can prove retry-safe message processing and conditional write behavior.
- It gives the AWS milestone an integration spine rather than three disconnected
  service snippets.

Docker/Testcontainers cost:

- Very high. It uses multiple AWS-compatible services in one emulator-backed
  suite. Keep fixtures small, run serially, and document startup cost.

Implementation boundary:

- Do not add real cloud deployment, IAM provisioning, or production credential
  setup.
- README should link #59, #60, and #61 as prerequisites.

## Deferred or Rejected AWS Candidates

- Reject a broad AWS SDK wrapper. Upstream research says helpers should exist
  only where repeated-service benefit is clear.
- Reject real AWS integration tests for workshop DoD. Local deterministic
  Floci/Testcontainers tests are the required proof.
- Defer LocalStack-specific paths unless compatibility with Floci is the issue
  under test.
- Defer IAM, STS, Lambda, or deployment examples. They introduce credential and
  cloud-account scope that #48 does not authorize.

## Accepted Audit / Outbox Candidates

### #56 Audit Order History

Accept as the first audit workshop example.

Reasons:

- It maps to Kotlin `exposed/javers-audit`, but keeps the Go lesson focused on
  audit concepts rather than JaVers internals.
- It can show append order, aggregate history queries, mutable current state
  versus immutable audit history, and retention/schema migration gaps.
- It is the smallest useful audit/event example before outbox and HTTP query
  wrappers.

Docker/Testcontainers cost:

- Low for the first pass if the upstream audit model is storage-neutral and can
  be proven with in-memory conformance tests. Medium if durable SQL storage is
  required by the upstream package.

Implementation boundary:

- README must distinguish audit history from event sourcing.
- Do not model a full event-sourced aggregate unless a later issue asks for it.

### #57 Transactional Outbox Publisher

Accept after #56 or once the upstream audit/outbox primitives are stable.

Reasons:

- It maps directly to Kotlin `messaging/transactional-outbox`, including atomic
  domain write + outbox append, publisher claim, retry, idempotency, and
  dead-letter states.
- It teaches the dual-write failure boundary that generic logging examples miss.
- It creates the durable publisher behavior needed by #68.

Docker/Testcontainers cost:

- Medium to very high depending on scope. A storage-only outbox claim/retry
  example may need only Postgres/Testcontainers. Adding Kafka-like publishing
  raises the cost and should be deferred unless the upstream package requires
  it.

Implementation boundary:

- Use Testcontainers only if durable storage behavior needs it.
- README must explain operator replay and poison-message gaps.

### #58 Gin Audit Query API

Accept as the public API wrapper after #56.

Reasons:

- It exposes audit history and event detail endpoints without making Gin part of
  the storage/query model.
- It tests useful user-facing behavior: not-found, pagination/filter inputs,
  successful query shape, and trust-boundary notes.

Docker/Testcontainers cost:

- Low to medium. Handler tests can stay local if #56 already proves storage
  behavior. Use durable storage only if the query package requires it.

Implementation boundary:

- Do not hide audit package errors behind generic HTTP responses.
- Keep storage/query logic framework-independent.

### #68 Audited Order Workflow Outbox Integration

Accept as the 0.11.0 integration example after #56, #57, and #58.

Reasons:

- It joins domain state changes, audit trail, outbox records, publisher handoff,
  and read API in one scenario-shaped workflow.
- It should demonstrate consistency between audit rows and outbox rows.
- It can link back to the focused audit examples instead of re-explaining all
  primitives.

Docker/Testcontainers cost:

- Medium to high. Durable storage is likely necessary. External broker
  emulation should be added only if #57 has already proven it is required and
  manageable.

Implementation boundary:

- Keep publisher retry or skipped-delivery behavior deterministic.
- Link README prerequisites to #56, #57, and #58.

## Deferred or Rejected Audit / Outbox Candidates

- Reject a JaVers clone. Upstream Go research says to port concepts, not the
  implementation.
- Reject generic application logging examples. #35 requires event/audit
  semantics, not log aggregation.
- Defer Kafka-backed publisher tests until the storage/outbox claim contract is
  stable. The Kotlin example uses Kafka, but Go workshop cost should remain
  proportional to the package surface.
- Defer event-sourcing claims. Audit history can be append-only without owning
  aggregate reconstruction semantics.

## Recommended Sequence

1. Implement #62 before the rest of the SQL track.
2. Implement #63 as the transaction/failure lesson.
3. Implement #64 as the HTTP wrapper over SQL repository behavior.
4. Implement #65 as the SQL integration example.
5. Implement #59 before the rest of the AWS track to establish Floci fixture
   shape and README environment guidance.
6. Implement #60 and #61 independently.
7. Implement #66 as the AWS integration example.
8. Implement #56 before outbox or query API examples.
9. Implement #57 when storage/outbox primitives are available.
10. Implement #58 as the public audit query wrapper.
11. Implement #68 as the audit/outbox integration example.

## DoD Evidence For #48

- Concrete source paths are listed in `Sources Checked`.
- Follow-up issue links are added for every accepted candidate:
  - SQL: #62, #63, #64, #65
  - AWS: #59, #60, #61, #66
  - Audit/outbox: #56, #57, #58, #68
- Docker/Testcontainers cost is called out per accepted candidate.
- Deferred and rejected candidates are listed with reasons.
- Stale upstream issue numbering is resolved against the current parent #31
  milestone mapping and current umbrella issues #32, #33, and #35.
