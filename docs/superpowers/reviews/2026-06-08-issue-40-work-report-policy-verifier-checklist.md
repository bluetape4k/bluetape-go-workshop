# Issue #40 Verifier Checklist

## Scope

- `docs/superpowers/research/2026-06-08-issue-40-work-report-policy-research.md`
- `docs/superpowers/specs/2026-06-08-issue-40-work-report-policy-design.md`
- `docs/superpowers/plans/2026-06-08-issue-40-work-report-policy-plan.md`
- `examples/operations-report-policy`
- `docs/images/readme-diagrams/operations-report-policy-*`
- root README navigation and workshop example map

## Checklist

| Requirement | Evidence | Result |
|---|---|---|
| Step 2-R spec review exists | `docs/superpowers/reviews/2026-06-08-issue-40-work-report-policy-spec-review.md` | PASS |
| Step 3-R plan review exists | `docs/superpowers/reviews/2026-06-08-issue-40-work-report-policy-plan-review.md` | PASS |
| Example is runnable | `examples/operations-report-policy/main.go` and compile smoke `go test -run '^$' ./examples/operations-report-policy` | PASS |
| Deterministic report output | Stable DTO in `server.go`; tests assert names, statuses, errors, reasons, and summary counts. | PASS |
| Retry, skip, fail-fast behavior | Tests cover retry partial report, aborted skip, and stop-on-failure truncation. | PASS |
| README scenario | `examples/operations-report-policy/README.md` and `README.ko.md` include Example Scenario sections. | PASS |
| README architecture | English and Korean READMEs include Architecture sections with PNG embeds. | PASS |
| README sequence | English and Korean READMEs include Sequence Diagram sections with PNG embeds. | PASS |
| Root navigation | `README.md`, `README.ko.md`, and `workshop-example-map.png` include `operations-report-policy`. | PASS |
| Code review | `docs/superpowers/reviews/2026-06-08-issue-40-work-report-policy-code-review.md` has P0=0/P1=0. | PASS |

## Verification Commands

- `bash scripts/generate-operations-report-policy-diagrams.sh`
- `go test -count=1 ./examples/operations-report-policy/...`
- `go test -race -count=1 ./examples/operations-report-policy/...`
- `go test -run '^$' ./examples/operations-report-policy`
- `git diff --check`
- `make ci`

## Status

Final repository-wide validation is recorded in the PR body and final report.
