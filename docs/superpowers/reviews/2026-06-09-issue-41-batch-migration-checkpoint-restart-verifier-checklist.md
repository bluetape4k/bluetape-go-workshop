# Issue 41 verifier checklist

Branch: `feat/issue-41-batch-migration-checkpoint-restart`

| 점검 | 결과 | 근거 |
|---|---|---|
| issue scope | PASS | #41을 새 account migration checkpoint restart example로 구현한다. |
| worktree isolation | PASS | 작업은 `.worktrees/feat-issue-41-batch-migration-checkpoint-restart` 아래에서 완료됐다. |
| Step 2-R spec gate | PASS | `docs/superpowers/reviews/2026-06-09-issue-41-batch-migration-checkpoint-restart-spec-review.md`는 `P0=0`, `P1=0`을 기록한다. |
| Step 3-R plan gate | PASS | `docs/superpowers/reviews/2026-06-09-issue-41-batch-migration-checkpoint-restart-plan-review.md`는 `P0=0`, `P1=0`을 기록한다. |
| Step 6-R review gate | PASS | `docs/superpowers/reviews/2026-06-09-issue-41-batch-migration-checkpoint-restart-code-review.md`는 `P0=0`, `P1=0`을 기록한다. |
| restart correctness | PASS | first run은 checkpoint `next_index=2`로 실패한다. restart는 `acct-1003`, `acct-1004`, `acct-1005`만 읽는다. final checkpoint는 `next_index=5`다. |
| completed chunk 재처리 방지 | PASS | `TestRunDemoRestartsFromCheckpoint`는 `acct-1001` 또는 `acct-1002`를 포함하는 restart read를 거부한다. |
| replaceable checkpoint store | PASS | `RunOptions.CheckpointStore`는 임의의 `batch.CheckpointStore`를 받는다. example은 stable evidence를 위해 `RecordingCheckpointStore`를 사용한다. |
| invalid checkpoint handling | PASS | wrong type, negative index, out-of-range index는 write 없이 failed report를 만든다. |
| cancellation behavior | PASS | test는 work 전 cancellation과 checkpoint advancement 없는 processing 중 cancellation을 다룬다. |
| race/stress coverage | PASS | `Concurrent`와 `Stress` test가 normal run과 `-race` run에서 모두 통과했다. |
| documentation | PASS | example README와 README.ko는 scenario, Architecture, Sequence Diagram, checkpoint contract, test, production hardening을 포함한다. |
| diagram skill compliance | PASS | generated scenario, architecture, sequence PNG에는 outer frame, balanced margin, semantic connector color가 있고 visual inspection에서 visible overlap이 없다. |
| root navigation | PASS | `README.md`, `README.ko.md`, `workshop-example-map.png`는 `account-migration-checkpoint-restart`를 포함한다. |
| CI lint follow-up | PASS | 기존 CSV checkpoint test helper는 GitHub staticcheck `SA5011`을 닫기 위해 nil-check `t.Fatalf` 뒤 explicit return을 수행한다. |
| repository-wide CI | PASS | feature worktree에서 `golangci-lint cache clean && make ci`가 통과했다. |
| CodeGraph | GAP | repository가 codegraph로 initialized되지 않아 CodeGraph를 사용할 수 없었다. review는 local diff와 rg evidence를 사용했다. |

## 검증 명령

```bash
scripts/generate-account-migration-checkpoint-restart-diagrams.sh
go test -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration
go test -race -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration
go test -count=1 ./examples/account-migration-checkpoint-restart/...
go test -race -count=1 ./examples/account-migration-checkpoint-restart/...
go run ./examples/account-migration-checkpoint-restart
go vet ./examples/account-migration-checkpoint-restart/...
golangci-lint run ./examples/account-migration-checkpoint-restart/...
golangci-lint run ./examples/chunked-csv-import-checkpoint/internal/csvimport --timeout=5m
go test -count=1 ./examples/chunked-csv-import-checkpoint/internal/csvimport
git diff --check
go test -run '^$' ./examples/account-migration-checkpoint-restart/...
golangci-lint cache clean && make ci
```

위 명령은 모두 현재 worktree에서 통과했다.
