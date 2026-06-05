# Issue 14 Payment Authorization Guard Design

## Classification

- Work type: Type A - Full Feature.
- Basis: issue #14 adds a new runnable example directory, focused domain code,
  tests, English/Korean READMEs, and top-level README entries.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `issue-14-payment-authorization-guard` under
  `.worktrees/issue-14-payment-authorization-guard`.

## Problem

`examples/resilience-http-web` already demonstrates retry, timeout, circuit
breaker, bulkhead, and event hooks in an HTTP service. Issue #14 asks for a
compact non-HTTP payment authorization example that isolates the v0.2.0
resilience expansion:

- `resilience.NewCircuitBreaker`
- `resilience.NewBulkhead`
- synchronous `OnEvent` hooks
- typed rejection errors through `errors.Is`

The example must remain scenario-first and avoid reusable abstractions beyond
the workshop boundary.

## Current Evidence

- GitHub issue #14 requires `examples/payment-authorization-guard`, focused
  tests for circuit-open rejection and bulkhead overflow rejection, event tests,
  root README updates, and no new dependencies.
- `go list -m` resolves `github.com/bluetape4k/bluetape-go v0.3.0`, so the
  workshop can use the v0.2.0 resilience APIs through the current dependency.
- `resilience.CircuitBreakerOptions` requires `FailureThreshold` and
  `OpenTimeout`; `SuccessThreshold`, `HalfOpenMaxConcurrent`, `FailureIf`,
  `Now`, and `OnEvent` are optional or defaulted.
- `resilience.BulkheadOptions` requires `MaxConcurrent`; `Wait=false` gives
  immediate rejection with `resilience.ErrBulkheadRejected`.
- `resilience.Event` exposes stable `PolicyType`, `Kind`, `Category`, `State`,
  `PreviousState`, `InFlight`, and low-cardinality error category fields.
- Existing workshop examples keep domain code thin:
  - `catalog-refresh-resilience` uses one domain package under
    `internal/catalogrefresh`, localized README pairs, and focused tests.
  - `resilience-http-web` shows the same policies in an HTTP context.
  - `product-enrichment-fanout` proves concurrency behavior with deterministic
    test synchronization rather than production infrastructure.

## Constraints

- No new dependencies.
- Use Go-shaped boundaries: small structs, `context.Context`, typed errors, and
  no Kotlin-shaped helper surface.
- Keep the gateway dependency as a narrow function or interface owned by the
  example.
- Do not model PAN, card token, customer identity, or other payment secrets.
  The request shape is limited to non-sensitive order metadata needed to explain
  resilience behavior.
- Keep event capture synchronous and testable; do not add logging or metrics
  packages.
- Update both `README.md` and `README.ko.md` for the example and root table.
- Existing diagram assets may be reused; if new README diagram assets are added,
  `bluetape4k-diagram` output, font, geometry, PNG preview, and SVG/PNG pair
  rules apply.

## Design Options

### Option A - Plain Domain Package With Gateway Function

Create `internal/paymentguard` with:

- `Request`, `Authorization`, and `Gateway` function types.
- `Authorizer` that owns a circuit breaker and bulkhead.
- `Authorize(ctx, request, gateway)` that validates input and runs the gateway
  through `resilience.Run(ctx, operation, breaker, bulkhead)`.

Benefits:

- Smallest example surface.
- Easy to prove open-circuit pre-call rejection by counting gateway invocations.
- Easy to force bulkhead overflow with one blocked operation and one concurrent
  rejected operation.
- Matches `catalog-refresh-resilience` style.

Costs:

- No runnable HTTP `main.go`, but issue #14 explicitly asks for non-HTTP domain
  isolation.

### Option B - Payment Service Struct With Embedded Gateway

Store `Gateway` inside `Authorizer` options and expose
`Authorize(ctx, request)`.

Benefits:

- Slightly closer to production dependency injection.

Costs:

- Makes test setup less explicit and adds state that the issue does not need.
- Harder to demonstrate swapping gateway behavior per test.
- More abstraction than the workshop goal requires.

### Option C - HTTP Payment Authorization Service

Add a chi service with `/authorize`, events endpoint, and simulated payment
gateway.

Benefits:

- Runnable with `go run`.

Costs:

- Duplicates `resilience-http-web`.
- Reintroduces HTTP routing, request parsing, server timeout, and handler error
  mapping concerns that issue #14 explicitly wants to avoid.
- Adds documentation and test surface unrelated to circuit breaker/bulkhead
  isolation.

## Decision

Adopt Option A.

The example will be a plain domain package named `paymentguard`. It keeps the
payment scenario recognizable while making circuit breaker, bulkhead, and event
hook behavior the only moving parts.

Rejected:

- Option B because embedding the gateway in the authorizer adds unnecessary
  state and weakens per-test scenario clarity.
- Option C because it duplicates the existing HTTP example and distracts from
  the non-HTTP domain focus in issue #14.

## Proposed API

```go
type Request struct {
    MerchantID string
    OrderID string
    AmountCents int
}

type Authorization struct {
    OrderID string
    Approved bool
    ProviderRef string
}

type Gateway func(context.Context, Request) (Authorization, error)

type Options struct {
    FailureThreshold int
    OpenTimeout time.Duration
    MaxConcurrent int
    OnEvent resilience.EventHandler
    Now func() time.Time
}

type Authorizer struct {
    breaker *resilience.CircuitBreakerPolicy[Authorization]
    bulkhead *resilience.BulkheadPolicy[Authorization]
}

func New(options Options) (*Authorizer, error)
func (a *Authorizer) Authorize(ctx context.Context, request Request, gateway Gateway) (Authorization, error)
```

`New` will support zero-value options for the workshop path:

- `FailureThreshold=2`
- `OpenTimeout=250ms`
- `MaxConcurrent=1`

Negative thresholds, negative timeout, and negative concurrency values are
configuration errors. A nil `OnEvent` and nil `Now` are accepted and delegated to
the underlying resilience defaults.

`Authorize` will validate the authorizer, request, and gateway before policy
execution. The policy order will be `breaker, bulkhead`, so open-circuit
rejection happens before bulkhead acquisition and before gateway invocation.
This directly satisfies the acceptance criterion that an open circuit rejects
before calling the gateway.

Request validation is intentionally small and explicit:

- `MerchantID` must be non-empty.
- `OrderID` must be non-empty.
- `AmountCents` must be positive.
- `gateway` must be non-nil.

## Failure Modes And Risks

1. **Policy order drift**: using `bulkhead, breaker` would still reject calls,
   but open-circuit calls could contend for bulkhead permits first. Tests must
   prove open-circuit rejection does not call the gateway.
2. **Nondeterministic circuit timing**: relying on wall-clock sleep makes tests
   flaky. The example should set `Now` in tests and use an `OpenTimeout` that
   does not require waiting for open-circuit assertions.
3. **Bulkhead overflow race**: concurrent tests can pass accidentally if the
   first call exits before the second tries to enter. Tests must use channels
   to block the first gateway call until the second rejection is observed.
4. **Event assertion weakness**: only checking that some event exists can miss
   wrong policy wiring. Tests must assert at least circuit transition,
   circuit rejection, bulkhead admission, and bulkhead rejection event kinds.
5. **Example overgrowth**: adding HTTP, logging, metrics, or reusable helpers
   would obscure the issue scope. Keep code local and minimal.

## Acceptance Criteria

- `go test -count=1 ./examples/payment-authorization-guard/...` passes.
- Repeated gateway failures open the circuit.
- Open circuit rejects before calling the gateway and returns an error matching
  `resilience.ErrCircuitOpen`.
- Concurrent overflow returns an error matching
  `resilience.ErrBulkheadRejected`.
- Event tests check at least `EventCircuitStateTransition`,
  `EventCircuitRejected`, `EventBulkheadAccepted`, and
  `EventBulkheadRejected`.
- No new dependency is added to `go.mod`.
- Root English/Korean README tables include the new example with
  `English | 한국어` language links.
- Example README pair explains the scenario, policy order, event behavior, and
  test command.

## Documentation Impact

- Add `examples/payment-authorization-guard/README.md`.
- Add `examples/payment-authorization-guard/README.ko.md`.
- Update root `README.md` and `README.ko.md` example tables.
- Add a payment-specific flow diagram if the implementation plan confirms it is
  needed for the example README. If a new diagram is added, apply the full
  `bluetape4k-diagram` rules, including Graphviz evidence, final SVG/PNG
  assets, font checks, and rendered PNG inspection.

## DoD

- Spec and plan are committed before implementation.
- Implementation uses `$bluetape-go-patterns`.
- Tests cover success/failure/rejection/event behavior and deterministic
  concurrency.
- `go test -count=1 ./examples/payment-authorization-guard/...` passes.
- `go test -race -count=1 ./examples/payment-authorization-guard/...` passes
  because the example touches concurrency and shared policy state.
- `go test ./...` passes.
- `git diff --check` passes.
- Step 6-R local 7-tier review reports `P0=0 P1=0` before PR creation.
