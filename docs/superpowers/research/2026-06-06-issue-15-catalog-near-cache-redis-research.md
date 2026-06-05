# Issue #15 Redis Near-Cache Catalog Example Research

## Scope

Issue #15 asks for `examples/catalog-near-cache-redis`, a scenario-first
workshop example for the `bluetape-go` v0.3.0 cache coordination packages.

Required behavior:

- Compose `cache.NewMemory`, `cache/redisnear.NewPubSub`, and
  `cache/rediscoord.NewStampedeCache`.
- Model two catalog service peers sharing invalidation over Redis Pub/Sub.
- Use the existing `testcontainers/redis` fixture.
- Prove peer invalidation and cold-miss load coordination.
- Add bilingual README coverage and update root README tables.

## Current Repository Evidence

- `go.mod` pins `github.com/bluetape4k/bluetape-go v0.3.0`.
- `go list ./...` currently has no `examples/catalog-near-cache-redis` package.
- Redis fixture usage already exists in:
  - `examples/leader-coordination-jobs/internal/leaderjobs/jobs_test.go`
  - `examples/leader-group-web/internal/groupweb/server_test.go`
  - `examples/order-pipeline-testcontainers/internal/pipeline/pipeline_test.go`
- Existing example READMEs use `[English](README.md) | [한국어](README.ko.md)`.
- Recent example documentation expects scenario diagrams with PNG embeds and
  matching SVG plus Graphviz evidence under `docs/images/readme-diagrams/`.

## `bluetape-go` API Evidence

From `github.com/bluetape4k/bluetape-go@v0.3.0/cache`:

- `cache.NewMemory[K,V]()` returns a process-local `LoadingCache`.
- `cache.LoadingCache` exposes `Get`, `Set`, `Delete`, `Clear`, and
  `GetOrLoad`.
- `cache.ErrCacheMiss` is the miss sentinel callers can check with
  `errors.Is`.

From `cache/redisnear`:

- `redisnear.NewPubSub[V](ctx, Options[V])` creates a Redis Pub/Sub near-cache.
- `Options` requires a Redis `Client`; `Namespace`, `OriginID`, `Local`, and
  `OnError` are optional.
- `Set`, `Delete`, and `Clear` mutate local cache and publish peer
  invalidation.
- `GetOrLoad` fills only the local cache and does not publish invalidation.
- `Close` is idempotent and should be registered with `t.Cleanup`.

From `cache/rediscoord`:

- `rediscoord.NewStampedeCache[V](Options[V])` wraps an existing
  `cache.LoadingCache[string,V]`.
- `Options` requires Redis `Client`, wrapped `Cache`, and `Codec`.
- `GetOrLoad` checks the wrapped cache, acquires a Redis owner-token lock for a
  cold miss, and lets waiters consume a short-lived result envelope.
- `rediscoord.JSONCodec[V]{}` is available for value payloads.

From `testcontainers/redis`:

- `redistestcontainer.Start(ctx, t)` launches Redis `7.4-alpine` and returns
  `host:port`.
- The helper owns container termination through `t.Cleanup`; no new container
  helper is needed.

## Prior Lessons Applied

- Keep workshop examples thin; reusable support code belongs in `bluetape-go`.
- Add `README.md` and `README.ko.md` together, with localized prose and shared
  English-label diagram assets.
- For coordination/concurrency behavior, include deterministic stress or burst
  tests before opening a PR.
- Render and inspect actual PNGs; do not rely on SVG syntax or Graphviz output
  alone.

## Adoption Decisions

| Decision | Rationale |
|---|---|
| Adopt `cache.NewMemory` as each peer's local store | Matches issue scope and keeps example local-cache behavior explicit. |
| Adopt `redisnear.NewPubSub` for peer invalidation | Directly models Redis Pub/Sub invalidation without adding app-level buses. |
| Adopt `rediscoord.NewStampedeCache` around the near-cache | Demonstrates cold-miss coordination while preserving invalidation semantics. |
| Reuse `redistestcontainer.Start(ctx,t)` | Required by issue and avoids another container helper. |
| Keep authoritative catalog store as example-local in-memory state | The scenario needs an origin of truth, not a new persistence abstraction. |
| Add README diagrams for scenario, architecture, and sequence/flow | User requested rich examples with scenario, architecture, flow, and sequence diagrams. |

## Constraints And Risks

- Redis Pub/Sub invalidation is asynchronous; tests need an eventually helper
  rather than immediate assertions.
- The stampede coordination test must create a real cold burst across two peers
  and prove exactly one backing loader invocation.
- The example must close Redis clients and near-cache subscribers on all test
  paths.
- No dependency changes are allowed.
- Testcontainers-backed validation must run serially.
