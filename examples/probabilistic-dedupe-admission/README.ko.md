# probabilistic-dedupe-admission

[English](README.md) | [한국어](README.ko.md)

`probabilistic`을 사용하는 Gin webhook admission 예제입니다.

이 예제는 `bluetape-go/probabilistic`의 in-memory Bloom filter로 authoritative
workflow 앞의 event ID를 prefilter합니다. Definitely-new event, probably-seen
event, false-positive tradeoff, production durable storage가 필요한 이유를 함께
보여줍니다.

## 시나리오

Checkout webhook receiver가 upstream payment/order system에서 event ID를 받습니다.
무거운 처리를 하기 전에 저렴한 front-door signal이 필요합니다. Bloom filter는 삽입된
값에 대해 false negative를 만들지 않으므로 event ID가 definitely new라고 말할 수
있습니다. 반대로 ID가 probably seen이라고 말할 때는 실제 duplicate일 수도 있고 false
positive일 수도 있습니다.

Demo는 동작을 명확히 보여주기 위해 `probably_seen` event를 prefilter boundary에서
거부합니다. Production system은 중요한 event를 버리기 전에 authoritative durable
store를 확인해야 합니다.

## 보여주는 것

- HTTP service 안에서 사용하는 in-memory `probabilistic.NewStringBloomFilter`.
- Deterministic first-event `admit` path와 repeat-event `probably_seen` path.
- Expected insertions, target false-positive probability, approximate element
  count, expected current false-positive probability를 보여주는 approximate Bloom
  filter stats.
- Malformed request와 missing event ID에 대한 stable public HTTP error.
- Production caveat: probabilistic filter는 prefilter이며 durable source of truth가
  아닙니다.

## 실행

```bash
go run ./examples/probabilistic-dedupe-admission
```

Service는 기본적으로 `127.0.0.1:8099`에서 실행됩니다.

```bash
curl http://127.0.0.1:8099/healthz
```

새 event를 admit합니다:

```bash
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

예상 decision은 `admit`, reason은 `definitely_new`, accepted는 `true`입니다.

같은 event를 다시 보냅니다:

```bash
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

예상 decision은 `probably_seen`, reason은
`might_be_duplicate_or_false_positive`, accepted는 `false`입니다.

Filter 상태를 확인합니다:

```bash
curl -s http://127.0.0.1:8099/filters/current | jq
```

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/events/admit` | Event ID 하나를 Bloom prefilter로 admit합니다. |
| `GET` | `/filters/current` | Approximate Bloom filter stats를 조회합니다. |

## Boundary Notes

- `probably_seen`은 duplicate 증명이 아닙니다. False positive일 수 있습니다.
- 이 예제에는 durable store가 없습니다. Production dedupe는 Bloom prefilter를
  authoritative table, log, idempotency store와 함께 사용해야 합니다.
- Bloom filter는 개별 entry 삭제를 지원하지 않습니다. 필요하면 expiry window,
  rotation, backend-backed design을 사용해야 합니다.
- Demo key는 trim된 `event_id`입니다. `source`는 scenario metadata입니다.

## 테스트

```bash
go test -count=1 ./examples/probabilistic-dedupe-admission/...
go test -race -count=1 ./examples/probabilistic-dedupe-admission/...
```
