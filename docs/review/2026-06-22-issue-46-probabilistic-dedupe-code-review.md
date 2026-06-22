# Issue #46 Code Review

Scope: branch diff for `examples/probabilistic-dedupe-admission`, root README updates, lesson/spec/plan artifacts, and probabilistic package usage.

Native review subagents are unavailable in this session, so the Step 6-R six-lane review was run locally from the full-feature review prompts.

## Six-Lane Findings

| Tier | Perspective | Scope | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | Bloom filter admission hot path and stats projection. | 0 | 0 | 0 | 0 |
| 2 | Stability | Shared filter state, concurrent duplicate admission, race gate. | 0 | 1 | 0 | 0 |
| 3 | Security | Public HTTP errors, input handling, no secrets. | 0 | 0 | 0 | 0 |
| 4 | Operator | Loopback-only demo service, health endpoint, durable-store caveat. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | Go API shape, error contracts, local example boundary. | 0 | 0 | 0 | 0 |
| 6 | User/Caller | README clarity, false-positive wording, root navigation. | 0 | 0 | 0 | 0 |

## Fixed Findings

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| P1 | `examples/probabilistic-dedupe-admission/internal/dedupe/service.go` | Stability | `MightContain` then `Put` was service-level non-atomic, so concurrent identical event IDs could both be admitted. | Added a service mutex around the admission decision and insertion, plus `TestServiceAdmitsConcurrentDuplicateOnlyOnce`; verified with targeted test and race test. |

## Performance/Stability Scan

Concurrency quick scan:

```bash
rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" examples/probabilistic-dedupe-admission
```

Reviewed hits:

- `main.go`: signal/shutdown contexts and one server goroutine match existing example lifecycle.
- `http_test.go`: `context.Background()` appears only in `httptest` requests.

No performance or stability issues found after the fixed P1.

## Verification Evidence

- `go test -count=1 ./examples/probabilistic-dedupe-admission/...`
- `go test -race -count=1 ./examples/probabilistic-dedupe-admission/...`

Final Step 6-R verdict after fix: P0 = 0, P1 = 0.
