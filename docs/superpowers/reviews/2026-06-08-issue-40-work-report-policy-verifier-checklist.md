# Issue #40 verifier checklist

## 범위

- `docs/superpowers/research/2026-06-08-issue-40-work-report-policy-research.md`
- `docs/superpowers/specs/2026-06-08-issue-40-work-report-policy-design.md`
- `docs/superpowers/plans/2026-06-08-issue-40-work-report-policy-plan.md`
- `examples/operations-report-policy`
- `docs/images/readme-diagrams/operations-report-policy-*`
- root README navigation and workshop example map

## checklist

| 요구사항 | 근거 | 결과 |
|---|---|---|
| Step 2-R spec review 존재 | `docs/superpowers/reviews/2026-06-08-issue-40-work-report-policy-spec-review.md` | PASS |
| Step 3-R plan review 존재 | `docs/superpowers/reviews/2026-06-08-issue-40-work-report-policy-plan-review.md` | PASS |
| example runnable | `examples/operations-report-policy/main.go`와 compile smoke `go test -run '^$' ./examples/operations-report-policy` | PASS |
| deterministic report output | `server.go`의 stable DTO. test는 name, status, error, reason, summary count를 검증한다. | PASS |
| retry, skip, fail-fast behavior | test는 retry partial report, aborted skip, stop-on-failure truncation을 다룬다. | PASS |
| README scenario | `examples/operations-report-policy/README.md`와 `README.ko.md`는 Example Scenario section을 포함한다. | PASS |
| README architecture | English/Korean README는 PNG embed가 있는 Architecture section을 포함한다. | PASS |
| README sequence | English/Korean README는 PNG embed가 있는 Sequence Diagram section을 포함한다. | PASS |
| root navigation | `README.md`, `README.ko.md`, `workshop-example-map.png`는 `operations-report-policy`를 포함한다. | PASS |
| code review | `docs/superpowers/reviews/2026-06-08-issue-40-work-report-policy-code-review.md`에는 P0=0/P1=0이 있다. | PASS |

## 검증 명령

- `bash scripts/generate-operations-report-policy-diagrams.sh`
- `go test -count=1 ./examples/operations-report-policy/...`
- `go test -race -count=1 ./examples/operations-report-policy/...`
- `go test -run '^$' ./examples/operations-report-policy`
- `git diff --check`
- `make ci`

## 상태

최종 repository-wide validation은 PR body와 final report에 기록되어 있다.
