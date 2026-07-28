# 교훈: Compensation workflow 예제에는 original-error와 cleanup evidence가 필요하다

## 맥락

issue #71은 뒤쪽 fulfillment step이 실패한 뒤 compensation을 보여 주는 request-scoped `workflow` + `workreport` 예제를 추가한다.

## 배운 점

- Compensation 예제는 forward failure와 cleanup failure를 분리해야 한다. response에는 `original_error` field와 nested report tree가 필요하다. 그래야 caller가 workflow가 왜 실패했는지와 어떤 cleanup step이 실행됐는지 볼 수 있다.
- Reverse cleanup order는 계약의 일부다. 테스트는 최종 HTTP status만이 아니라 정확한 compensation child order를 assert해야 한다.
- request-scoped 예제는 in-memory side-effect flag를 사용할 수 있지만, README는 production compensation에 durable storage, idempotent external command, retry policy, outbox/workflow-engine support가 필요하다고 명시해야 한다.
- README diagram에서는 semantic line color를 가진 Graphviz final render가 rigid grid layout보다 이 flow에 더 잘 맞았다. forward, failure, reverse path를 구분하기 쉽기 때문이다.

## 재사용

rollback-style 또는 saga-like 동작을 모델링하는 future workflow 예제에도 같은 check를 적용한다. original error를 보존하고, cleanup failure를 보고하고, cleanup order를 assert하고, durability boundary를 문서화한다.
