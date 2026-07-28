# Issue #72 리서치: Order Fulfillment Workflow Integration 예제

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #72 `[v0.4.0] Add order fulfillment workflow integration example`
- 마일스톤: `0.4.0`
- 작업 유형: Type A - Full Feature

## 확인한 출처

- GitHub issue #72.
- compensation workflow에 대한 closed issue #71과 PR #86 merge result.
- 기존 0.4.0 examples:
  - `examples/order-lifecycle-state-api`
  - `examples/payment-authorization-state`
  - `examples/fulfillment-workflow-runner`
  - `examples/operations-report-policy`
  - `examples/compensation-workflow`
- 기존 design/research/lesson artifacts:
  - `docs/superpowers/specs/2026-06-08-issue-39-fulfillment-workflow-runner-design.md`
  - `docs/superpowers/specs/2026-06-08-issue-71-compensation-workflow-design.md`
  - `docs/superpowers/research/2026-06-08-issue-71-compensation-workflow-research.md`
  - `docs/lessons/2026-06-08-fulfillment-workflow-runner.md`
  - `docs/lessons/2026-06-08-compensation-workflow.md`
- 0.4.0 state/workflow/workreport research에 대한 GNO `bluetape4k-docs` 결과.

## 현재 근거

0.4.0 워크숍에는 개별 primitive를 위한 focused example이 이미 있다.

- `order-lifecycle-state-api`는 `state.Machine` transition legality, final state,
  guard rejection, 안정적인 HTTP error mapping을 보여 준다.
- `payment-authorization-state`는 payment-specific state transition과
  application-layer idempotent replay를 보여 준다.
- `fulfillment-workflow-runner`는 `Sequential`, `Parallel`, `Conditional`,
  cancellation, 안정적인 `workreport.Report` projection을 포함하는 request-scoped
  `workflow` composition을 보여 준다.
- `operations-report-policy`는 `StopOnFailure`와 `ContinueOnFailure`의 결정적인
  `workreport` aggregation 및 summary projection을 보여 준다.
- `compensation-workflow`는 application-owned compensation stack, reverse cleanup
  order, original-error preservation, cleanup failure reporting을 보여 준다.

#72는 또 다른 focused primitive example이 아니라 milestone-level integration example을
요구한다. 따라서 새 예제는 code를 request-scoped와 workshop size로 유지하면서, 이
개념들이 하나의 order flow에서 어떻게 맞물리는지 보여 주어야 한다.

## 설계 결정

`examples/order-fulfillment-integration` 아래 새 Gin 예제를 만든다.

예제는 HTTP request마다 하나의 request-scoped order run을 만든다.

1. local `state.Machine`이 order lifecycle state를 소유한다.
2. `workflow.Sequential` runner가 step execution order를 소유한다.
3. 성공한 reversible step은 compensation handler를 등록한다.
4. 후속 workflow 실패 뒤 compensation runner는 `workreport.ContinueOnFailure`와 함께
   `workflow.Sequential`을 사용한다.
5. `workreport.Report`는 summary가 포함된 안정적인 JSON response로 project된다.

이렇게 하면 durable orchestration, persistence, queue, external service,
background worker를 피하면서도 예제를 production vocabulary에 가깝게 유지할 수 있다.

## 거부한 선택지

- 기존 example package 직접 재사용: 이 package들은 의도적으로 example-local이며
  request/response shape가 서로 다르다. import하면 integration example이 통합 domain
  flow를 가르치기보다 wrapper glue가 된다.
- durable saga engine 또는 in-memory run store 추가: milestone primitive는
  long-running orchestration이 아니라 request-scoped state/workflow behavior다.
- database, queue, Redis, Testcontainers 의존성 추가: #72는 package behavior가 요구하지
  않는 한 external dependency를 제외하라고 한다.

## 구현 제약

- #72가 public service example을 명시적으로 요구하므로 Gin을 사용한다.
- 모든 mutable state는 request-scoped로 유지한다.
- JSON에 `workreport.Report` timestamp를 노출하지 않는다.
- compensation도 실패할 때 original workflow error를 보존한다.
- `bluetape4k-diagram`을 따르는 diagram asset을 포함한다.
  - 최종 README PNG/SVG asset은 기존 decorated workshop baseline을 사용한다.
  - Graphviz `.dot`, `.plain`, `*-graphviz.*` artifact는 route evidence로 남긴다.
  - generator는 구체적인 L/R/T/B margin evidence를 출력한다.
  - render된 각 PNG를 시각적으로 검사한다.

## 테스트 영향

focused test는 다음을 다루어야 한다.

- health endpoint
- happy path가 `shipped`에 도달하는지
- invalid lifecycle transition이 state/report evidence가 있는 `409`로 매핑되는지
- reversible step 뒤 shipment failure가 `409`로 매핑되고, lifecycle을 `cancelled`로
  설정하며, compensation을 역순으로 실행하는지
- compensation failure가 original shipment error를 보존하는지
- malformed JSON과 invalid request field가 `400`으로 매핑되는지
- parallel request가 lifecycle 또는 side-effect state를 공유하지 않는지
- example package에 대한 race test
