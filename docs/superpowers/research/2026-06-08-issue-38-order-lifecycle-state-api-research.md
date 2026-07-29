# Issue 38 Order Lifecycle State API 리서치

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #38, `[v0.4.0] Add Gin order lifecycle state API example`
- 마일스톤: 0.4.0
- 상위 이슈: #28, state machine and workflow workshop examples

## 현재 저장소 근거

- 기존 예제는 `examples/<scenario>/internal/<package>` 구조에 집중된 패키지,
  패키지 옆 테스트, English/Korean README 파일을 둔다.
- `leader-redis-web`, `leader-group-web`, `resilience-http-web` 같은 기존 HTTP
  예제는 `http.Handler`를 구현하는 `Server`를 노출한다.
- 루트 README는 현재 chi를 기본 lightweight 스타일로 설명하지만, #38과 갱신된
  roadmap은 public HTTP API 예제에 Gin을 명시적으로 선택한다. 이 이슈는 워크숍의
  첫 Gin 예제 중 하나가 되어야 한다.
- `Makefile` gate는 `fmt`, `fmt-check`, `tidy-check`, `vet`, `lint`, `test`,
  `race`, `ci`다.
- 현재 의존성은 `github.com/bluetape4k/bluetape-go v0.5.1`이며, 여기에는 0.4.0
  패키지인 `state`, `workflow`, `workreport`가 포함된다.

## bluetape-go State API 근거

`github.com/bluetape4k/bluetape-go/state` 패키지는 다음을 제공한다.

- `NewMachine[S, E](initial, transitions, options...)`
- `state.Result`를 반환하는 `Transition(ctx, event)`
- `State()`
- `CanTransition(ctx, event)`
- `AllowedEvents()`
- `WithFinalStates`
- sentinel-compatible error:
  - `ErrInvalidTransition`
  - `ErrGuardRejected`
  - `ErrFinalState`
  - `ErrConcurrentTransition`
  - `ErrDuplicateTransition`
  - `ErrUnknownInitialState`

이 패키지는 concurrency-safe하다. `Transition`은 guard를 평가하고 lock 아래에서
현재 상태를 다시 확인하며, lookup과 mutation 사이에 다른 호출자가 상태를 옮기면
`ErrConcurrentTransition`을 반환할 수 있다.

## upstream 마일스톤 근거

GNO 결과는 upstream 0.4.0 research와 epic을 가리킨다.

- `bluetape-go/docs/research/2026-06-01-milestone-0.4.0-state-workflow-research.md`
- `bluetape-go/docs/superpowers/research/2026-06-05-issue-135-0.4.0-state-workflow-inventory.md`
- `bluetape-go` issue #4, `[Epic] 0.4.0 State machine and workflow primitives`
- `bluetape-go` issue #26 and PR #139 for finite state machine primitives

upstream 결정은 이 계층을 lightweight하고 Go-first로 유지하는 것이다. 즉 typed
state/event, context-aware guard, final state, transition result, 결정적인
sentinel-compatible error를 제공한다. durable orchestration과 대형 workflow engine은
0.4.0 범위 밖이다.

## Gin 근거

Context7은 공식 Gin 문서를 `/gin-gonic/gin`으로 resolve했다. 현재 문서는 다음을
보여 준다.

- `gin.Default()`는 기본 logger/recovery middleware가 포함된 router를 만든다.
- `GET`과 `POST` handler는 engine/router에 등록한다.
- `ShouldBindJSON`으로 handler가 JSON을 bind하고 명시적 error를 반환할 수 있다.
- `c.JSON(status, value)`가 일반적인 JSON 응답 경로다.
- Gin router는 `ServeHTTP`를 통해 `net/http/httptest`와 함께 동작한다.

## 채택 결정

| 후보 | 결정 | 근거 |
| --- | --- | --- |
| `state` package | 채택 | 이 이슈는 release된 finite state machine primitive를 보여 주기 위한 것이다. |
| `workflow` package | 제외 | #38은 finite-state-machine 전용이며, #39와 #72가 workflow composition을 다룬다. |
| `workreport` package | 제외 | work report/failure policy는 #40과 #72의 범위다. |
| Gin | 채택 | #38과 roadmap이 public HTTP API 예제에 Gin을 요구한다. |
| Testcontainers | 제외 | in-memory order lifecycle에는 외부 서비스가 필요하지 않다. |
| New dependencies | 제한 | Gin은 아직 `go.mod`에 없다. 이 이슈에서는 Gin만 추가하고 관련 없는 의존성은 거부한다. |

## 구현 제약

- public API surface는 production order management용 재사용 패키지가 아니라 예제
  서버다.
- 상태 전이는 드러나 있어야 한다. `state.Machine`을 큰 application framework 뒤에
  숨기지 않는다.
- transition과 guard check에는 request의 `context.Context`를 사용한다.
- invalid transition과 guard rejection을 안정적인 HTTP 응답에 매핑한다.
- `go test -race`와 집중된 concurrency test로 concurrent request 안전성을 증명한다.
- README.md와 README.ko.md를 동기화한다.

## 리서치 결론

`examples/order-lifecycle-state-api` 아래에 compact Gin service를 만든다. 이
서비스는 in-memory order lifecycle machine을 보유하고, 현재 상태와 transition
command를 노출하며, 다음을 보여 주어야 한다.

- 허용된 transition,
- invalid transition error,
- guard-rejected transition,
- idempotent read,
- final-state 동작,
- 동시성 안전성.
