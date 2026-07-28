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

The executable is run with:

```bash
NEO4J_URI=bolt://127.0.0.1:7687 \
  go run ./examples/graph-abuse-cluster
```

The local README run path starts Neo4j with authentication disabled and binds
Bolt to loopback. `NEO4J_URI` is required; the CLI does not start a container.
Authentication, TLS, routing, secret loading, and production Neo4j deployment
remain explicit non-goals.

Because the example deliberately uses `neo4j.NoAuth`, configuration validation
accepts only a `bolt` URI with a required numeric port and a loopback host:
`localhost`, `127.0.0.0/8`, or `::1`. It rejects user information, paths, query
parameters, fragments, non-loopback hosts, missing or invalid ports, and every
other scheme before driver creation. This is a local lesson, not a general
Neo4j connection parser.

The CLI:

- derives one signal-aware context with a 15-second operation deadline;
- creates the official Neo4j driver and released adapter client;
- verifies connectivity before fixture reset;
- seeds, reads, analyzes, and encodes one JSON report into memory;
- closes the client/driver with a fresh bounded cleanup context before success
  output;
- writes the complete encoded report to stdout only after cleanup succeeds;
- writes only a stable error category to stderr and returns non-zero on failure.

The JSON report contains `clusters` and `isolated_users`. Each cluster contains
`cluster_id`, sorted `users`, sorted `evidence`, and `risk_score`. Evidence
contains only fixture-safe `kind`, `opaque_id`, `user_count`, and `weight`.

The exact checked-in fixture output is:

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

Internal errors preserve causes with `%w` for tests and caller inspection. The
application defines stable categories for invalid fixture, invalid graph data,
oversized graph, configuration, and backend operation failure. It preserves
`context.Canceled` and `context.DeadlineExceeded` so tests can distinguish
caller cancellation from provider failure.

The terminal boundary does not print raw Cypher, parameters, Neo4j provider
messages, credentials, URIs, or identifier values on failures. Successful JSON
contains only the intentionally opaque fixture evidence.

The driver is owned by the CLI or test that created it. Cleanup uses a fresh
bounded context so a canceled operation context does not prevent resource
closure. Testcontainers termination is registered immediately after startup
and uses its own bounded cleanup context.

The complete JSON report is encoded into a buffer before cleanup and output.
Any invalid record, missing endpoint, unknown kind, over-limit result,
cancellation, query failure, JSON encoding failure, or cleanup failure returns
an error and does not attempt success output. A low-level stdout write failure
may leave bytes already accepted by the operating system; it still returns
non-zero and is never reported as success.

## Dependency Decision

This issue promotes only the dependencies required by the approved Neo4j
boundary:

- `github.com/neo4j/neo4j-go-driver/v6` for the caller-owned runtime driver;
- `github.com/testcontainers/testcontainers-go/modules/neo4j` for serial
  integration evidence.

Both match the released `bluetape-go v0.18.0 graph/neo4j` implementation and
test surface. No alternative driver, graph algorithm library, CLI framework, or
logging dependency is added. `go mod tidy` owns the exact selected versions and
`tidy-check` must remain clean.

## Security and Privacy Boundary

- All fixture identifiers are synthetic opaque strings.
- The application never hashes raw user input and therefore makes no hashing
  strength or collision claim.
- Cypher structure is fixed; all data values are parameterized.
- The local run command binds Neo4j only to loopback, and CLI validation rejects
  non-loopback or credential-bearing connection URIs before driver creation.
- Raw provider errors and connection details are redacted at the terminal
  boundary.
- Authentication, authorization, tenant isolation, encryption, retention,
  audit logging, and production fraud decisions are not implemented and must
  be called out in both READMEs.

## Documentation and Diagrams

`README.md` and `README.ko.md` will stay source-equivalent and include:

- the lesson and non-goals;
- the vertex/edge schema and score formula;
- the Docker and `go run` commands;
- representative deterministic JSON output;
- cancellation, bounds, privacy, and production caveats;
- focused, race, and serial Testcontainers test commands.

The root README pair adds one navigation and run entry.

Two paired SVG/PNG diagrams will be produced through `bluetape-diagram`:

1. architecture: fixture validation -> Neo4j adapter -> Neo4j -> bounded read
   -> Go analyzer -> JSON report;
2. sequence: configuration, connectivity, scoped reset, seed, bounded reads,
   cluster analysis, output, and cleanup.

The diagram checklist includes automated connector/geometry/endpoint/style
audits, deterministic rerendering, original-size SVG/PNG eye inspection,
arrowhead direction after conversion, and bend clearance for marker size.

## Test Strategy

### Pure tests

- valid fixture construction and graph-value validation;
- duplicate vertex, duplicate edge, missing endpoint, and unknown kind;
- direct and transitive shared-identifier clusters;
- isolated users and unique identifiers;
- evidence deduplication and weight calculation;
- cluster and evidence tie-breaking under reversed/shuffled input;
- vertex and edge limit-plus-one rejection;
- loopback URI acceptance plus remote, credential-bearing, malformed, and
  missing-port rejection before driver creation;
- canceled context propagation;
- deterministic JSON projection, encode-before-cleanup ordering, cleanup-error
  suppression of success output, and stdout write failure.

### Neo4j integration test

One serial test starts `neo4j:5.26.0`, registers cleanup immediately, obtains a
Bolt URL, constructs the official driver and released adapter, verifies actual
connectivity, writes a unique fixture namespace, reads through both adapter
methods, analyzes the result, checks the exact report, proves cancellation, and
deletes only its namespace. It repeats the atomic reset/seed to prove idempotent
counts and verifies that unrelated fixture namespaces remain untouched.

Memgraph is not started. Real-service tests do not run in parallel with other
container suites.

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
| Neo4j is unreachable or becomes unavailable | Preserve context/provider cause internally, render one redacted backend category, close the driver, and return non-zero. |
| Fixture reset is accidentally broad or seeding fails partway | One scoped reset/seed statement runs in one managed transaction; review and integration evidence reject unscoped deletion and half-seeded state. |
| A query returns more records than the lesson bounds | Detect `maximum + 1`, return the typed oversized-graph error, and emit no partial cluster report. |
| Neo4j record order changes | Sort users, evidence, clusters, and isolated users according to the deterministic contract. |
| A transitive component contains malformed data | Fail the whole analysis with a typed invalid-graph error; never silently drop a record. |
| Cancellation occurs during query or cleanup | Return caller cancellation for the operation; use a separate bounded context for cleanup. |
| Two CLI processes use the fixed fixture namespace | This focused lesson does not support concurrent runs; document the boundary and use unique namespaces in tests. |
| A diagram renders with reversed or colliding arrowheads | Adjust endpoints and bends, rerender, rerun audits, and repeat original-size PNG inspection. |

## Compatibility and Migration

The new example compiles against the existing workshop dependency baseline and
adds no reusable API. Existing examples, workflows, and README behavior remain
compatible. `go.mod` and `go.sum` change only as required to compile the direct
Neo4j driver and official Neo4j Testcontainers module.

The historical milestone remains 0.10.0 while implementation uses the current
stable `bluetape-go v0.18.0`. A later stable library release requires an issue
rebaseline before the example adopts new surfaces.

## Spec Review Convergence

Three delegated review lanes exceeded two bounded waits and an explicit finish
request, so the required perspectives were completed through the documented
main-session fallback rather than leaving the workflow stalled.

| Perspective | Initial result | Resolution | Final result |
|---|---|---|---|
| Performance | P0=0, P1=0 | Fixed three-operation database path, result caps, and no-N+1 boundary made explicit. | P0=0, P1=0 |
| Stability | P0=0, P1=2 | Reset/seed is one managed transaction; JSON is buffered, cleanup precedes output, and concurrent CLI runs are explicitly unsupported. | P0=0, P1=0 |
| Security | P0=0, P1=1 | No-auth configuration now accepts only strict loopback Bolt URIs and rejects credentials or remote targets before driver creation. | P0=0, P1=0 |
| Operator/Ops | P0=0, P1=0 | Connectivity, scoped rollback, exit behavior, bounded cleanup, and local-only deployment boundary are owned. | P0=0, P1=0 |
| Developer/API | P0=0, P1=0 | Exact fixture, adapter calls, typed failure categories, dependency scope, and expected report are implementable. | P0=0, P1=0 |
| User/caller | P0=0, P1=0 | Deterministic output, illustrative-score caveat, unsupported concurrency, run path, and production non-goals are explicit. | P0=0, P1=0 |

The integrated spec review has no deferred P2/P3 finding and no open user
decision. The repairs narrow and prove the approved Neo4j-only architecture;
they do not change its selected approach.

## Acceptance Criteria

- The fixture is validated with released graph values before persistence.
- One atomic scoped Neo4j reset/seed and two bounded reads use the released
  caller-owned adapter without N+1 queries.
- Go code produces exact direct and transitive clusters, evidence, scores,
  tie-breaks, and isolated users.
- Invalid graph data, limit overflow, backend failure, and cancellation fail
  closed without partial output or provider leakage.
- The CLI accepts only a strict loopback Bolt URI, is runnable against a local
  Neo4j instance, and emits deterministic JSON only after cleanup succeeds.
- A serial Neo4j Testcontainers test proves connection readiness, adaptation,
  exact output, cancellation, and cleanup.
- English and Korean READMEs, root navigation, and paired architecture/sequence
  diagrams are synchronized and visually verified.
- Focused, race, serial container, repository, lint, and `make ci` gates pass.
- Type A verifier and six review lenses converge at P0=0/P1=0 before PR.
- PR metadata mirrors #50 and CI is green before the merge decision gate.

## Definition of Done

- Approved spec and implementation plan are committed before code.
- The example remains application-shaped and Neo4j-only.
- No backend-neutral repository, Cypher DSL, generic fraud engine, or raw
  identifier/provider disclosure is introduced.
- Every acceptance criterion has fresh test, document, diagram, or live GitHub
  evidence.
- Required workflow checks report `Blocked: 0`, with no unresolved P0/P1.
- Push and PR are allowed only after the local gates converge; merge remains a
  separate explicit user decision.
