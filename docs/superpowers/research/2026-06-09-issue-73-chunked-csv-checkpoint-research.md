# Issue #73 리서치: Chunked CSV Import Checkpoint 예제

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #73 `[v0.5.0] Add chunked CSV import checkpoint example`
- 마일스톤: `0.5.0`
- 작업 유형: Type A - Full Feature

## 확인한 출처

- GitHub issue #73.
- 관련 GitHub issue:
  - #29 `[v0.5.0] Add batch checkpoint and restart workshop examples`
  - #41 `[v0.5.0] Add batch migration checkpoint restart example`
- 현재 워크숍 예제와 루트 README navigation.
- module version
  `v0.5.1`:
  의 upstream `github.com/bluetape4k/bluetape-go/batch`
  - `batch/checkpoint.go`
  - `batch/interfaces.go`
  - `batch/step.go`
  - `batch/job.go`
  - `batch/report.go`
  - `batch/step_test.go`
- upstream batch research:
  - `bluetape-go/docs/research/2026-06-01-milestone-0.5.0-batch-research.md`
- GNO:
  - `gno query "issue 73 chunked CSV import checkpoint restart bluetape-go-workshop batch" -c bluetape4k-github --fast --no-rerank`
  - `gno query "0.5.0 batch checkpoint restart csv import chunk reader writer" -c bluetape4k-docs --no-rerank`
  - `gno query "issue 73 chunked CSV import checkpoint restart batch" -c wiki --no-rerank`

## 현재 근거

#73은 작은 fixture를 chunk로 처리하고, 마지막 successful checkpoint를 기록하며,
중간 실행에서 실패한 뒤 완료된 row를 중복하지 않고 restart하는 focused 0.5.0 CSV
import example을 요구한다. 또한 English/Korean README navigation과 base
checkpoint/restart track인 #41에 대한 README link를 요구한다.

#29는 batch processing example을 위한 0.5.0 umbrella이며 다음을 명시한다.

- deterministic test로 restart/checkpoint behavior를 증명한다.
- upstream batch support가 명시적으로 제공하지 않는 한 durable queue semantic은 범위
  밖으로 둔다.
- Gin은 HTTP operations boundary에서만 사용한다.

#41은 아직 열려 있으며 base checkpoint/restart example을 설명한다. 따라서 #73은 #41을
base track으로 연결하되, #41이 이미 구현되었다고 주장해서는 안 된다.

워크숍 저장소에는 현재 `examples/*batch*`, `*checkpoint*`, `*csv*` 예제가 없다. 기존
마일스톤 예제는 다음을 사용한다.

- 격리된 `examples/<name>` directory
- 구현을 위한 `internal/<domain>` package
- runnable `main.go`
- focused package test와 race test
- English/Korean README 파일
- `docs/images/readme-diagrams` 아래 decorated README diagram
- 루트 `README.md`와 `README.ko.md` navigation update

## upstream Batch API 근거

`batch.Reader[T]`는 lifecycle method를 소유한다.

- `Open(context.Context) error`
- `Read(context.Context) (T, bool, error)`
- `Close(context.Context) error`

`batch.Processor[I,O]`는 context를 받고 item 하나를 변환하거나 filter한다.

`batch.Writer[T]`는 `Write(context.Context, []T) error`로 chunk를 저장한다.

`batch.CheckpointReader`는 다음을 추가한다.

- `Restore(context.Context, any) error`
- `Checkpoint(context.Context) (any, bool, error)`

`batch.CheckpointStore`는 key별로 checkpoint를 저장하며, in-memory implementation은
test/local job에서 concurrency-safe하다.

`batch.Step.Run`은 read loop 전에 checkpoint를 복원하고, 처리된 item이
filtered/skipped된 뒤 또는 chunk write가 성공한 뒤에만 reader checkpoint를 저장한다.
따라서 chunk를 부분 commit한 뒤 writer failure가 발생하면 checkpoint를 안전하게
전진시킬 수 없다. restart 시 해당 failed chunk가 다시 읽힐 수 있으므로, 예제는 batch
boundary에서 duplicate prevention을 보여 주기 위해 customer ID를 key로 쓰는
idempotent writer가 필요하다.

`batch.Job`은 step report를 aggregate하고 첫 failing step 뒤 중단한다. `batch.Report`는
`Name`, `Status`, count, `Err`, child report를 노출하며, README/CLI output의 golden
output으로 노출하면 안 되는 runtime timestamp도 포함한다.

## GNO Results

`bluetape4k-github`는 package direction에 대한 closed evidence로 upstream
`bluetape-go` batch epic #5와 task #30을 반환했다. workshop issue #73은 직접 반환하지
않았다.

`bluetape4k-docs`는 다음을 반환했다.

- `bluetape-go/docs/research/2026-06-01-milestone-0.5.0-batch-research.md`
- 같은 reader/processor/writer, chunk, checkpoint, restart vocabulary를 사용하는
  Kotlin `bluetape4k-batch` design/plan document

`wiki` collection은 #73에 대한 직접 결과를 반환하지 않았다. 두 GNO command 모두 local
Metal backend warning인 `ggml_metal_library_init_from_source: error compiling source`를
출력했지만, GitHub/docs collection의 result set은 반환되었다.

## 설계 결정

`examples/chunked-csv-import-checkpoint` 아래 새 local runnable example을 만든다.

이 예제는 HTTP service가 아니라 local batch job이어야 한다. 이는 Gin을 HTTP operations
boundary에서만 사용하라는 #29의 규칙을 따른다. lesson은 web routing이 아니라 restart와
checkpoint semantic이다.

예제는 다음을 수행한다.

1. deterministic customer CSV fixture를 읽는다.
2. `ChunkSize=2`로 `batch.NewStep`을 통해 row를 처리한다.
3. 안정적인 checkpoint key 아래 `batch.MemoryCheckpointStore`에 checkpoint record를
   저장한다.
4. chunk를 부분 commit한 뒤 writer crash를 simulate한다.
5. 같은 checkpoint store와 customer sink 위에서 새 reader와 writer로 restart한다.
6. 완료된 row가 중복되지 않고, replay된 failed chunk가 customer ID 기준으로
   idempotent임을 증명한다.

## 거부한 선택지

- HTTP API: #73은 batch example이고 #29는 Gin을 HTTP operations boundary로 제한한다.
  HTTP를 추가하면 checkpoint semantic에서 주의가 흐려진다.
- durable database/checkpoint store: milestone API는 `CheckpointStore`를 제공하며,
  #73은 작은 deterministic fixture만 요구한다. durable store는 이후 integration
  example에 속한다.
- partial writer commit 뒤 checkpoint 전진: upstream step contract는 successful chunk
  write 뒤 checkpoint를 저장한다. failed writer 뒤 전진하면 silent data loss 위험이 있다.
- `batch.Reader`, `batch.Processor`, `batch.Writer`, `CheckpointReader`를 generic
  import service 뒤에 숨기기: 워크숍은 batch package surface를 보이게 해야 한다.

## 구현 제약

- fixture는 작고 deterministic하며 예제 로컬로 유지한다.
- `encoding/csv`와 upstream `batch` package를 사용하고 의존성을 추가하지 않는다.
- reader, processor, writer, checkpoint, runner boundary에서 context를 확인한다.
- success, failure, cancellation에서 reader와 writer resource를 닫는다.
- report를 runtime timestamp 없는 stable output으로 project한다.
- duplicate prevention을 명시적이고 domain-shaped로 유지한다. customer ID가
  idempotency key다.
- README 파일에는 다음을 포함해야 한다.
  - Example Scenario
  - Architecture
  - Sequence Diagram
  - base checkpoint/restart track인 #41 link
- diagram asset은 `bluetape4k-diagram`을 따라야 한다.
  - 최종 README PNG/SVG asset은 decorated workshop baseline을 사용한다.
  - Graphviz `.dot`, `.plain`, `*-graphviz.*` artifact는 route evidence로 남긴다.
  - generator는 구체적인 L/R/T/B margin evidence를 출력한다.
  - render된 각 PNG를 시각적으로 검사한다.

## 테스트 영향

focused test는 다음을 다루어야 한다.

- initial import가 성공하고 final CSV position을 checkpoint하는지
- simulated mid-run writer failure가 checkpoint를 이전 successful chunk에 남기는지
- restart가 checkpoint에서 복원하고, failed chunk를 replay하며, 이미 부분 commit된
  customer ID를 skip하고 모든 row를 완료하는지
- malformed CSV 또는 invalid row가 눈에 보이게 실패하는지
- caller cancellation이 cancelled batch report를 반환하고 checkpoint를 전진시키지 않는지
- success/failure/cancellation에서 resource가 닫히는지
- example package에 대한 race test
