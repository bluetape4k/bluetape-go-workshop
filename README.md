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

Use the map as a learning route selector. Each card points to runnable example
modules, and each module README owns the deeper scenario, architecture,
sequence, or flow diagrams for that behavior.

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
| [`examples/safe-decompression-upload`](examples/safe-decompression-upload) | [English](examples/safe-decompression-upload/README.md) \| [한국어](examples/safe-decompression-upload/README.ko.md) | Gin upload API that bounds compressed requests and decompressed payloads for untrusted input. | `compression` |
| [`examples/measured-shipping-quote`](examples/measured-shipping-quote) | [English](examples/measured-shipping-quote/README.md) \| [한국어](examples/measured-shipping-quote/README.ko.md) | Gin shipping quote API that parses caller-facing units, derives area/volume, and selects billable weight. | `measure` |
| [`examples/sql-access-strategy-decision`](examples/sql-access-strategy-decision) | [English](examples/sql-access-strategy-decision/README.md) \| [한국어](examples/sql-access-strategy-decision/README.ko.md) | Local SQL access strategy example that compares direct `database/sql`, `sqlkit`, generated query boundaries, and migration tooling choices. | `sqlkit`, `testcontainers/postgres` |
| [`examples/order-intake-cleanup`](examples/order-intake-cleanup) | [English](examples/order-intake-cleanup/README.md) \| [한국어](examples/order-intake-cleanup/README.ko.md) | Partner order feed cleanup with validation, defaults, filtering, deduplication, and grouping. | `core`, `collections` |
| [`examples/invitation-codecs`](examples/invitation-codecs) | [English](examples/invitation-codecs/README.md) \| [한국어](examples/invitation-codecs/README.ko.md) | Invitation links, callback state, and partner references with practical string codecs. | `codec`, `core` |
| [`examples/id-jwt-boundary`](examples/id-jwt-boundary) | [English](examples/id-jwt-boundary/README.md) \| [한국어](examples/id-jwt-boundary/README.ko.md) | Gin order intake boundary that verifies JWT claims and generates internal UUID v7 order IDs. | `id`, `jwt` |
| [`examples/token-refresh-claims`](examples/token-refresh-claims) | [English](examples/token-refresh-claims/README.md) \| [한국어](examples/token-refresh-claims/README.ko.md) | Gin token boundary that separates access-token claims from refresh-token exchange claims. | `jwt` |
| [`examples/distributed-jwt-key-rotation`](examples/distributed-jwt-key-rotation) | [English](examples/distributed-jwt-key-rotation/README.md) \| [한국어](examples/distributed-jwt-key-rotation/README.ko.md) | Redis-backed distributed JWT key rotation with retained `kid` verification and cached reader revalidation. | `jwt`, `jwt/redis`, `cache`, `testcontainers/redis` |
| [`examples/money-rule-pricing`](examples/money-rule-pricing) | [English](examples/money-rule-pricing/README.md) \| [한국어](examples/money-rule-pricing/README.ko.md) | Gin cart pricing API with decimal-backed money values, rounded totals, accepted discounts, and rejected rule decisions. | `money` |
| [`examples/exchange-rate-pricing`](examples/exchange-rate-pricing) | [English](examples/exchange-rate-pricing/README.md) \| [한국어](examples/exchange-rate-pricing/README.ko.md) | Gin display-pricing API that converts base totals through provider-backed exchange rates, locale currency defaults, and explicit stale quote policy. | `money` |
| [`examples/multi-currency-invoice-rules`](examples/multi-currency-invoice-rules) | [English](examples/multi-currency-invoice-rules/README.md) \| [한국어](examples/multi-currency-invoice-rules/README.ko.md) | Gin invoice API that groups money totals by currency and exposes discount and tax-like rule decisions. | `money` |
| [`examples/probabilistic-dedupe-admission`](examples/probabilistic-dedupe-admission) | [English](examples/probabilistic-dedupe-admission/README.md) \| [한국어](examples/probabilistic-dedupe-admission/README.ko.md) | Gin webhook admission API that uses a Bloom filter for definitely-new and probably-seen event paths. | `probabilistic` |
| [`examples/shared-redis-bloom-admission`](examples/shared-redis-bloom-admission) | [English](examples/shared-redis-bloom-admission/README.md) \| [한국어](examples/shared-redis-bloom-admission/README.ko.md) | Gin webhook admission API that shares Redis-backed Bloom state across API instances. | `probabilistic`, `probabilistic/redis`, `testcontainers/redis` |
| [`examples/checkout-guard-integration`](examples/checkout-guard-integration) | [English](examples/checkout-guard-integration/README.md) \| [한국어](examples/checkout-guard-integration/README.ko.md) | Gin checkout guard API that composes JWT claims, IDs, money/rules, and probabilistic repeated-submission admission. | `id`, `jwt`, `money`, `probabilistic` |
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
| [`examples/chunked-csv-import-checkpoint`](examples/chunked-csv-import-checkpoint) | [English](examples/chunked-csv-import-checkpoint/README.md) \| [한국어](examples/chunked-csv-import-checkpoint/README.ko.md) | Local batch job that imports CSV rows in chunks, persists checkpoints, restarts after a partial writer crash, and skips duplicate boundary rows. | `batch` |
| [`examples/account-migration-checkpoint-restart`](examples/account-migration-checkpoint-restart) | [English](examples/account-migration-checkpoint-restart/README.md) \| [한국어](examples/account-migration-checkpoint-restart/README.ko.md) | Local account migration batch job that fails at a known account, restarts from a saved checkpoint, and proves the completed chunk is not reprocessed. | `batch` |
| [`examples/retry-dead-letter-batch-worker`](examples/retry-dead-letter-batch-worker) | [English](examples/retry-dead-letter-batch-worker/README.md) \| [한국어](examples/retry-dead-letter-batch-worker/README.ko.md) | Local batch worker that retries transient ticket failures and records permanent ticket failures as dead letters. | `batch` |
| [`examples/customer-migration-batch-integration`](examples/customer-migration-batch-integration) | [English](examples/customer-migration-batch-integration/README.md) \| [한국어](examples/customer-migration-batch-integration/README.ko.md) | Milestone Gin API that combines checkpoint restart, retry/dead-letter handling, leader-guarded scheduling, status/report inspection, and active-run cancellation. | `batch`, `leader` |

## Run the SQL Access Strategy Decision Example

Print the local SQL strategy report:

```bash
go run ./examples/sql-access-strategy-decision
```

The example compares direct `database/sql` and `sqlkit` against the same
order-hold repository flow, then documents when to move generated query code or
schema migrations outside the runtime dependency boundary.

Run the PostgreSQL Testcontainers contract tests:

```bash
go test -count=1 ./examples/sql-access-strategy-decision/...
```

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

## Run the Chunked CSV Import Checkpoint Example

Run the local batch demonstration:

```bash
go run ./examples/chunked-csv-import-checkpoint
```

The first run fails after partially committing the second chunk and leaves the
checkpoint at `next_row=2`. The restart run restores that checkpoint, skips the
duplicate customer at the chunk boundary, and completes with `next_row=5`.

## Run the Account Migration Checkpoint Restart Example

Run the local batch demonstration:

```bash
go run ./examples/account-migration-checkpoint-restart
```

The first run writes `acct-1001` and `acct-1002`, saves
`next_index=2`, and fails on `acct-1003`. The restart run restores that cursor,
reads only `acct-1003` through `acct-1005`, and completes with `next_index=5`.

## Run the Retry Dead-Letter Batch Worker Example

Run the local batch demonstration:

```bash
go run ./examples/retry-dead-letter-batch-worker
```

The run retries `ticket-1002` once, records `ticket-1003` in the dead-letter
list, skips that permanent item, and completes with `read=4`, `write=3`,
`retry=1`, and `skip=1`.

## Run the Customer Migration Batch Integration Example

Run the local operations API:

```bash
go run ./examples/customer-migration-batch-integration
```

Useful endpoints:

```bash
curl http://127.0.0.1:8095/healthz
curl -X POST http://127.0.0.1:8095/batch/start \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"manual-001","crash_after_new_writes":3}'
curl http://127.0.0.1:8095/batch/status
curl -X POST http://127.0.0.1:8095/batch/schedule/tick \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"scheduled-001"}'
curl http://127.0.0.1:8095/batch/report
```

Set `LEADER_MODE=missing` to reproduce `not_leader`. `HTTP_ADDR` may point to
another loopback bind such as `127.0.0.1:8096`; non-loopback binds are rejected
because the workshop API is unauthenticated.

## Run the ID and JWT Boundary Example

Run the local order intake API:

```bash
go run ./examples/id-jwt-boundary
```

Useful endpoints:

```bash
curl http://127.0.0.1:8096/healthz
TOKEN=$(
  curl -s -X POST http://127.0.0.1:8096/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["orders:create"],"ttl_seconds":900}' \
  | jq -r '.token'
)
curl -X POST http://127.0.0.1:8096/orders \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"sku":"sku-blue-tape","quantity":2}'
```

The example demonstrates that UUID v7 values are internal identifiers, not
bearer secrets, and that signed JWT claims are verified but not encrypted.

## Run the Token Refresh Claims Example

This example builds on the base [ID and JWT Boundary](examples/id-jwt-boundary)
example from #44 and focuses on access-token versus refresh-token claim
contracts.

```bash
go run ./examples/token-refresh-claims
```

Useful endpoints:

```bash
curl http://127.0.0.1:8097/healthz
SESSION=$(
  curl -s -X POST http://127.0.0.1:8097/sessions \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["profile:read"],"ttl_seconds":300}'
)
ACCESS_TOKEN=$(printf '%s' "${SESSION}" | jq -r '.access_token')
REFRESH_TOKEN=$(printf '%s' "${SESSION}" | jq -r '.refresh_token')
curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
curl -X POST http://127.0.0.1:8097/tokens/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refresh_token\":\"${REFRESH_TOKEN}\"}"
```

The example demonstrates that signed refresh tokens still need operation-specific
claim checks; signature validity alone is not permission to call every endpoint.

## Run the Money Rule Pricing Example

Run the local cart pricing API:

```bash
go run ./examples/money-rule-pricing
```

Useful endpoints:

```bash
curl http://127.0.0.1:8098/healthz
curl -s -X POST http://127.0.0.1:8098/quotes \
  -H 'Content-Type: application/json' \
  -d '{"cart_id":"cart-1001","currency":"USD","customer_tier":"vip","coupon_code":"SAVE10","items":[{"sku":"book-1","unit_price":"19.995","currency":"USD","quantity":2},{"sku":"pen-1","unit_price":"2.50","currency":"USD","quantity":1}]}' | jq
```

The example demonstrates why money values stay as strings plus explicit
currency, and why `float64` is not used for cart totals.

## Run the Exchange-Rate Pricing Example

This example builds on the `money-rule-pricing` lesson by keeping the base cart
subtotal in `USD`, selecting a buyer display currency from locale, and converting
the final display total through a provider-backed exchange-rate quote.

Run the local display-pricing API:

```bash
go run ./examples/exchange-rate-pricing
```

Useful endpoints:

```bash
curl http://127.0.0.1:8101/healthz
curl -s -X POST http://127.0.0.1:8101/quotes \
  -H 'Content-Type: application/json' \
  -d '{"quote_id":"quote-1001","base_currency":"USD","locale":"ko-KR","items":[{"sku":"pro-plan","unit_price":"19.995","currency":"USD","quantity":2},{"sku":"support","unit_price":"5.00","currency":"USD","quantity":1}]}' | jq
```

The response keeps `subtotal` and `display_total` separate, exposes rate source
and freshness metadata, and rejects stale quotes unless the caller explicitly
sets `allow_stale_quote`.

## Run the Multi-Currency Invoice Rule Example

This example builds on #45's base
[`money-rule-pricing`](examples/money-rule-pricing/README.md) lesson by keeping
money values explicit while grouping invoice totals by currency. It does not
perform exchange-rate conversion.

Run the local invoice evaluation API:

```bash
go run ./examples/multi-currency-invoice-rules
```

Useful endpoints:

```bash
curl http://127.0.0.1:8100/healthz
curl -s -X POST http://127.0.0.1:8100/invoices/evaluate \
  -H 'Content-Type: application/json' \
  -d '{"invoice_id":"inv-1001","customer_tier":"vip","region":"EU","lines":[{"line_id":"svc-usd","amount":"19.995","currency":"USD","quantity":2,"category":"service"},{"line_id":"goods-eur","amount":"10.00","currency":"EUR","quantity":1,"category":"goods"},{"line_id":"goods-jpy","amount":"100.60","currency":"JPY","quantity":1,"category":"tax_exempt"}]}' | jq
```

The response keeps `USD`, `EUR`, and `JPY` totals separate, makes the VIP
service discount and regional VAT decisions visible, and sets
`conversion_applied` to `false`.

## Run the Probabilistic Dedupe Admission Example

Run the local webhook admission API:

```bash
go run ./examples/probabilistic-dedupe-admission
```

Useful endpoints:

```bash
curl http://127.0.0.1:8099/healthz
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

The example demonstrates that a Bloom filter can prove an event is definitely
new, but a hit is only `probably_seen` and still needs durable-store pairing in
production.

## Run the Shared Redis Bloom Admission Example

Start Redis locally, then run the webhook admission API:

```bash
export REDIS_ADDR=127.0.0.1:6379
go run ./examples/shared-redis-bloom-admission
```

Useful endpoints:

```bash
curl http://127.0.0.1:8102/healthz
curl -s -X POST http://127.0.0.1:8102/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
curl -s -X POST http://127.0.0.1:8102/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
curl -s http://127.0.0.1:8102/filters/current | jq
```

The example demonstrates how multiple API instances share a Redis Bloom
namespace while preserving the reader rule: `probably_seen` is not authorization
and not exact dedupe.

## Run the Safe Decompression Upload Example

Run the local upload guard API:

```bash
go run ./examples/safe-decompression-upload
```

Useful endpoints:

```bash
curl http://127.0.0.1:8103/healthz
curl http://127.0.0.1:8103/uploads/limits
printf '%s' '{"document_id":"doc-1001","tenant":"checkout","body":"invoice upload"}' \
  | gzip -c \
  | curl -s -X POST http://127.0.0.1:8103/uploads/compressed \
      -H 'X-Compression-Algorithm: gzip' \
      --data-binary @- | jq
```

The example demonstrates the difference between compressed request-size limits
and decompressed payload-size limits for untrusted upload bytes.

## Run the Measured Shipping Quote Example

Run the local measured quote API:

```bash
go run ./examples/measured-shipping-quote
```

Useful endpoints:

```bash
curl http://127.0.0.1:8104/healthz
curl -s -X POST http://127.0.0.1:8104/quotes \
  -H 'Content-Type: application/json' \
  -d '{"quote_id":"ship-1001","destination_zone":"kr-seoul","width":"40 cm","height":"30 cm","length":"20 cm","weight":"3.2 kg"}' | jq
```

The example demonstrates how `measure` keeps caller-facing units typed while the
application derives area, volume, dimensional weight, billable weight, and a
stable HTTP error policy.

## Run the Checkout Guard Integration Example

This example composes the ID/JWT, token-claim, money/rule, and probabilistic
admission lessons into one protected checkout boundary.

Run the local checkout guard API:

```bash
go run ./examples/checkout-guard-integration
```

Useful endpoints:

```bash
curl http://127.0.0.1:8101/healthz
TOKEN=$(
  curl -s -X POST http://127.0.0.1:8101/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["checkout:submit"],"ttl_seconds":300}' \
  | jq -r '.token'
)
curl -s -X POST http://127.0.0.1:8101/checkout/guard \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"checkout_id":"chk-1001","idempotency_key":"idem-1001","customer_tier":"vip","region":"EU","currency":"USD","items":[{"line_id":"svc-1","sku":"support-plan","unit_price":"19.995","currency":"USD","quantity":2,"category":"service"}]}' | jq
```

The response includes verified subject/session claims, generated request/order
IDs, rounded checkout totals, rule decisions, and a Bloom admission decision.

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
| `0.5.0` | Batch processing examples for chunked CSV checkpoint/restart, batch operations APIs, scheduled execution, retry/dead-letter behavior, and milestone integration. |
| `0.6.0` | ID and JWT examples for generated identifiers, signed request claims, token refresh boundaries, and HTTP trust-boundary handling. |

## Roadmap Planning

- [Workshop roadmap matrix](docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md)
- [Example selection scorecard](docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md)
- [Integration example template and acceptance rubric](docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md)
- [Cross-milestone integration blueprint](docs/superpowers/plans/2026-06-23-issue-81-cross-milestone-integration-blueprint.md)
