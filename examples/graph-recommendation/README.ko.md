# Graph Recommendation

[English](README.md) | [한국어](README.ko.md)

이 `v0.10.0` 워크숍 예제는 issue #51의 범위를
호출자가 소유한 [`graph`](https://github.com/bluetape4k/bluetape-go/tree/develop/graph)
모델로 구현합니다. 순회, 점수 계산, 검증, 출력 변환은 애플리케이션
패키지에 두며 backend, query DSL, 추천 framework는 추가하지 않습니다.

## 예제의 경계

작은 graph에서 설명 가능한 상품·follow 후보를 결정론적으로 만드는
방법을 다룹니다.

- `User`와 `Product` 정점은 `fixture_id`와 검증된 표시 속성을 가집니다.
- `PURCHASED` 엣지는 user에서 product로 향하며 1~5 범위의 정수 `rating`을
  가집니다.
- `FOLLOWS` 엣지는 한 user에서 다른 user로 향합니다.
- 각 증거 순회는 `graph.Path`에 저장합니다. library 모델은 값과 step 형태를
  검증하고 backend query의 순회 의미는 보장하지 않으므로 endpoint 의미는
  애플리케이션이 소유합니다.

## 순회와 점수

### 상품 추천

`alice`를 seed로 다음 순회를 제한된 범위에서 실행합니다.

```text
seed user → PURCHASED 상품 → incoming PURCHASED 공동구매자
          → 공동구매자의 PURCHASED 상품
```

점수는 후보를 연결한 고유 공동구매자 수입니다. seed가 이미 구매한
상품은 제외합니다. 결과는 `score` 내림차순, `product_id` 오름차순으로
정렬합니다. 공동구매자와 후보 상품 조합마다 공유 상품 하나를 같은 규칙으로
선택해 JSON 경로를 재현할 수 있게 합니다.
공동구매자의 구매 엣지는 저장된 `User → Product` 방향을 유지하되, 증거
순회에서는 incoming 관계가 드러나도록 표시합니다.

### Follow 추천

FOAF 순회는 outgoing `FOLLOWS` 두 홉입니다.

```text
seed user → 직접 follow → 두 번째 홉 후보
```

점수는 후보에 도달하게 만든 고유 직접 follow 수입니다. seed와 이미
follow한 user는 제외합니다. 상품 추천과 마찬가지로 점수 내림차순, 식별자
오름차순을 사용합니다.

## 결정론적 fixture

`graph-recommendation-v1` namespace의 fixture에는 user 6명, product 6개, `PURCHASED` 13개, `FOLLOWS` 12개가
있습니다. `alice`로 CLI를 실행하면 상품 후보 3개와 follow 후보 2개를
반환합니다.

```json
{
  "seed_user": "alice",
  "product_recommendations": [
    {
      "product_id": "headphones",
      "score": 3,
      "evidence": [
        {
          "co_buyer_id": "bob",
          "shared_product_id": "laptop",
          "path": [
            "alice",
            "laptop",
            "bob",
            "headphones"
          ]
        },
        {
          "co_buyer_id": "carol",
          "shared_product_id": "phone",
          "path": [
            "alice",
            "phone",
            "carol",
            "headphones"
          ]
        },
        {
          "co_buyer_id": "dave",
          "shared_product_id": "tablet",
          "path": [
            "alice",
            "tablet",
            "dave",
            "headphones"
          ]
        }
      ]
    },
    {
      "product_id": "keyboard",
      "score": 1,
      "evidence": [
        {
          "co_buyer_id": "eve",
          "shared_product_id": "laptop",
          "path": [
            "alice",
            "laptop",
            "eve",
            "keyboard"
          ]
        }
      ]
    },
    {
      "product_id": "mouse",
      "score": 1,
      "evidence": [
        {
          "co_buyer_id": "frank",
          "shared_product_id": "phone",
          "path": [
            "alice",
            "phone",
            "frank",
            "mouse"
          ]
        }
      ]
    }
  ],
  "follow_recommendations": [
    {
      "user_id": "dave",
      "score": 1,
      "evidence": [
        {
          "via_user_id": "bob",
          "path": [
            "alice",
            "bob",
            "dave"
          ]
        }
      ]
    },
    {
      "user_id": "eve",
      "score": 1,
      "evidence": [
        {
          "via_user_id": "carol",
          "path": [
            "alice",
            "carol",
            "eve"
          ]
        }
      ]
    }
  ]
}
```

## 실행

로컬 report를 출력합니다.

```bash
go run ./examples/graph-recommendation
```

검증, 빈 후보, 동률 정렬, 입력 순서 독립성, 경로 증거, 취소를 포함한
집중 테스트를 실행합니다.

```bash
go test -count=1 ./examples/graph-recommendation/...
go test -race -count=1 ./examples/graph-recommendation/...
```

## 범위와 운영 경계

이 예제는 결정론적인 학습용 fixture이며 운영용 추천기가 아닙니다. 추천
관련성, 개인화 품질, 개인정보 정책 준수, 온라인 serving 성능을 보장하지
않습니다. 실제 서비스에서는 identity, 동의, 최신 여부, ranking 정책,
pagination, 영속화, backend별 query 계획을 이 예제 바깥에서 소유해야
합니다.
