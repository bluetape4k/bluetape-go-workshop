# Retry Dead-Letter Batch Worker

[English](README.md) | [한국어](README.ko.md)

이 예제는 `bluetape-go/batch`로 bounded retry와 dead-letter 처리를 보여줍니다.
Queue, retry policy, skip policy, processed sink, dead-letter list를 큰 worker
framework 뒤에 숨기지 않고 그대로 드러냅니다.

## 예제 시나리오

Support operations batch worker가 queued ticket notification 네 개를 읽습니다:

| Ticket | Scenario | Outcome |
|---|---|---|
| `ticket-1001` | 바로 성공 | processed sink에 기록 |
| `ticket-1002` | transient provider throttling | 한 번 retry한 뒤 기록 |
| `ticket-1003` | blocked delivery address | dead-letter list에 기록한 뒤 skip |
| `ticket-1004` | 바로 성공 | processed sink에 기록 |

`batch.RetryPolicy`는 `ErrTransientTicket`만 처리합니다.
`batch.SkipPolicy`는 `ErrPermanentTicket`만 처리합니다. Domain processor는
permanent error를 반환하기 전에 dead-letter entry를 기록하므로 batch report와
domain evidence가 같은 결정을 설명합니다.

## 실행

```bash
go run ./examples/retry-dead-letter-batch-worker
```

출력은 deterministic하며 runtime timestamp를 제외합니다:

```json
{
  "job_name": "ticket-batch-worker",
  "step_name": "ticket-retry-dead-letter-worker",
  "chunk_size": 2,
  "status": "completed",
  "read_count": 4,
  "write_count": 3,
  "retry_count": 1,
  "skip_count": 1
}
```

전체 JSON에는 processed ticket 순서, dead-letter record, timestamp-free
`batch.Report` projection도 포함됩니다.

## Retry and Dead-Letter Policy

| Boundary | Local rule |
|---|---|
| Retry | `batch.RetryErrors(3, errors.Is(err, ErrTransientTicket))` |
| Skip | `batch.SkipErrors(2, errors.Is(err, ErrPermanentTicket))` |
| Dead letter | processor가 permanent error 반환 전에 ticket ID, reason, attempts를 기록 |
| Writer error | duplicate processed ticket ID는 batch를 실패시키며 dead letter로 바꾸지 않음 |
| Cancellation | caller cancellation은 `batch.StatusCancelled`를 반환하며 retry/dead-letter 대상이 아님 |

## Architecture

![Retry Dead-Letter Batch Worker Architecture](../../docs/images/readme-diagrams/retry-dead-letter-batch-worker-architecture.png)

## Sequence Diagram

![Retry Dead-Letter Batch Worker Sequence](../../docs/images/readme-diagrams/retry-dead-letter-batch-worker-sequence.png)

## Scenario Diagram

![Retry Dead-Letter Batch Worker Scenario](../../docs/images/readme-diagrams/retry-dead-letter-batch-worker-scenario.png)

## 테스트

```bash
go test -count=1 ./examples/retry-dead-letter-batch-worker/...
go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...
```

테스트는 transient retry 후 성공, permanent dead-letter capture, skip budget
exhaustion, writer duplicate failure, caller cancellation, timestamp-free
report projection, concurrent run isolation, race detector 기반 concurrent
store access를 검증합니다.

## 관련 예제

- [`operations-report-policy`](../operations-report-policy/README.ko.md)는
  failure policy가 `workreport.Report` output을 어떻게 바꾸는지 보여줍니다.
- [`chunked-csv-import-checkpoint`](../chunked-csv-import-checkpoint/README.ko.md)는
  chunked batch import의 checkpoint/restart 동작을 보여줍니다.

## Production Hardening

이 예제는 의도적으로 in-memory state를 사용합니다. Production worker에는 durable
queue, persistent dead-letter table 또는 topic, idempotent handler, jitter가
있는 backoff, attempt와 DLT 증가량 metric, repeated failure alerting, 안전한
replay tooling이 필요합니다.
