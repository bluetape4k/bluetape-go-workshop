# Issue 14 Payment Authorization Guard Design

## 분류

- Work type: Type A - Full Feature.
- Basis: issue #14는 새 runnable example directory, focused domain code,
  test, English/Korean README, top-level README entry를 추가한다.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree:
  `.worktrees/issue-14-payment-authorization-guard`.

## 문제

`examples/resilience-http-web`는 이미 HTTP service에서 retry, timeout, circuit breaker,
bulkhead, event hook을 보여준다. issue #14는 v0.2.0 resilience expansion을 분리해 보여주는
compact non-HTTP payment authorization example을 요구한다.

- `resilience.NewCircuitBreaker`
- `resilience.NewBulkhead`
- synchronous `OnEvent` hooks
- typed rejection errors through `errors.Is`

example은 scenario-first 형태를 유지하고 workshop boundary를 넘는 reusable abstraction을 피해야 한다.

## 현재 근거

- GitHub issue #14는 `examples/payment-authorization-guard`, circuit-open rejection 및
  bulkhead overflow rejection focused test, event test, root README update, no new dependency를 요구한다.
- `go list -m`은 `github.com/bluetape4k/bluetape-go v0.3.0`으로 resolve하므로
  workshop은 current dependency를 통해 v0.2.0 resilience API를 사용할 수 있다.
- `resilience.CircuitBreakerOptions`는 `FailureThreshold`와 `OpenTimeout`을 요구한다.
  `SuccessThreshold`, `HalfOpenMaxConcurrent`, `FailureIf`, `Now`, `OnEvent`는 optional 또는 defaulted다.
- `resilience.BulkheadOptions`는 `MaxConcurrent`를 요구한다. `Wait=false`는
  `resilience.ErrBulkheadRejected`로 immediate rejection을 제공한다.
- `resilience.Event`는 stable `PolicyType`, `Kind`, `Category`, `State`,
  `PreviousState`, `InFlight`, low-cardinality error category field를 노출한다.
- existing workshop example은 domain code를 얇게 유지한다.
- `catalog-refresh-resilience`는 `internal/catalogrefresh` 아래 domain package 하나,
  localized README pair, focused test를 사용한다.
- `resilience-http-web`는 같은 policy를 HTTP context에서 보여준다.
- `product-enrichment-fanout`은 production infrastructure 대신 deterministic test synchronization으로 concurrency behavior를 증명한다.

## 제약

- 새 dependency를 추가하지 않는다.
- small struct, `context.Context`, typed error처럼 Go-shaped boundary를 사용하고 Kotlin-shaped helper surface를 만들지 않는다.
- gateway dependency는 example이 소유하는 narrow function 또는 interface로 유지한다.
- PAN, card token, customer identity, 그 밖의 payment secret은 model하지 않는다.
  request shape는 resilience behavior 설명에 필요한 non-sensitive order metadata로 제한한다.
- event capture는 synchronous하고 testable하게 유지한다. logging 또는 metrics package를 추가하지 않는다.
- example 및 root table의 `README.md`와 `README.ko.md`를 모두 update한다.
- existing diagram asset은 재사용할 수 있다. 새 README diagram asset을 추가한다면
  `bluetape4k-diagram` output, font, geometry, PNG preview, SVG/PNG pair rule을 적용한다.

## 설계 옵션

### Option A - gateway function을 사용하는 plain domain package

`internal/paymentguard`를 다음 항목으로 만든다.

- `Request`, `Authorization`, and `Gateway` function types.
- circuit breaker와 bulkhead를 소유하는 `Authorizer`.
- `Authorize(ctx, request, gateway)` that validates input and runs the gateway
  through `resilience.Run(ctx, operation, breaker, bulkhead)`.

장점:

- example surface가 가장 작다.
- gateway invocation count로 open-circuit pre-call rejection을 쉽게 증명할 수 있다.
- blocked operation 하나와 concurrent rejected operation 하나로 bulkhead overflow를 쉽게 강제할 수 있다.
- `catalog-refresh-resilience` style과 맞다.

비용:

- runnable HTTP `main.go`가 없다. 다만 issue #14는 non-HTTP domain isolation을 명시적으로 요구한다.

### Option B - embedded gateway를 가진 payment service struct

`Gateway`를 `Authorizer` option 안에 저장하고 `Authorize(ctx, request)`를 노출한다.

장점:

- production dependency injection에 조금 더 가깝다.

비용:

- test setup이 덜 explicit해지고 issue에 필요 없는 state가 추가된다.
- test별 gateway behavior 교체를 보여주기 어렵다.
- workshop goal에 필요한 것보다 abstraction이 많다.

### Option C - HTTP payment authorization service

`/authorize`, events endpoint, simulated payment gateway를 가진 chi service를 추가한다.

장점:

- `go run`으로 실행할 수 있다.

비용:

- `resilience-http-web`와 중복된다.
- issue #14가 명시적으로 피하려는 HTTP routing, request parsing, server timeout,
  handler error mapping concern을 다시 끌어온다.
- circuit breaker/bulkhead isolation과 무관한 documentation 및 test surface를 추가한다.

## 결정

Option A를 채택한다.

example은 `paymentguard`라는 plain domain package가 된다. payment scenario는 알아볼 수 있게 유지하면서
circuit breaker, bulkhead, event hook behavior만 움직이는 부분으로 둔다.

거절한 대안:

- Option B: gateway를 authorizer에 embed하면 unnecessary state가 추가되고 per-test scenario clarity가 약해진다.
- Option C: existing HTTP example과 중복되고 issue #14의 non-HTTP domain focus를 흐린다.

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

`New`는 workshop path를 위해 zero-value option을 지원한다.

- `FailureThreshold=2`
- `OpenTimeout=250ms`
- `MaxConcurrent=1`

negative threshold, negative timeout, negative concurrency value는 configuration error다.
nil `OnEvent`와 nil `Now`는 accept하고 underlying resilience default에 delegate한다.

`Authorize`는 policy execution 전에 authorizer, request, gateway를 validate한다.
policy order는 `breaker, bulkhead`이므로 open-circuit rejection은 bulkhead acquisition 및 gateway invocation보다 먼저 발생한다.
이는 open circuit이 gateway 호출 전에 reject해야 한다는 acceptance criterion을 직접 만족한다.

request validation은 의도적으로 작고 explicit하다.

- `MerchantID`는 non-empty여야 한다.
- `OrderID`는 non-empty여야 한다.
- `AmountCents`는 positive여야 한다.
- `gateway`는 non-nil이어야 한다.

## Failure Mode 및 Risk

1. **Policy order drift**: `bulkhead, breaker`를 사용해도 call은 reject될 수 있지만,
   open-circuit call이 먼저 bulkhead permit을 두고 contend할 수 있다. test는 open-circuit rejection이 gateway를 호출하지 않음을 증명해야 한다.
2. **Nondeterministic circuit timing**: wall-clock sleep에 의존하면 test가 flaky해진다.
   example은 test에서 `Now`를 설정하고 open-circuit assertion에 wait가 필요 없는 `OpenTimeout`을 사용해야 한다.
3. **Bulkhead overflow race**: 첫 call이 두 번째 call 진입 전에 끝나면 concurrent test가 우연히 pass할 수 있다.
   test는 두 번째 rejection이 관측될 때까지 첫 gateway call을 channel로 block해야 한다.
4. **Event assertion weakness**: 어떤 event가 있다는 것만 확인하면 잘못된 policy wiring을 놓칠 수 있다.
   test는 최소한 circuit transition, circuit rejection, bulkhead admission, bulkhead rejection event kind를 assert해야 한다.
5. **Example overgrowth**: HTTP, logging, metrics, reusable helper를 추가하면 issue scope가 흐려진다.
   code는 local하고 minimal하게 유지한다.

## Acceptance Criteria

- `go test -count=1 ./examples/payment-authorization-guard/...`가 pass한다.
- repeated gateway failure가 circuit을 open한다.
- open circuit은 gateway 호출 전에 reject하고 `resilience.ErrCircuitOpen`과 match하는 error를 반환한다.
- concurrent overflow는 `resilience.ErrBulkheadRejected`와 match하는 error를 반환한다.
- event test는 최소 `EventCircuitStateTransition`, `EventCircuitRejected`,
  `EventBulkheadAccepted`, `EventBulkheadRejected`를 check한다.
- `go.mod`에 새 dependency를 추가하지 않는다.
- root English/Korean README table은 새 example과 `English | 한국어` language link를 포함한다.
- example README pair는 scenario, policy order, event behavior, test command를 설명한다.

## Documentation 영향

- `examples/payment-authorization-guard/README.md`를 추가한다.
- `examples/payment-authorization-guard/README.ko.md`를 추가한다.
- root `README.md`와 `README.ko.md` example table을 update한다.
- implementation plan이 example README에 필요하다고 확인하면 payment-specific flow diagram을 추가한다.
  새 diagram을 추가하면 Graphviz evidence, final SVG/PNG asset, font check, rendered PNG inspection을 포함한
  전체 `bluetape4k-diagram` rule을 적용한다.

## DoD

- spec과 plan은 implementation 전에 commit되어 있다.
- implementation은 `$bluetape-go-patterns`를 사용한다.
- test는 success/failure/rejection/event behavior와 deterministic concurrency를 cover한다.
- `go test -count=1 ./examples/payment-authorization-guard/...`가 pass한다.
- example이 concurrency와 shared policy state를 다루므로
  `go test -race -count=1 ./examples/payment-authorization-guard/...`가 pass한다.
- `go test ./...`가 pass한다.
- `git diff --check`가 pass한다.
- PR creation 전에 Step 6-R local 7-tier review가 `P0=0 P1=0`을 report한다.
