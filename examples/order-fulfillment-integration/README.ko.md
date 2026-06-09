# Order Fulfillment Integration

[English](README.md) | [한국어](README.ko.md)

이 예제는 v0.4.0 milestone 통합 시나리오입니다. Gin HTTP API 하나에서
`state`, `workflow`, `workreport`를 조합해 request-scoped 주문 fulfillment
흐름과 application-owned compensation을 함께 보여줍니다.

## 예제 시나리오

API는 주문 fulfillment 요청 하나를 받습니다. Forward workflow는 주문을
submit하고, inventory를 reserve하고, payment를 authorize하고, order를 pack한 뒤
shipment를 생성합니다. 성공하면 lifecycle state machine은 `draft -> submitted ->
paid -> packed -> shipped`를 기록합니다.

Inventory reservation과 payment authorization은 되돌릴 수 있는 side effect입니다.
뒤 단계가 실패하면 예제는 compensation을 역순으로 실행하고, original forward
error를 보존하며, state machine이 허용할 때 주문을 `cancelled`로 전환합니다.

![Order fulfillment integration scenario](../../docs/images/readme-diagrams/order-fulfillment-integration-scenario.png)

## 작은 0.4.0 예제들의 통합 위치

| 원본 예제 | 이 예제에서의 역할 |
|---|---|
| [`order-lifecycle-state-api`](../order-lifecycle-state-api/README.ko.md) | 주문 lifecycle state machine과 합법 transition 검증. |
| [`payment-authorization-state`](../payment-authorization-state/README.ko.md) | Payment authorization을 application-level stateful side effect로 다루는 부분. |
| [`fulfillment-workflow-runner`](../fulfillment-workflow-runner/README.ko.md) | 첫 failure에서 멈추는 sequential fulfillment workflow. |
| [`operations-report-policy`](../operations-report-policy/README.ko.md) | Timestamp noise 없는 stable report projection과 summary count. |
| [`compensation-workflow`](../compensation-workflow/README.ko.md) | Reverse-order cleanup과 original error 보존. |

## 실행

```bash
go run ./examples/order-fulfillment-integration
```

선택 환경 변수:

| 변수 | 기본값 | 용도 |
|---|---:|---|
| `HTTP_ADDR` | `:8088` | Listen 주소. |

## Endpoints

```bash
curl http://localhost:8088/healthz
curl -X POST http://localhost:8088/orders/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{
    "order_id": "order-1001",
    "total_cents": 2599,
    "stock_available": true,
    "payment_authorized": true,
    "shipment_provider_available": true
  }'
```

유용한 변형:

```bash
# Shipment failure는 void-payment, release-inventory, cancellation을 유발합니다.
curl -X POST http://localhost:8088/orders/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1002","total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":false}'

# Workflow 내부 invalid transition도 완료된 side effect를 compensation합니다.
curl -X POST http://localhost:8088/orders/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1003","total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":true,"force_invalid_transition":true}'

# Compensation failure가 있어도 original shipment error는 유지합니다.
curl -X POST http://localhost:8088/orders/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1004","total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":false,"void_payment_fails":true}'
```

성공한 fulfillment는 `200 OK`를 반환합니다. Domain failure와 compensated failure는
`409 Conflict`를 반환합니다. Caller cancellation은 `408 Request Timeout`입니다.
Malformed JSON, blank `order_id`, non-positive `total_cents`는 `400 Bad Request`를
반환합니다.

응답에는 final state, lifecycle history, side-effect flags, stable report summary,
`original_error`, timestamp-free report tree가 포함됩니다.

## Architecture

Gin은 routing, JSON binding, HTTP status mapping을 맡습니다. 각 요청은 자체
`state.Machine`, state history, side-effect flags, compensation stack을 가진
새 `orderRun`을 만듭니다. `workflow.Sequential`은 forward step을 `StopOnFailure`로
실행합니다. 되돌릴 수 있는 side effect가 있고 뒤 단계가 실패하면 두 번째
`workflow.Sequential`이 compensation stack을 `ContinueOnFailure`로 실행합니다.

![Order fulfillment integration architecture](../../docs/images/readme-diagrams/order-fulfillment-integration-architecture.png)

## Sequence Diagram

요청 경로는 동기식입니다. Handler는 forward workflow를 실행하고, success 또는
failed report를 받은 뒤 필요하면 cleanup context로 reverse compensation을 실행한
후 stable response projection을 반환합니다.

![Order fulfillment integration sequence](../../docs/images/readme-diagrams/order-fulfillment-integration-sequence.png)

## Production Hardening

이 예제는 의도적으로 request-scoped입니다. Production fulfillment service라면
durable order state, idempotent inventory/payment/shipping command, retry/timeout
policy, audit record, outbox 또는 queue, process restart 이후 cleanup recovery가
필요합니다.

## 테스트

```bash
go test -count=1 ./examples/order-fulfillment-integration/...
go test -race -count=1 ./examples/order-fulfillment-integration/...
```
