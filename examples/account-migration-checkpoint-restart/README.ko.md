# Account Migration Checkpoint Restart

이 예제는 legacy account row를 고정 chunk로 처리하고, restart checkpoint를 저장한
뒤, 지정된 account에서 실패하고, 저장된 cursor부터 재시작하는 local `batch`
migration을 보여줍니다.

## 예제 시나리오

Fixture에는 legacy account 5개가 있습니다. 첫 실행은 `acct-1001`,
`acct-1002`를 write한 뒤 `account-migration-v1` key에
`MigrationCheckpoint{NextIndex: 2}`를 저장합니다. 이후 processor가
`acct-1003`에서 실패하므로 batch report는 failed가 되고 checkpoint는 index
`2`에 남습니다.

Restart 실행은 같은 checkpoint key와 store를 사용합니다. `MigrationReader`는
`NextIndex: 2`를 restore하고 `acct-1003`, `acct-1004`, `acct-1005`를 읽은
뒤 `NextIndex: 5`로 완료합니다. 이미 완료된 첫 chunk는 restart 실행에서 다시
읽지 않습니다.

![Account migration checkpoint restart scenario](../../docs/images/readme-diagrams/account-migration-checkpoint-restart-scenario.png)

## Architecture

`RunDemo`는 두 번의 batch run이 하나의 `CheckpointStore`와 하나의
`TargetAccountStore`를 공유하도록 구성합니다. 각 run은 checkpoint-aware
`batch.Step` 하나를 가진 `batch.Job`을 만듭니다.

`MigrationReader`는 `batch.CheckpointReader`를 구현하고 checkpoint를 다음에
읽을 source index로 해석합니다. `TargetAccountWriter`는 committed chunk를
`TargetAccountStore`에 쓰며, store는 account ID uniqueness와 write order를
기록합니다. `RecordingCheckpointStore`는 의도적으로 교체 가능하게 만들었습니다.
Production에서는 step contract를 바꾸지 않고 durable storage로 바꿀 수 있습니다.

![Account migration checkpoint restart architecture](../../docs/images/readme-diagrams/account-migration-checkpoint-restart-architecture.png)

## Sequence Diagram

Checkpoint는 chunk write가 성공한 뒤에만 전진합니다. 다음 chunk가 commit되기
전에 processor가 실패하면 checkpoint는 이전 safe cursor에 남습니다. Restart는
같은 key를 load하고 index `2`부터 재개합니다.

![Account migration checkpoint restart sequence](../../docs/images/readme-diagrams/account-migration-checkpoint-restart-sequence.png)

## 실행

```bash
go run ./examples/account-migration-checkpoint-restart
```

출력은 deterministic JSON입니다. 핵심 필드는 다음과 같습니다:

```json
{
  "checkpoint_key": "account-migration-v1",
  "chunk_size": 2,
  "first_run": {
    "checkpoint": { "next_index": 2 },
    "read_ids": ["acct-1001", "acct-1002", "acct-1003"],
    "written_ids": ["acct-1001", "acct-1002"]
  },
  "restart_run": {
    "checkpoint": { "next_index": 5 },
    "read_ids": ["acct-1003", "acct-1004", "acct-1005"],
    "written_ids": ["acct-1003", "acct-1004", "acct-1005"]
  }
}
```

## Checkpoint Contract

- `CheckpointKey`: `account-migration-v1`
- `ChunkSize`: `2`
- Checkpoint payload: `MigrationCheckpoint{NextIndex int}`
- Save rule: chunk write가 성공한 뒤 저장
- Restart rule: 첫 read 전에 `NextIndex` restore
- Store contract: recording memory store 대신 어떤 `batch.CheckpointStore`도 사용 가능

## 테스트

```bash
go test -count=1 ./examples/account-migration-checkpoint-restart/...
go test -race -count=1 ./examples/account-migration-checkpoint-restart/...
go test -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration
go test -race -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration
```

검증 범위는 restart correctness, invalid checkpoint handling, cancellation,
deterministic report projection, store concurrency, bounded stress, race
detection입니다.

## 관련 예제

- [`examples/chunked-csv-import-checkpoint`](../chunked-csv-import-checkpoint):
  writer가 partial commit 이후 실패할 때 replay와 duplicate handling을 다룹니다.
- [`examples/retry-dead-letter-batch-worker`](../retry-dead-letter-batch-worker):
  item-level failure의 retry, skip, dead-letter policy를 다룹니다.

## Production Hardening

이 pattern을 service로 옮길 때는 durable checkpoint store, transactional target
write, idempotent target key, audit event, 명시적인 retry/dead-letter policy를
추가해야 합니다.
