# Issue #15 Redis Near-Cache Catalog 예제 리서치

## 범위

#15는 `bluetape-go` v0.3.0 캐시 조정 패키지를 시나리오 중심으로 보여 주는
`examples/catalog-near-cache-redis` 워크숍 예제를 요구한다.

필수 동작은 다음과 같다.

- `cache.NewMemory`, `cache/redisnear.NewPubSub`,
  `cache/rediscoord.NewStampedeCache`를 조합한다.
- Redis Pub/Sub으로 무효화를 공유하는 두 카탈로그 서비스 피어를 모델링한다.
- 기존 `testcontainers/redis` fixture를 사용한다.
- 피어 무효화와 cold-miss 적재 조정을 증명한다.
- 이중 언어 README를 추가하고 루트 README 표를 갱신한다.

## 현재 저장소 근거

- `go.mod`는 `github.com/bluetape4k/bluetape-go v0.3.0`에 고정되어 있다.
- 현재 `go list ./...`에는 `examples/catalog-near-cache-redis` 패키지가 없다.
- Redis fixture 사용 사례는 이미 다음 위치에 있다.
  - `examples/leader-coordination-jobs/internal/leaderjobs/jobs_test.go`
  - `examples/leader-group-web/internal/groupweb/server_test.go`
  - `examples/order-pipeline-testcontainers/internal/pipeline/pipeline_test.go`
- 기존 예제 README는 `[English](README.md) | [한국어](README.ko.md)` 형식을 사용한다.
- 최근 예제 문서는 PNG embed가 있는 시나리오 다이어그램과 이에 대응하는 SVG,
  Graphviz 근거를 `docs/images/readme-diagrams/` 아래에 둔다.

## `bluetape-go` API 근거

`github.com/bluetape4k/bluetape-go@v0.3.0/cache` 기준 근거는 다음과 같다.

- `cache.NewMemory[K,V]()`는 프로세스 로컬 `LoadingCache`를 반환한다.
- `cache.LoadingCache`는 `Get`, `Set`, `Delete`, `Clear`, `GetOrLoad`를
  노출한다.
- `cache.ErrCacheMiss`는 호출자가 `errors.Is`로 확인할 수 있는 miss sentinel이다.

`cache/redisnear` 기준 근거는 다음과 같다.

- `redisnear.NewPubSub[V](ctx, Options[V])`는 Redis Pub/Sub near-cache를 만든다.
- `Options`에는 Redis `Client`가 필요하고, `Namespace`, `OriginID`, `Local`,
  `OnError`는 선택 항목이다.
- `Set`, `Delete`, `Clear`는 로컬 캐시를 변경하고 피어 무효화를 publish한다.
- `GetOrLoad`는 로컬 캐시만 채우며 무효화를 publish하지 않는다.
- `Close`는 멱등이며 `t.Cleanup`에 등록해야 한다.

`cache/rediscoord` 기준 근거는 다음과 같다.

- `rediscoord.NewStampedeCache[V](Options[V])`는 기존
  `cache.LoadingCache[string,V]`를 감싼다.
- `Options`에는 Redis `Client`, 감쌀 `Cache`, `Codec`이 필요하다.
- `GetOrLoad`는 감싼 캐시를 확인하고, cold miss에서는 Redis owner-token lock을
  획득한 뒤, 대기 호출자가 짧게 유지되는 result envelope을 소비하게 한다.
- 값 payload에는 `rediscoord.JSONCodec[V]{}`를 사용할 수 있다.

`testcontainers/redis` 기준 근거는 다음과 같다.

- `redistestcontainer.Start(ctx, t)`는 Redis `7.4-alpine`을 띄우고 `host:port`를
  반환한다.
- 이 helper는 `t.Cleanup`으로 컨테이너 종료를 소유하므로 새 컨테이너 helper가
  필요하지 않다.

## 적용한 기존 교훈

- 워크숍 예제는 얇게 유지하고, 재사용 가능한 지원 코드는 `bluetape-go`에 둔다.
- `README.md`와 `README.ko.md`를 함께 추가하되, prose는 현지화하고 다이어그램
  asset의 English label은 공유한다.
- 조정/동시성 동작은 PR을 열기 전에 결정적인 stress 또는 burst 테스트로 고정한다.
- SVG 문법이나 Graphviz 출력만 신뢰하지 말고 실제 PNG를 render해 검사한다.

## 채택 결정

| 결정 | 근거 |
|---|---|
| 각 피어의 로컬 저장소로 `cache.NewMemory` 채택 | 이슈 범위와 맞고 예제의 local-cache 동작을 명시적으로 보여 준다. |
| 피어 무효화에 `redisnear.NewPubSub` 채택 | app-level bus를 추가하지 않고 Redis Pub/Sub 무효화를 직접 모델링한다. |
| near-cache를 `rediscoord.NewStampedeCache`로 감싸기 | 무효화 의미를 보존하면서 cold-miss 조정을 보여 준다. |
| `redistestcontainer.Start(ctx,t)` 재사용 | 이슈에서 요구하며, 별도 컨테이너 helper를 피한다. |
| authoritative catalog store를 예제 로컬 in-memory 상태로 유지 | 시나리오에는 진실의 원천이 필요하지만 새 persistence 추상화는 필요하지 않다. |
| 시나리오, 아키텍처, sequence/flow README 다이어그램 추가 | 사용자가 시나리오, 아키텍처, flow, sequence 다이어그램이 있는 풍부한 예제를 요구했다. |

## 제약과 위험

- Redis Pub/Sub 무효화는 비동기이므로 테스트에는 즉시 assertion이 아니라
  eventually helper가 필요하다.
- stampede 조정 테스트는 두 피어에 걸친 실제 cold burst를 만들고 backing loader
  호출이 정확히 한 번임을 증명해야 한다.
- 예제는 모든 테스트 경로에서 Redis client와 near-cache subscriber를 닫아야 한다.
- 의존성 변경은 허용하지 않는다.
- Testcontainers 기반 검증은 순차 실행해야 한다.
