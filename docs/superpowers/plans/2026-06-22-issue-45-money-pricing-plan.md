# Issue #45 Plan: Money and Rule Based Pricing Example

Spec: `docs/superpowers/specs/2026-06-22-issue-45-money-pricing-design.md`

## Plan

1. Add failing tests for `examples/money-rule-pricing/internal/moneypricing`:
   - rounded line totals and discount totals,
   - mixed item/cart currency rejection,
   - accepted VIP and `SAVE10` discounts,
   - unsupported coupon rejection,
   - fixed discount exceeding subtotal rejection.
2. Add failing HTTP tests for `POST /quotes` and public error mapping.
3. Implement `moneypricing` domain types, quote service, money DTO helpers, and deterministic rule decisions.
4. Implement Gin router with `/healthz` and `/quotes`, JSON body cap, public error responses, and loopback-only `main.go` binding.
5. Add English and Korean example READMEs with run commands, expected behavior, and a clear no-`float64` money note.
6. Update root English and Korean READMEs with the new example entry.
7. Add `docs/lessons/2026-06-22-money-rule-pricing.md`.
8. Run `go mod tidy` to add required `bluetape-go/money` transitive sums.
9. Verify with:
   - `gofmt` on changed Go files,
   - `go test -count=1 ./examples/money-rule-pricing/...`,
   - `go test -race -count=1 ./examples/money-rule-pricing/...`,
   - `go test -p 1 ./...`,
   - `make fmt-check`,
   - `make tidy-check`,
   - `make vet`,
   - `make lint`,
   - `git diff --check`.
10. Run code review, record findings under `docs/review`, fix P0/P1 issues, then commit, push, and open a PR linked to #45 with the milestone and labels inherited from the issue.

## Task Mapping

| Spec requirement | Plan task |
|---|---|
| Runnable Gin API | 2, 4, 5 |
| Use `bluetape-go/money` | 1, 3, 8 |
| No `float64` money arithmetic | 1, 3, 5 |
| Currency mismatch rejection | 1, 2, 3, 4 |
| Rounded line and discount totals | 1, 3 |
| Valid discount rules | 1, 3 |
| Rejected rule paths | 1, 2, 3 |
| English/Korean docs | 5, 6, 7 |
| Verification gates | 9, 10 |

## Step 3-R Integrated Review

Native subagent spawning is not available in this Codex surface, so the six review lanes were run as independent main-session checks using the full-feature reference contract.

| Priority | Area | Finding | Required plan edit |
|---|---|---|---|
| P2 | Stability | HTTP and service tests both need to prove rejected rules are non-fatal while invalid money/currency errors are fatal. | Add separate service and HTTP test tasks. Done. |
| P2 | Developer | Dependency sums for `govalues` may be added when importing `bluetape-go/money`. | Add `go mod tidy` task. Done. |
| P3 | Operator | The example has no external resources, so race and package tests are enough for runtime stability. | Keep verification targeted plus repo gates. Done. |

Final Step 3-R verdict: P0 = 0, P1 = 0.
