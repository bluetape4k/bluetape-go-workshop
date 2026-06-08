# Issue 38 Order Lifecycle State API Design

## Classification

- Work type: Type A - Full Feature.
- Basis: issue #38 adds a new runnable Gin example directory, Go code, tests,
  English/Korean README files, root README entries, review artifacts, lessons,
  and a PR.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `feat/issue-38-order-lifecycle-state-api` under
  `.worktrees/feat-issue-38-order-lifecycle-state-api`.

## Problem

The workshop needs a 0.4.0 example that shows how the `state` package behaves
inside a realistic HTTP boundary. The example should not be a state-machine API
catalog page. It should be a small order lifecycle service where readers can
inspect current state, advance state with commands, and see invalid transitions
reported clearly.

## Current Evidence

- GitHub issue #38 requires a Gin HTTP example around released state machine
  primitives.
- Parent issue #28 requires state/workflow primitives to stay visible and tests
  to prove invalid transitions, workflow failures, cancellation,
  compensation, and report output across the milestone.
- `github.com/bluetape4k/bluetape-go/state` provides a concurrency-safe finite
  state machine with explicit transitions, context-aware guards, final states,
  allowed events, and sentinel-compatible transition errors.
- Existing workshop HTTP examples use a focused `Server` type implementing
  `http.Handler`, `httptest` tests, `main.go` with `http.Server` timeout
  fields, and bilingual README files.
- Gin official docs show `gin.Default`, route registration, `ShouldBindJSON`,
  `c.JSON`, and `httptest` compatibility through `ServeHTTP`.

## Goals

- Add a runnable `examples/order-lifecycle-state-api` example.
- Demonstrate `state.NewMachine`, `Transition`, `State`, `AllowedEvents`, and
  `CanTransition`.
- Expose a small Gin API for reading and advancing one in-memory order
  lifecycle.
- Show stable error mapping for invalid transition, guard rejection, final
  state, cancellation, and malformed input.
- Prove concurrent transition safety with tests and race validation.
- Keep the example scenario-shaped and suitable as a prerequisite for the later
  0.4.0 workflow examples.

## Non-Goals

- Do not add persistence, message queues, Testcontainers, or external services.
- Do not implement a generic order management framework.
- Do not use `workflow` or `workreport`; those belong to #39, #40, and #72.
- Do not add new dependencies beyond Gin, which is required by issue #38 and
  the roadmap for public HTTP API examples.
- Do not add decorative diagrams unless review finds a specific readability
  need.

## Proposed Example Shape

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

The package owns a `Server` that implements `http.Handler`. Internally it owns
one `state.Machine[OrderState, OrderEvent]` plus a small mutex-protected order
record for fields that are not managed by the state machine.

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

Transitions:

| From | Event | To | Notes |
| --- | --- | --- | --- |
| `draft` | `submit` | `submitted` | Submit a draft order. |
| `submitted` | `pay` | `paid` | Requires a positive total amount. |
| `paid` | `pack` | `packed` | Prepare fulfillment. |
| `packed` | `ship` | `shipped` | Final success state. |
| `draft` | `cancel` | `cancelled` | Customer abandons before submit. |
| `submitted` | `cancel` | `cancelled` | Cancel before payment. |
| `paid` | `cancel` | `cancelled` | Cancel before packing. |

Final states: `shipped`, `cancelled`.

The `pay` guard rejects orders with non-positive totals. This makes guard
behavior visible without introducing payment infrastructure.

## HTTP API

Use Gin for all routes:

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/healthz` | Return service health. |
| `GET` | `/orders/current` | Return order ID, current state, allowed events, and total. |
| `POST` | `/orders/current/transitions` | Bind `{ "event": "..." }`, apply transition, return result and new allowed events. |
| `GET` | `/orders/current/transitions/:event/can` | Return whether the current state can transition with the event. |

Response conventions:

- Success: JSON with stable field names.
- Invalid JSON or unknown event: `400`.
- Invalid transition/final state/guard rejection: `409`.
- Context cancellation/deadline: return `408 Request Timeout`.
- Unexpected error: `500`.

## Error Contract

The server must use `errors.Is` against `state` sentinel errors instead of
string matching:

- `state.ErrInvalidTransition`
- `state.ErrFinalState`
- `state.ErrGuardRejected`
- `state.ErrConcurrentTransition`

For concurrent transition conflicts, returning `409 Conflict` is acceptable
because the client can reread current state and retry the desired event if it
is still allowed.

## Concurrency Contract

- `state.Machine` protects the current state internally.
- Example-owned order metadata must not introduce races.
- Concurrent transition tests must create competing transition requests and
  assert exactly one succeeds when both race for the same source state.
- `go test -race -count=1 ./examples/order-lifecycle-state-api/...` is
  mandatory before PR.

## Design Options

### Option A - One In-Memory Order With Gin API

Expose a single current order and transition endpoint. This keeps the example
focused on state machine behavior.

Benefits:

- Smallest runnable HTTP example.
- Easy to understand as a prerequisite for later workflow examples.
- Easy to test invalid transitions and concurrent transition conflicts.

Costs:

- Not a multi-order production API.

### Option B - Multi-Order Store

Expose CRUD-like routes with order IDs and per-order machines.

Benefits:

- More realistic application shape.

Costs:

- Adds map locking, lifecycle ownership, and API surface that distract from the
  `state` package.

### Option C - Non-HTTP Domain Example

Only expose a domain package and tests.

Benefits:

- Very small.

Costs:

- Fails issue #38, which explicitly asks for a Gin HTTP example.

## Decision

Adopt Option A.

The first 0.4.0 workshop example should be easy to run, easy to inspect, and
centered on finite state machine behavior. Multi-order storage and workflow
composition are deferred to later examples.

Rejected:

- Option B because persistence-like storage and multi-order lifecycle
  management would hide the state machine behind application scaffolding.
- Option C because issue #38 requires Gin routes.

## Test Strategy

Focused package tests:

- health endpoint returns OK.
- current-state endpoint returns initial state and allowed events.
- allowed transition path moves `draft -> submitted -> paid`.
- invalid transition returns `409` and keeps current state unchanged.
- guard rejection returns `409` for non-positive total.
- final state rejects further transitions.
- malformed JSON and unknown event return `400`.
- concurrent duplicate transition requests are race-safe and leave a valid
  current state.

Validation commands:

- `go test -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -race -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -count=1 ./...`
- `git diff --check`
- `make ci`

## Documentation Impact

- Add `examples/order-lifecycle-state-api/README.md`.
- Add `examples/order-lifecycle-state-api/README.ko.md`.
- Update root `README.md` and `README.ko.md` example tables and 0.4.0 roadmap
  wording where needed.
- README must explain when a finite state machine is enough without a workflow
  runner.
- README must state this is an in-memory workshop example, not a production
  persistence model.

## Risks

1. **Gin dependency drift**: `go.mod` does not currently require Gin. Adding
   Gin is allowed by the roadmap decision, but it must be explicit and verified
   through `go mod tidy` and review.
2. **Framework mismatch**: root README currently says chi is the default. This
   PR must update that guidance to reflect the newer roadmap: Gin for
   framework-visible public HTTP APIs, net/http/chi for compatibility-focused
   examples.
3. **Weak concurrency test**: a test that only sends sequential requests cannot
   prove concurrent request safety. Use actual goroutines and race validation.
4. **Overgrown app shape**: adding persistence, user identity, order items, or
   background workflows would make the example too broad for #38.

## Acceptance Criteria

- The example is runnable with `go run ./examples/order-lifecycle-state-api`.
- Gin routes expose current state and transition commands.
- Tests cover allowed transitions, invalid transitions, guard rejection, final
  state behavior, and concurrent request safety.
- `README.md` and `README.ko.md` exist and are synchronized.
- Root `README.md` and `README.ko.md` link the example.
- No unrelated files are changed.
- Step 2-R, Step 3-R, and Step 6-R close with `P0=0 P1=0`.
