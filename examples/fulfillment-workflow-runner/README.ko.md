# Fulfillment Workflow Runner

[English](README.md) | [한국어](README.ko.md)

이 예제는 request-scoped fulfillment workflow를 작은 Gin HTTP API로 노출합니다.
`github.com/bluetape4k/bluetape-go/workflow`로 sequential, parallel,
conditional runner를 조합하고, `workreport`로 안정적인 실행 tree를 반환합니다.

## 예제 시나리오

API는 하나의 fulfillment request를 받습니다. Workflow는 먼저 주문을 검증하고,
inventory 예약과 payment authorization을 병렬로 실행합니다. 두 risk check가 모두
완료되면 conditional step이 shipment를 생성하거나 pickup/digital delivery라서
shipment를 건너뛰었다고 기록합니다. Risk check가 실패하면 workflow를 중단하고
느린 sibling step을 취소합니다. Caller cancellation은 cancelled report로
반환됩니다.

![Fulfillment workflow scenario](../../docs/images/readme-diagrams/fulfillment-workflow-runner-scenario.png)

## Workflow Steps

| Step | Runner | Boundary | 실패 동작 |
|---|---|---|---|
| `validate-order` | `workflow.Sequential` child | Fan-out 전에 request를 검증합니다. | Risk check 실행 전에 workflow를 중단합니다. |
| `risk-checks` | `workflow.Parallel` | `reserve-inventory`와 `authorize-payment`를 `StopOnFailure`로 실행합니다. | `409 Conflict`를 반환하고 끝나지 않은 sibling을 취소합니다. |
| `shipment-decision` | `workflow.Conditional` | `create-shipment` 또는 `shipment-skipped`를 선택합니다. | Risk check 실패나 context 취소 시 실행되지 않습니다. |

HTTP handler는 `workreport.Report`를 stable JSON으로 투영하고 runtime timestamp를
의도적으로 제외합니다. 그래서 테스트는 timing noise 없이 실행 tree를 검증할 수
있습니다.

## 실행

```bash
go run ./examples/fulfillment-workflow-runner
```

선택 환경 변수:

| 변수 | 기본값 | 목적 |
|---|---:|---|
| `HTTP_ADDR` | `:8084` | Listen 주소입니다. |

## Endpoints

```bash
curl http://localhost:8084/healthz
curl -X POST http://localhost:8084/fulfillment/run \
  -H 'Content-Type: application/json' \
  -d '{
    "order_id": "order-1001",
    "stock_available": true,
    "payment_authorized": true,
    "requires_shipment": true
  }'
```

유용한 변형:

```bash
# Conditional skip path.
curl -X POST http://localhost:8084/fulfillment/run \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"pickup-1001","stock_available":true,"payment_authorized":true,"requires_shipment":false}'

# Risk-check failure path.
curl -X POST http://localhost:8084/fulfillment/run \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1002","stock_available":false,"payment_authorized":true,"requires_shipment":true}'
```

성공한 report는 `200 OK`를 반환합니다. Failed 또는 partial workflow report는
`409 Conflict`를 반환합니다. Cancelled workflow report는 `408 Request Timeout`을
반환합니다. 잘못된 JSON, 누락된 `order_id`, blank `order_id`, 범위를 벗어난
`inventory_delay_ms`는 `400 Bad Request`를 반환합니다.

## 언제 이 정도면 충분한가

하나의 HTTP request가 workflow 전체를 소유할 수 있고, 모든 step이 동기적으로
끝나며, caller가 실행 report만 필요로 할 때 이 패턴이면 충분합니다. Boundary
예제, validation/risk fan-out, 단순 command orchestration에 적합합니다.

Service restart를 넘어 살아남아야 하거나, human approval을 기다리거나, 긴 시간
창에서 retry해야 하거나, 이미 commit된 side effect를 보상하거나, 저장된 state에서
resume해야 한다면 durable workflow engine이나 persistent job system을 사용해야
합니다.

## Architecture

Gin은 routing, JSON binding, HTTP status mapping을 맡습니다. Request handler는
매 요청마다 새 workflow runner를 구성하므로 각 실행은 자기 flag와 context
cancellation을 유지합니다. Workflow package는 실행 순서를 맡고, `workreport`는
결과 tree를 맡습니다.

![Fulfillment workflow architecture](../../docs/images/readme-diagrams/fulfillment-workflow-runner-architecture.png)

## Sequence Diagram

주요 request path는 동기적입니다. Handler는 sequential workflow를 실행하고,
workflow는 risk check를 병렬로 실행하며, conditional step은 risk check가 성공한
경우에만 shipment branch를 기록합니다.

![Fulfillment workflow sequence](../../docs/images/readme-diagrams/fulfillment-workflow-runner-sequence.png)

## 테스트

```bash
go test -count=1 ./examples/fulfillment-workflow-runner/...
go test -race -count=1 ./examples/fulfillment-workflow-runner/...
```
