# Issue #15 Step 3-P 구현 전 예측

참조:

- `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`
- `docs/superpowers/plans/2026-06-06-issue-15-catalog-near-cache-redis-plan.md`
- `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`
- `/Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`

## 예측한 위험과 완화책

| Priority | Area | Risk | Mitigation |
|---|---|---|---|
| P1 avoided | Pub/Sub timing | peer invalidation이 write assertion 뒤에 도착할 수 있다. | miss/reload check에는 bounded eventual assertion을 사용한다. |
| P1 avoided | Stampede proof | concurrent read가 겹치지 않아 loader 두 개가 잘못 실행되거나 coordination이 숨어 보일 수 있다. | blocking loader hook을 사용하고, load 하나가 시작될 때까지 기다린 뒤 두 caller를 모두 release한다. |
| P1 avoided | Resource lifecycle | near-cache subscriber와 Redis client가 goroutine이나 socket을 leak할 수 있다. | `Peer.Close`가 near-cache를 닫고, test는 peer와 client cleanup을 등록한다. |
| P1 avoided | Error contract | missing SKU가 실수로 cache되거나 모호한 error로 반환될 수 있다. | sentinel missing error를 사용하고 repeated miss behavior를 assert한다. |
| P2 avoided | Testcontainers stability | parallel Testcontainers test는 CI를 느리거나 flaky하게 만들 수 있다. | targeted/race Testcontainers command를 serial로 유지한다. |
| P2 avoided | Diagram drift | README diagram이 실제 구현과 달라질 수 있다. | code가 존재한 뒤 diagram을 생성하고 최종 package의 source-role name을 포함한다. |

Step 3-P 판정: PASS. implementation은 unblocked 상태다.
