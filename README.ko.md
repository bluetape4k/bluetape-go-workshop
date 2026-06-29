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

이 map은 학습 경로를 고르는 기준입니다. 각 card는 실행 가능한 예제 module을
가리키고, 더 자세한 scenario, architecture, sequence, flow diagram은 각 module
README가 소유합니다.

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
| [`examples/safe-decompression-upload`](examples/safe-decompression-upload/README.ko.md) | [English](examples/safe-decompression-upload/README.md) \| [한국어](examples/safe-decompression-upload/README.ko.md) | Untrusted input에 대해 compressed request와 decompressed payload를 모두 제한하는 Gin upload API입니다. | `compression` |
| [`examples/measured-shipping-quote`](examples/measured-shipping-quote/README.ko.md) | [English](examples/measured-shipping-quote/README.md) \| [한국어](examples/measured-shipping-quote/README.ko.md) | Caller-facing unit을 parse하고 area/volume을 derive한 뒤 billable weight를 고르는 Gin shipping quote API입니다. | `measure` |
| [`examples/sql-access-strategy-decision`](examples/sql-access-strategy-decision/README.ko.md) | [English](examples/sql-access-strategy-decision/README.md) \| [한국어](examples/sql-access-strategy-decision/README.ko.md) | direct `database/sql`, `sqlkit`, generated query boundary, migration tooling 선택 기준을 비교하는 local SQL access strategy 예제입니다. | `sqlkit`, `testcontainers/postgres` |
| [`examples/sql-order-repository`](examples/sql-order-repository/README.ko.md) | [English](examples/sql-order-repository/README.md) \| [한국어](examples/sql-order-repository/README.ko.md) | visible `sqlkit` statement로 insert, find-by-ID, filtered list, not-found behavior를 보여주는 PostgreSQL-backed order repository 예제입니다. | `sqlkit`, `testcontainers/postgres` |
| [`examples/sql-transaction-boundary`](examples/sql-transaction-boundary/README.ko.md) | [English](examples/sql-transaction-boundary/README.md) \| [한국어](examples/sql-transaction-boundary/README.ko.md) | Stock debit과 order row를 함께 commit하거나 실패 시 함께 rollback하는 PostgreSQL-backed order placement transaction 예제입니다. | `sqlkit`, `testcontainers/postgres` |
| [`examples/gin-sql-crud-api`](examples/gin-sql-crud-api/README.ko.md) | [English](examples/gin-sql-crud-api/README.md) \| [한국어](examples/gin-sql-crud-api/README.ko.md) | HTTP parsing, public error, request timeout, sqlkit repository ownership을 분리해서 보여주는 Gin 주문 CRUD API입니다. | `sqlkit`, `gin`, `testcontainers/postgres` |
| [`examples/gin-sql-order-service`](examples/gin-sql-order-service/README.ko.md) | [English](examples/gin-sql-order-service/README.md) \| [한국어](examples/gin-sql-order-service/README.ko.md) | HTTP parsing, service-owned SQL transaction, multi-table repository, item update, rollback proof를 합친 milestone Gin order service입니다. | `sqlkit`, `gin`, `testcontainers/postgres` |
| [`examples/s3-floci-storage`](examples/s3-floci-storage/README.ko.md) | [English](examples/s3-floci-storage/README.md) \| [한국어](examples/s3-floci-storage/README.ko.md) | AWS SDK v2와 opt-in Floci smoke coverage로 tenant receipt object upload, download, list, delete, presign을 보여주는 local S3 storage 예제입니다. | `AWS SDK v2`, `testcontainers/floci` |
| [`examples/sqs-floci-worker`](examples/sqs-floci-worker/README.ko.md) | [English](examples/sqs-floci-worker/README.md) \| [한국어](examples/sqs-floci-worker/README.ko.md) | Fulfillment task enqueue, handler success delete acknowledgement, handler failure retry visibility를 Floci smoke coverage로 보여주는 local SQS worker 예제입니다. | `AWS SDK v2`, `testcontainers/floci` |
| [`examples/dynamodb-batchwrite-materializer`](examples/dynamodb-batchwrite-materializer/README.ko.md) | [English](examples/dynamodb-batchwrite-materializer/README.md) \| [한국어](examples/dynamodb-batchwrite-materializer/README.ko.md) | DynamoDB document-index materializer가 batch write를 chunking하고, `UnprocessedItems`만 retry하며, retry exhaustion과 AWS service error를 분리하는 local 예제입니다. | `dynamodb/batchwrite`, `testcontainers/floci`, `AWS SDK v2` |
| [`examples/dynamodb-conditional-repository`](examples/dynamodb-conditional-repository/README.ko.md) | [English](examples/dynamodb-conditional-repository/README.md) \| [한국어](examples/dynamodb-conditional-repository/README.ko.md) | DynamoDB catalog repository가 create-if-absent write, optimistic version update, tenant query, typed conditional conflict handling을 보여주는 local 예제입니다. | `AWS SDK v2`, `testcontainers/floci` |
| [`examples/s3-sqs-dynamodb-document-workflow`](examples/s3-sqs-dynamodb-document-workflow/README.ko.md) | [English](examples/s3-sqs-dynamodb-document-workflow/README.md) \| [한국어](examples/s3-sqs-dynamodb-document-workflow/README.ko.md) | S3 object 저장, SQS event 발행, DynamoDB idempotent processing state 기록을 하나로 합친 local document workflow 예제입니다. | `AWS SDK v2`, `testcontainers/floci` |
| [`examples/order-intake-cleanup`](examples/order-intake-cleanup/README.ko.md) | [English](examples/order-intake-cleanup/README.md) \| [한국어](examples/order-intake-cleanup/README.ko.md) | validation, default, filtering, deduplication, grouping으로 partner order feed를 정리하는 예제입니다. | `core`, `collections` |
| [`examples/invitation-codecs`](examples/invitation-codecs/README.ko.md) | [English](examples/invitation-codecs/README.md) \| [한국어](examples/invitation-codecs/README.ko.md) | invitation link, callback state, partner reference를 실용적인 string codec으로 다루는 예제입니다. | `codec`, `core` |
| [`examples/id-jwt-boundary`](examples/id-jwt-boundary/README.ko.md) | [English](examples/id-jwt-boundary/README.md) \| [한국어](examples/id-jwt-boundary/README.ko.md) | JWT claim을 검증하고 내부 UUID v7 order ID를 생성하는 Gin order intake boundary 예제입니다. | `id`, `jwt` |
| [`examples/token-refresh-claims`](examples/token-refresh-claims/README.ko.md) | [English](examples/token-refresh-claims/README.md) \| [한국어](examples/token-refresh-claims/README.ko.md) | Access-token claim과 refresh-token exchange claim을 분리하는 Gin token boundary 예제입니다. | `jwt` |
| [`examples/distributed-jwt-key-rotation`](examples/distributed-jwt-key-rotation/README.ko.md) | [English](examples/distributed-jwt-key-rotation/README.md) \| [한국어](examples/distributed-jwt-key-rotation/README.ko.md) | Redis-backed distributed JWT key rotation, retained `kid` 검증, cached reader revalidation 예제입니다. | `jwt`, `jwt/redis`, `cache`, `testcontainers/redis` |
| [`examples/money-rule-pricing`](examples/money-rule-pricing/README.ko.md) | [English](examples/money-rule-pricing/README.md) \| [한국어](examples/money-rule-pricing/README.ko.md) | Decimal-backed money value, rounded total, accepted discount, rejected rule decision을 보여주는 Gin cart pricing API입니다. | `money` |
| [`examples/exchange-rate-pricing`](examples/exchange-rate-pricing/README.ko.md) | [English](examples/exchange-rate-pricing/README.md) \| [한국어](examples/exchange-rate-pricing/README.ko.md) | Provider-backed exchange rate, locale currency default, explicit stale quote policy로 base total을 display total로 변환하는 Gin API입니다. | `money` |
| [`examples/multi-currency-invoice-rules`](examples/multi-currency-invoice-rules/README.ko.md) | [English](examples/multi-currency-invoice-rules/README.md) \| [한국어](examples/multi-currency-invoice-rules/README.ko.md) | Money total을 currency별로 group하고 discount와 tax-like rule decision을 노출하는 Gin invoice API입니다. | `money` |
| [`examples/probabilistic-dedupe-admission`](examples/probabilistic-dedupe-admission/README.ko.md) | [English](examples/probabilistic-dedupe-admission/README.md) \| [한국어](examples/probabilistic-dedupe-admission/README.ko.md) | Bloom filter로 definitely-new와 probably-seen event path를 보여주는 Gin webhook admission API입니다. | `probabilistic` |
| [`examples/shared-redis-bloom-admission`](examples/shared-redis-bloom-admission/README.ko.md) | [English](examples/shared-redis-bloom-admission/README.md) \| [한국어](examples/shared-redis-bloom-admission/README.ko.md) | Redis-backed Bloom state를 여러 API instance가 공유하는 Gin webhook admission API입니다. | `probabilistic`, `probabilistic/redis`, `testcontainers/redis` |
| [`examples/checkout-guard-integration`](examples/checkout-guard-integration/README.ko.md) | [English](examples/checkout-guard-integration/README.md) \| [한국어](examples/checkout-guard-integration/README.ko.md) | JWT claim, ID, money/rule, probabilistic repeated-submission admission을 조합하는 Gin checkout guard API입니다. | `id`, `jwt`, `money`, `probabilistic` |
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
| [`examples/order-fulfillment-integration`](examples/order-fulfillment-integration/README.ko.md) | [English](examples/order-fulfillment-integration/README.md) \| [한국어](examples/order-fulfillment-integration/README.ko.md) | Order state transition, fulfillment workflow execution, report projection, compensation을 합친 milestone 통합 Gin API입니다. | `state`, `workflow`, `workreport` |
| [`examples/operations-report-policy`](examples/operations-report-policy/README.ko.md) | [English](examples/operations-report-policy/README.md) \| [한국어](examples/operations-report-policy/README.ko.md) | Work report와 failure policy를 deterministic operations output으로 투영하는 Gin API입니다. | `workreport` |
| [`examples/chunked-csv-import-checkpoint`](examples/chunked-csv-import-checkpoint/README.ko.md) | [English](examples/chunked-csv-import-checkpoint/README.md) \| [한국어](examples/chunked-csv-import-checkpoint/README.ko.md) | CSV row를 chunk 단위로 import하고 checkpoint 저장, partial writer crash 이후 restart, boundary duplicate skip을 보여주는 local batch job입니다. | `batch` |
| [`examples/account-migration-checkpoint-restart`](examples/account-migration-checkpoint-restart/README.ko.md) | [English](examples/account-migration-checkpoint-restart/README.md) \| [한국어](examples/account-migration-checkpoint-restart/README.ko.md) | Account migration batch job이 지정 account에서 실패한 뒤 저장된 checkpoint부터 재시작하고 완료된 chunk를 다시 처리하지 않음을 증명합니다. | `batch` |
| [`examples/retry-dead-letter-batch-worker`](examples/retry-dead-letter-batch-worker/README.ko.md) | [English](examples/retry-dead-letter-batch-worker/README.md) \| [한국어](examples/retry-dead-letter-batch-worker/README.ko.md) | Transient ticket failure는 retry하고 permanent ticket failure는 dead letter로 기록하는 local batch worker입니다. | `batch` |
| [`examples/customer-migration-batch-integration`](examples/customer-migration-batch-integration/README.ko.md) | [English](examples/customer-migration-batch-integration/README.md) \| [한국어](examples/customer-migration-batch-integration/README.ko.md) | Checkpoint restart, retry/dead-letter handling, leader-guarded scheduling, status/report 조회, active-run cancellation을 합친 milestone Gin API입니다. | `batch`, `leader` |

## SQL Access Strategy Decision 예제 실행

Local SQL strategy report를 출력합니다.

```bash
go run ./examples/sql-access-strategy-decision
```

이 예제는 같은 order-hold repository flow를 direct `database/sql`과 `sqlkit`으로
비교하고, generated query code나 schema migration을 언제 runtime dependency 밖의
boundary로 옮겨야 하는지 설명합니다.

PostgreSQL Testcontainers contract test를 실행합니다.

```bash
go test -count=1 ./examples/sql-access-strategy-decision/...
```

## SQL Order Repository 예제 실행

Local repository preview를 출력합니다.

```bash
go run ./examples/sql-order-repository
```

이 예제는 order persist, primary key read, customer/status list를 보여주고,
`sqlkit`이 ORM이나 migration owner가 되지 않으면서 direct `database/sql`을 어떻게
보완하는지 문서화합니다.

PostgreSQL Testcontainers contract test를 실행합니다.

```bash
go test -count=1 ./examples/sql-order-repository/...
```

## SQL Transaction Boundary 예제 실행

Local transaction preview를 출력합니다.

```bash
go run ./examples/sql-transaction-boundary
```

이 예제는 service가 명시적으로 소유하는 하나의 transaction 안에서 order를 생성하고,
product stock debit과 order row insert를 함께 처리하며, stock conflict, payment
rejection, context cancellation에서 rollback을 증명합니다.

PostgreSQL Testcontainers contract test를 실행합니다.

```bash
go test -count=1 ./examples/sql-transaction-boundary/...
```

## Gin SQL CRUD API 예제 실행

Database 없이 API/repository preview를 출력합니다.

```bash
go run ./examples/gin-sql-crud-api
```

`DATABASE_URL`을 설정하면 PostgreSQL을 대상으로 HTTP service를 실행합니다. 예제는
local demo table을 시작 시 생성하지만, production service는 schema 변경을 별도
migration owner로 옮겨야 합니다.

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable'
go run ./examples/gin-sql-crud-api
```

PostgreSQL-backed handler/repository test를 실행합니다.

```bash
go test -count=1 ./examples/gin-sql-crud-api/...
```

## Gin SQL Order Service 예제 실행

Database 없이 order service preview를 출력합니다.

```bash
go run ./examples/gin-sql-order-service
```

이 예제는 SQL repository, SQL transaction boundary, Gin CRUD lesson을 하나로
합칩니다. Gin은 HTTP parsing과 public error를 소유하고, service는
`sqlkit.WithTx`를 소유하며, repository는 order header, item, status-event SQL을
소유합니다. `reject_after_items` request는 payment, inventory, fulfillment,
outbox scope를 추가하지 않고 item insert 이후 rollback을 증명하는 deterministic
failure switch입니다.

`DATABASE_URL`을 설정하면 PostgreSQL을 대상으로 HTTP service를 실행합니다. 예제는
local demo table을 시작 시 생성하지만, production service는 schema 변경을 별도
migration owner로 옮겨야 합니다.

```bash
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable'
go run ./examples/gin-sql-order-service
```

PostgreSQL-backed handler/service/repository test를 실행합니다.

```bash
go test -count=1 ./examples/gin-sql-order-service/...
```

## S3 Floci Storage 예제 실행

Local storage preview를 출력합니다.

```bash
go run ./examples/s3-floci-storage
```

이 예제는 upload, download, tenant-prefix listing, delete, presigned download
URL을 모델링하고, Floci local endpoint가 real AWS deployment와 어떻게 다른지
문서화합니다.

Deterministic storage test를 실행합니다.

```bash
go test -count=1 ./examples/s3-floci-storage/...
```

Docker가 있을 때 opt-in Floci S3 smoke test를 serial로 실행합니다.

```bash
BLUETAPE_S3_FLOCI_STORAGE_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/s3-floci-storage/...
```

## SQS Floci Worker 예제 실행

Local worker preview를 출력합니다.

```bash
go run ./examples/sqs-floci-worker
```

이 예제는 enqueue, receive, handler success acknowledgement, handler failure
retry visibility를 모델링하고, SQS at-least-once delivery와 handler idempotency
요구사항을 문서화합니다.

Deterministic worker test를 실행합니다.

```bash
go test -count=1 ./examples/sqs-floci-worker/...
```

Docker가 있을 때 opt-in Floci SQS smoke test를 serial로 실행합니다.

```bash
BLUETAPE_SQS_FLOCI_WORKER_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/sqs-floci-worker/...
```

## DynamoDB Batch Write Materializer 예제 실행

Local batch-write preview를 출력합니다.

```bash
go run ./examples/dynamodb-batchwrite-materializer
```

이 예제는 30개 document event를 DynamoDB `WriteRequest`로 변환하고, `25 + 5`
chunk boundary를 보여주며, `batchwrite.WriteAll`이 `UnprocessedItems` retry
exhaustion과 AWS SDK typed service error를 어떻게 분리하는지 설명합니다.

Deterministic retry와 error-policy test를 실행합니다.

```bash
go test -count=1 ./examples/dynamodb-batchwrite-materializer/...
```

Docker가 있을 때 opt-in Floci DynamoDB smoke test를 serial로 실행합니다.

```bash
BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/dynamodb-batchwrite-materializer/...
```

## DynamoDB Conditional Repository 예제 실행

Local repository preview를 출력합니다.

```bash
go run ./examples/dynamodb-conditional-repository
```

이 예제는 create-if-absent write, optimistic version update, tenant partition query를
모델링하고, typed DynamoDB conditional conflict error를 application-level
`ErrConditionalConflict` 뒤에 보존합니다.

Deterministic repository test를 실행합니다.

```bash
go test -count=1 ./examples/dynamodb-conditional-repository/...
```

Docker가 있을 때 opt-in Floci DynamoDB smoke test를 serial로 실행합니다.

```bash
BLUETAPE_DYNAMODB_CONDITIONAL_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/dynamodb-conditional-repository/...
```

## S3-SQS-DynamoDB Document Workflow 예제 실행

Local document workflow preview를 출력합니다.

```bash
go run ./examples/s3-sqs-dynamodb-document-workflow
```

이 예제는 작은 S3, SQS, DynamoDB lesson을 하나의 local document ingestion flow로
합칩니다. S3에 bytes를 저장하고, SQS `DocumentEvent`를 발행하며, worker가 S3
object를 읽고 DynamoDB idempotency record를 만든 뒤 SQS message를
acknowledge합니다.

Deterministic workflow test를 실행합니다.

```bash
go test -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...
```

Docker가 있을 때 optional multi-service Floci smoke test를 serial로 실행합니다.

```bash
BLUETAPE_DOCUMENT_WORKFLOW_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...
```

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

## Order Fulfillment Integration 예제 실행

Service를 로컬에서 실행합니다:

```bash
go run ./examples/order-fulfillment-integration
```

주요 endpoint:

```bash
curl http://localhost:8088/healthz
curl -X POST http://localhost:8088/orders/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1001","total_cents":2599,"stock_available":true,"payment_authorized":true,"shipment_provider_available":false}'
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

## Chunked CSV Import Checkpoint 예제 실행

Local batch demonstration을 실행합니다:

```bash
go run ./examples/chunked-csv-import-checkpoint
```

첫 실행은 두 번째 chunk를 부분 commit한 뒤 실패하고 checkpoint를 `next_row=2`에
둡니다. Restart 실행은 이 checkpoint를 restore하고, chunk boundary의 duplicate
customer를 skip한 뒤 `next_row=5`로 완료합니다.

## Account Migration Checkpoint Restart 예제 실행

Local batch demonstration을 실행합니다:

```bash
go run ./examples/account-migration-checkpoint-restart
```

첫 실행은 `acct-1001`, `acct-1002`를 write하고 `next_index=2`를 저장한 뒤
`acct-1003`에서 실패합니다. Restart 실행은 이 cursor를 restore하고
`acct-1003`부터 `acct-1005`까지만 읽은 뒤 `next_index=5`로 완료합니다.

## Retry Dead-Letter Batch Worker 예제 실행

Local batch demonstration을 실행합니다:

```bash
go run ./examples/retry-dead-letter-batch-worker
```

실행은 `ticket-1002`를 한 번 retry하고, `ticket-1003`을 dead-letter list에
기록한 뒤 permanent item으로 skip합니다. 최종 report는 `read=4`, `write=3`,
`retry=1`, `skip=1`로 완료됩니다.

## Customer Migration Batch Integration 예제 실행

Local operations API를 실행합니다:

```bash
go run ./examples/customer-migration-batch-integration
```

주요 endpoint:

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

`LEADER_MODE=missing`을 설정하면 `not_leader`를 재현할 수 있습니다. `HTTP_ADDR`는
`127.0.0.1:8096` 같은 다른 loopback bind로 바꿀 수 있습니다. 이 workshop API는
인증이 없으므로 non-loopback bind는 거부합니다.

## ID and JWT Boundary 예제 실행

Local order intake API를 실행합니다:

```bash
go run ./examples/id-jwt-boundary
```

주요 endpoint:

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

이 예제는 UUID v7 값이 내부 identifier이지 bearer secret이 아니며, signed JWT
claim은 검증되지만 암호화된 값은 아니라는 boundary를 보여줍니다.

## Token Refresh Claims 예제 실행

이 예제는 #44의 기본 [ID and JWT Boundary](examples/id-jwt-boundary/README.ko.md)
예제를 기반으로 하며, access-token claim과 refresh-token claim contract의 차이에
초점을 둡니다.

```bash
go run ./examples/token-refresh-claims
```

주요 endpoint:

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

이 예제는 signed refresh token에도 operation-specific claim check가 필요하다는
점을 보여줍니다. Signature가 유효하다고 모든 endpoint 호출 권한이 생기지는 않습니다.

## Money Rule Pricing 예제 실행

Local cart pricing API를 실행합니다:

```bash
go run ./examples/money-rule-pricing
```

주요 endpoint:

```bash
curl http://127.0.0.1:8098/healthz
curl -s -X POST http://127.0.0.1:8098/quotes \
  -H 'Content-Type: application/json' \
  -d '{"cart_id":"cart-1001","currency":"USD","customer_tier":"vip","coupon_code":"SAVE10","items":[{"sku":"book-1","unit_price":"19.995","currency":"USD","quantity":2},{"sku":"pen-1","unit_price":"2.50","currency":"USD","quantity":1}]}' | jq
```

이 예제는 money value를 explicit currency가 있는 string으로 유지하는 이유와 cart
total에 `float64`를 쓰지 않는 이유를 보여줍니다.

## Exchange-Rate Pricing 예제 실행

이 예제는 `money-rule-pricing` lesson을 이어받아 base cart subtotal은 `USD`로
유지하고, buyer locale에서 display currency를 고른 뒤 provider-backed
exchange-rate quote로 final display total을 변환합니다.

Local display-pricing API를 실행합니다:

```bash
go run ./examples/exchange-rate-pricing
```

주요 endpoint:

```bash
curl http://127.0.0.1:8101/healthz
curl -s -X POST http://127.0.0.1:8101/quotes \
  -H 'Content-Type: application/json' \
  -d '{"quote_id":"quote-1001","base_currency":"USD","locale":"ko-KR","items":[{"sku":"pro-plan","unit_price":"19.995","currency":"USD","quantity":2},{"sku":"support","unit_price":"5.00","currency":"USD","quantity":1}]}' | jq
```

Response는 `subtotal`과 `display_total`을 분리해 유지하고, rate source와 freshness
metadata를 노출하며, caller가 `allow_stale_quote`를 명시하지 않으면 stale quote를
reject합니다.

## Multi-Currency Invoice Rule 예제 실행

이 예제는 #45의 기본
[`money-rule-pricing`](examples/money-rule-pricing/README.ko.md) lesson을 이어받아
money value를 명시적으로 유지하면서 invoice total을 currency별로 group합니다.
Exchange-rate conversion은 수행하지 않습니다.

Local invoice evaluation API를 실행합니다:

```bash
go run ./examples/multi-currency-invoice-rules
```

주요 endpoint:

```bash
curl http://127.0.0.1:8100/healthz
curl -s -X POST http://127.0.0.1:8100/invoices/evaluate \
  -H 'Content-Type: application/json' \
  -d '{"invoice_id":"inv-1001","customer_tier":"vip","region":"EU","lines":[{"line_id":"svc-usd","amount":"19.995","currency":"USD","quantity":2,"category":"service"},{"line_id":"goods-eur","amount":"10.00","currency":"EUR","quantity":1,"category":"goods"},{"line_id":"goods-jpy","amount":"100.60","currency":"JPY","quantity":1,"category":"tax_exempt"}]}' | jq
```

Response는 `USD`, `EUR`, `JPY` total을 분리해 유지하고, VIP service discount와
regional VAT decision을 보여주며, `conversion_applied`를 `false`로 둡니다.

## Probabilistic Dedupe Admission 예제 실행

Local webhook admission API를 실행합니다:

```bash
go run ./examples/probabilistic-dedupe-admission
```

주요 endpoint:

```bash
curl http://127.0.0.1:8099/healthz
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

이 예제는 Bloom filter가 event가 definitely new임은 증명할 수 있지만 hit는
`probably_seen`일 뿐이며 production에서는 durable store와 함께 써야 한다는 점을
보여줍니다.

## Shared Redis Bloom Admission 예제 실행

Redis를 먼저 실행한 뒤 webhook admission API를 실행합니다:

```bash
export REDIS_ADDR=127.0.0.1:6379
go run ./examples/shared-redis-bloom-admission
```

주요 endpoint:

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

이 예제는 여러 API instance가 Redis Bloom namespace를 공유하는 방법과
`probably_seen`을 authorization이나 exact dedupe로 읽으면 안 된다는 boundary를 함께
보여줍니다.

## Safe Decompression Upload 예제 실행

Local upload guard API를 실행합니다:

```bash
go run ./examples/safe-decompression-upload
```

주요 endpoint:

```bash
curl http://127.0.0.1:8103/healthz
curl http://127.0.0.1:8103/uploads/limits
printf '%s' '{"document_id":"doc-1001","tenant":"checkout","body":"invoice upload"}' \
  | gzip -c \
  | curl -s -X POST http://127.0.0.1:8103/uploads/compressed \
      -H 'X-Compression-Algorithm: gzip' \
      --data-binary @- | jq
```

이 예제는 untrusted upload byte를 다룰 때 compressed request-size limit과
decompressed payload-size limit을 별도로 둬야 하는 이유를 보여줍니다.

## Measured Shipping Quote 예제 실행

Local measured quote API를 실행합니다:

```bash
go run ./examples/measured-shipping-quote
```

주요 endpoint:

```bash
curl http://127.0.0.1:8104/healthz
curl -s -X POST http://127.0.0.1:8104/quotes \
  -H 'Content-Type: application/json' \
  -d '{"quote_id":"ship-1001","destination_zone":"kr-seoul","width":"40 cm","height":"30 cm","length":"20 cm","weight":"3.2 kg"}' | jq
```

이 예제는 `measure`가 caller-facing unit을 typed value로 유지하고, application이 area,
volume, dimensional weight, billable weight, stable HTTP error policy를 만드는 흐름을
보여줍니다.

## Checkout Guard Integration 예제 실행

이 예제는 ID/JWT, token claim, money/rule, probabilistic admission lesson을 하나의
protected checkout boundary로 조합합니다.

Local checkout guard API를 실행합니다:

```bash
go run ./examples/checkout-guard-integration
```

주요 endpoint:

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

Response는 검증된 subject/session claim, 생성된 request/order ID, rounded checkout
total, rule decision, Bloom admission decision을 포함합니다.

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
| `0.4.0` | Gin order lifecycle과 payment authorization state API, fulfillment workflow runner, compensation workflow, operations report policy API, order fulfillment integration을 포함한 state/workflow 예제. |
| `0.5.0` | Chunked CSV checkpoint/restart, batch operations API, scheduled execution, retry/dead-letter behavior, milestone integration을 포함한 batch processing 예제. |
| `0.6.0` | Generated identifier, signed request claim, token refresh boundary, HTTP trust-boundary handling을 다루는 ID/JWT 예제. |

## Roadmap Planning

- [Workshop roadmap matrix](docs/superpowers/plans/2026-06-23-issue-49-workshop-roadmap-matrix-plan.md)
- [Example selection scorecard](docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md)
- [Integration example template and acceptance rubric](docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md)
- [Cross-milestone integration blueprint](docs/superpowers/plans/2026-06-23-issue-81-cross-milestone-integration-blueprint.md)
