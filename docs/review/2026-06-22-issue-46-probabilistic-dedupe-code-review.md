# Issue #46 Code Review

## 범위

`examples/probabilistic-dedupe-admission`, root README update, lesson/spec/plan artifact,
probabilistic package usage를 branch diff 기준으로 검토했다.

native review subagent는 이 session에서 사용하지 않았고, Step 6-R six-lane review는
full-feature review prompt를 기준으로 local에서 수행했다.

## Six-Lane Findings

| Tier | Perspective | Scope | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | Bloom filter admission hot path and stats projection. | 0 | 0 | 0 | 0 |
| 2 | Stability | shared filter state, concurrent duplicate admission, race gate. | 0 | 1 | 0 | 0 |
| 3 | Security | public HTTP errors, input handling, no secrets. | 0 | 0 | 0 | 0 |
| 4 | Operator | loopback-only demo service, health endpoint, durable-store caveat. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | Go API shape, error contracts, local example boundary. | 0 | 0 | 0 | 0 |
| 6 | User/Caller | README clarity, false-positive wording, root navigation. | 0 | 0 | 0 | 0 |

## Fixed Findings

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| P1 | `examples/probabilistic-dedupe-admission/internal/dedupe/service.go` | Stability | `MightContain` 이후 `Put`이 service level에서 atomic하지 않아 concurrent identical event ID 둘이 모두 admitted될 수 있었다. | admission decision과 insertion 주변에 service mutex를 추가하고 `TestServiceAdmitsConcurrentDuplicateOnlyOnce`를 추가했다. targeted test와 race test로 검증했다. |

## Performance/Stability Scan

Concurrency quick scan:

```bash
rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" examples/probabilistic-dedupe-admission
```

검토한 hit:

- `main.go`: signal/shutdown context와 server goroutine 하나는 기존 example lifecycle과 일치한다.
- `http_test.go`: `context.Background()`는 `httptest` request에만 등장한다.

fixed P1 이후 performance 또는 stability issue는 발견되지 않았다.

## 검증 Evidence

- `go test -count=1 ./examples/probabilistic-dedupe-admission/...`
- `go test -race -count=1 ./examples/probabilistic-dedupe-admission/...`

fix 이후 final Step 6-R verdict: P0 = 0, P1 = 0.
