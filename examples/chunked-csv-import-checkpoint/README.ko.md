# Chunked CSV Import Checkpoint

[English](README.md) | [한국어](README.ko.md)

이 예제는 v0.5.0 batch focused scenario입니다. Local customer CSV import를
`batch.Job`, `batch.Step`, chunk checkpoint, restart behavior로 실행합니다.
[#41](https://github.com/bluetape4k/bluetape-go-workshop/issues/41)의 기본
checkpoint/restart track을 CSV import 관점으로 좁힌 companion 예제입니다.

## 예제 시나리오

Job은 `testdata/customers.csv`의 deterministic customer row 5개를 chunk size
`2`로 import합니다. 첫 실행은 첫 chunk를 commit하고 `next_row=2`를 저장합니다.
그 다음 두 번째 chunk를 시작해 `cust-1003`을 commit한 뒤, chunk checkpoint를
저장하기 전에 simulated writer crash를 반환합니다.

Restart 실행은 fresh reader를 만들고 `next_row=2`를 restore한 뒤 실패했던 chunk를
다시 읽습니다. Customer ID 기준 idempotent writer가 duplicate `cust-1003`을
skip하고, 남은 row를 commit한 뒤 final checkpoint `next_row=5`를 저장합니다.

![Chunked CSV import checkpoint scenario](../../docs/images/readme-diagrams/chunked-csv-import-checkpoint-scenario.png)

## 실행

```bash
go run ./examples/chunked-csv-import-checkpoint
```

출력은 runtime timestamp가 없는 stable JSON입니다. 핵심 field는 다음과 같습니다.

```json
{
  "checkpoint_key": "customer-csv-import",
  "chunk_size": 2,
  "first_run": {
    "report": {"status": "failed", "read_count": 4, "write_count": 2},
    "checkpoint": {"next_row": 2},
    "new_commits": 3,
    "duplicate_skips": 0
  },
  "restart_run": {
    "report": {"status": "completed", "read_count": 3, "write_count": 3},
    "checkpoint": {"next_row": 5},
    "new_commits": 5,
    "duplicate_skips": 1
  },
  "customers_imported": 5,
  "duplicate_skips": 1
}
```

`batch.Report.WriteCount`는 writer call이 성공한 chunk item 수를 셉니다. Domain
sink는 replay 동작을 명확히 보이도록 `new_commits`와 `duplicate_skips`도 별도로
노출합니다.

## Architecture

Runnable demo는 하나의 `batch.MemoryCheckpointStore`와 customer sink를 공유하면서
동일 import job을 두 번 실행합니다. `CSVReader`는 `batch.CheckpointReader`를
구현하고, processor는 row를 validate/normalize하며, writer는 customer ID 기준으로
chunk를 idempotent commit합니다.

![Chunked CSV import checkpoint architecture](../../docs/images/readme-diagrams/chunked-csv-import-checkpoint-architecture.png)

## Sequence Diagram

Checkpoint save는 chunk write가 성공한 뒤에만 일어납니다. 두 번째 chunk가 partial
side effect 이후 실패하므로 restart는 마지막 safe cursor에서 replay해야 하고,
writer는 duplicate를 흡수해야 합니다.

![Chunked CSV import checkpoint sequence](../../docs/images/readme-diagrams/chunked-csv-import-checkpoint-sequence.png)

## Production Hardening

이 예제는 의도적으로 in-memory checkpoint store와 sink를 사용합니다. Production
importer라면 durable checkpoint store, transactional write 또는 database upsert,
file identity/version contract, audit record, retry/dead-letter handling, 반복
실패 alerting, incoming CSV file의 schema evolution 처리가 필요합니다.

## 테스트

```bash
go test -count=1 ./examples/chunked-csv-import-checkpoint/...
go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...
```
