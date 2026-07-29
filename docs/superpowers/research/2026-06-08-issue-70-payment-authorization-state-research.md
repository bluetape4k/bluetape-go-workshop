# Issue #70 리서치: Payment Authorization State Transition 예제

## 맥락

- 이슈: `#70 [v0.4.0] Add payment authorization state transition example`
- umbrella: `#28 [v0.4.0] Add state machine and workflow workshop examples`
- 상위 roadmap epic: `#27`
- 현재 완료된 focused example:
  - `#38` order lifecycle state API
  - `#39` fulfillment workflow runner
  - `#40` operations report policy

## 저장소 근거

- `examples/order-lifecycle-state-api`는 이미 Gin route, allowed event, guard
  rejection, final-state rejection, concurrent transition test로 일반적인 order
  finite state machine을 가르친다.
- #70 예제는 전체 order lifecycle API를 반복하지 않아야 한다. domain을 payment
  authorization으로 좁히고, `state.Machine` 주변의 idempotent command retry 동작이라는
  새 application concern 하나를 추가해야 한다.
- 루트 README는 이제 v0.4.0을 state/workflow/report example로 묶으므로, #70은 같은
  lane 아래 payment-specific state example로 추가할 수 있다.

## 라이브러리 근거

`github.com/bluetape4k/bluetape-go@v0.5.1/state`에서 확인한 내용은 다음과 같다.

- `state.Machine`은 concurrency-safe하다.
- `Transition(ctx, event)`:
  - lookup 전과 commit 전에 context를 확인한다.
  - write lock을 얻기 전에 guard를 평가한다.
  - stale concurrent transition을 `ErrConcurrentTransition`으로 거부한다.
- `CanTransition(ctx, event)`는 guard를 평가하지만 state를 변경하지 않는다.
- `AllowedEvents()`는 현재 state에 등록된 event를 반환하며 guard를 평가하지 않는다.
- final state는 추가 transition을 `ErrFinalState`로 거부한다.
- 패키지는 idempotency를 구현하지 않는다. idempotent retry 동작은 application
  boundary에 있어야 한다.

## 시나리오 결정

단일 in-memory payment authorization용 Gin API인 `examples/payment-authorization-state`를
만든다.

상태:

- `requested`
- `authorized`
- `captured`
- `failed`
- `cancelled`

이벤트:

- `authorize`: `requested -> authorized`, positive amount로 guard한다.
- `capture`: `authorized -> captured`.
- `fail`: `requested -> failed`, `authorized -> failed`.
- `cancel`: `requested -> cancelled`, `authorized -> cancelled`.

최종 상태:

- `captured`
- `failed`
- `cancelled`

멱등성:

- transition command는 `idempotency_key`를 받는다.
- 성공한 command는 key별로 response를 저장한다.
- 같은 key와 event를 반복하면 저장된 transition response를 `idempotent_replay=true`로
  반환하고 state를 변경하지 않는다.
- 같은 key를 다른 event로 재사용하면 `409 Conflict`를 반환한다.
- invalid 또는 guard-rejected transition은 successful idempotent result로 저장하지 않는다.

## 거부한 방향

- 외부 payment gateway 호출: state package lesson에는 불필요한 infrastructure다.
- guard 내부에 request-scoped mutable state로 gateway approval 인코딩: guard는
  `CanTransition` 호출에도 안전해야 한다.
- order lifecycle 예제 path 재사용: #70은 별도의 payment-domain lesson으로 읽혀야
  하며 #38을 prerequisite로 연결해야 한다.
- durable idempotency store 구축: 범위 밖이다. README는 production idempotency에
  durable storage가 필요하다고 명시해야 한다.

## 인수 기준 매핑

- `examples/` 아래 runnable example: `examples/payment-authorization-state`를 추가한다.
- valid transition: 테스트가 authorize, capture, fail, cancel path를 다룬다.
- invalid transition: 테스트가 authorize 전 capture와 final-state rejection을 다룬다.
- retry/idempotency: 테스트가 same-key replay와 key reuse conflict를 다룬다.
- README prerequisite: README가 `examples/order-lifecycle-state-api`를 연결한다.
- README sync: English와 Korean README 및 root navigation을 추가한다.
