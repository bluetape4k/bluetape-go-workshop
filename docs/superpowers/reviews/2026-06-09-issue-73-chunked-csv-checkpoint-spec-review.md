# Issue #73 명세 리뷰

## 판정

- Gate: iteration 1 뒤 PASS
- P0: 0
- P1: 0
- reviewer stance: implementation 전 Step 2-R spec/design review.

## 검토 범위

- `docs/superpowers/research/2026-06-09-issue-73-chunked-csv-checkpoint-research.md`
- `docs/superpowers/specs/2026-06-09-issue-73-chunked-csv-checkpoint-design.md`
- GitHub issues #29, #41, and #73.
- Upstream `github.com/bluetape4k/bluetape-go/batch` v0.5.1 API:
  `Reader`, `Processor`, `Writer`, `CheckpointReader`, `CheckpointStore`,
  `Step`, `Job`, and `Report`.

## 반복 기록

### 반복 1

| Severity | finding | 해결 |
|---|---|---|
| P2 | `Step`은 successful writer call 뒤 duplicate-skipped item을 포함해 chunk item을 count하므로, implementation이 restart 중 `batch.Report.WriteCount`와 domain-level newly committed row를 혼동할 수 있다. | implementation guardrail로 수용했다. test는 batch report count와 sink-level `new_commits`/`duplicate_skips`를 별도로 검증해야 한다. writer section이 이미 separate duplicate tracking을 요구하므로 spec rewrite는 필요 없다. |
| P2 | cancellation wording은 "checkpoint를 전혀 저장하지 않는다"가 아니라 "마지막으로 성공적으로 committed된 chunk 이후로 advance하지 않는다"는 뜻인데, 오해될 수 있었다. | implementation guardrail로 수용했다. test는 가능하면 work 전 cancellation과 successful chunk 하나 이상 뒤 cancellation을 모두 다뤄야 한다. spec이 이미 committed work 뒤에만 checkpoint를 저장한다고 말하므로 P1은 아니다. |

## 다중 관점 리뷰

| 관점 | 결과 | 메모 |
|---|---|---|
| Developer | PASS | design은 `batch`의 작은 Go interface를 사용하고 example을 `examples/chunked-csv-import-checkpoint` 아래에 격리하며 generic CSV framework를 피한다. |
| Security | PASS | fixture는 local이고 input은 deterministic이다. design은 auth boundary, shell execution, template rendering, external network, secret handling을 추가하지 않는다. CSV validation과 wrapped row error가 명시적이다. |
| Ops/SRE | PASS | restart semantic, failure classification, cancellation, resource close path, production hardening caveat이 명시적이다. durable-store limitation을 과장하지 않는다. |
| User/caller | PASS | scenario는 chunk size, checkpoint key, failed-chunk replay, idempotent writer behavior를 설명하고, #41이 구현됐다고 주장하지 않으면서 base checkpoint/restart track으로 #41을 연결한다. |

## Seven-Tier 점검

| Tier | 결과 | 근거 |
|---|---|---|
| Security | PASS | local CSV fixture만 사용한다. repository-owned fixture path 밖의 command/path injection surface가 없고 runnable demo는 external input을 받지 않는다. |
| Ops/SRE reliability | PASS | spec은 reader/processor/writer/checkpoint boundary 전반의 context check, success/failure/cancellation 시 close, deterministic failure output, 명확한 production hardening note를 요구한다. |
| Structural impact | PASS | 새 isolated example directory, 새 diagram script/assets, root README navigation, workflow docs만 포함한다. shared package API change는 없다. |
| Go code quality | PASS | design은 Go-shaped narrow type을 유지하고 upstream `batch` interface를 직접 사용하며 sentinel error를 wrap하고 새 dependency를 피한다. |
| Tests/types | PASS | test는 success, mid-run failure, restart, duplicate prevention, malformed CSV, invalid row, cancellation, resource close behavior, race validation을 다룬다. |
| Performance/stability | PASS | fixture는 bounded하고 goroutine이나 queue는 도입되지 않는다. chunk size는 명시적이고 replay behavior는 deterministic하다. |
| Documentation/release | PASS | README scenario, Architecture, Sequence Diagram, EN/KO sync, root navigation, decorated diagram evidence, PR validation command가 요구된다. |

## Critic 통합

iteration 1 뒤 남은 P0/P1 blocker는 없다.

구현 중 필수 guardrail:

- `Checkpoint.NextRow`를 last committed row number가 아니라 next unread data-row index로
  취급한다.
- writer가 simulated crash를 반환한 뒤 checkpoint를 advance하지 않는다.
- idempotency를 customer ID 기준 writer/sink에 유지하고, duplicate skip을
  `batch.Report.WriteCount`와 독립적으로 test한다.
- cancellation test는 resource close와 checkpoint가 last successful chunk까지만 advance됨을
  증명해야 한다.
- example은 local batch-only로 유지하고 Gin이나 durable store를 도입하지 않는다.
- concrete `margins=L/R/T/B` output을 포함해 SVG sibling과 Graphviz route evidence가 있는
  decorated README PNG asset을 생성한다.
