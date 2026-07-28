# Issue #75 Customer Migration Diagram 설계

## Status

2026-07-13 대화에서 승인되었다. 작성된 spec은 diagram implementation 전에 review되고
승인되었다.

## Classification

- Work type: Type E maintenance.
- Scope: `examples/customer-migration-batch-integration`용 bilingual README pair와 세 쌍의
  SVG/PNG asset.
- production behavior, Go source, dependency, module metadata, workflow는 변경하지 않는다.
- PR은 닫힌 issue #75와 milestone `0.5.0`을 참조하되 feature issue를 다시 열거나 닫지 않는다.

## Goal

repository-wide audit에서 발견된 첫 diagram-coverage gap을 닫는다. README는 reader가 prose만으로
behavior를 추론하지 않고 다음 세 질문에 답할 수 있게 해야 한다.

1. 주입된 writer crash와 scheduled restart 사이에서 무엇이 일어나는가?
2. HTTP, run lifecycle, leader gating, batch execution, in-memory state는 각각 어느 layer가
   소유하는가?
3. checkpoint rollback, duplicate replay, retry, dead-letter, completion은 어떤 순서로 발생하는가?

## Source Model

diagram은 다음 현재 file에 근거한다.

- `examples/customer-migration-batch-integration/README.md`
- `examples/customer-migration-batch-integration/README.ko.md`
- `examples/customer-migration-batch-integration/main.go`
- `examples/customer-migration-batch-integration/internal/customermigration/service.go`
- `examples/customer-migration-batch-integration/internal/customermigration/engine.go`
- `examples/customer-migration-batch-integration/internal/customermigration/scheduler.go`
- 같은 example directory 아래의 package test

source-backed invariant는 다음과 같다.

- manual run은 `ChunkSize: 2`와 checkpoint key `customer-migration`을 사용한다.
- 첫 chunk는 `cust-1001`과 `cust-1002`를 commit하고 `NextIndex=2`가 된다.
- `cust-1003`은 injected crash 전에 write되지만 checkpoint는 이전 committed chunk로 rollback된다.
- leader-held scheduled run은 `NextIndex=2`를 restore하고 replay된 `cust-1003`을 idempotent
  duplicate no-op으로 처리하며, `cust-1004`를 dead-letter로 보내고, `cust-1005`를 write한 뒤
  `NextIndex=5`에서 complete된다.
- Gin은 bounded JSON과 HTTP mapping을 소유한다. `Service`는 active-run lifecycle과 snapshot을
  소유한다. `LeaderGate`는 scheduled admission을 소유한다. batch job은 reader/processor/writer
  policy를 소유한다. in-memory store는 checkpoint, migrated-customer, dead-letter state를 소유한다.
- scheduler는 deterministic leader-guarded tick loop이지 durable queue가 아니다.

## Approaches Considered

### A. Three source-specific diagrams

scenario flow, static architecture view, chronological sequence를 추가한다. 각 asset은 reader 질문 하나에
답하고 기존 workshop README family를 따른다. 이것이 선택된 approach다.

### B. One combined overview

단일 image는 asset 수를 줄일 수 있지만 crash/restart chronology와 ownership connector가 같은 공간을
두고 경쟁하게 된다. 결과물은 너무 빽빽해지거나 duplicate replay와 checkpoint rollback의 차이를 빠뜨릴
것이다.

### C. Reuse all five focused 0.5.0 example diagrams

component example을 link하면 각 local lesson은 보여줄 수 있지만 integration 자체의 service lifecycle,
leader gate, shared store를 설명하지 못한다. 또한 reader가 여러 page에서 integrated scenario를 다시
조립해야 한다.

## Asset Design

### Scenario

- Files:
  - `docs/images/readme-diagrams/customer-migration-batch-integration-scenario.svg`
  - `docs/images/readme-diagrams/customer-migration-batch-integration-scenario.png`
- Reader question: crash와 restart 이후 어떤 visible business outcome이 나타나는가?
- Shape: manual run과 leader-held scheduled restart 사이 boundary가 명확한 left-to-right numbered
  workflow.
- Required concepts: first committed chunk, written-but-uncheckpointed `cust-1003`, checkpoint
  rollback, duplicate no-op, transient retry, permanent dead letter, final checkpoint와 migrated set.
- Visual baseline: best-practices `workflow-image-upload`와 현재 workshop scenario family.

### Architecture

- Files:
  - `docs/images/readme-diagrams/customer-migration-batch-integration-architecture.svg`
  - `docs/images/readme-diagrams/customer-migration-batch-integration-architecture.png`
- Reader question: 각 runtime responsibility와 state boundary는 누가 소유하는가?
- Shape: operator/Gin, operations service와 leader gate, batch job policy, 세 in-memory store를
  위한 static ownership layer.
- Required relationships: manual 및 scheduled entry point, leader-gated scheduling,
  service-owned run lifecycle, batch-owned step policy, shared store에서 나온 report/status projection.
- Visual baseline: best-practices `leader-ktor-architecture-01`와 가장 가까운 workshop architecture
  family.

### Sequence

- Files:
  - `docs/images/readme-diagrams/customer-migration-batch-integration-sequence.svg`
  - `docs/images/readme-diagrams/customer-migration-batch-integration-sequence.png`
- Reader question: manual crash에서 scheduled completion까지 정확한 시간 순서는 무엇인가?
- Participants: operator, Gin/service boundary, leader gate와 batch job, writer/sink,
  checkpoint/dead-letter state.
- Required chronological frames: manual run, writer-crash branch, leader-held restart,
  successful completion.
- Required messages include checkpoint load/save/rollback, writer crash, HTTP conflict, scheduled
  tick admission, duplicate no-op, retry/dead-letter, final save, completed report.
- Visual baselines: best-practices `sequence-workflow-sample`과 repo-local
  `account-migration-checkpoint-restart-sequence`.

## README Integration

두 locale file은 같은 PNG를 같은 순서로 embed한다.

1. 기존 `Scenario` section 아래의 scenario
2. `Operations Contract` 앞의 architecture
3. ownership explanation 뒤와 runbook guidance 앞의 sequence

English prose는 source-facing public guidance로 유지한다. Korean prose는 identifier, count, error
code, behavioral claim을 보존하면서 자연스럽게 localize한다. 두 locale이 같은 asset을 쓰도록 diagram
label은 English로 유지한다.

## Visual Contract

- 현재 light workshop palette와 함께 `Architects Daughter` 및 `Comic Mono`를 사용한다.
- Mermaid, Graphviz, emoji, invented product logo, legacy cylinder art는 사용하지 않는다.
- 검증된 catalog infrastructure icon이 comprehension을 실질적으로 높일 때가 아니면 text-only card를 사용한다.
- 명시적 per-color fixed-size marker와 rounded orthogonal connector를 사용한다.
- scenario 및 architecture primary arrow는 14x14 role을 사용한다. sequence message는 10x10 filled
  triangle과 명시적 per-color definition을 가진 기존 13x13 user-space marker를 사용한다.
- 한 번에 하나의 asset만 작업한다. SVG edit, XML validation, CairoSVG 2x render, automated audit,
  final full-size PNG inspection 순서로 진행한다.

## Validation

각 asset에 대해 다음을 수행한다.

- `xmllint --noout <asset>.svg`
- `cairosvg <asset>.svg -o <asset>.png -s 2`
- 의미 있는 nonzero count 또는 targeted fallback invariant를 가진 connector, geometry, endpoint,
  mixed-corner audit
- 해당되는 경우 architecture 또는 sequence-specific audit
- 최종 coordinate 변경 이후 label, marker parity, endpoint, crossing, card intrusion, corner, font,
  margin, excess whitespace에 대한 full-size PNG inspection

README와 branch에 대해서는 다음을 수행한다.

- English/Korean asset order와 link resolution 검증
- `git diff --check`
- `go test -count=1 ./examples/customer-migration-batch-integration/...`
- `go test -race -count=1 ./examples/customer-migration-batch-integration/...`
- PR completion 전에 `make ci`

PR body는 `## DoD Status`로 끝난다. merge approval을 요청하기 전에 CI, review thread, milestone,
assignee, live body shape를 다시 확인한다. 명시적 user approval 없이 merge 또는 remote-branch deletion을
수행하지 않는다.

## Non-Goals

- production scheduler, queue, database, Redis, authentication, durable checkpoint implementation은
  만들지 않는다.
- Go behavior, test, example command, HTTP contract, error code는 변경하지 않는다.
- 기존 0.5.0 example diagram을 redesign하지 않는다.
- 이 PR에서 이후 milestone용 diagram을 만들지 않는다.
