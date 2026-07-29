# Issue #72 계획: Order Fulfillment Workflow Integration 예제

## 작업 유형

Type A - Full Feature.

이유: 새 runnable example directory, Go implementation, test, bilingual README,
generated diagram, root navigation update, review artifact, PR이 필요하다.

## 구현 작업

1. `HTTP_ADDR`
   default `:8088`.
   를 가진 `examples/order-fulfillment-integration/main.go`를 추가한다.
2. `examples/order-fulfillment-integration/internal/orderfulfillment`를 추가한다.
   - Gin routes: `GET /healthz`, `POST /orders/fulfillment`.
   - Request validation for malformed JSON, blank `order_id`, and non-positive
     totals.
   - Request-scoped `state.Machine` for draft/submitted/paid/packed/shipped/
     cancelled lifecycle 전환.
   - `workflow.Sequential` forward runner with `StopOnFailure`.
   - Reverse compensation runner with `ContinueOnFailure`.
   - timestamp field 없는 stable `workreport` projection 및 summary.
3. focused test를 추가한다.
   - health
   - shipped happy path
   - invalid transition inside workflow
   - shipment failure with reverse compensation and cancelled state
   - compensation failure preserving original shipment error
   - caller cancellation before/during workflow
   - invalid requests
   - parallel request independence
4. English/Korean README를 추가한다.
   - scenario
   - how the example integrates smaller 0.4.0 examples
   - API examples
   - Architecture
   - Sequence Diagram
   - production hardening notes
5. `scripts/generate-order-fulfillment-integration-diagrams.sh`를 추가한다.
   - DOT/plain/Graphviz SVG/PNG route evidence.
   - final decorated SVG/PNG assets.
   - geometry gate output with concrete `margins=L/R/T/B` values.
6. root `README.md`와 `README.ko.md`의 example map 및 quickstart entry를 갱신한다.
7. 구현 뒤 Step 6-R review/verifier artifact를 추가한다.

## 검증 계획

- `bash scripts/generate-order-fulfillment-integration-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/order-fulfillment-integration/...`
- `go test -race -count=1 ./examples/order-fulfillment-integration/...`
- `go test -run '^$' ./examples/order-fulfillment-integration`
- `git diff --check`
- `golangci-lint cache clean && make ci`
- GitHub PR checks

## 위험과 완화

| 위험 | 완화 |
|---|---|
| integration example이 작은 concept를 숨김 | README와 code가 state, workflow, workreport, compensation boundary를 명시적으로 유지한다. |
| 예제가 durable workflow behavior를 과장 | README가 request-scoped임을 밝히고 durable storage, outbox, idempotency, retry hardening을 나열한다. |
| compensation failure가 forward error를 숨김 | response에 `original_error`가 있고 test는 compensation이 실패해도 shipment failure로 남는지 assert한다. |
| cancellation이 side effect 뒤 cleanup을 건너뜀 | reversible side effect가 있으면 spec과 test가 `context.WithoutCancel` compensation을 요구한다. |
| diagram drift가 margin/decorator regression 반복 | generator는 raw Graphviz output뿐 아니라 decorated asset을 사용하고 concrete margin evidence를 출력한다. |
