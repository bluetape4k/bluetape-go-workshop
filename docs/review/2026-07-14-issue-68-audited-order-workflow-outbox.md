# Issue #68 Audited Order Workflow Outbox 리뷰

## 범위와 기준선

- 브랜치: `feat/issue-68-audited-order-workflow-outbox`
- 기준: `origin/develop@64d1a80dd88ecc365957b19f7fb74d70098120fc`
- 리뷰한 구현 head: `b453cc4`
- Library 기준선: released `bluetape-go` v0.18.0 APIs
- 범위: order, immutable audit history, SQL outbox record를 원자적으로 commit한 뒤
  audit event를 Redis Streams로 relay하는 application-shaped HTTP order workflow
- 경계: reusable history store, outbox relay, Redis publisher는 workshop에서
  재구현하지 않았다.

## 인수 리뷰

| 계약 | 증거 | 결과 |
| --- | --- | --- |
| Atomic workflow write | 실제 PostgreSQL test가 order state, immutable audit history, official SQL outbox entry를 하나의 `*sql.Tx`로 commit한다. rollback 뒤에는 셋 모두 남지 않는다 | PASS |
| Strict POST JSON API | handler는 JSON content type, 하나의 bounded object, known field, unique key, valid UTF-8, bounded identity/reason/metadata value를 강제한다 | PASS |
| Replay semantics | 같은 identity와 같은 payload는 저장된 response를 replay한다. 같은 identity와 다른 payload는 history/outbox data를 중복하지 않고 scoped conflict를 반환한다 | PASS |
| History query | store는 bounded cursor pagination과 immutable ordered revision으로 released audit history reader contract를 구현한다 | PASS |
| Released relay integration | supervised `sqloutbox.Relay`가 released Redis Streams adapter를 통해 publish하고 event identity를 보존하며 lease를 복구하고 SQL source를 잃지 않은 채 Redis degradation을 견딘다 | PASS |
| Lifecycle and readiness | SQL은 hard readiness dependency이고 Redis/relay state는 별도로 보인다. unexpected relay exit는 readiness를 실패시키고 shutdown은 하나의 hard deadline을 공유한다 | PASS |
| Runnable lesson | `curl` 및 `requests.http`는 success, replay, conflict, validation, history, readiness, status inspection용 POST JSON scenario를 제공한다 | PASS |
| Bilingual documentation | English/Korean README가 같은 run path, API contract, architecture, failure model, production boundary를 설명한다 | PASS |
| Visual explanation | matching SVG/PNG architecture 및 sequence diagram이 양쪽 locale에 embed되어 있고 automated/rendered visual check를 통과했다 | PASS |

## Six-lens 리뷰

| 관점 | 결과 | 결정 |
| --- | --- | --- |
| Performance | P0=0, P1=0 | request body, concurrency, database/Redis pool, query page, relay claim, status probe는 bounded다. throughput claim은 하지 않는다. |
| Stability | P0=0, P1=0 | row locking, transaction rollback, concurrent replay/conflict test, lease recovery, relay supervision, restart/backlog test, shared shutdown deadline 하나가 failure boundary를 다룬다. |
| Security | P0=0, P1=0 | 예제는 loopback literal에만 bind하고 trusted proxy를 비활성화하며 input을 검증/제한하고 SQL을 parameterize한다. compression을 거부하고 public error/log를 redact한다. authentication, TLS, authorization, tenant isolation은 명시적 production work로 남는다. |
| Operator/Ops | P0=0, P1=0 | health, readiness, bounded status endpoint는 durable SQL health를 Redis transport degradation 및 relay state와 분리한다. runbook은 reset, retention, replay, at-least-once boundary를 문서화한다. |
| Developer/API | P0=0, P1=0 | workshop code는 dependency를 추가하거나 shared workflow를 바꾸지 않고 released v0.18.0 audit, SQL outbox, Redis Streams, Testcontainers API를 조합한다. exported teaching API에는 유용한 문서가 있고 lint는 clean이다. |
| User/caller | P0=0, P1=0 | copyable POST JSON example은 양쪽 locale에서 metadata, 정확한 response/error behavior, replay identity, cursor pagination, readiness, status inspection을 포함한다. |

통합 리뷰에는 남은 P0, P1, P2, P3 finding이 없다. 리뷰 중 P1 하나를 발견했다:
lifecycle shutdown이 cancellation을 무시하는 relay를 bound 없이 기다려 advertised
hard shutdown deadline을 초과할 수 있었다. failing regression이 먼저
`relay join exceeded shutdown deadline`을 재현했다. 현재 runtime은 HTTP shutdown과
relay join에 하나의 shared deadline을 쓰고, 필요하면 server close를 강제하며,
redacted `relay_shutdown: timeout` failure를 반환하고 readiness를 false로 표시한다.

첫 repository lint run은 noise로 취급되지 않고 63개 finding을 보고했다. 수정은
package 및 exported-symbol documentation을 추가하고, `rows.Close` error를
전파했으며, `errors.Is`와 wrapped error를 사용했다. contextless network operation을
교체하고 staticcheck conversion/formatting을 바로잡았으며 compare-and-swap loop
termination을 명시적으로 만들었다. 최종 lint run은 suppression 없이 `0 issues`를
보고했다.

## 다이어그램 검증 기록

| Asset | 자동화 증거 | Render 및 눈검사 |
| --- | --- | --- |
| Architecture | marker 5개, connector 10개, card 11개, intrusion 0, crossing 0, geometry failure 0. endpoint 및 mixed-corner PASS with 10 quadratic bends | 3200x2000 PNG; SHA-256 `91331f16a2efab3b0ab6ed6ee6e2b2cc94a3a1ec7743978fe71cb5d069c510a2`; deterministic rerender PASS; full source-size preview 및 original-pixel quadrant inspection 4개 PASS |
| Sequence | marker 6개, connector 22개, numbered pill/circular badge 22개, card 6개, intrusion 0, crossing 0, geometry failure 0. endpoint, mixed-corner, sequence-style PASS | 3200x3650 PNG; SHA-256 `e2f1dc14ddcc60c6222d24e06ce715d3759c565245dbb5cb53018087c3f91d08`; deterministic rerender PASS; full source-size preview 및 original-pixel quadrant inspection 4개 PASS |

렌더링된 PNG inspection은 SVG to PNG conversion 뒤 모든 arrowhead, marker size
대비 terminal straight clearance, rounded bend placement, receiver direction,
label/card overlap, route merging, clipping, whitespace를 명시적으로 확인했다.
sequence diagram은 처음에 automated geometry check가 보고하지 않은 phase-title과
pill overlap이 있었다. 첫 message row와 activation bar를 옮겨 수정했고 최종
render와 pixel crop을 다시 점검했다. 이후 best-practices 비교는 plain inline
call number가 더 오래된 label style을 쓰고 있음을 추가로 찾았다. 지금 22개
label은 모두 승인된 34-pixel pill, semantic-color 13-pixel circular number badge,
10-pixel label-to-line clearance, 인접 row 사이 최소 6px를 사용한다.

## 최신 검증

최종 구현 변경 뒤 다음 명령이 exit 0으로 끝난 것을 확인했다.

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/...
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox -run '^TestRequestsHTTPSmoke$'
go test -p 1 -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check origin/develop
```

Testcontainers-backed check는 순차 실행했다. smoke test는 handler를 직접 호출하지
않고 실제 application을 시작해 문서화된 HTTP request contract를 실행했다. 잃어버린
process handle과 이전 partial output은 증거로 인정하지 않았다. 기록된 full gate는
observed exit code 0을 가진다.

## 검증기 결정

| Gate | 결과 |
| --- | --- |
| Approved scope and released baseline | PASS |
| Contract and failure-path tests | PASS |
| Bilingual README parity and navigation | PASS |
| Diagram source, render, audits, and eye inspection | PASS |
| Repository formatting, tidy, vet, lint, normal tests, and race tests | PASS |
| Six-lens review with P0=0 and P1=0 | PASS |
| Clean integration handoff | PASS |

필수 check: 7/7; N/A: 0; Blocked: 0.

## 통합 결정

구현은 로컬에서 PR-ready다. delivery는 at-least-once로 남는다. relay가 SQL outbox
record를 durably mark하기 전에 Redis가 event를 accept할 수 있으므로 consumer는
stable event identity로 deduplicate해야 한다. push와 PR creation은 별도 handoff
단계다. merge, local synchronization, worktree cleanup은 live PR CI가 성공하고
review thread를 다시 확인한 뒤 명시 사용자 승인을 받을 때까지 blocked 상태다.
