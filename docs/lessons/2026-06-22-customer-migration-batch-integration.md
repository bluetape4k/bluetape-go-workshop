# Customer Migration Batch Integration Lessons

## Context

- Issue #29 is the v0.5.0 batch umbrella.
- This implementation closes the remaining child scope:
  - #42 Gin batch operations API.
  - #43 leader-guarded scheduled batch.
  - #75 customer migration integration example.
- Focused prerequisites #41, #73, and #74 already covered checkpoint restart,
  policy reporting, and retry/dead-letter behavior. The useful gap was a
  runnable composition example, not another focused primitive demo.

## Implementation Notes

- Keep the example self-contained under `examples/customer-migration-batch-integration`.
  Existing example packages are under `internal`, so importing them across
  sibling examples would break Go visibility rules.
- Use fixed `DefaultChunkSize = 2` for the tutorial. The simulated crash writes
  three new customers but rolls the checkpoint back to the last complete chunk
  so the restart visibly replays the boundary chunk.
- Store emails internally but omit them from public JSON projections. The
  README and tests should speak in IDs, counts, checkpoints, and stable error
  codes.
- Keep leader logic explicit. `leader.ErrAlreadyLeader` means this process may
  run the tick but should not resign a lease acquired elsewhere in the call.
- Use `context.WithoutCancel(ctx)` only for bounded cleanup, such as best-effort
  leader resign after caller cancellation. Normal batch work must keep caller
  cancellation semantics.
- Treat the HTTP API as an unauthenticated operations demo. Default and override
  binds must stay loopback-only unless a separate auth/trust-boundary design is
  added.

## Validation Commands

```bash
make lint
go test -count=1 ./examples/customer-migration-batch-integration/...
go test -race -count=1 ./examples/customer-migration-batch-integration/...
GOFLAGS=-p=1 make ci
git diff --check
```

Live smoke coverage used the README flow:

- held leader: health, manual crash, status, schedule restart, report, idle
  cancel, malformed JSON, blank run id, and oversized body.
- missing leader: schedule rejection with `not_leader` and status showing
  `leader_held=false`.

## Review Outcome

- Step 5 verifier: PASS.
- Step 4-P performance/stability scan: no findings.
- Step 6-R six-lane review: P0 = 0, P1 = 0.
- Review artifact: `docs/review/2026-06-22-issue-29-batch-integration-code-review.md`.

## Follow-Up Risks

- Process restart resets all demo state by design. Do not describe the in-memory
  stores as durable.
- Exposing this API outside loopback needs a new authenticated operator
  boundary; changing `HTTP_ADDR` alone is intentionally rejected for non-loopback
  addresses.
- A production scheduler would need durable leases, observability, and retry
  backoff. The workshop scheduler is deliberately a tiny injectable ticker loop.
