# Issue #79 리뷰: Example Selection Scorecard

## 검토 범위

- Issue: #79 `[v0.7.0] Add bluetape4k-workshop example selection scorecard`
- Artifact:
  `docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md`
- work type: Type E - Planning / Maintenance

## finding

P0=0 P1=0

## coverage 점검

- scorecard는 다섯 parent umbrella를 모두 다룬다.
  - #32 SQL
  - #33 AWS/Floci
  - #34 text
  - #35 audit/outbox
  - #36 graph
- scorecard는 #47과 #48 research window의 모든 candidate issue를 포함한다.
  - Graph: #50, #51, #52, #69
  - Text: #53, #54, #55, #67
  - Audit/outbox: #56, #57, #58, #68
  - AWS/Floci: #59, #60, #61, #66
  - SQL: #62, #63, #64, #65
- accepted, bounded, deferred, rejected pattern은 reason과 함께 문서화되어 있다.
- #49가 #79-#81 완료 전까지 navigation link를 defer하므로 이 PR에서는 README navigation을
  변경하지 않는다.

## Acceptance Criteria 매핑

| #79 acceptance criterion | 상태 | 근거 |
|---|---|---|
| scorecard artifact가 repo 또는 linked planning docs에 존재한다. | PASS | `docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md` |
| 각 candidate에는 clear accept/defer/reject decision이 있다. | PASS | candidate table은 #50-#69를 다루고 rejected/deferred pattern은 별도로 나열되어 있다. |
| decision은 관련 umbrella issue #32부터 #36으로 다시 link한다. | PASS | 각 track section은 umbrella issue heading을 갖는다. |

## 검증 명령

```bash
git diff --check
rg -n "#32|#33|#34|#35|#36|#50|#51|#52|#53|#54|#55|#56|#57|#58|#59|#60|#61|#62|#63|#64|#65|#66|#67|#68|#69|Accept first|Accept after|Accept bounded|Defer|Reject" docs/superpowers/plans/2026-06-23-issue-79-example-selection-scorecard.md
```

## 잔여 위험

- score는 planning guidance이지 implementation proof가 아니다. future runnable PR에는 여전히
  package-level test, README update, CI verification이 필요하다.
- Docker/Testcontainers cost는 #47/#48 research에서 추정한 값이다. 이후 각 issue를 시작할 때
  actual upstream package surface에 대해 다시 확인해야 한다.
