# shared-redis-bloom-admission

[English](README.md) | [한국어](README.ko.md)

`probabilistic/redis`를 사용하는 Gin webhook admission 예제입니다.

이 예제는 `bluetape-go/probabilistic/redis`의 Redis-backed Bloom filter로 여러 API
instance가 공유하는 가벼운 admission signal을 만듭니다. 기존
`probabilistic-dedupe-admission`의 in-memory 예제를 이어받아 실제 운영형
애플리케이션에서 중요한 분산 요소를 더합니다. 같은 Redis namespace를 공유하는 방법,
metadata fingerprint mismatch 처리, Docker-backed integration test, false positive를
읽는 기준을 함께 보여줍니다.

## 시나리오

Checkout platform이 여러 API instance에서 같은 webhook stream을 받습니다. Instance
A가 `evt-1001`을 먼저 볼 수 있고, retry나 load balancing 때문에 같은 event가
Instance B로 들어올 수도 있습니다. 각 process가 in-memory Bloom filter만 가지면 두
번째 process는 첫 번째 insert를 볼 수 없습니다. Redis-backed Bloom filter는 bit와
metadata를 Redis로 옮겨, 같은 namespace를 쓰는 모든 instance가 동일한 probabilistic
boundary를 읽게 합니다.

핵심은 probabilistic입니다. `Put(ctx, eventID)`가 하나 이상의 bit를 바꾸면, 그
event는 해당 Bloom filter 기준으로 definitely new입니다. 삽입된 값에 대해 Bloom
filter는 나중에 false negative를 만들지 않기 때문입니다. 반대로 bit가 하나도
바뀌지 않으면 결과는 `probably_seen`일 뿐입니다. 실제 duplicate일 수도 있고, Bloom
filter 포화나 hash collision 때문에 생긴 false positive일 수도 있습니다. 이 service는
그 boundary를 눈에 보이게 만들지만, production workflow에서는 중요한 일을 버리기 전에
authoritative idempotency table, log, state store를 반드시 확인해야 합니다.

## Architecture

![Shared Redis Bloom admission architecture](../../docs/images/readme-diagrams/shared-redis-bloom-architecture.png)

HTTP service는 의도적으로 작게 유지합니다. 각 instance는 webhook 형태의 request를
받고, 같은 Redis Bloom namespace를 호출한 뒤, 안정적인 admission projection을
반환합니다. Redis는 Bloom bit와 configuration metadata를 소유합니다. Diagram에서
Redis 옆에 authoritative store를 둔 이유는 운영 규칙을 강조하기 위해서입니다. Bloom
filter는 front-door prefilter이지 final truth가 아닙니다.

## Admission Flow

![Cross-instance Redis Bloom admission sequence](../../docs/images/readme-diagrams/shared-redis-bloom-sequence.png)

1. Client가 `evt-1001`을 Instance A로 보냅니다.
2. Instance A는 Redis-backed filter에 `Put(ctx, "evt-1001")`을 호출합니다.
3. Redis의 bit가 하나 이상 바뀌므로 service는 `admit`, `definitely_new`,
   `accepted=true`를 반환합니다.
4. 같은 event가 나중에 Instance B로 들어옵니다.
5. Instance B는 같은 Redis namespace를 호출합니다. 이미 모든 bit가 설정되어 있으므로
   service는 `probably_seen`, `might_be_duplicate_or_false_positive`,
   `accepted=false`를 반환합니다.

두 번째 response가 증명하는 것은 모든 Bloom offset이 이미 set되어 있었다는 사실뿐입니다.
Caller가 unauthorized라는 뜻도 아니고, event가 duplicate임을 정확히 증명하는 것도
아닙니다.

## Decision Policy

![Redis Bloom admission decision policy](../../docs/images/readme-diagrams/shared-redis-bloom-policy.png)

| Redis Bloom 결과 | HTTP decision | 읽는 방법 |
|---|---|---|
| `Put`이 bit를 바꿈 | `200 admit`, `accepted=true` | 다음 workflow step에 넣어도 되는 definitely-new path입니다. |
| 바뀐 bit 없음 | `200 probably_seen`, `accepted=false` | Duplicate-or-false-positive로 읽어야 하며 exact dedupe가 아닙니다. |
| Config fingerprint 다름 | `409 filter_config_mismatch` | 다른 Bloom sizing의 reader를 한 namespace에 섞지 않습니다. |
| Redis unavailable 또는 request canceled | `503 filter_unavailable` | Prefilter가 답하지 못했으므로 upstream에서 나중에 retry합니다. |

Redis package는 bit 옆에 metadata fingerprint를 저장합니다. 어떤 instance가 10,000
expected insertions로 namespace를 만들었는데, 다른 instance가 다른 size나
false-positive target으로 같은 namespace를 재사용하려 하면 이 예제는 그 mismatch를
`409 filter_config_mismatch`로 매핑합니다. Bloom sizing을 바꿀 때는 새 namespace를
쓰거나 의도적으로 filter를 rebuild해야 합니다.

## 보여주는 것

- Gin HTTP service 안에서 사용하는 `redisbloom.NewStringBloomFilter`.
- 여러 Redis client와 service instance가 공유하는 Bloom state.
- First insert, repeated value, cross-instance visibility, config mismatch,
  Redis failure, cancellation, invalid request test.
- Bit count, approximate element count, expected current false-positive
  probability, bit size, hash count, hasher key를 포함한 approximate stats.
- Repository Testcontainers fixture를 사용한 Docker-backed Redis integration test.
- Bloom hit를 authorization이나 exact dedupe로 취급하지 않는 stable public HTTP
  error와 decision.

## 실행

Redis를 먼저 실행합니다. 수동 확인에는 Docker만으로 충분합니다.

```bash
docker run --rm -p 6379:6379 redis:7-alpine
```

그다음 service를 실행합니다.

```bash
export REDIS_ADDR=127.0.0.1:6379
go run ./examples/shared-redis-bloom-admission
```

Service는 기본적으로 `127.0.0.1:8102`에서 실행됩니다.

```bash
curl http://127.0.0.1:8102/healthz
```

새 event를 admit합니다:

```bash
curl -s -X POST http://127.0.0.1:8102/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

예상 decision은 `admit`, reason은 `definitely_new`, accepted는 `true`입니다.

같은 event를 다시 보냅니다:

```bash
curl -s -X POST http://127.0.0.1:8102/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

예상 decision은 `probably_seen`, reason은
`might_be_duplicate_or_false_positive`, accepted는 `false`입니다.

Shared filter 상태를 확인합니다:

```bash
curl -s http://127.0.0.1:8102/filters/current | jq
```

주요 환경 변수:

| 변수 | 기본값 | 용도 |
|---|---|---|
| `HTTP_ADDR` | `127.0.0.1:8102` | Loopback HTTP bind address입니다. Non-loopback bind는 거부합니다. |
| `REDIS_ADDR` | `127.0.0.1:6379` | Bloom filter가 사용할 Redis address입니다. |
| `BLOOM_NAMESPACE` | `shared-redis-bloom-admission:webhooks` | 공유 Redis Bloom namespace입니다. |
| `INSTANCE_ID` | `api-instance-local` | Response에 포함할 instance label입니다. |
| `BLOOM_EXPECTED_INSERTIONS` | `10000` | Bloom filter sizing에 쓰는 expected insertions입니다. |
| `BLOOM_FALSE_POSITIVE_PROBABILITY` | `0.01` | Target false-positive probability입니다. |

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/events/admit` | Event ID 하나를 shared Redis Bloom prefilter로 admit합니다. |
| `GET` | `/filters/current` | Shared Bloom filter의 approximate stats를 조회합니다. |

## Boundary Notes

- `probably_seen`은 duplicate 증명이 아닙니다. False positive일 수 있습니다.
- Bloom filter hit는 authorization decision이 아닙니다.
- Redis가 probabilistic boundary를 공유해도 workflow completion의 durable source of
  truth가 되지는 않습니다.
- Namespace 하나에는 Bloom sizing configuration 하나만 둡니다. Size나 false-positive
  target을 바꿀 때는 새 namespace를 쓰거나 rebuild를 명시적으로 수행합니다.
- Bloom filter는 개별 entry 삭제를 지원하지 않습니다. 삭제가 필요하면 time-windowed
  namespace, rotation, rebuild, 또는 다른 data structure를 사용합니다.
- Demo key는 trim된 `event_id`입니다. `source`는 scenario metadata입니다.

## 테스트

이 테스트는 Testcontainers로 Redis를 시작합니다. Container-backed 예제가 Docker
resource를 두고 경쟁하지 않도록 serial로 실행합니다.

```bash
go test -p 1 -count=1 ./examples/shared-redis-bloom-admission/...
go test -p 1 -race -count=1 ./examples/shared-redis-bloom-admission/...
```
