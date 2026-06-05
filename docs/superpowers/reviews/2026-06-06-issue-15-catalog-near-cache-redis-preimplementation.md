# Issue #15 Step 3-P Pre-Implementation Prediction

References:

- `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`
- `docs/superpowers/plans/2026-06-06-issue-15-catalog-near-cache-redis-plan.md`
- `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`
- `/Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`

## Predicted Risks And Mitigations

| Priority | Area | Risk | Mitigation |
|---|---|---|---|
| P1 avoided | Pub/Sub timing | Peer invalidation can arrive after the write assertion. | Use bounded eventual assertions for miss/reload checks. |
| P1 avoided | Stampede proof | Concurrent reads may not overlap and can falsely run two loaders or hide coordination. | Use a blocking loader hook, wait until one load starts, then release both callers. |
| P1 avoided | Resource lifecycle | Near-cache subscribers and Redis clients can leak goroutines or sockets. | `Peer.Close` closes near-cache; tests register peer and client cleanup. |
| P1 avoided | Error contract | Missing SKU can be accidentally cached or returned as an ambiguous error. | Use a sentinel missing error and assert repeated miss behavior. |
| P2 avoided | Testcontainers stability | Parallel Testcontainers tests can cause slow or flaky CI. | Keep targeted and race Testcontainers commands serial. |
| P2 avoided | Diagram drift | README diagrams can diverge from actual implementation. | Generate diagrams after code exists and include source-role names from final package. |

Step 3-P verdict: PASS. Implementation is unblocked.
