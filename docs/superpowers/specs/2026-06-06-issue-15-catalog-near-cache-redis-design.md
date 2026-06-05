# Issue #15 Catalog Near-Cache Redis Example Design

## Problem

`bluetape-go` v0.3.0 adds cache coordination packages, but the workshop does
not yet have an application-shaped example that demonstrates how a caller would
combine process-local cache, Redis Pub/Sub invalidation, and Redis-based
stampede coordination.

Issue #15 requires a new `examples/catalog-near-cache-redis` scenario where two
catalog service peers share Redis invalidation and coordinate a cold-miss burst.

## Scope

In scope:

- Add `examples/catalog-near-cache-redis`.
- Compose:
  - `cache.NewMemory[string, Product]`
  - `redisnear.NewPubSub[Product]`
  - `rediscoord.NewStampedeCache[Product]`
- Model two catalog peers that read and write an authoritative product store.
- Use `github.com/bluetape4k/bluetape-go/testcontainers/redis`.
- Add tests for peer invalidation and cross-peer cold-miss load coordination.
- Add English and Korean README files.
- Add scenario, architecture, and sequence/flow diagrams with PNG embeds,
  matching SVG sources, and Graphviz evidence.
- Update root `README.md` and `README.ko.md` example tables.

Out of scope:

- New reusable cache helpers in the workshop repository.
- Durable Redis L2 cache behavior.
- HTTP service or CLI wrapper.
- New dependencies.
- Production observability adapters beyond README guidance.

## Evidence

Research note:

- `docs/superpowers/research/2026-06-06-issue-15-catalog-near-cache-redis-research.md`

Current source evidence:

- `go.mod` pins `github.com/bluetape4k/bluetape-go v0.3.0`.
- `cache.NewMemory[K,V]` is a process-local `LoadingCache`.
- `redisnear.NewPubSub[V]` wraps a local `LoadingCache[string,V]` and publishes
  invalidation on `Set`, `Delete`, and `Clear`.
- `rediscoord.NewStampedeCache[V]` wraps a `LoadingCache[string,V]` and shares
  a short-lived Redis owner-token result envelope during cold misses.
- `redistestcontainer.Start(ctx,t)` already owns Redis container lifecycle for
  workshop tests.

Prior lessons:

- Keep examples scenario-first and thin.
- Add `README.md` and `README.ko.md` together.
- Use shared English-label PNG/SVG diagram assets for localized READMEs.
- For coordination or concurrency behavior, include deterministic stress or
  burst tests.

## Architecture Pre-Design

### Option A: Near-Cache Only

Use `cache.NewMemory` plus `redisnear.NewPubSub`. Peer writes publish
invalidation, and peer reads reload from the authoritative store.

Rejected because it does not demonstrate `rediscoord.NewStampedeCache` or prove
cross-peer cold-miss stampede coordination.

### Option B: Stampede Coordination Only

Use `cache.NewMemory` plus `rediscoord.NewStampedeCache`. Two peers can
coordinate a cold-miss burst and run one loader.

Rejected because peer writes would not invalidate a different peer's local
cache. That misses the near-cache requirement and roadmap wording.

### Option C: Near-Cache Wrapped By Stampede Coordination

Each peer owns:

1. `cache.NewMemory[string, Product]`
2. `redisnear.NewPubSub[Product]` around the memory cache
3. `rediscoord.NewStampedeCache[Product]` around the near-cache

Accepted. This matches the `rediscoord` README behavior: waiters fill the
wrapped near-cache through `GetOrLoad`, while writes use the near-cache `Set`
path to publish invalidation to peers.

## Proposed Example Boundary

Package:

```text
examples/catalog-near-cache-redis/internal/catalogcache
```

Domain model:

- `Product`
  - `SKU string`
  - `Name string`
  - `Version int`
- `Store`
  - authoritative in-memory product source for tests and example code
  - concurrency-safe
- `Peer`
  - `Name string`
  - wraps a `cache.LoadingCache[string, Product]`
  - reads through `GetProduct`
  - writes through `PutProduct`
  - closes near-cache subscriber through `Close`

Construction:

- `NewPeer(ctx, name, redisClient, namespace, store, options)` creates the
  memory cache, near-cache, and stampede coordinator.
- Options stay narrow:
  - `TTL`
  - `LockTTL`
  - `ResultTTL`
  - `PollInterval`
- Defaults keep tests fast but realistic.

## Behavioral Contract

### Read

`Peer.GetProduct(ctx, sku)` calls the coordinated cache `GetOrLoad`. On miss,
the loader reads the authoritative store and returns a `Product`.

Expected behavior:

- Hot local reads do not call the store again.
- Cold cross-peer bursts for the same SKU run the backing loader once through
  Redis coordination.
- Missing SKU errors are returned and are not cached.

### Write

`Peer.PutProduct(ctx, product)` writes the authoritative store, then updates the
peer's near-cache via `Set`. The `Set` path publishes Redis invalidation.

Expected behavior:

- The writer sees the fresh value locally.
- Other peers delete their stale local entry.
- The next peer read reloads the authoritative product.

### Close

`Peer.Close()` closes the near-cache subscriber and Redis Pub/Sub resources.
Redis clients remain caller-owned and are closed by the test or application.

## Failure Modes And Risks

| Risk | Mitigation |
|---|---|
| Pub/Sub invalidation is asynchronous | Tests use bounded eventual assertions for peer cache miss/reload. |
| Cold burst test becomes flaky | Use a blocking loader and wait until exactly one loader invocation is observed before releasing it. |
| Goroutine/resource leak from near-cache subscribers | `Peer.Close` is idempotent and tests register cleanup for every peer. |
| Redis client ownership is unclear | Peers do not close caller-owned Redis clients; tests close clients separately. |
| Store errors are hidden by cache behavior | Missing SKU test verifies the sentinel/wrapped error path and no accidental cache fill. |
| Diagram drift from source | Diagram nodes and routes are generated after implementation from actual `Peer`, store, cache, near-cache, coordinator, and Redis roles. |

## Acceptance Criteria

- `go test -count=1 ./examples/catalog-near-cache-redis/...` passes.
- `go test -race -count=1 ./examples/catalog-near-cache-redis/...` passes.
- Peer A write invalidates peer B's local cache.
- Peer B reloads the authoritative product after invalidation.
- A cold miss burst across two peers runs the backing loader exactly once.
- Redis Testcontainers fixture is reused; no new container helper exists.
- `go.mod` and `go.sum` do not gain new dependencies.
- Root `README.md` and `README.ko.md` include the example row with
  `English | 한국어` links.
- Example `README.md` and `README.ko.md` both include scenario, architecture,
  flow/sequence explanation, run command, and operational boundaries.
- README diagrams embed PNG files only, with matching SVG, Graphviz DOT, plain,
  Graphviz SVG, and Graphviz PNG evidence where node-and-connector diagrams are
  added.

## DoD

| Item | Status |
|---|---|
| Research note created | Planned |
| Spec reviewed with P0=0/P1=0 | Planned |
| Plan reviewed with P0=0/P1=0 | Planned |
| Example implementation and tests added | Planned |
| Targeted test and race test pass | Planned |
| Full local validation passes or blocker recorded | Planned |
| README locale set updated | Planned |
| Diagram geometry gates and PNG inspections pass | Planned |
| Step 6-R code review P0=0/P1=0 | Planned |
| Lessons committed before PR | Planned |
| PR body ends with `## DoD Status` | Planned |
| GitHub CI passes | Planned |
