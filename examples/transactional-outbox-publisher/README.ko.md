# Transactional Outbox Publisher 예제

[English](README.md) | 한국어

이 예제는 SQL transaction boundary 다음 단계를 다룹니다. 주문 row와 audit event를
하나의 PostgreSQL transaction으로 commit한 뒤, 릴리스된 `bluetape-go` relay가
commit된 record를 Redis Streams로 발행합니다. Workshop에서 outbox나 transport를
다시 구현하지 않고 `audit/sqloutbox`와 `audit/sqloutbox/redisstreams`를 직접
사용합니다.

![Transactional outbox publisher architecture](../../docs/images/readme-diagrams/transactional-outbox-publisher-architecture.png)

## 시나리오

1. `Service.Place`가 transaction을 열기 전에 order command를 검증합니다.
2. 하나의 `sqlkit.WithTx`가 order를 insert하고 `sqloutbox.Store.Enqueue`를 호출합니다.
3. PostgreSQL이 두 row를 함께 commit하거나 함께 rollback합니다.
4. Commit 이후 `sqloutbox.Relay.RunOnce`가 durable outbox row를 claim합니다.
5. 릴리스된 Redis Streams publisher가 13개 record field와 encoded audit entry를
   append합니다.
6. Relay는 `XADD` 성공 이후에만 SQL row를 published로 표시합니다.

![Transactional outbox publisher sequence](../../docs/images/readme-diagrams/transactional-outbox-publisher-sequence.png)

Retry할 때도 `event_id`와 `idempotency_key`는 바뀌지 않습니다. `attempts`는 delivery
진단 값이지 새 event identity가 아닙니다. Claim 이후 caller가 cancel하면 retry를
예약하거나 dead letter로 옮기지 않고 종료합니다. Claim된 row는 이후 lease recovery로
다시 eligible해집니다.

## 실행

버전을 고정한 local PostgreSQL과 Redis container를 시작합니다.

```bash
docker run --rm -d --name workshop-outbox-postgres \
  -e POSTGRES_DB=bluetape \
  -e POSTGRES_USER=bluetape \
  -e POSTGRES_PASSWORD=bluetape \
  -p 5432:5432 \
  postgres:16-alpine

docker run --rm -d --name workshop-outbox-redis \
  -p 6379:6379 \
  redis:7.4-alpine

until docker exec workshop-outbox-postgres pg_isready -U bluetape -d bluetape; do sleep 1; done
until docker exec workshop-outbox-redis redis-cli ping | rg -q PONG; do sleep 1; done
```

주문 한 건을 commit하고 relay batch 한 번을 실행합니다.

```bash
export DATABASE_URL='postgres://bluetape:bluetape@127.0.0.1:5432/bluetape?sslmode=disable'
export REDIS_ADDR='127.0.0.1:6379'
export REDIS_STREAM='workshop:transactional-outbox'
export ORDER_ID='order-1001'
export CUSTOMER_ID='customer-42'
export COMMAND_ID='command-1001'
export ORDER_CREATED_AT='2026-07-14T12:00:00Z'

go run ./examples/transactional-outbox-publisher
```

Command는 newline으로 끝나는 JSON object 한 개를 출력합니다.

```json
{"order":{"OrderID":"order-1001","CustomerID":"customer-42","Status":"placed","TotalCents":3700,"CreatedAt":"2026-07-14T12:00:00Z"},"relay":{"Claimed":1,"Published":1,"Failed":0,"DeadLettered":0},"stream":"workshop:transactional-outbox","event_id":"command-1001","idempotency_key":"command-1001"}
```

Transport record를 확인합니다.

```bash
docker exec workshop-outbox-redis redis-cli XRANGE workshop:transactional-outbox - +
```

각 stream message에는 다음 13개 field가 들어갑니다.

| Identity와 state | Audit envelope | Delivery |
| --- | --- | --- |
| `record_id`, `status`, `aggregate_type`, `aggregate_id`, `revision` | `event_id`, `idempotency_key`, `event_type`, `occurred_at`, `recorded_at`, `schema_version`, `entry_json` | `attempts` |

`entry_json`을 decode한 aggregate, revision, event identity, event type, schema
version은 scalar field와 같습니다. Integration test가 실제 PostgreSQL과 Redis
container를 대상으로 이 parity를 검증합니다.

Order, command, stream identity는 durable합니다. 같은 container를 두고 다시 실행할
때는 새 identity를 지정합니다.

```bash
ORDER_ID='order-1002' COMMAND_ID='command-1002' go run ./examples/transactional-outbox-publisher
```

이 combined command는 disposable clean-state 시연용이지 outbox recovery runner가
아닙니다. Order commit 이후 publication이 실패하면 pending row가 durable하게 남고,
다음 sample보다 먼저 claim될 수 있습니다. 이때 command는 다른 event를 성공으로
보고하지 않고 identity check에서 실패합니다. Production에서는 placement와
continuous relay/recovery를 독립 lifecycle로 운영합니다. Pending work를 처리하려고
새 order를 만들면 안 됩니다.

작업을 마치면 local container를 중지합니다.

```bash
docker stop workshop-outbox-postgres workshop-outbox-redis
```

## 테스트

예제의 focused suite와 race gate를 실행합니다.

```bash
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
go test -count=10 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRelayConcurrentRunOnce$'
```

테스트는 다음을 증명합니다.

- Order와 outbox row가 atomic하게 commit됩니다.
- Duplicate order와 outbox identity conflict는 transaction 전체를 rollback합니다.
- Transient publish failure는 설정한 retry delay가 정확히 지난 뒤 eligible해지고
  stable identity를 유지합니다.
- 세 번째 publish failure는 record를 dead letter로 옮깁니다.
- Cancellation은 claimed record를 retry나 dead-letter state로 덮어쓰지 않습니다.
- Concurrent relay worker는 테스트한 batch 안에서 claim한 record를 각각 한 번씩
  publish합니다.
- 릴리스된 Redis adapter가 13개 field와 일치하는 `entry_json`을 발행합니다.

## Production Boundary

- PostgreSQL commit이 durability boundary입니다. Redis publication은
  asynchronous하며 order transaction 안에서 실행하면 안 됩니다.
- One-shot command는 lesson을 위해 placement 한 번과 relay batch 한 번을 묶습니다.
  Production placement와 relay/recovery lifecycle은 분리해야 합니다.
- Delivery는 at-least-once입니다. Consumer는 `event_id` 또는
  `idempotency_key`로 deduplicate하고 duplicate와 replay를 허용해야 합니다.
- SQL outbox가 durable source of truth이며 Redis Streams는 recovery ledger가
  아니라 transport입니다.
- 이 예제는 end-to-end exactly-once 처리를 보장하지 않습니다. `XADD` 이후 SQL
  published mark가 durable해지기 전에 process가 실패할 수 있습니다.
- Production worker에는 dead-letter inspection, outbox retention, Redis Stream
  trimming, consumer group, lag/attempt metric, alerting이 필요합니다. 이 예제는
  poison-message나 dead-letter replay 자동화를 구현하지 않으며, replay에는 별도
  authorization과 audit policy가 필요합니다.
- PostgreSQL과 Redis의 authentication, TLS, secret rotation, network policy,
  connection limit, shutdown deadline은 이 예제 밖에서 설정합니다.
- 실행형 lesson을 위해 schema를 코드에서 생성합니다. Production schema ownership은
  versioned migration에 둡니다.
