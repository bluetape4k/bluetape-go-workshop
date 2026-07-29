# Lesson: 주문 처리 통합 예제

## 변경

- `examples/order-fulfillment-integration`을 v0.4.0 마일스톤 수준 통합 예제로 추가했다.
- 하나의 Gin API 안에서 `state.Machine`, `workflow.Sequential`,
  `workreport.Report` projection, 애플리케이션 소유 compensation을 조합했다.
- compensation 하위 실패를 보고하면서도 원래 forward 오류를 보존했다.
- 부작용 이전 취소와 되돌릴 수 있는 부작용 등록 이후 취소를 모두 다루는 취소
  coverage를 추가했다.
- 영어/한국어 README와 scenario, architecture, sequence diagram asset을 추가했다.

## 가드레일

- 예제는 request scope로 유지한다. durable workflow engine이나 saga coordinator처럼
  보이게 쓰지 않는다.
- 코드와 문서에서 `state`, `workflow`, `workreport`를 계속 드러낸다. 일반적인
  orchestration wrapper 뒤에 학습 포인트를 숨기지 않는다.
- compensation이 실패해도 `original_error`를 보존한다.
- `context.WithoutCancel`은 되돌릴 수 있는 부작용이 이미 등록된 뒤의 bounded cleanup에만
  사용한다.
- README diagram은 장식된 workshop baseline, PNG embed, SVG sibling, Graphviz
  route evidence, 구체적인 L/R/T/B margin output을 유지한다.

## 검증

- `bash scripts/generate-order-fulfillment-integration-diagrams.sh`
- 다음 asset을 시각적으로 점검했다.
  - `order-fulfillment-integration-scenario.png`
  - `order-fulfillment-integration-architecture.png`
  - `order-fulfillment-integration-sequence.png`
  - `workshop-example-map.png`
- `go test -count=1 ./examples/order-fulfillment-integration/...`
- `go test -race -count=1 ./examples/order-fulfillment-integration/...`
- `go test -run '^$' ./examples/order-fulfillment-integration`
