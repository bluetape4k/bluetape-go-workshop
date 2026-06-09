# bluetape-go-workshop

[English](README.md) | [한국어](README.ko.md)

[`bluetape-go`](https://github.com/bluetape4k/bluetape-go)를 실제 웹 애플리케이션 형태로 사용하는 예제 저장소입니다.

이 저장소는 재사용 가능한 library package와 runnable application 예제를 분리하기
위해 둡니다. Library 저장소는 안정적인 package에 집중하고, workshop 저장소는
그 package들이 HTTP service와 container-backed integration test 안에서 어떻게
동작하는지 보여줍니다.

기본 web style은 lightweight 방향으로 둡니다. Framework 자체가 학습 포인트인
public HTTP API 예제는 [`Gin`](https://github.com/gin-gonic/gin)을 사용하고,
호환성 중심 예제는 [`chi`](https://github.com/go-chi/chi)나 plain `net/http`
handler를 사용합니다.

## Example Map

![Workshop example map](docs/images/readme-diagrams/workshop-example-map.png)

## v0.3.0 Cache 예제

Cache 예제는 작은 흐름으로 읽으면 됩니다.

| 먼저 볼 예제 | README | 사용할 때 |
|---|---|---|
| [`examples/cache-snapshot-codecs`](examples/cache-snapshot-codecs/README.ko.md) | [English](examples/cache-snapshot-codecs/README.md) \| [한국어](examples/cache-snapshot-codecs/README.ko.md) | portable cache snapshot format, versioned serialization envelope, 측정 가능한 compression tradeoff가 필요할 때 봅니다. |
| [`examples/catalog-near-cache-redis`](examples/catalog-near-cache-redis/README.ko.md) | [English](examples/catalog-near-cache-redis/README.md) \| [한국어](examples/catalog-near-cache-redis/README.ko.md) | 여러 catalog peer가 Redis invalidation을 공유하고 cold miss를 조정해 backing loader를 한 번만 실행해야 할 때 봅니다. |

`cache-snapshot-codecs`는 local payload/storage 쪽 예제입니다. Product snapshot을
안전하게 serialize하고, compression 선택을 명시적인 tradeoff로 남깁니다.
`catalog-near-cache-redis`는 distributed runtime 쪽 예제입니다. Local memory는 빠른
read path로 유지하고, Redis Pub/Sub는 stale peer를 invalidate하며, Redis
lock/result envelope는 cold burst가 backing store로 몰리는 일을 막습니다.

## 예제

| 예제 | README | 목적 | bluetape-go package |
|---|---|---|---|
| [`examples/cache-snapshot-codecs`](examples/cache-snapshot-codecs/README.ko.md) | [English](examples/cache-snapshot-codecs/README.md) \| [한국어](examples/cache-snapshot-codecs/README.ko.md) | 안전한 serialization과 compression 선택 기준을 보여주는 versioned product cache snapshot 예제입니다. | `serialization`, `compression` |
| [`examples/order-intake-cleanup`](examples/order-intake-cleanup/README.ko.md) | [English](examples/order-intake-cleanup/README.md) \| [한국어](examples/order-intake-cleanup/README.ko.md) | validation, default, filtering, deduplication, grouping으로 partner order feed를 정리하는 예제입니다. | `core`, `collections` |
| [`examples/invitation-codecs`](examples/invitation-codecs/README.ko.md) | [English](examples/invitation-codecs/README.md) \| [한국어](examples/invitation-codecs/README.ko.md) | invitation link, callback state, partner reference를 실용적인 string codec으로 다루는 예제입니다. | `codec`, `core` |
| [`examples/catalog-refresh-resilience`](examples/catalog-refresh-resilience/README.ko.md) | [English](examples/catalog-refresh-resilience/README.md) \| [한국어](examples/catalog-refresh-resilience/README.ko.md) | SKU refresh job에서 retry, attempt별 timeout, event visibility, diagram 기반 outcome을 보여주는 예제입니다. | `resilience` |
| [`examples/payment-authorization-guard`](examples/payment-authorization-guard/README.ko.md) | [English](examples/payment-authorization-guard/README.md) \| [한국어](examples/payment-authorization-guard/README.ko.md) | Circuit breaker, bulkhead overflow rejection, synchronous event로 payment authorization gateway를 보호하는 예제입니다. | `resilience` |
| [`examples/leader-redis-web`](examples/leader-redis-web/README.ko.md) | [English](examples/leader-redis-web/README.md) \| [한국어](examples/leader-redis-web/README.ko.md) | Redis 기반 leader election을 수행하고 leader 상태를 노출하는 최소 chi 기반 HTTP service입니다. | `leader`, `leader/redis`, `testcontainers/redis` |
| [`examples/leader-coordination-jobs`](examples/leader-coordination-jobs/README.ko.md) | [English](examples/leader-coordination-jobs/README.md) \| [한국어](examples/leader-coordination-jobs/README.ko.md) | Redis leader election으로 migration gate와 cache warmer job을 조정하는 예제입니다. | `leader`, `leader/redis`, `testing/concurrency` |
| [`examples/product-enrichment-fanout`](examples/product-enrichment-fanout/README.ko.md) | [English](examples/product-enrichment-fanout/README.md) \| [한국어](examples/product-enrichment-fanout/README.ko.md) | bounded goroutine, cancellation, panic capture, stress test를 포함한 product detail fan-out 예제입니다. | `concurrency`, `testing/concurrency` |
| [`examples/order-pipeline-testcontainers`](examples/order-pipeline-testcontainers/README.ko.md) | [English](examples/order-pipeline-testcontainers/README.md) \| [한국어](examples/order-pipeline-testcontainers/README.ko.md) | repository Testcontainers fixture로 PostgreSQL, Redis, NATS 통합 흐름을 검증하는 예제입니다. | `testcontainers/postgres`, `testcontainers/redis`, `testcontainers/nats` |
| [`examples/catalog-near-cache-redis`](examples/catalog-near-cache-redis/README.ko.md) | [English](examples/catalog-near-cache-redis/README.md) \| [한국어](examples/catalog-near-cache-redis/README.ko.md) | catalog peer를 위한 Redis near-cache invalidation과 cold-miss stampede coordination 예제입니다. | `cache`, `cache/redisnear`, `cache/rediscoord`, `testcontainers/redis` |
| [`examples/resilience-http-web`](examples/resilience-http-web/README.ko.md) | [English](examples/resilience-http-web/README.md) \| [한국어](examples/resilience-http-web/README.ko.md) | retry, timeout, circuit breaker, bulkhead, event hook을 조합하는 HTTP service입니다. | `resilience` |
| [`examples/leader-group-web`](examples/leader-group-web/README.ko.md) | [English](examples/leader-group-web/README.md) \| [한국어](examples/leader-group-web/README.ko.md) | Redis 기반 bounded multi-leader group election을 노출하는 HTTP service입니다. | `leader`, `leader/redis`, `testing/concurrency` |
| [`examples/order-lifecycle-state-api`](examples/order-lifecycle-state-api/README.ko.md) | [English](examples/order-lifecycle-state-api/README.md) \| [한국어](examples/order-lifecycle-state-api/README.ko.md) | In-memory 주문 lifecycle finite state machine과 transition command를 노출하는 Gin API입니다. | `state` |
| [`examples/payment-authorization-state`](examples/payment-authorization-state/README.ko.md) | [English](examples/payment-authorization-state/README.md) \| [한국어](examples/payment-authorization-state/README.ko.md) | Payment authorization transition과 application-layer idempotent retry 동작을 보여주는 Gin API입니다. | `state` |
| [`examples/fulfillment-workflow-runner`](examples/fulfillment-workflow-runner/README.ko.md) | [English](examples/fulfillment-workflow-runner/README.md) \| [한국어](examples/fulfillment-workflow-runner/README.ko.md) | Sequential, parallel, conditional fulfillment workflow runner를 조합하는 Gin API입니다. | `workflow`, `workreport` |
| [`examples/compensation-workflow`](examples/compensation-workflow/README.ko.md) | [English](examples/compensation-workflow/README.md) \| [한국어](examples/compensation-workflow/README.ko.md) | Fulfillment step을 실행하고 뒤 workflow step이 실패하면 완료된 side effect를 되돌리는 Gin API입니다. | `workflow`, `workreport` |
| [`examples/operations-report-policy`](examples/operations-report-policy/README.ko.md) | [English](examples/operations-report-policy/README.md) \| [한국어](examples/operations-report-policy/README.ko.md) | Work report와 failure policy를 deterministic operations output으로 투영하는 Gin API입니다. | `workreport` |

## Leader 예제 실행

Redis를 먼저 실행한 뒤 service를 시작합니다:

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-redis-web
```

주요 endpoint:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/leader
curl -X POST http://localhost:8080/campaign
curl -X POST http://localhost:8080/resign
```

## Resilience 예제 실행

Catalog service를 로컬에서 먼저 실행한 뒤 service를 시작합니다:

```bash
export CATALOG_URL=http://localhost:9090
go run ./examples/resilience-http-web
```

주요 endpoint:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/catalog/book-1
curl -X POST 'http://localhost:8081/orders?delay=25ms'
curl http://localhost:8081/events
```

## Leader Group 예제 실행

Redis를 먼저 실행한 뒤 service를 시작합니다:

```bash
export REDIS_ADDR=localhost:6379
export MAX_LEADERS=2
go run ./examples/leader-group-web
```

주요 endpoint:

```bash
curl http://localhost:8082/healthz
curl http://localhost:8082/group
curl -X POST http://localhost:8082/campaign
curl -X POST http://localhost:8082/resign
```

## Order Lifecycle State API 예제 실행

Service를 로컬에서 실행합니다:

```bash
go run ./examples/order-lifecycle-state-api
```

주요 endpoint:

```bash
curl http://localhost:8083/healthz
curl http://localhost:8083/orders/current
curl http://localhost:8083/orders/current/transitions/pay/can
curl -X POST http://localhost:8083/orders/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"submit"}'
```

## Fulfillment Workflow Runner 예제 실행

Service를 로컬에서 실행합니다:

```bash
go run ./examples/fulfillment-workflow-runner
```

주요 endpoint:

```bash
curl http://localhost:8084/healthz
curl -X POST http://localhost:8084/fulfillment/run \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1001","stock_available":true,"payment_authorized":true,"requires_shipment":true}'
```

## Payment Authorization State 예제 실행

Service를 로컬에서 실행합니다:

```bash
go run ./examples/payment-authorization-state
```

주요 endpoint:

```bash
curl http://localhost:8086/healthz
curl http://localhost:8086/payments/current
curl -X POST http://localhost:8086/payments/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"authorize","idempotency_key":"auth-1"}'
```

## Compensation Workflow 예제 실행

Service를 로컬에서 실행합니다:

```bash
go run ./examples/compensation-workflow
```

주요 endpoint:

```bash
curl http://localhost:8087/healthz
curl -X POST http://localhost:8087/compensation/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1001","stock_available":true,"payment_authorized":true,"shipment_provider_available":false}'
```

## Operations Report Policy 예제 실행

Service를 로컬에서 실행합니다:

```bash
go run ./examples/operations-report-policy
```

주요 endpoint:

```bash
curl http://localhost:8085/healthz
curl -X POST http://localhost:8085/operations/report \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"release-1001","policy":"continue_on_failure","products_valid":true}'
```

## 개발

자주 쓰는 명령:

```bash
make ci
```

| 명령 | 설명 |
|---|---|
| `make fmt` | Go source를 `gofmt`로 format합니다. |
| `make fmt-check` | Go source가 `gofmt` 형식이 아니면 실패합니다. |
| `make tidy-check` | `go mod tidy` 후 `go.mod`/`go.sum` 변경이 있으면 실패합니다. |
| `make vet` | `go vet ./...`를 실행합니다. |
| `make lint` | `golangci-lint run ./...`를 실행합니다. |
| `make test` | Testcontainers 테스트가 실제 실행되도록 `go test -count=1 ./...`를 실행합니다. |
| `make race` | Testcontainers 테스트가 race detector에서도 실제 실행되도록 `go test -race -count=1 ./...`를 실행합니다. |
| `make ci` | 로컬 CI gate를 실행합니다. |

Integration test는 Testcontainers를 사용하므로 Docker가 필요합니다. 일반 CI와
Nightly workflow 모두 실제 container를 사용해 테스트합니다.

## Roadmap

| bluetape-go milestone | Workshop 예제 방향 |
|---|---|
| `0.1.0` | Redis leader election web service. |
| `0.1.1` | Quality-closure resilience primitive을 위한 focused retry/timeout 예제. |
| `0.2.0` | HTTP client/service resilience, payment authorization guard, bounded leader group coordination 예제. |
| `0.3.0` | Near-cache, Redis invalidation, stampede coordination 예제. |
| `0.4.0` | Gin order lifecycle과 payment authorization state API, fulfillment workflow runner, compensation workflow, operations report policy API를 포함한 state/workflow 예제. |
| `0.5.0` | Batch processing 예제. |
