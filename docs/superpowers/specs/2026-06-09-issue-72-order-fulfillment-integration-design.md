# Issue #72 설계: Order Fulfillment Workflow Integration 예제

## 목표

Order lifecycle state transition, fulfillment workflow 실행, report projection,
failure policy, compensation을 하나의 실행 가능한 Gin API로 결합하는
milestone-level 0.4.0 통합 예제를 추가한다.

## 비목표

- durable workflow engine, saga coordinator, queue, database, retry scheduler,
  외부 inventory/payment/shipping service를 추가하지 않는다.
- 이전 예제 package를 구현 의존성으로 import하지 않는다.
- `state`, `workflow`, `workreport`를 generic framework 뒤에 숨기지 않는다.
- request-scoped compensation이 production-grade durability라고 주장하지 않는다.

## 예제

- 경로: `examples/order-fulfillment-integration`
- 패키지: `internal/orderfulfillment`
- HTTP framework: Gin
- 기본 포트: `:8088`

## 시나리오

API는 order fulfillment 요청을 받고 요청 범위 fulfillment 하나를 실행한다.

1. `submit-order`는 lifecycle을 `draft`에서 `submitted`로 transition한다.
2. `reserve-inventory`는 inventory side effect를 기록하고 `release-inventory`를
   등록한다.
3. `authorize-payment`는 lifecycle을 `submitted`에서 `paid`로 transition하고,
   payment side effect를 기록하며 `void-payment`를 등록한다.
4. `pack-order`는 lifecycle을 `paid`에서 `packed`로 transition한다.
5. `create-shipment`는 shipment provider를 사용할 수 있을 때 lifecycle을
   `packed`에서 `shipped`로 transition한다.

Reversible side effect 이후 나중 단계가 실패하면, 예제는 등록된 compensation을
역순으로 실행하고 해당 transition이 합법적일 때 lifecycle을 `cancelled`로
transition한다. 응답은 원래 failure를 보존하고 forward 및 compensation report
tree를 모두 포함한다.

## Lifecycle 모델

상태:

- `draft`
- `submitted`
- `paid`
- `packed`
- `shipped`
- `cancelled`

Event:

- `submit`: `draft -> submitted`
- `pay`: `submitted -> paid`
- `pack`: `paid -> packed`
- `ship`: `packed -> shipped`
- `cancel`: `draft|submitted|paid|packed -> cancelled`

Final state:

- `shipped`
- `cancelled`

## API

### `GET /healthz`

`{"status":"ok"}`와 함께 `200 OK`를 반환한다.

### `POST /orders/fulfillment`

요청:

```json
{
  "order_id": "order-1001",
  "total_cents": 2599,
  "stock_available": true,
  "payment_authorized": true,
  "shipment_provider_available": true,
  "force_invalid_transition": false,
  "void_payment_fails": false,
  "release_inventory_fails": false
}
```

검증:

- `order_id`는 공백 trim 이후 필수다.
- `total_cents`는 양수여야 한다.
- 잘못된 JSON과 유효하지 않은 필드는 `400 Bad Request`를 반환한다.

시나리오 flag:

- `stock_available=false`는 inventory side effect 전에 실패한다.
- `payment_authorized=false`는 inventory reservation 이후 실패하고 inventory를
  release한다.
- `shipment_provider_available=false`는 inventory reservation과 payment
  authorization 이후 실패한 다음 payment를 void하고 inventory를 release한다.
- `force_invalid_transition=true`는 workflow 내부 state-machine invalid
  transition을 보여주기 위해 payment authorization 이후 `pack` 전에 `ship`을
  의도적으로 시도한다.
- `void_payment_fails=true`는 실패한 payment compensation을 기록하면서도
  inventory release를 계속 실행한다.
- `release_inventory_fails=true`는 실패한 inventory compensation을 기록한다.

응답:

```json
{
  "order_id": "order-1001",
  "state": "shipped",
  "state_history": ["draft", "submitted", "paid", "packed", "shipped"],
  "completed": true,
  "compensated": false,
  "original_error": "",
  "effects": {
    "inventory_reserved": true,
    "payment_authorized": true,
    "shipment_created": true
  },
  "summary": {
    "completed": 5,
    "failed": 0,
    "partial": 0,
    "cancelled": 0,
    "total": 5,
    "success": true,
    "failure": false
  },
  "report": {
    "name": "order-fulfillment",
    "status": "completed",
    "children": []
  }
}
```

Report projection은 안정적이며 런타임 timestamp를 생략한다.

## HTTP Status Mapping

- successful fulfillment -> `200 OK`
- invalid transition, domain failure, compensated failure -> `409 Conflict`
- caller cancellation -> `408 Request Timeout`
- invalid request -> `400 Bad Request`
- unexpected report state -> `500 Internal Server Error`

## 설계

`orderRun`은 요청 범위 state machine 하나, side-effect flag, state history,
compensation stack을 소유한다.

Forward runner:

```go
workflow.Sequential(
    "order-fulfillment",
    workreport.StopOnFailure,
    r.submitOrder,
    r.reserveInventory,
    r.authorizePayment,
    r.packOrder,
    r.createShipment,
)
```

Compensation runner:

```go
workflow.Sequential(
    "compensation",
    workreport.ContinueOnFailure,
    r.reverseCompensations()...,
)
```

Top-level response는 `operations-report-policy`와 비슷한 `workreport` status
predicate 및 summary helper를 사용하지만, mutable runtime timestamp field는
노출하지 않는다.

## 다이어그램

`docs/images/readme-diagrams/` 아래에 README 다이어그램 자산을 생성한다.

- `order-fulfillment-integration-scenario`
- `order-fulfillment-integration-architecture`
- `order-fulfillment-integration-sequence`

README 파일은 PNG만 embed한다. SVG 파일은 review를 위해 PNG 옆에 유지한다.
Graphviz `.dot`, `.plain`, `*-graphviz.svg`, `*-graphviz.png`는 route evidence로
남긴다. 최종 README SVG/PNG 자산은 raw Graphviz output이 아니라 기존 decorated
workshop baseline을 사용해야 한다.

Diagram generator는 `margins=L/R/T/B`를 포함한 구체적인 geometry evidence를
출력해야 한다.

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- health endpoint
- happy path가 `shipped`에 도달하고, 기대한 모든 lifecycle state를 기록하며,
  successful report summary를 갖는지
- invalid transition 시나리오가 `409`를 반환하고 state-machine error를
  보존하며 failed report node를 노출하는지
- shipment-provider failure가 `409`를 반환하고 `void-payment` 이후
  `release-inventory`를 실행하며 lifecycle을 `cancelled`로 transition하고 원래
  shipment error를 보존하는지
- compensation failure가 원래 shipment error를 보존하면서 compensation child
  failure를 보고하는지
- fulfillment 전 또는 중 caller cancellation이 `408`로 mapping되고, reversible
  side effect가 이미 등록된 경우 반환 전에 compensation을 계속 실행하는지
- 잘못된 JSON, 빈 `order_id`, 양수가 아닌 total이 `400`을 반환하는지
- parallel HTTP request가 독립 state와 side-effect flag를 유지하는지
- `./examples/order-fulfillment-integration/...` 대상 race test

## 문서

영어 및 한국어 README 파일에 다음을 추가한다.

- Example Scenario
- 이 예제가 더 작은 0.4.0 예제들을 통합하는 방식
- API 및 응답 예제
- Architecture
- Sequence Diagram
- durability, idempotency, retry, audit, external service integration에 대한
  production hardening note

루트 `README.md`와 `README.ko.md`의 예제 표, quickstart 섹션, 0.4.0 roadmap
row를 갱신한다.
