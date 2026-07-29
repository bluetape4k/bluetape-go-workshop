# Issue #71 리서치: Compensation Workflow 예제

## 확인한 출처

- GitHub issue #71: focused v0.4.0 compensation workflow example 범위.
- GitHub issue #28: 0.4.0 umbrella track과 sequencing.
- `examples/fulfillment-workflow-runner`: request-scoped Gin workflow runner 구조.
- `examples/operations-report-policy`: 안정적인 `workreport.Report` projection과
  HTTP status mapping.
- `examples/payment-authorization-state`: payment-specific state와 retry boundary.
- `github.com/bluetape4k/bluetape-go@v0.4.0/workflow`: `Sequential`,
  `Parallel`, `Conditional`, and context-aware `Work`.
- `github.com/bluetape4k/bluetape-go@v0.4.0/workreport`: report status,
  failure policy, aggregation, caller-visible error.

## 현재 패키지 근거

- `workflow.Sequential`은 work function을 입력 순서대로 실행하고
  `workreport.FailurePolicy`에 따라 중단한다.
- `workreport.Report`는 caller-visible `Err` 값과 child report 순서를 보존한다.
- `workflow`는 durable compensation이나 rollback semantic을 제공하지 않는다.
  따라서 compensation 예제는 built-in saga engine을 주장하지 말고 runner 주변의
  application boundary를 보여 주어야 한다.

## 예제 선택

이 이슈 시나리오는 이전 0.4.0 예제들을 조합하므로 적합하다.

- inventory reservation은 되돌릴 수 있는 side effect다.
- payment authorization은 되돌릴 수 있는 side effect다.
- shipment creation은 실패 시 reverse compensation이 필요한 후속 step이다.
- report tree는 원래 실패와 compensation outcome을 모두 보존할 수 있다.

## 결정

`examples/compensation-workflow`를 Gin 예제로 만든다.

forward path는 다음 항목에 `workflow.Sequential`을 사용한다.

1. `reserve-inventory`
2. `authorize-payment`
3. `create-shipment`

성공한 각 reversible step은 하나의 compensation handler를 등록한다. 후속 step이
실패하면 예제는 등록된 compensation handler를 역순으로
`workflow.Sequential("compensation", workreport.ContinueOnFailure, ...)`로 실행해,
하나의 compensation 실패가 뒤따르는 cleanup을 숨기거나 중단하지 않게 한다.

## 제약

- 모든 side effect는 request-scoped memory flag 안에 둔다.
- top-level response에 원래 forward error를 보존한다.
- timestamp 없는 안정적인 JSON report projection을 반환한다.
- production compensation에는 durable state, idempotent compensator, retry policy,
  audit logging, operator intervention path가 필요하다고 문서화한다.
