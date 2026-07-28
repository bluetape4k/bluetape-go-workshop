# Issue #50 Graph Abuse Cluster Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** opaque abuse graph를 `bluetape-go`로 validate하고, `graph/neo4j`를 통해 persist/read하며, Go에서 deterministic cluster를 계산하고 approved JSON report를 출력하는 runnable Neo4j-only CLI를 만든다.

**Architecture:** fixture builder가 logical graph validation을 소유하고, 좁은 Neo4j store가 atomic scoped reset/seed 하나와 bounded read 두 개를 소유한다. analyzer는 connected component, evidence, scoring, ordering을 소유한다. CLI는 strict loopback configuration, caller-owned driver lifecycle, buffered JSON output, redacted error를 소유한다. reusable repository, Cypher DSL, Memgraph, HTTP, production fraud behavior는 scope 밖이다.

**Tech Stack:** Go 1.26.3, `bluetape-go v0.18.0`, official Neo4j Go driver v6.1.0, Testcontainers Neo4j v0.42.0, standard library JSON/context/signal handling, SVG 및 CairoSVG PNG rendering을 사용한다.

---

## Approved Inputs and Boundaries

- Approved spec:
  `docs/superpowers/specs/2026-07-14-issue-50-graph-abuse-cluster-design.md`
- Base: `origin/develop@ed1fb311c256bf1c95c9545f5c418a5460c9b278`
- Worktree:
  `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster`
- Branch: `feat/issue-50-graph-abuse-cluster`
- GitHub: issue #50, parent #36, roadmap #27, milestone `0.10.0`, assignee
  `debop`, labels `enhancement` and `examples`
- approved workflow가 허용한 external effect: local convergence 이후 commit, push, PR.
  merge는 별도의 명시적 사용자 결정으로 남는다.

## File Responsibility Map

| Path | Responsibility |
|---|---|
| `examples/graph-abuse-cluster/internal/abusecluster/doc.go` | package lesson과 boundary documentation |
| `examples/graph-abuse-cluster/internal/abusecluster/errors.go` | stable internal sentinel category와 redacted operation error |
| `examples/graph-abuse-cluster/internal/abusecluster/model.go` | identifier kind, fixture/report/evidence/cluster value, constant, limit |
| `examples/graph-abuse-cluster/internal/abusecluster/fixture.go` | exact checked-in fixture와 pre-persistence graph validation |
| `examples/graph-abuse-cluster/internal/abusecluster/analyzer.go` | graph adaptation, connected component, evidence, score, deterministic sort |
| `examples/graph-abuse-cluster/internal/abusecluster/store.go` | atomic parameterized Neo4j reset/seed와 bounded adapter read |
| `examples/graph-abuse-cluster/internal/abusecluster/workflow.go` | narrow test seam 뒤 ordered store -> analyze application use case |
| `examples/graph-abuse-cluster/internal/abusecluster/output.go` | memory로 deterministic indented JSON encoding |
| Matching `*_test.go` files | 각 responsibility의 table-driven RED/GREEN proof |
| `examples/graph-abuse-cluster/main.go` | loopback config, signal/deadline, driver/client ownership, cleanup-before-output |
| `examples/graph-abuse-cluster/main_test.go` | URI rejection, redaction, cleanup/output ordering, exit behavior |
| `examples/graph-abuse-cluster/integration_test.go` | serial real-Neo4j fixture round trip 하나, cancellation, idempotence, cleanup |
| `examples/graph-abuse-cluster/README.md` | English lesson, command, exact output, caveat |
| `examples/graph-abuse-cluster/README.ko.md` | source-equivalent natural Korean lesson |
| `docs/images/readme-diagrams/graph-abuse-cluster-architecture.{svg,png}` | static component/ownership view |
| `docs/images/readme-diagrams/graph-abuse-cluster-sequence.{svg,png}` | ordered lifecycle view |
| `README.md`, `README.ko.md` | root example table 및 runnable section |
| `docs/review/2026-07-14-issue-50-graph-abuse-cluster.md` | integrated Type A 및 diagram evidence |
| `docs/lessons/2026-07-14-issue-50-graph-abuse-cluster.md` | durable decision, miss, future guard |
| `go.mod`, `go.sum` | approved direct Neo4j runtime/test dependency만 포함 |

## Task 1: Dependencies, Errors, Validated Fixture 고정

**Complexity:** Medium. **Depends on:** approved plan. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** dependency file과 fixture/model package file.

- [ ] **Step 1: approved dependency version만 추가**

실행한다.

```bash
go get github.com/neo4j/neo4j-go-driver/v6@v6.1.0
go get github.com/testcontainers/testcontainers-go/modules/neo4j@v0.42.0
go mod tidy
```

기대값: `go.mod`가 `bluetape-go v0.18.0`에서 사용하는 같은 version의 driver와 Neo4j Testcontainers module을 direct require한다.
graph algorithm, CLI, logging, alternative-driver dependency는 나타나지 않는다.

- [ ] **Step 2: fixture validation test를 먼저 작성**

planned signature를 호출하는 table로 `fixture_test.go`를 만든다.

```go
func NewFixture(fixtureID string) (Fixture, error)
func DefaultFixture() (Fixture, error)
func ValidateFixture(Fixture) error
```

필수 case:

```go
tests := []struct {
    name   string
    mutate func(Fixture) Fixture
    target error
}{
    {"duplicate vertex", duplicateFirstVertex, ErrInvalidFixture},
    {"duplicate logical edge", duplicateFirstEdge, ErrInvalidFixture},
    {"missing endpoint", replaceFirstEndWithMissing, ErrInvalidFixture},
    {"unknown identifier kind", replaceFirstKind("cookie"), ErrInvalidFixture},
    {"too many vertices", appendUntilVertexCount(MaxVertices + 1), ErrGraphTooLarge},
    {"too many edges", appendUntilEdgeCount(MaxEdges + 1), ErrGraphTooLarge},
}
```

happy-path assertion은 `User` vertex 6개, `Identifier` vertex 6개, `USES_IDENTIFIER` edge 10개,
caller mutation의 영향을 받지 않는 defensive copy를 요구한다. max+1 case가 fail하기 전에 256 vertex와 1024 edge가 accept됨을 증명하는 exact-boundary fixture를 추가한다.
`NewFixture("test-namespace")`는 fixture와 모든 vertex/edge property에 해당 namespace를 가진 같은 logical graph를 rebuild해야 한다.
blank/whitespace ID는 `ErrInvalidFixture`를 반환한다. integration test가 shared fixture data를 mutate하지 않도록 `DefaultFixture`는 `NewFixture(FixtureID)`에 delegate한다.

- [ ] **Step 3: RED 확인**

Run:

```bash
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(NewFixture|DefaultFixture|ValidateFixture)'
```

기대값: `Fixture`, `NewFixture`, `DefaultFixture`, error value가 없으므로 compile FAIL한다.

- [ ] **Step 4: minimum fixture contract 구현**

`doc.go`, `errors.go`, `model.go`, `fixture.go`를 만든다. 이후 task에서 다음 core shape를 일관되게 사용한다.

```go
type IdentifierKind string

const (
    IdentifierDevice       IdentifierKind = "device"
    IdentifierIP           IdentifierKind = "ip"
    IdentifierPaymentToken IdentifierKind = "payment_token"
    FixtureID                              = "graph-abuse-cluster-v1"
    MaxVertices                            = 256
    MaxEdges                               = 1024
)

type Fixture struct {
    ID       string
    Vertices []graph.Vertex
    Edges    []graph.Edge
}

var (
    ErrInvalidFixture = errors.New("graph abuse cluster: invalid fixture")
    ErrInvalidGraph   = errors.New("graph abuse cluster: invalid graph")
    ErrGraphTooLarge  = errors.New("graph abuse cluster: graph too large")
    ErrConfiguration  = errors.New("graph abuse cluster: invalid configuration")
    ErrBackend        = errors.New("graph abuse cluster: backend operation failed")
)
```

모든 vertex는 `graph.ParseVertex`로 만들고 모든 edge는 `graph.ParseEdge`로 만든다.
`NewFixture`는 returned property map을 mutate하지 말고 supplied namespace에 대한 모든 graph value를 reconstruct한다.
ID, exact label, required string property, allowed kind, duplicate logical ID/edge, endpoint existence를 validate한다.
safe sentinel cause는 `%w`로 wrap한다. rendered error text는 property value나 opaque ID를 보존하면 안 된다.

- [ ] **Step 5: GREEN 확인 및 package surface lint**

Run:

```bash
gofmt -w examples/graph-abuse-cluster/internal/abusecluster
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(NewFixture|DefaultFixture|ValidateFixture)'
go vet ./examples/graph-abuse-cluster/internal/abusecluster
```

기대값: touched package에 exported-symbol documentation warning 없이 PASS한다.

- [ ] **Step 6: Task 1 commit**

```bash
git add go.mod go.sum examples/graph-abuse-cluster/internal/abusecluster
git commit -m "feat: add validated graph abuse fixture"
```

기대값: dependency와 fixture/model ownership만 포함한 commit 하나.

## Task 2: Deterministic Cluster Analysis 구현

**Complexity:** High. **Depends on:** Task 1. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** analyzer와 해당 test.

- [ ] **Step 1: approved report에 대한 RED test 작성**

다음을 대상으로 `analyzer_test.go`를 만든다.

```go
func Analyze(vertices []graph.Vertex, edges []graph.Edge) (Report, error)
```

explicit report value를 사용한다.

```go
want := Report{
    Clusters: []Cluster{
        {
            ClusterID: "cluster:usr-001",
            Users: []string{"usr-001", "usr-002", "usr-003"},
            Evidence: []Evidence{
                {Kind: IdentifierDevice, OpaqueID: "dev-001", UserCount: 2, Weight: 3},
                {Kind: IdentifierIP, OpaqueID: "ip-001", UserCount: 2, Weight: 1},
            },
            RiskScore: 4,
        },
        {
            ClusterID: "cluster:usr-004",
            Users: []string{"usr-004", "usr-005"},
            Evidence: []Evidence{
                {Kind: IdentifierDevice, OpaqueID: "dev-002", UserCount: 2, Weight: 3},
                {Kind: IdentifierIP, OpaqueID: "ip-002", UserCount: 2, Weight: 1},
            },
            RiskScore: 4,
        },
    },
    IsolatedUsers: []string{"usr-006"},
}
```

transitive linkage, shared payment-token weight 5, evidence에서 제외되는 unique identifier,
reversed/shuffled record, equal score/user-count smallest-ID tie-break, duplicate read edge,
wrong direction, unknown endpoint, wrong label, missing/typed-wrong property, empty graph에 대한 table case를 추가한다.

- [ ] **Step 2: RED 확인**

```bash
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'TestAnalyze'
```

기대값: `Analyze`, `Report`, `Cluster`, `Evidence`가 구현되지 않았으므로 FAIL한다.

- [ ] **Step 3: minimum analyzer 구현**

exact JSON tag와 함께 report type을 `model.go`에 추가한다.

```go
type Evidence struct {
    Kind      IdentifierKind `json:"kind"`
    OpaqueID  string         `json:"opaque_id"`
    UserCount int            `json:"user_count"`
    Weight    int            `json:"weight"`
}

type Cluster struct {
    ClusterID string     `json:"cluster_id"`
    Users     []string   `json:"users"`
    Evidence  []Evidence `json:"evidence"`
    RiskScore int        `json:"risk_score"`
}

type Report struct {
    Clusters      []Cluster `json:"clusters"`
    IsolatedUsers []string  `json:"isolated_users"`
}
```

`analyzer.go`에서 backend `graph.ElementID`를 validated user/identifier node로 map한다.
모든 relationship이 User에서 Identifier로 향하는 `USES_IDENTIFIER`인지 validate하고 duplicate logical relationship을 reject한다.
양방향 adjacency를 만들고 sorted user ID마다 iterative BFS를 실행한다. distinct shared identifier를 count하고 weight 5/3/1을 적용하며 spec이 요구한 대로 정확히 sort한다.
empty valid graph에는 non-nil empty slice를 반환해 JSON이 `null`이 아니라 `[]`를 emit하게 한다.

- [ ] **Step 4: GREEN 확인 및 input-order independence 반복 증명**

```bash
gofmt -w examples/graph-abuse-cluster/internal/abusecluster
go test -count=10 ./examples/graph-abuse-cluster/internal/abusecluster -run 'TestAnalyze'
go test -race -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'TestAnalyze'
```

기대값: 모든 ordering에서 exact report가 pass하고 race가 finding을 보고하지 않는다.

- [ ] **Step 5: Task 2 commit**

```bash
git add examples/graph-abuse-cluster/internal/abusecluster/model.go examples/graph-abuse-cluster/internal/abusecluster/analyzer.go examples/graph-abuse-cluster/internal/abusecluster/analyzer_test.go
git commit -m "feat: analyze graph abuse clusters"
```

## Task 3: Add the Atomic Bounded Neo4j Store

**Complexity:** High. **Depends on:** Tasks 1-2. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** store and
pure store validation tests; the real container test remains Task 6.

- [ ] **Step 1: Write RED tests for parameter conversion and read bounds**

Create `store_test.go` in the same package. Test pure helpers behind the public
store methods:

```go
func fixtureParameters(Fixture) (map[string]any, error)
func validateLoadedGraph([]graph.Vertex, []graph.Edge) error
```

Require parameter maps to contain only `fixture_id`, `users`, `identifiers`,
and `edges`; no raw Cypher or provider error retains opaque values. Require
257 vertices and 1025 edges to return `ErrGraphTooLarge`, while exact limits
pass. Require an invalid loaded edge to return `ErrInvalidGraph`.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(FixtureParameters|ValidateLoadedGraph|StoreRejectsNil)'
```

Expected: FAIL because `Store` and helper functions do not exist.

- [ ] **Step 3: Implement Store and fixed Cypher constants**

Create `store.go` with:

```go
type Store struct { client *neo4jgraph.Client }

func NewStore(client *neo4jgraph.Client) (*Store, error)
func (s *Store) ReplaceFixture(ctx context.Context, fixture Fixture) error
func (s *Store) LoadFixture(ctx context.Context, fixtureID string) ([]graph.Vertex, []graph.Edge, error)
```

`ReplaceFixture` validates before I/O and executes one fixed statement through
the released adapter's `ExecuteWrite`, whose managed transaction covers the
entire reset and seed. Use this shape so an empty namespace still produces one
row and reaches the create clauses:

```cypher
OPTIONAL MATCH (stale {fixture_id: $fixture_id})
WITH [node IN collect(stale) WHERE node IS NOT NULL] AS stale_nodes
FOREACH (node IN stale_nodes | DETACH DELETE node)
WITH size(stale_nodes) AS deleted
FOREACH (user IN $users |
  CREATE (:User {opaque_id: user.opaque_id, fixture_id: $fixture_id}))
FOREACH (identifier IN $identifiers |
  CREATE (:Identifier {
    opaque_id: identifier.opaque_id,
    kind: identifier.kind,
    fixture_id: $fixture_id
  }))
WITH deleted
UNWIND $edges AS edge
MATCH (user:User {opaque_id: edge.start, fixture_id: $fixture_id})
MATCH (identifier:Identifier {opaque_id: edge.end, fixture_id: $fixture_id})
CREATE (user)-[:USES_IDENTIFIER {
  opaque_id: edge.opaque_id,
  fixture_id: $fixture_id
}]->(identifier)
```

The two reads are fixed, parameterized, ordered, and request limit+1:

```cypher
MATCH (n {fixture_id: $fixture_id})
WHERE n:User OR n:Identifier
RETURN n ORDER BY elementId(n) LIMIT $limit
```

```cypher
MATCH ()-[r:USES_IDENTIFIER {fixture_id: $fixture_id}]->()
RETURN r ORDER BY elementId(r) LIMIT $limit
```

Use `ReadVertices(..., "n")` and `ReadEdges(..., "r")`; preserve
`context.Canceled`/`DeadlineExceeded` through `%w`, map other provider failures
to `ErrBackend`, and never include query text, parameters, URI, or opaque IDs in
the rendered operation error.

- [ ] **Step 4: Observe GREEN**

```bash
gofmt -w examples/graph-abuse-cluster/internal/abusecluster
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(FixtureParameters|ValidateLoadedGraph|StoreRejectsNil)'
go vet ./examples/graph-abuse-cluster/internal/abusecluster
```

Expected: PASS and limit+1 is rejected before analysis.

- [ ] **Step 5: Commit Task 3**

```bash
git add examples/graph-abuse-cluster/internal/abusecluster/store.go examples/graph-abuse-cluster/internal/abusecluster/store_test.go
git commit -m "feat: persist graph abuse fixture in neo4j"
```

## Task 4: Compose the Workflow and Buffered Output

**Complexity:** Medium. **Depends on:** Tasks 2-3. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** workflow,
output and their tests.

- [ ] **Step 1: Write RED workflow tests with a narrow unexported seam**

Define the test seam in `workflow.go`:

```go
type WorkflowBackend interface {
    ReplaceFixture(context.Context, Fixture) error
    LoadFixture(context.Context, string) ([]graph.Vertex, []graph.Edge, error)
}

func Execute(ctx context.Context, backend WorkflowBackend, fixture Fixture) (Report, error)
```

`workflow_test.go` must prove replace -> load -> analyze order, exact report,
no load after replace failure, no analysis after load failure, and preservation
of caller cancellation without retry or late store calls. Keep this two-method
interface inside the example's `internal` package; it is a use-case test seam,
not a reusable graph repository abstraction.

- [ ] **Step 2: Write RED buffered-output tests**

Plan the exact function:

```go
func EncodeReport(Report) ([]byte, error)
```

`output_test.go` compares the bytes to the exact standard-library
`json.MarshalIndent` representation shown in the spec, requires one trailing
newline, and requires deterministic repetition. Add the
small package-private seam below solely to inject a marshaler error and prove
that an encoding failure returns no bytes:

```go
type marshalIndentFunc func(any, string, string) ([]byte, error)

func encodeReport(report Report, marshal marshalIndentFunc) ([]byte, error)
```

`EncodeReport` delegates to `encodeReport(report, json.MarshalIndent)`; neither
function writes to an `io.Writer`.

- [ ] **Step 3: Observe RED**

```bash
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(Execute|EncodeReport)'
```

Expected: FAIL because workflow and output functions do not exist.

- [ ] **Step 4: Implement the minimal workflow and encoder**

Normalize nil context to `context.Background` only at the top-level CLI; the
internal workflow rejects nil store/invalid fixture and otherwise passes the
caller context unchanged. Use `json.MarshalIndent`, append exactly one newline,
and return bytes only after complete encoding.

- [ ] **Step 5: Observe GREEN and commit**

```bash
gofmt -w examples/graph-abuse-cluster/internal/abusecluster
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(Execute|EncodeReport)'
git add examples/graph-abuse-cluster/internal/abusecluster/workflow.go examples/graph-abuse-cluster/internal/abusecluster/workflow_test.go examples/graph-abuse-cluster/internal/abusecluster/output.go examples/graph-abuse-cluster/internal/abusecluster/output_test.go
git commit -m "feat: run graph abuse cluster workflow"
```

## Task 5: Build the Strict Loopback CLI and Lifecycle

**Complexity:** High. **Depends on:** Task 4. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** executable
and executable tests.

- [ ] **Step 1: Write RED configuration tests**

Create `main_test.go` for:

```go
type appConfig struct { neo4jURI string }
func loadConfig(getenv func(string) string) (appConfig, error)
```

Accept `bolt://localhost:7687`, `bolt://127.0.0.1:7687`,
`bolt://127.9.8.7:7687`, and `bolt://[::1]:7687`. Reject nil getenv, missing
URI, `neo4j://`, HTTP, DNS host, non-loopback IP, userinfo, path, query,
fragment, missing/non-numeric/zero/>65535 port, and whitespace-smuggled values.
Assert errors do not contain the supplied URI.

- [ ] **Step 2: Write RED lifecycle and output-order tests**

Use small function seams rather than a global mutable hook. The opened
application owns the released client/store/fixture composition, so `run` can
test every stage without trying to recover a store from a close-only owner:

```go
type application interface {
    VerifyConnectivity(context.Context) error
    Execute(context.Context) (abusecluster.Report, error)
    Close(context.Context) error
}

type openApplicationFunc func(appConfig) (application, error)
type encodeReportFunc func(abusecluster.Report) ([]byte, error)

func run(
    ctx context.Context,
    cfg appConfig,
    stdout io.Writer,
    open openApplicationFunc,
    encode encodeReportFunc,
) error
```

The fake application records events. Require:

```text
open -> verify -> replace -> load -> analyze -> encode -> close -> stdout
```

Cover open/verify/workflow/encode/close/write failures, caller cancellation,
one 15-second operation deadline, fresh 3-second cleanup context, cleanup
attempt after every successful open, no stdout before close, and redacted
errors. Fake failures include a secret URI and opaque ID; neither may appear in
the returned/logged message. A short writer must return `io.ErrShortWrite` and
non-zero behavior.

Test the process boundary through:

```go
func realMain(
    ctx context.Context,
    getenv func(string) string,
    stdout io.Writer,
    stderr io.Writer,
    open openApplicationFunc,
    encode encodeReportFunc,
) int
```

It returns 0 only for a complete run and 1 for every classified failure, so
`main` is only signal setup plus `os.Exit(realMain(...))`.

- [ ] **Step 3: Observe RED**

```bash
go test -count=1 ./examples/graph-abuse-cluster -run 'Test(LoadConfig|Run)'
```

Expected: FAIL because executable behavior is absent.

- [ ] **Step 4: Implement `main.go`**

Use `net/url`, `net.ParseIP`, `IP.IsLoopback`, `strconv.Atoi`, and an exact
case-insensitive `localhost` check. Reject every URI component not allowed by
the spec. Construct:

```go
driver, err := neo4jdriver.NewDriver(cfg.neo4jURI, neo4jdriver.NoAuth())
client, err := neo4jgraph.NewClient(driver)
store, err := abusecluster.NewStore(client)
```

Wrap those concrete values in a small `neo4jApplication` that implements the
three-method seam and calls `abusecluster.Execute` with the store and fixture.
`main` uses `signal.NotifyContext`, derives one 15-second operation context,
calls `run` with the production opener and encoder, logs only
`application failed` plus stable `stage`/`class`, and exits 1. `run` encodes to
memory, explicitly closes with
`context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)`, then performs
one checked full write to stdout. Keep a deferred close fallback protected by
a local `closeAttempted` flag set immediately before the close call so every
successful open results in exactly one close attempt, including close failure.

- [ ] **Step 5: Observe GREEN and run static checks**

```bash
gofmt -w examples/graph-abuse-cluster
go test -count=1 ./examples/graph-abuse-cluster -run 'Test(LoadConfig|Run)'
go test -race -count=1 ./examples/graph-abuse-cluster -run 'Test(LoadConfig|Run)'
go vet ./examples/graph-abuse-cluster/...
```

Expected: PASS with exact lifecycle ordering and no leaked URI/provider value.

- [ ] **Step 6: Commit Task 5**

```bash
git add examples/graph-abuse-cluster/main.go examples/graph-abuse-cluster/main_test.go
git commit -m "feat: add graph abuse cluster cli"
```

## Task 6: Prove the Real Neo4j Boundary Serially

**Complexity:** High. **Depends on:** Tasks 1-5. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** one
integration test. **Heavy-command limit:** one Neo4j container at a time.

- [ ] **Step 1: Write the Testcontainers test**

Create `integration_test.go` with one named test:

```go
func TestGraphAbuseClusterWithNeo4j(t *testing.T)
```

Start `tcneo4j.Run(ctx, "neo4j:5.26.0")`, register termination immediately,
obtain `BoltUrl`, parse it and replace only its `neo4j` scheme with `bolt`, then
pass the result through `loadConfig` before creating
`neo4jdriver.NewDriver(..., NoAuth())`. Construct the released adapter and call
`VerifyConnectivity` under a bounded context before any assertion. This makes
the real test exercise the same strict loopback boundary even though the
Testcontainers helper names its endpoint `neo4j://`.

Build a unique test fixture with `NewFixture(t.Name()+uniqueSuffix)`. Assert:

- unrelated sentinel namespace survives both replacements;
- first replace/load/analyze equals the exact approved report;
- second replace produces exactly 12 vertices and 10 edges, not duplicates;
- adapter read values include backend ElementIDs and correct graph labels;
- a pre-canceled context returns `context.Canceled` and makes no late mutation;
- cleanup deletes only the test namespace and closes driver/container under
  fresh bounded contexts.

- [ ] **Step 2: Observe RED for missing or incorrect real behavior**

```bash
go test -p 1 -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
```

Expected before repairs: FAIL at the first store/query/lifecycle mismatch, not
a skipped or log-readiness-only result.

- [ ] **Step 3: Make the minimum store/CLI corrections and observe GREEN**

Only edit prior task-owned files when the real backend proves a contract gap.
Then run:

```bash
go test -p 1 -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
go test -p 1 -race -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
```

Expected: both PASS from fresh container runs with observed exit 0.

- [ ] **Step 4: Run the complete focused package gate**

```bash
go test -count=1 ./examples/graph-abuse-cluster/...
go test -race -count=1 ./examples/graph-abuse-cluster/...
```

Expected: pure and real-service tests PASS; no parallel second container is
started.

- [ ] **Step 5: Commit Task 6**

```bash
git add examples/graph-abuse-cluster
git commit -m "test: verify graph abuse cluster with neo4j"
```

## Task 7: Write the Bilingual Runnable Lesson and Root Navigation

**Complexity:** Medium. **Depends on:** Task 6. **Skills:**
`bluetape-writer`, `bluetape-go-patterns`. **Write scope:** README pair and
root navigation only.

- [ ] **Step 1: Write `README.md` from verified source behavior**

Include the language switch, lesson, component boundary, vertex/edge schema,
exact score table, exact JSON output, atomic reset/seed versus non-atomic
delivery non-applicability, loopback trust boundary, unsupported concurrent CLI
runs, production non-goals, and these runnable commands. Label the long-running
container command as Terminal 1 and the CLI/test commands as Terminal 2:

```bash
docker run --rm --name graph-abuse-cluster-neo4j \
  -e NEO4J_AUTH=none \
  -p 127.0.0.1:7687:7687 \
  neo4j:5.26.0

NEO4J_URI=bolt://127.0.0.1:7687 \
  go run ./examples/graph-abuse-cluster

go test -count=1 ./examples/graph-abuse-cluster/...
go test -p 1 -race -count=1 ./examples/graph-abuse-cluster/...
```

State that the risk score is illustrative, identifiers are synthetic opaque
fixtures, the process supports one run per fixed namespace, and production
auth/TLS/authorization/retention/fraud policy are absent.

- [ ] **Step 2: Write natural source-equivalent `README.ko.md`**

Keep terms, numbers, commands, output, diagrams, caveats, and section order
aligned. Use practical Korean engineering prose, not literal English sentence
structure. The current locale is plain text in each language switch.

- [ ] **Step 3: Update both root README files**

Add one row after the audit/outbox track and one run section before the next
unrelated service family. Both root files link their matching locale and name
`graph`, `graph/neo4j`, and Neo4j Testcontainers without claiming Memgraph.

- [ ] **Step 4: Add README parity tests**

Add `documentation_test.go` in the example package. Parse both README sources
and require exact presence/parity for `NEO4J_URI`, `NEO4J_AUTH=none`, loopback
publish, the 5/3/1 weights, 256/1024 limits, fixed namespace concurrency caveat,
expected cluster IDs, test commands, and both future diagram paths.

- [ ] **Step 5: Validate and commit prose**

```bash
go test -count=1 ./examples/graph-abuse-cluster -run '^TestDocumentationParity$'
rg -n 'graph-abuse-cluster|NEO4J_URI|risk_score|payment_token|Memgraph' README.md README.ko.md examples/graph-abuse-cluster/README.md examples/graph-abuse-cluster/README.ko.md
git diff --check
git add README.md README.ko.md examples/graph-abuse-cluster/README.md examples/graph-abuse-cluster/README.ko.md examples/graph-abuse-cluster/documentation_test.go
git commit -m "docs: explain graph abuse cluster example"
```

Expected: locale contracts are source-equivalent, root navigation works, and
Memgraph appears only as explicitly excluded scope if mentioned at all.

## Task 8: Create Architecture and Sequence Diagrams One at a Time

**Complexity:** High. **Depends on:** Task 7 and verified source. **Skills:**
`bluetape-diagram`. **Write scope:** four canonical assets, README embeds,
diagram evidence ledger. **Rule:** complete the full one-asset loop before
editing the second SVG.

- [ ] **Step 1: Pin diagram sources and references**

Read the final source and both example READMEs. Open at original size:

- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/graph-graph-core-architecture-01.png`;
- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png`;
- `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster/docs/images/readme-diagrams/audited-order-workflow-outbox-architecture.png`;
- `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster/docs/images/readme-diagrams/audited-order-workflow-outbox-sequence.png`.

Record exact reference paths. Architecture answers ownership/responsibility;
sequence answers time-ordered lifecycle. Use catalog Neo4j/database icon only
if an exact verified icon exists; otherwise use a text-only Neo4j card.

- [ ] **Step 2: Complete the architecture SVG -> PNG loop**

Create:

```text
docs/images/readme-diagrams/graph-abuse-cluster-architecture.svg
docs/images/readme-diagrams/graph-abuse-cluster-architecture.png
```

Show fixed fixture validation, caller-owned driver plus released adapter,
atomic reset/seed, Neo4j, two bounded reads, Go analyzer, and JSON report as
static responsibilities. Use orthogonal rounded connectors, 14x14 primary
arrowheads, perpendicular ports, corner/marker clearance, no unexplained
colors, and balanced whitespace.

Run XML, CairoSVG scale-2 render, connector, geometry with `--fail-diagonal`,
endpoint, and mixed-corner audits. Require meaningful nonzero card/connector
counts and failures=0. Open the final PNG at original size and inspect every
arrowhead direction, bend clearance, line/card intrusion, label, icon, and
canvas edge after the last coordinate change.

```bash
ARCH=docs/images/readme-diagrams/graph-abuse-cluster-architecture
xmllint --noout "$ARCH.svg"
cairosvg "$ARCH.svg" -o "$ARCH.png" -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" "$ARCH.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal "$ARCH.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" "$ARCH.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" "$ARCH.svg"
```

- [ ] **Step 3: Complete the sequence SVG -> PNG loop**

Create:

```text
docs/images/readme-diagrams/graph-abuse-cluster-sequence.svg
docs/images/readme-diagrams/graph-abuse-cluster-sequence.png
```

Participants: CLI, Fixture Validator, `graph/neo4j`, Neo4j, Go Analyzer, JSON
Output. Number every visible message with the approved 34px pill and 13px
semantic-color circular badge. Show config validation, connectivity, atomic
reset/seed, vertex read, edge read, limit validation, analysis, encode, close,
and output, plus transparent error/cancellation branch frames. Use 16x16
message arrowheads and 6-12px label-to-line gaps.

Run the common audits plus `diagram-sequence-style-audit.py`. Require numbered
label count to match the source-ledger message count, palette/marker parity,
transparent branch frames, deterministic rerender, and original-size PNG eye
inspection. Explicitly compare call-number style against the best-practices
reference rather than older inline-number repo assets.

```bash
SEQ=docs/images/readme-diagrams/graph-abuse-cluster-sequence
xmllint --noout "$SEQ.svg"
cairosvg "$SEQ.svg" -o "$SEQ.png" -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" "$SEQ.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal "$SEQ.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" "$SEQ.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" "$SEQ.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" "$SEQ.svg"
```

- [ ] **Step 4: Prove deterministic rendering and README exposure**

For each asset rerender to `/tmp`, require `cmp` exit 0, record dimensions and
SHA-256, embed the PNG in both locale READMEs, and verify canonical SVG/PNG
paths. Run:

```bash
ARCH=docs/images/readme-diagrams/graph-abuse-cluster-architecture
SEQ=docs/images/readme-diagrams/graph-abuse-cluster-sequence
cairosvg "$ARCH.svg" -o /tmp/graph-abuse-cluster-architecture.png -s 2
cmp "$ARCH.png" /tmp/graph-abuse-cluster-architecture.png
cairosvg "$SEQ.svg" -o /tmp/graph-abuse-cluster-sequence.png -s 2
cmp "$SEQ.png" /tmp/graph-abuse-cluster-sequence.png
sips -g pixelWidth -g pixelHeight "$ARCH.png" "$SEQ.png"
shasum -a 256 "$ARCH.svg" "$ARCH.png" "$SEQ.svg" "$SEQ.png"
```

```bash
git diff --check -- \
  docs/images/readme-diagrams/graph-abuse-cluster-architecture.svg \
  docs/images/readme-diagrams/graph-abuse-cluster-architecture.png \
  docs/images/readme-diagrams/graph-abuse-cluster-sequence.svg \
  docs/images/readme-diagrams/graph-abuse-cluster-sequence.png \
  examples/graph-abuse-cluster/README.md \
  examples/graph-abuse-cluster/README.ko.md
```

- [ ] **Step 5: Commit diagrams**

```bash
git add docs/images/readme-diagrams/graph-abuse-cluster-* examples/graph-abuse-cluster/README.md examples/graph-abuse-cluster/README.ko.md
git commit -m "docs: diagram graph abuse cluster workflow"
```

## Task 9: Run Full Verification, Review, and Durable Learning

**Complexity:** High. **Depends on:** Tasks 1-8. **Skills:**
`verification-before-completion`, `bluetape-full-feature`,
`bluetape-go-patterns`. **Write scope:** in-scope P0/P1 fixes, review and lesson
artifacts. **Heavy commands:** sequential only.

- [ ] **Step 1: Run focused and repository gates from scratch**

```bash
git diff --check origin/develop
gofmt -l examples/graph-abuse-cluster
go test -count=1 ./examples/graph-abuse-cluster/...
go test -race -count=1 ./examples/graph-abuse-cluster/...
go test -p 1 -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
go test -p 1 -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

Expected: every command has a fresh observed exit 0. If a container test fails
then passes on retry, diagnose lifecycle/cleanup/host contention, repair the
cause, rerun the isolated test, then rerun the full required gate; a retry-only
pass is not accepted.

- [ ] **Step 2: Execute performance/stability and dependency scans**

Verify fixed three-operation DB path, 256/1024 caps, iterative traversal,
bounded buffers, no per-node query, strict deadline propagation, exact close
ownership, no late mutation on cancellation, scoped transaction rollback,
single container startup, direct dependency versions/licenses, and no raw
provider/URI logging. Fix P0/P1 and rerun affected tests.

- [ ] **Step 3: Verify every spec and plan item**

Use the Type A Step 5 verifier against the exact spec, this plan, current diff,
tests, README pair, root navigation, four diagram assets, and issue metadata.
Map every acceptance criterion to fresh evidence; missing behavior returns to
its owning task and is never reinterpreted as optional.

- [ ] **Step 4: Run six final code-review perspectives**

Review performance, stability, security, operator/Ops, developer/API, and
user/caller independently, then integrate in the main session. Fix P0/P1,
resolve or file P2/P3, rerun affected tests/lenses, and write
`docs/review/2026-07-14-issue-50-graph-abuse-cluster.md` with exact commands,
diagram counts/hashes/dimensions, dependency evidence, and final P0=0/P1=0.

- [ ] **Step 5: Write and commit the durable lesson**

Record the Neo4j/Go policy split, ElementID versus opaque ID mapping, atomic
reset/seed, limit+1 proof, strict NoAuth loopback boundary, cleanup-before-
output, Testcontainers behavior, diagram eye-check findings, review misses, and
the #27 body-edit fail-closed restoration guard.

```bash
git add docs/review/2026-07-14-issue-50-graph-abuse-cluster.md docs/lessons/2026-07-14-issue-50-graph-abuse-cluster.md
git commit -m "docs: record graph abuse cluster verification"
git status --short
```

Expected: clean worktree, no unresolved P0/P1, and all evidence committed
before PR creation.

## Task 10: Push, Create the PR, and Stop at Merge Approval

**Complexity:** Medium. **Depends on:** Task 9. **Skills:**
`bluetape-workflow`. **Write scope:** live PR metadata/body only unless review
requires an approved repair.

- [ ] **Step 1: Refresh live issue metadata and branch state**

Confirm #50 is OPEN, assigned to `debop`, milestone `0.10.0`, labels
`enhancement`/`examples`, parent #36 remains open, worktree is clean, and branch
head equals the intended pushed head.

- [ ] **Step 2: Push and create the English PR**

Push only `feat/issue-50-graph-abuse-cluster`. Create an English PR that closes
#50, mirrors milestone/assignee/labels, explains why before what, lists exact
validation and diagram eye evidence, and ends with final heading
`## DoD Status`. Verify the live body and metadata with `gh pr view`.

- [ ] **Step 3: Run live PR review and CI gate**

Review the actual PR diff through all six perspectives. Wait in bounded
intervals until every required check is `SUCCESS`; pending, skipped, stale,
missing, canceled, or failed is not green. After green, reread reviews,
comments, and unresolved threads; any new P0/P1 reopens implementation.

- [ ] **Step 4: Report merge-ready state and stop**

Report PR URL, head/base/CI SHAs, issue metadata, P0/P1 counts, required checks,
diagram paths, and clean local state. Do not merge, delete the branch/worktree,
close #36, or synchronize `develop` without a new explicit user instruction.

## Acceptance Traceability

| Spec acceptance | Owning tasks | Fresh evidence |
|---|---|---|
| Released graph-value validation before persistence | Task 1 | Fixture RED/GREEN and graph validation tests |
| Atomic scoped Neo4j reset/seed and two bounded reads | Tasks 3, 6 | Store tests plus real Neo4j idempotence/unrelated-namespace proof |
| Exact direct/transitive clusters, scores and tie-breaks | Task 2 | Exact report, shuffle, payment weight and tie tables |
| Fail-closed invalid/bounds/backend/cancellation behavior | Tasks 2-6 | Negative, cancellation, lifecycle and integration tests |
| Strict loopback NoAuth CLI and cleanup-before-output | Task 5 | URI matrix and event-order tests |
| Runnable deterministic JSON lesson | Tasks 4-7 | Exact encoder, CLI and README parity tests |
| Serial Neo4j readiness/adaptation/cleanup | Task 6 | Normal and race Testcontainers commands |
| Bilingual docs/root navigation/visuals | Tasks 7-8 | Locale parity plus two complete diagram ledgers |
| Full validation and P0/P1 convergence | Task 9 | Fresh focused, repository, make CI, verifier and review evidence |
| PR metadata and green live CI | Task 10 | `gh` live body/metadata/check/review evidence |

## Plan Review Convergence

The main session reviewed this plan independently through all six required
perspectives after the approved spec. Each P1 below was repaired in the plan
before implementation approval; no P0 was found.

| Perspective | Initial finding | Plan repair | Final |
|---|---|---|---|
| Performance | P1: loaded-result caps existed, but the pre-write fixture boundary did not explicitly test exact/max+1 sizes. | Task 1 now rejects write-side 257/1025 inputs and proves 256/1024 acceptance; Tasks 3 and 9 retain max+1 reads and the fixed three-operation path. | P0=0, P1=0 |
| Stability | P1: the seed shape and close flag could fail on an empty namespace or retry a failed close; integration namespace creation was implicit. | Task 3 uses `OPTIONAL MATCH` plus one managed transaction, Task 5 marks `closeAttempted` before close, and Tasks 1/6 define `NewFixture` for isolated runs. | P0=0, P1=0 |
| Security | P1: URI rejection was broad, but rendered provider-secret leakage lacked an explicit adversarial assertion. | Task 5 injects URI/opaque-ID-bearing failures and requires returned/logged redaction; strict loopback parsing remains before driver creation. | P0=0, P1=0 |
| Operator/Ops | P1: lifecycle tests did not own the actual process exit-code boundary. | Task 5 defines injectable `realMain`, verifies 0/1 behavior, one operation deadline, fresh cleanup, and only then leaves `main` to signal setup and `os.Exit`. | P0=0, P1=0 |
| Developer/API | P1: the original opener returned only a close owner and could not supply the workflow store; the unique fixture API was missing. | Task 5 now opens one three-method application, Task 1 owns `NewFixture`, Task 4 has a concrete encoder-failure seam, and Task 8 gives executable audit commands. | P0=0, P1=0 |
| User/caller | P1: the foreground container and CLI commands could look like one terminal flow, and visual references were directory-level. | Task 7 labels Terminal 1/2; Task 8 pins four full reference paths and preserves exact JSON, caveats, and original-size eye checks. | P0=0, P1=0 |

## Risk Prediction and Rerun Points

| Risk | Signal | Mitigation | Rerun or rollback point |
|---|---|---|---|
| Cypher reset leaves partial fixture | counts differ after injected/second seed | one scoped managed transaction and exact real counts | reopen Task 3, rerun Task 6 from fresh container |
| Adapter ElementID is confused with opaque ID | endpoint lookup or output IDs drift | map backend IDs only for topology; domain output uses validated properties | reopen Task 2 and shuffle/integration tests |
| Limit query silently truncates | exactly 256/1024 passes but larger graph reports success | query max+1 and reject before analysis | reopen Task 3 boundary tests |
| NoAuth URI reaches remote host | hostname/userinfo/scheme matrix unexpectedly passes | strict parsed loopback Bolt validation before driver | reopen Task 5 config tests/security lens |
| Cleanup failure follows successful output | stdout event precedes close or close error still prints JSON | buffer, explicit close, then checked stdout write | reopen Tasks 4-5 lifecycle tests |
| Two processes race on fixed namespace | inconsistent vertices/edges or missing endpoint | explicit single-run boundary; unique namespaces in tests | stop concurrent run, rerun Task 6 serially |
| Docker host contention creates flaky CI | readiness timeout or retry-only pass | one serial container, connectivity proof, bounded cleanup | diagnose, isolated rerun, then full Task 9 gates |
| Diagram arrows pass scripts but render reversed/colliding | PNG differs from intended receiver or bend | PNG authoritative; move ports/bends and rerun one-asset loop | return to owning Task 8 asset |
| Scope grows into graph abstraction or Memgraph | new package/interface/dependency outside map | stop and request new scope; preserve issue #50 boundary | revert only new task-owned drift |

## Repository Hazard Decisions

- New module/CI/Nightly/Kover/BOM/catalog: N/A; this is an example directory in
  the existing Go module and current workflows discover `./...`.
- Dependency change: triggered and pinned to upstream v0.18.0 versions; Task 1
  plus `tidy-check`, license/source review, and no-extra-dependency diff prove it.
- HTTP/auth/TLS: no HTTP server; strict local NoAuth Bolt configuration is
  intentionally narrow and documented as non-production.
- Testcontainers: triggered; one Neo4j container at a time with connectivity,
  cancellation, scoped cleanup, normal and race evidence.
- README locales and diagrams: triggered; Tasks 7-8 own parity and visual
  ledgers.
- Benchmark: N/A; no throughput claim, fixed tiny fixture, bounded operations,
  and this issue is a teaching example rather than performance evidence.
- Release/changelog: N/A; workshop delivery is issue/README/PR based and does
  not publish a library artifact.
