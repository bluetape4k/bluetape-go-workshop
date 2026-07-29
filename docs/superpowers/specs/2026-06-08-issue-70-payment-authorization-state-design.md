# Issue #70 설계: Payment Authorization State 예제

## 목표

`state.Machine`을 사용해 payment authorization 상태 전이, 유효하지 않은 전이
오류, idempotent retry 동작을 보여주는 집중 v0.4.0 예제를 추가한다.

## 비목표

- 실제 payment gateway를 호출하지 않는다.
- database, queue, durable idempotency store, workflow runner를 추가하지 않는다.
- 큰 application framework 뒤에 `state.Machine`을 숨기지 않는다.
- #38의 더 넓은 order lifecycle 예제를 복제하지 않는다.

## 예제

- 경로: `examples/payment-authorization-state`
- 패키지: `internal/paymentauth`
- HTTP framework: Gin
- 기본 포트: `:8086`

## 상태 모델

| From | Event | To | 규칙 |
|---|---|---|---|
| `requested` | `authorize` | `authorized` | `amount_cents`는 양수여야 한다. |
| `authorized` | `capture` | `captured` | Captured는 final이다. |
| `requested`, `authorized` | `fail` | `failed` | Failed는 final이다. |
| `requested`, `authorized` | `cancel` | `cancelled` | Cancelled는 final이다. |

Final state는 이후 transition 시도를 거부한다.

## API

### `GET /healthz`

`{"status":"ok"}`와 함께 `200 OK`를 반환한다.

### `GET /payments/current`

반환:

```json
{
  "payment_id": "pay-1001",
  "state": "requested",
  "amount_cents": 12900,
  "allowed_events": ["authorize", "fail", "cancel"]
}
```

### `GET /payments/current/transitions/:event/can`

현재 상태에서 event를 실행할 수 있는지 평가한다. Guard rejection은
`409 Conflict`로 mapping하고, 사용할 수 없는 event는 `allowed=false`와 함께
`200 OK`를 반환한다.

### `POST /payments/current/transitions`

요청:

```json
{
  "event": "authorize",
  "idempotency_key": "auth-1"
}
```

응답:

```json
{
  "payment": {},
  "previous": "requested",
  "event": "authorize",
  "current": "authorized",
  "idempotent_replay": false
}
```

규칙:

- `event`와 `idempotency_key`는 공백 trim 이후 필수다.
- 같은 `idempotency_key`와 같은 event를 반복하면 저장된 성공 transition 응답을
  `idempotent_replay=true`와 함께 반환한다.
- 같은 `idempotency_key`를 다른 event에 재사용하면 `idempotency_conflict`
  code와 함께 `409 Conflict`를 반환한다.
- 알 수 없는 event는 `400 Bad Request`를 반환한다.
- 유효하지 않은 transition, final-state transition, guard rejection, concurrent
  transition conflict는 `409 Conflict`를 반환한다.
- 취소된 context나 deadline context는 `408 Request Timeout`을 반환한다.

## 구현 메모

- `state.Machine`은 현재 상태와 허용 event의 source of truth로 남는다.
- 작은 `idempotencyStore`가 mutex로 성공 transition 응답을 보호한다.
- Replay metadata와 state change가 일관되게 남도록 transition과 idempotency
  record write는 server-level mutex로 직렬화한다.
- Snapshot 응답은 안정적인 DTO를 사용한다.

## 다이어그램

`docs/images/readme-diagrams/` 아래에 PNG와 SVG 자산을 생성하고 commit한다.

- `payment-authorization-state-scenario`
- `payment-authorization-state-architecture`
- `payment-authorization-state-sequence`

README는 PNG만 embed하고 생성된 label은 영어로 유지한다.

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- health/current endpoint
- 허용된 authorize/capture 경로
- final state로 가는 fail 및 cancel 경로
- capture-before-authorize invalid transition
- 양수가 아닌 amount에 대한 guard rejection
- 추가 mutation 없는 same-key idempotent replay
- same-key different-event conflict
- final-state rejection
- malformed/missing/unknown event 요청
- concurrent duplicate authorize 안전성
- 예제 package 대상 race test

## 운영 환경 Hardening 메모

README hardening gap은 다음을 명시해야 한다.

- durable payment state storage
- TTL과 request hash validation을 갖춘 durable idempotency-key storage
- 실제 payment gateway integration
- audit logging과 observability
- domain reporting에서 authorization failure와 operator cancellation의 분리
