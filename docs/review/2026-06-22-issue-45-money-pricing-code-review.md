# Issue #45 Code Review

## 범위

`examples/money-rule-pricing`, root README update, lesson/spec/plan artifact,
`go.mod`/`go.sum` 변경을 branch diff 기준으로 검토했다.

native review subagent는 이 session에서 사용하지 않았고, Step 6-R six-lane review는
full-feature review prompt를 기준으로 local에서 수행했다.

## Six-Lane Findings

| Tier | Perspective | Scope | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | cart pricing loop, money formatting, HTTP JSON handling. | 0 | 0 | 0 | 0 |
| 2 | Stability | discount ordering, mixed currency, invalid money, shutdown. | 0 | 1 | 0 | 0 |
| 3 | Security | public HTTP errors, input validation, loopback default. | 0 | 0 | 0 | 0 |
| 4 | Operator | health endpoint, deterministic local service, docs. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | Go package boundary, no reusable rule framework, tests. | 0 | 0 | 0 | 0 |
| 6 | User/Caller | English/Korean README, rejected-rule visibility. | 0 | 0 | 0 | 0 |

## Fixed Findings

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| P1 | `examples/money-rule-pricing/internal/moneypricing/service.go:248` | Stability | `SAVE10`이 prior VIP discount 이후 remaining subtotal이 아니라 subtotal과 비교되어 small VIP cart에서 final total이 음수가 될 수 있었다. | `TestServiceRejectsCouponWhenPriorDiscountConsumesRemainingSubtotal`를 추가하고 fixed coupon discount를 `subtotal - discountTotal`과 비교하도록 수정했다. |

## Performance/Stability Scan

Concurrency quick scan:

```bash
rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" examples/money-rule-pricing
```

검토한 hit:

- `main.go`: signal/shutdown context와 server goroutine 하나는 기존 example lifecycle과 일치한다.
- `http_test.go`: `context.Background()`는 `httptest` request에만 등장한다.

fixed P1 이후 performance 또는 stability issue는 발견되지 않았다.

## 검증 Evidence

- `go test -count=1 ./examples/money-rule-pricing/...`
- `go test -race -count=1 ./examples/money-rule-pricing/...`

fix 이후 final Step 6-R verdict: P0 = 0, P1 = 0.
