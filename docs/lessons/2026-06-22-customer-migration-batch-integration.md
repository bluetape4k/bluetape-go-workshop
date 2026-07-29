# Customer Migration Batch 통합 Lesson

## 맥락

- Issue #29는 v0.5.0 batch umbrella다.
- 이 구현은 남은 child scope를 닫는다.
  - #42 Gin batch operations API.
  - #43 leader-guarded scheduled batch.
  - #75 customer migration 통합 예제.
- 선행 focused issue #41, #73, #74는 이미 checkpoint restart, policy reporting,
  retry/dead-letter 동작을 다뤘다. 필요한 빈칸은 또 다른 primitive demo가 아니라
  실행 가능한 composition example이었다.

## 구현 메모

- 예제는 `examples/customer-migration-batch-integration` 아래에서 self-contained로
  유지한다. 기존 예제 package는 `internal` 아래에 있으므로 sibling example끼리
  import하면 Go visibility rule을 깨게 된다.
- tutorial에서는 고정 `DefaultChunkSize = 2`를 사용한다. simulated crash는 새
  customer 3개를 쓰지만 checkpoint를 마지막 complete chunk로 되돌려 restart가
  boundary chunk를 명확히 replay하도록 만든다.
- email은 내부적으로 저장하되 public JSON projection에서는 제외한다. README와 test는
  ID, count, checkpoint, 안정적인 error code로 설명해야 한다.
- leader logic은 명시적으로 유지한다. `leader.ErrAlreadyLeader`는 이 process가 tick을
  실행할 수 있지만, 같은 call 안의 다른 곳에서 얻은 lease를 resign하면 안 된다는
  뜻이다.
- `context.WithoutCancel(ctx)`는 caller cancellation 이후 best-effort leader resign
  같은 bounded cleanup에만 사용한다. 일반 batch work는 caller cancellation semantics를
  유지해야 한다.
- HTTP API는 인증 없는 operations demo로 취급한다. 별도의 auth/trust-boundary design을
  추가하기 전까지 default bind와 override bind는 loopback-only로 유지해야 한다.

## 검증 명령

```bash
make lint
go test -count=1 ./examples/customer-migration-batch-integration/...
go test -race -count=1 ./examples/customer-migration-batch-integration/...
GOFLAGS=-p=1 make ci
git diff --check
```

Live smoke coverage는 README flow를 사용했다.

- held leader: health, manual crash, status, schedule restart, report, idle
  cancel, malformed JSON, blank run id, oversized body.
- missing leader: `not_leader` schedule rejection과 `leader_held=false`를 보여주는
  status.

## Review 결과

- Step 5 verifier: PASS.
- Step 4-P performance/stability scan: finding 없음.
- Step 6-R six-lane review: P0 = 0, P1 = 0.
- Review artifact:
  `docs/review/2026-06-22-issue-29-batch-integration-code-review.md`.

## Diagram Review Lesson

- arrowhead direction은 CairoSVG로 렌더링된 PNG를 authoritative evidence로 취급한다.
  marker definition과 SVG path order만으로는 충분하지 않다. 최종 PNG에서 좌/우/상/하
  방향 head를 모두 확인하고, 조밀하거나 꺾인 connector 영역은 native-pixel crop으로
  점검한다.
- target 앞 terminal segment에는 arrowhead 크기를 예약한다. 14 px primary architecture
  head라면 최소 14 px의 곧고 수직인 approach가 남도록 final bend를 충분히 앞당긴다.
  cramped route를 유효해 보이게 만들려고 marker를 줄이지 않는다.
- incoming/outgoing relationship은 별도 card port와 corridor에 둔다. 첫 architecture
  draft는 아래 방향 `Gin Router -> Service` path와 위 방향 scheduled-admission path가
  겹쳤다. 후자를 Service right-side port로 옮겨 PNG에서 두 방향을 모두 분명하게 만들었다.
- geometry script가 통과해도 bend 뒤 짧은 segment는 렌더링된 head를 회전시키거나
  압박할 수 있다. `Service -> Job + Step` route는 terminal segment를 4 px에서 18 px로
  늘려야 했다. `Job + Step -> Checkpoint Store` route는 5 px horizontal tail을
  88 px straight vertical entry로 바꿔 head가 target edge 아래 방향으로 들어가게 했다.
- `paths=0`, `connectors=0` 같은 약한 generic audit count를 PASS로 받아들이지 않는다.
  audit이 인식하는 connector class나 targeted invariant를 추가한 뒤 의미 있는 nonzero
  count를 요구한다. 수리된 architecture는 intrusion, crossing, endpoint, geometry,
  mixed-corner failure 없이 `connectors=13`, `paths=2`, `q_bends=6`을 기록한다.
- 마지막 coordinate 또는 connector-class 변경 뒤에는 2x로 다시 렌더링하고 전체 이미지
  점검과 focused original-pixel crop을 모두 반복한다. geometry 변경 뒤에는 이전 visual
  approval이 stale하다.

## Follow-Up Risk

- process restart는 의도적으로 모든 demo state를 reset한다. in-memory store를 durable로
  설명하지 않는다.
- 이 API를 loopback 밖으로 노출하려면 새로운 authenticated operator boundary가 필요하다.
  `HTTP_ADDR`만 바꿔 non-loopback address로 쓰는 방식은 의도적으로 거부한다.
- production scheduler에는 durable lease, observability, retry backoff가 필요하다.
  workshop scheduler는 의도적으로 작은 injectable ticker loop다.
