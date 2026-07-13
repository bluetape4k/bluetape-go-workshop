# Issue #75 Customer Migration Diagrams Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three source-backed, bilingual README diagrams that explain the customer migration crash/restart scenario, runtime ownership, and chronological recovery sequence.

**Architecture:** The implementation changes documentation only. Each diagram is a hand-authored SVG in the canonical README asset directory, rendered to a paired 2x PNG with CairoSVG, audited independently, and embedded in the English and Korean README pair in identical order.

**Tech Stack:** SVG, CairoSVG, XMLLint, bluetape-diagram audit scripts, Markdown, Go repository verification.

---

## File Map

- Create: `docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg`
- Create: `docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png`
- Create: `docs/images/readme-diagrams/customer-migration-batch-integration-architecture.svg`
- Create: `docs/images/readme-diagrams/customer-migration-batch-integration-architecture.png`
- Create: `docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg`
- Create: `docs/images/readme-diagrams/customer-migration-batch-integration-sequence.png`
- Modify: `examples/customer-migration-batch-integration/README.md`
- Modify: `examples/customer-migration-batch-integration/README.ko.md`
- Modify: `docs/superpowers/specs/2026-07-13-issue-75-customer-migration-diagrams-design.md`

## Task 1: Lock Source and Reference Evidence

- [ ] **Step 1: Confirm the approved source invariants**

Read these files before drawing:

```bash
sed -n '1,220p' examples/customer-migration-batch-integration/README.md
sed -n '1,430p' examples/customer-migration-batch-integration/internal/customermigration/service.go
sed -n '1,620p' examples/customer-migration-batch-integration/internal/customermigration/engine.go
sed -n '1,180p' examples/customer-migration-batch-integration/internal/customermigration/scheduler.go
```

Expected evidence: manual run, `ChunkSize: 2`, checkpoint rollback to `NextIndex=2`, leader-held restart, duplicate no-op for `cust-1003`, retry/dead-letter behavior, and final `NextIndex=5` are present.

- [ ] **Step 2: Open the authoritative visual references at full size**

Open:

```text
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/workflow-image-upload.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-ktor-architecture-01.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png
docs/images/readme-diagrams/account-migration-checkpoint-restart-sequence.png
```

Expected evidence: light canvas, handwritten title, readable cards, muted semantic palette, explicit same-color arrowheads, dashed lifelines, activation bars, visible numbered message pills, and transparent chronological frames.

## Task 2: Create and Prove the Scenario Diagram

- [ ] **Step 1: Create the scenario SVG**

Create `customer-migration-batch-integration-scenario.svg` as a 1200x600 light-theme workflow with these numbered stages:

1. `Manual Start` — `crash_after_new_writes=3`
2. `Commit Chunk 1` — `cust-1001 + cust-1002`, `checkpoint = 2`
3. `Writer Crash` — `cust-1003 written`, `checkpoint rolls back to 2`
4. `Leader-held Restart` — `restore index 2`, `cust-1003 duplicate no-op`
5. `Finish Migration` — `cust-1004 dead letter`, `cust-1005 written`, `checkpoint = 5`

Add a bottom outcome strip with `migrated: 1001, 1002, 1003, 1005`, `dead letter: 1004`, and `status: completed`. Use 14x14 primary progression heads, muted red for the crash branch, muted violet for restart, and muted olive for completion.

- [ ] **Step 2: Validate and render the scenario asset**

Run:

```bash
xmllint --noout docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
cairosvg docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg -o docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
```

Expected: XML/render success, five numbered cards, four primary progression connectors, zero diagonal failures, zero endpoint failures, and zero mixed-corner failures.

- [ ] **Step 3: Inspect the full-size scenario PNG**

Open the 2400x1200 rendered PNG at original detail. Verify all labels fit, the crash and restart distinction is immediate, arrows attach perpendicularly with matching heads, no line crosses a card, and bottom whitespace is balanced.

- [ ] **Step 4: Commit the scenario asset**

```bash
git add docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png
git commit -m "docs: add customer migration scenario diagram"
```

## Task 3: Create and Prove the Architecture Diagram

- [ ] **Step 1: Create the architecture SVG**

Create `customer-migration-batch-integration-architecture.svg` as a 1400x900 static ownership view with four horizontal layers:

- `HTTP boundary`: Operator, Gin Router, bounded JSON/status mapping.
- `Operations lifecycle`: Service, active-run guard/cancel/snapshots, Leader Gate.
- `Batch execution`: Job + Step, Reader, Processor with retry/skip, Writer with idempotent sink behavior.
- `Process-local state`: Checkpoint Store, Customer Sink, Dead-Letter Store, latest report/status projection.

Use solid blue ownership arrows and dashed violet scheduled-admission dependencies with an in-image legend. Keep all connectors orthogonal and connect concrete cards rather than lane borders.

- [ ] **Step 2: Validate and render the architecture asset**

Run the XML, CairoSVG, connector, `--fail-diagonal` geometry, endpoint, and mixed-corner commands from Task 2 against the architecture SVG.

Expected: XML/render success, at least ten cards, meaningful nonzero connector/card/path counts, zero diagonal failures, zero endpoint failures, and zero mixed-corner failures.

- [ ] **Step 3: Inspect the full-size architecture PNG**

Open the 2800x1800 PNG at original detail. Verify ownership and dependency legend parity, peer-card alignment, no connector runs along a layer title or card border, no card intrusion, and even outer margins.

- [ ] **Step 4: Commit the architecture asset**

```bash
git add docs/images/readme-diagrams/customer-migration-batch-integration-architecture.svg docs/images/readme-diagrams/customer-migration-batch-integration-architecture.png
git commit -m "docs: add customer migration architecture diagram"
```

## Task 4: Create and Prove the Sequence Diagram

- [ ] **Step 1: Create the sequence SVG**

Create `customer-migration-batch-integration-sequence.svg` as a 1600x1200 chronological diagram with participants `Operator`, `Gin + Service`, `Leader Gate + Batch`, `Writer + Sink`, and `Checkpoint + Dead Letters`.

Show visible numbered messages in this order:

1. `POST /batch/start`
2. `load checkpoint: none`
3. `write cust-1001..1002`
4. `save next_index=2`
5. `write cust-1003`
6. `writer crash`
7. `rollback next_index=2`
8. `409 writer_crash`
9. `POST /batch/schedule/tick`
10. `leadership held`
11. `restore next_index=2`
12. `cust-1003 duplicate no-op`
13. `retry 1003 / dead-letter 1004`
14. `write cust-1005`
15. `save next_index=5`
16. `200 completed report`

Use transparent `alt writer crash` and `restart while leader` frames, five dashed lifelines, activation bars, the established 13x13 user-space per-color message markers with 10x10 filled triangles, and labels above continuous message lines.

- [ ] **Step 2: Validate and render the sequence asset**

Run the common XML/render/audit commands and:

```bash
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg
```

Expected: five participants, five lifelines, visible activation bars, sixteen numbered labels, two transparent chronological frames, explicit marker color parity, zero common audit failures, and a passing sequence-style audit.

- [ ] **Step 3: Inspect the full-size sequence PNG**

Open the 3200x2400 PNG at original detail. Verify every number and label is readable above its own line, lines remain continuous, frames are transparent and chronological, activation bars do not cover arrowheads, returns point left, and no footer overlaps a frame.

- [ ] **Step 4: Commit the sequence asset**

```bash
git add docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg docs/images/readme-diagrams/customer-migration-batch-integration-sequence.png
git commit -m "docs: add customer migration sequence diagram"
```

## Task 5: Embed the Assets in Both README Locales

- [ ] **Step 1: Update the English README**

Embed the scenario after the existing crash/restart walkthrough, add an `Architecture` section before `Operations Contract`, and add a `Crash and Restart Sequence` section before `Runbook Notes`. Explain that the architecture is an ownership view and the sequence shows the manual failure followed by a leader-held restart.

- [ ] **Step 2: Update the Korean README**

Add the same three PNG paths in the same order under natural Korean headings and source-equivalent explanations. Preserve `cust-*`, `NextIndex`, endpoint paths, error codes, and checkpoint values exactly.

- [ ] **Step 3: Verify locale and link parity**

Run:

```bash
diff <(rg -o 'readme-diagrams/[^) ]+\.png' examples/customer-migration-batch-integration/README.md) <(rg -o 'readme-diagrams/[^) ]+\.png' examples/customer-migration-batch-integration/README.ko.md)
for asset in $(rg -o 'readme-diagrams/[^) ]+\.png' examples/customer-migration-batch-integration/README.md | sed 's#readme-diagrams/##'); do test -f "docs/images/readme-diagrams/$asset"; test -f "docs/images/readme-diagrams/${asset%.png}.svg"; done
git diff --check
```

Expected: no diff output, all six assets resolve, and diff check exits zero.

- [ ] **Step 4: Commit README integration and approved artifacts**

```bash
git add examples/customer-migration-batch-integration/README.md examples/customer-migration-batch-integration/README.ko.md docs/superpowers/specs/2026-07-13-issue-75-customer-migration-diagrams-design.md docs/superpowers/plans/2026-07-13-issue-75-customer-migration-diagrams-plan.md
git commit -m "docs: explain customer migration diagrams"
```

## Task 6: Final Verification and Delivery

- [ ] **Step 1: Run focused normal and race tests**

```bash
go test -count=1 ./examples/customer-migration-batch-integration/...
go test -race -count=1 ./examples/customer-migration-batch-integration/...
```

Expected: every package passes with zero failures and the race detector reports no race.

- [ ] **Step 2: Run the authoritative repository gate**

```bash
make ci
```

Expected: formatting, tidy, vet, lint, test, and race gates pass.

- [ ] **Step 3: Review the final diff and checklist ledger**

```bash
git status --short
git diff --check develop...HEAD
git diff --stat develop...HEAD
git log --oneline develop..HEAD
```

Expected: only the approved spec, plan, README pair, and six diagram assets differ; all DIA, Type E, Go public-evidence, and workflow rows have concrete evidence with no blocker.

- [ ] **Step 4: Push and create the PR**

Push `docs/issue-75-customer-migration-diagrams`, create an English PR referencing #75 and milestone `0.5.0`, assign the repository owner, and make `## DoD Status` the final level-two heading.

- [ ] **Step 5: Wait for CI and stop before merge**

Verify live PR body, labels, milestone, assignee, checks, review threads, and head SHA. Report the PR URL and local worktree path. Do not merge or delete the remote branch until the user explicitly approves the merge.
