# Issue #49 계획: Workshop Roadmap Matrix와 README 확장

## 목표

0.8.0 이후 구현이 시작되기 전에 더 큰 `bluetape-go-workshop` 예제 묶음을 탐색 가능하게 유지하는 계획 산출물을 만든다.

Issue #49는 이 PR에서 root README를 다시 쓰는 것이 아니라 `docs/superpowers` 계획 자료를 요구한다. 아래 root README 작업은 이후 예제 PR과 남은 0.7.0 planning issue를 위한 실행 계획이다.

## 범위

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #49 `[v0.7.0] Add workshop roadmap matrix and README expansion plan`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- 작업 유형: Type E - Planning / Maintenance

## 확인한 출처

- GitHub issues:
  - #27 epic roadmap
  - #31 0.7.0 research/planning parent
  - #47 graph/text candidate research
  - #48 audit/AWS/SQL candidate research
  - #28, #29, #30 completed 0.4.0-0.6.0 umbrella tracks
  - #32, #33, #34, #35, #36 future 0.8.0-0.12.0 umbrella tracks
  - #38-#46, #50-#69, #70-#81 child issues
- 현재 repository 파일:
  - `README.md`
  - `README.ko.md`
  - `docs/superpowers/research/2026-06-23-issue-47-graph-text-candidates-research.md`
  - `docs/superpowers/research/2026-06-23-issue-48-audit-aws-sql-candidates-research.md`
  - `docs/superpowers/plans/2026-06-22-issue-29-customer-migration-batch-integration-plan.md`
- 검색:
  - #49 roadmap matrix와 HTTP framework decision에 대한 context-mode 검색.
  - `gno query "bluetape-go-workshop issue 49 roadmap matrix README expansion #27 #31" -c bluetape4k-github --fast --no-rerank`
  - `gno query "bluetape-go-workshop roadmap matrix README expansion examples milestones" -c bluetape4k-docs --no-rerank`

## Roadmap Matrix

| Milestone | Track / issue | Example과 issue link | Package coverage | HTTP framework 선택 | Docker/Testcontainers 요구사항 | README / diagram 작업 |
|---|---|---|---|---|---|---|
| 0.4.0 | State and workflow, #28 | #38 order lifecycle state API, #39 fulfillment workflow runner, #40 operations report policy, #70 payment authorization state, #71 compensation workflow, #72 order fulfillment integration | `state`, `workflow`, `workreport` | 이 milestone의 모든 public HTTP API는 Gin을 사용한다. | Docker 불필요. 예제는 in-memory/service-local이다. | 이미 root README에 있다. Diagram을 다시 생성할 때 milestone group과 example map을 최신으로 유지한다. |
| 0.5.0 | Batch checkpoint/restart, #29 | #41 account migration checkpoint restart, #73 chunked CSV import checkpoint, #74 retry/dead-letter worker, #42/#43/#75는 customer migration batch integration으로 통합 | scheduled integration에는 `batch`, `leader` | Local job은 HTTP framework를 사용하지 않는다. #75는 operations API와 scheduled tick endpoint에 Gin을 사용한다. | 현재 예제에는 Docker가 필요 없다. #75의 leader gate는 deterministic/local이다. | 이미 root README에 있다. 중복 README entry를 피하기 위해 #42/#43이 #75에 통합됐다고 문서화한다. |
| 0.6.0 | Portable utilities, #30 | #44 ID/JWT boundary, #76 token refresh claims, #45 money pricing, #77 multi-currency invoice, #46 probabilistic dedupe, #78 checkout guard integration | `id`, `jwt`, `money`, `probabilistic` | Public utility boundary API는 Gin을 사용한다. | Docker 불필요. 예제는 local HTTP service다. | 이미 root README에 있다. Checkout guard를 milestone integration route로 유지한다. |
| 0.7.0 | Research and planning, #31 | #47 graph/text research, #48 audit/AWS/SQL research, #49 roadmap matrix, #79 scorecard, #80 integration template/rubric, #81 cross-milestone blueprint | SQL, AWS, text, audit, graph track에 대한 planning | N/A. Planning artifact만 있다. | N/A. Upstream package가 준비되지 않는 한 이 milestone에는 runnable example이 없다. | #79-#81이 완료된 뒤에만 future "Roadmap Planning" 섹션 아래에 planning link를 추가한다. |
| 0.8.0 | SQL DSL/repository, #32 | #62 SQL order repository, #63 SQL transaction boundary, #64 Gin SQL CRUD API, #65 Gin SQL order service integration | SQL DSL/repository helper, transaction, repository boundary | #62/#63은 public HTTP boundary가 없다. #64/#65는 Gin을 사용한다. | 중간에서 높음. 실제 SQL transaction/lock 동작을 증명해야 하면 Postgres/Testcontainers를 사용하고, 그 외에는 handler-only coverage용 빠른 local test를 유지한다. | 먼저 focused example로 SQL milestone group을 추가한 뒤 #65를 integration으로 추가한다. DB setup note와 Docker badge/legend를 추가한다. |
| 0.9.0 | AWS/Floci, #33 | #59 S3 Floci storage, #60 SQS Floci worker, #61 DynamoDB conditional repository, #66 S3-SQS-DynamoDB document workflow integration | AWS SDK for Go v2, Floci/Testcontainers, S3, SQS, DynamoDB | #59/#60/#61은 기본적으로 public HTTP가 없는 package/worker example이다. #66이 document workflow API를 노출하면 Gin을 사용할 수 있다. | 높음에서 매우 높음. Local AWS 동작에는 Floci/Testcontainers가 필요하고 실제 AWS credential은 사용하지 않는다. | Docker-required marker와 명시적인 local emulator setup link가 있는 AWS/Floci milestone group을 추가한다. |
| 0.10.0 | Text search/tokenizer, #34 | #53 text moderation masking, #54 Gin text search service, #55 tokenizer/language detection feasibility, #67 Gin content moderation workflow integration | Text search, masking, tokenizer/language feasibility | #53/#55는 public HTTP boundary가 없다. #54/#67은 Gin을 사용한다. | 낮음. 예제는 local/deterministic으로 유지하고 network/model dependency를 피한다. | Unicode/language caveat note와 #67 integration route가 있는 text milestone group을 추가한다. |
| 0.11.0 | Audit/event/outbox, #35 | #56 audit order history, #57 transactional outbox publisher, #58 Gin audit query API, #68 audited order workflow outbox integration | Audit/event model, outbox, query API | #56/#57은 public HTTP boundary가 없다. #58/#68은 Gin을 사용한다. | 낮음에서 높음. 가능한 곳은 storage-neutral로 시작하고 durable outbox 동작을 증명할 때만 SQL/Testcontainers를 사용한다. | "audit is not event sourcing"와 operator replay caveat가 있는 audit/outbox group을 추가한다. |
| 0.12.0 | Graph domain examples, #36 | #50 graph abuse cluster, #51 graph recommendation, #52 graph import/export, #69 graph risk intelligence integration | Graph abstraction, graph I/O, traversal, scoring | #50/#51/#52는 기본적으로 public HTTP boundary가 없다. #69는 의도적으로 Gin risk API를 노출하지 않는 한 package-first로 유지한다. | 낮음에서 높음. in-memory/domain fixture로 시작하고, upstream adapter가 안정화될 때까지 multi-backend Testcontainers는 미룬다. | Domain-scenario ordering과 import/export fixture link가 있는 graph milestone group을 추가한다. |

## HTTP Framework Decision Table

| Example / issue | 결정 | 이유 |
|---|---|---|
| Existing 0.4.0 state/workflow APIs | Gin | Public HTTP API가 lesson의 일부다. |
| Existing 0.5.0 local batch jobs | None | lesson은 routing이 아니라 checkpoint/retry 동작이다. |
| Existing #75 customer migration integration | Gin | batch operations API와 scheduled tick endpoint를 소유한다. |
| Existing 0.6.0 utility APIs | Gin | public HTTP trust-boundary 예제다. |
| Existing leader/resilience compatibility examples before 0.4.0 | chi | 기존 compatibility-focused example은 변경하지 않는다. |
| Future #62, #63, #53, #55, #56, #57, #59, #60, #61, #50, #51, #52 | 기본값 None | repository, worker, text, audit, AWS, graph package 동작을 직접 가르친다. Issue가 API를 요구할 때만 HTTP를 추가한다. |
| Future #64, #65, #54, #67, #58, #68 | Gin | Issue가 public API 또는 milestone integration API를 명시적으로 요구한다. |
| Future #66 | API를 노출하면 Gin, 아니면 None | 시나리오는 document workflow다. Upload/status 동작을 명확히 하지 않는 한 HTTP를 추가하지 않는다. |
| Future #69 | integration이 risk API를 노출할 때만 Gin | Graph package 동작을 보이게 유지하고 장식적인 service wrapper를 피한다. |
| net/http | compatibility lesson에만 사용 | 현재 0.8.0-0.12.0 issue 중 plain `net/http` boundary가 필요한 것은 없다. |

## Root README 확장 계획

### 1. 현재 Example Table을 구현된 Route의 기준으로 유지

현재 root README 표는 `examples/` 아래에 존재하는 실행 가능한 예제만 계속 나열해야 한다. Future issue row를 구현된 것처럼 추가하지 않는다.

구현된 예제별 필수 항목:

- 영어 및 한국어 README link.
- 한 문장짜리 scenario 목적.
- bluetape-go package coverage.
- Lesson에 의미가 있을 때만 public HTTP framework.

### 2. 계획 Track용 Milestone Roadmap Section 추가

#79-#81이 완료된 뒤, 기존 `## Roadmap` 섹션을 이 문서의 matrix를 반영하는 compact planned-track matrix로 확장한다.

- Milestone과 umbrella issue.
- Focused example.
- Integration example.
- Runtime requirement: `Local`, `Docker/Testcontainers`, 또는 `TBD`.
- HTTP boundary: `Gin`, `chi`, `net/http`, 또는 `None`.
- Documentation status: `planned`, `implemented`, 또는 `needs diagram refresh`.

이 방식은 planned example을 implemented examples table에 섞지 않으면서 future work를 탐색 가능하게 유지한다.

### 3. Runtime Legend 추가

Roadmap matrix 앞에 짧은 legend를 추가한다.

- `Local`: Docker나 외부 service 없이 실행된다.
- `Docker/Testcontainers`: integration proof에 Docker가 필요하다.
- `Floci`: local AWS emulator이며 실제 AWS credential을 사용하지 않는다.
- `Gin`: framework가 드러나는 public API example.
- `None`: HTTP boundary가 없는 package, worker, local job example.

### 4. README.md와 README.ko.md 동기화 유지

모든 README navigation 변경은 같은 PR에서 두 파일을 함께 갱신해야 한다. 한국어 README는 `Gin`, `Testcontainers`, `Floci`, `outbox`, `checkpoint`처럼 예제가 사용하는 정확한 API/runtime 용어는 영어로 유지할 수 있다.

### 5. Diagram Update 정책

Planning-only issue마다 root example map을 다시 생성하지 않는다. 다음 중 하나에 해당할 때 root diagram을 갱신한다.

- milestone integration example이 구현된다.
- #81이 cross-milestone integration blueprint를 확정하고 learning route를 변경한다.

개별 example diagram은 각 example README가 계속 소유한다.

## 이후 Issue 실행 지침

- 각 future milestone은 focused example을 먼저 추가하고 integration example을 마지막에 추가해야 한다.
- Integration README는 focused prerequisite을 연결해야 한다.
- Testcontainers-backed example은 root README에 명시해야 하며 Docker resource를 공유할 때는 순차 실행해야 한다.
- Public HTTP API는 기본적으로 Gin을 사용한다. Issue가 compatibility 또는 standard-library handler shape을 명시적으로 요구할 때만 `chi`나 `net/http`를 사용한다.
- Upstream package availability로 runnable example이 낮은 위험과 명시적 scope를 갖추지 않는 한 0.7.0은 planning-only로 유지해야 한다.

## DoD Evidence For #49

- Matrix가 0.4.0부터 0.12.0까지 다룬다.
- 현재 및 planned HTTP example category별로 Gin vs `net/http` / `chi` / no-HTTP 선택이 명시되어 있다.
- Root README expansion plan이 포함되어 있다.
- Docker/Testcontainers 요구사항이 milestone별로 드러난다.
- Candidate selection을 다시 열지 않고 #47과 #48 research를 참조한다.
