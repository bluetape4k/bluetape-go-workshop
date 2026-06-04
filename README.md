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

## Examples

| Example | Purpose | bluetape-go packages |
|---|---|---|
| [`examples/cache-snapshot-codecs`](examples/cache-snapshot-codecs) | Versioned product cache snapshots with safe serialization and compression tradeoff notes. | `serialization`, `compression` |
| [`examples/order-intake-cleanup`](examples/order-intake-cleanup) | Partner order feed cleanup with validation, defaults, filtering, deduplication, and grouping. | `core`, `collections` |
| [`examples/invitation-codecs`](examples/invitation-codecs) | Invitation links, callback state, and partner references with practical string codecs. | `codec`, `core` |
| [`examples/leader-redis-web`](examples/leader-redis-web) | Minimal chi-based HTTP service that campaigns for Redis-backed leadership and exposes leader state. | `leader`, `leader/redis`, `testcontainers/redis` |
| [`examples/leader-coordination-jobs`](examples/leader-coordination-jobs) | Migration gate and cache warmer jobs guarded by Redis leader election. | `leader`, `leader/redis`, `testing/concurrency` |
| [`examples/product-enrichment-fanout`](examples/product-enrichment-fanout) | Product detail fan-out with bounded goroutines, cancellation, panic capture, and stress tests. | `concurrency`, `testing/concurrency` |
| [`examples/order-pipeline-testcontainers`](examples/order-pipeline-testcontainers) | PostgreSQL, Redis, and NATS integration flow using repository Testcontainers fixtures. | `testcontainers/postgres`, `testcontainers/redis`, `testcontainers/nats` |
| [`examples/resilience-http-web`](examples/resilience-http-web) | HTTP service that composes retry, timeout, circuit breaker, bulkhead, and event hooks. | `resilience` |
| [`examples/leader-group-web`](examples/leader-group-web) | HTTP service for Redis-backed bounded multi-leader group election. | `leader`, `leader/redis`, `testing/concurrency` |

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
| `0.2.0` | Resilience examples for HTTP clients and services; bounded leader group coordination. |
| `0.3.0` | Near-cache and Redis invalidation examples. |
| `0.4.0` | State and workflow examples. |
| `0.5.0` | Batch processing examples. |
