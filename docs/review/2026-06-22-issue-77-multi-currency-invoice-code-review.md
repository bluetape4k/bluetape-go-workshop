# Issue #77 Code Review: Multi-Currency Invoice Rules

Scope: branch diff for `examples/multi-currency-invoice-rules`, root README
updates, lesson/spec/plan artifacts, and `bluetape-go/money` usage.

## Six-Lane Findings

| Lane | Perspective | Reviewed Evidence | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | One request-local map/slices only; no unbounded goroutine fan-out in `Service.Evaluate` (`service.go:131-195`). | 0 | 0 | 0 | 0 |
| 2 | Stability | Main server owns one process lifecycle goroutine and bounded shutdown (`main.go:33-53`); race test passed. | 0 | 0 | 0 | 0 |
| 3 | Security | JSON body capped (`service.go:382`), trusted proxies disabled (`service.go:376`), loopback bind enforced (`main.go:68-90`). | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | `/healthz` exists (`service.go:378-380`), HTTP timeouts are bounded (`main.go:57-65`), smoke test passed. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | Money parsing/arithmetic stays in `money.Money` (`service.go:211-229`, `service.go:280-317`); no reusable rule framework added. | 0 | 0 | 0 | 0 |
| 6 | User/Caller | Example README links #45 base lesson and documents no conversion plus invalid currency behavior (`README.md:7-21`, `README.md:87-115`). | 0 | 0 | 0 | 0 |

## Quick Scan Hits

| Hit | File:Line | Disposition |
|---|---|---|
| `context.Background` | `main.go:33`, `main.go:50` | Intentional process-root signal context and bounded shutdown context. |
| `go func` | `main.go:37` | Intentional `ListenAndServe` lifecycle goroutine with buffered error channel and shutdown path. |
| `SetTrustedProxies` | `service.go:376` | Intentional Gin trust-boundary disable, matching sibling examples. |
| `MaxBytesReader` | `service.go:382` | Intentional request body cap. |
| `float64` | Root README #45 section only | Documentation explaining why money examples avoid `float64`; new code has no `float64` arithmetic. |

## Baseline Finding Fixed

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| P2 | `service.go` router construction | Stability | Initial draft panicked if `SetTrustedProxies(nil)` returned an error. | Replaced with the repository's sibling-example pattern `_ = router.SetTrustedProxies(nil)` and reran focused tests plus race test. |

## Verification Evidence

```bash
go test -count=1 ./examples/multi-currency-invoice-rules/...
go test -race -count=1 ./examples/multi-currency-invoice-rules/...
go test -p 1 ./...
make fmt-check
make tidy-check
make vet
make lint
GOFLAGS=-p=1 make ci
git diff --check
```

Live smoke:

- `GET /healthz` returned `{"status":"ok"}`.
- `POST /invoices/evaluate` returned grouped `EUR`, `JPY`, and `USD` totals
  with `conversion_applied:false`.

## Gate

Final Step 6-R convergence: `P0 = 0`, `P1 = 0`.
