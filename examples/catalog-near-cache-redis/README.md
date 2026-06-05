# catalog-near-cache-redis

[English](README.md) | [한국어](README.ko.md)

Catalog cache example for `bluetape-go/cache`, `cache/redisnear`, and
`cache/rediscoord`.

This example models two catalog service peers that keep a per-process memory
cache while sharing Redis for cross-peer invalidation and cold-miss stampede
coordination. The example stays deliberately small: Redis clients are
caller-owned, the authoritative store is an in-memory test projection, and
`Peer.Close` only stops the near-cache subscriber.

## Scenario

![Catalog near-cache Redis scenario](../../docs/images/readme-diagrams/catalog-near-cache-redis-scenario.png)

Peer B may already have `Product v1` in its local memory cache. Peer A writes
`Product v2` through `PutProduct`, updates the authoritative store, and publishes
a Redis near-cache event. Peer B ignores its own origin events but accepts peer A
events, deletes the stale SKU locally, and reloads the next read from the store.

## Architecture

![Catalog near-cache Redis architecture](../../docs/images/readme-diagrams/catalog-near-cache-redis-architecture.png)

The peer composes three bluetape-go cache pieces:

- `cache.NewMemory[string, Product]` keeps fast per-peer local entries.
- `redisnear.NewPubSub[Product]` wraps local memory and publishes invalidation
  events through Redis Pub/Sub.
- `rediscoord.NewStampedeCache[Product]` wraps the near cache and uses Redis
  locks/result envelopes so a cold burst has one owner loader.

`Store.Load` is the only backing loader. Tests assert `Store.LoadCount` so the
example proves behavior instead of only checking returned values.

## Sequence

![Catalog near-cache Redis sequence](../../docs/images/readme-diagrams/catalog-near-cache-redis-sequence.png)

When two peers miss the same SKU at the same time, one peer acquires the Redis
stampede lock and runs `Store.Load`. The other peer polls the Redis result
envelope and returns the same product without invoking the backing loader. A
later `Set` event invalidates the stale local entry on the other peer.

```go
product, err := peer.GetProduct(ctx, "sku-1")
```

```go
err := peer.PutProduct(ctx, Product{
    SKU:     "sku-1",
    Name:    "Updated",
    Version: 2,
})
```

## Outcomes

| Case | Behavior | Test |
|---|---|---|
| Peer write | Peer A writes `Product v2`, Redis Pub/Sub reaches peer B, and peer B reloads instead of serving stale local data. | `TestPeerWriteInvalidatesPeerLocalCacheAndReloadsStore` |
| Cold burst | Two peers miss the same SKU while the first loader is blocked, but the backing loader count stays at one. | `TestColdMissBurstAcrossPeersRunsBackingLoaderOnce` |
| Missing SKU | Loader errors are not cached, so a later store write can be loaded normally. | `TestMissingProductIsNotCached` |
| Invalid input | Blank peer names, namespaces, SKUs, nil Redis clients, nil stores, and negative options fail before hiding cache behavior. | `TestPeerRejectsInvalidInput`, `TestStoreRespectsContextCancellation` |

## Run

The tests start Redis with the repository Testcontainers fixture.

```bash
go test -count=1 ./examples/catalog-near-cache-redis/...
```

The race gate is useful because the example intentionally exercises concurrent
cold misses:

```bash
go test -race -count=1 ./examples/catalog-near-cache-redis/...
```
