# SQL Outbox를 사용한 감사 가능한 주문 Workflow

[English](README.md) | 한국어

이 실행형 Gin 서비스는 변경되는 주문 상태, 불변 audit history, 공식 SQL outbox
record를 하나의 PostgreSQL transaction에서 저장합니다. Commit이 끝난 record는
감독되는 `sqloutbox.Relay`가 나중에 Redis Streams로 발행합니다. 따라서 command
응답이 보장하는 것은 PostgreSQL durable commit이지 Redis 전송 완료가 아닙니다.

![Audited order workflow architecture](../../docs/images/readme-diagrams/audited-order-workflow-outbox-architecture.png)

## 패키지에서 배울 내용

| 구성 요소 | 책임 |
| --- | --- |
| Gin adapter | Strict POST JSON, 32 KiB body 제한, 2초 작업 deadline, 안정된 error, 동시 요청 32개 제한 |
| `orderworkflow.Service` | 검증, 상태 전이, row lock, idempotent replay, 하나의 `sqlkit.WithTx` |
| Order/history/outbox table | 현재 projection, 불변 조회 원본, durable 전송 backlog |
| `sqloutbox.Relay` | 제한된 claim, retry, dead-letter, 연속 lifecycle |
| Redis Streams adapter | PostgreSQL commit 이후의 at-least-once 전송 |

![Audited order workflow sequence](../../docs/images/readme-diagrams/audited-order-workflow-outbox-sequence.png)

Create는 `pending` revision 1에서 시작합니다. `confirm`은 pending을
`confirmed`로 바꾸고, `cancel`은 pending 또는 confirmed를 `cancelled`로
바꿉니다. 종료 상태에서 다시 전이하면 `409 invalid_transition`을 반환합니다.

Command ID는 `event_id`이면서 `idempotency_key`입니다. 같은 canonical command를
재시도하면 최초에 commit한 주문을 `replayed: true`와 함께 반환하며 history나
outbox row를 추가하지 않습니다. 같은 ID를 다른 주문, action, reason, metadata에
재사용하면 409입니다.

32개 request slot이 모두 사용 중이면 다음 요청은 JSON decode 전에
`429 too_many_requests`, `Retry-After: 1`, `Connection: close`로 거부됩니다.
같은 command ID를 재사용하면 이 retry도 안전합니다.

## 실행

버전을 고정한 local dependency를 시작합니다.

```bash
docker run --rm -d --name workshop-audited-postgres \
  -e POSTGRES_DB=bluetape -e POSTGRES_USER=bluetape -e POSTGRES_PASSWORD=bluetape \
  -p 5432:5432 postgres:16-alpine
docker run --rm -d --name workshop-audited-redis \
  -p 6379:6379 redis:7.4-alpine
until docker exec workshop-audited-postgres pg_isready -U bluetape -d bluetape; do sleep 1; done
until docker exec workshop-audited-redis redis-cli ping | rg -q PONG; do sleep 1; done
```

인증이 없는 예제이므로 `HTTP_ADDR`에는 IPv4 또는 IPv6 loopback literal만
허용합니다. Wildcard, hostname, zone-scoped address, remote address는 거부합니다.

```bash
export DATABASE_URL='postgres://bluetape:bluetape@127.0.0.1:5432/bluetape?sslmode=disable'
export REDIS_ADDR='127.0.0.1:6379'
export REDIS_STREAM='workshop:audited-orders'
export HTTP_ADDR='127.0.0.1:8080'
go run ./examples/audited-order-workflow-outbox
```

Liveness, durable readiness, 제한된 delivery 진단 값을 확인합니다.

```bash
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/readyz
curl -sS http://127.0.0.1:8080/statusz
```

Redis 장애가 나면 `/readyz`는 200을 유지하되 `"delivery":"degraded"`를
반환합니다. Database 장애나 relay 중단은 503입니다. `/statusz`에는 Redis/relay
상태, delivery count, 초 단위 oldest-pending age만 나옵니다. Event identity,
payload, metadata, endpoint, provider error 원문은 노출하지 않습니다.

## POST JSON 시나리오

Metadata를 포함해 주문을 생성합니다. 새 command는 201과
`delivery: "asynchronous"`를 반환합니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/orders \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-1001","command_id":"cmd-create-1001","metadata":{"channel":"workshop"}}'
```

주문을 confirm합니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-1001","command_id":"cmd-confirm-1001","action":"confirm","metadata":{"operator":"demo"}}'
```

위 confirm을 그대로 다시 보내면 200, `replayed: true`, 최초 revision 2
projection을 반환합니다. Audit row와 outbox row는 늘어나지 않습니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-1001","command_id":"cmd-confirm-1001","action":"confirm","metadata":{"operator":"demo"}}'
```

첫 페이지를 조회합니다. 모든 범위는 inclusive이고 `limit`은 1부터 100까지
필수입니다. Revision 1과 2가 있으면 revision 1과
`next_from_revision: 2`를 반환합니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/search \
  -H 'Content-Type: application/json' \
  --data '{"aggregate":{"type":"order","id":"order-1001"},"from_revision":1,"to_revision":null,"from_recorded_at":null,"to_recorded_at":null,"limit":1}'
```

그 inclusive cursor로 두 번째 페이지를 조회하면 revision 2와
`next_from_revision: null`을 반환합니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/search \
  -H 'Content-Type: application/json' \
  --data '{"aggregate":{"type":"order","id":"order-1001"},"from_revision":2,"to_revision":null,"from_recorded_at":null,"to_recorded_at":null,"limit":1}'
```

Redis를 거치지 않고 revision 2를 읽습니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/audit/history/detail \
  -H 'Content-Type: application/json' \
  --data '{"aggregate":{"type":"order","id":"order-1001"},"revision":2}'
```

두 번째 주문을 만들고 cancel합니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/orders \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-2001","command_id":"cmd-create-2001","metadata":{"channel":"workshop"}}'
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-2001","command_id":"cmd-cancel-2001","action":"cancel","reason":"customer request","metadata":{"operator":"demo"}}'
```

Cancel 이후 새 confirm은 안정된 `409 invalid_transition`으로 거부됩니다.

```bash
curl -sS -X POST http://127.0.0.1:8080/orders/transitions \
  -H 'Content-Type: application/json' \
  --data '{"order_id":"order-2001","command_id":"cmd-confirm-2001","action":"confirm","metadata":{"operator":"demo"}}'
```

저장소의 [`requests.http`](requests.http)에는 JetBrains와 VS Code REST client에서
바로 실행할 수 있도록 같은 전체 body와 expected status를 넣었습니다.

## 장애와 복구 Runbook

- Redis 장애: command는 PostgreSQL에 계속 commit됩니다. `delivery:
  "degraded"`와 증가하는 pending/retrying count를 확인하고 Redis를 복구한 뒤
  backlog가 줄어드는지 봅니다.
- 중단된 publish: claimed row는 공식 30초 lease가 지난 뒤 다시 claim할 수
  있습니다. 조기 retry를 위해 row를 직접 수정하지 않습니다.
- Dead letter: `/statusz`의 aggregate count로 확인하고 request path 밖에서
  publisher/consumer를 조사합니다. 이 예제는 audit 없이 실행되는 replay endpoint를
  제공하지 않습니다.
- 부분 startup: dependency 또는 incompatible schema를 바로잡고 재시작합니다.
  단일 버전 bootstrap은 중간에 끊긴 create를 idempotent하게 수렴시키지만 기존의
  incompatible table을 자동 변경하지 않습니다.
- Local에서만 사용하는 파괴적 reset:

```bash
docker exec workshop-audited-postgres psql -U bluetape -d bluetape -c \
  'TRUNCATE audited_order_workflow_outbox_records, audited_order_workflow_audit_entries, audited_order_workflow_orders RESTART IDENTITY;'
```

이 명령은 workshop reset용이며 production migration이나 rollback 절차가 아닙니다.

## Delivery 경계

Delivery는 at-least-once입니다. Redis append 결과가 모호하거나 claim lease가
만료되면 같은 logical event가 다시 전송될 수 있습니다. Consumer는 `event_id` 또는
`idempotency_key`로 deduplicate하고 duplicate와 reordering을 허용해야 합니다.
SQL history가 불변 조회 원본이며 Redis Streams는 recovery ledger가 아닌
transport입니다. Consumer group, retention, stream trimming, dead-letter replay,
authentication, tenant authorization, exactly-once 처리는 이 예제 범위 밖입니다.

Shutdown할 때는 HTTP를 drain하고 relay를 cancel/join한 뒤 Redis와 PostgreSQL을
닫습니다. Shutdown 도중 취소된 publish는 lease recovery 대상입니다.

## 테스트

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/...
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox/... -run TestIntegration
```
