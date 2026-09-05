# Graph Import/Export

[English](README.md) | [한국어](README.ko.md)

이 `v0.10.0` 워크숍 예제는 issue #52의 범위에 맞춰 named partner를 위한 작은
risk-analysis import job을 구현합니다. 릴리스된 `graph/graphio` record 경계를
사용하며, 기본 interchange는 NDJSON이고 명시적으로 선택한 GraphML subset은
optional bounded `graph/graphio/graphml` package로 처리합니다. 애플리케이션은
domain graph를 검증하고 scalar 값과 엣지 방향을 보존한 뒤 결정론적인 summary와
정규화된 그래프 기준 데이터를 출력합니다.

## 예제의 핵심

예제는 하나의 좁은 애플리케이션 경계를 유지합니다.

- `Account`와 `Device` 정점은 `partner` property를 가집니다. Account에는 정수
  risk score와 boolean 상태가, Device에는 kind와 trust flag가 들어갈 수 있습니다.
- `USES_DEVICE` 엣지는 `Account`에서 `Device`로 향하는 directed 관계이며
  floating-point confidence 값을 가집니다.
- 정점과 엣지를 합친 record ID는 고유해야 합니다. export와 기준 데이터를 만들 때
  정점과 엣지를 ID 오름차순으로 정규화합니다.
- 안전한 범위의 JSON 정수는 `int64`로 정규화하고 GraphML typed scalar는
  `bool`, `int64`, `float64`, `string`으로 복원합니다. GraphML에서 선언한
  `double`은 값이 정수여도 `float64`로 유지합니다. 배열, map, null,
  non-finite number는 거부합니다.

고정된 `risk-partner-acme-v1` fixture에는 정점 4개와 directed 엣지 3개가
있습니다. `RunDemo`는 두 format으로 export/import한 뒤 정규화된 그래프 기준
데이터가 동일한지 비교합니다.

## 상한과 fail-closed 동작

`DefaultOptions`는 untrusted input을 다음과 같이 제한합니다.

| 경계 | 기본값 |
| --- | ---: |
| 전체 입력 | `64 KiB` |
| NDJSON line | `16 KiB` |
| NDJSON record | `16 KiB` |
| 전체 record 수 | `64` |

job은 중복 ID, missing endpoint, partner property 불일치, 지원하지 않는 label이나
엣지 방향, scalar가 아닌 property, unknown GraphML key, XML directive/extension,
nested graph, oversized input을 fail-closed로 거부합니다. GraphML reader는 bounded
whole-document parser입니다. 이미 block된 reader를 취소로 깨워야 한다면 reader의
소유자가 close하거나 deadline을 제공해야 합니다.

## 실행

결정론적인 format 비교 report를 출력합니다.

```bash
go run ./examples/graph-import-export
```

report에는 `partner`, `equivalent`, `ndjson`와 `graphml` 각각의 run, format별
count, ID 순서로 정렬된 정규화된 그래프 기준 데이터가 들어갑니다. 안정적인 summary는
다음과 같이 시작합니다.

```json
{
  "partner": "acme-payments",
  "equivalent": true,
  "runs": [
    {
      "format": "ndjson",
      "summary": {
        "vertices": 4,
        "edges": 3,
        "accounts": 2,
        "devices": 2,
        "uses_device_edges": 3,
        "directed": true
      }
    },
    {
      "format": "graphml",
      "summary": {
        "vertices": 4,
        "edges": 3,
        "accounts": 2,
        "devices": 2,
        "uses_device_edges": 3,
        "directed": true
      }
    }
  ]
}
```

두 run은 정규화된 그래프 기준 데이터에서도 같은 순서와 directed endpoint를 유지합니다
(일부 발췌).

```json
{
  "vertices": [
    {"id": "account-a", "label": "Account"},
    {"id": "account-b", "label": "Account"},
    {"id": "device-01", "label": "Device"},
    {"id": "device-02", "label": "Device"}
  ],
  "edges": [
    {"id": "use-001", "from": "account-a", "to": "device-01"}
  ]
}
```

집중 테스트와 race test를 실행합니다.

```bash
go test -count=1 ./examples/graph-import-export/...
go test -race -count=1 ./examples/graph-import-export/...
```

## 지원 경계와 운영 범위

이 lesson은 NDJSON round-trip, bounded GraphML round-trip, 결정론적 ordering,
scalar 정규화, 확인 가능한 failure class를 보여줍니다. Paired CSV는
`graph/graphio`에서 계속 사용할 수 있지만 이 CLI는 기본 NDJSON 경로와 named
partner의 GraphML 경로 비교에만 집중합니다.

Broad GraphML, yEd, yFiles, Gephi, NetworkX, Neo4j APOC 호환성을 약속하지
않습니다. Compression, encryption, filesystem ownership, atomic file replacement,
graph database adapter, schema migration, 운영용 risk decision은 caller가
소유하며 이 워크숍 예제의 범위 밖입니다.
