# Issue #40 Plan: Operations Report Policy Example

## Objective

Create `examples/operations-report-policy`, a Gin API that demonstrates
deterministic `workreport` output and the behavioral difference between
`StopOnFailure` and `ContinueOnFailure`.

## Implementation Steps

1. Add the example package skeleton:
   - `examples/operations-report-policy/main.go`
   - `examples/operations-report-policy/internal/operations/server.go`
   - `examples/operations-report-policy/internal/operations/server_test.go`
2. Implement a request model with `run_id`, `policy`, `products_valid`,
   `retry_partner_notification`, and `skip_search_index`.
3. Implement report construction:
   - check `ctx.Err()` before building the run and return
     `workreport.Cancelled("operations-run", err)` when cancelled
   - create deterministic child reports
   - aggregate with the requested `workreport.FailurePolicy`
4. Implement stable projection:
   - report node DTO without timestamps
   - summary counts over root and all descendants
   - HTTP status mapper for completed, partial, failed, aborted, and cancelled
5. Add focused tests for endpoint behavior, failure policies, retry evidence,
   skip mapping, invalid input, cancellation, and summary counts.
6. Add README.md and README.ko.md with:
   - scenario
   - report fields
   - failure policy behavior
   - production hardening gaps
   - architecture and sequence diagram sections
7. Add `scripts/generate-operations-report-policy-diagrams.sh` and generate PNG
   plus SVG assets.
8. Update root README.md and README.ko.md example tables, run section, and
   v0.4.0 roadmap wording.
9. Add a lessons note and Step 6-R code review artifact.
10. Verify with targeted tests, race test, diagram inspection, diff check, and
    repository CI gate before opening the PR.

## Validation Commands

```bash
bash scripts/generate-operations-report-policy-diagrams.sh
go test -count=1 ./examples/operations-report-policy/...
go test -race -count=1 ./examples/operations-report-policy/...
go test -run '^$' ./examples/operations-report-policy
git diff --check
make ci
```

## PR Deliverables

- implementation commit(s) after the planning commit
- PR body in English ending with `## DoD Status`
- evidence for tests and CI checks
- no merge until explicitly requested
