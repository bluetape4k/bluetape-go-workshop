# Issue #73 Spec Review

## Verdict

- Gate: PASS after iteration 1
- P0: 0
- P1: 0
- Reviewer stance: Step 2-R spec/design review before implementation.

## Reviewed Scope

- `docs/superpowers/research/2026-06-09-issue-73-chunked-csv-checkpoint-research.md`
- `docs/superpowers/specs/2026-06-09-issue-73-chunked-csv-checkpoint-design.md`
- GitHub issues #29, #41, and #73.
- Upstream `github.com/bluetape4k/bluetape-go/batch` v0.5.1 API:
  `Reader`, `Processor`, `Writer`, `CheckpointReader`, `CheckpointStore`,
  `Step`, `Job`, and `Report`.

## Iteration Log

### Iteration 1

| Severity | Finding | Resolution |
|---|---|---|
| P2 | The implementation can confuse `batch.Report.WriteCount` with domain-level newly committed rows during restart because `Step` counts chunk items after a successful writer call, including duplicate-skipped items. | Accepted as an implementation guardrail. Tests must assert both batch report counts and sink-level `new_commits`/`duplicate_skips` separately. No spec rewrite needed because the writer section already requires separate duplicate tracking. |
| P2 | Cancellation wording could be misread as "never save any checkpoint" rather than "do not advance beyond the last successfully committed chunk." | Accepted as an implementation guardrail. Tests must cover cancellation before work and cancellation after at least one successful chunk when practical. No P1 because the spec already says checkpoints are saved only after committed work. |

## Multi-Perspective Review

| Perspective | Result | Notes |
|---|---|---|
| Developer | PASS | The design uses small Go interfaces from `batch`, keeps the example isolated under `examples/chunked-csv-import-checkpoint`, and avoids a generic CSV framework. |
| Security | PASS | The fixture is local, inputs are deterministic, and the design adds no auth boundary, shell execution, template rendering, external network, or secret handling. CSV validation and wrapped row errors are explicit. |
| Ops/SRE | PASS | Restart semantics, failure classification, cancellation, resource close paths, and production hardening caveats are explicit. The durable-store limitation is not overclaimed. |
| User/caller | PASS | The scenario explains chunk size, checkpoint key, failed-chunk replay, idempotent writer behavior, and links to #41 as the base checkpoint/restart track without claiming #41 is implemented. |

## Seven-Tier Checks

| Tier | Result | Evidence |
|---|---|---|
| Security | PASS | Local CSV fixture only; no command/path injection surface beyond a repository-owned fixture path; no external input is accepted by the runnable demo. |
| Ops/SRE reliability | PASS | Spec requires context checks across reader/processor/writer/checkpoint boundaries, close-on-success/failure/cancellation, deterministic failure output, and clear production hardening notes. |
| Structural impact | PASS | New isolated example directory, new diagram script/assets, root README navigation, and workflow docs only; no shared package API change. |
| Go code quality | PASS | Design keeps Go-shaped narrow types, uses upstream `batch` interfaces directly, wraps sentinel errors, and avoids new dependencies. |
| Tests/types | PASS | Tests cover success, mid-run failure, restart, duplicate prevention, malformed CSV, invalid row, cancellation, resource close behavior, and race validation. |
| Performance/stability | PASS | Fixture is bounded, no goroutines or queues are introduced, chunk size is explicit, and replay behavior is deterministic. |
| Documentation/release | PASS | README scenario, Architecture, Sequence Diagram, EN/KO sync, root navigation, decorated diagram evidence, and PR validation commands are required. |

## Critic Integration

No remaining P0/P1 blockers after iteration 1.

Required guardrails during implementation:

- Treat `Checkpoint.NextRow` as the next unread data-row index, not the last
  committed row number.
- Do not advance the checkpoint after a writer returns the simulated crash.
- Keep idempotency in the writer/sink by customer ID and test duplicate skips
  independently from `batch.Report.WriteCount`.
- Make cancellation tests prove resources close and checkpoints advance only up
  to the last successful chunk.
- Keep the example local batch-only; do not introduce Gin or durable stores.
- Generate decorated README PNG assets with SVG siblings and Graphviz route
  evidence, including concrete `margins=L/R/T/B` output.
