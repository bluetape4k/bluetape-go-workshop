# Issue #73 설계: Chunked CSV Import Checkpoint 예제

## 목표

Chunked CSV import, checkpoint persistence, 마지막 성공 chunk부터의 restart,
실패한 chunk replay 시 duplicate prevention을 보여주는 집중 v0.5.0 batch
예제를 추가한다.

## 비목표

- HTTP service, Gin route, background scheduler를 추가하지 않는다.
- durable database, Redis, queue, object storage checkpoint adapter를 추가하지
  않는다.
- generic CSV import framework를 구현하지 않는다.
- `batch.MemoryCheckpointStore`가 production durability라고 주장하지 않는다.
- 부분 실패한 writer chunk 이후 checkpoint를 advance하지 않는다.

## 예제

- 경로: `examples/chunked-csv-import-checkpoint`
- 패키지: `internal/csvimport`
- 실행 entrypoint: `main.go`
- Fixture: `testdata/customers.csv`
- Package dependency focus: `batch`

## 시나리오

Merchant가 customer CSV 파일을 업로드한다. Import job은 row를 2개 단위 chunk로
읽고, 각 customer를 검증 및 정규화한 뒤 committed customer를 domain sink에
쓰며, 성공한 각 chunk write 이후 checkpoint를 저장한다.

첫 번째 실행은 두 번째 chunk를 쓰는 중 crash를 simulation한다. Writer가 error를
반환하기 전에 해당 chunk의 customer 하나는 이미 commit된다. Chunk가 실패했기
때문에 checkpoint는 첫 번째 성공 chunk 끝에 남는다. Restart 실행은 fresh
reader를 만들고 checkpoint를 restore하며, 실패한 chunk를 replay하고 이미
commit된 customer ID를 skip한 뒤 나머지 row를 쓰고 checkpoint를 파일 끝으로
advance한다.

이 예제는 연결된 두 계약을 설명한다.

1. Checkpoint는 commit된 work 이후 아직 읽지 않은 다음 CSV row를 나타낸다.
2. 실패한 chunk가 replay될 수 있으므로 writer는 batch boundary에서 idempotent
   해야 한다.

## 도메인 모델

CSV column:

- `customer_id`
- `email`
- `tier`

Imported customer:

```go
type Customer struct {
    ID    string `json:"id"`
    Email string `json:"email"`
    Tier  string `json:"tier"`
}
```

Checkpoint:

```go
type Checkpoint struct {
    NextRow int `json:"next_row"`
}
```

`NextRow`는 CSV header를 제외한 parsed data row 기준 zero-based 값이다. Final
checkpoint `5`는 fixture row 다섯 개를 모두 읽고 commit했다는 뜻이다.

## Batch 설계

Reader:

- `encoding/csv`로 fixture를 parse한다.
- 정확한 header를 검증한다.
- field를 trim한다.
- `nextRow`를 추적한다.
- `batch.CheckpointReader`를 구현한다.
- `Checkpoint`에서만 restore한다.
- `Open`, `Read`, `Restore`, `Checkpoint`, `Close`에서 `ctx.Err()`를 확인한다.

Processor:

- 비어 있지 않은 customer ID, email, tier를 검증한다.
- email과 tier를 lowercase로 정규화한다.
- row/customer context를 포함한 wrapped error를 반환한다.
- 작업 전에 `ctx.Err()`를 확인한다.

Writer:

- chunk를 in-memory customer sink에 쓴다.
- `Customer.ID`를 idempotency key로 취급한다.
- duplicate skip을 성공한 새 commit과 별도로 기록한다.
- 설정된 new commit count 이후 crash를 simulation할 수 있다.
- restart test를 위해 wrapped `ErrSimulatedCrash` sentinel을 반환한다.
- 쓰기 전과 chunk item 사이에서 `ctx.Err()`를 확인한다.

Runner:

- 다음 값으로 `batch.Step[CSVRow, Customer]`를 만든다.
  - `Name: "chunked-customer-csv-import"`
  - `ChunkSize: 2`
  - `CheckpointStore: batch.NewMemoryCheckpointStore()` for the demo
  - `CheckpointKey: "customer-csv-import"`
- step을 `customer-csv-import` 이름의 `batch.Job`으로 wrap한다.
- timestamp 없는 안정적인 result projection을 반환한다.

## CLI 출력

`go run ./examples/chunked-csv-import-checkpoint`는 failure와 restart demo를
보여주는 안정적인 JSON을 출력한다.

```json
{
  "checkpoint_key": "customer-csv-import",
  "chunk_size": 2,
  "first_run": {
    "status": "failed",
    "read_count": 4,
    "write_count": 2,
    "checkpoint": {"next_row": 2}
  },
  "restart_run": {
    "status": "completed",
    "read_count": 3,
    "write_count": 3,
    "checkpoint": {"next_row": 5}
  },
  "customers_imported": 5,
  "duplicate_skips": 1
}
```

정확한 field name은 구현 중 바뀔 수 있지만, 출력은 timestamp-free이고 결정적이어야
한다.

## 실패 및 취소 계약

- 잘못된 CSV header는 reader open 중 실패한다.
- 유효하지 않은 customer row는 processing 중 실패하고 row/customer context를
  wrap한다.
- Simulated writer crash는 `ErrSimulatedCrash`를 반환하고 checkpoint를 이전
  성공 chunk 위치에 남긴다.
- Caller cancellation은 `batch.StatusCancelled`를 반환하고 열린 resource를
  닫으며, 이후 checkpoint를 저장하지 않는다.
- nil context는 upstream `batch.Step.Run`에서 background context로 허용되지만,
  이 예제는 테스트와 runner 코드에서 explicit context로 호출해야 한다.

## 다이어그램

`docs/images/readme-diagrams/` 아래에 README 다이어그램 자산을 생성한다.

- `chunked-csv-import-checkpoint-scenario`
- `chunked-csv-import-checkpoint-architecture`
- `chunked-csv-import-checkpoint-sequence`

README 파일은 PNG만 embed한다. SVG 파일은 review를 위해 PNG 파일 옆에 유지한다.
Graphviz `.dot`, `.plain`, `*-graphviz.svg`, `*-graphviz.png`는 route evidence로
남긴다. 최종 README SVG/PNG 자산은 raw Graphviz output이 아니라 decorated
workshop baseline을 사용해야 한다.

Generator는 구체적인 `margins=L/R/T/B` 값을 포함한 deterministic geometry
evidence를 출력해야 하며, 문서화된 threshold를 초과하면 rendering 전에
실패해야 한다.

## 문서

영어 및 한국어 README 파일에 다음을 추가한다.

- Example Scenario
- local batch demonstration 실행 방법
- output shape
- checkpoint key와 chunk size
- 실패한 chunk가 replay되는 이유
- writer가 customer ID 기준 idempotent인 이유
- Architecture
- Sequence Diagram
- durable checkpoint store, transaction, database upsert, audit record,
  retry/dead-letter handling, file schema evolution에 대한 production hardening
  note
- base checkpoint/restart track인 issue #41 link

루트 `README.md`와 `README.ko.md`를 갱신한다.

- example table row
- run section
- 0.5.0 batch example roadmap row
- workshop example map diagram

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- initial import가 완료되고 모든 fixture row를 쓰며 `NextRow`를 row count까지
  checkpoint하는지
- simulated mid-run failure가 failed status를 반환하고 `ErrSimulatedCrash`를
  wrap하며, partial customer를 한 번 commit하고 checkpoint를 이전 성공 chunk에
  유지하는지
- restart가 fresh reader를 만들고 checkpoint를 restore하며, 실패한 chunk를
  replay하고 duplicate customer ID를 skip하며, 파일을 완료하고 각 customer를
  정확히 한 번 쓰는지
- 잘못된 CSV header가 쓰기 전에 실패하는지
- 유효하지 않은 customer row가 row context와 함께 실패하는지
- processing 전 또는 중 cancellation이 `batch.StatusCancelled`를 반환하고
  resource를 닫으며 checkpoint state를 변경하지 않는지
- `go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...`

## 검증

- `bash scripts/generate-chunked-csv-import-checkpoint-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go test -run '^$' ./examples/chunked-csv-import-checkpoint`
- `git diff --check`
- `golangci-lint cache clean && make ci`
- GitHub PR checks
