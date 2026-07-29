# Issue #74 Retry and Dead-Letter Batch Worker 리서치

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #74, `[v0.5.0] Add retry and dead letter batch worker example`
- 상위 이슈: #29, 0.5.0 batch checkpoint and restart workshop examples
- 의존성 기준선: `github.com/bluetape4k/bluetape-go v0.5.1`

## 현재 근거

- #74는 batch worker의 retry policy와 dead-letter handling을 위한 focused example을
  요구한다.
- #29는 upstream batch support가 명시적으로 제공하지 않는 한 durable queue semantic을
  범위 밖으로 둔다고 말한다.
- `bluetape-go v0.5.1`의 `batch.Step`은 이미 다음을 노출한다.
  - processor와 writer failure를 위한 `RetryPolicy`.
  - processor item skip과 writer chunk skip을 위한 `SkipPolicy`.
  - `batch.Report`의 `RetryCount`, `SkipCount`, `ReadCount`, `WriteCount`.
  - writer chunk skipping이 checkpoint를 안전하지 않게 전진시킬 때의
    `ErrUnsafeWriterSkipCheckpoint`.
- `workreport.Report`는 named child outcome, failure policy, stable status value,
  timestamp-free projection에 대한 기존 워크숍 shape다.
- `examples/operations-report-policy`는 이미 retry evidence와 failure policy를
  가르치지만 `batch.Step`은 사용하지 않는다.
- `examples/chunked-csv-import-checkpoint`는 이미 checkpoint/restart를 가르치지만,
  retry/dead-letter handling은 의도적으로 이후 예제에 넘긴다.
- GNO lookup은 retry, skip, checkpoint, restart batch feature를 도입한
  `bluetape-go` issue #30을 찾았다.
- CodeGraph는 이 저장소나 worktree에 초기화되어 있지 않았으므로, source discovery는
  현재 저장소 파일, `gh issue view`, `rg`, `go list` module source path를 사용했다.

## 채택 방향

`examples/retry-dead-letter-batch-worker` 아래 in-memory support-ticket queue worker를
만든다.

예제는 `batch` primitive를 보이게 유지한다.

- `TicketReader`는 deterministic queued work item을 공급한다.
- `TicketProcessor`는 outcome을 분류한다.
  - transient item failure는 bounded retry 뒤 성공한다.
  - permanent item failure는 dead-letter entry를 기록하고 skippable error를 반환한다.
  - context cancellation은 retry 없이 caller cancellation을 반환한다.
- `TicketWriter`는 성공적으로 처리된 ticket receipt를 저장한다.
- `batch.Step`은 `RetryPolicy`, `SkipPolicy`, 최종 batch report를 소유한다.
- stable result projection은 processed receipt, dead letter, retry count, skip count,
  timestamp-free report tree를 노출한다.

## 거부한 선택지

| Option | Reason |
|---|---|
| durable queue 또는 database-backed dead-letter table | #29는 durable queue semantic을 범위 밖으로 두며, 이를 추가하면 `batch` policy behavior에서 주의가 흐려진다. |
| `batch.Step` 밖 custom retry loop | 마일스톤이 가르치려는 upstream `batch.RetryPolicy` contract를 숨긴다. |
| checkpointing이 있는 writer-level dead-letter skip | `batch.ErrUnsafeWriterSkipCheckpoint`는 checkpoint가 활성화된 동안 failed writer chunk를 안전하게 skip할 수 없는 이유를 보여 준다. |
| queue 또는 scheduling을 위한 새 의존성 | focused local example에는 필요하지 않으며 no-new-dependency 기본값과 충돌한다. |

## 설계 메모

- dead-letter entry는 processor가 permanent, skippable sentinel error를 반환하기 전에
  생성하는 domain record다.
- 이 focused example에서 retry는 transient processor error에만 적용한다.
- `batch.SkipPolicy`는 permanent item error를 item skip으로 바꾸며, domain dead-letter
  list는 그 skip을 검사 가능하게 만든다.
- 예제는 production durability를 주장하지 않아야 한다. README hardening note는
  durable queue, persistent dead-letter storage, idempotent handler, observability,
  backoff, alerting을 production concern으로 명시해야 한다.

## 검증 영향

- 테스트는 retry 뒤 transient success, permanent dead-letter capture, retry/skip count,
  context cancellation에 대한 no retry, stable report projection을 assert해야 한다.
- 예제가 shared in-memory sink state와 dead-letter storage를 소유하므로 race validation은
  필수다.
- README diagram은 decorated workshop baseline을 따르고 구체적인 margin gate evidence를
  출력해야 한다.
