# Issue #75 Customer Migration Diagrams Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** customer migration crash/restart scenario, runtime ownership, chronological recovery sequence를 설명하는 source-backed bilingual README diagram 세 개를 추가한다.

**Architecture:** 구현은 documentation-only 변경이다. 각 diagram은 canonical README asset directory에 hand-authored SVG로 작성하고, CairoSVG로 paired 2x PNG를 렌더링하며, 독립 audit를 거친 뒤 English/Korean README pair에 같은 순서로 embed한다.

**Tech Stack:** SVG, CairoSVG, XMLLint, bluetape-diagram audit script, Markdown, Go repository verification을 사용한다.

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

## Task 1: Source and Reference Evidence 고정

- [ ] **Step 1: 승인된 source invariant 확인**

그리기 전에 다음 file을 읽는다.

```bash
sed -n '1,220p' examples/customer-migration-batch-integration/README.md
sed -n '1,430p' examples/customer-migration-batch-integration/internal/customermigration/service.go
sed -n '1,620p' examples/customer-migration-batch-integration/internal/customermigration/engine.go
sed -n '1,180p' examples/customer-migration-batch-integration/internal/customermigration/scheduler.go
```

기대 evidence: manual run, `ChunkSize: 2`, checkpoint rollback to `NextIndex=2`, leader-held restart, duplicate no-op for `cust-1003`, retry/dead-letter behavior, final `NextIndex=5`가 존재한다.

- [ ] **Step 2: authoritative visual reference를 full size로 열기**

연다.

```text
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/workflow-image-upload.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-ktor-architecture-01.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png
docs/images/readme-diagrams/account-migration-checkpoint-restart-sequence.png
```

기대 evidence: light canvas와 handwritten title을 사용한다. card는 읽기 쉬워야 하고, muted semantic palette, same-color arrowhead, dashed lifeline, activation bar, numbered message pill, transparent chronological frame이 보여야 한다.

## Task 2: Scenario Diagram 생성 및 증명

- [ ] **Step 1: scenario SVG 생성**

`customer-migration-batch-integration-scenario.svg`를 1200x600 light-theme workflow로 생성하고 다음 numbered stage를 포함한다.

1. `Manual Start` — `crash_after_new_writes=3`
2. `Commit Chunk 1` — `cust-1001 + cust-1002`, `checkpoint = 2`
3. `Writer Crash` — `cust-1003 written`, `checkpoint rolls back to 2`
4. `Leader-held Restart` — `restore index 2`, `cust-1003 duplicate no-op`
5. `Finish Migration` — `cust-1004 dead letter`, `cust-1005 written`, `checkpoint = 5`

bottom outcome strip에는 `migrated: 1001, 1002, 1003, 1005`, `dead letter: 1004`, `status: completed`를 추가한다. primary progression head는 14x14를 사용한다. crash branch에는 muted red, restart에는 muted violet, completion에는 muted olive를 사용한다.

- [ ] **Step 2: scenario asset 검증 및 렌더링**

실행한다.

```bash
xmllint --noout docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
cairosvg docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg -o docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg
```

기대값: XML/render success, numbered card 다섯 개, primary progression connector 네 개, diagonal failure 0개, endpoint failure 0개, mixed-corner failure 0개.

- [ ] **Step 3: full-size scenario PNG 검사**

2400x1200 rendered PNG를 original detail로 연다. 모든 label이 맞는지, crash와 restart 구분이 즉시 보이는지, arrow가 matching head로 perpendicular하게 붙는지, line이 card를 가로지르지 않는지, bottom whitespace가 균형적인지 확인한다.

- [ ] **Step 4: scenario asset commit**

```bash
git add docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png
git commit -m "docs: add customer migration scenario diagram"
```

## Task 3: Architecture Diagram 생성 및 증명

- [ ] **Step 1: architecture SVG 생성**

`customer-migration-batch-integration-architecture.svg`를 1400x900 static ownership view로 생성하고 네 horizontal layer를 둔다.

- `HTTP boundary`: Operator, Gin Router, bounded JSON/status mapping.
- `Operations lifecycle`: Service, active-run guard/cancel/snapshots, Leader Gate.
- `Batch execution`: Job + Step, Reader, retry/skip을 수행하는 Processor, idempotent sink behavior를 가진 Writer.
- `Process-local state`: Checkpoint Store, Customer Sink, Dead-Letter Store, latest report/status projection.

solid blue ownership arrow와 dashed violet scheduled-admission dependency를 사용하고 in-image legend를 넣는다. 모든 connector는 orthogonal이어야 하며 lane border가 아니라 concrete card에 연결한다.

- [ ] **Step 2: architecture asset 검증 및 렌더링**

Task 2의 XML, CairoSVG, connector, `--fail-diagonal` geometry, endpoint, mixed-corner command를 architecture SVG에 대해 실행한다.

기대값: XML/render success, card 최소 10개, 의미 있는 nonzero connector/card/path count, diagonal failure 0개, endpoint failure 0개, mixed-corner failure 0개.

- [ ] **Step 3: full-size architecture PNG 검사**

2800x1800 PNG를 original detail로 연다. ownership/dependency legend parity, peer-card alignment, layer title이나 card border를 타고 흐르는 connector 없음, card intrusion 없음, 균일한 outer margin을 확인한다.

- [ ] **Step 4: architecture asset commit**

```bash
git add docs/images/readme-diagrams/customer-migration-batch-integration-architecture.svg docs/images/readme-diagrams/customer-migration-batch-integration-architecture.png
git commit -m "docs: add customer migration architecture diagram"
```

## Task 4: Sequence Diagram 생성 및 증명

- [ ] **Step 1: sequence SVG 생성**

`customer-migration-batch-integration-sequence.svg`를 1600x1200 chronological diagram으로 생성하고 participant `Operator`, `Gin + Service`, `Leader Gate + Batch`, `Writer + Sink`, `Checkpoint + Dead Letters`를 둔다.

visible numbered message는 다음 순서를 따른다.

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

transparent `alt writer crash`와 `restart while leader` frame, dashed lifeline 다섯 개, activation bar, established 13x13 user-space per-color message marker와 10x10 filled triangle, continuous message line 위 label을 사용한다.

- [ ] **Step 2: sequence asset 검증 및 렌더링**

common XML/render/audit command와 다음을 실행한다.

```bash
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg
```

기대값: participant 다섯 개, lifeline 다섯 개, visible activation bar, numbered label 16개, transparent chronological frame 두 개, explicit marker color parity, common audit failure 0개, sequence-style audit PASS.

- [ ] **Step 3: full-size sequence PNG 검사**

3200x2400 PNG를 original detail로 연다. 모든 number와 label이 자기 line 위에서 읽히는지, line이 continuous인지, frame이 transparent이며 chronological인지, activation bar가 arrowhead를 가리지 않는지, return이 왼쪽을 향하는지, footer가 frame과 overlap하지 않는지 확인한다.

- [ ] **Step 4: sequence asset commit**

```bash
git add docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg docs/images/readme-diagrams/customer-migration-batch-integration-sequence.png
git commit -m "docs: add customer migration sequence diagram"
```

## Task 5: 두 README Locale에 Asset Embed

- [ ] **Step 1: English README 업데이트**

existing crash/restart walkthrough 뒤에 scenario를 embed한다. `Operations Contract` 앞에 `Architecture` section을 추가하고, `Runbook Notes` 앞에 `Crash and Restart Sequence` section을 추가한다. architecture는 ownership view이며 sequence는 manual failure 뒤 leader-held restart를 보여준다고 설명한다.

- [ ] **Step 2: Korean README 업데이트**

같은 PNG path 세 개를 같은 순서로 자연스러운 한국어 heading과 source-equivalent explanation 아래에 추가한다. `cust-*`, `NextIndex`, endpoint path, error code, checkpoint value는 정확히 보존한다.

- [ ] **Step 3: locale 및 link parity 검증**

실행한다.

```bash
diff <(rg -o 'readme-diagrams/[^) ]+\.png' examples/customer-migration-batch-integration/README.md) <(rg -o 'readme-diagrams/[^) ]+\.png' examples/customer-migration-batch-integration/README.ko.md)
for asset in $(rg -o 'readme-diagrams/[^) ]+\.png' examples/customer-migration-batch-integration/README.md | sed 's#readme-diagrams/##'); do test -f "docs/images/readme-diagrams/$asset"; test -f "docs/images/readme-diagrams/${asset%.png}.svg"; done
git diff --check
```

기대값: diff output 없음, asset 여섯 개 모두 resolve, diff check exit zero.

- [ ] **Step 4: README integration 및 approved artifact commit**

```bash
git add examples/customer-migration-batch-integration/README.md examples/customer-migration-batch-integration/README.ko.md docs/superpowers/specs/2026-07-13-issue-75-customer-migration-diagrams-design.md docs/superpowers/plans/2026-07-13-issue-75-customer-migration-diagrams-plan.md
git commit -m "docs: explain customer migration diagrams"
```

## Task 6: Final Verification and Delivery

- [ ] **Step 1: focused normal 및 race test 실행**

```bash
go test -count=1 ./examples/customer-migration-batch-integration/...
go test -race -count=1 ./examples/customer-migration-batch-integration/...
```

기대값: 모든 package가 zero failure로 pass하고 race detector가 race를 보고하지 않는다.

- [ ] **Step 2: authoritative repository gate 실행**

```bash
make ci
```

기대값: formatting, tidy, vet, lint, test, race gate가 pass한다.

- [ ] **Step 3: final diff 및 checklist ledger review**

```bash
git status --short
git diff --check develop...HEAD
git diff --stat develop...HEAD
git log --oneline develop..HEAD
```

기대값: approved spec, plan, README pair, diagram asset 여섯 개만 differ한다. 모든 DIA, Type E, Go public-evidence, workflow row가 구체적 evidence를 가지며 blocker가 없다.

- [ ] **Step 4: push 및 PR 생성**

`docs/issue-75-customer-migration-diagrams`를 push하고 #75와 milestone `0.5.0`을 참조하는 English PR을 생성한다. repository owner를 assign하고 `## DoD Status`를 final level-two heading으로 만든다.

- [ ] **Step 5: CI 대기 및 merge 전 중단**

live PR body, label, milestone, assignee, check, review thread, head SHA를 확인한다. PR URL과 local worktree path를 보고한다. 사용자가 명시적으로 merge를 승인하기 전까지 merge하거나 remote branch를 삭제하지 않는다.
