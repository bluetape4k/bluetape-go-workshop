# Issue #80 Review: Integration Example Template and Acceptance Rubric

## Scope Reviewed

- Issue: #80 `[v0.7.0] Add integration example template and acceptance rubric`
- Artifact:
  `docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md`
- Work type: Type E - Planning / Maintenance

## Findings

P0=0 P1=0

## Coverage Checks

- Foundation integration examples are referenced:
  - #72 order fulfillment workflow integration
  - #75 customer migration batch integration
  - #78 checkout guard integration
- Future integration examples are referenced:
  - #65 SQL order service integration
  - #66 S3-SQS-DynamoDB document workflow integration
  - #67 content moderation workflow integration
  - #68 audited order workflow/outbox integration
  - #69 graph risk intelligence integration
- README templates include English and Korean section guidance.
- Framework/runtime guidance covers Gin, `net/http`, chi, no HTTP,
  Testcontainers, Floci, and diagrams.
- The pasteable PR checklist ends with a `## DoD Status` section.
- README navigation remains a plan, not a root README edit, matching #49 and
  #79.

## Acceptance Criteria Mapping

| #80 acceptance criterion | Status | Evidence |
|---|---|---|
| Template/rubric artifact exists in the repo or linked planning docs. | PASS | `docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md` |
| Rubric references existing integration issues #65 through #69 and new foundation integration issues #72, #75, and #78. | PASS | `Canonical Integration Issue Set` table covers #65-#69, #72, #75, and #78. |
| README navigation plan explains where the template belongs. | PASS | `README Navigation Plan` section keeps planning links separate from runnable examples. |

## Verification Commands

```bash
git diff --check
rg -n "#65|#66|#67|#68|#69|#72|#75|#78|README\\.md|README\\.ko\\.md|Gin|net/http|chi|Testcontainers|Floci|Diagrams|DoD Status" docs/superpowers/plans/2026-06-23-issue-80-integration-example-template-rubric.md
```

## Residual Risk

- This PR standardizes planning expectations only. Future runnable examples
  still need their own tests, README updates, PR review, and CI evidence.
- The final root README planning-link section remains gated by #81 so the
  roadmap can link the complete 0.7.0 planning set in one pass.
