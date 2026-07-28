# Issue #81 계획: Cross-Milestone Integration Blueprint

## 목표

0.4.0부터 0.6.0까지 구현된 foundation example을 0.8.0부터 0.12.0까지 계획된 더 큰 SQL, AWS, text, audit, graph domain으로 연결하는 workshop integration spine을 정의한다.

Issue #81은 계획 전용이다. 이후 `bluetape-go` milestone에 속한 package를 성급히 구현하지 않으면서 future path를 보이게 해야 한다.

## 범위

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #81 `[v0.7.0] Add cross milestone workshop integration blueprint`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- 작업 유형: Type E - Planning / Maintenance

## 확인한 출처

- GitHub issues:
  - #27 milestone-aligned workshop roadmap epic
  - #31 0.7.0 research/planning parent
  - Foundation integrations: #72, #75, #78
  - Future integrations: #65, #66, #67, #68, #69
  - Planning artifacts: #49, #79, #80
- 현재 repository planning/spec docs:
  - `docs/superpowers/specs/2026-06-09-issue-72-order-fulfillment-integration-design.md`
  - `docs/superpowers/specs/2026-06-22-issue-29-customer-migration-batch-integration-design.md`
  - `docs/superpowers/specs/2026-06-23-issue-78-checkout-guard-design.md`
  - `docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md`
  - `docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md`
  - `docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md`
- 검색:
  - #81, #49, #80, #27, integration-spine history에 대한 context-mode 검색.
  - `gno query "bluetape-go-workshop issue 81 cross milestone workshop integration blueprint #31 #49 #79 #80" -c bluetape4k-github --fast --no-rerank`

## Spine 원칙

Workshop은 점점 넓어지는 경로로 읽혀야 한다.

1. Foundation example은 local service behavior, batch recovery, portable utility trust boundary를 가르친다.
2. 0.7.0 planning artifact는 이후 domain에 포함할 것과 scope 밖에 둘 것을 정의한다.
3. Future example은 같은 domain noun과 integration habit을 재사용하되 repository를 하나의 큰 demo application으로 바꾸지 않는다.

Spine은 하나의 runnable app이 아니다. 각 milestone을 독립적으로 유용하게 유지하면서 다음 milestone이 자연스러운 continuation처럼 보이게 하는 narrative와 sequencing rule이다.

## Foundation Integration Anchors

| Issue | Milestone | 구현된 integration anchor | 재사용 가능한 lesson |
|---|---|---|---|
| #72 | 0.4.0 | Order fulfillment workflow integration | Domain state, workflow step, work report, compensation, failure projection은 하나의 service-facing scenario로 조합될 수 있다. |
| #75 | 0.5.0 | Customer migration batch integration | Batch checkpoint/restart, operations API, scheduled leader guard, retry, dead-letter handling은 운영자가 볼 수 있는 recovery behavior로 조합될 수 있다. |
| #78 | 0.6.0 | Checkout guard integration | ID/JWT, money/rules, probabilistic dedupe utility는 generic framework가 되지 않고 HTTP trust boundary에서 조합될 수 있다. |

이 예제들은 future milestone이 재사용해야 할 pattern을 세운다.

- focused example 우선
- integration example 마지막
- 구체적인 domain scenario 하나
- 결정적 fixture
- 영어/한국어 README walkthrough
- PR merge 전 local validation evidence
- 명시적인 production hardening note

## Future Integration Path

| Issue | Milestone | 계획된 integration | Foundation bridge | 주요 명사 | Runtime posture |
|---|---|---|---|---|---|
| #65 | 0.8.0 SQL | Gin SQL order service integration | #72 order lifecycle과 #78 checkout order ID를 persisted order, item, status table로 확장한다. | order, order item, status, transaction | Local first. Transaction 또는 lock semantic이 요구할 때만 Postgres/Testcontainers를 사용한다. |
| #66 | 0.9.0 AWS/Floci | S3-SQS-DynamoDB document workflow integration | #75 operational recovery와 #78 request identity를 document ingestion, queued processing, idempotent state로 확장한다. | document, processing event, idempotency key, customer/account | Floci/Testcontainers local emulator. 실제 AWS credential은 사용하지 않는다. |
| #67 | 0.10.0 text | Gin content moderation workflow integration | #78 trust-boundary check를 text moderation과 search record로 확장한다. 이후 #69 risk signal로 공급할 수 있다. | content, language, token, moderation record | Local deterministic fixture. 의존성이 큰 tokenizer는 bounded 상태로 둔다. |
| #68 | 0.11.0 audit/outbox | Audited order workflow outbox integration | #72 order state change와 #65 SQL persistence를 auditable event 및 outbox handoff로 확장한다. | order, audit event, outbox record, publisher attempt | Storage-neutral first. Durable outbox behavior가 필요할 때 SQL/Testcontainers를 사용한다. |
| #69 | 0.12.0 graph | Graph risk intelligence integration | #66 document/event, #67 moderation record, #68 audit event의 risk signal을 graph traversal과 scoring으로 가져온다. | account, device, document, risk event, cluster | In-memory/domain fixture 우선. Upstream adapter가 안정화될 때까지 backend matrix는 미룬다. |

## 공유 Domain Noun

Example이 연결되어 보이도록 shared noun을 의도적으로 사용하되 global schema를 강제하지 않는다.

| Noun | 시작 위치 | 이어지는 위치 | 규칙 |
|---|---|---|---|
| `order` | #72, #78 | #65, #68 | Order identifier를 안정적이고 domain-shaped로 유지한다. Example 사이에서 Go package를 공유하지 않는다. |
| `customer` | #75, #78 | #65, #66, #67 | 결정적 fixture customer를 사용하고 실제 personal data를 피한다. |
| `document` | #66 | #67, #69 | Document는 local fixture 또는 emulator object로 다룬다. 실제 cloud data는 사용하지 않는다. |
| `account` | #69 | #66, #68, #69 | Example이 risk 또는 graph analysis를 다룰 때는 opaque 또는 synthetic account ID를 사용한다. |
| `risk event` | #67, #68 | #69 | Risk event는 explainable하고 deterministic하게 유지한다. Model 또는 opaque scoring dependency를 두지 않는다. |
| `idempotency key` | #75, #78 | #66, #68 | 각 README에서 key가 authoritative, probabilistic, demo-only 중 무엇인지 명시한다. |

Shared noun contract는 documentation-level이다. 각 example은 자기 `examples/<name>` directory 아래에서 self-contained로 남는다.

## 순서 규칙

1. Focused prerequisite issue가 package contract를 증명하기 전에는 integration issue를 시작하지 않는다.
2. Future milestone은 parent #31 순서인 #65, #66, #67, #68, #69를 유지한다.
3. 각 milestone 안에서는 ad hoc issue selection보다 #79 scorecard sequence를 우선한다.
4. 각 integration issue의 design, README, validation plan, PR DoD table을 작성할 때 #80 template을 사용한다.
5. 이 blueprint PR이 추가하는 0.7.0 planning link를 제외하고, root README navigation은 runnable example이 존재할 때만 갱신한다.
6. Diagram은 lifecycle, data flow, graph shape, sequence, operator behavior를 명확히 할 때만 추가한다.
7. 실제 external service는 workshop DoD 밖에 둔다. Package behavior가 service semantic을 요구할 때만 local deterministic fixture, Testcontainers, Floci를 사용한다.

## Integration Spine By Reader Journey

### Path A: Order와 Consistency

1. #72는 하나의 order workflow 안에서 lifecycle과 compensation을 가르친다.
2. #78은 checkout trust boundary와 생성된 request/order ID를 도입한다.
3. #65는 repository 및 transaction contract로 order data를 영속화한다.
4. #68은 order change를 audit event와 outbox record로 기록한다.
5. #69는 이후 account/order risk signal을 graph input으로 검사할 수 있다.

### Path B: Customer와 Operations

1. #75는 batch checkpoint/restart, operations visibility, leader-guarded scheduling을 가르친다.
2. #66은 queued document processing과 idempotent state에 operational pattern을 재사용한다.
3. #67은 deterministic text behavior로 customer/user content를 moderation한다.
4. #69는 customer, account, document, moderation risk signal을 연결할 수 있다.

### Path C: Trust Boundary와 Risk

1. #78은 service boundary에서 JWT claim, money/rule decision, probabilistic dedupe를 가르친다.
2. #67은 text behavior를 moderation decision으로 바꾼다.
3. #68은 domain behavior를 auditable fact로 바꾼다.
4. #69는 event와 relationship을 explainable graph risk intelligence로 바꾼다.

## README Roadmap Placement

Root README는 이 integration spine을 runnable route가 아니라 planning material로 가리켜야 한다.

`Roadmap Planning` 섹션은 milestone roadmap table 바로 뒤에 둔다. 다음을 link해야 한다.

- #49 roadmap matrix
- #79 example selection scorecard
- #80 integration template/rubric
- #81 cross-milestone integration blueprint

이렇게 하면 planned track을 implemented examples table에 섞지 않으면서 #27과 README roadmap reader가 방향을 잃지 않는다.

## Future Work 중단 조건

Integration issue가 다음 항목 중 하나를 추가하려 하면 future issue 작성자는 작업을 중단하고 분할해야 한다.

- Example 전반에 걸친 새 shared framework
- Production deployment story
- 실제 cloud credential 또는 IAM setup
- Focused example이 surface를 증명하기 전의 multi-backend matrix
- 넓은 graph/backend abstraction
- opaque AI/model dependency
- 아직 존재하지 않는 example의 README row

이 항목들은 별도 research 또는 implementation issue가 될 수 있지만 integration PR에 끼워 넣으면 안 된다.

## DoD Evidence For #81

- Blueprint artifact가 `docs/superpowers/plans`에 있다.
- Blueprint가 #72, #75, #78, #65, #66, #67, #68, #69를 연결한다.
- Shared domain noun이 foundation과 future milestone 전반에 매핑되어 있다.
- Sequencing rule이 #31, #79, #80을 가리킨다.
- README roadmap placement가 #27과 root README가 이 integration spine을 어떻게 가리킬 수 있는지 설명한다.
- Root `README.md`와 `README.ko.md`가 #49, #79, #80, #81 planning artifact link를 포함한 `Roadmap Planning` 섹션을 가진다.
