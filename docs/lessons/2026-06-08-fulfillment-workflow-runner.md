# Fulfillment Workflow Runner 예제

## 맥락

issue #39는 첫 v0.4.0 workflow runner HTTP 예제를 추가했다. 이 예제는 Gin을 사용하고, `bluetape-go/workflow`를 exercise하고, sequential, parallel, conditional runner를 다루며, scenario, architecture, sequence diagram이 포함된 다국어 README content를 제공해야 했다.

## 결정

workflow를 request-scoped로 유지한다. handler는 request마다 runner 하나를 만들고, HTTP request context를 runner에 전달하며, runtime timestamp 없이 `workreport.Report`를 deterministic JSON으로 project한다. 이 예제에서는 durable workflow engine behavior, retry scheduling, database, queue, mutable workflow context map을 피한다.

## 결과

- 실행 가능한 Gin main package가 있는 `examples/fulfillment-workflow-runner`를 추가했다.
- success, conditional skip, inventory failure, sibling cancellation을 동반한 payment failure, caller cancellation, bad request에 대한 focused test를 추가했다.
- workflow boundary, failure semantics, cancellation, production hardening gap을 설명하는 다국어 README content를 추가했다.
- scenario, architecture, sequence flow를 위한 generated README diagram을 PNG/SVG와 DOT, Plain, Graphviz evidence로 추가했다.

## 검증

- `bash scripts/generate-fulfillment-workflow-diagrams.sh`
- `go test -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -run '^$' ./examples/fulfillment-workflow-runner`
- `git diff --check`
- `make ci`
- scenario, architecture, sequence diagram의 rendered PNG를 개별 검사했다.

## 이후 Guard

request-scoped workflow 예제에서는 cancellation ownership을 테스트에서 보이게 유지한다. failure가 slow sibling을 cancel해야 한다면, 테스트는 fast branch가 실패하기 전에 sibling이 cancellable work에 들어갔음을 증명해야 한다. README diagram은 generator 변경 뒤 rendered PNG를 검사한다. 첫 draft에는 rendered image에서만 드러난 scenario title-gap issue와 sequence failure-label overlap이 있었다.
