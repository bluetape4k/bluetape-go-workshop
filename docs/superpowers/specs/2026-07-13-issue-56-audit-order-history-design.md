# Issue #56 Audit Order History 설계

## Status

Issue #56에 대한 user-approved design이며 bluetape-go v0.18.0 기준으로 구현되었다.
Type A specification review는 2026-07-13에 P0=0/P1=0으로 수렴했다.

## Goal

mutable current-state view와 immutable audit history를 대비하는 실행 가능한
application-shaped order service를 추가한다. 이 예제는 order creation과 status transition,
append-before-state consistency, duplicate command detection, aggregate history, filtered
query, deterministic JSON을 보여준다.

## Context and Current Evidence

Issue #56은 milestone track #35의 첫 dependency이며 Gin query API (#58), SQL outbox
publisher (#57), Redis Streams integration (#68)보다 앞선다. 현재 stable dependency는
bluetape-go v0.18.0이다.

released `audit` package는 `NewAggregateID`, `NewDomainEvent`, `NewEntry`,
`Repository.Append`, `LoadHistory`, `Find`, goroutine-safe `MemoryRepository`를 제공한다.
repository는 연속 revision을 검증하고 전역 중복 event ID와 idempotency key를 거부한다. 또한
defensive copy를 반환하고 context cancellation을 보존한다. 이 repository는 의도적으로
non-durable이다.

library-local `examples/audit`는 이미 create, add-item, complete, outbox replay를 보여준다.
이 workshop은 해당 예제의 append-before-mutation boundary를 채택하지만 scenario를 복사하지
않는다. workshop은 status history, current-state contrast, query window에 집중한다. outbox
replay는 issue #57과 #68이 소유하는 lesson이므로 여기서는 제외한다.

## Chosen Approach

주입된 `audit.Repository`, UTC clock, mutex, in-memory current-state map을 가진 작은
`orderhistory.Service`를 사용한다. 명시적인 `Create`, `Confirm`, `Ship`, `Cancel` method는
state machine과 failure mode를 읽기 쉽게 만든다. 모든 command는 하나의 fixed-size domain
event를 검증하고 생성한 뒤 append하고, 그 이후에야 같은 service lock을 잡은 상태에서 current
state를 update한다.

실행 가능한 preview는 고정 order lifecycle을 수행하고 current snapshot, full aggregate
history, filtered history window를 담은 deterministic indented JSON을 출력한다. repository
value는 audit source of truth로 남는다. mutable map은 교육용 projection일 뿐이다.

## Alternatives

### Direct repository fixture

`main` function이 hand-built entry를 append하고 query를 출력할 수도 있다. 더 작지만
validation, revision, idempotency, state mutation이 application boundary의 어디에 속하는지
가르칠 수 없다. 너무 얇아서 거부한다.

### Durable SQL current state and outbox

SQL transaction으로 order row를 update하고 audit message를 enqueue할 수 있다. production-shaped
접근이지만 첫 audit lesson을 persistence와 delivery에 섞어 버린다. issue #57과 #68로 미룬다.

### Event-sourced aggregate

current state를 audit event replay만으로 재구성할 수도 있다. package가 event store가 아니라
audit history를 명시적으로 model하므로 거부한다. README는 history replay가 recovery model이
아니라고 밝혀야 한다.

## Package and Files

```text
examples/audit-order-history/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/orderhistory/
    model.go
    service.go
    service_test.go
    preview.go
    preview_test.go
```

root `README.md`와 `README.ko.md`는 example을 link한다. dependency, module, workflow,
container, database, public library API, diagram 변경은 필요하지 않다.

## Service API Contract

internal package는 다음 compact teaching API를 노출한다.

```go
type Options struct {
    Author string
    Now    func() time.Time
}

type CreateCommand struct { OrderID, CommandID string }
type TransitionCommand struct { OrderID, CommandID string }
type CancelCommand struct { OrderID, CommandID, Reason string }

func NewService(repo audit.Repository, options Options) (*Service, error)
func (s *Service) Create(context.Context, CreateCommand) (Order, error)
func (s *Service) Confirm(context.Context, TransitionCommand) (Order, error)
func (s *Service) Ship(context.Context, TransitionCommand) (Order, error)
func (s *Service) Cancel(context.Context, CancelCommand) (Order, error)
func (s *Service) Current(orderID string) (Order, bool)
func (s *Service) History(context.Context, string) (audit.History, bool, error)
func (s *Service) Find(context.Context, audit.Query) ([]audit.Entry, error)
```

`NewService`는 nil repository와 유효하지 않은 author를 `ErrInvalidConfig`로 거부한다. nil
clock은 `time.Now().UTC`를 기본값으로 사용한다. nil context는 audit package와 맞게
`context.Background`로 정규화한다. `Service{}`는 사용할 수 있는 configuration이 아니다. 모든
method는 missing dependency를 감지하고 panic 대신 `ErrInvalidConfig`를 반환하거나 `Current`의
경우 `false`를 반환한다.

stable service sentinel은 `ErrInvalidConfig`, `ErrInvalidCommand`, `ErrOrderExists`,
`ErrOrderNotFound`, `ErrInvalidTransition`이다. construction, ID, reason, state-machine error는
`errors.Is`를 위해 대응 sentinel을 wrap한다. 100을 초과하는 `Find` limit 또는 incompatible
aggregate type은 `audit.ErrInvalidQuery`를 wrap한다. repository error는 다른 error로 번역해
없애지 않는다.

## Domain Model and State Machine

`Order`는 canonical order ID, status, revision, UTC update time을 포함한다. 지원 status는
`pending`, `confirmed`, `shipped`, `cancelled`이다.

허용 transition은 다음과 같다.

| Command | From | To |
|---|---|---|
| Create | absent | pending |
| Confirm | pending | confirmed |
| Ship | confirmed | shipped |
| Cancel | pending or confirmed | cancelled |

`shipped`와 `cancelled`는 terminal이다. 기존 order를 생성하거나, 없는 order에 작업하거나, 그
밖의 transition을 요청하면 audit history 또는 current state를 쓰지 않고 stable sentinel
error를 반환한다.

order ID와 command ID는 trim되며 `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`과 일치해야 한다. 이는
모호한 canonicalization을 막고 repository key를 bounded로 유지한다. caller는 고유 command ID를
제공한다. 같은 canonical value는 `EventID`와 `IdempotencyKey`에 모두 사용되며 lossy
concatenation으로 파생하지 않는다. 이것은 duplicate detection이지 성공적인 idempotent replay가
아니다. 이미 commit된 command를 다시 제출하면 실패하며 `errors.Is(err,
audit.ErrRevisionConflict)`를 보존한다.

각 event는 aggregate type `order`와 order용 `audit.AggregateID`를 사용한다. event type은
`order.created`, `order.confirmed`, `order.shipped`, `order.cancelled`이다. revision은 이전
current-state revision에 1을 더한 값이다. change는 application이 소유하는 고정 field인
`status` before/after와 optional cancellation reason이다. cancellation reason은 valid UTF-8이어야
하며 256 rune을 넘을 수 없다. 임의 caller JSON은 받지 않는다. author는 service configuration으로
고정되고, trim되며, 필수이고, 128 rune으로 제한된다.

## Consistency and Concurrency

각 command는 context를 확인하고, input을 검증하고, service mutex를 획득하고, context를 다시
확인하고, current state를 검증하고, event와 entry를 만든 뒤 mutex를 계속 잡은 상태에서
`Repository.Append`를 호출한다. append 성공만 map을 mutate한다. 이 순서는 revision selection을
직렬화하고 한 order에 대한 두 concurrent command가 race하지 못하게 한다. 예제는 per-aggregate
lock 장치보다 단순하고 눈에 보이는 invariant를 의도적으로 선호한다.

append success가 commit point다. append 전이나 중에 cancellation이 발생하면 repository가 error를
반환하고 어느 쪽도 변경되지 않는다. append가 성공한 뒤에는 바로 cancellation이 도착하더라도
in-memory state update가 실패 없이 lock 아래에서 완료된다. 이 두 operation 사이에서 cancellation을
확인하면 audit/current-state split이 생기므로 금지한다.

repository error는 operation context로 wrap하되 `errors.Is`를 보존한다. duplicate event와
idempotency-key conflict는 모두 `audit.ErrRevisionConflict`를 보존한다. caller가 구분해야 할 때는
`errors.As`로 `audit.ValidationError.Field`를 검사할 수 있다. 반환된 order, preview slice,
repository history는 복사된다. caller는 stored state를 mutate할 수 없다.

repository call 동안 lock을 유지하는 것은 이 예제가 bounded in-memory repository를 사용하고 network
또는 database I/O가 없기 때문에만 허용된다. 이 demo-scale service는 의도적으로 unrelated order까지
직렬화하며 throughput 또는 horizontal-scaling claim을 하지 않는다. README는 durable application이
process mutex를 I/O 동안 잡는 대신 caller-owned SQL transaction/outbox boundary를 사용해야 하며,
이 service-wide lock 대신 per-aggregate 또는 database-owned concurrency를 사용해야 한다고 설명한다.

## Query and Preview Contract

`Service.History`는 한 aggregate에 대해 `LoadHistory`에 위임한다. 이는 aggregate-demo-only full
read이지 production pagination contract가 아니다. `Service.Find`는 `audit.Query`를 받고 inclusive
revision/time filter와 newest-first ordering을 보존하며, aggregate type `order`를 강제하거나
검증하고, 생략된 limit을 20으로 대체하며, 100을 넘는 limit을 거부한다. 이를 통해 shared repository가
workshop service를 cross-domain query surface로 바꾸지 못하게 한다. 두 method 모두 context를
확인한다. 없는 aggregate는 `History`에서 `(audit.History{}, false, nil)`을 반환한다. `Find`는 일치
entry가 없으면 nil이 아닌 빈 `[]audit.Entry`를 반환한다. README는 production history API가 unbounded
full copy가 아니라 storage-backed pagination과 retention을 필요로 한다고 경고한다.

`BuildPreview`는 하나의 order를 만들고 고정 command ID와 주입 timestamp로 create, confirm, ship을
실행한다. 반환 형태는 다음과 같다.

```json
{
  "current": {"order_id":"order-1001","status":"shipped","revision":3},
  "history": [],
  "recent_history": []
}
```

실제 history array는 stable event identity, type, revision, occurred-at time, author, status
change를 포함한다. `recent_history`는 revision range와 newest-first limit을 보여준다. `main.go`는
deterministic UTC clock을 사용하고, indentation으로 marshal하며, JSON document만 stdout에 쓴다.

## Failure Modes

1. 유효하지 않거나 중복된 command는 current-state mutation 전에 실패한다. repository가 재사용된
   global event/idempotency identity를 감지하면 command는 repository error를 반환하고 projection을
   변경하지 않는다.
2. append 중 repository failure 또는 cancellation은 history와 current state를 모두 변경하지 않는다.
3. concurrent command는 직렬화된다. 정확히 하나의 valid transition만 이길 수 있다. 이후 command는
   결과 state를 관찰하고 계속 유효하게 진행하거나 invalid transition으로 실패한다. revision gap 또는
   race는 허용하지 않는다.
4. 반환된 order 또는 preview value를 mutate해도 stored state나 이후 query result를 변경할 수 없다.

## Security and Operations Boundaries

예제에는 HTTP server, authentication, secret, external process, durable store가 없다.
cancellation reason은 256 UTF-8 rune으로 제한되며 demonstration metadata이지 PII 또는 credential을
담는 곳이 아니다. 실제 application은 event payload와 log를 classify하고 redact해야 한다.

`MemoryRepository`는 process exit 시 모든 data를 잃는다. production owner는 retention, deletion,
archival, access control, schema versioning과 migration, payload-size limit, encryption,
PII/redaction policy를 선택해야 한다. 이 예제는 disaster recovery나 exactly-once delivery를 제공하지
않는다.

## Testing

focused test는 다음을 증명한다.

- create-confirm-ship ordering, revision, event metadata, history, query
- pending 및 confirmed에서의 cancel과 terminal/invalid transition
- missing/duplicate order와 duplicate command/event identity
- projection mutation 없는 repository failure 및 context cancellation
- append 직후 cancellation에도 successful append가 commit point임
- invalid ID와 bounded cancellation reason
- defensive copy, absent-history `false`, nil이 아닌 empty query result
- deterministic preview JSON
- sleep 없는 `go test -race` 아래의 bounded concurrent reuse

validation order는 focused package test, focused race test, runnable preview,
`git diff --check`, repository-wide `make ci`다.

## Compatibility, Migration, and Rollback

예제는 새 workshop file만 추가하고 기존 v0.18.0 module dependency를 사용한다. public API를 변경하지
않는다. rollback은 example, root navigation link, documentation artifact를 삭제하는 것이다. 미래의
durable implementation은 `MemoryRepository` data를 migratable production state로 취급하면 안 된다.
후속 issue에서 명시적인 persistence와 outbox design을 도입해야 한다.

## Acceptance Criteria and DoD

- application-shaped service는 정의된 모든 transition과 append-before-mutation consistency contract를
  구현한다.
- preview는 current state, full immutable history, filtered query를 deterministic하게 대비한다.
- focused success, failure, edge, cancellation, defensive-copy, concurrent reuse test가 race
  detection을 포함해 통과한다.
- English 및 Korean README file은 lesson, run command, expected behavior,
  audit-versus-event-sourcing boundary, operational gap을 설명한다.
- root English 및 Korean navigation은 runnable example을 link한다.
- Type A spec, plan, verifier, review, lesson, PR, CI, merge, sync, cleanup gate가
  P0=0/P1=0으로 완료된다.

## Specification Review Record

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0, P2=2 | demo-scale global serialization과 bounded `Find`를 문서화했다. full history는 명시적으로 demo-only다. |
| Stability | P0=0, P1=0 | commit-point cancellation, failure atomicity, bounded race proof를 명시했다. |
| Security | P0=0, P1=0 | ID, author, reason, query domain, metadata, UTF-8, PII boundary를 제한했다. |
| Operator/Ops | P0=0, P1=0 | durability, retention, migration, rollback, recovery, diagnostics boundary를 명시했다. |
| Developer/API | P0=0, P1=0 after repair | exact API, zero-value/config/error semantic, duplicate retry behavior, absent-history contract를 추가했다. |
| User/caller | P0=0, P1=0 | preview, README lesson, unsupported behavior, production misuse warning을 명시했다. |
| Main integration | P0=0, P1=0 | unresolved contradiction, scope expansion, dependency, repository hazard가 남지 않았다. |

stability, security, operator, user lane은 bounded wait 이후 timeout되었다. 필요한 fallback
review는 main session에서 독립적으로 완료했다.
