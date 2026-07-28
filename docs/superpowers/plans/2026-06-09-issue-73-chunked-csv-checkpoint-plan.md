# Issue #73 계획: Chunked CSV Import Checkpoint 예제

## 작업 유형

Type A - Full Feature.

이유: 새 runnable example directory, Go implementation, fixture, test, bilingual
README, generated diagram, root navigation update, review artifact, PR이 필요하다.

## 구현 작업

1. `examples/chunked-csv-import-checkpoint/testdata/customers.csv`를 추가한다.
   - deterministic customer row 5개.
   - Header exactly `customer_id,email,tier`.
2. `examples/chunked-csv-import-checkpoint/internal/csvimport`를 추가한다.
   - Domain types: `CSVRow`, `Customer`, `Checkpoint`, result DTOs.
   - Sentinel errors for invalid CSV/header/row and simulated crash.
   - Stable JSON projection helpers that omit `batch.Report` timestamps.
3. CSV reader를 구현한다.
   - Parse fixture with `encoding/csv`.
   - header 및 field count를 검증한다.
   - Track `NextRow` over data rows.
   - Implement `batch.CheckpointReader`.
   - Check context in `Open`, `Read`, `Restore`, `Checkpoint`, and `Close`.
   - test용 close state를 기록한다.
4. processor를 구현한다.
   - Trim customer ID, email, and tier.
   - Normalize email/tier to lowercase.
   - row context와 함께 blank field를 거부한다.
   - processing 전에 context를 확인한다.
5. in-memory idempotent writer/sink를 구현한다.
   - customer ID를 idempotency key로 사용한다.
   - Track new commits, duplicate skips, write attempts, and committed order.
   - configurable new commit 수 뒤 crash를 simulate한다.
   - write 전과 중간에 context를 확인한다.
   - test용 close state를 기록한다.
6. runner를 구현한다.
   - Build `batch.Step[CSVRow, Customer]` with chunk size `2` and checkpoint
     key `customer-csv-import`.
   - Wrap step in `batch.Job`.
   - Provide a demo function that runs first failure plus restart over the same
     checkpoint store and sink.
   - Keep public result output deterministic and timestamp-free.
7. `examples/chunked-csv-import-checkpoint/main.go`를 추가한다.
   - Run the demo with `context.Background()`.
   - Print indented JSON.
   - demo setup 자체가 예기치 않게 실패할 때만 non-zero로 exit한다.
8. focused test를 추가한다.
   - initial import success는 모든 fixture row를 write하고 final row를 checkpoint한다.
   - mid-run simulated crash는 checkpoint를 이전 successful chunk에 유지한다.
   - restart replays the failed chunk, records one duplicate skip, and commits
     각 customer를 정확히 한 번 commit한다.
   - malformed header는 write 전에 실패한다.
   - invalid row는 row context와 함께 실패한다.
   - cancellation before work and during processing returns
     `batch.StatusCancelled`, closes any opened resources, and only preserves
     이전에 committed checkpoint만 보존한다.
   - example package에 대한 race validation.
9. README file을 추가한다.
   - `examples/chunked-csv-import-checkpoint/README.md`
   - `examples/chunked-csv-import-checkpoint/README.ko.md`
   - Include Example Scenario, run command, sample output, checkpoint/chunk
     explanation, Architecture, Sequence Diagram, production hardening, and #41
     link.
10. `scripts/generate-chunked-csv-import-checkpoint-diagrams.sh`를 추가한다.
    - Generate `.dot`, `.plain`, `*-graphviz.svg`, `*-graphviz.png`.
    - Generate final decorated SVG/PNG assets for scenario, architecture, and
      sequence diagrams.
    - Print geometry gate output with concrete `margins=L/R/T/B`.
    - font role과 forbidden UI font를 검증한다.
11. root navigation을 갱신한다.
    - `README.md` example table, run section, 0.5.0 roadmap row.
    - `README.ko.md` equivalent Korean updates.
    - `workshop-example-map` diagram asset 및 route evidence.
12. 구현 뒤 Step 6-R review/verifier artifact를 추가한다.
    - Code review findings with P0/P1/P2/P3 counts.
    - command evidence를 포함한 verifier checklist.

## 검증 계획

- `bash scripts/generate-chunked-csv-import-checkpoint-diagrams.sh`
- visual inspection of:
  - `docs/images/readme-diagrams/chunked-csv-import-checkpoint-scenario.png`
  - `docs/images/readme-diagrams/chunked-csv-import-checkpoint-architecture.png`
  - `docs/images/readme-diagrams/chunked-csv-import-checkpoint-sequence.png`
  - `docs/images/readme-diagrams/workshop-example-map.png`
- `go test -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go test -race -count=1 ./examples/chunked-csv-import-checkpoint/...`
- `go run ./examples/chunked-csv-import-checkpoint`
- `go test -run '^$' ./examples/chunked-csv-import-checkpoint`
- `git diff --check`
- `golangci-lint cache clean && make ci`
- GitHub PR checks

## 위험과 완화

| 위험 | 완화 |
|---|---|
| checkpoint semantic이 off-by-one이 됨 | test가 failed second chunk 뒤 `NextRow=2`, restart 뒤 `NextRow=5`를 assert한다. |
| domain-level commit이 `batch.Report.WriteCount`와 혼동됨 | result DTO와 test가 sink-level new commit 및 duplicate skip을 report count와 별도로 추적한다. |
| writer failure가 restart에서 row를 조용히 중복 | sink는 customer ID idempotency를 사용하고 test는 committed ID가 unique인지 assert한다. |
| cancellation이 checkpoint를 너무 멀리 전진 | cancellation test가 checkpoint는 성공한 이전 chunk만 반영한다고 assert한다. |
| 예제가 durability를 과장 | README production hardening이 memory store는 demo-only이고 durable store/transaction은 production work라고 명시한다. |
| diagram output이 이전 margin/decorator 결함 반복 | generator가 concrete margin value를 출력하고 font role을 검증하며 Graphviz evidence를 내고 final PNG를 점검한다. |

## Step 3 체크리스트 완료 보고

| 항목 | 상태 | 메모 |
|---|---|---|
| 모든 spec requirement가 task에 매핑됨 | Done | task가 code, test, README, diagram, navigation, review, validation을 포함한다. |
| dependency 순서가 올바름 | Done | fixture/domain이 runner/test/docs보다 앞서고 validation은 generation 뒤에 온다. |
| concrete verification command 명시 | Done | targeted test, race, run, diff, CI, PR check가 나열되어 있다. |
| user-visible docs 포함 | Done | EN/KO example README와 root README update가 명시되어 있다. |
| scope가 approved spec으로 제한됨 | Done | HTTP, durable store, queue, scheduler, new dependency task는 포함되지 않는다. |
