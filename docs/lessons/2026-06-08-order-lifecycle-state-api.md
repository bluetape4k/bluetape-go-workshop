# Order Lifecycle State API 예제

## 맥락

issue #38은 첫 v0.4.0 state-machine HTTP 예제를 추가했다. 이 예제는 Gin을 사용하고, `bluetape-go/state`를 exercise하고, scenario, architecture, sequence diagram을 포함한 다국어 README content를 제공해야 했다.

## 결정

service는 의도적으로 in-memory로 유지하고 작은 Gin router를 통해 order 하나를 노출한다. `state.Machine`이 transition legality, final-state rejection, guard rejection, concurrent transition conflict를 소유하게 한다. handler는 JSON binding, event parsing, HTTP error mapping으로 제한한다.

## 결과

- 실행 가능한 main package가 있는 `examples/order-lifecycle-state-api`를 추가했다.
- allowed transition, invalid transition, guard rejection, final-state rejection, bad request, concurrent duplicate transition safety에 대한 focused test를 추가했다.
- finite-state-machine과 workflow runner의 차이를 설명하는 README와 README.ko content를 추가했다.
- scenario, architecture, sequence flow를 위한 generated README diagram을 PNG/SVG와 DOT, Plain, Graphviz evidence로 추가했다.

## 검증

- `bash scripts/generate-order-lifecycle-diagrams.sh`
- `go test -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -race -count=1 ./examples/order-lifecycle-state-api/...`
- `go test -count=1 ./...`
- `make ci`
- scenario, architecture, sequence diagram의 rendered PNG를 개별 검사했다.

## 이후 Guard

`go mod tidy`에 의존하기 전에 framework import를 추가한다. 그렇지 않으면 tidy가 dependency를 제거하고 direct dependency check가 misleading해진다. README diagram은 checked-in script에서 asset을 regenerate하고, 양쪽 locale에 PNG embed를 유지하며, visual route 변경 뒤 rendered PNG를 검사한다.
