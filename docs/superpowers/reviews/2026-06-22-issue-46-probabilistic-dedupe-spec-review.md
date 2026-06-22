# Issue #46 Spec Review

Reviewed spec: `docs/superpowers/specs/2026-06-22-issue-46-probabilistic-dedupe-design.md`

Native subagent lanes are unavailable in this session, so this artifact records the required six independent perspectives as separate main-session review lanes.

| Tier | Perspective | Scope | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|
| 1 | Performance | Bloom filter hot path and stats reads. | 0 | 0 | 0 | 0 |
| 2 | Stability | Shared filter state, repeat events, invalid input. | 0 | 0 | 1 | 0 |
| 3 | Security | Public error shape, no sensitive payload echoing. | 0 | 0 | 0 | 0 |
| 4 | Operator | Durable-store caveat and production boundary docs. | 0 | 0 | 1 | 0 |
| 5 | Developer/API | `probabilistic` package fit, no reusable framework. | 0 | 0 | 0 | 1 |
| 6 | User/Caller | False-positive wording and scenario clarity. | 0 | 0 | 1 | 0 |

## Findings

| Priority | Area | Finding | Required spec edit | Status |
|---|---|---|---|---|
| P2 | Stability | A shared in-memory filter makes race validation relevant even if the upstream filter is documented as goroutine-safe. | Add targeted `go test -race` acceptance criterion. | Applied |
| P2 | Operator | The acceptance criteria require false-positive and durable-store explanation. | Add explicit README requirement and production non-goal. | Applied |
| P2 | User/Caller | "Duplicate" wording would overclaim Bloom filter certainty. | Use `probably_seen` decision terminology. | Applied |
| P3 | Developer/API | `source` exists only for scenario shape and should not alter keying. | State that trimmed `event_id` is the prefilter key. | Applied |

Final verdict: P0 = 0, P1 = 0. Spec is ready for implementation planning.
