# Issue 38 Order Lifecycle State API Design

## 분류

- Work type: Type A - Full Feature.
- Basis: issue #38은 새 runnable Gin example directory, Go code, test,
  English/Korean README file, root README entry, review artifact, lesson, PR을 추가한다.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `feat/issue-38-order-lifecycle-state-api` under
  `.worktrees/feat-issue-38-order-lifecycle-state-api`.

## 문제

workshop에는 realistic HTTP boundary 안에서 `state` package가 어떻게 동작하는지 보여주는 0.4.0 example이 필요하다.
이 example은 state-machine API catalog page가 아니어야 한다. reader가 current state를 inspect하고,
command로 state를 advance하며, invalid transition이 명확히 report되는 모습을 볼 수 있는 작은 order lifecycle service여야 한다.

## 현재 근거

- GitHub issue #38은 released state machine primitive를 감싼 Gin HTTP example을 요구한다.
- parent issue #28은 milestone 전체에서 state/workflow primitive가 visible해야 하고,
  test가 invalid transition, workflow failure, cancellation, compensation, report output을 증명해야 한다고 요구한다.
- `github.com/bluetape4k/bluetape-go/state`는 explicit transition, context-aware guard,
  final state, allowed event, sentinel-compatible transition error를 가진 concurrency-safe finite state machine을 제공한다.
- existing workshop HTTP example은 `http.Handler`를 구현하는 focused `Server` type,
  `httptest` test, `http.Server` timeout field를 가진 `main.go`, bilingual README file을 사용한다.
- Gin official docs는 `gin.Default`, route registration, `ShouldBindJSON`, `c.JSON`,
  `ServeHTTP`를 통한 `httptest` compatibility를 보여준다.

## 목표

- runnable `examples/order-lifecycle-state-api` example을 추가한다.
- `state.NewMachine`, `Transition`, `State`, `AllowedEvents`, `CanTransition`을 보여준다.
- in-memory order lifecycle 하나를 read하고 advance하는 작은 Gin API를 노출한다.
- invalid transition, guard rejection, final state, cancellation, malformed input에 대한 stable error mapping을 보여준다.
- test와 race validation으로 concurrent transition safety를 증명한다.
- example은 scenario-shaped로 유지하고 이후 0.4.0 workflow example의 prerequisite로 적합해야 한다.

## 비목표

- persistence, message queue, Testcontainers, external service를 추가하지 않는다.
- generic order management framework를 구현하지 않는다.
- `workflow` 또는 `workreport`를 사용하지 않는다. 이는 #39, #40, #72의 범위다.
- issue #38과 public HTTP API example roadmap이 요구하는 Gin 외의 새 dependency를 추가하지 않는다.
- review가 specific readability need를 찾기 전까지 decorative diagram을 추가하지 않는다.

## 제안하는 Example Shape

Directory:

```text
examples/order-lifecycle-state-api/
  main.go
  README.md
  README.ko.md
  internal/orderstate/
    server.go
    server_test.go
```

Package name: `orderstate`.

package는 `http.Handler`를 구현하는 `Server`를 소유한다. 내부적으로 `state.Machine[OrderState, OrderEvent]` 하나와
state machine이 관리하지 않는 field를 위한 작은 mutex-protected order record를 소유한다.

## Domain Model

States:

- `draft`
- `submitted`
- `paid`
- `packed`
- `shipped`
- `cancelled`

Events:

- `submit`
- `pay`
- `pack`
- `ship`
- `cancel`

transition:

| From | Event | To | Notes |
| --- | --- | --- | --- |
| `draft` | `submit` | `submitted` | draft order를 submit한다. |
| `submitted` | `pay` | `paid` | positive total amount가 필요하다. |
| `paid` | `pack` | `packed` | fulfillment를 준비한다. |
| `packed` | `ship` | `shipped` | final success state다. |
| `draft` | `cancel` | `cancelled` | customer가 submit 전에 포기한다. |
| `submitted` | `cancel` | `cancelled` | payment 전에 cancel한다. |
| `paid` | `cancel` | `cancelled` | packing 전에 cancel한다. |

final state: `shipped`, `cancelled`.

`pay` guard는 non-positive total order를 reject한다. payment infrastructure를 도입하지 않고 guard behavior를 visible하게 만든다.

## HTTP API

모든 route에 Gin을 사용한다.

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/healthz` | service health를 반환한다. |
| `GET` | `/orders/current` | order ID, current state, allowed event, total을 반환한다. |
| `POST` | `/orders/current/transitions` | `{ "event": "..." }`를 bind하고 transition을 apply한 뒤 result와 new allowed event를 반환한다. |
| `GET` | `/orders/current/transitions/:event/can` | current state가 event로 transition할 수 있는지 반환한다. |

response convention:

- success: stable field name을 가진 JSON.
- invalid JSON 또는 unknown event: `400`.
- invalid transition/final state/guard rejection: `409`.
- context cancellation/deadline: `408 Request Timeout` 반환.
- unexpected error: `500`.

## Error Contract

server는 string matching 대신 `state` sentinel error에 대해 `errors.Is`를 사용해야 한다.

- `state.ErrInvalidTransition`
- `state.ErrFinalState`
- `state.ErrGuardRejected`
- `state.ErrConcurrentTransition`

concurrent transition conflict에서는 `409 Conflict` 반환을 허용한다.
client가 current state를 다시 읽고 desired event가 아직 allowed라면 retry할 수 있기 때문이다.

## Concurrency Contract

- `state.Machine`은 current state를 내부적으로 보호한다.
- example-owned order metadata는 race를 만들면 안 된다.
- concurrent transition test는 competing transition request를 만들고,
  같은 source state를 두고 race할 때 정확히 하나만 succeed하는지 assert해야 한다.
- PR 전에 `go test -race -count=1 ./examples/order-lifecycle-state-api/...`가 mandatory다.

## 설계 옵션

### Option A - Gin API가 있는 in-memory order 하나

single current order와 transition endpoint를 노출한다. 이렇게 하면 example이 state machine behavior에 집중한다.

장점:

- 가장 작은 runnable HTTP example이다.
- 이후 workflow example의 prerequisite로 이해하기 쉽다.
- invalid transition과 concurrent transition conflict를 test하기 쉽다.

비용:

- multi-order production API가 아니다.

### Option B - Multi-Order Store

order ID와 per-order machine을 가진 CRUD-like route를 노출한다.

장점:

- application shape가 더 realistic하다.

비용:

- `state` package에서 주의를 분산시키는 map locking, lifecycle ownership, API surface를 추가한다.

### Option C - Non-HTTP domain example

domain package와 test만 노출한다.

장점:

- 매우 작다.

비용:

- Gin HTTP example을 명시적으로 요구하는 issue #38을 만족하지 못한다.

## 결정

Option A를 채택한다.

첫 0.4.0 workshop example은 실행하기 쉽고 inspect하기 쉬우며 finite state machine behavior에 집중해야 한다.
multi-order storage와 workflow composition은 이후 example로 미룬다.

거절한 대안:

- Option B: persistence-like storage와 multi-order lifecycle management가 state machine을 application scaffolding 뒤에 숨긴다.
- Option C: issue #38이 Gin route를 요구한다.

## Test Strategy

focused package test:

- health endpoint는 OK를 반환한다.
- current-state endpoint는 initial state와 allowed event를 반환한다.
- allowed transition path는 `draft -> submitted -> paid`로 이동한다.
- invalid transition은 `409`를 반환하고 current state를 unchanged로 유지한다.
- guard rejection은 non-positive total에 `409`를 반환한다.
- final state는 추가 transition을 reject한다.
- malformed JSON과 unknown event는 `400`을 반환한다.
- concurrent duplicate transition request는 race-safe이고 valid current state를 남긴다.

validation command:

- `go test -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -race -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -count=1 ./...`
- `git diff --check`
- `make ci`

## Documentation 영향

- `examples/order-lifecycle-state-api/README.md`를 추가한다.
- `examples/order-lifecycle-state-api/README.ko.md`를 추가한다.
- 필요한 곳에서 root `README.md`와 `README.ko.md` example table 및 0.4.0 roadmap wording을 update한다.
- README는 workflow runner 없이 finite state machine만으로 충분한 시점을 설명해야 한다.
- README는 이것이 production persistence model이 아니라 in-memory workshop example이라고 명시해야 한다.

## Risk

1. **Gin dependency drift**: `go.mod`는 현재 Gin을 require하지 않는다.
   Gin 추가는 roadmap decision상 허용되지만 explicit해야 하며 `go mod tidy`와 review로 verify해야 한다.
2. **Framework mismatch**: root README는 현재 chi가 default라고 말한다. 이 PR은 newer roadmap을 반영하도록 guidance를 update해야 한다.
   framework-visible public HTTP API에는 Gin을, compatibility-focused example에는 net/http/chi를 사용한다.
3. **Weak concurrency test**: sequential request만 보내는 test는 concurrent request safety를 증명할 수 없다.
   actual goroutine과 race validation을 사용한다.
4. **Overgrown app shape**: persistence, user identity, order item, background workflow를 추가하면 #38에 비해 example이 너무 넓어진다.

## Acceptance Criteria

- example은 `go run ./examples/order-lifecycle-state-api`로 runnable하다.
- Gin route는 current state와 transition command를 노출한다.
- test는 allowed transition, invalid transition, guard rejection, final state behavior,
  concurrent request safety를 cover한다.
- `README.md`와 `README.ko.md`가 존재하고 synchronized 상태다.
- root `README.md`와 `README.ko.md`가 example을 link한다.
- unrelated file은 변경하지 않는다.
- Step 2-R, Step 3-R, Step 6-R은 `P0=0 P1=0`으로 close된다.
