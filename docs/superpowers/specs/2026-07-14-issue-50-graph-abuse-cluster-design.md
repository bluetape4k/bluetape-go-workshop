# Issue #50 Graph Abuse Cluster 설계

## Status

- Work type: Type A - Full Feature
- Issue: #50 `[v0.10.0] Add graph abuse cluster example`
- Parent track: #36
- Parent roadmap: #27
- Implementation baseline: `bluetape-go v0.18.0`
- Design decision: user가 approach 1을 승인했다. Neo4j-only persistence와 application-owned
  cluster analysis를 사용한다.
- Approval date: 2026-07-14 KST

## Problem and Outcome

workshop에는 첫 graph-domain example이 필요하다. 이 예제는 workshop을 graph framework나 fraud
product로 만들지 않으면서, 구체적인 abuse investigation scenario 안에서 released `graph` value와
좁은 `graph/neo4j` adapter를 보여줘야 한다.

예제는 opaque user와 identifier를 하나의 Neo4j fixture에 seed하고, released adapter로 bounded graph를
읽은 뒤, Go에서 connected abuse cluster를 계산하고 deterministic JSON evidence를 출력한다. reader는 어떤
behavior가 Neo4j에 속하고, 어떤 behavior가 `bluetape-go`에 속하며, 무엇이 application policy로 남는지
볼 수 있어야 한다.

## Evidence and Adopt Decisions

### Adopt

- fixture value에는 `graph.Vertex`, `graph.Edge`, `graph.Properties`와 이들의 validation
  contract를 사용한다.
- caller-owned official Neo4j driver 주위에는 `graph/neo4j.Client`를 사용한다.
- 작은 fixed Cypher boundary에는 `ExecuteWrite`, `ReadVertices`, `ReadEdges`를 사용한다.
- `bluetape-go v0.18.0`이 reusable `testcontainers/neo4j` launcher를 제공하지 않으므로
  `testcontainers-go/modules/neo4j`를 사용하는 upstream `graph/neo4j` Testcontainers pattern을
  채택한다.
- runnable `main`과 `internal` package, bilingual README pair, root navigation, focused test,
  serial container evidence라는 repository convention을 따른다.

### Borrow

- #50이 첫 graph example이며 opaque identifier, deterministic fixture, domain-owned algorithm을
  사용해야 한다는 이전 graph candidate research decision을 빌린다.
- Kotlin abuse-detection scenario는 domain inspiration으로만 사용한다. Kotlin repository, traversal
  abstraction, API shape는 port하지 않는다.

### Reject

- Cypher 안의 full cluster computation. 이는 teaching policy를 backend query에 숨기고 deterministic
  unit testing을 덜 직접적으로 만든다.
- Neo4j를 connectivity smoke test로만 사용하는 in-memory-only graph. 실제 data flow에서 released
  adapter를 증명하지 못한다.
- Memgraph coverage. user는 이 focused example에 Neo4j-only scope를 선택했다. backend compatibility는
  upstream 또는 later work에 속한다.

## Architecture

example은 `examples/graph-abuse-cluster`에 위치하며 네 개의 bounded component를 가진다.

1. **Fixture builder**는 opaque constant에서 검증된 graph vertex와 edge를 만든다. database behavior는
   포함하지 않는다.
2. **Neo4j store**는 검증된 fixture를 parameter map으로 변환하고, 하나의 atomic write 안에서 자기 fixture
   namespace만 reset/seed하며, `graph/neo4j.Client`로 bounded vertex와 edge를 읽는다.
3. **Analyzer**는 in-memory bipartite adjacency map을 만들고, connected user component를 찾고, shared
   evidence를 추출하며, illustrative risk score를 계산하고 report를 deterministic하게 정렬한다.
4. **CLI**는 configuration, driver/client lifecycle, signal 및 timeout context, JSON output, redacted
   terminal error를 소유한다.

어떤 component도 reusable graph repository, algorithm library, schema DSL, query builder, provider
abstraction을 정의하지 않는다. reusable capability change가 발견되면 별도 `bluetape-go` issue에 속한다.

## Graph Schema

### Vertices

| Label | Required properties | Meaning |
|---|---|---|
| `User` | `opaque_id`, `fixture_id` | One investigated account fixture |
| `Identifier` | `opaque_id`, `kind`, `fixture_id` | Opaque device, IP, or payment-token reference |

`kind`는 `device`, `ip`, `payment_token`만 허용한다. 모든 ID는 raw device, address, account,
payment data를 드러내지 않는 fixture value다.

### Edges

`(User)-[:USES_IDENTIFIER]->(Identifier)`는 directed association을 기록한다. fixture builder는
database 작업 전에 모든 graph value를 검증한다. application은 unknown identifier kind, duplicate
opaque vertex ID, duplicate logical edge, endpoint가 없는 edge도 거부한다.

Neo4j `ElementId` value는 adapter identity로 남는다. application은 `graph.ElementID`로 읽은 vertex와
edge를 join하고, opaque property는 domain output과 deterministic ordering에만 사용한다.

## Fixture and Database Boundary

checked-in fixture는 다음을 증명할 만큼 충분한 구조를 가진다.

- 둘 이상의 identifier kind를 통해 transitive하게 연결된 three-user component 하나
- score tie candidate를 가진 two-user component 하나
- unique identifier만 가진 isolated user 하나
- input order와 무관한 deterministic ordering

The exact logical fixture is:

| Identifier | Kind | Linked users |
|---|---|---|
| `dev-001` | `device` | `usr-001`, `usr-002` |
| `ip-001` | `ip` | `usr-002`, `usr-003` |
| `pay-001` | `payment_token` | `usr-003` only; not shared evidence |
| `dev-002` | `device` | `usr-004`, `usr-005` |
| `ip-002` | `ip` | `usr-004`, `usr-005` |
| `ip-003` | `ip` | `usr-006` only; isolated user evidence |

이는 두 개의 score-4 cluster를 만든다. `cluster:usr-001`은 user 세 명을 포함하므로 user 두 명을
포함하는 `cluster:usr-004`보다 먼저 정렬된다. `usr-006`은 유일한 isolated user다. pure unit fixture는
payment-token weight와 최종 smallest-user-ID tie-break를 별도로 증명한다.

CLI는 하나의 constant fixture namespace를 사용한다. 하나의 `ExecuteWrite` statement는 정확히 해당
`fixture_id`를 가진 node만 삭제한 뒤 같은 managed Neo4j transaction 안에서 모든 fixture vertex와 edge를
생성한다. validation 또는 write failure는 reset과 seed를 모두 rollback한다. half-seeded fixture를 남기지
않고 unscoped delete를 실행하지 않는다. test code는 per-test namespace를 사용하고 bounded cleanup을
등록한다.

모든 Cypher label과 relationship type은 fixed source constant다. 모든 fixture value는 query parameter다.
write는 `graph/neo4j.Client.ExecuteWrite`를 통해 발생한다. read는 `ReadVertices`와 `ReadEdges`를 통한
별도 bounded vertex/edge query를 사용한다.

connectivity 이후 normal path에는 정확히 세 database operation이 있다. 하나의 atomic reset/seed, 하나의
vertex read, 하나의 edge read다. per-user 또는 per-identifier query는 수행하지 않는다. focused CLI는
single-run teaching code다. constant fixture namespace를 공유하는 concurrent process는 지원하지 않으며
양쪽 README에 명시한다.

각 read는 `maximum + 1` record를 요청한다. result가 다음 limit을 넘으면 store는 조용히 truncate하지 않고
typed size error를 반환한다.

- maximum vertices: 256
- maximum edges: 1024

예제는 이 bounded fixture를 의도적으로 memory에 collect한다. production-scale traversal 또는 throughput
claim은 하지 않는다.

## Cluster Contract

analyzer는 graph를 bipartite user-to-identifier graph로 다룬다.

1. validated `USES_IDENTIFIER` edge에서만 adjacency를 만든다.
2. identifier를 통해 connected component를 traverse한다.
3. user가 두 명 이상인 component는 abuse cluster다.
4. identifier는 해당 component 안의 user 최소 두 명이 reference할 때만 evidence다.
5. 모든 abuse cluster 밖의 user는 isolated로 report한다.

transitive linkage는 의도된 동작이다. user A가 B와 device를 공유하고 B가 C와 IP를 공유하면, A와 C가
identifier를 직접 공유하지 않아도 A, B, C는 하나의 component를 형성한다.

### Illustrative Score

각 distinct shared identifier는 한 번만 기여한다.

| Identifier kind | Weight |
|---|---:|
| `payment_token` | 5 |
| `device` | 3 |
| `ip` | 1 |

cluster score는 해당 evidence weight의 합이다. 이는 투명한 workshop policy이지 fraud probability,
production threshold, model이 아니다.

### Deterministic Ordering

- evidence: kind, 그다음 opaque identifier ID, 둘 다 ascending
- cluster 안의 user: opaque user ID ascending
- cluster: score descending, user count descending, 그다음 smallest opaque user ID ascending
- isolated user: opaque user ID ascending
- cluster ID: `cluster:`에 component 안의 smallest opaque user ID를 붙인 값

input fixture order와 Neo4j record order는 output을 바꾸면 안 된다.

## CLI Contract

executable은 다음과 같이 실행한다.

```bash
NEO4J_URI=bolt://127.0.0.1:7687 \
  go run ./examples/graph-abuse-cluster
```

local README run path는 authentication을 비활성화한 Neo4j를 시작하고 Bolt를 loopback에 bind한다.
`NEO4J_URI`는 필수다. CLI는 container를 시작하지 않는다. Authentication, TLS, routing, secret loading,
production Neo4j deployment는 명시적 non-goal로 남는다.

예제는 의도적으로 `neo4j.NoAuth`를 사용하므로 configuration validation은 필수 numeric port와 loopback
host를 가진 `bolt` URI만 허용한다. 허용 host는 `localhost`, `127.0.0.0/8`, `::1`이다. driver 생성
전에 user information, path, query parameter, fragment, non-loopback host, 누락되었거나 유효하지 않은
port, 그 밖의 모든 scheme을 거부한다. 이것은 local lesson이지 general Neo4j connection parser가 아니다.

CLI는 다음을 수행한다.

- 15초 operation deadline을 가진 signal-aware context 하나를 파생한다.
- official Neo4j driver와 released adapter client를 생성한다.
- fixture reset 전에 connectivity를 검증한다.
- seed, read, analyze를 수행하고 하나의 JSON report를 memory에 encode한다.
- success output 전에 fresh bounded cleanup context로 client/driver를 닫는다.
- cleanup이 성공한 뒤에만 완전히 encode된 report를 stdout에 쓴다.
- failure 시 stderr에는 stable error category만 쓰고 non-zero를 반환한다.

JSON report는 `clusters`와 `isolated_users`를 포함한다. 각 cluster는 `cluster_id`, 정렬된
`users`, 정렬된 `evidence`, `risk_score`를 포함한다. evidence는 fixture-safe `kind`, `opaque_id`,
`user_count`, `weight`만 포함한다.

정확한 checked-in fixture output은 다음과 같다.

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

## Errors, Cancellation, and Cleanup

internal error는 test와 caller inspection을 위해 `%w`로 cause를 보존한다. application은 invalid
fixture, invalid graph data, oversized graph, configuration, backend operation failure에 대한 stable
category를 정의한다. test가 caller cancellation과 provider failure를 구분할 수 있도록
`context.Canceled`와 `context.DeadlineExceeded`를 보존한다.

terminal boundary는 failure 시 raw Cypher, parameter, Neo4j provider message, credential, URI,
identifier value를 출력하지 않는다. successful JSON은 의도적으로 opaque한 fixture evidence만 포함한다.

driver는 이를 생성한 CLI 또는 test가 소유한다. cleanup은 fresh bounded context를 사용해 canceled
operation context가 resource closure를 막지 못하게 한다. Testcontainers termination은 startup 직후
등록되고 자체 bounded cleanup context를 사용한다.

complete JSON report는 cleanup과 output 전에 buffer에 encode된다. invalid record, missing endpoint,
unknown kind, over-limit result, cancellation, query failure, JSON encoding failure, cleanup
failure는 error를 반환하고 success output을 시도하지 않는다. low-level stdout write failure는 operating
system이 이미 받아들인 byte를 남길 수 있다. 그래도 non-zero를 반환하며 success로 보고하지 않는다.

## Dependency Decision

이 issue는 승인된 Neo4j boundary에 필요한 dependency만 promote한다.

- caller-owned runtime driver용 `github.com/neo4j/neo4j-go-driver/v6`
- serial integration evidence용 `github.com/testcontainers/testcontainers-go/modules/neo4j`

둘 다 released `bluetape-go v0.18.0 graph/neo4j` implementation 및 test surface와 맞는다.
alternative driver, graph algorithm library, CLI framework, logging dependency는 추가하지 않는다.
`go mod tidy`가 정확한 selected version을 소유하며 `tidy-check`는 계속 clean해야 한다.

## Security and Privacy Boundary

- 모든 fixture identifier는 synthetic opaque string이다.
- application은 raw user input을 hash하지 않으므로 hashing strength 또는 collision claim을 하지 않는다.
- Cypher structure는 고정이고 모든 data value는 parameterized다.
- local run command는 Neo4j를 loopback에만 bind하며, CLI validation은 driver 생성 전에 non-loopback
  또는 credential-bearing connection URI를 거부한다.
- raw provider error와 connection detail은 terminal boundary에서 redact된다.
- authentication, authorization, tenant isolation, encryption, retention, audit logging,
  production fraud decision은 구현하지 않으며 양쪽 README에 명시해야 한다.

## Documentation and Diagrams

`README.md`와 `README.ko.md`는 source-equivalent를 유지하고 다음을 포함한다.

- lesson과 non-goal
- vertex/edge schema와 score formula
- Docker 및 `go run` command
- representative deterministic JSON output
- cancellation, bound, privacy, production caveat
- focused, race, serial Testcontainers test command

root README pair는 navigation 및 run entry 하나를 추가한다.

두 쌍의 SVG/PNG diagram은 `bluetape-diagram`을 통해 생성한다.

1. architecture: fixture validation -> Neo4j adapter -> Neo4j -> bounded read
   -> Go analyzer -> JSON report
2. sequence: configuration, connectivity, scoped reset, seed, bounded reads,
   cluster analysis, output, cleanup

diagram checklist는 automated connector/geometry/endpoint/style audit, deterministic rerendering,
original-size SVG/PNG eye inspection, conversion 이후 arrowhead direction, marker size에 대한 bend
clearance를 포함한다.

## Test Strategy

### Pure tests

- valid fixture construction과 graph-value validation
- duplicate vertex, duplicate edge, missing endpoint, unknown kind
- direct 및 transitive shared-identifier cluster
- isolated user와 unique identifier
- evidence deduplication과 weight calculation
- reversed/shuffled input 아래의 cluster 및 evidence tie-breaking
- vertex 및 edge limit-plus-one rejection
- loopback URI acceptance와 driver 생성 전 remote, credential-bearing, malformed, missing-port rejection
- canceled context propagation
- deterministic JSON projection, encode-before-cleanup ordering, cleanup-error가 success output을 억제하는
  동작, stdout write failure

### Neo4j integration test

하나의 serial test는 `neo4j:5.26.0`을 시작하고 cleanup을 즉시 등록하며 Bolt URL을 얻는다. 그런 뒤
official driver와 released adapter를 구성하고 actual connectivity를 검증하며, unique fixture namespace를
write하고 두 adapter method로 read한다. 이어서 result를 analyze하고 exact report를 확인하며,
cancellation을 증명하고 자기 namespace만 삭제한다. 또한 atomic reset/seed를 반복해 idempotent count를
증명하고 unrelated fixture namespace가 건드려지지 않았음을 검증한다.

Memgraph는 시작하지 않는다. real-service test는 다른 container suite와 parallel로 실행하지 않는다.

### Repository gates

```bash
go test -count=1 ./examples/graph-abuse-cluster/...
go test -race -count=1 ./examples/graph-abuse-cluster/...
go test -p 1 -count=1 ./examples/graph-abuse-cluster/... -run Neo4j
go test -p 1 -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check origin/develop
```

## Failure Modes and Responses

| Failure | Required response |
|---|---|
| Neo4j is unreachable or becomes unavailable | context/provider cause를 내부적으로 보존하고, redacted backend category 하나를 render하며, driver를 닫고 non-zero를 반환한다. |
| Fixture reset is accidentally broad or seeding fails partway | 하나의 scoped reset/seed statement가 하나의 managed transaction에서 실행된다. review 및 integration evidence는 unscoped deletion과 half-seeded state를 거부한다. |
| A query returns more records than the lesson bounds | `maximum + 1`을 감지하고 typed oversized-graph error를 반환하며 partial cluster report를 내보내지 않는다. |
| Neo4j record order changes | deterministic contract에 따라 user, evidence, cluster, isolated user를 정렬한다. |
| A transitive component contains malformed data | typed invalid-graph error로 전체 analysis를 실패시킨다. record를 조용히 drop하지 않는다. |
| Cancellation occurs during query or cleanup | operation에는 caller cancellation을 반환하고 cleanup에는 별도 bounded context를 사용한다. |
| Two CLI processes use the fixed fixture namespace | 이 focused lesson은 concurrent run을 지원하지 않는다. boundary를 문서화하고 test에서는 unique namespace를 사용한다. |
| A diagram renders with reversed or colliding arrowheads | endpoint와 bend를 조정하고, rerender와 audit을 다시 실행하며, original-size PNG inspection을 반복한다. |

## Compatibility and Migration

새 example은 기존 workshop dependency baseline에 맞춰 compile되며 reusable API를 추가하지 않는다. 기존
example, workflow, README behavior는 compatible하게 유지된다. `go.mod`와 `go.sum`은 direct Neo4j driver와
official Neo4j Testcontainers module compile에 필요한 만큼만 변경된다.

historical milestone은 0.10.0으로 유지되지만 implementation은 현재 stable `bluetape-go v0.18.0`을
사용한다. 이후 stable library release의 새 surface를 example이 채택하려면 먼저 issue rebaseline이
필요하다.

## Spec Review Convergence

세 delegated review lane이 두 번의 bounded wait와 explicit finish request를 초과했다. workflow를
stalled 상태로 두지 않기 위해 required perspective는 documented main-session fallback으로 완료했다.

| Perspective | Initial result | Resolution | Final result |
|---|---|---|---|
| Performance | P0=0, P1=0 | fixed three-operation database path, result cap, no-N+1 boundary를 명시했다. | P0=0, P1=0 |
| Stability | P0=0, P1=2 | reset/seed는 하나의 managed transaction이다. JSON은 buffer에 담고 cleanup은 output보다 앞서며 concurrent CLI run은 명시적으로 unsupported다. | P0=0, P1=0 |
| Security | P0=0, P1=1 | no-auth configuration은 strict loopback Bolt URI만 허용하고 driver 생성 전에 credential 또는 remote target을 거부한다. | P0=0, P1=0 |
| Operator/Ops | P0=0, P1=0 | connectivity, scoped rollback, exit behavior, bounded cleanup, local-only deployment boundary를 소유한다. | P0=0, P1=0 |
| Developer/API | P0=0, P1=0 | exact fixture, adapter call, typed failure category, dependency scope, expected report는 implementable하다. | P0=0, P1=0 |
| User/caller | P0=0, P1=0 | deterministic output, illustrative-score caveat, unsupported concurrency, run path, production non-goal을 명시했다. | P0=0, P1=0 |

integrated spec review에는 deferred P2/P3 finding과 open user decision이 없다. repair는 승인된
Neo4j-only architecture를 좁히고 증명하며 selected approach를 변경하지 않는다.

## Acceptance Criteria

- fixture는 persistence 전에 released graph value로 검증된다.
- 하나의 atomic scoped Neo4j reset/seed와 두 bounded read는 N+1 query 없이 released caller-owned
  adapter를 사용한다.
- Go code는 정확한 direct 및 transitive cluster, evidence, score, tie-break, isolated user를 만든다.
- invalid graph data, limit overflow, backend failure, cancellation은 partial output 또는 provider
  leakage 없이 fail closed한다.
- CLI는 strict loopback Bolt URI만 허용하고 local Neo4j instance에 대해 실행 가능하며 cleanup 성공
  이후에만 deterministic JSON을 내보낸다.
- serial Neo4j Testcontainers test는 connection readiness, adaptation, exact output, cancellation,
  cleanup을 증명한다.
- English 및 Korean README, root navigation, paired architecture/sequence diagram은 synchronized되고
  visually verified된다.
- focused, race, serial container, repository, lint, `make ci` gate가 통과한다.
- Type A verifier와 여섯 review lens는 PR 전에 P0=0/P1=0으로 수렴한다.
- PR metadata는 #50을 mirror하고 merge decision gate 전에 CI가 green이어야 한다.

## Definition of Done

- approved spec과 implementation plan은 code 전에 commit된다.
- example은 application-shaped 및 Neo4j-only로 남는다.
- backend-neutral repository, Cypher DSL, generic fraud engine, raw identifier/provider disclosure를
  도입하지 않는다.
- 모든 acceptance criterion은 fresh test, document, diagram 또는 live GitHub evidence를 가진다.
- required workflow check는 unresolved P0/P1 없이 `Blocked: 0`을 report한다.
- push와 PR은 local gate가 수렴한 뒤에만 허용된다. merge는 별도의 explicit user decision으로 남는다.
