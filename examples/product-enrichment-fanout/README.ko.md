# product-enrichment-fanout

[English](README.md) | [한국어](README.ko.md)

`concurrency`와 `testing/concurrency`를 사용하는 product detail enrichment
예제입니다.

이 예제는 하나의 request context 아래에서 pricing, inventory, recommendation,
review-summary lookup을 fan-out합니다. required provider failure는 남은 작업을
취소하고, optional provider failure는 request 전체를 실패시키지 않고 결과에
기록합니다.

## Scenario

![Concurrency and resilience flow](../../docs/images/readme-diagrams/concurrency-resilience-flow.png)

하나의 request가 제한된 concurrency budget 안에서 여러 downstream provider를
호출해야 하는 상황을 보여줍니다. price와 inventory는 required입니다.
recommendations와 review summary는 optional이므로 실패해도 전체 product view를
실패시키지 않고 result metadata로 남깁니다.

## What It Demonstrates

- 고정 worker limit을 가진 `concurrency.Group`.
- required provider failure가 남은 작업을 취소하는 흐름.
- optional provider failure를 result metadata로 보존하는 방식.
- `concurrency.Go`를 통한 panic capture.
- `testing/concurrency` 기반 stress validation.

## Run

```bash
go test -count=1 ./examples/product-enrichment-fanout/...
```
