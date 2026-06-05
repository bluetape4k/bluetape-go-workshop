# order-intake-cleanup

[English](README.md) | [한국어](README.ko.md)

`core`와 `collections`를 사용해 partner order feed를 정리하는 예제입니다.

이 예제는 필수 필드를 검증하고, 선택적인 note/coupon 값을 기본값으로 채우고,
나쁜 line item을 걸러내고, tag를 deduplicate한 뒤, accepted order를 sales
channel별로 그룹화합니다. helper 호출보다 명확한 곳에서는 의도적으로 평범한
`for` loop를 유지합니다.

## Scenario

![Data codec and cleanup flow](../../docs/images/readme-diagrams/data-codec-cleanup-flow.png)

partner feed가 domain pipeline에 들어가기 전에 결정적인 cleanup이 필요한 상황을
보여줍니다. validation, default, filtering, deduplication, grouping 단계를
하나의 helper에 숨기지 않고 명시적으로 드러냅니다.

## What It Demonstrates

- `core` helper를 이용한 required-field validation.
- optional notes/coupons defaulting.
- grouping 전에 unusable line item filtering.
- 안정적인 출력을 유지하는 tag deduplication.
- accepted order를 sales channel별로 grouping.

## Run

```bash
go test -count=1 ./examples/order-intake-cleanup/...
```
