# Issue #57 Transactional Outbox Publisher Lesson

## 맥락과 Boundary

workshop에는 이미 service-owned SQL transaction example이 있었고, bluetape-go v0.18.0은
`audit/sqloutbox`, deterministic test publisher, Redis Streams adapter를 제공했다. 따라서 유용한
lesson은 새 outbox implementation이 아니었다. application boundary가 핵심이었다. 같은
`*sql.Tx`를 통해 order를 insert하고 `Store.Enqueue`를 호출하며, commit 이후에만 order를
반환하고, caller-owned relay가 나중에 publish하게 한다.

runnable command는 의도적으로 bounded `RunOnce` batch 하나만 실행한다. 기존 pending record가
normal outbox ordering에서 먼저 처리되므로 command는 published identity와 new command를 비교해
wrong event를 success로 출력하지 않고 fail closed한다. production worker에는 이 single-record
teaching invariant가 아니라 continuous relay와 explicit restart/replay policy가 필요하다.

success output도 lifecycle contract의 일부다. command는 PostgreSQL과 Redis client가 성공적으로
close될 때까지 JSON을 buffer한다. 따라서 shutdown error가 nonzero exit 옆 stdout에 그럴듯한
success object를 남길 수 없다. dedicated close-failure test는 stdout이 empty로 남는지 assert한다.

## Retry, Identity, Cancellation 경계

store와 relay test는 같은 injected clock을 공유해야 한다. clock 하나만 전진시키면 persisted
`retry_at`과 claim eligibility check가 같은 timeline을 설명하지 않으므로 retry test가 오해를
만들 수 있다. test는 이제 250 ms 전 no claim, 정확히 250 ms의 eligibility, attempt 1과 2,
변하지 않는 `event_id`와 `idempotency_key`를 증명한다.

publisher boundary의 caller cancellation은 일반 delivery failure가 아니다. released relay는
`context.Canceled`를 반환하고 record를 attempt 1로 claimed 상태에 둔다. retry를 schedule하거나
row를 dead-letter하지 않는다. 따라서 continuous-run shutdown에는 sleep이나 cancellation이
delivery state를 rewrite한다는 assertion이 아니라 joined goroutine test가 필요하다.

at-least-once는 consumer contract로 남는다. publisher가 ambiguous error를 관찰하기 전이나 SQL
completion mark가 durable해지기 전에 Redis가 `XADD`를 받을 수 있다. retry와 operator replay는
identity를 보존해야 하며, consumer는 attempt counter를 identity로 취급하지 말고 deduplicate해야
한다.

## 통합과 Tooling 누락

real PostgreSQL/Redis test는 Redis field 13개를 모두 확인하고, `entry_json`을 decode하며,
aggregate, revision, event identity, event type, schema version을 scalar field와 비교한다.
stream length만 확인하면 transport reachability만 증명하고 adapter compatibility는 증명하지 못한다.

첫 repository-wide lint run은 독립 원인 두 가지를 찾았다.

- 새 exported teaching symbol에 repository가 요구하는 package/API comment가 없었고, contract
  test의 literal nil context가 SA1012를 trigger했다.
- golangci-lint가 삭제된 worktree의 finding을 여전히 들고 있었다.

유용한 English comment가 source finding을 고쳤다. typed zero-value `context.Context` variable은
static analysis를 suppress하지 않으면서 의도적인 nil-context contract를 보이게 유지한다.
focused package lint가 source fix를 증명했고, `golangci-lint cache clean` 뒤 repository-wide
lint가 deleted path 문제는 environment였음을 증명했다. full `make ci`는 처음부터 다시 실행했고
observed exit code가 0인 뒤에만 인정했다.

## Diagram Lesson

SVG path가 marker를 가진다는 이유만으로 connector audit의 일부가 되지는 않는다. 첫 sequence
source는 `callBlue` 같은 semantic class를 사용했지만 common `connector` class를 빠뜨려 message
path가 2개만 count됐다. 이제 모든 message path가 두 class를 모두 가지며 final audit는 20개
전체를 보고한다.

SVG correctness도 PNG deliverable의 충분한 evidence가 아니다. final inspection은 두 diagram을
CairoSVG로 render하고 deterministic PNG parity를 확인했으며 resizing 없이 original-resolution
quadrant crop을 열었다. 이를 통해 conversion 뒤 중요한 failure를 잡거나 방어했다.

- 의도한 message direction과 반대로 보이는 arrowhead.
- arrowhead에 너무 가까운 bend로 인해 terminal straight segment가 남지 않는 문제.
- 여러 semantic route가 card port를 공유하며 시각적으로 합쳐지는 문제.
- card 밖으로 넘치는 label 또는 unsupported glyph.
- otherwise valid pixel을 숨기는 whole-image viewer artifact.

architecture는 outbox card의 transaction port와 relay port를 분리하고 모든 terminal 앞에 최소
marker-size clearance를 유지한다. sequence는 explicit fixed-size marker, left-pointing return
arrow, right-pointing call, distinct cancellation color를 사용한다. automated connector,
geometry, endpoint, mixed-corner, sequence-style audit는 PNG eye inspection을 보완하지만
대체하지 않는다.

sequence semantics도 geometry만큼 꼼꼼히 봐야 한다. `XADD`는 publisher에서 Redis로 향하는
right-going call이고, error 또는 success는 별도 dashed left-going return이다. “XADD returns
error”만 Redis 방향으로 그리면 arrowhead는 기술적으로 맞더라도 반대 interaction을 가르칠 수
있다. cancellation branch도 publish cancellation을 보이기 전에 자체 claim을 시작하므로 위 success
branch에 시각적으로 의존하지 않는다.

## 향후 Guard

이 예제를 reusable worker framework로 확장하지 않는다. production work는 application
infrastructure 또는 bluetape-go에 속하며 measured batch size, connection pooling, consumer
group, stream trimming, outbox retention, lag metric, dead-letter inspection 및 authorized
replay, tenant isolation, authentication/TLS, versioned migration이 필요하다. automated replay나
exactly-once claim을 추가하기 전에는 design을 다시 연다. 둘 다 operational contract를 바꾼다.
