# Issue #80 계획: Integration Example Template과 Acceptance Rubric

## 목표

0.8.0 이후 workshop track의 milestone-level integration example이 scenario-shaped, testable 상태를 유지하고 기존 foundation integration과 정렬되도록 반복 가능한 template과 acceptance rubric을 정의한다.

Issue #80은 계획 전용이다. Future example PR의 기대치를 표준화하며, 이 PR에서는 실행 가능한 예제를 추가하거나 root README navigation을 수정하지 않는다.

## 범위

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #80 `[v0.7.0] Add integration example template and acceptance rubric`
- Parent track: #31
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- 작업 유형: Type E - Planning / Maintenance

## 확인한 출처

- GitHub issues:
  - #31 0.7.0 research/planning parent
  - #49 roadmap matrix and README expansion plan
  - #79 example selection scorecard
  - Foundation integration examples: #72, #75, #78
  - Future integration examples: #65, #66, #67, #68, #69
- 현재 repository planning/spec docs:
  - `docs/superpowers/specs/2026-06-09-issue-72-order-fulfillment-integration-design.md`
  - `docs/superpowers/plans/2026-06-09-issue-72-order-fulfillment-integration-plan.md`
  - `docs/superpowers/specs/2026-06-22-issue-29-customer-migration-batch-integration-design.md`
  - `docs/superpowers/specs/2026-06-23-issue-78-checkout-guard-design.md`
  - `docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md`
  - `docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md`
- 검색:
  - #80, #72, #75, #78, #49, #79 계획 이력에 대한 context-mode 검색.
  - `gno query "bluetape-go-workshop issue 80 integration example template acceptance rubric #31 #49 #79" -c bluetape4k-github --fast --no-rerank`

## Integration Example 정의

Integration example은 같은 umbrella track의 focused example을 조합하는 milestone-level 실행 시나리오다. 범용 애플리케이션 프레임워크가 되지 않으면서 package contract, domain behavior, validation, operator/user visibility 사이의 연결을 가르쳐야 한다.

필수 형태:

- Milestone 순서에서 focused example 뒤에 배치한다.
- Design, README, PR body에서 focused prerequisite issue를 명시한다.
- 안정적인 픽스처를 갖춘 구체적인 domain scenario 하나를 소유한다.
- Package lesson에는 실제 `bluetape-go` package surface를 사용한다.
- 프레임워크, persistence, 에뮬레이터, 다이어그램 작업은 시나리오에 비례하게 유지한다.
- Workshop 구현을 production infrastructure처럼 보이게 하지 않고 production hardening gap을 기록한다.

Non-goals:

- Example 경계를 넘어 sibling example의 `internal` package를 import하지 않는다.
- 서로 무관한 예제를 통일하기 위해 프레임워크 계층을 추가하지 않는다.
- 테스트 대상 package behavior가 요구하지 않는 한 durable database, queue, cloud service, graph backend를 도입하지 않는다.
- Package lesson을 장식적인 API shell 뒤에 숨기지 않는다.

## Canonical Integration Issue Set

| Issue | Milestone | Integration role | Template 영향 |
|---|---|---|---|
| #72 | 0.4.0 | state, workflow, workreport, compensation을 조합한 order fulfillment integration | Lesson이 service-facing workflow이므로 Gin이 적합하다. Diagram은 lifecycle/sequence를 명확히 할 때 사용한다. |
| #75 | 0.5.0 | checkpoint, operations API, leader scheduling, retry, dead-letter handling을 조합한 customer migration batch integration | start/status/cancel 작업에는 Gin이 적합하다. 결정적 in-memory store는 restart behavior를 보이게 유지한다. |
| #78 | 0.6.0 | ID/JWT, money/rules, probabilistic admission을 조합한 checkout guard integration | Checkout boundary에는 Gin이 적합하다. 문서는 probabilistic dedupe 한계를 명시해야 한다. |
| #65 | 0.8.0 | repository, transaction, CRUD example을 조합한 SQL order service integration | Gin이 적합하다. Testcontainers는 실제 transaction/lock semantic이 필요할 때만 사용한다. |
| #66 | 0.9.0 | AWS/Floci example을 조합한 S3-SQS-DynamoDB document workflow integration | Gin은 선택 사항이다. Local AWS behavior에는 Floci/Testcontainers가 필요하며 순차 실행해야 한다. |
| #67 | 0.10.0 | text search, masking, tokenizer, language detection을 조합한 content moderation workflow integration | Gin이 적합하다. 지원하지 않는 language/tokenizer case는 명시해야 한다. |
| #68 | 0.11.0 | audit history, outbox, query API를 조합한 audited order workflow/outbox integration | read/query API에는 Gin이 적합하다. Durable outbox behavior는 broker emulation 전에 결정적이어야 한다. |
| #69 | 0.12.0 | graph modeling, import/export, traversal, scoring을 조합한 graph risk intelligence integration | Gin은 선택 사항이다. Graph package behavior를 보이게 유지하고, risk workflow를 명확히 할 때만 diagram을 추가한다. |

## README Template

모든 integration example은 같은 구조와 동등한 기술 내용을 가진 `README.md`와 `README.ko.md`를 포함해야 한다.

### English README Sections

Issue가 더 좁은 구조를 제시하지 않는 한 다음 섹션을 순서대로 사용한다.

1. `# <Example Name>`
2. `## Scenario`
   - Domain workflow를 설명하는 한 단락.
   - 조합한 focused example 또는 package lesson을 나열하는 bullet list.
3. `## What This Integrates`
   - Focused example 또는 prerequisite issue link.
   - `state`, `workflow`, `batch`, `leader`, `sql`, `aws`, `text`, `audit`, `graph` 같은 명시적 package coverage.
4. `## Run`
   - 필수 명령.
   - HTTP가 있으면 기본 bind address.
   - 적용 가능한 Docker/Testcontainers 또는 Floci prerequisite.
5. `## API` or `## Workflow`
   - HTTP example에는 `API`를 사용한다.
   - Public HTTP boundary가 없는 package, worker, local job, graph example에는 `Workflow`를 사용한다.
6. `## Validation`
   - Focused test command.
   - Concurrency, worker, HTTP server state, dedupe, retry, scheduling, cache, shared state가 contract의 일부이면 race command.
   - Docker-backed test가 있으면 순차 실행 Testcontainers note.
7. `## Diagrams`
   - Diagram이 prose보다 lifecycle, architecture, sequence, data flow, graph shape, operator flow를 더 잘 설명할 때만 포함한다.
   - Root README asset이 관련되면 `docs/images/readme-diagrams/`의 generated PNG asset을 link한다.
8. `## Production Hardening`
   - Workshop scope 밖에 남는 durability, idempotency, retry, authentication, authorization, persistence, cloud, audit, observability, operational gap을 명시한다.

### Korean README Sections

한국어 README는 영어 README와 동기화한다. 권장 섹션 label은 다음과 같다.

1. `# <Example Name>`
2. `## 시나리오`
3. `## 통합하는 내용`
4. `## 실행`
5. `## API` or `## 워크플로우`
6. `## 검증`
7. `## 다이어그램`
8. `## 운영 환경 보강 지점`

`Gin`, `chi`, `net/http`, `Testcontainers`, `Floci`, `outbox`, `checkpoint`, `idempotency`, `race`처럼 정확한 API/runtime 용어는 repository vocabulary와 맞을 때 영어로 유지할 수 있다.

## Implementation Template

Future integration example design/spec 작업에는 다음 구조를 사용한다.

```markdown
# Issue #<number> Design: <Milestone> Integration Example

## Goal

<focused examples>를 하나의 실행 가능한 <domain scenario>로 조합하는 milestone-level integration example을 추가한다.

## Non-Goals

- Package behavior가 요구하지 않는 <durability/cloud/framework scope>를 추가하지 않는다.
- Sibling example internal package를 import하지 않는다.
- <package lessons>를 generic framework 뒤에 숨기지 않는다.

## Example

- Path: `examples/<example-name>`
- Package: `internal/<packagename>`
- HTTP framework: `Gin` / `chi` / `net/http` / `None`
- Default address: `127.0.0.1:<port>` when HTTP is present
- Runtime requirement: `Local` / `Docker/Testcontainers` / `Floci`

## Scenario

Domain flow, fixture, failure path, expected output을 설명한다.

## Contracts

- Package contract
- 적용 가능한 HTTP contract
- Error와 status mapping
- Context/cancellation/resource ownership
- 적용 가능한 concurrency/race ownership

## Documentation Requirements

- `README.md`
- `README.ko.md`
- Example이 실행 가능해지면 root README navigation 갱신
- Workflow를 명확히 할 때만 diagram asset 사용

## Test Requirements

- Focused package test
- 적용 가능한 HTTP handler test
- Failure path와 invalid input path
- Context가 contract의 일부이면 cancellation/timeout
- Shared state, goroutine, worker, dedupe, scheduling, HTTP state에 대한 race test
- Docker-backed resource를 사용하면 serial Testcontainers test
```

## Framework And Runtime Rubric

| Choice | 사용 조건 | 피할 조건 | 필수 증거 |
|---|---|---|---|
| Gin | 예제가 service-facing이고 public HTTP route를 소유하거나 기존 issue 본문이 Gin API를 요구할 때. | Package lesson이 repository, worker, graph fixture, local batch behavior이며 유용한 public boundary가 없을 때. | Route table, status/error mapping, request validation, server ownership이 범위에 있을 때 timeout. |
| `net/http` | Lesson이 standard-library compatibility이거나 issue가 standard handler shape를 명시적으로 요구할 때. | Gin이 이미 public API convention에 맞고 compatibility lesson이 없을 때. | Handler contract, status mapping, 적용 가능한 body-size/body-close behavior. |
| chi | 이미 chi를 사용하는 historical compatibility example 또는 명시적인 router-compatibility issue. | chi-specific lesson이 없는 새 milestone integration example. | Compatibility rationale과 Gin을 쓰지 않는 이유를 설명하는 README note. |
| No HTTP | Focused package, worker, import/export, repository, graph, local job behavior가 API wrapper 없이 더 명확할 때. | Issue가 public operations/query/workflow API를 요구할 때. | CLI/workflow command, 결정적 fixture, 안정적인 output. |
| Testcontainers | Package contract를 증명하려면 실제 database, emulator, queue, cloud, backend behavior가 필요할 때. | In-memory fixture로 같은 workshop contract를 증명할 수 있을 때. | README의 Docker requirement, serial test note, local-only credential, focused command. |
| Floci | AWS-compatible local behavior가 필요할 때. | Real AWS, IAM, deployment, cloud-account behavior를 제안할 때. | 실제 credential 없음, endpoint override, serial emulator-backed test. |
| Diagrams | Lifecycle, sequence, architecture, graph shape, operator flow를 prose만으로 보기 어려울 때. | Diagram이 README prose를 반복만 하거나 planning-only PR에 root asset churn을 요구할 때. | 생성된 PNG/SVG asset, generator command, 적용 가능한 geometry/rendering evidence. |

## Validation Rubric For Integration PRs

모든 integration example PR은 merge 전에 다음 증거를 제공해야 한다.

| Gate | 필수 증거 |
|---|---|
| Scope | PR body가 umbrella issue, focused prerequisite issue, integration issue를 연결한다. |
| README pair | `README.md`와 `README.ko.md`가 존재하고 동등한 섹션을 가진다. |
| Root navigation | 실행 가능한 예제가 생기면 root `README.md`와 `README.ko.md`가 이를 한 번만 나열한다. |
| Package contract | 테스트가 HTTP wrapper만이 아니라 package lesson을 증명한다. |
| Failure behavior | 테스트가 최소 하나의 domain failure path와 하나의 invalid input path를 다룬다. |
| Context/resource behavior | 관련될 때 cancellation, timeout, body/rows/container close 또는 cleanup behavior를 테스트한다. |
| Race/concurrency | shared state, worker, dedupe, scheduling, HTTP state, goroutine-owned behavior에 대해 `go test -race`가 통과한다. |
| Docker/emulator | Testcontainers와 Floci test가 serial, local-only이며 README에 문서화되어 있다. |
| Diagrams | Diagram이 변경되면 diagram generation command와 visual evidence를 기록한다. |
| Review | PR review/CI gate 전에 review artifact가 `P0=0 P1=0`을 보고한다. |
| PR body | 최종 PR body section은 `## DoD Status`이고 validation, CI, merge readiness를 포함한다. |

## Pasteable PR Checklist

Future integration example PR은 최종 `## DoD Status` 섹션에 아래 내용을 붙여 넣고, 실제로 적용되지 않는 row만 줄일 수 있다.

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
| Review P0/P1 | PENDING | `P0=0 P1=0`을 기록한 review artifact |
| CI | PENDING | GitHub Actions check URL/result |
```

## README Navigation Plan

#49가 이미 root README behavior를 정의한다.

- 현재 example table은 실행 가능한 예제만 나열한다.
- #79, #80, #81이 완료된 뒤 root `## Roadmap` 섹션에는 compact `Roadmap Planning` 또는 planned-track matrix를 추가할 수 있다.
- Future integration template은 runnable example table이 아니라 해당 roadmap 섹션 아래 planning-doc link에 둔다.
- 실제 integration example이 구현되면 해당 example PR에서 `README.md`와 `README.ko.md`를 함께 갱신한다.

#81 이후 권장 위치:

```markdown
## Roadmap Planning

- [Workshop roadmap matrix](docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md)
- [Example selection scorecard](docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md)
- [Integration example template and acceptance rubric](docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md)
- [Cross-milestone integration blueprint](docs/superpowers/plans/<issue-81-file>.md)
```

Future planned track이 example directory가 생기기 전에 실행 가능해 보이지 않도록 이 섹션은 implemented example과 분리한다.

## DoD Evidence For #80

- Template/rubric artifact가 `docs/superpowers/plans`에 있다.
- Rubric이 기존 foundation integration #72, #75, #78을 참조한다.
- Rubric이 future integration issue #65, #66, #67, #68, #69를 참조한다.
- 영어 및 한국어 README section template이 정의되어 있다.
- Integration PR에서 기대하는 validation evidence가 정의되어 있다.
- Gin, `net/http`, chi, Testcontainers, Floci, diagram 사용 규칙이 명시되어 있다.
- 붙여 넣을 수 있는 future PR checklist가 포함되어 있다.
- README navigation plan explains where this template belongs after #81.
