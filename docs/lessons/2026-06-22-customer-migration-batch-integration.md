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

## Diagram Review Lessons

- Treat the CairoSVG-rendered PNG as authoritative for arrowhead direction.
  Marker definitions and SVG path order are not sufficient evidence. Inspect
  every left-, right-, up-, and down-facing head in the final PNG, with
  native-pixel crops for dense or bent connector regions.
- Reserve the arrowhead size in the terminal segment before the target. For a
  14 px primary architecture head, move the final bend early enough to leave
  at least 14 px of straight, perpendicular approach. Do not shrink the marker
  to make a cramped route appear valid.
- Keep incoming and outgoing relationships on separate card ports and
  corridors. The first architecture draft overlapped the downward
  `Gin Router -> Service` path with the upward scheduled-admission path. Moving
  the latter to a dedicated Service right-side port made both directions
  unambiguous in the PNG.
- A short segment after a bend can rotate or crowd the rendered head even when
  geometry scripts pass. The `Service -> Job + Step` route needed its terminal
  segment increased from 4 px to 18 px. The `Job + Step -> Checkpoint Store`
  route replaced a 5 px horizontal tail with an 88 px straight vertical entry
  so the head points down into the target edge.
- Do not accept `paths=0`, `connectors=0`, or another weak generic audit count
  as PASS. Add an audit-recognized connector class or a targeted invariant, then
  require meaningful nonzero counts. The repaired architecture records
  `connectors=13`, `paths=2`, and `q_bends=6` with zero intrusion, crossing,
  endpoint, geometry, or mixed-corner failures.
- After the last coordinate or connector-class change, rerender at 2x and
  repeat both the whole-image inspection and focused original-pixel crops.
  Earlier visual approval is stale after any geometry change.

## Follow-Up Risks

- Process restart resets all demo state by design. Do not describe the in-memory
  stores as durable.
- Exposing this API outside loopback needs a new authenticated operator
  boundary; changing `HTTP_ADDR` alone is intentionally rejected for non-loopback
  addresses.
- A production scheduler would need durable leases, observability, and retry
  backoff. The workshop scheduler is deliberately a tiny injectable ticker loop.
