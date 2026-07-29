# Issue #46 명세 리뷰

검토한 spec: `docs/superpowers/specs/2026-06-22-issue-46-probabilistic-dedupe-design.md`

이 session에서는 native subagent lane을 사용할 수 없었으므로, 이 artifact는 필요한 여섯 독립 관점을
main-session review lane으로 분리해 기록한다.

| Tier | 관점 | 범위 | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | Bloom filter hot path와 stats read. | 0 | 0 | 0 | 0 |
| 2 | Stability | shared filter state, repeat event, invalid input. | 0 | 0 | 1 | 0 |
| 3 | Security | public error shape, no sensitive payload echoing. | 0 | 0 | 0 | 0 |
| 4 | Operator | durable-store caveat과 production boundary docs. | 0 | 0 | 1 | 0 |
| 5 | Developer/API | `probabilistic` package fit, no reusable framework. | 0 | 0 | 0 | 1 |
| 6 | User/Caller | false-positive wording과 scenario clarity. | 0 | 0 | 1 | 0 |

## finding

| Priority | 영역 | finding | 필요한 spec edit | 상태 |
|---|---|---|---|---|
| P2 | Stability | upstream filter가 goroutine-safe로 문서화되어 있더라도 shared in-memory filter는 race validation을 relevant하게 만든다. | targeted `go test -race` acceptance criterion을 추가한다. | Applied |
| P2 | Operator | acceptance criteria는 false-positive와 durable-store 설명을 요구한다. | explicit README requirement와 production non-goal을 추가한다. | Applied |
| P2 | User/Caller | "Duplicate" 표현은 Bloom filter certainty를 과장할 수 있다. | `probably_seen` decision terminology를 사용한다. | Applied |
| P3 | Developer/API | `source`는 scenario shape에만 존재하며 keying을 바꾸면 안 된다. | trimmed `event_id`가 prefilter key임을 명시한다. | Applied |

최종 판정: P0 = 0, P1 = 0. spec은 implementation planning 준비가 됐다.
