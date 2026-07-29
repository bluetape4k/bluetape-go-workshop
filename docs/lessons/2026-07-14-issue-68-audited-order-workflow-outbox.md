# Issue #68 Audited Order Workflow Outbox Lesson

## 맥락과 선택한 Boundary

workshop에는 이미 audit history와 transactional outbox lesson이 분리되어 있었고, released
`bluetape-go` v0.18.0은 audit contract, SQL outbox relay, Redis Streams publisher를 제공했다.
다음 유용한 lesson은 그들의 application boundary였다. audited order command 하나를 HTTP로 받고,
current order state와 immutable history와 outbox record를 하나의 PostgreSQL transaction으로
commit한 뒤, committed audit event를 asynchronously publish한다.

history와 outbox는 서로 다른 truth를 담당한다. PostgreSQL order/history row는 durable query와
audit source다. outbox는 durable delivery intent다. Redis Streams는 audit storage가 아니라
transport다. 이 역할을 명시적으로 유지하면 Redis degradation이 committed workflow history를
사용 불가로 만드는 일을 막고, 잘못된 distributed transaction을 가르치지 않게 된다.

## Replay, Readiness, Shutdown

idempotency key는 outbox row만이 아니라 전체 command에 속한다. 같은 identity와 같은 canonical
payload로 retry하면 stored result를 반환한다. 다른 payload로 identity를 재사용하면 그 command에
scoped된 conflict다. conflict path는 history revision을 append하거나 다른 event를 enqueue하면
안 된다.

readiness는 durable ownership을 따른다. SQL failure는 service를 unready로 만든다. Redis
transport failure는 degraded로 보고하지만 accepted SQL work는 relay로 recoverable하게 남는다.
unexpected relay exit는 delivery progress가 멈췄기 때문에 process를 unready로 만든다. status
probe는 failing dependency가 전체 handler pool을 소비하지 않도록 bounded로 남아야 한다.

cancellation만으로는 shutdown bound가 아니다. 첫 lifecycle implementation은 relay cancellation을
호출한 뒤 relay join을 무기한 기다렸다. cancellation을 무시하는 test relay가 advertised deadline을
넘길 수 있음을 증명했다. fix는 HTTP shutdown과 relay join에 하나의 shared deadline budget을
사용하고, graceful HTTP shutdown이 budget을 다 쓰면 server close를 강제하며, relay가 여전히
join하지 않으면 redacted timeout을 반환한다. 향후 supervised worker는 lifecycle 승인 전에 같은
adversarial test를 받아야 한다.

## 통합과 Tooling 의외점

container-backed proof는 serialize해야 한다. PostgreSQL을 Redis보다 먼저 띄우고, Docker resource를
공유하는 example integration, smoke, repository test, race gate는 sequential하게 실행한다.
parallel green run은 port, cleanup, container lifecycle의 deterministic ownership을 증명하지
못한다.

첫 lint pass는 실제 quality finding 63개를 찾았다. public teaching documentation 누락,
unchecked row-close failure, direct error comparison, unwrapped error, contextless network
call, staticcheck conversion/format issue, empty compare-and-swap loop이었다. 이를 고치면서
lesson과 failure semantics가 모두 개선됐다. 모든 source finding을 분류하기 전에는 큰 lint count를
tooling problem으로 축소하지 않는다. cache cleanup은 verified stale-worktree path에만 사용한다.

HTTP smoke test는 handler를 process 안에서 호출하는 데 그치지 않고 documented application을
실행해야 한다. 그래야 listener binding, JSON transport, readiness, route wiring, lifecycle,
copyable `requests.http` scenario가 같은 program을 설명한다는 점을 증명한다.

## Diagram Lesson

automated geometry check는 필요하지만 불완전하다. SVG source뿐 아니라 final PNG를 inspect해야
한다. marker rendering이 apparent direction을 뒤집거나 arrowhead를 bend와 충돌할 만큼 키울 수
있기 때문이다. 모든 endpoint에는 최소 marker-size terminal straight segment를 주고, route는 card
edge에서 끝내며, rendered marker가 그 clearance를 소비하면 bend coordinate를 옮긴다.

architecture audit는 모든 connector, card intrusion, crossing, endpoint, mixed corner를
확인했다. sequence audit는 message direction과 activation layout을 추가로 확인했다. 그래도
original-pixel quadrant inspection은 필요했다. geometry script가 flag하지 못한 phase-title과
pill overlap을 잡았기 때문이다. 첫 message row와 activation bar를 옮긴 뒤 full preview와 모든
original-resolution crop을 다시 inspect했다.

sequence style은 오래된 nearby repository diagram에서 추론하지 말고 current best-practices
reference와 시각적으로 비교해야 한다. 첫 version은 rounded pill을 사용했지만 call number를 plain
inline text로 남겼다. 승인된 baseline은 34-pixel pill 안의 semantic-color circular badge로 number를
분리한다. 이 pattern을 message 22개 전체에 적용하려면 badge가 label을 message line 위로 밀지
않도록 row, phase, activation, lifeline, frame, canvas coordinate도 더 커져야 했다.

향후 diagram에서는 이 순서를 유지한다.

1. SVG에서 connector count, endpoint, crossing, card intrusion, bend geometry를 audit한다.
2. PNG를 deterministic하게 render하고 두 번째 render hash와 비교한다.
3. PNG에서 모든 arrowhead direction과 marker clearance를 inspect한다.
4. one-/two-digit number를 포함해 numbered pill과 badge styling을 current best-practices
   sequence PNG와 비교한다.
5. text, pill, activation, card overlap을 original-pixel crop으로 inspect한다.
6. coordinate가 바뀌면 모든 audit와 eye inspection을 다시 실행한다.

## 향후 Guard

이 workshop을 generic workflow engine으로 바꾸거나 exactly-once delivery를 약속하지 않는다.
production adoption에는 여전히 authentication/authorization, tenant isolation, TLS/proxy policy,
schema migration, outbox retention, stream trimming, consumer group, lag/dead-letter metric,
audit record가 있는 authorized replay, stable event identity 기반 consumer deduplication이 필요하다.

예제를 확장하기 전에는 다음 regression guard를 보존한다. atomic rollback,
same/different-payload replay, concurrent command serialization, relay lease recovery, Redis
degradation, unexpected relay exit, cancellation-ignoring relay shutdown, actual-app HTTP
smoke, 이중 언어 계약 parity, 렌더링된 다이어그램 검사를 보존한다.
