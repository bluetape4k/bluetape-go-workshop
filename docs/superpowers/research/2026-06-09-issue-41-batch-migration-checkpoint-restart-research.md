# Issue #41 리서치: Batch Migration Checkpoint Restart 예제

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #41 `[v0.5.0] Add batch migration checkpoint restart example`
- 마일스톤: `0.5.0`
- 상위 track: #29

## 현재 근거

- Worktree: `.worktrees/feat-issue-41-batch-migration-checkpoint-restart`
- Baseline: `origin/develop` at `a3529c5`
- 의존성: `github.com/bluetape4k/bluetape-go v0.6.0`
- CodeGraph: 이 저장소에는 초기화되어 있지 않다. 직접 source inspection과 GNO 결과를
  fallback evidence로 사용한다.

## 이슈 요구사항

#41은 다음을 만족하는 scenario-shaped batch migration 예제를 요구한다.

- account 또는 catalog row를 reader/processor/writer chunk로 처리한다.
- checkpoint를 영속화한다.
- 알려진 chunk 뒤에서 crash를 발생시킨다.
- checkpoint에서 restart한다.
- 완료된 chunk가 잘못 재처리되지 않음을 증명한다.
- checkpoint storage를 교체 가능하고 작게 유지한다.
- chunk size, checkpoint key, restart contract를 문서화한다.

## bluetape-go Batch API

v0.6.0의 `batch.StepOptions`는 다음을 지원한다.

- `CheckpointStore batch.CheckpointStore`
- `CheckpointKey string`

`batch.CheckpointStore`는 다음과 같다.

```go
type CheckpointStore interface {
    Load(context.Context, string) (any, bool, error)
    Save(context.Context, string, any) error
}
```

`batch.CheckpointReader`는 다음과 같다.

```go
type CheckpointReader interface {
    Restore(context.Context, any) error
    Checkpoint(context.Context) (any, bool, error)
}
```

`batch.Step`은 reader/writer open 뒤 checkpoint를 복원한다. filtered/skipped processor
item 뒤와 chunk write 성공 뒤 checkpoint를 저장한다. checkpoint가 활성화된 상태에서
writer chunk가 skip되면 `ErrUnsafeWriterSkipCheckpoint`가 안전하지 않은 checkpoint
전진을 막는다.

## 기존 워크숍 예제

- `examples/chunked-csv-import-checkpoint`: CSV-focused companion example이다. partial
  writer side effect, 마지막 safe cursor에서의 replay, idempotent duplicate handling을
  보여 준다.
- `examples/retry-dead-letter-batch-worker`: shared state와 retry behavior에 대한
  bounded stress test를 포함한 retry/dead-letter focused example이다.

#41은 CSV file import scenario를 반복하지 않아야 한다. 대신 성공한 chunk가
checkpoint된 뒤 나중에 crash가 발생하면 해당 checkpoint에서 restart하고 완료된 chunk를
다시 읽지 않아야 한다는 기본 migration restart contract를 가르쳐야 한다.

## 제안 예제 형태

예제 path:

- `examples/account-migration-checkpoint-restart`

시나리오:

- 다섯 개의 deterministic legacy account를 두 개 단위 chunk로 migration한다.
- 첫 실행은 첫 chunk를 write하고 checkpoint `next_index=2`를 저장한다.
- 이후 processor가 `acct-1003`에서 crash를 simulate한다.
- restart는 같은 checkpoint store와 target sink를 사용해 `next_index=2`를 복원하고
  `acct-1003`부터 `acct-1005`까지 migration한다.
- operation log는 restart 중 `acct-1001`과 `acct-1002`가 다시 read되거나 write되지
  않았음을 증명한다.

Checkpoint contract:

- `Checkpoint{NextIndex int}` only.
- `CheckpointKey = "account-migration-v1"`.
- storage는 `batch.CheckpointStore`를 통해 교체 가능하게 유지한다. 예제는 demo output을
  눈에 보이게 하기 위해 in-memory recording store를 사용한다.

## 테스트 요구사항

필수 focused test:

- restart가 `next_index=2`에서 이어진다.
- 완료된 첫 chunk가 restart에서 재처리되지 않는다.
- checkpoint value는 작고 typed다.
- invalid checkpoint type/range는 눈에 보이게 실패한다.
- cancellation은 cancelled status를 반환하고 checkpoint를 전진시키지 않는다.
- report projection은 timestamp를 생략한다.

필수 race/stress:

- independent store/sink 위에서 bounded concurrent complete migration을 수행한다.
- mutex protection과 uniqueness를 증명하기 위해 bounded concurrent sink/store access를
  수행한다.
- stress coverage를 일반 `go test`와 `go test -race` 아래에서 실행한다.

## 문서 요구사항

- 예제 README pair: `README.md`, `README.ko.md`.
- 필수 섹션: Example Scenario, Architecture, Sequence Diagram.
- PNG embed는 대응하는 SVG, DOT, PLAIN, Graphviz evidence가 있을 때만 사용한다.
- 루트 `README.md`, `README.ko.md`, workshop example map을 갱신한다.

## 위험과 결정

- durable checkpoint persistence는 범위 밖이다. production hardening에는 durable
  store, transactional write, idempotent migration, audit, metrics, replay tooling을
  명시해야 한다.
- 새 의존성은 필요하지 않다.
- 이것은 Gin API가 아니라 local batch example이다. Gin은 #42 같은 HTTP operations
  example에 남겨 둔다.
