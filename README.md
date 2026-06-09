# bluetape-go-workshop

[English](README.md) | [한국어](README.ko.md)

Runnable web application examples for [`bluetape-go`](https://github.com/bluetape4k/bluetape-go).

This repository keeps application-shaped examples separate from reusable library
packages. The library repository should stay focused on stable packages; this
workshop shows how those packages behave inside real HTTP services and
container-backed integration tests.

The default web style stays lightweight. Public HTTP API examples use
[`Gin`](https://github.com/gin-gonic/gin) when the framework is part of the
lesson, while compatibility-focused examples use
[`chi`](https://github.com/go-chi/chi) or plain `net/http` handlers.

## Example Map

![Workshop example map](docs/images/readme-diagrams/workshop-example-map.png)

## v0.3.0 Cache Examples

Read the cache examples as a small progression:

| Start here | README | Use when |
|---|---|---|
| [`examples/cache-snapshot-codecs`](examples/cache-snapshot-codecs) | [English](examples/cache-snapshot-codecs/README.md) \| [한국어](examples/cache-snapshot-codecs/README.ko.md) | You need a portable cache snapshot format, versioned serialization envelope, and measured compression tradeoff. |
| [`examples/catalog-near-cache-redis`](examples/catalog-near-cache-redis) | [English](examples/catalog-near-cache-redis/README.md) \| [한국어](examples/catalog-near-cache-redis/README.ko.md) | You need multiple catalog peers to share Redis invalidation and coordinate cold misses so one backing loader runs. |

`cache-snapshot-codecs` is the local payload/storage side of the story:
serializing a product snapshot safely and compressing it with an explicit
tradeoff. `catalog-near-cache-redis` is the distributed runtime side: local
memory stays fast, Redis Pub/Sub invalidates stale peers, and Redis locks/result
envelopes prevent a cold burst from stampeding the backing store.

## Examples

| Example | README | Purpose | bluetape-go packages |
|---|---|---|---|
| [`examples/cache-snapshot-codecs`](examples/cache-snapshot-codecs) | [English](examples/cache-snapshot-codecs/README.md) \| [한국어](examples/cache-snapshot-codecs/README.ko.md) | Versioned product cache snapshots with safe serialization and compression tradeoff notes. | `serialization`, `compression` |
| [`examples/order-intake-cleanup`](examples/order-intake-cleanup) | [English](examples/order-intake-cleanup/README.md) \| [한국어](examples/order-intake-cleanup/README.ko.md) | Partner order feed cleanup with validation, defaults, filtering, deduplication, and grouping. | `core`, `collections` |
| [`examples/invitation-codecs`](examples/invitation-codecs) | [English](examples/invitation-codecs/README.md) \| [한국어](examples/invitation-codecs/README.ko.md) | Invitation links, callback state, and partner references with practical string codecs. | `codec`, `core` |
| [`examples/catalog-refresh-resilience`](examples/catalog-refresh-resilience) | [English](examples/catalog-refresh-resilience/README.md) \| [한국어](examples/catalog-refresh-resilience/README.ko.md) | SKU refresh job with retry, per-attempt timeout, event visibility, and diagrammed policy outcomes. | `resilience` |
| [`examples/payment-authorization-guard`](examples/payment-authorization-guard) | [English](examples/payment-authorization-guard/README.md) \| [한국어](examples/payment-authorization-guard/README.ko.md) | Payment authorization gateway protected by circuit breaker, bulkhead overflow rejection, and synchronous events. | `resilience` |
| [`examples/leader-redis-web`](examples/leader-redis-web) | [English](examples/leader-redis-web/README.md) \| [한국어](examples/leader-redis-web/README.ko.md) | Minimal chi-based HTTP service that campaigns for Redis-backed leadership and exposes leader state. | `leader`, `leader/redis`, `testcontainers/redis` |
| [`examples/leader-coordination-jobs`](examples/leader-coordination-jobs) | [English](examples/leader-coordination-jobs/README.md) \| [한국어](examples/leader-coordination-jobs/README.ko.md) | Migration gate and cache warmer jobs guarded by Redis leader election. | `leader`, `leader/redis`, `testing/concurrency` |
| [`examples/product-enrichment-fanout`](examples/product-enrichment-fanout) | [English](examples/product-enrichment-fanout/README.md) \| [한국어](examples/product-enrichment-fanout/README.ko.md) | Product detail fan-out with bounded goroutines, cancellation, panic capture, and stress tests. | `concurrency`, `testing/concurrency` |
| [`examples/order-pipeline-testcontainers`](examples/order-pipeline-testcontainers) | [English](examples/order-pipeline-testcontainers/README.md) \| [한국어](examples/order-pipeline-testcontainers/README.ko.md) | PostgreSQL, Redis, and NATS integration flow using repository Testcontainers fixtures. | `testcontainers/postgres`, `testcontainers/redis`, `testcontainers/nats` |
| [`examples/catalog-near-cache-redis`](examples/catalog-near-cache-redis) | [English](examples/catalog-near-cache-redis/README.md) \| [한국어](examples/catalog-near-cache-redis/README.ko.md) | Redis near-cache invalidation and cold-miss stampede coordination for catalog peers. | `cache`, `cache/redisnear`, `cache/rediscoord`, `testcontainers/redis` |
| [`examples/resilience-http-web`](examples/resilience-http-web) | [English](examples/resilience-http-web/README.md) \| [한국어](examples/resilience-http-web/README.ko.md) | HTTP service that composes retry, timeout, circuit breaker, bulkhead, and event hooks. | `resilience` |
| [`examples/leader-group-web`](examples/leader-group-web) | [English](examples/leader-group-web/README.md) \| [한국어](examples/leader-group-web/README.ko.md) | HTTP service for Redis-backed bounded multi-leader group election. | `leader`, `leader/redis`, `testing/concurrency` |
| [`examples/order-lifecycle-state-api`](examples/order-lifecycle-state-api) | [English](examples/order-lifecycle-state-api/README.md) \| [한국어](examples/order-lifecycle-state-api/README.ko.md) | Gin API that exposes an in-memory order lifecycle finite state machine and transition commands. | `state` |
| [`examples/payment-authorization-state`](examples/payment-authorization-state) | [English](examples/payment-authorization-state/README.md) \| [한국어](examples/payment-authorization-state/README.ko.md) | Gin API for payment authorization transitions with app-layer idempotent retry behavior. | `state` |
| [`examples/fulfillment-workflow-runner`](examples/fulfillment-workflow-runner) | [English](examples/fulfillment-workflow-runner/README.md) \| [한국어](examples/fulfillment-workflow-runner/README.ko.md) | Gin API that composes sequential, parallel, and conditional fulfillment workflow runners. | `workflow`, `workreport` |
| [`examples/compensation-workflow`](examples/compensation-workflow) | [English](examples/compensation-workflow/README.md) \| [한국어](examples/compensation-workflow/README.ko.md) | Gin API that runs fulfillment steps and reverses completed side effects when later workflow steps fail. | `workflow`, `workreport` |
| [`examples/order-fulfillment-integration`](examples/order-fulfillment-integration) | [English](examples/order-fulfillment-integration/README.md) \| [한국어](examples/order-fulfillment-integration/README.ko.md) | Milestone integration Gin API that combines order state transitions, fulfillment workflow execution, report projection, and compensation. | `state`, `workflow`, `workreport` |
| [`examples/operations-report-policy`](examples/operations-report-policy) | [English](examples/operations-report-policy/README.md) \| [한국어](examples/operations-report-policy/README.ko.md) | Gin API that turns work reports and failure policies into deterministic operations output. | `workreport` |

## Run the Leader Example

Start Redis locally, then run the service:

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-redis-web
```

Useful endpoints:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/leader
curl -X POST http://localhost:8080/campaign
curl -X POST http://localhost:8080/resign
```

## Run the Resilience Example

Start a catalog service locally, then run the service:

```bash
export CATALOG_URL=http://localhost:9090
go run ./examples/resilience-http-web
```

Useful endpoints:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/catalog/book-1
curl -X POST 'http://localhost:8081/orders?delay=25ms'
curl http://localhost:8081/events
```

## Run the Leader Group Example

Start Redis locally, then run the service:

```bash
export REDIS_ADDR=localhost:6379
export MAX_LEADERS=2
go run ./examples/leader-group-web
```

Useful endpoints:

```bash
curl http://localhost:8082/healthz
curl http://localhost:8082/group
curl -X POST http://localhost:8082/campaign
curl -X POST http://localhost:8082/resign
```

## Run the Order Lifecycle State API Example

Run the service locally:

```bash
go run ./examples/order-lifecycle-state-api
```

Useful endpoints:

```bash
curl http://localhost:8083/healthz
curl http://localhost:8083/orders/current
curl http://localhost:8083/orders/current/transitions/pay/can
curl -X POST http://localhost:8083/orders/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"submit"}'
```

## Run the Fulfillment Workflow Runner Example

Run the service locally:

```bash
go run ./examples/fulfillment-workflow-runner
```

Useful endpoints:

```bash
curl http://localhost:8084/healthz
curl -X POST http://localhost:8084/fulfillment/run \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1001","stock_available":true,"payment_authorized":true,"requires_shipment":true}'
```

## Run the Payment Authorization State Example

Run the service locally:

```bash
go run ./examples/payment-authorization-state
```

Useful endpoints:

```bash
curl http://localhost:8086/healthz
curl http://localhost:8086/payments/current
curl -X POST http://localhost:8086/payments/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"authorize","idempotency_key":"auth-1"}'
```

## Run the Compensation Workflow Example

Run the service locally:

```bash
go run ./examples/compensation-workflow
```

Useful endpoints:

```bash
curl http://localhost:8087/healthz
curl -X POST http://localhost:8087/compensation/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1001","stock_available":true,"payment_authorized":true,"shipment_provider_available":false}'
```

## Run the Order Fulfillment Integration Example

Run the service locally:

```bash
go run ./examples/order-fulfillment-integration
```

Useful endpoints:

```bash
curl http://localhost:8088/healthz
curl -X POST http://localhost:8088/orders/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1001","total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":false}'
```

## Run the Operations Report Policy Example

Run the service locally:

```bash
go run ./examples/operations-report-policy
```

Useful endpoints:

```bash
curl http://localhost:8085/healthz
curl -X POST http://localhost:8085/operations/report \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"release-1001","policy":"continue_on_failure","products_valid":true}'
```

## Development

Common commands:

```bash
make ci
```

| Command | Description |
|---|---|
| `make fmt` | Format Go sources with `gofmt`. |
| `make fmt-check` | Fail when Go sources are not `gofmt`-formatted. |
| `make tidy-check` | Fail when `go.mod` or `go.sum` drift after `go mod tidy`. |
| `make vet` | Run `go vet ./...`. |
| `make lint` | Run `golangci-lint run ./...`. |
| `make test` | Run `go test -count=1 ./...` so Testcontainers tests execute. |
| `make race` | Run `go test -race -count=1 ./...` so Testcontainers tests execute under the race detector. |
| `make ci` | Run the local CI gate. |

The integration tests use Testcontainers and require Docker. Regular CI and
Nightly workflows run these tests against real containers.

## Roadmap

| bluetape-go milestone | Workshop example direction |
|---|---|
| `0.1.0` | Redis leader election web service. |
| `0.1.1` | Focused retry and timeout examples for quality-closure resilience primitives. |
| `0.2.0` | Resilience examples for HTTP clients, services, payment authorization guards, and bounded leader group coordination. |
| `0.3.0` | Near-cache, Redis invalidation, and stampede coordination examples. |
| `0.4.0` | State and workflow examples, including Gin order lifecycle and payment authorization state APIs, fulfillment workflow runner, compensation workflow, operations report policy APIs, and an order fulfillment integration example. |
| `0.5.0` | Batch processing examples. |
