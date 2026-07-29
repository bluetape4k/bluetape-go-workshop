# Issue #77 Code Review: Multi-Currency Invoice Rules

## 범위

`examples/multi-currency-invoice-rules`, root README update, lesson/spec/plan artifact,
`bluetape-go/money` usage를 branch diff 기준으로 검토했다.

## Six-Lane Findings

| Lane | Perspective | Reviewed Evidence | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | `Service.Evaluate`의 request-local map/slice만 사용하고 unbounded goroutine fan-out은 없다 (`service.go:131-195`). | 0 | 0 | 0 | 0 |
| 2 | Stability | main server는 process lifecycle goroutine 하나와 bounded shutdown을 소유한다 (`main.go:33-53`); race test passed. | 0 | 0 | 0 | 0 |
| 3 | Security | JSON body cap(`service.go:382`), trusted proxies disabled(`service.go:376`), loopback bind enforced(`main.go:68-90`). | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | `/healthz` exists(`service.go:378-380`), HTTP timeout은 bounded(`main.go:57-65`), smoke test passed. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | Money parsing/arithmetic은 `money.Money`에 남아 있다 (`service.go:211-229`, `service.go:280-317`); reusable rule framework는 추가하지 않았다. | 0 | 0 | 0 | 0 |
| 6 | User/Caller | example README는 #45 base lesson을 link하고 no conversion 및 invalid currency behavior를 문서화한다 (`README.md:7-21`, `README.md:87-115`). | 0 | 0 | 0 | 0 |

## Quick Scan Hits

| Hit | File:Line | Disposition |
|---|---|---|
| `context.Background` | `main.go:33`, `main.go:50` | intentional process-root signal context와 bounded shutdown context. |
| `go func` | `main.go:37` | intentional `ListenAndServe` lifecycle goroutine with buffered error channel and shutdown path. |
| `SetTrustedProxies` | `service.go:376` | sibling example과 맞춘 intentional Gin trust-boundary disable. |
| `MaxBytesReader` | `service.go:382` | intentional request body cap. |
| `float64` | root README #45 section only | money example이 `float64`를 피하는 이유를 설명하는 documentation이며, 새 code에는 `float64` arithmetic이 없다. |

## Baseline Finding Fixed

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| P2 | `service.go` router construction | Stability | initial draft는 `SetTrustedProxies(nil)`가 error를 반환하면 panic했다. | repository sibling-example pattern인 `_ = router.SetTrustedProxies(nil)`로 바꾸고 focused test와 race test를 다시 실행했다. |

## 검증 Evidence

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

## Gate 결과

Final Step 6-R convergence: `P0 = 0`, `P1 = 0`.
