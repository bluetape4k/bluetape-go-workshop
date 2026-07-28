# Foundation과 Leader Group Workshop 예제

## 맥락

resilience HTTP 예제가 들어간 뒤에도 `bluetape-go-workshop`에는 열린 `0.1.0` foundation 예제 issue와 `0.2.0` `LeaderGroupElector` 예제 issue가 남아 있었다.

## 결정

API catalog page가 아니라 scenario-first 예제를 추가한다. 각 package는 얇게 유지하고, 일반 `go test`, Testcontainers integration test, concurrency 동작이 중요한 경우의 stress helper로 snippet을 증명한다.

## 결과

workshop은 이제 cache snapshot codec, order feed cleanup, invitation codec, Redis leader job, product enrichment fan-out, Testcontainers-backed order pipeline integration, Redis leader group web coordination을 다룬다.

## 검증

- `go test -count=1 ./examples/cache-snapshot-codecs/... ./examples/order-intake-cleanup/... ./examples/invitation-codecs/... ./examples/product-enrichment-fanout/...`
- `go test -count=1 ./examples/leader-coordination-jobs/... ./examples/order-pipeline-testcontainers/... ./examples/leader-group-web/...`
- `go test -count=1 ./...`
- `make ci`
- `git diff --check`

## 이후 Guard

goroutine, cancellation, async, shared-state, coordination 동작을 다루는 Go feature 예제는 PR을 열기 전에 `GoroutineStressTester` 또는 `AsyncJobTester` coverage를 포함한다.
