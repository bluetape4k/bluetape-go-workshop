# Issue #15 Step 5 검증자

Result: PASS

## 확인한 범위

- 이슈: `#15 [v0.3.0] Add Redis near-cache and stampede coordination catalog example`
- 명세: `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`
- 계획: `docs/superpowers/plans/2026-06-06-issue-15-catalog-near-cache-redis-plan.md`
- Code:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog.go`
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog_test.go`
- README:
  - `examples/catalog-near-cache-redis/README.md`
  - `examples/catalog-near-cache-redis/README.ko.md`
  - `README.md`
  - `README.ko.md`
- Diagrams:
  - `docs/images/readme-diagrams/catalog-near-cache-redis-scenario.*`
  - `docs/images/readme-diagrams/catalog-near-cache-redis-architecture.*`
  - `docs/images/readme-diagrams/catalog-near-cache-redis-sequence.*`
  - `docs/images/readme-diagrams/workshop-example-map.*`

## 인수 기준 매핑

| Requirement | Evidence |
|---|---|
| `examples/catalog-near-cache-redis` 추가 | 새 example package와 bilingual README를 추가했다. |
| `cache.NewMemory`, `redisnear.NewPubSub`, `rediscoord.NewStampedeCache` 조합 | `NewPeer`가 memory cache, Redis Pub/Sub near-cache, stampede coordinator를 조합한다. |
| Redis Pub/Sub으로 invalidation을 공유하는 catalog peer 두 개 모델링 | `TestPeerWriteInvalidatesPeerLocalCacheAndReloadsStore`는 서로 다른 Redis client와 origin ID를 가진 peer A와 peer B를 사용한다. |
| 기존 `testcontainers/redis` fixture 사용 | `redisClients`가 `redistestcontainer.Start(ctx, t)`를 호출한다. 새 fixture/helper package는 추가하지 않았다. |
| peer A write가 peer B local cache를 invalidation | test가 peer B를 prime하고 peer A로 write한 뒤 peer B가 `Product v2`를 reload할 때까지 기다린다. load count는 2에 도달한다. |
| cold miss burst가 loader를 한 번만 실행 | `TestColdMissBurstAcrossPeersRunsBackingLoaderOnce`는 owner load를 막고 두 concurrent peer를 실행한 뒤 final load count가 1임을 assert한다. |
| README와 root table 갱신 | example README pair와 root README/README.ko.md table row를 `English | 한국어` link와 함께 추가했다. |
| 풍부한 diagram 추가 | scenario, architecture, sequence diagram은 PNG embed와 matching SVG 및 Graphviz evidence를 포함한다. |
| 새 의존성 없음 | `bluetape-go/testing` helper 사용을 example-local polling helper로 바꾼 뒤 `go mod tidy`가 `go.mod` diff를 남기지 않는다. |

## 최신 검증

```text
go test -count=1 ./examples/catalog-near-cache-redis/... PASS
go test -race -count=1 ./examples/catalog-near-cache-redis/... PASS
git diff --check PASS
README SVG embed check PASS
SVG font-family stale UI font check PASS
```

diagram geometry gate summary:

```text
catalog-near-cache-redis-scenario.svg: nodes=6 routes=9 segments=21 badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=pass titleGap=pass
catalog-near-cache-redis-architecture.svg: nodes=6 routes=8 segments=19 badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=pass titleGap=pass
catalog-near-cache-redis-sequence.svg: nodes=6 routes=11 segments=11 badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=pass titleGap=pass
```

rendered PNG를 개별 검사했다.

- `catalog-near-cache-redis-scenario.png`
- `catalog-near-cache-redis-architecture.png`
- `catalog-near-cache-redis-sequence.png`
- `workshop-example-map.png`

## 알려진 공백

- full repository validation은 Step 6에 남아 있다: `go test ./...`, `make ci`, final
  `git diff --check`.
