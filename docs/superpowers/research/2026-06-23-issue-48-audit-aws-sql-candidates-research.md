# Issue #48 리서치: Audit, AWS, SQL 워크숍 후보

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #48 `[v0.7.0] Research audit AWS and SQL workshop candidates`
- 상위 track: #31 `[v0.7.0] Plan post-utility workshop tracks`
- 상위 roadmap epic: #27
- 마일스톤: `0.7.0`
- 작업 유형: Type E - Research / Maintenance

## 확인한 출처

- GitHub issue #48, 상위 #31, follow-up workshop issue:
  - SQL: #32, #62, #63, #64, #65
  - AWS: #33, #59, #60, #61, #66
  - Audit/outbox: #35, #56, #57, #58, #68
- upstream bluetape-go research:
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.8.0-sql-research.md`
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.9.0-aws-research.md`
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.11.0-audit-javers-research.md`
- Kotlin workshop example:
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/exposed/mvc-jdbc/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/exposed/webflux-r2dbc/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/exposed/javers-audit/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/messaging/transactional-outbox/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/aws/s3-spring-cloud/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/aws/storage-abstraction/README.md`
- upstream package reference:
  - `/Users/debop/work/bluetape4k/bluetape4k-exposed/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-aws/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-javers/README.md`
- 검색:
  - #48 / audit / AWS / SQL workshop history에 대한 context-mode search.
  - `gno query "bluetape-go-workshop issue 48 audit AWS SQL candidates #32 #33 #35" -c bluetape4k-github --fast --no-rerank`
  - `gno query "bluetape-go 0.8.0 SQL 0.9.0 AWS 0.11.0 audit research" -c bluetape4k-docs --no-rerank`

## 현재 근거

#48은 구현 전 candidate selection을 요구한다. 상위 #31은 현재 관련 track을 다음과 같이
매핑한다.

- 0.8.0 SQL DSL and repository helpers (#32)
- 0.9.0 AWS helper packages and Floci-backed examples (#33)
- 0.11.0 audit/event packages and outbox-style workflows (#35)

upstream bluetape-go research는 이미 guardrail을 정했다.

- SQL은 SQL을 보이게 유지하고, full ORM layer와 Kotlin Exposed clone을 피하며,
  `context.Context`, transaction, explicit error, safe query construction에
  집중해야 한다.
- AWS는 helper/example driven으로 유지하고, repeated-service evidence 없이 AWS SDK for
  Go v2를 감싸지 않으며, local AWS behavior에는 Floci/Testcontainers를 사용해야 한다.
- Audit은 JaVers internal이 아니라 concept를 port하고, durable publisher adapter 전에
  storage-neutral audit/event model로 시작해야 한다.

일부 upstream research issue reference는 이후 workshop issue map이 바뀌어 stale하다.
이 note는 #31, #32, #33, #35의 현재 workshop issue를 source of truth로 사용한다.

## 채택한 SQL 후보

### #62 SQL Order Repository

첫 SQL workshop example로 채택한다.

이유:

- repository가 table access를 소유하고 service가 transaction boundary를 보이게 유지하는
  Kotlin `exposed/mvc-jdbc`에 깔끔하게 매핑된다.
- 넓은 ORM abstraction을 강제하지 않고 insert, find, filtered list, not-found behavior를
  가르칠 수 있다.
- package-first SQL helper, visible SQL, explicit error, context ownership이라는
  upstream Go 방향에 맞는다.

Docker/Testcontainers 비용:

- 중간. upstream SQL helper가 real database semantic을 요구할 때만
  Postgres/Testcontainers를 사용한다. sqlite 또는 package-provided in-memory fixture가
  같은 repository contract를 증명할 수 있으면 첫 pass는 Docker-free로 유지한다.

구현 경계:

- SQL string snapshot만으로 동작을 assert하지 않는다.
- README에는 직접 `database/sql` 비교를 유지하되, example code에는 package helper
  사용을 둔다.

### #63 SQL Transaction Boundary

#62 뒤에 채택한다.

이유:

- `exposed/mvc-jdbc`와 `exposed/webflux-r2dbc`의 order placement flow에 매핑된다.
  여기에는 order header, line, product stock, lock ordering, stock conflict 시 rollback이
  포함된다.
- SQL helper의 가장 중요한 production contract인 transaction ownership이 service
  boundary에 있다는 점을 가르친다.
- CRUD-only behavior 대신 concrete failure case를 track에 제공한다.

Docker/Testcontainers 비용:

- 중간에서 높음. rollback과 lock behavior는 real database에서 더 강하게 증명된다. row
  locking이 범위에 있으면 Postgres/Testcontainers를 사용하고 test를 순차 실행해야 한다.

구현 경계:

- transaction lifetime을 명시적으로 유지한다.
- README에 context cancellation note를 포함한다.
- heavy unit-of-work 또는 ORM layer를 만들지 않는다.

### #64 Gin SQL CRUD API

#62 뒤 HTTP wrapper로 채택한다.

이유:

- Gin을 HTTP boundary에 격리하면서 SQL repository에 public API shape를 제공한다.
- 다른 persistence lesson을 만들지 않고 같은 order repository behavior를 재사용할 수
  있다.
- 이슈가 `net/http`를 요구하지 않는 한 public HTTP example은 Gin을 사용한다는 워크숍
  관례와 맞는다.

Docker/Testcontainers 비용:

- repository의 real database test를 재사용하면 중간 수준이다. repository contract가
  다른 곳에서 이미 증명되었다면 package-level interface 또는 local test store를 사용해
  handler-only test를 빠르게 유지한다.

구현 경계:

- handler는 validate와 response projection을 담당하고, repository는 Gin 없이도 사용할 수
  있어야 한다.
- README에는 migration/setup note와 curl example을 포함해야 한다.

### #65 Gin SQL Order Service Integration

#62, #63, #64 뒤 0.8.0 integration example로 채택한다.

이유:

- 각 focused lesson을 반복하지 않고 repository modeling, transaction boundary, HTTP API를
  조합한다.
- order creation, status lookup, item update, rollback/failure behavior를 다루는
  scenario-shaped example이다.
- 더 작은 SQL example과 어떻게 다른지 설명해야 한다.

Docker/Testcontainers 비용:

- 중간에서 높음. integrated example이 multi-table write와 rollback을 다루면 real
  database-backed test를 선호한다. suite를 bounded하게 유지하고 Testcontainers를 순차
  실행한다.

구현 경계:

- inventory, fulfillment, payment, outbox behavior로 확장하지 않는다.
- README prerequisite를 #62, #63, #64에 연결한다.

## 보류하거나 거부한 SQL 후보

- Kotlin Exposed clone은 거부한다. upstream Go SQL research는 full ORM layer를 피하고
  Go API를 runtime-first로 유지하라고 명시한다.
- upstream SQL research가 가장 작은 안전한 경로임을 증명할 때까지 workshop path의
  mandatory code generation은 거부한다.
- R2DBC/coroutine-style port는 보류한다. Go example은 Kotlin coroutine architecture를
  모방하지 말고 Go context와 database contract를 사용해야 한다.

## 채택한 AWS 후보

### #59 S3 Floci Storage

첫 AWS workshop example로 채택한다.

이유:

- Kotlin `aws/s3-spring-cloud`와 `aws/storage-abstraction`의 S3 profile에 직접
  매핑된다.
- S3 object lifecycle은 bucket/object setup, upload, download, list, delete, 선택적
  pre-signed URL behavior를 포함하는 가장 작은 유용한 AWS behavior다.
- Floci와 SDK endpoint override를 통해 AWS credential을 local-only로 유지한다.

Docker/Testcontainers 비용:

- 높지만 의도된 비용이다. #59는 Floci/Testcontainers-only test를 요구한다. README는
  Docker가 필요하고, real AWS credential을 사용하지 않으며, real AWS 차이는 local proof
  밖이라고 명시해야 한다.

구현 경계:

- caller-owned AWS SDK client를 사용한다.
- storage abstraction을 작게 유지하고 전체 AWS SDK를 감싸지 않는다.

### #60 SQS Floci Worker

#59가 Floci fixture를 증명한 뒤 채택한다.

이유:

- AWS helper track에 매핑되며 concrete producer/consumer lesson을 제공한다.
- message round trip, handler success, retry-visible failure, idempotency
  requirement를 가르칠 수 있다.
- integration issue 전까지는 S3/DynamoDB와 독립적으로 유지해야 한다.

Docker/Testcontainers 비용:

- 높음. SQS behavior는 Floci/Testcontainers로 검증해야 한다. emulator를 공유할 때 test는
  순차 실행해야 한다.

구현 경계:

- README는 delivery semantic과 idempotency requirement를 명시해야 한다.
- dead-letter 또는 parking-lot behavior는 upstream helper/emulator support가 명확할 때만
  문서화한다.

### #61 DynamoDB Conditional Repository

AWS persistence-focused example로 채택한다.

이유:

- conditional write, optimistic update, partition-key query behavior, conflict
  mapping이라는 bluetape-specific pain point를 target한다.
- 하나의 oversized demo를 만들지 않고 S3와 SQS를 보완한다.
- #66에 deterministic idempotency state store를 제공한다.

Docker/Testcontainers 비용:

- 높음. conditional write에는 emulator-backed behavior가 필요하다. Floci/local emulator
  test를 사용하고 real credential은 피한다.

구현 경계:

- README에 key design과 consistency caveat를 유지한다.
- #66 전에는 이것을 S3/SQS와 결합하지 않는다.

### #66 S3-SQS-DynamoDB Document Workflow Integration

#59, #60, #61 뒤 0.9.0 integration example로 채택한다.

이유:

- focused AWS example을 object upload, processing enqueue, idempotent state 기록이라는
  하나의 현실적인 local workflow로 조합한다.
- retry-safe message processing과 conditional write behavior를 증명할 수 있다.
- AWS milestone에 세 개의 분리된 service snippet이 아니라 integration spine을 제공한다.

Docker/Testcontainers 비용:

- 매우 높음. 하나의 emulator-backed suite에서 여러 AWS-compatible service를 사용한다.
  fixture를 작게 유지하고, 순차 실행하며, startup cost를 문서화한다.

구현 경계:

- real cloud deployment, IAM provisioning, production credential setup을 추가하지 않는다.
- README는 #59, #60, #61을 prerequisite로 연결해야 한다.

## 보류하거나 거부한 AWS 후보

- 넓은 AWS SDK wrapper는 거부한다. upstream research는 repeated-service benefit이 명확한
  곳에만 helper가 있어야 한다고 말한다.
- workshop DoD의 real AWS integration test는 거부한다. local deterministic
  Floci/Testcontainers test가 required proof다.
- Floci compatibility가 테스트 대상 이슈가 아닌 한 LocalStack-specific path는 보류한다.
- IAM, STS, Lambda, deployment example은 보류한다. 이들은 #48이 승인하지 않은
  credential과 cloud-account scope를 도입한다.

## 채택한 Audit / Outbox 후보

### #56 Audit Order History

첫 audit workshop example로 채택한다.

이유:

- Kotlin `exposed/javers-audit`에 매핑되지만, Go lesson은 JaVers internal이 아니라 audit
  concept에 집중하게 한다.
- append order, aggregate history query, mutable current state와 immutable audit
  history의 차이, retention/schema migration gap을 보여 줄 수 있다.
- outbox와 HTTP query wrapper 전에 가장 작은 유용한 audit/event example이다.

Docker/Testcontainers 비용:

- upstream audit model이 storage-neutral이고 in-memory conformance test로 증명할 수
  있으면 첫 pass에서는 낮음. upstream package가 durable SQL storage를 요구하면 중간.

구현 경계:

- README는 audit history와 event sourcing을 구분해야 한다.
- 이후 이슈가 요구하지 않는 한 full event-sourced aggregate를 모델링하지 않는다.

### #57 Transactional Outbox Publisher

#56 뒤 또는 upstream audit/outbox primitive가 안정되면 채택한다.

이유:

- atomic domain write + outbox append, publisher claim, retry, idempotency,
  dead-letter state를 포함하는 Kotlin `messaging/transactional-outbox`에 직접 매핑된다.
- generic logging example이 놓치는 dual-write failure boundary를 가르친다.
- #68에 필요한 durable publisher behavior를 만든다.

Docker/Testcontainers 비용:

- scope에 따라 중간에서 매우 높음. storage-only outbox claim/retry example은
  Postgres/Testcontainers만 필요할 수 있다. Kafka-like publishing을 추가하면 비용이
  커지므로 upstream package가 요구하지 않는 한 보류해야 한다.

구현 경계:

- durable storage behavior에 필요할 때만 Testcontainers를 사용한다.
- README는 operator replay와 poison-message gap을 설명해야 한다.

### #58 Gin Audit Query API

#56 뒤 public API wrapper로 채택한다.

이유:

- Gin을 storage/query model의 일부로 만들지 않고 audit history와 event detail endpoint를
  노출한다.
- not-found, pagination/filter input, successful query shape, trust-boundary note라는
  유용한 user-facing behavior를 테스트한다.

Docker/Testcontainers 비용:

- 낮음에서 중간. #56이 storage behavior를 이미 증명했다면 handler test는 local로
  유지할 수 있다. query package가 요구할 때만 durable storage를 사용한다.

구현 경계:

- audit package error를 generic HTTP response 뒤에 숨기지 않는다.
- storage/query logic을 framework-independent하게 유지한다.

### #68 Audited Order Workflow Outbox Integration

#56, #57, #58 뒤 0.11.0 integration example로 채택한다.

이유:

- domain state change, audit trail, outbox record, publisher handoff, read API를
  하나의 scenario-shaped workflow로 연결한다.
- audit row와 outbox row 사이의 consistency를 보여 주어야 한다.
- 모든 primitive를 다시 설명하지 않고 focused audit example로 다시 연결할 수 있다.

Docker/Testcontainers 비용:

- 중간에서 높음. durable storage가 필요할 가능성이 높다. external broker emulation은
  #57이 이미 필요성과 관리 가능성을 증명한 경우에만 추가한다.

구현 경계:

- publisher retry 또는 skipped-delivery behavior를 deterministic하게 유지한다.
- README prerequisite를 #56, #57, #58에 연결한다.

## 보류하거나 거부한 Audit / Outbox 후보

- JaVers clone은 거부한다. upstream Go research는 implementation이 아니라 concept를
  port하라고 말한다.
- generic application logging example은 거부한다. #35는 log aggregation이 아니라
  event/audit semantic을 요구한다.
- storage/outbox claim contract가 안정될 때까지 Kafka-backed publisher test는 보류한다.
  Kotlin example은 Kafka를 사용하지만 Go workshop 비용은 package surface에 비례해야
  한다.
- event-sourcing claim은 보류한다. audit history는 aggregate reconstruction semantic을
  소유하지 않고도 append-only일 수 있다.

## 권장 순서

1. SQL track의 나머지보다 #62를 먼저 구현한다.
2. #63을 transaction/failure lesson으로 구현한다.
3. #64를 SQL repository behavior 위의 HTTP wrapper로 구현한다.
4. #65를 SQL integration example로 구현한다.
5. AWS track의 나머지보다 #59를 먼저 구현해 Floci fixture shape와 README environment
   guidance를 확립한다.
6. #60과 #61은 독립적으로 구현한다.
7. #66을 AWS integration example로 구현한다.
8. outbox 또는 query API example보다 #56을 먼저 구현한다.
9. storage/outbox primitive를 사용할 수 있으면 #57을 구현한다.
10. #58을 public audit query wrapper로 구현한다.
11. #68을 audit/outbox integration example로 구현한다.

## #48 DoD 근거

- 구체적인 source path는 `확인한 출처`에 나열했다.
- 모든 accepted candidate에 follow-up issue link를 추가했다.
  - SQL: #62, #63, #64, #65
  - AWS: #59, #60, #61, #66
  - Audit/outbox: #56, #57, #58, #68
- Docker/Testcontainers 비용은 accepted candidate별로 명시했다.
- 보류하거나 거부한 candidate는 이유와 함께 나열했다.
- stale upstream issue numbering은 현재 parent #31 milestone mapping과 현재 umbrella
  issue #32, #33, #35 기준으로 해소했다.
