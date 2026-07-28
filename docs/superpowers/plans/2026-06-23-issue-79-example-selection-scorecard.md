# Issue #79 계획: Example Selection Scorecard

## 목표

#47, #48, #49 조사 결과를 더 큰 post-utility workshop track의 실행 가능한 순서로 바꾸는 0.7.0 계획 점수표를 만든다.

Issue #79는 계획 전용이다. 이 PR에서는 실행 가능한 예제나 root README navigation을 추가하지 않는다. README 확장은 #49 계획의 gate를 따른다. #79, #80, #81이 완료된 뒤 계획 링크를 추가한다.

## 범위

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #79 `[v0.7.0] Add bluetape4k-workshop example selection scorecard`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- 작업 유형: Type E - Planning / Maintenance

## 확인한 출처

- GitHub issues:
  - #31 0.7.0 research/planning parent
  - #32 SQL umbrella
  - #33 AWS/Floci umbrella
  - #34 text umbrella
  - #35 audit/outbox umbrella
  - #36 graph umbrella
  - #47 graph/text 후보 조사
  - #48 audit/AWS/SQL 후보 조사
  - #49 roadmap matrix and README expansion plan
  - #50-#69 accepted 또는 gated 후보 issue
- 현재 repository 계획 문서:
  - `docs/superpowers/research/2026-06-23-issue-47-graph-text-candidates-research.md`
  - `docs/superpowers/research/2026-06-23-issue-48-audit-aws-sql-candidates-research.md`
  - `docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md`
- 검색:
  - #79, #47, #48, #49 계획 이력에 대한 context-mode 검색.
  - `gno query "bluetape-go-workshop issue 79 scorecard #32 #33 #34 #35 #36" -c bluetape4k-github --fast --no-rerank`

## 점수 기준

점수는 1부터 5까지이며, 5가 workshop path에 가장 잘 맞는 값이다.

| Dimension | 의미 |
|---|---|
| Package maturity | 상위 또는 예상 `bluetape-go` package surface가 안정적인 예제에 얼마나 준비되어 있는지. |
| Workshop value | 얇은 API tour가 아니라 domain-shaped lesson을 얼마나 명확히 가르치는지. |
| Dependency lightness | 런타임/테스트 의존성 비용이 얼마나 작은지. 로컬 결정적 예제가 Docker/cloud-heavy example보다 높은 점수를 받는다. |
| Testability | 동작을 결정적 테스트와 픽스처로 얼마나 쉽게 검증할 수 있는지. |
| Source similarity | 검증된 `bluetape4k-workshop` source example 또는 package research와 얼마나 직접적으로 대응되는지. |

Decision rule:

- `Accept first`: umbrella track의 선행 예제.
- `Accept after`: 가치 있고 범위 안에 있지만 focused predecessor에 의존한다.
- `Accept bounded`: 명시한 축소 계약 안에서만 범위에 포함한다.
- `Defer`: 지정한 상위 또는 의존성 조건이 충족될 때까지 구현하지 않는다.
- `Reject`: future issue가 제약을 바꾸지 않는 한 workshop path에서 제외한다.

Total score는 우선순위 신호이며 자동 일정이 아니다. 실제 구현 순서는 여전히 umbrella milestone 순서, prerequisite issue, 의존성 준비도가 제어한다.

## 후보 점수표

### SQL Track - #32

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #62 | SQL order repository | 4 | 5 | 3 | 4 | 4 | 20 | Accept first. 가장 작은 저장소형 SQL lesson이며 SQL을 보이게 유지한다. |
| #63 | SQL transaction boundary | 4 | 5 | 2 | 4 | 4 | 19 | Accept after #62. Rollback, lock, 서비스 소유 트랜잭션 동작을 가르치기 전에 repository model이 필요하다. |
| #64 | Gin SQL CRUD API | 4 | 4 | 3 | 4 | 3 | 18 | Accept after #62. Gin은 HTTP boundary에 두고 repository contract를 재사용한다. |
| #65 | Gin SQL order service integration | 3 | 5 | 2 | 4 | 4 | 18 | Accept after #62, #63, and #64. 첫 SQL lesson이 아니라 0.8.0 통합 경로다. |

SQL notes:

- #62는 Go ORM을 새로 만들지 않고 package helper 사용을 증명할 수 있으므로 track의 anchor다.
- Lock 또는 rollback semantic이 범위에 있으면 #63과 #65에는 Postgres/Testcontainers가 필요할 수 있다.
- Repository 동작이 이미 증명된 뒤에는 #64를 handler-focused로 유지해야 한다.

### AWS / Floci Track - #33

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #59 | S3 Floci storage | 3 | 5 | 2 | 3 | 5 | 18 | Accept first. 로컬 AWS 에뮬레이터와 credential story를 세운다. |
| #60 | SQS Floci worker | 3 | 4 | 2 | 3 | 4 | 16 | Accept after #59. Floci setup을 재사용하고 delivery/idempotency boundary를 명시적으로 유지해야 한다. |
| #61 | DynamoDB conditional repository | 3 | 5 | 2 | 4 | 3 | 17 | Accept after #59, independently from #60. 결정적 conditional-write와 conflict behavior를 추가한다. |
| #66 | S3-SQS-DynamoDB document workflow integration | 2 | 5 | 1 | 3 | 4 | 15 | Accept after #59, #60, and #61. 0.9.0 통합 예제로 유지하고 에뮬레이터 기반 테스트는 순차 실행한다. |

AWS notes:

- 낮은 dependency-lightness 점수는 의도적이다. 로컬 AWS 동작에는 Docker와 Floci/Testcontainers가 필요하지만 실제 AWS credential은 범위 밖이다.
- #66은 IAM, deployment, 실제 cloud setup을 추가하면 안 된다.

### Text Track - #34

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #53 | Text moderation masking | 4 | 5 | 5 | 5 | 4 | 23 | Accept first. 결정적/로컬 예제이며 text search 및 masking behavior에 직접 대응된다. |
| #54 | Gin text search service | 4 | 4 | 5 | 5 | 3 | 21 | Accept after #53. HTTP 자체를 lesson으로 만들지 않고 text behavior를 public API로 노출한다. |
| #55 | Tokenizer / language detection feasibility | 2 | 4 | 3 | 3 | 4 | 16 | Accept bounded. Dependency, dictionary, license, binary-size risk가 해결될 때까지 feasibility-only로 유지한다. |
| #67 | Gin content moderation workflow integration | 3 | 5 | 4 | 4 | 4 | 20 | Accept after #53, #54, and #55. Moderation, search, language handling을 하나의 결정적 처리 흐름으로 합성한다. |

Text notes:

- #53은 완전히 로컬이고 관찰 가능하므로 가장 강한 first candidate다.
- #55가 실수로 production tokenizer dependency decision이 되면 안 된다.
- #67은 모든 primitive를 반복하는 대신 focused example을 연결해야 한다.

### Audit / Outbox Track - #35

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #56 | Audit order history | 3 | 5 | 5 | 5 | 4 | 22 | Accept first. Event-sourcing semantic을 주장하지 않고 immutable audit history를 가르친다. |
| #57 | Transactional outbox publisher | 2 | 5 | 2 | 3 | 5 | 17 | Accept after #56 or once upstream outbox primitives are stable. Dual-write lesson을 소유한다. |
| #58 | Gin audit query API | 3 | 4 | 4 | 4 | 3 | 18 | Accept after #56. Query/storage behavior는 framework-independent로 유지한다. |
| #68 | Audited order workflow outbox integration | 2 | 5 | 2 | 3 | 4 | 16 | Accept after #56, #57, and #58. 0.11.0 통합 경로다. |

Audit notes:

- 상위 package contract가 허용하면 #56은 첫 pass를 storage-neutral로 유지한다.
- #57에는 durable SQL/Testcontainers가 필요할 수 있지만, Kafka-like broker emulation은 storage/outbox claim behavior가 안정화될 때까지 미룬다.
- #68은 audit row와 outbox row가 일관되게 유지됨을 증명해야 한다.

### Graph Track - #36

| Issue | Candidate | Package maturity | Workshop value | Dependency lightness | Testability | Source similarity | Total | Decision |
|---|---|---:|---:|---:|---:|---:|---:|---|
| #50 | Graph abuse cluster | 3 | 5 | 4 | 5 | 5 | 22 | Accept first. 구체적이고 security-shaped이며 Kotlin abuser-detection example에 직접 대응된다. |
| #51 | Graph recommendation | 3 | 4 | 4 | 5 | 5 | 21 | Accept after #50. Backend matrix 없이 ranking과 tie-breaking을 추가한다. |
| #52 | Graph import/export fixtures | 2 | 4 | 3 | 4 | 4 | 17 | 상위 graph I/O helper가 안정화될 때까지 defer하고, 이후 graph example의 fixture path로 accept한다. |
| #69 | Graph risk intelligence integration | 2 | 5 | 3 | 4 | 4 | 18 | Accept after #50, #51, and #52. 0.12.0 integration route다. |

Graph notes:

- #50과 #51은 작은 결정적 fixture로 시작하고 backend abstraction layer는 두지 않아야 한다.
- #52는 workshop-only loader가 아니라 upstream graph I/O를 사용할 때만 유용하다.
- #69는 scoring을 설명 가능하게 유지하고 package behavior를 보이게 해야 한다.

## Reject 또는 Defer된 Pattern

| Track | Pattern | Decision | 이유 |
|---|---|---|---|
| SQL - #32 | Kotlin Exposed clone | Reject | Go example은 Kotlin ORM shape를 다시 만드는 대신 SQL과 runtime contract를 보이게 유지해야 한다. |
| SQL - #32 | Mandatory code generation path | Reject for now | 현재 research는 codegen이 가장 작은 안전한 workshop path임을 증명하지 않는다. |
| SQL - #32 | R2DBC/coroutine-style port | Reject | Go example은 Kotlin coroutine architecture가 아니라 `context.Context`와 database contract를 사용해야 한다. |
| AWS - #33 | Broad AWS SDK wrapper | Reject | Helper는 repeated-service value가 명확한 곳에만 있어야 한다. |
| AWS - #33 | Real AWS integration tests | Reject | Workshop DoD는 Floci/Testcontainers 기반 로컬/결정적 상태를 유지해야 한다. |
| AWS - #33 | IAM, STS, Lambda, or deployment examples | Defer | #33 밖의 credential 및 cloud-account scope를 도입한다. |
| Text - #34 | Spring Data Elasticsearch port | Reject | #34는 local text behavior로 시작하지만 이 pattern은 external-service 및 Spring-shaped다. |
| Text - #34 | Full production Korean/Japanese tokenizer example | Defer | Dependency, dictionary, license, binary-size decision이 해결되지 않았다. |
| Text - #34 | Generic text framework scaffolding | Reject | Workshop에는 scenario-shaped observable behavior가 필요하다. |
| Audit - #35 | JaVers clone | Reject | Go track은 JaVers 내부가 아니라 audit concept을 port해야 한다. |
| Audit - #35 | Generic application logging example | Reject | #35는 log aggregation이 아니라 audit/event semantic을 요구한다. |
| Audit - #35 | Kafka-backed publisher tests | Defer | Broker emulation을 추가하기 전에 storage/outbox claim behavior가 안정화되어야 한다. |
| Audit - #35 | Event-sourcing claims | Defer | Audit history는 aggregate reconstruction semantic을 소유하지 않고도 append-only일 수 있다. |
| Graph - #36 | Broad backend abstraction | Reject | Backend-specific graph behavior를 너무 이르게 숨긴다. |
| Graph - #36 | Multi-backend Testcontainers matrix | Defer | Upstream confidence에는 유용하지만 첫 workshop graph example에는 너무 무겁다. |
| Graph - #36 | Standalone knowledge-graph example | Defer | 가치가 있지만 첫 abuse-cluster 및 recommendation pass보다 넓다. |
| Graph - #36 | Bare traversal snippets | Reject | #36은 isolated API demonstration이 아니라 domain graph scenario를 요구한다. |

## 권장 순서

먼저 parent #31 milestone 순서를 사용한다.

1. 0.8.0 SQL - #32: #62, #63, #64, then #65.
2. 0.9.0 AWS/Floci - #33: #59, then #60 and #61, then #66.
3. 0.10.0 text - #34: #53, #54, bounded #55, then #67.
4. 0.11.0 audit/outbox - #35: #56, #57 and #58, then #68.
5. 0.12.0 graph - #36: #50, #51, deferred #52, then #69.

구현이 시작되면 각 umbrella issue는 같은 pattern을 유지해야 한다. Focused example을 먼저 두고, integration issue를 마지막에 두며, README navigation은 실행 가능한 예제가 존재한 뒤에만 갱신한다.

## DoD Evidence For #79

- Scorecard가 graph, text, audit, AWS, SQL 후보를 다룬다.
- 모든 후보 issue #50-#69에 명시적 decision이 있다.
- Decision이 umbrella issue #32, #33, #34, #35, #36으로 다시 연결된다.
- Accepted, bounded, deferred, rejected example이 이유와 함께 분리되어 있다.
- #49 계획대로 #79-#81이 완료될 때까지 README navigation을 의도적으로 미룬다.
