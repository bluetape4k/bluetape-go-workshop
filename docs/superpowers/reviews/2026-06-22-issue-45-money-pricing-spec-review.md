# Issue #45 명세 리뷰

검토한 spec: `docs/superpowers/specs/2026-06-22-issue-45-money-pricing-design.md`

이 session에서는 native subagent lane을 사용할 수 없었으므로, 이 artifact는 필요한 여섯 독립 관점을
main-session review lane으로 분리해 기록한다.

| Tier | 관점 | 범위 | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | money arithmetic, rule count, HTTP request body size. | 0 | 0 | 0 | 0 |
| 2 | Stability | currency mismatch, invalid amount, deterministic rule rejection. | 0 | 0 | 0 | 0 |
| 3 | Security | public error code, parser detail scrubbing, no secret. | 0 | 0 | 0 | 0 |
| 4 | Operator | local-only service, health endpoint, no external dependency. | 0 | 0 | 0 | 0 |
| 5 | Developer/API | package boundary, no reusable rule framework, Go idiom. | 0 | 0 | 1 | 0 |
| 6 | User/Caller | README lesson clarity, rejected-rule response visibility. | 0 | 0 | 1 | 0 |

## finding

| Priority | 영역 | finding | 필요한 spec edit | 상태 |
|---|---|---|---|---|
| P2 | Developer/API | 현재 `bluetape-go` module에는 `money`가 있지만 reusable rule package는 없다. workshop-local rule framework는 #45를 초과한다. | rule은 작은 application-local primitive라고 명시하고 reusable framework 작업은 non-goal로 나열한다. | Applied |
| P2 | User/Caller | rejected pricing rule은 generic validation error로 숨겨지지 않고 API response에 보여야 한다. | `accepted`, `rejected`, `skipped` status가 있는 rule decision과 rejection path acceptance test를 추가한다. | Applied |

최종 판정: P0 = 0, P1 = 0. spec은 implementation planning 준비가 됐다.
