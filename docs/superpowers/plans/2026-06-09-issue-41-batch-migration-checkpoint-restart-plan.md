# Issue #41 계획: Batch Migration Checkpoint Restart 예제

## 범위

focused 0.5.0 batch checkpoint/restart 예제로
`examples/account-migration-checkpoint-restart`를 구현한다.

## 작업

### T1 - 예제 패키지

- `internal/accountmigration`을 추가한다.
- account fixture, `MigrationCheckpoint`, result/report projection, sentinel
  error를 정의한다.
- `Restore`와 `Checkpoint`를 가진 checkpoint-aware reader를 구현한다.
- 첫 checkpointed chunk 뒤 deterministic crash injection이 있는 processor를 구현한다.
- idempotency 및 duplicate detection이 있는 target sink와 writer를 구현한다.
- `RunMigration` 및 `RunDemo`를 구현한다.

### T2 - 테스트

- 다음 항목에 대한 table/focused test를 추가한다.
  - first run이 first chunk checkpoint 뒤 실패한다.
  - restart가 `next_index=2`에서 재개되어 완료된다.
  - 완료된 first chunk는 다시 read/write되지 않는다.
  - final checkpoint는 `next_index=5`다.
  - invalid checkpoint type/range는 `ErrInvalidCheckpoint`를 wrap한다.
  - duplicate target write는 `ErrDuplicateAccount`를 wrap한다.
  - 작업 전 및 processing 중 cancellation.
  - timestamp-free report projection.
- 다음 항목에 대한 bounded stress test를 추가한다.
  - 독립 store/sink를 가진 concurrent complete migration 실행.
  - concurrent checkpoint store 및 target sink access.
- stress test는 일반 `go test`와 `go test -race`에서 모두 통과해야 한다.

### T3 - CLI

- `RunDemo`의 deterministic indented JSON을 출력하는 `main.go`를 추가한다.
- output에는 runtime timestamp를 넣지 않는다.

### T4 - README

- English 및 Korean README file을 추가한다.
- Example Scenario, Architecture, Sequence Diagram, checkpoint key, chunk size,
  restart contract, test, related example, production hardening을 포함한다.

### T5 - 다이어그램

- generator script를 추가한다.
  - `scripts/generate-account-migration-checkpoint-restart-diagrams.sh`
- scenario, architecture, sequence asset을 생성한다.
  - DOT
  - PLAIN
  - Graphviz SVG/PNG
  - decorated final SVG/PNG
- gate output은 concrete margin과 geometry failure 0을 포함해야 한다.
- 모든 rendered PNG를 점검한다.

### T6 - 루트 navigation

- root `README.md`와 `README.ko.md`에 example을 추가한다.
- workshop example map 및 generated map asset을 갱신한다.

### T7 - Review 및 검증

- Step 6-R code review artifact와 verifier checklist를 추가한다.
- 다음을 실행한다.
  - `bash scripts/generate-account-migration-checkpoint-restart-diagrams.sh`
  - `go test -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`
  - `go test -race -count=1 -run 'Stress|Concurrent' ./examples/account-migration-checkpoint-restart/internal/accountmigration`
  - `go test -count=1 ./examples/account-migration-checkpoint-restart/...`
  - `go test -race -count=1 ./examples/account-migration-checkpoint-restart/...`
  - `go run ./examples/account-migration-checkpoint-restart`
  - `go test -run '^$' ./examples/account-migration-checkpoint-restart`
  - `go vet ./examples/account-migration-checkpoint-restart/...`
  - `golangci-lint run ./examples/account-migration-checkpoint-restart/...`
  - `git diff --check`
  - `golangci-lint cache clean && make ci`

### T8 - PR

- Lore trailer로 commit한다.
- branch를 push한다.
- `Closes #41`가 있는 PR을 만든다.
- PR body가 비어 있지 않고 final section이 `## DoD Status`인지 검증한다.
- GitHub CI를 확인한다.

## Step 3 체크리스트 완료 보고

| 항목 | 상태 | 메모 |
|------|--------|-------|
| 모든 spec requirement가 task에 매핑됨 | Done | T1-T8이 implementation, test, docs, diagram, review, PR을 매핑한다. |
| task ordering 유효 | Done | source, test, CLI, docs/diagram, review, validation, PR. |
| race/stress 계획됨 | Done | T2와 T7이 normal 및 race stress run을 요구한다. |
| README 및 localized docs 포함 | Done | T4와 T6. |
| diagram gate 포함 | Done | T5가 generator gate와 PNG inspection을 요구한다. |
| verification command 구체적 | Done | T7이 exact command를 나열한다. |
