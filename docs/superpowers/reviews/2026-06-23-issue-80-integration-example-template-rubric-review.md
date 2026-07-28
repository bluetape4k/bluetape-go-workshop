# Issue #80 리뷰: Integration Example Template and Acceptance Rubric

## 검토 범위

- Issue: #80 `[v0.7.0] Add integration example template and acceptance rubric`
- Artifact:
  `docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md`
- work type: Type E - Planning / Maintenance

## finding

P0=0 P1=0

## coverage 점검

- foundation integration example이 참조되어 있다.
  - #72 order fulfillment workflow integration
  - #75 customer migration batch integration
  - #78 checkout guard integration
- future integration example이 참조되어 있다.
  - #65 SQL order service integration
  - #66 S3-SQS-DynamoDB document workflow integration
  - #67 content moderation workflow integration
  - #68 audited order workflow/outbox integration
  - #69 graph risk intelligence integration
- README template은 English/Korean section guidance를 포함한다.
- framework/runtime guidance는 Gin, `net/http`, chi, no HTTP, Testcontainers, Floci,
  diagram을 다룬다.
- pasteable PR checklist는 `## DoD Status` section으로 끝난다.
- README navigation은 #49와 #79에 맞게 root README edit이 아니라 plan으로 남아 있다.

## Acceptance Criteria 매핑

| #80 acceptance criterion | 상태 | 근거 |
|---|---|---|
| template/rubric artifact가 repo 또는 linked planning docs에 존재한다. | PASS | `docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md` |
| rubric은 existing integration issue #65부터 #69와 new foundation integration issue #72, #75, #78을 참조한다. | PASS | `Canonical Integration Issue Set` table은 #65-#69, #72, #75, #78을 다룬다. |
| README navigation plan은 template이 어디에 속하는지 설명한다. | PASS | `README Navigation Plan` section은 planning link를 runnable example과 분리해 유지한다. |

## 검증 명령

```bash
git diff --check
rg -n "#65|#66|#67|#68|#69|#72|#75|#78|README\\.md|README\\.ko\\.md|Gin|net/http|chi|Testcontainers|Floci|Diagrams|DoD Status" docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md
```

## 잔여 위험

- 이 PR은 planning expectation만 표준화한다. future runnable example에는 여전히 자체 test,
  README update, PR review, CI evidence가 필요하다.
- roadmap이 complete 0.7.0 planning set을 한 번에 link할 수 있도록 final root README
  planning-link section은 #81에 의해 gated 상태로 남아 있다.
