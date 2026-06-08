# Issue 38 Order Lifecycle State API Research

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #38, `[v0.4.0] Add Gin order lifecycle state API example`
- Milestone: 0.4.0
- Parent: #28, state machine and workflow workshop examples

## Current Repository Evidence

- Existing examples use `examples/<scenario>/internal/<package>` with a
  focused package, tests beside the package, and English/Korean README files.
- Existing HTTP examples such as `leader-redis-web`, `leader-group-web`, and
  `resilience-http-web` expose a `Server` that implements `http.Handler`.
- Root README currently says chi is the default lightweight style, but issue
  #38 and the updated roadmap explicitly choose Gin for public HTTP API
  examples. This issue should be one of the first Gin examples in the workshop.
- `Makefile` gates are `fmt`, `fmt-check`, `tidy-check`, `vet`, `lint`,
  `test`, `race`, and `ci`.
- Current dependency is `github.com/bluetape4k/bluetape-go v0.5.1`, which
  includes the 0.4.0 packages `state`, `workflow`, and `workreport`.

## bluetape-go State API Evidence

Package `github.com/bluetape4k/bluetape-go/state` provides:

- `NewMachine[S, E](initial, transitions, options...)`
- `Transition(ctx, event)` returning `state.Result`
- `State()`
- `CanTransition(ctx, event)`
- `AllowedEvents()`
- `WithFinalStates`
- sentinel-compatible errors:
  - `ErrInvalidTransition`
  - `ErrGuardRejected`
  - `ErrFinalState`
  - `ErrConcurrentTransition`
  - `ErrDuplicateTransition`
  - `ErrUnknownInitialState`

The package is concurrency-safe. `Transition` evaluates guards, rechecks the
current state under lock, and can return `ErrConcurrentTransition` if another
caller moved the state between lookup and mutation.

## Upstream Milestone Evidence

GNO results point to the upstream 0.4.0 research and epic:

- `bluetape-go/docs/research/2026-06-01-milestone-0.4.0-state-workflow-research.md`
- `bluetape-go/docs/superpowers/research/2026-06-05-issue-135-0.4.0-state-workflow-inventory.md`
- `bluetape-go` issue #4, `[Epic] 0.4.0 State machine and workflow primitives`
- `bluetape-go` issue #26 and PR #139 for finite state machine primitives

The upstream decision is to keep this layer lightweight and Go-first:
typed states/events, context-aware guards, final states, transition results,
and deterministic sentinel-compatible errors. Durable orchestration and large
workflow engines are out of scope for 0.4.0.

## Gin Evidence

Context7 resolved the official Gin documentation to `/gin-gonic/gin`.
The current docs show:

- `gin.Default()` creates a router with default logger/recovery middleware.
- `GET` and `POST` handlers are registered on the engine/router.
- `ShouldBindJSON` lets handlers bind JSON and return explicit errors.
- `c.JSON(status, value)` is the normal JSON response path.
- Gin routers work with `net/http/httptest` through `ServeHTTP`.

## Adoption Decisions

| Candidate | Decision | Rationale |
| --- | --- | --- |
| `state` package | Adopt | This issue exists to demonstrate released finite state machine primitives. |
| `workflow` package | Skip | Issue #38 is finite-state-machine only; #39 and #72 cover workflow composition. |
| `workreport` package | Skip | Work report/failure policy belongs to #40 and #72. |
| Gin | Adopt | Issue #38 and roadmap require Gin for public HTTP API examples. |
| Testcontainers | Skip | No external service is needed for an in-memory order lifecycle. |
| New dependencies | Constrain | Gin is not in `go.mod` yet. Add only Gin for this issue and reject unrelated dependencies. |

## Implementation Constraints

- The public API surface is an example server, not a reusable package for
  production order management.
- State transitions must stay visible; do not hide `state.Machine` behind a
  large application framework.
- Use `context.Context` from the request for transitions and guard checks.
- Map invalid transitions and guard rejections to stable HTTP responses.
- Prove concurrent request safety with `go test -race` and a focused
  concurrency test.
- Keep README.md and README.ko.md synchronized.

## Research Conclusion

Build a compact Gin service under `examples/order-lifecycle-state-api`.
The service should hold an in-memory order lifecycle machine, expose current
state and transition commands, and demonstrate:

- allowed transitions,
- invalid transition errors,
- guard-rejected transitions,
- idempotent reads,
- final-state behavior, and
- concurrency safety.
