# Issue #71 설계: Compensation Workflow 예제

## 목표

순서가 있는 workflow 실행과, 이후 fulfillment 단계가 실패했을 때 수행하는
rollback-style compensation을 보여주는 집중 v0.4.0 예제를 추가한다.

## 비목표

- durable saga coordinator, queue, database, retry scheduler, 외부
  inventory/payment/shipping service를 구현하지 않는다.
- `workflow`가 compensation을 자체 제공한다고 주장하지 않는다.
- #39의 더 넓은 fulfillment runner 예제를 복제하지 않는다.
- 원래의 forward failure를 compensation report 뒤에 숨기지 않는다.

## 예제

- 경로: `examples/compensation-workflow`
- 패키지: `internal/compensation`
- HTTP framework: Gin
- 기본 포트: `:8087`

## 시나리오

Fulfillment workflow는 세 개의 forward step을 실행한다.

1. `reserve-inventory`
2. `authorize-payment`
3. `create-shipment`

처음 두 단계는 되돌릴 수 있다. Inventory와 payment가 성공한 뒤
`create-shipment`가 실패하면, 예제는 역순으로 compensation을 실행한다.

1. `void-payment`
2. `release-inventory`

응답은 원래 shipment error를 보존하고 compensation report tree도 함께 노출한다.

## API

### `GET /healthz`

`{"status":"ok"}`와 함께 `200 OK`를 반환한다.

### `POST /compensation/fulfillment`

요청:

```json
{
  "order_id": "order-1001",
  "stock_available": true,
  "payment_authorized": true,
  "shipment_provider_available": false,
  "void_payment_fails": false,
  "release_inventory_fails": false
}
```

규칙:

- `order_id`는 공백 trim 이후 필수다.
- `stock_available=false`는 inventory compensation 등록 전에 실패한다.
- `payment_authorized=false`는 inventory reservation 이후 실패하고 inventory를
  release한다.
- `shipment_provider_available=false`는 inventory reservation과 payment
  authorization 이후 실패한 다음 payment를 void하고 inventory를 release한다.
- `void_payment_fails=true`는 실패한 payment compensation을 기록하지만
  inventory release는 계속 실행한다.
- `release_inventory_fails=true`는 실패한 inventory compensation을 기록한다.
- 잘못된 JSON과 유효하지 않은 요청 필드는 `400 Bad Request`를 반환한다.
- Caller cancellation은 `408 Request Timeout`으로 mapping한다.

응답:

```json
{
  "order_id": "order-1001",
  "completed": false,
  "compensated": true,
  "original_error": "shipment provider is unavailable",
  "effects": {
    "inventory_reserved": false,
    "payment_authorized": false,
    "shipment_created": false
  },
  "report": {
    "name": "compensating-fulfillment",
    "status": "failed",
    "children": []
  }
}
```

## Report 형태

안정적인 DTO를 노출한다.

- `name`
- `status`
- `error`
- `reason`
- `success`
- `failure`
- `partial`
- `cancelled`
- `children`

Timestamp는 의도적으로 생략한다.

## 설계

`compensationRun`은 요청 범위 side-effect flag와 compensation handler stack을
소유한다. Forward runner는 다음처럼 구성한다.

```go
workflow.Sequential(
    "fulfillment-forward",
    workreport.StopOnFailure,
    r.reserveInventory,
    r.authorizePayment,
    r.createShipment,
)
```

성공한 reversible step은 자신의 compensation work를 stack에 추가한다. Forward
report가 성공이 아니면 compensation handler를 역순으로 복사하고 다음 runner로
실행한다.

```go
workflow.Sequential("compensation", workreport.ContinueOnFailure, works...)
```

Top-level report는 원래 forward error를 자신의 error로 유지하고, forward 및
compensation child report를 모두 embed한다.

## HTTP Status Mapping

- completed workflow -> `200 OK`
- compensated forward failure -> `409 Conflict`
- cancellation -> `408 Request Timeout`
- invalid JSON/request -> `400 Bad Request`
- unexpected report state -> `500 Internal Server Error`

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- health endpoint
- compensation 없는 성공 실행
- shipment failure가 `void-payment` 이후 `release-inventory`를 실행하는지
- payment failure가 `release-inventory`만 실행하는지
- compensation failure가 원래 shipment error를 보존하면서 남은 compensation을
  계속 실행하는지
- inventory failure에 compensation work가 없는지
- caller cancellation이 `408`로 mapping되는지
- 잘못된 JSON과 유효하지 않은 요청 필드가 `400`을 반환하는지
- 예제 package 대상 race test

## 문서 및 다이어그램

영어 및 한국어 README 파일에 다음을 추가한다.

- Example Scenario
- compensation이 단순 state transition과 다른 시점
- API 및 응답 예제
- Architecture
- Sequence Diagram
- 운영 환경 hardening notes

`docs/images/readme-diagrams/` 아래에 README 다이어그램 자산을 생성한다.

- `compensation-workflow-scenario`
- `compensation-workflow-architecture`
- `compensation-workflow-sequence`

README 파일은 PNG만 embed하고 생성된 다이어그램 label은 영어로 유지한다.
