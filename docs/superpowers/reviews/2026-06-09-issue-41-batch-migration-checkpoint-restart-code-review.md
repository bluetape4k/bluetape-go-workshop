# Issue 41 Code Review

Branch: `feat/issue-41-batch-migration-checkpoint-restart`
범위:
- `examples/account-migration-checkpoint-restart/**`
- `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`
- `docs/images/readme-diagrams/account-migration-checkpoint-restart-*`
- root `README.md`, `README.ko.md`, `workshop-example-map` asset
- `examples/chunked-csv-import-checkpoint/internal/csvimport/importer_test.go` CI lint follow-up: GitHub staticcheck `SA5011`을 만족하도록 nil-check `t.Fatalf` 뒤 explicit `return` 추가.

CodeGraph note: `CodeGraph not initialized in /Users/debop/work/bluetape4k/bluetape-go-workshop`.
structural impact는 branch diff, changed import, direct call surface로 검토했다.

## Tier finding

| Tier | 초점 | P0 | P1 | P2 | P3 | 결과 |
|---|---|---:|---:|---:|---:|---|
| 1 | Security | 0 | 0 | 0 | 0 | network, auth, secret, SQL, deserialization, user-controlled trust boundary가 도입되지 않았다. |
| 2 | Ops/SRE reliability | 0 | 0 | 0 | 0 | reader, processor, writer, checkpoint store에서 context cancellation을 확인한다. deterministic report는 restart evidence를 노출한다. |
| 3 | Structural impact | 0 | 0 | 0 | 0 | 새 standalone example만 추가되며 shared package API나 module dependency change는 없다. |
| 4 | Go code quality | 0 | 0 | 0 | 0 | `batch.StepOptions`, typed error, context propagation, defensive copy, mutex-protected shared store를 사용한다. |
| 5 | Tests/types/silent failure | 0 | 0 | 0 | 0 | test는 checkpoint cursor, restart read ID, invalid checkpoint failure, cancellation, deterministic projection, concurrency, stress를 검증한다. |
| 6 | Performance/stability | 0 | 0 | 0 | 0 | unbounded production goroutine, retry, polling, hot-path reflection이 없다. bounded stress가 shared state를 다룬다. |
| 7 | Docs/release/evidence | 0 | 0 | 0 | 0 | English/Korean README와 scenario/architecture/sequence diagram이 추가됐다. root map과 run docs가 업데이트됐다. |

최종 blocker gate: `P0 = 0`, `P1 = 0`.

## 근거

- restart contract: `RunDemo`는 first/restart run 사이에 하나의 checkpoint store와 target store를 공유한 뒤 final checkpoint와 checkpoint activity를 기록한다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:127`).
- step wiring: `RunMigration`은 chunk size, `CheckpointStore`, `CheckpointKey`가 있는 checkpoint-aware `batch.Step`을 만든다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:170`).
- restore contract: `MigrationReader.Restore`는 cursor를 옮기기 전에 `MigrationCheckpoint`를 검증하고 `NextIndex`를 bounds check한다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:293`).
- save contract: `MigrationReader.Checkpoint`는 다음 unread source index를 반환한다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:312`).
- shared state safety: `TargetAccountStore`와 `RecordingCheckpointStore`는 mutex로 map과 operation log를 보호한다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:368`, `examples/account-migration-checkpoint-restart/internal/accountmigration/migration.go:496`).
- restart assertion: test는 restart가 `acct-1003..acct-1005`를 읽고 `acct-1001` 또는 `acct-1002`를 다시 처리하지 않음을 증명한다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:16`).
- invalid checkpoint assertion: test는 wrong type, negative index, out-of-range index를 다룬다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:56`).
- concurrency/stress assertion: test는 concurrent demo와 concurrent checkpoint/target store access를 실행한다(`examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:158`, `examples/account-migration-checkpoint-restart/internal/accountmigration/migration_test.go:190`).
- README coverage: scenario, Architecture, Sequence Diagram, checkpoint contract, test command, production hardening이 문서화되어 있다(`examples/account-migration-checkpoint-restart/README.md:8`).
- diagram gate: `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`는 font, marker size, margin을 검증하고 concrete gate line을 출력한다(`scripts/generate-account-migration-checkpoint-restart-diagrams.sh:58`).

## Risk Pattern Scan

명령:

```bash
rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" examples/account-migration-checkpoint-restart
```

결과:
- `context.Background()`는 CLI/test/demo call site와 `normalizeContext(nil)` fallback에만 나타난다.
- `go func`는 `TestConcurrentDemoRunsRemainDeterministic`와 `TestStoresStressConcurrentAccess`에만 나타난다.
- `context.TODO`, `time.Tick`, `http.ListenAndServe`, `panic`, `RealIP`, `X-Forwarded-For` hit는 없다.

## 검증 snapshot

- `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`: PASS
  - scenario: `nodes=8 routes=6 segments=9 margins=44/44/34/34 fontFallback=0`
  - architecture: `nodes=8 routes=8 segments=13 margins=44/44/34/34 fontFallback=0`
  - sequence: `nodes=10 routes=10 segments=10 margins=44/44/34/34 fontFallback=0`
- PNG visual inspection: scenario, architecture, sequence, `workshop-example-map.png`에서 PASS.
- `go test -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`: PASS
- `go test -race -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`: PASS
- `go test -count=1 ./examples/account-migration-checkpoint-restart/...`: PASS
- `go test -race -count=1 ./examples/account-migration-checkpoint-restart/...`: PASS
- `go run ./examples/account-migration-checkpoint-restart`: PASS
- `go vet ./examples/account-migration-checkpoint-restart/...`: PASS
- `golangci-lint run ./examples/account-migration-checkpoint-restart/...`: PASS
- `golangci-lint run ./examples/chunked-csv-import-checkpoint/internal/csvimport --timeout=5m`: PASS
- `go test -count=1 ./examples/chunked-csv-import-checkpoint/internal/csvimport`: PASS
- `git diff --check`: PASS
- `go test -run '^$' ./examples/account-migration-checkpoint-restart/...`: PASS
- `golangci-lint cache clean && make ci`: PASS
