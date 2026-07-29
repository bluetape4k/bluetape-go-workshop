# Issue #41 설계: Batch Migration Checkpoint Restart 예제

## 목표

Account migration job에서 checkpoint restore를 설명하는 집중 milestone 0.5.0
batch 예제를 추가한다. 이 예제는 restart 이후 완료된 chunk가 다시 처리되지
않는다는 점을 증명해야 한다.

## 비목표

- Durable queue processing.
- Distributed scheduling 또는 leader election.
- HTTP/Gin operations API.
- Database-backed checkpoint storage.
- Partial writer side-effect replay. 이는 `chunked-csv-import-checkpoint`에서
  다룬다.

## 예제

경로:

- `examples/account-migration-checkpoint-restart`

도메인:

- `LegacyAccount`: 결정적 input row.
- `TargetAccount`: 정규화된 migrated row.
- `MigrationCheckpoint`: `{next_index}` cursor.
- `RecordingCheckpointStore`: demo output과 테스트를 위해 load/save activity를
  기록하는 작은 in-memory `batch.CheckpointStore` wrapper.
- `TargetAccountStore`: write/read log를 가진 idempotent in-memory sink.

상수:

- `JobName = "account-migration"`
- `StepName = "legacy-account-migration"`
- `DefaultCheckpointKey = "account-migration-v1"`
- `DefaultChunkSize = 2`

## 시나리오 계약

입력 account:

- `acct-1001`
- `acct-1002`
- `acct-1003`
- `acct-1004`
- `acct-1005`

첫 번째 실행:

- `acct-1001`, `acct-1002`를 읽고 쓴다.
- checkpoint `next_index=2`를 저장한다.
- `acct-1003` 처리 중 실패한다.
- target store에는 migrated account 두 개가 남는다.

Restart 실행:

- checkpoint `next_index=2`를 load한다.
- `acct-1003`부터 시작한다.
- `acct-1003`, `acct-1004`, `acct-1005`를 쓴다.
- final checkpoint `next_index=5`를 저장한다.
- `acct-1001`, `acct-1002`가 다시 읽히거나 쓰이지 않았음을 증명한다.

## API 및 오류 계약

Public package API는 예제 크기를 유지해야 한다.

- `RunDemo(ctx context.Context) (DemoResult, error)`
- `RunMigration(ctx context.Context, options RunOptions) (MigrationRun, error)`
- 테스트에 필요한 reader/store/sink constructor.

Sentinel error:

- `ErrInvalidAccount`
- `ErrMigrationCrash`
- `ErrInvalidCheckpoint`
- `ErrDuplicateAccount`

Processor, reader restore, writer가 반환하는 error는 테스트와 caller가
`errors.Is`를 사용할 수 있도록 sentinel을 `%w`로 wrap해야 한다.

`nil` context는 `context.Background()`로 정규화한다.

## Checkpoint 계약

- Checkpoint 값은 typed `MigrationCheckpoint`여야 한다.
- 저장된 값은 `NextIndex`만 포함해야 한다.
- `Restore`는 잘못된 type과 범위를 벗어난 cursor 값을 거부해야 한다.
- Checkpoint save는 성공한 chunk write 또는 안전하게 filtered/skipped 된 item
  이후 `batch.Step`을 통해서만 일어나야 한다.
- `RunMigration`은 어떤 `batch.CheckpointStore`도 받을 수 있어야 하며, demo는
  `RecordingCheckpointStore`를 사용한다.

## 동시성 및 Stress 계약

이 예제는 checkpoint store와 target store에 공유 mutable state가 있으므로,
테스트는 bounded stress coverage를 포함해야 한다.

- 독립 store/sink를 사용하는 동시 complete demo run.
- uniqueness와 ordering assertion을 포함한 concurrent store/sink access.
- stress test가 일반 `go test`에서 통과한다.
- 같은 stress test가 `go test -race`에서도 통과한다.

## 문서 및 다이어그램 계약

예제 README 파일은 다음을 포함해야 한다.

- Example Scenario,
- Architecture,
- Sequence Diagram,
- checkpoint key와 chunk size,
- restart contract,
- test와 production hardening.

다이어그램 자산:

- `account-migration-checkpoint-restart-scenario.{dot,plain,svg,png}`
- `account-migration-checkpoint-restart-architecture.{dot,plain,svg,png}`
- `account-migration-checkpoint-restart-sequence.{dot,plain,svg,png}`
- matching `*-graphviz.svg/png` evidence files.

최종 README embed는 PNG만 사용해야 한다.

## Acceptance Test

필수 테스트:

- 첫 번째 실행은 checkpoint된 첫 chunk 이후 실패하고 restart는 완료된다.
- 완료된 첫 chunk는 restart에서 다시 처리되지 않는다.
- final checkpoint는 `next_index=5`다.
- 잘못된 checkpoint type과 range는 `ErrInvalidCheckpoint`로 실패한다.
- 작업 전 cancellation은 `batch.StatusCancelled`를 반환하고 write를 남기지
  않는다.
- 처리 중 cancellation은 cancelled status를 반환하고 retry하지 않는다.
- duplicate target write는 `ErrDuplicateAccount`로 실패한다.
- report projection은 런타임 timestamp를 생략한다.
- bounded stress normal/race coverage.

## Step 2 Checklist 완료 보고

| 항목 | 상태 | 메모 |
|------|--------|-------|
| Target repository confirmed | 완료 | `origin/develop` 기준 `bluetape4k/bluetape-go-workshop` worktree. |
| Relevant memory/GNO searched | 완료 | GNO에서 bluetape-go #5/#30/#153 및 workshop #73 산출물을 확인했다. |
| User intent and boundaries clear | 완료 | 사용자는 다음 예제 진행을 요청했고, #41은 #75 이전의 다음 집중 prerequisite이다. |
| Existing APIs inspected | 완료 | `batch.Step`, `CheckpointReader`, `CheckpointStore`, `MemoryCheckpointStore`. |
| Race/stress expectations explicit | 완료 | 공유 상태, checkpoint, ordering, uniqueness 계약에 필요하다고 spec에 명시했다. |
| Diagram requirements explicit | 완료 | PNG embed와 Graphviz evidence를 포함한 Scenario, Architecture, Sequence Diagram. |
