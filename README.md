# bluetape-go-workshop

[English](README.md) | [한국어](README.ko.md)

Runnable web application examples for [`bluetape-go`](https://github.com/bluetape4k/bluetape-go).

This repository keeps application-shaped examples separate from reusable library
packages. The library repository should stay focused on stable packages; this
workshop shows how those packages behave inside real HTTP services and
container-backed integration tests.

The default web style is lightweight and close to the Go standard library:
examples use [`chi`](https://github.com/go-chi/chi) when routing and middleware
are useful, while keeping handlers compatible with `net/http`.

## Example Map

![Workshop example map](docs/images/readme-diagrams/workshop-example-map.png)

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
| `0.4.0` | State and workflow examples. |
| `0.5.0` | Batch processing examples. |
