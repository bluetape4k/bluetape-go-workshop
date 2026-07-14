# Neo4j Graph Abuse Cluster

[English](README.md) | 한국어

이 CLI 예제는 합성 사용자와 식별자 관계를 Neo4j에 저장한 뒤, `bluetape-go`의
`graph` 값과 `graph/neo4j` adapter로 다시 읽어 Go에서 연결 요소를 계산합니다.
그래프 DB가 cluster 점수를 계산하는 예제가 아닙니다. Neo4j는 제한된 그래프를
저장하고 읽는 경계만 맡고, schema 검증, cluster 구성, 점수 계산, 정렬은 예제
코드가 담당합니다.

![Graph abuse cluster architecture](../../docs/images/readme-diagrams/graph-abuse-cluster-architecture.png)

## 패키지에서 배울 내용

| 구성 요소 | 책임 |
| --- | --- |
| Fixture builder | 합성 opaque ID로 12개 vertex와 10개 edge를 만들고 저장 전에 schema, endpoint, 중복을 검증합니다. |
| `abusecluster.Store` | 고정된 parameterized Cypher 하나로 fixture namespace를 reset/seed합니다. 초과를 탐지하려고 limit+1까지 조회하며 vertex 256개와 edge 1024개까지만 허용합니다. |
| `graph/neo4j` | Caller-owned Neo4j driver를 사용해 managed write와 `graph.Vertex`/`graph.Edge` 변환을 수행합니다. |
| Go analyzer | User와 Identifier의 연결 요소를 순회해 공유 식별자 근거와 위험 점수를 계산하고 결과를 결정적으로 정렬합니다. |
| CLI | Strict loopback URI 검증, 15초 작업 deadline, 3초 cleanup, JSON buffering, cleanup 이후 stdout 출력을 맡습니다. |

![Graph abuse cluster sequence](../../docs/images/readme-diagrams/graph-abuse-cluster-sequence.png)

## 그래프 schema

| Graph value | Label/type | Property | 관계 |
| --- | --- | --- | --- |
| 사용자 vertex | `User` | `opaque_id`, `fixture_id` | `USES_IDENTIFIER`의 시작점 |
| 식별자 vertex | `Identifier` | `opaque_id`, `fixture_id`, `kind` | `USES_IDENTIFIER`의 끝점 |
| 관계 edge | `USES_IDENTIFIER` | `opaque_id`, `fixture_id` | `User -> Identifier` 방향만 허용 |

`kind`는 `device`, `ip`, `payment_token` 중 하나입니다. 모든 ID는 예제를 위해
만든 합성 opaque ID이며 실제 사용자 정보나 원본 식별자를 담지 않습니다. 저장
전 fixture와 저장 후 graph를 모두 검증합니다. Vertex가 256개, edge가 1024개를
넘으면 전체 분석을 실패시키며 일부 결과를 출력하지 않습니다.

## Cluster와 점수

사용자가 같은 식별자를 공유하면 같은 연결 요소에 속합니다. 직접 식별자를
공유하지 않더라도 다른 사용자를 거쳐 이어지면 같은 cluster입니다. 둘 이상의
사용자가 공유한 식별자만 evidence에 넣고, 식별자마다 아래 weight를 한 번씩
더합니다.

| Identifier kind | Weight |
| --- | ---: |
| `payment_token` | 5 |
| `device` | 3 |
| `ip` | 1 |

이 위험 점수는 graph traversal과 결정적 projection을 설명하기 위한 값일
뿐, 실제 사기 판정 기준이 아닙니다. Cluster는 위험 점수 내림차순, 사용자 수
내림차순, 가장 작은 사용자 ID 오름차순으로 정렬합니다. Evidence는 kind와
opaque ID 순으로 정렬합니다.

## 실행

### Terminal 1: Neo4j 실행

아래 foreground command는 인증을 끄고 Bolt port를 IPv4 loopback에만 공개합니다.
컨테이너를 실행한 terminal은 그대로 둡니다.

```bash
docker run --rm --name graph-abuse-cluster-neo4j \
  -e NEO4J_AUTH=none \
  -p 127.0.0.1:7687:7687 \
  neo4j:5.26.0
```

### Terminal 2: CLI와 test 실행

CLI는 `bolt://` URI만 받고 host가 `localhost`, `127.0.0.0/8`, `::1`인 경우만
허용합니다. `port`는 숫자로 지정해야 합니다. 자격 증명, path, query, fragment,
remote host는 driver를 만들기 전에 거부합니다.

```bash
NEO4J_URI=bolt://127.0.0.1:7687 \
  go run ./examples/graph-abuse-cluster

go test -count=1 ./examples/graph-abuse-cluster/...
go test -p 1 -race -count=1 ./examples/graph-abuse-cluster/...
```

Neo4j Testcontainers integration test는 `neo4j:5.26.0`을 시작하고 실제
connectivity, 두 번의 idempotent replace, 취소, namespace cleanup을 검증합니다.
Docker resource를 공유하므로 race command도 `-p 1`로 직렬 실행합니다.

## 예상 JSON

동일한 fixture는 항상 다음 JSON을 출력합니다.

```json
{
  "clusters": [
    {
      "cluster_id": "cluster:usr-001",
      "users": [
        "usr-001",
        "usr-002",
        "usr-003"
      ],
      "evidence": [
        {
          "kind": "device",
          "opaque_id": "dev-001",
          "user_count": 2,
          "weight": 3
        },
        {
          "kind": "ip",
          "opaque_id": "ip-001",
          "user_count": 2,
          "weight": 1
        }
      ],
      "risk_score": 4
    },
    {
      "cluster_id": "cluster:usr-004",
      "users": [
        "usr-004",
        "usr-005"
      ],
      "evidence": [
        {
          "kind": "device",
          "opaque_id": "dev-002",
          "user_count": 2,
          "weight": 3
        },
        {
          "kind": "ip",
          "opaque_id": "ip-002",
          "user_count": 2,
          "weight": 1
        }
      ],
      "risk_score": 4
    }
  ],
  "isolated_users": [
    "usr-006"
  ]
}
```

## Transaction과 실행 경계

`ReplaceFixture`는 `graph-abuse-cluster-v1` namespace의 기존 node를 지우고 새
fixture를 만드는 고정 Cypher를 하나의 Neo4j managed write transaction에서
실행합니다. Reset과 seed는 함께 commit되거나 함께 rollback됩니다. CLI는 이
고정 namespace를 한 번 처리하고 종료하는 학습용 프로세스입니다. 같은 namespace를
대상으로 여러 CLI를 동시에 실행하는 방식은 지원하지 않습니다.

이 atomic reset/seed는 message delivery나 distributed transaction을 보장하지
않습니다. Outbox, retry, consumer deduplication, at-least-once/exactly-once delivery는
이 예제에 적용되지 않습니다.

`NEO4J_AUTH=none`과 `NoAuth`는 loopback으로 격리한 local workshop에서만 쓰는
신뢰 경계입니다. Production에서 필요한 authentication, TLS, authorization,
secret loading, routing, tenant isolation, retention, audit logging, migration,
backup, high availability, 일반 Neo4j deployment는 구현하지 않습니다. 원본
식별자 수집·hashing, production fraud policy, 자동 제재, 수동 검토 workflow도
범위 밖입니다.

이 예제는 Neo4j-only CLI입니다. Memgraph, HTTP API, reusable graph repository,
algorithm library, schema DSL, query builder, provider abstraction을 추가하지 않습니다.
재사용 기능은 `bluetape-go`의 별도 issue에서 다뤄야 합니다.
