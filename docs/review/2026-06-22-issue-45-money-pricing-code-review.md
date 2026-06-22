# Issue #45 Code Review

Scope: branch diff for `examples/money-rule-pricing`, root README updates, lesson/spec/plan artifacts, and `go.mod`/`go.sum` changes.

Native review subagents are unavailable in this session, so the Step 6-R six-lane review was run locally from the full-feature review prompts.

## Six-Lane Findings

| Tier | Perspective | Scope | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | Cart pricing loop, money formatting, HTTP JSON handling. | 0 | 0 | 0 | 0 |
| 2 | Stability | Discount ordering, mixed currency, invalid money, shutdown. | 0 | 1 | 0 | 0 |
| 3 | Security | Public HTTP errors, input validation, loopback default. | 0 | 0 | 0 | 0 |
| 4 | Operator | Health endpoint, deterministic local service, docs. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | Go package boundary, no reusable rule framework, tests. | 0 | 0 | 0 | 0 |
| 6 | User/Caller | English/Korean README, rejected-rule visibility. | 0 | 0 | 0 | 0 |

## Fixed Findings

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| P1 | `examples/money-rule-pricing/internal/moneypricing/service.go:248` | Stability | `SAVE10` was compared against subtotal instead of remaining subtotal after prior VIP discounts, allowing a negative final total for small VIP carts. | Added `TestServiceRejectsCouponWhenPriorDiscountConsumesRemainingSubtotal` and compare fixed coupon discount against `subtotal - discountTotal`. |

## Performance/Stability Scan

Concurrency quick scan:

```bash
rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" examples/money-rule-pricing
```

Reviewed hits:

- `main.go`: signal/shutdown contexts and one server goroutine match existing example lifecycle.
- `http_test.go`: `context.Background()` appears only in `httptest` requests.

No performance or stability issues found after the fixed P1.

## Verification Evidence

- `go test -count=1 ./examples/money-rule-pricing/...`
- `go test -race -count=1 ./examples/money-rule-pricing/...`

Final Step 6-R verdict after fix: P0 = 0, P1 = 0.
