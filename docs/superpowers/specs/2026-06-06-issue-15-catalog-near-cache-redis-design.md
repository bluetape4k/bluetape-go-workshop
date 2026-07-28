# Issue #15 Catalog Near-Cache Redis Example Design

## 문제

`bluetape-go` v0.3.0은 cache coordination package를 추가했지만, workshop에는 caller가
process-local cache, Redis Pub/Sub invalidation, Redis-based stampede coordination을 어떻게 조합하는지 보여주는
application-shaped example이 아직 없다.

issue #15는 catalog service peer 두 개가 Redis invalidation을 공유하고 cold-miss burst를 coordinate하는
새 `examples/catalog-near-cache-redis` scenario를 요구한다.

## Scope

범위 포함:

- Add `examples/catalog-near-cache-redis`.
- Compose:
  - `cache.NewMemory[string, Product]`
  - `redisnear.NewPubSub[Product]`
  - `rediscoord.NewStampedeCache[Product]`
- authoritative product store를 read/write하는 catalog peer 두 개를 model한다.
- `github.com/bluetape4k/bluetape-go/testcontainers/redis`를 사용한다.
- peer invalidation 및 cross-peer cold-miss load coordination test를 추가한다.
- English 및 Korean README file을 추가한다.
- PNG embed, matching SVG source, Graphviz evidence를 가진 scenario, architecture, sequence/flow diagram을 추가한다.
- root `README.md`와 `README.ko.md` example table을 update한다.

범위 제외:

- workshop repository의 새 reusable cache helper.
- durable Redis L2 cache behavior.
- HTTP service 또는 CLI wrapper.
- 새 dependency.
- README guidance 범위를 넘는 production observability adapter는 제외한다.

## 근거

research note:

- `docs/superpowers/research/2026-06-06-issue-15-catalog-near-cache-redis-research.md`

current source evidence:

- `go.mod`는 `github.com/bluetape4k/bluetape-go v0.3.0`을 pin한다.
- `cache.NewMemory[K,V]`는 process-local `LoadingCache`다.
- `redisnear.NewPubSub[V]`는 local `LoadingCache[string,V]`를 wrap하고
  `Set`, `Delete`, `Clear`에서 invalidation을 publish한다.
- `rediscoord.NewStampedeCache[V]`는 `LoadingCache[string,V]`를 wrap하고
  cold miss 동안 short-lived Redis owner-token result envelope를 공유한다.
- `redistestcontainer.Start(ctx,t)`는 workshop test용 Redis container lifecycle을 이미 소유한다.

prior lesson:

- example은 scenario-first이고 thin하게 유지한다.
- `README.md`와 `README.ko.md`를 함께 추가한다.
- localized README에는 shared English-label PNG/SVG diagram asset을 사용한다.
- coordination 또는 concurrency behavior에는 deterministic stress 또는 burst test를 포함한다.

## Architecture Pre-Design

### Option A: near-cache만 사용

`cache.NewMemory`와 `redisnear.NewPubSub`를 사용한다. peer write는 invalidation을 publish하고,
peer read는 authoritative store에서 reload한다.

`rediscoord.NewStampedeCache`를 보여주지 못하고 cross-peer cold-miss stampede coordination을 증명하지 못하므로 거절한다.

### Option B: stampede coordination만 사용

`cache.NewMemory`와 `rediscoord.NewStampedeCache`를 사용한다.
두 peer는 cold-miss burst를 coordinate하고 loader 하나만 실행할 수 있다.

peer write가 다른 peer의 local cache를 invalidate하지 못하므로 거절한다.
이는 near-cache requirement와 roadmap wording을 놓친다.

### Option C: stampede coordination으로 감싼 near-cache

각 peer는 다음 항목을 소유한다.

1. `cache.NewMemory[string, Product]`
2. `redisnear.NewPubSub[Product]` around the memory cache
3. `rediscoord.NewStampedeCache[Product]` around the near-cache

채택한다. 이는 `rediscoord` README behavior와 맞다. waiter는 `GetOrLoad`를 통해 wrapped near-cache를 채우고,
write는 near-cache `Set` path를 사용해 peer에 invalidation을 publish한다.

## Proposed Example Boundary

Package:

```text
examples/catalog-near-cache-redis/internal/catalogcache
```

domain model:

- `Product`
  - `SKU string`
  - `Name string`
  - `Version int`
- `Store`
  - test 및 example code용 authoritative in-memory product source
  - concurrency-safe
- `Peer`
  - `Name string`
  - `cache.LoadingCache[string, Product]`를 wrap한다
  - `GetProduct`로 read한다
  - `PutProduct`로 write한다
  - `Close`로 near-cache subscriber를 close한다

construction:

- `NewPeer(ctx, name, redisClient, namespace, store, options)`는 memory cache,
  near-cache, stampede coordinator를 만든다.
- option은 narrow하게 유지한다.
  - `TTL`
  - `LockTTL`
  - `ResultTTL`
  - `PollInterval`
- default는 test를 빠르지만 realistic하게 유지한다.

## Behavioral Contract

### Read

`Peer.GetProduct(ctx, sku)`는 coordinated cache `GetOrLoad`를 호출한다.
miss에서는 loader가 authoritative store를 읽고 `Product`를 반환한다.

기대 behavior:

- hot local read는 store를 다시 호출하지 않는다.
- 같은 SKU에 대한 cold cross-peer burst는 Redis coordination으로 backing loader를 한 번만 실행한다.
- missing SKU error는 반환되고 cache되지 않는다.

### Write

`Peer.PutProduct(ctx, product)`는 authoritative store에 write한 뒤 `Set`으로 peer의 near-cache를 update한다.
`Set` path는 Redis invalidation을 publish한다.

기대 behavior:

- writer는 fresh value를 local에서 본다.
- 다른 peer는 stale local entry를 delete한다.
- 다음 peer read는 authoritative product를 reload한다.

### Close

`Peer.Close()`는 near-cache subscriber와 Redis Pub/Sub resource를 close한다.
Redis client는 caller-owned로 남고 test 또는 application이 close한다.

## Failure Mode 및 Risk

| Risk | Mitigation |
|---|---|
| Pub/Sub invalidation이 asynchronous | test는 peer cache miss/reload에 bounded eventual assertion을 사용한다. |
| cold burst test가 flaky해짐 | blocking loader를 사용하고 정확히 loader invocation 하나가 관측될 때까지 기다린 뒤 release한다. |
| near-cache subscriber의 goroutine/resource leak | `Peer.Close`는 idempotent이고 test는 모든 peer에 cleanup을 등록한다. |
| Redis client ownership이 불명확 | peer는 caller-owned Redis client를 close하지 않는다. test가 client를 별도로 close한다. |
| store error가 cache behavior에 가려짐 | missing SKU test가 sentinel/wrapped error path와 accidental cache fill 없음을 verify한다. |
| source와 diagram drift | implementation 뒤 actual `Peer`, store, cache, near-cache, coordinator, Redis role에서 diagram node와 route를 생성한다. |

## Acceptance Criteria

- `go test -count=1 ./examples/catalog-near-cache-redis/...`가 pass한다.
- `go test -race -count=1 ./examples/catalog-near-cache-redis/...`가 pass한다.
- peer A write는 peer B의 local cache를 invalidate한다.
- peer B는 invalidation 뒤 authoritative product를 reload한다.
- 두 peer에 걸친 cold miss burst는 backing loader를 정확히 한 번 실행한다.
- Redis Testcontainers fixture를 재사용하며 새 container helper는 없다.
- `go.mod`와 `go.sum`에 새 dependency가 추가되지 않는다.
- root `README.md`와 `README.ko.md`는 `English | 한국어` link가 있는 example row를 포함한다.
- example `README.md`와 `README.ko.md`는 둘 다 scenario, architecture,
  flow/sequence explanation, run command, operational boundary를 포함한다.
- README diagram은 PNG file만 embed하고, node-and-connector diagram을 추가하는 경우 matching SVG,
  Graphviz DOT, plain, Graphviz SVG, Graphviz PNG evidence를 가진다.

## DoD

| Item | Status |
|---|---|
| research note 생성 | Planned |
| spec review P0=0/P1=0 | Planned |
| plan review P0=0/P1=0 | Planned |
| example implementation 및 test 추가 | Planned |
| targeted test 및 race test pass | Planned |
| full local validation pass 또는 blocker 기록 | Planned |
| README locale set update | Planned |
| diagram geometry gate 및 PNG inspection pass | Planned |
| Step 6-R code review P0=0/P1=0 | Planned |
| PR 전 lesson commit | Planned |
| PR body가 `## DoD Status`로 끝남 | Planned |
| GitHub CI pass | Planned |
