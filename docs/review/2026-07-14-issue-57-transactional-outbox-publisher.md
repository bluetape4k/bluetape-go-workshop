# Issue #57 Transactional Outbox Publisher 리뷰

## 범위와 기준선

- 브랜치: `feat/issue-57-transactional-outbox-publisher`
- 기준: `origin/develop@a7f2627d9e8457a4c88e910692a67016842a1032`
- Library 기준선: released `bluetape-go` v0.18.0 APIs
- 범위: application-shaped PostgreSQL order placement와 released SQL outbox relay,
  Redis Streams publisher의 조합. reusable outbox implementation은 만들지 않는다.
- CodeGraph fallback: repository query가 indexed node와 edge를 0개 반환했으므로
  source, test, issue contract, 직접 diff inspection을 사용했다.

## 인수 리뷰

| 계약 | 증거 | 결과 |
| --- | --- | --- |
| Atomic order and outbox commit | 실제 PostgreSQL commit/rollback test가 하나의 `sqlkit.WithTx`와 `Store.Enqueue`를 사용한다 | PASS |
| Released relay behavior | success, 정확한 250 ms retry, third-attempt dead letter, cancellation, continuous run, concurrent claim test | PASS |
| Redis adapter contract | 실제 Redis test가 13개 scalar/envelope field와 decoded `entry_json` parity를 검증한다 | PASS |
| Stable delivery identity | retry test가 `event_id`와 `idempotency_key`를 보존한다. stale pending output은 fail closed 처리된다 | PASS |
| Resource lifecycle | partial-open cleanup, idempotent close, joined cancellation, close-failure stdout suppression test | PASS |
| Runnable lesson | pinned PostgreSQL/Redis command가 문서화된 JSON과 정확히 13개 Redis field를 생성한다 | PASS |
| Bilingual documentation | English/Korean 예제가 대응되는 run, test, field, retry, replay, production boundary를 노출한다 | PASS |
| Navigation | 두 root README가 예제와 released package set을 link한다 | PASS |

## Six-lens 리뷰

| 관점 | 결과 | 결정 |
| --- | --- | --- |
| Performance | P0=0, P1=0 | claim과 stream verification은 bounded이다. throughput claim은 하지 않는다. |
| Stability | P0=0, P1=0 | shared clock, 정확한 retry eligibility, joined cancellation, deterministic cleanup이 다뤄진다. |
| Security | P0=0, P1=0 | identifier는 bounded/validated이고 SQL은 parameterized다. endpoint는 출력하지 않으며 replay에는 별도 authorization/audit policy가 필요하다. |
| Operator/Ops | P0=0, P1=0 | SQL은 durable source로 명명되고 Redis는 transport다. recovery, retention, trimming, metrics, replay automation은 명시적 production work로 남는다. |
| Developer/API | P0=0, P1=0 | workshop code는 library abstraction을 복제하지 않고 released store, relay, publisher, fixture, transaction helper를 조합한다. |
| User/caller | P0=0, P1=0 | 정확한 command, output, rerun identity, failure boundary, 두 diagram이 양쪽 locale에서 보인다. |

독립 code-review lane은 남은 CRITICAL/HIGH/MEDIUM/LOW finding 없이 `APPROVE`로
끝났다. 독립 architecture lane은 P0/P1/P2/P3가 모두 0인 `CLEAR`로 끝났다.
초기 review finding은 다음으로 수정했다.

- 두 client가 성공적으로 close될 때까지 success JSON을 buffer한다.
- close-failure 및 stale-pending fail-closed regression test를 추가한다.
- 오른쪽으로 향하는 `XADD` call과 왼쪽으로 돌아오는 error/success return을 분리한다.
- cancellation branch를 독립 claim-to-cancel sequence로 만든다.
- clean-state command, 독립 production lifecycle, durable SQL source, Redis
  transport role, replay authorization boundary를 문서화한다.

## 다이어그램 검증 기록

| Asset | 자동화 증거 | Render 및 눈검사 |
| --- | --- | --- |
| Architecture | marker 4개, connector 8개, card 9개, intrusion 0, crossing 0, geometry failure 0. endpoint 및 mixed-corner PASS with 3 Q bends | 3000x1800 PNG; SHA-256 `caecee685692d8269d11db6130dbc04c8ac7c568ed346bee6dbdca7704e46bf6`; CairoSVG byte parity PASS; original 및 full-resolution quadrant inspection PASS |
| Sequence | marker 5개, connector 20개, card 6개, intrusion 0, crossing 0, geometry failure 0. endpoint, mixed-corner, sequence-style PASS | 3200x2440 PNG; SHA-256 `24903cf08a663364b2892c3f883219e1f7d65840d9cc1f704bac602780869394`; CairoSVG byte parity PASS; original 및 full-resolution quadrant inspection PASS |

PNG inspection은 arrowhead 방향과 크기, dashed return-marker rendering, card edge
앞 terminal straight clearance, rounded bend, activation endpoint, label,
line/card intrusion, crossing, whitespace를 명시적으로 확인했다. call은 receiver를
향하고 error/success return은 caller로 돌아간다.

## 최신 검증

마지막 code 및 diagram 변경 뒤 다음 명령이 exit 0으로 끝난 것을 확인했다.

```bash
git diff --check
golangci-lint run ./examples/transactional-outbox-publisher/...
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
go test -count=10 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRelayConcurrentRunOnce$'
make ci
```

README container command도 `postgres:16-alpine` 및 `redis:7.4-alpine` clean
state에서 실행했다. 명령은 문서화된 newline-terminated JSON object를 출력했고,
`XRANGE`에는 문서화된 13개 field name이 정확히 포함됐다. 두 container는 이후
제거했다.

첫 full lint 시도는 누락된 exported comment, 의도적 nil context literal, stale
deleted-worktree cache entry를 드러냈다. source finding은 수정했고 cache를
정리했으며, 이후 focused lint와 fresh `make ci` exit 0만 최종 증거로 인정한다.

## 통합 결정

구현은 P0=0, P1=0으로 PR-ready다. delivery는 설계상 at-least-once다. 모호한
Redis success는 SQL row가 marked되기 전에 두 번 이상 publish될 수 있다.
consumer는 stable event identity로 deduplicate해야 한다. merge, local
synchronization, worktree cleanup은 live PR CI 성공 뒤 새 명시 승인 전까지
blocked 상태다.
