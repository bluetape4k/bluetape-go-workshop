# Issue #56 주문 감사 이력 교훈

## 맥락

첫 0.9.0 트랙 workshop 예제는 이후 issue가 담당하는 HTTP, SQL outbox, Redis Streams 교훈을
흡수하지 않고 변경 불가 주문 감사 이력을 설명해야 했다. bluetape-go v0.18.0은 이미 검증된
audit event와 entry, 메모리 저장소, revision 확인, 중복 감지, query를 제공한다.

## 결정

서비스는 변경 가능한 교육용 projection을 유지하고 모든 state mutation 전에 audit entry 하나를
append한다. 메모리 append 동안 service mutex를 잡아 revision selection, append, projection
assignment가 하나의 명확한 임계 구역이 되게 한다. append 성공이 커밋 지점이다. cancellation은
append 전과 append 중에만 확인하고, 성공한 append와 실패하지 않는 map assignment 사이에서는
확인하지 않는다.

안정적인 caller-owned command ID는 event ID와 idempotency key가 모두 된다. 중복된 local retry는
repository에서 발생한 conflict와 맞게 `errors.Is(err, audit.ErrRevisionConflict)` 및
`errors.As`와 호환되는 `audit.ValidationError`를 반환한다.

## 의외였던 점과 리뷰 누락

첫 command 구현은 서비스가 구성되었는지 확인하기 전에 input을 validate했다. 따라서 zero-value
service는 malformed command에 대해 승인된 `ErrInvalidConfig` 대신 `ErrInvalidCommand`를
반환했다. PR 전 리뷰가 ordering mismatch를 찾았고, RED 테스트가 zero value와 nil receiver
계약을 고쳤다.

첫 concurrency command는 의도한 테스트 두 개 중 하나만 일치했고 unrelated-order case는 모든
projection/history를 확인하지 않고 error만 assert했다. 두 테스트는 이제 `^TestServiceConcurrent`를
공유하고 정확한 16-goroutine outcome을 assert하며 race detection 전에 20회 실행한다.

첫 `make ci` 실행은 package/export doc comment 22개 누락으로 `revive`에서 실패했다. 이
repository는 surrounding block뿐 아니라 모든 exported error와 status constant에 comment를
요구한다. 정확한 English API comment를 추가해 결과를 `0 issues`로 줄인 뒤 전체 게이트를 다시
실행했다.

## 결과와 검증 근거

- lifecycle, invalid transition, duplicate, repository failure, cancellation,
  defensive copy, bounded query, concurrent reuse 테스트가 통과한다.
- 정확한 CLI JSON은 golden file과 writer-error 테스트로 보호된다.
- 영어/한국어 README는 같은 command, 동작, production limit를 설명한다.
- 집중 테스트, 20회 stress repetition, 집중 race 테스트, `go run`, diff check,
  repository-wide `make ci`가 검증 연쇄를 제공한다.

## 향후 가드레일

프로세스 전역 lock을 durable adapter로 복사하지 않는다. database-backed order state와 audit
delivery에는 caller-owned SQL transaction/outbox boundary, storage pagination, retention,
migration, access-control, redaction policy가 필요하다. query limit는 returned-cardinality
bound로만 취급한다. v0.18.0 memory repository는 여전히 O(total stored entries)를 scan/copy한다.
