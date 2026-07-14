# Transactional Outbox Publisher Example

English | [한국어](README.ko.md)

This example closes the gap left by the SQL transaction boundary lesson: an
order row and its audit event commit in the same PostgreSQL transaction, then a
released `bluetape-go` relay publishes the committed record to Redis Streams.
The application uses `audit/sqloutbox` and
`audit/sqloutbox/redisstreams` directly; it does not reimplement the outbox or
transport in workshop code.

![Transactional outbox publisher architecture](../../docs/images/readme-diagrams/transactional-outbox-publisher-architecture.png)

## Scenario

1. `Service.Place` validates the order command before opening a transaction.
2. One `sqlkit.WithTx` inserts the order and calls `sqloutbox.Store.Enqueue`.
3. PostgreSQL commits both rows or rolls both back.
4. After commit, `sqloutbox.Relay.RunOnce` claims the durable outbox row.
5. The released Redis Streams publisher appends all 13 record fields and the
   encoded audit entry.
6. The relay marks the SQL row as published only after `XADD` succeeds.

![Transactional outbox publisher sequence](../../docs/images/readme-diagrams/transactional-outbox-publisher-sequence.png)

The retry path keeps `event_id` and `idempotency_key` stable. `attempts` is a
delivery diagnostic, not a new event identity. Caller cancellation after a
claim exits without scheduling a retry or dead-lettering the row; lease
recovery makes the claimed row eligible later.

## Run

Start pinned local PostgreSQL and Redis containers:

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

Run one order commit and one relay batch:

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

The command writes one newline-terminated JSON object:

```json
{"order":{"OrderID":"order-1001","CustomerID":"customer-42","Status":"placed","TotalCents":3700,"CreatedAt":"2026-07-14T12:00:00Z"},"relay":{"Claimed":1,"Published":1,"Failed":0,"DeadLettered":0},"stream":"workshop:transactional-outbox","event_id":"command-1001","idempotency_key":"command-1001"}
```

Inspect the transport record:

```bash
docker exec workshop-outbox-redis redis-cli XRANGE workshop:transactional-outbox - +
```

Every stream message contains these 13 fields:

| Identity and state | Audit envelope | Delivery |
| --- | --- | --- |
| `record_id`, `status`, `aggregate_type`, `aggregate_id`, `revision` | `event_id`, `idempotency_key`, `event_type`, `occurred_at`, `recorded_at`, `schema_version`, `entry_json` | `attempts` |

`entry_json` decodes to the same aggregate, revision, event identity, event
type, and schema version as the scalar fields. The integration test verifies
that parity against real PostgreSQL and Redis containers.

The order, command, and stream identities are durable. To run the command again
against the same containers, provide new identities:

```bash
ORDER_ID='order-1002' COMMAND_ID='command-1002' go run ./examples/transactional-outbox-publisher
```

This combined command is a disposable clean-state demonstration, not an outbox
recovery runner. If publication fails after the order commits, the pending row
remains durable and may be claimed before a later sample. The command then fails
its identity check instead of reporting the wrong event. Production placement
and continuous relay/recovery run as independent lifecycles; do not create a new
order merely to drain pending work.

Stop the local containers when finished:

```bash
docker stop workshop-outbox-postgres workshop-outbox-redis
```

## Tests

Run the focused example suite and race gate:

```bash
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
go test -count=10 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRelayConcurrentRunOnce$'
```

The tests prove:

- order and outbox rows commit atomically;
- duplicate order and outbox identity conflicts roll the whole transaction back;
- transient publish failures become eligible exactly after the configured
  retry delay and preserve stable identities;
- the third failed attempt moves a record to dead letter;
- cancellation does not overwrite a claimed record with retry or dead-letter
  state;
- concurrent relay workers publish each claimed record once within the tested
  batch; and
- the released Redis adapter emits all 13 fields and a matching `entry_json`.

## Production Boundaries

- The PostgreSQL commit is the durability boundary. Redis publication is
  asynchronous and must never happen inside the order transaction.
- The one-shot command intentionally couples one placement and one relay batch
  for teaching. Production placement and relay/recovery lifecycles are separate.
- Delivery is at-least-once. Consumers must deduplicate by `event_id` or
  `idempotency_key` and tolerate duplicate or replayed messages.
- The SQL outbox is the durable source of truth; Redis Streams is transport,
  not the recovery ledger.
- This example does not claim end-to-end exactly-once processing. A process can
  fail after `XADD` and before the SQL published mark is durable.
- Production workers need dead-letter inspection, outbox retention, Redis
  Stream trimming, consumer groups, lag/attempt metrics, and alerting. This
  example does not implement poison-message or dead-letter replay automation;
  replay requires separate authorization and audit policy.
- Configure PostgreSQL and Redis authentication, TLS, secret rotation, network
  policy, connection limits, and shutdown deadlines outside this example.
- Schema creation is kept in the runnable lesson. Production schema ownership
  belongs in versioned migrations.
