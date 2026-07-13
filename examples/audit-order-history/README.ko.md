# audit-order-history

[English](README.md) | [한국어](README.ko.md)

bluetape-go `audit`로 주문 상태 이력을 기록하는 application-shaped 예제입니다.

변경 가능한 현재 주문 snapshot과 불변의 create, confirm, ship audit entry를
함께 유지합니다. JSON preview는 두 관점을 나란히 보여주고 revision 범위를
newest-first로 조회한 결과도 포함합니다.

## Package Lesson

| Component | Owns |
|---|---|
| `audit.Repository` | 검증된 append, aggregate revision 연속성, 중복 event/idempotency 감지, history query. |
| Order service | 상태 전이, caller-owned command ID, append-before-mutation 일관성, current-state projection. |
| Application | Durable transaction/outbox, retention, access control, schema evolution, payload policy, redaction. |

Service는 학습용 lock을 잡은 상태에서 `Repository.Append`를 호출하고 성공한
뒤에만 현재 상태를 바꿉니다. Append 실패나 append 전/중 cancellation은 어느
쪽도 변경하지 않습니다. Append 성공이 commit point이므로 직후 cancellation이
도착해도 오류가 날 수 없는 in-memory 상태 대입까지 마칩니다. 두 동작 사이에서
다시 cancellation을 확인하면 audit history와 현재 상태가 갈라집니다.

Command ID는 caller가 소유하는 안정적인 식별자이며 event ID와 idempotency
key에 함께 사용합니다. 같은 ID를 다시 보내면 성공 replay가 아니라 중복으로
거절하며 오류는 `audit.ErrRevisionConflict`로 검사할 수 있습니다.

## Run

```bash
go run ./examples/audit-order-history
```

결정적인 출력은 current revision 3으로 끝나고 append 순서와 newest-first
filter 결과를 비교합니다.

```json
{
  "current": {
    "order_id": "order-1001",
    "status": "shipped",
    "revision": 3,
    "updated_at": "2026-07-13T09:10:00Z"
  },
  "history": [
    {"event_id":"command-create-1001","event_type":"order.created","revision":1,"status_after":"pending"},
    {"event_id":"command-confirm-1001","event_type":"order.confirmed","revision":2,"status_before":"pending","status_after":"confirmed"},
    {"event_id":"command-ship-1001","event_type":"order.shipped","revision":3,"status_before":"confirmed","status_after":"shipped"}
  ],
  "recent_history": [
    {"event_id":"command-ship-1001","event_type":"order.shipped","revision":3},
    {"event_id":"command-confirm-1001","event_type":"order.confirmed","revision":2}
  ]
}
```

위 entry는 README 길이를 줄이기 위해 timestamp와 author를 생략했습니다.
실제 실행 결과에는 두 필드도 포함됩니다.

## Test

```bash
go test -count=1 ./examples/audit-order-history/...
go test -count=20 ./examples/audit-order-history/internal/orderhistory -run '^TestServiceConcurrent'
go test -race -count=1 ./examples/audit-order-history/...
```

정상/cancel lifecycle, 잘못된 전이, 중복 command, repository 실패, append
경계 cancellation, 없는 history와 filtered history, defensive copy, 동시 실행의
정확한 결과를 sleep 없이 검증합니다.

## Audit History는 Event Sourcing이 아닙니다

학습용 source state는 현재 주문 map입니다. Audit event를 replay해서 주문을
복구하지 않으며 audit snapshot도 recovery model로 사용하지 않습니다. History는
무엇이 바뀌었는지 설명하고 조회할 수 있게 하지만 event store 계약은 아닙니다.

## Production Boundaries

- `audit.MemoryRepository`는 goroutine-safe지만 durable하지 않습니다. Process가
  끝나면 entry와 현재 snapshot을 모두 잃습니다.
- 하나의 process-wide mutex가 서로 다른 주문도 직렬화합니다. 단순한 invariant를
  보여주는 demo-scale 선택일 뿐 throughput이나 horizontal scaling 설계가 아닙니다.
  Durable code는 database-owned 또는 aggregate별 concurrency가 필요합니다.
- 이 예제의 `Find`는 기본 20건, 최대 100건을 반환하지만 in-memory repository는
  limit을 적용하기 전에 O(total stored entries)로 scan/copy합니다. Production API는
  storage-backed pagination과 retention이 필요합니다.
- 실제 application은 retention/deletion, archival, disaster recovery, encryption,
  access control, schema versioning/migration, payload size policy를 소유해야 합니다.
- Event payload와 cancellation reason에 credential이나 분류되지 않은 PII를 넣지
  마십시오. 저장하거나 log에 남기기 전에 분류하고 최소화하며 redact해야 합니다.
- Durable order row와 audit delivery를 맞추려면 caller-owned SQL transaction/outbox가
  필요합니다. 이 예제는 exactly-once delivery를 주장하지 않습니다.

#58은 Gin query API lesson을, #57과 #68은 durable outbox와 Redis Streams
delivery를 다룹니다.
