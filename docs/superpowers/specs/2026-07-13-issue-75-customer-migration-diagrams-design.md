# Issue #75 Customer Migration Diagram Design

## Status

Approved in conversation on 2026-07-13; the written spec was reviewed and
approved before diagram implementation.

## Classification

- Work type: Type E maintenance.
- Scope: the bilingual README pair and three paired SVG/PNG assets for
  `examples/customer-migration-batch-integration`.
- Production behavior, Go source, dependencies, module metadata, and workflows
  remain unchanged.
- The PR will reference closed issue #75 and its `0.5.0` milestone without
  reopening or closing the feature issue.

## Goal

Close the first diagram-coverage gap found by the repository-wide audit. The
README should let a reader answer three different questions without inferring
behavior from prose alone:

1. What happens across the injected writer crash and scheduled restart?
2. Which layer owns HTTP, run lifecycle, leader gating, batch execution, and
   in-memory state?
3. In what order do checkpoint rollback, duplicate replay, retry, dead-letter,
   and completion occur?

## Source Model

The diagrams are grounded in these current files:

- `examples/customer-migration-batch-integration/README.md`
- `examples/customer-migration-batch-integration/README.ko.md`
- `examples/customer-migration-batch-integration/main.go`
- `examples/customer-migration-batch-integration/internal/customermigration/service.go`
- `examples/customer-migration-batch-integration/internal/customermigration/engine.go`
- `examples/customer-migration-batch-integration/internal/customermigration/scheduler.go`
- the package tests under the same example directory

Source-backed invariants:

- the manual run uses `ChunkSize: 2` and checkpoint key
  `customer-migration`;
- the first chunk commits `cust-1001` and `cust-1002` at `NextIndex=2`;
- `cust-1003` is written before the injected crash, while the checkpoint is
  rolled back to the previous committed chunk;
- the leader-held scheduled run restores `NextIndex=2`, treats replayed
  `cust-1003` as an idempotent duplicate no-op, dead-letters `cust-1004`, writes
  `cust-1005`, and completes at `NextIndex=5`;
- Gin owns bounded JSON and HTTP mapping, `Service` owns active-run lifecycle
  and snapshots, `LeaderGate` owns scheduled admission, the batch job owns
  reader/processor/writer policies, and the in-memory stores own checkpoint,
  migrated-customer, and dead-letter state;
- the scheduler is a deterministic leader-guarded tick loop, not a durable
  queue.

## Approaches Considered

### A. Three source-specific diagrams

Add a scenario flow, a static architecture view, and a chronological sequence.
Each asset answers one reader question and follows the established workshop
README family. This is the selected approach.

### B. One combined overview

A single image would reduce asset count, but crash/restart chronology and
ownership connectors would compete for the same space. It would either become
too dense or omit the duplicate replay and checkpoint rollback distinction.

### C. Reuse all five focused 0.5.0 example diagrams

Linking the component examples would show their local lessons but would not
explain the integration's own service lifecycle, leader gate, or shared stores.
It would also make readers reconstruct the integrated scenario across several
pages.

## Asset Design

### Scenario

- Files:
  - `docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg`
  - `docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png`
- Reader question: what visible business outcome follows the crash and restart?
- Shape: a left-to-right numbered workflow with a clear boundary between the
  manual run and leader-held scheduled restart.
- Required concepts: first committed chunk, written-but-uncheckpointed
  `cust-1003`, checkpoint rollback, duplicate no-op, transient retry,
  permanent dead letter, final checkpoint and migrated set.
- Visual baseline: best-practices `workflow-image-upload` plus the current
  workshop scenario family.

### Architecture

- Files:
  - `docs/images/readme-diagrams/customer-migration-batch-integration-architecture.svg`
  - `docs/images/readme-diagrams/customer-migration-batch-integration-architecture.png`
- Reader question: who owns each runtime responsibility and state boundary?
- Shape: static ownership layers for operator/Gin, operations service and
  leader gate, batch job policies, and the three in-memory stores.
- Required relationships: manual and scheduled entry points, leader-gated
  scheduling, service-owned run lifecycle, batch-owned step policies, and
  report/status projections from the shared stores.
- Visual baseline: best-practices `leader-ktor-architecture-01` plus the nearest
  workshop architecture family.

### Sequence

- Files:
  - `docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg`
  - `docs/images/readme-diagrams/customer-migration-batch-integration-sequence.png`
- Reader question: what is the exact time order from manual crash to scheduled
  completion?
- Participants: operator, Gin/service boundary, leader gate and batch job,
  writer/sink, and checkpoint/dead-letter state.
- Required chronological frames: manual run, writer-crash branch, leader-held
  restart, and successful completion.
- Required messages include checkpoint load/save/rollback, writer crash, HTTP
  conflict, scheduled tick admission, duplicate no-op, retry/dead-letter, final
  save, and completed report.
- Visual baselines: best-practices `sequence-workflow-sample` and repo-local
  `account-migration-checkpoint-restart-sequence`.

## README Integration

Both locale files embed the same PNGs in the same order:

1. scenario under the existing `Scenario` section;
2. architecture before `Operations Contract`;
3. sequence after the ownership explanation and before runbook guidance.

English prose remains source-facing public guidance. Korean prose is localized
naturally while preserving identifiers, counts, error codes, and behavioral
claims. Diagram labels remain English so the same assets serve both locales.

## Visual Contract

- Use `Architects Daughter` and `Comic Mono` with the current light workshop
  palette.
- Do not use Mermaid, Graphviz, emoji, invented product logos, or legacy
  cylinder art.
- Use text-only cards unless a verified catalog infrastructure icon materially
  improves comprehension.
- Use explicit per-color, fixed-size markers and rounded orthogonal connectors.
- Scenario and architecture primary arrows use the 14x14 role; sequence
  messages use the 16x16 role.
- Work one asset at a time: SVG edit, XML validation, CairoSVG 2x render,
  automated audits, then final full-size PNG inspection.

## Validation

For every asset:

- `xmllint --noout <asset>.svg`
- `cairosvg <asset>.svg -o <asset>.png -s 2`
- connector, geometry, endpoint, and mixed-corner audits with meaningful
  nonzero counts or targeted fallback invariants;
- architecture or sequence-specific audit as applicable;
- full-size PNG inspection after the final coordinate change for labels,
  marker parity, endpoints, crossings, card intrusion, corners, fonts,
  margins, and excess whitespace.

For the README and branch:

- verify English/Korean asset order and link resolution;
- `git diff --check`;
- `go test -count=1 ./examples/customer-migration-batch-integration/...`;
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`;
- `make ci` before PR completion.

The PR body ends with `## DoD Status`. CI, review threads, milestone, assignee,
and live body shape are rechecked before requesting merge approval. No merge or
remote-branch deletion occurs without explicit user approval.

## Non-Goals

- No production scheduler, queue, database, Redis, authentication, or durable
  checkpoint implementation.
- No changes to Go behavior, tests, example commands, HTTP contracts, or error
  codes.
- No redesign of existing 0.5.0 example diagrams.
- No diagrams for later milestones in this PR.
