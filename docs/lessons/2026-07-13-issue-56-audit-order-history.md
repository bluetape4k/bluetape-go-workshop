# Issue #56 Audit Order History Lesson

## 맥락

첫 0.9.0-track workshop example은 이후 issue가 담당하는 HTTP, SQL outbox, Redis Streams
lesson을 흡수하지 않고 immutable order audit history를 설명해야 했다. bluetape-go v0.18.0은
이미 validated audit event, entry, in-memory storage, revision check, duplicate detection,
query를 제공한다.

## 결정

service는 mutable teaching projection을 유지하고 모든 state mutation 전에 audit entry 하나를
append한다. in-memory append 동안 service mutex를 잡아 revision selection, append, projection
assignment가 하나의 보이는 critical section이 되게 한다. append success가 commit point다.
cancellation은 append 전과 append 중에만 확인하고, successful append와 실패하지 않는 map
assignment 사이에서는 확인하지 않는다.

stable caller-owned command ID는 event ID와 idempotency key가 모두 된다. duplicate local retry는
repository-originated conflict와 맞게 `errors.Is(err, audit.ErrRevisionConflict)` 및
`errors.As`와 호환되는 `audit.ValidationError`를 반환한다.

## 의외였던 점과 Review 누락

첫 command implementation은 service configured 여부를 확인하기 전에 input을 validate했다.
따라서 zero-value service는 malformed command에 대해 승인된 `ErrInvalidConfig` 대신
`ErrInvalidCommand`를 반환했다. pre-PR review가 ordering mismatch를 찾았고, RED test가
zero value와 nil receiver contract를 고쳤다.

첫 concurrency command는 의도한 test 두 개 중 하나만 match했고 unrelated-order case는 모든
projection/history를 확인하지 않고 error만 assert했다. 두 test는 이제 `^TestServiceConcurrent`를
공유하고 정확한 16-goroutine outcome을 assert하며 race detection 전에 20회 실행한다.

첫 `make ci` run은 package/export doc comment 22개 누락으로 `revive`에서 실패했다. 이
repository는 surrounding block뿐 아니라 모든 exported error와 status constant에 comment를
요구한다. precise English API comment를 추가해 결과를 `0 issues`로 줄인 뒤 full gate를 다시
실행했다.

## 결과와 Proof

- lifecycle, invalid transition, duplicate, repository failure, cancellation,
  defensive copy, bounded query, concurrent reuse test가 통과한다.
- exact CLI JSON은 golden file과 writer-error test로 보호된다.
- 영어/한국어 README는 같은 command, behavior, production limit를 설명한다.
- focused test, 20회 stress repetition, focused race test, `go run`, diff check,
  repository-wide `make ci`가 validation chain을 제공한다.

## 향후 Guard

process-wide lock을 durable adapter로 복사하지 않는다. database-backed order state와 audit
delivery에는 caller-owned SQL transaction/outbox boundary, storage pagination, retention,
migration, access-control, redaction policy가 필요하다. query limit는 returned-cardinality
bound로만 취급한다. v0.18.0 memory repository는 여전히 O(total stored entries)를 scan/copy한다.
