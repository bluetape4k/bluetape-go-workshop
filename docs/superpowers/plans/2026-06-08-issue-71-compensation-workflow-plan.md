# Issue #71 계획: Compensation Workflow 예제

## 작업 유형

Type A - Full Feature.

이유: 새 runnable example directory, Go implementation, test, bilingual README,
generated diagram, root navigation, review artifact, lesson, PR, CI가 필요하다.

## 구현 작업

1. Add `examples/compensation-workflow/main.go` with `HTTP_ADDR` default
   `:8087`.
2. `examples/compensation-workflow/internal/compensation/server.go`를 추가한다.
   - Gin routes: `GET /healthz`, `POST /compensation/fulfillment`.
   - request validation 및 stable error response.
   - Forward `workflow.Sequential` for inventory, payment, shipment.
   - 성공한 reversible step에 대한 reverse-order compensation stack.
   - original forward error를 보존하는 top-level report.
3. `server_test.go`를 추가한다.
   - success
   - shipment failure with reverse compensation order
   - payment failure with inventory release only
   - compensation failure with original error preserved and later compensation
     still executed
   - inventory failure without compensation
   - caller cancellation
   - bad requests
4. English/Korean README를 추가한다.
   - scenario
   - architecture
   - sequence
   - API examples
   - compensation vs state transition guidance
   - production hardening notes
5. `scripts/generate-compensation-workflow-diagrams.sh`를 추가한다.
   - DOT/plain/Graphviz SVG/PNG evidence.
   - final SVG/PNG assets.
   - deterministic gate summary with zero bad geometry/font counts.
6. root README navigation 및 run section을 갱신한다.
7. `compensation-workflow`를 포함하도록 workshop example map을 재생성한다.
8. lesson 및 Step 6-R review/verifier artifact를 추가한다.

## 검증 계획

- `bash scripts/generate-compensation-workflow-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/compensation-workflow/...`
- `go test -race -count=1 ./examples/compensation-workflow/...`
- `go test -run '^$' ./examples/compensation-workflow`
- `git diff --check`
- `code-review-graph build --repo "$PWD"`
- `golangci-lint cache clean && make ci`
- GitHub PR checks

## 위험과 완화

| 위험 | 완화 |
|---|---|
| 예제가 durable saga behavior를 과장 | README와 code comment에서 compensation을 app-layer request-scoped teaching boundary로 명명한다. |
| compensation failure가 original error를 숨김 | response에 `original_error`가 있고 top-level report는 original error를 유지하며 test가 이를 assert한다. |
| compensation order가 불명확 | test가 `release-inventory` 전 `void-payment`를 assert하고 diagram은 reverse order를 보여 준다. |
| diagram layout regression | Graphviz evidence, generated PNG pair, geometry gate output, visual preview를 사용한다. |
