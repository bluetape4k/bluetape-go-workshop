# Issue 39 Fulfillment Workflow Runner 리서치

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #39 `[v0.4.0] Add fulfillment workflow runner example`
- 마일스톤: `0.4.0`
- 패키지 초점: `github.com/bluetape4k/bluetape-go/workflow`,
  `github.com/bluetape4k/bluetape-go/workreport`

## 현재 저장소 근거

- 루트 README는 framework가 lesson의 일부일 때 public HTTP API 예제가 Gin을
  사용한다고 설명한다.
- `examples/order-lifecycle-state-api`는 현재 0.4.0 선행 예제이며 다음 형태를
  사용한다.
  - `main.go`
  - `README.md`
  - `README.ko.md`
  - `internal/<domain>/server.go`
  - `internal/<domain>/server_test.go`
- 저장소의 현재 의존성은 `github.com/bluetape4k/bluetape-go v0.5.1`이며,
  release된 0.4.0 `workflow`와 `workreport` 패키지를 포함한다.
- 설계 작업 전에 feature worktree에서 CodeGraph를 초기화했으며, 35 files,
  484 nodes, 1,081 edges로 보고되었다.

## bluetape-go 패키지 근거

`github.com/bluetape4k/bluetape-go@v0.5.1`의 local module cache source는 다음을
보여 준다.

- `workflow.Sequential(name, policy, works...)`는 입력 순서대로 work를 실행한다.
  `StopOnFailure`는 첫 failed 또는 partial child에서 중단한다. cancelled 또는
  aborted child report는 항상 sequence를 중단한다.
- `workflow.Parallel(name, policy, works...)`는 공유 cancellable context로 모든
  work item을 시작하고 child report를 입력 순서대로 보존한다. `StopOnFailure`,
  aborted, cancelled child report는 sibling을 cancel하고 시작된 goroutine을 기다린다.
- `workflow.Conditional(name, predicate, trueWork, falseWork...)`는 하나의
  predicate를 평가하고 선택된 branch 하나만 실행한다.
- work function은 `context.Context`를 받고 `workreport.Report`를 반환한다.
- `workreport.Report`는 다음 terminal status를 노출한다.
  `completed`, `failed`, `partial`, `aborted`, `cancelled`.
- `workreport.StopOnFailure`와 `workreport.ContinueOnFailure`는 aggregation을
  위한 failure policy model을 제공한다.

GNO retrieval은 `bluetape-go` PR #142, `feat: add workflow runners`를 패키지
동작의 source PR로 찾았다. 이 워크숍 예제에서는 local dependency source를 구현
권위로 사용한다.

## 이슈 요구사항

#39는 다음을 요구한다.

- sequential, parallel, conditional step을 보여 주는 workflow runner 예제.
- fulfillment 모델:
  - validate
  - reserve inventory와 authorize payment를 parallel로 실행
  - conditional shipment creation
- 테스트에서 cancellation과 failure propagation을 볼 수 있어야 한다.
- success, step failure, conditional skip, cancellation 테스트.
- workflow step boundary와 failure semantic을 설명하는 README 문서.
- 0.4.0 아래 루트 README navigation.
- 얇은 API snippet이 아니라 scenario-shaped example.
- public HTTP API 예제에는 Gin 사용.
- English와 Korean README 파일 동기화.

## 채택 방향

요청마다 하나의 in-memory fulfillment workflow를 실행하는 Gin API인
`examples/fulfillment-workflow-runner`를 추가한다. request body는 다음 scenario
input을 선택한다.

- 주문 식별자
- 재고 보유 여부
- 결제 승인 결과
- 배송 필요 여부

서버는 `workreport.Report` tree의 안정적인 JSON view를 반환한다. 이 예제는
sequential, parallel, conditional workflow runner가 cancellation과 failure semantic을
보존하면서 일반 Go function을 어떻게 조합하는지 독자에게 보여 주어야 한다.

## 거부한 방향

- durable workflow engine simulation: `workflow`가 명시적으로 lightweight하고
  non-durable하므로 거부한다.
- mutable shared workflow context map: `workflow`가 대신 일반 closure와 명시적
  input을 문서화하므로 거부한다.
- Testcontainers 또는 외부 서비스: 이 이슈는 infrastructure integration이 아니라
  runner semantic에 집중하므로 거부한다.
- HTTP 없는 순수 domain-only 예제: 이슈와 워크숍 방향이 public API 예제에 Gin을
  요구하므로 거부한다.

## 검증 대상

- `go test -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
- `bash scripts/generate-fulfillment-workflow-diagrams.sh`
- `git diff --check`
- `make ci`
