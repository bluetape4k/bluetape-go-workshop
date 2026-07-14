# Issue #50 Graph Abuse Cluster Design

## Status

- Work type: Type A - Full Feature
- Issue: #50 `[v0.10.0] Add graph abuse cluster example`
- Parent track: #36
- Parent roadmap: #27
- Implementation baseline: `bluetape-go v0.18.0`
- Design decision: user approved approach 1, Neo4j-only persistence with
  application-owned cluster analysis
- Approval date: 2026-07-14 KST

## Problem and Outcome

The workshop needs its first graph-domain example. It must demonstrate the
released `graph` values and narrow `graph/neo4j` adapter in a concrete abuse
investigation scenario without turning the workshop into a graph framework or
fraud product.

The example will seed opaque users and identifiers into one Neo4j fixture,
read a bounded graph through the released adapter, calculate connected abuse
clusters in Go, and print deterministic JSON evidence. A reader should be able
to see which behavior belongs to Neo4j, which belongs to `bluetape-go`, and
which remains application policy.

## Evidence and Adopt Decisions

### Adopt

- `graph.Vertex`, `graph.Edge`, `graph.Properties`, and their validation
  contracts for fixture values.
- `graph/neo4j.Client` around a caller-owned official Neo4j driver.
- `ExecuteWrite`, `ReadVertices`, and `ReadEdges` for the small fixed Cypher
  boundary.
- The upstream `graph/neo4j` Testcontainers pattern using
  `testcontainers-go/modules/neo4j`, because `bluetape-go v0.18.0` does not
  provide a reusable `testcontainers/neo4j` launcher.
- The repository convention of a runnable `main` plus an `internal` package,
  bilingual README pair, root navigation, focused tests, and serial container
  evidence.

### Borrow

- The earlier graph candidate research decision that #50 is the first graph
  example and should use opaque identifiers, a deterministic fixture, and a
  domain-owned algorithm.
- The Kotlin abuse-detection scenario only as domain inspiration. No Kotlin
  repository, traversal abstraction, or API shape is ported.

### Reject

- Full cluster computation in Cypher. It hides the teaching policy in a
  backend query and makes deterministic unit testing less direct.
- An in-memory-only graph with Neo4j used as a connectivity smoke test. It does
  not prove the released adapter in the actual data flow.
- Memgraph coverage. The user selected Neo4j-only scope for this focused
  example; backend compatibility belongs upstream or in later work.

## Architecture

The example lives at `examples/graph-abuse-cluster` and has four bounded
components.

1. **Fixture builder** creates validated graph vertices and edges from opaque
   constants. It contains no database behavior.
2. **Neo4j store** translates the validated fixture to parameter maps, resets
   and seeds only its fixture namespace in one atomic write, and reads bounded
   vertices and edges through `graph/neo4j.Client`.
3. **Analyzer** builds the bipartite adjacency map in memory, finds connected
   user components, extracts shared evidence, calculates an illustrative risk
   score, and sorts the report deterministically.
4. **CLI** owns configuration, driver/client lifecycle, signal and timeout
   context, JSON output, and redacted terminal errors.

No component defines a reusable graph repository, algorithm library, schema
DSL, query builder, or provider abstraction. Reusable capability changes, if
discovered, belong in a separate `bluetape-go` issue.

## Graph Schema

### Vertices

| Label | Required properties | Meaning |
|---|---|---|
| `User` | `opaque_id`, `fixture_id` | One investigated account fixture |
| `Identifier` | `opaque_id`, `kind`, `fixture_id` | Opaque device, IP, or payment-token reference |

`kind` accepts only `device`, `ip`, or `payment_token`. All IDs are fixture
values that reveal no raw device, address, account, or payment data.

### Edges

`(User)-[:USES_IDENTIFIER]->(Identifier)` records a directed association. The
fixture builder validates every graph value before database work. The
application also rejects an unknown identifier kind, duplicate opaque vertex
ID, duplicate logical edge, or edge whose endpoint is absent.

Neo4j `ElementId` values remain adapter identities. The application joins read
vertices and edges by `graph.ElementID` and uses the opaque properties only for
domain output and deterministic ordering.

## Fixture and Database Boundary

The checked-in fixture has enough structure to prove:

- one three-user component connected transitively through more than one
  identifier kind;
- one two-user component with a score tie candidate;
- one isolated user with only unique identifiers;
- deterministic ordering independent of input order.

The exact logical fixture is:

| Identifier | Kind | Linked users |
|---|---|---|
| `dev-001` | `device` | `usr-001`, `usr-002` |
| `ip-001` | `ip` | `usr-002`, `usr-003` |
| `pay-001` | `payment_token` | `usr-003` only; not shared evidence |
| `dev-002` | `device` | `usr-004`, `usr-005` |
| `ip-002` | `ip` | `usr-004`, `usr-005` |
| `ip-003` | `ip` | `usr-006` only; isolated user evidence |

This produces two score-4 clusters. `cluster:usr-001` sorts first because it
contains three users, while `cluster:usr-004` contains two. `usr-006` is the
only isolated user. Pure unit fixtures separately prove the payment-token
weight and the final smallest-user-ID tie-break.

The CLI uses one constant fixture namespace. One `ExecuteWrite` statement
deletes only nodes with that exact `fixture_id` and then creates every fixture
vertex and edge in the same managed Neo4j transaction. A validation or write
failure rolls back both reset and seed; it never leaves a half-seeded fixture
and never runs an unscoped delete. Test code uses a per-test namespace and
registers bounded cleanup.

All Cypher labels and relationship types are fixed source constants. Every
fixture value is a query parameter. Writes occur through
`graph/neo4j.Client.ExecuteWrite`. Reads use separate bounded vertex and edge
queries through `ReadVertices` and `ReadEdges`.

After connectivity, the normal path has exactly three database operations: one
atomic reset/seed, one vertex read, and one edge read. It performs no per-user
or per-identifier query. The focused CLI is single-run teaching code; concurrent
processes sharing its constant fixture namespace are unsupported and are
called out in both READMEs.

Each read asks for `maximum + 1` records. The store returns a typed size error
instead of silently truncating when the result exceeds these limits:

- maximum vertices: 256;
- maximum edges: 1024.

The example intentionally collects this bounded fixture in memory. It makes no
production-scale traversal or throughput claim.

## Cluster Contract

The analyzer treats the graph as a bipartite user-to-identifier graph.

1. Build adjacency only from validated `USES_IDENTIFIER` edges.
2. Traverse connected components through identifiers.
3. A component with two or more users is an abuse cluster.
4. An identifier is evidence only when at least two users in that component
   reference it.
5. A user outside every abuse cluster is reported as isolated.

Transitive linkage is intentional: if user A shares a device with B and B
shares an IP with C, A, B, and C form one component even when A and C share no
identifier directly.

### Illustrative Score

Each distinct shared identifier contributes once:

| Identifier kind | Weight |
|---|---:|
| `payment_token` | 5 |
| `device` | 3 |
| `ip` | 1 |

The cluster score is the sum of those evidence weights. It is a transparent
workshop policy, not a fraud probability, production threshold, or model.

### Deterministic Ordering

- evidence: kind, then opaque identifier ID, both ascending;
- users inside a cluster: opaque user ID ascending;
- clusters: score descending, user count descending, then smallest opaque user
  ID ascending;
- isolated users: opaque user ID ascending;
- cluster ID: `cluster:` plus the smallest opaque user ID in the component.

Input fixture order and Neo4j record order must not change the output.

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
