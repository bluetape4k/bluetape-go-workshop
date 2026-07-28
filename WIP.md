# WIP

스냅샷: 2026-06-04 KST
범위: `debop`에게 할당된 열린 GitHub issue.
열린 항목 수: 7개 issue.

## 현재 Milestone

`bluetape-go` package를 실제 웹 애플리케이션 형태로 보여 주는 얇은 workshop
저장소를 bootstrap한다.

## 현재 범위

- 예제는 작고 실행 가능한 상태로 유지한다.
- library logic을 중복 구현하지 않고 `bluetape-go` package를 직접 사용한다.
- 예제에 routing 또는 middleware가 필요할 때는 기본 lightweight web framework로
  `chi`를 사용한다.
- local CI, GitHub CI, Nightly에서 Testcontainers-backed 테스트를 실행한다.
- workshop milestone을 닫기 전에 `0.1.0` foundation 예제와 `0.2.0` leader
  group 예제를 완료한다.

## 다음 예제

- cache coordination package가 준비된 뒤 near-cache 예제를 추가한다.
- API가 안정화되면 state, workflow, batch 예제를 추가한다.

## 결정 기록

- application 예제는 `bluetape-go`가 아니라 `bluetape-go-workshop`에 둔다.
- workshop이 library보다 앞서 나가지 않도록 Redis leader web 예제 하나에서 시작한다.
- full-stack framework abstraction보다 lightweight `chi` 예제를 우선한다.
- Go test cache가 Testcontainers 실행을 숨기지 못하도록 test command에 `-count=1`을 사용한다.
- Go feature 예제에서 concurrency, goroutine, async, cancellation, shared-state
  동작을 다룰 때는 `GoroutineStressTester`와 `AsyncJobTester`를 사용한 stress
  validation을 포함한다.
- 예제는 scenario-first로 유지한다. 각 예제는 모든 helper function을 나열하기보다
  하나의 business-shaped 문제를 해결해야 한다.
