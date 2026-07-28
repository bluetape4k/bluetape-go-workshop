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

## Task 3: Atomic Bounded Neo4j Store 추가

**Complexity:** High. **Depends on:** Tasks 1-2. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** store와 pure store validation test를 포함한다.
real container test는 Task 6에 남긴다.

- [ ] **Step 1: parameter conversion 및 read bound에 대한 RED test 작성**

같은 package에 `store_test.go`를 만든다. public store method 뒤의 pure helper를 테스트한다.

```go
func fixtureParameters(Fixture) (map[string]any, error)
func validateLoadedGraph([]graph.Vertex, []graph.Edge) error
```

parameter map에는 `fixture_id`, `users`, `identifiers`, `edges`만 포함되어야 한다.
raw Cypher나 provider error가 opaque value를 보존하면 안 된다. vertex 257개와 edge 1025개는 `ErrGraphTooLarge`를 반환해야 하며,
exact limit은 pass해야 한다. invalid loaded edge는 `ErrInvalidGraph`를 반환해야 한다.

- [ ] **Step 2: RED 확인**

```bash
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(FixtureParameters|ValidateLoadedGraph|StoreRejectsNil)'
```

기대값: `Store`와 helper function이 없으므로 FAIL한다.

- [ ] **Step 3: Store 및 fixed Cypher constant 구현**

`store.go`를 다음과 함께 만든다.

```go
type Store struct { client *neo4jgraph.Client }

func NewStore(client *neo4jgraph.Client) (*Store, error)
func (s *Store) ReplaceFixture(ctx context.Context, fixture Fixture) error
func (s *Store) LoadFixture(ctx context.Context, fixtureID string) ([]graph.Vertex, []graph.Edge, error)
```

`ReplaceFixture`는 I/O 전에 validate하고 released adapter의 `ExecuteWrite`를 통해 fixed statement 하나를 실행한다.
managed transaction은 reset과 seed 전체를 감싼다. empty namespace도 row 하나를 만들고 create clause에 도달하도록 다음 shape를 사용한다.

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

두 read는 fixed, parameterized, ordered이며 limit+1을 요청한다.

```cypher
MATCH (n {fixture_id: $fixture_id})
WHERE n:User OR n:Identifier
RETURN n ORDER BY elementId(n) LIMIT $limit
```

```cypher
MATCH ()-[r:USES_IDENTIFIER {fixture_id: $fixture_id}]->()
RETURN r ORDER BY elementId(r) LIMIT $limit
```

`ReadVertices(..., "n")`와 `ReadEdges(..., "r")`를 사용한다.
`context.Canceled`/`DeadlineExceeded`는 `%w`로 보존하고, 다른 provider failure는 `ErrBackend`로 map한다.
rendered operation error에는 query text, parameter, URI, opaque ID를 절대 포함하지 않는다.

- [ ] **Step 4: GREEN 확인**

```bash
gofmt -w examples/graph-abuse-cluster/internal/abusecluster
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(FixtureParameters|ValidateLoadedGraph|StoreRejectsNil)'
go vet ./examples/graph-abuse-cluster/internal/abusecluster
```

기대값: PASS하고 limit+1은 analysis 전에 reject된다.

- [ ] **Step 5: Task 3 commit**

```bash
git add examples/graph-abuse-cluster/internal/abusecluster/store.go examples/graph-abuse-cluster/internal/abusecluster/store_test.go
git commit -m "feat: persist graph abuse fixture in neo4j"
```

## Task 4: Workflow 및 Buffered Output 구성

**Complexity:** Medium. **Depends on:** Tasks 2-3. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** workflow, output 및 해당 test.

- [ ] **Step 1: narrow unexported seam으로 RED workflow test 작성**

`workflow.go`에 test seam을 정의한다.

```go
type WorkflowBackend interface {
    ReplaceFixture(context.Context, Fixture) error
    LoadFixture(context.Context, string) ([]graph.Vertex, []graph.Edge, error)
}

func Execute(ctx context.Context, backend WorkflowBackend, fixture Fixture) (Report, error)
```

`workflow_test.go`는 replace -> load -> analyze 순서, exact report, replace failure 뒤 load 없음,
load failure 뒤 analysis 없음, retry나 late store call 없이 caller cancellation 보존을 증명해야 한다.
이 two-method interface는 example의 `internal` package 안에 둔다. reusable graph repository abstraction이 아니라 use-case test seam이다.

- [ ] **Step 2: RED buffered-output test 작성**

exact function은 다음과 같다.

```go
func EncodeReport(Report) ([]byte, error)
```

`output_test.go`는 byte를 spec에 표시된 exact standard-library `json.MarshalIndent` representation과 비교하고,
trailing newline 하나와 deterministic repetition을 요구한다. marshaler error를 inject하고 encoding failure가 byte를 반환하지 않음을 증명하기 위한 작은 package-private seam을 추가한다.

```go
type marshalIndentFunc func(any, string, string) ([]byte, error)

func encodeReport(report Report, marshal marshalIndentFunc) ([]byte, error)
```

`EncodeReport`는 `encodeReport(report, json.MarshalIndent)`에 delegate한다. 두 function 모두 `io.Writer`에 쓰지 않는다.

- [ ] **Step 3: RED 확인**

```bash
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(Execute|EncodeReport)'
```

기대값: workflow 및 output function이 없으므로 FAIL한다.

- [ ] **Step 4: minimal workflow 및 encoder 구현**

nil context는 top-level CLI에서만 `context.Background`로 normalize한다. internal workflow는 nil store/invalid fixture를 reject하고,
그 외에는 caller context를 변경하지 않고 전달한다. `json.MarshalIndent`를 사용하고 정확히 newline 하나를 append하며,
complete encoding 이후에만 byte를 반환한다.

- [ ] **Step 5: GREEN 확인 및 commit**

```bash
gofmt -w examples/graph-abuse-cluster/internal/abusecluster
go test -count=1 ./examples/graph-abuse-cluster/internal/abusecluster -run 'Test(Execute|EncodeReport)'
git add examples/graph-abuse-cluster/internal/abusecluster/workflow.go examples/graph-abuse-cluster/internal/abusecluster/workflow_test.go examples/graph-abuse-cluster/internal/abusecluster/output.go examples/graph-abuse-cluster/internal/abusecluster/output_test.go
git commit -m "feat: run graph abuse cluster workflow"
```

## Task 5: Strict Loopback CLI 및 Lifecycle 작성

**Complexity:** High. **Depends on:** Task 4. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** executable 및 executable test.

- [ ] **Step 1: RED configuration test 작성**

다음을 위한 `main_test.go`를 만든다.

```go
type appConfig struct { neo4jURI string }
func loadConfig(getenv func(string) string) (appConfig, error)
```

`bolt://localhost:7687`, `bolt://127.0.0.1:7687`, `bolt://127.9.8.7:7687`,
`bolt://[::1]:7687`를 accept한다. nil getenv, missing URI, `neo4j://`, HTTP, DNS host,
non-loopback IP, userinfo, path, query, fragment, missing/non-numeric/zero/>65535 port,
whitespace-smuggled value를 reject한다. error에 supplied URI가 포함되지 않는지 assert한다.

- [ ] **Step 2: RED lifecycle 및 output-order test 작성**

global mutable hook 대신 small function seam을 사용한다. opened application이 released client/store/fixture composition을 소유하므로,
`run`은 close-only owner에서 store를 복구하려 하지 않고 모든 stage를 test할 수 있다.

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

fake application은 event를 기록한다. 다음을 요구한다.

```text
open -> verify -> replace -> load -> analyze -> encode -> close -> stdout
```

open/verify/workflow/encode/close/write failure, caller cancellation, 15-second operation deadline 하나,
fresh 3-second cleanup context, 모든 successful open 뒤 cleanup attempt, close 전 stdout 없음, redacted error를 cover한다.
fake failure에는 secret URI와 opaque ID가 포함된다. 반환/logged message에는 둘 다 나타나면 안 된다.
short writer는 `io.ErrShortWrite`와 non-zero behavior를 반환해야 한다.

process boundary는 다음으로 테스트한다.

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

complete run에서만 0을 반환하고 모든 classified failure에서는 1을 반환한다. 따라서 `main`은 signal setup과 `os.Exit(realMain(...))`만 담당한다.

- [ ] **Step 3: RED 확인**

```bash
go test -count=1 ./examples/graph-abuse-cluster -run 'Test(LoadConfig|Run)'
```

기대값: executable behavior가 없으므로 FAIL한다.

- [ ] **Step 4: `main.go` 구현**

`net/url`, `net.ParseIP`, `IP.IsLoopback`, `strconv.Atoi`, exact case-insensitive `localhost` check를 사용한다.
spec이 허용하지 않는 모든 URI component를 reject한다. 다음을 구성한다.

```go
driver, err := neo4jdriver.NewDriver(cfg.neo4jURI, neo4jdriver.NoAuth())
client, err := neo4jgraph.NewClient(driver)
store, err := abusecluster.NewStore(client)
```

이 concrete value를 three-method seam을 구현하는 작은 `neo4jApplication`으로 감싸고 store와 fixture로 `abusecluster.Execute`를 호출한다.
`main`은 `signal.NotifyContext`를 사용하고 15-second operation context 하나를 파생하며, production opener와 encoder로 `run`을 호출한다.
log에는 `application failed`와 stable `stage`/`class`만 남기고 exit 1로 끝난다. `run`은 memory로 encode하고,
`context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)`로 명시적으로 close한 뒤 stdout에 checked full write를 한 번 수행한다.
close call 직전에 설정되는 local `closeAttempted` flag로 deferred close fallback을 보호해 close failure를 포함한 모든 successful open이 정확히 한 번의 close attempt를 만들게 한다.

- [ ] **Step 5: GREEN 확인 및 static check 실행**

```bash
gofmt -w examples/graph-abuse-cluster
go test -count=1 ./examples/graph-abuse-cluster -run 'Test(LoadConfig|Run)'
go test -race -count=1 ./examples/graph-abuse-cluster -run 'Test(LoadConfig|Run)'
go vet ./examples/graph-abuse-cluster/...
```

기대값: exact lifecycle ordering과 leaked URI/provider value 없이 PASS한다.

- [ ] **Step 6: Task 5 commit**

```bash
git add examples/graph-abuse-cluster/main.go examples/graph-abuse-cluster/main_test.go
git commit -m "feat: add graph abuse cluster cli"
```

## Task 6: Real Neo4j Boundary를 serial하게 증명

**Complexity:** High. **Depends on:** Tasks 1-5. **Skills:**
`test-driven-development`, `bluetape-go-patterns`. **Write scope:** integration test 하나.
**Heavy-command limit:** 한 번에 Neo4j container 하나.

- [ ] **Step 1: Testcontainers test 작성**

named test 하나를 가진 `integration_test.go`를 만든다.

```go
func TestGraphAbuseClusterWithNeo4j(t *testing.T)
```

`tcneo4j.Run(ctx, "neo4j:5.26.0")`를 시작하고 즉시 termination을 register한다.
`BoltUrl`을 얻어 parse하고 `neo4j` scheme만 `bolt`로 바꾼 뒤, `neo4jdriver.NewDriver(..., NoAuth())`를 만들기 전에
결과를 `loadConfig`에 통과시킨다. released adapter를 구성하고 어떤 assertion보다 먼저 bounded context에서 `VerifyConnectivity`를 호출한다.
Testcontainers helper가 endpoint를 `neo4j://`로 명명하더라도 real test가 같은 strict loopback boundary를 실행하게 한다.

`NewFixture(t.Name()+uniqueSuffix)`로 unique test fixture를 만든다. 다음을 assert한다.

- unrelated sentinel namespace가 두 replacement 뒤에도 남는다.
- 첫 replace/load/analyze가 exact approved report와 같다.
- 두 번째 replace는 duplicate가 아니라 정확히 vertex 12개와 edge 10개를 만든다.
- adapter read value는 backend ElementID와 올바른 graph label을 포함한다.
- pre-canceled context는 `context.Canceled`를 반환하고 late mutation을 만들지 않는다.
- cleanup은 test namespace만 삭제하고 fresh bounded context 아래에서 driver/container를 close한다.

- [ ] **Step 2: missing 또는 incorrect real behavior에 대한 RED 확인**

```bash
go test -p 1 -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
```

repair 전 기대값: skipped 또는 log-readiness-only result가 아니라 첫 store/query/lifecycle mismatch에서 FAIL한다.

- [ ] **Step 3: minimum store/CLI correction 수행 및 GREEN 확인**

real backend가 contract gap을 증명할 때만 이전 task-owned file을 수정한다. 그다음 실행한다.

```bash
go test -p 1 -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
go test -p 1 -race -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'
```

기대값: fresh container run에서 둘 다 observed exit 0으로 PASS한다.

- [ ] **Step 4: complete focused package gate 실행**

```bash
go test -count=1 ./examples/graph-abuse-cluster/...
go test -race -count=1 ./examples/graph-abuse-cluster/...
```

기대값: pure 및 real-service test가 PASS한다. parallel second container는 시작되지 않는다.

- [ ] **Step 5: Task 6 commit**

```bash
git add examples/graph-abuse-cluster
git commit -m "test: verify graph abuse cluster with neo4j"
```

## Task 7: Bilingual Runnable Lesson 및 Root Navigation 작성

**Complexity:** Medium. **Depends on:** Task 6. **Skills:**
`bluetape-writer`, `bluetape-go-patterns`. **Write scope:** README pair와 root navigation만 포함한다.

- [ ] **Step 1: verified source behavior 기준으로 `README.md` 작성**

language switch, lesson, component boundary, vertex/edge schema, exact score table, exact JSON output,
atomic reset/seed와 non-atomic delivery non-applicability의 차이, loopback trust boundary,
unsupported concurrent CLI run, production non-goal, 다음 runnable command를 포함한다.
long-running container command는 Terminal 1로, CLI/test command는 Terminal 2로 label한다.

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

risk score는 illustrative이며 identifier는 synthetic opaque fixture라고 명시한다. process는 fixed namespace마다 한 run만 지원하고,
production auth/TLS/authorization/retention/fraud policy는 없다고 쓴다.

- [ ] **Step 2: 자연스러운 source-equivalent `README.ko.md` 작성**

term, number, command, output, diagram, caveat, section order를 aligned로 유지한다.
literal English sentence structure가 아니라 실용적인 한국어 engineering prose를 사용한다.
각 language switch에서 현재 locale은 plain text로 표시한다.

- [ ] **Step 3: 두 root README file 업데이트**

audit/outbox track 뒤에 row 하나를 추가하고, 다음 unrelated service family 앞에 run section 하나를 추가한다.
두 root file은 matching locale로 link하고, Memgraph를 claim하지 않으면서 `graph`, `graph/neo4j`,
Neo4j Testcontainers를 명명한다.

- [ ] **Step 4: README parity test 추가**

example package에 `documentation_test.go`를 추가한다. 두 README source를 parse하고 `NEO4J_URI`,
`NEO4J_AUTH=none`, loopback publish, 5/3/1 weight, 256/1024 limit, fixed namespace concurrency caveat,
expected cluster ID, test command, 두 future diagram path의 exact presence/parity를 요구한다.

- [ ] **Step 5: prose 검증 및 commit**

```bash
go test -count=1 ./examples/graph-abuse-cluster -run '^TestDocumentationParity$'
rg -n 'graph-abuse-cluster|NEO4J_URI|risk_score|payment_token|Memgraph' README.md README.ko.md examples/graph-abuse-cluster/README.md examples/graph-abuse-cluster/README.ko.md
git diff --check
git add README.md README.ko.md examples/graph-abuse-cluster/README.md examples/graph-abuse-cluster/README.ko.md examples/graph-abuse-cluster/documentation_test.go
git commit -m "docs: explain graph abuse cluster example"
```

기대값: locale contract가 source-equivalent이고 root navigation이 동작한다.
Memgraph가 언급된다면 explicitly excluded scope로만 나타난다.

## Task 8: Architecture 및 Sequence Diagram을 하나씩 생성

**Complexity:** High. **Depends on:** Task 7 and verified source. **Skills:**
`bluetape-diagram`. **Write scope:** canonical asset 네 개, README embed, diagram evidence ledger.
**Rule:** 두 번째 SVG를 편집하기 전에 첫 asset의 full loop를 완료한다.

- [ ] **Step 1: diagram source 및 reference 고정**

final source와 두 example README를 읽는다. original size로 연다.

- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/graph-graph-core-architecture-01.png`;
- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png`;
- `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster/docs/images/readme-diagrams/audited-order-workflow-outbox-architecture.png`;
- `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster/docs/images/readme-diagrams/audited-order-workflow-outbox-sequence.png`.

exact reference path를 기록한다. Architecture는 ownership/responsibility에 답하고,
sequence는 time-ordered lifecycle에 답한다. exact verified icon이 있을 때만 catalog Neo4j/database icon을 사용하고,
그렇지 않으면 text-only Neo4j card를 사용한다.

- [ ] **Step 2: architecture SVG -> PNG loop 완료**

Create:

```text
docs/images/readme-diagrams/graph-abuse-cluster-architecture.svg
docs/images/readme-diagrams/graph-abuse-cluster-architecture.png
```

fixed fixture validation, caller-owned driver와 released adapter, atomic reset/seed, Neo4j,
bounded read 두 개, Go analyzer, JSON report를 static responsibility로 보여준다.
orthogonal rounded connector, 14x14 primary arrowhead, perpendicular port, corner/marker clearance,
설명 없는 color 없음, balanced whitespace를 사용한다.

XML, CairoSVG scale-2 render, connector, `--fail-diagonal` geometry, endpoint, mixed-corner audit를 실행한다.
의미 있는 nonzero card/connector count와 failures=0을 요구한다. 마지막 coordinate 변경 뒤 final PNG를 original size로 열고
모든 arrowhead direction, bend clearance, line/card intrusion, label, icon, canvas edge를 검사한다.

```bash
ARCH=docs/images/readme-diagrams/graph-abuse-cluster-architecture
xmllint --noout "$ARCH.svg"
cairosvg "$ARCH.svg" -o "$ARCH.png" -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" "$ARCH.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal "$ARCH.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" "$ARCH.svg"
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" "$ARCH.svg"
```

- [ ] **Step 3: sequence SVG -> PNG loop 완료**

Create:

```text
docs/images/readme-diagrams/graph-abuse-cluster-sequence.svg
docs/images/readme-diagrams/graph-abuse-cluster-sequence.png
```

participant는 CLI, Fixture Validator, `graph/neo4j`, Neo4j, Go Analyzer, JSON Output이다.
approved 34px pill과 13px semantic-color circular badge로 모든 visible message에 번호를 붙인다.
config validation, connectivity, atomic reset/seed, vertex read, edge read, limit validation,
analysis, encode, close, output, transparent error/cancellation branch frame을 보여준다.
16x16 message arrowhead와 6-12px label-to-line gap을 사용한다.

common audit와 `diagram-sequence-style-audit.py`를 실행한다. numbered label count는 source-ledger message count와 맞아야 한다.
palette/marker parity, transparent branch frame, deterministic rerender, original-size PNG eye inspection을 요구한다.
call-number style은 오래된 inline-number repo asset이 아니라 best-practices reference와 명시적으로 비교한다.

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

- [ ] **Step 4: deterministic rendering 및 README exposure 증명**

각 asset을 `/tmp`에 rerender하고 `cmp` exit 0을 요구한다. dimension과 SHA-256을 기록하고,
PNG를 두 locale README에 embed하며 canonical SVG/PNG path를 검증한다. 실행한다.

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

- [ ] **Step 5: diagram commit**

```bash
git add docs/images/readme-diagrams/graph-abuse-cluster-* examples/graph-abuse-cluster/README.md examples/graph-abuse-cluster/README.ko.md
git commit -m "docs: diagram graph abuse cluster workflow"
```

## Task 9: Full Verification, Review, Durable Learning 실행

**Complexity:** High. **Depends on:** Tasks 1-8. **Skills:**
`verification-before-completion`, `bluetape-full-feature`, `bluetape-go-patterns`.
**Write scope:** in-scope P0/P1 fix, review 및 lesson artifact.
**Heavy commands:** sequential only.

- [ ] **Step 1: focused 및 repository gate를 처음부터 실행**

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

기대값: 모든 command가 fresh observed exit 0을 가진다. container test가 fail한 뒤 retry에서 pass하면
lifecycle/cleanup/host contention을 진단하고 원인을 수정한다. isolated test를 다시 실행한 뒤 full required gate를 다시 실행한다.
retry-only pass는 수락하지 않는다.

- [ ] **Step 2: performance/stability 및 dependency scan 실행**

fixed three-operation DB path, 256/1024 cap, iterative traversal, bounded buffer, per-node query 없음,
strict deadline propagation, exact close ownership, cancellation 뒤 late mutation 없음, scoped transaction rollback,
single container startup, direct dependency version/license, raw provider/URI logging 없음 을 확인한다.
P0/P1을 수정하고 affected test를 다시 실행한다.

- [ ] **Step 3: 모든 spec 및 plan item 검증**

exact spec, 이 plan, current diff, test, README pair, root navigation, diagram asset 네 개, issue metadata에 대해
Type A Step 5 verifier를 사용한다. 모든 acceptance criterion을 fresh evidence에 매핑한다.
missing behavior는 owning task로 되돌아가며 optional로 재해석하지 않는다.

- [ ] **Step 4: final code-review perspective 여섯 개 실행**

performance, stability, security, operator/Ops, developer/API, user/caller를 독립적으로 review한 뒤 main session에서 통합한다.
P0/P1을 수정하고 P2/P3를 resolve 또는 file하며 affected test/lens를 다시 실행한다.
exact command, diagram count/hash/dimension, dependency evidence, final P0=0/P1=0을 포함해
`docs/review/2026-07-14-issue-50-graph-abuse-cluster.md`를 작성한다.

- [ ] **Step 5: durable lesson 작성 및 commit**

Neo4j/Go policy split, ElementID와 opaque ID mapping, atomic reset/seed, limit+1 proof,
strict NoAuth loopback boundary, cleanup-before-output, Testcontainers behavior,
diagram eye-check finding, review miss, #27 body-edit fail-closed restoration guard를 기록한다.

```bash
git add docs/review/2026-07-14-issue-50-graph-abuse-cluster.md docs/lessons/2026-07-14-issue-50-graph-abuse-cluster.md
git commit -m "docs: record graph abuse cluster verification"
git status --short
```

기대값: clean worktree, unresolved P0/P1 없음, PR 생성 전 모든 evidence commit 완료.

## Task 10: Push, PR 생성, Merge Approval에서 중단

**Complexity:** Medium. **Depends on:** Task 9. **Skills:**
`bluetape-workflow`. **Write scope:** review가 approved repair를 요구하지 않는 한 live PR metadata/body만 포함한다.

- [ ] **Step 1: live issue metadata 및 branch state refresh**

#50이 OPEN이고 `debop`에게 assigned되었으며 milestone `0.10.0`, label `enhancement`/`examples`를 갖는지 확인한다.
parent #36은 open으로 남아야 한다. worktree가 clean이고 branch head가 intended pushed head와 같은지 확인한다.

- [ ] **Step 2: push 및 English PR 생성**

`feat/issue-50-graph-abuse-cluster`만 push한다. #50을 close하고 milestone/assignee/label을 mirror하는 English PR을 생성한다.
what보다 why를 먼저 설명하고 exact validation과 diagram eye evidence를 나열하며 final heading `## DoD Status`로 끝낸다.
`gh pr view`로 live body와 metadata를 확인한다.

- [ ] **Step 3: live PR review 및 CI gate 실행**

actual PR diff를 여섯 perspective 모두로 review한다. 모든 required check가 `SUCCESS`가 될 때까지 bounded interval로 기다린다.
pending, skipped, stale, missing, canceled, failed는 green이 아니다. green 이후 review, comment, unresolved thread를 다시 읽는다.
새 P0/P1은 implementation을 다시 연다.

- [ ] **Step 4: merge-ready state 보고 및 중단**

PR URL, head/base/CI SHA, issue metadata, P0/P1 count, required check, diagram path, clean local state를 보고한다.
새 명시적 사용자 지시 없이 merge, branch/worktree 삭제, #36 close, `develop` synchronize를 하지 않는다.

## Acceptance Traceability

| Spec acceptance | Owning tasks | Fresh evidence |
|---|---|---|
| Released graph-value validation before persistence | Task 1 | Fixture RED/GREEN 및 graph validation test |
| Atomic scoped Neo4j reset/seed and two bounded reads | Tasks 3, 6 | store test와 real Neo4j idempotence/unrelated-namespace proof |
| Exact direct/transitive clusters, scores and tie-breaks | Task 2 | exact report, shuffle, payment weight, tie table |
| Fail-closed invalid/bounds/backend/cancellation behavior | Tasks 2-6 | negative, cancellation, lifecycle, integration test |
| Strict loopback NoAuth CLI and cleanup-before-output | Task 5 | URI matrix 및 event-order test |
| Runnable deterministic JSON lesson | Tasks 4-7 | exact encoder, CLI, README parity test |
| Serial Neo4j readiness/adaptation/cleanup | Task 6 | normal 및 race Testcontainers command |
| Bilingual docs/root navigation/visuals | Tasks 7-8 | locale parity와 complete diagram ledger 두 개 |
| Full validation and P0/P1 convergence | Task 9 | fresh focused, repository, make CI, verifier, review evidence |
| PR metadata and green live CI | Task 10 | `gh` live body/metadata/check/review evidence |

## Plan Review Convergence

main session은 approved spec 이후 여섯 required perspective로 이 plan을 독립 review했다.
아래 각 P1은 implementation approval 전에 plan에서 수정되었고 P0는 발견되지 않았다.

| Perspective | Initial finding | Plan repair | Final |
|---|---|---|---|
| Performance | P1: loaded-result cap은 있었지만 pre-write fixture boundary가 exact/max+1 size를 명시적으로 test하지 않았다. | Task 1은 write-side 257/1025 input을 reject하고 256/1024 acceptance를 증명한다. Tasks 3/9는 max+1 read와 fixed three-operation path를 유지한다. | P0=0, P1=0 |
| Stability | P1: seed shape와 close flag가 empty namespace에서 실패하거나 failed close를 retry할 수 있었다. integration namespace creation은 암묵적이었다. | Task 3은 `OPTIONAL MATCH`와 managed transaction 하나를 사용한다. Task 5는 close 전에 `closeAttempted`를 mark하고, Tasks 1/6은 isolated run용 `NewFixture`를 정의한다. | P0=0, P1=0 |
| Security | P1: URI rejection은 넓었지만 rendered provider-secret leakage에 명시적인 adversarial assertion이 부족했다. | Task 5는 URI/opaque-ID-bearing failure를 inject하고 returned/logged redaction을 요구한다. strict loopback parsing은 driver creation 전에 남는다. | P0=0, P1=0 |
| Operator/Ops | P1: lifecycle test가 실제 process exit-code boundary를 소유하지 않았다. | Task 5는 injectable `realMain`을 정의하고 0/1 behavior, operation deadline 하나, fresh cleanup을 검증한 뒤 `main`을 signal setup과 `os.Exit`에만 남긴다. | P0=0, P1=0 |
| Developer/API | P1: original opener가 close owner만 반환하여 workflow store를 제공할 수 없었고 unique fixture API가 없었다. | Task 5는 three-method application 하나를 열고, Task 1은 `NewFixture`를 소유하며, Task 4는 concrete encoder-failure seam을 갖고, Task 8은 executable audit command를 제공한다. | P0=0, P1=0 |
| User/caller | P1: foreground container와 CLI command가 하나의 terminal flow처럼 보일 수 있었고 visual reference가 directory-level이었다. | Task 7은 Terminal 1/2를 label하고, Task 8은 full reference path 네 개를 pin하며 exact JSON, caveat, original-size eye check를 보존한다. | P0=0, P1=0 |

## Risk Prediction and Rerun Points

| Risk | Signal | Mitigation | Rerun or rollback point |
|---|---|---|---|
| Cypher reset leaves partial fixture | injected/second seed 뒤 count가 다름 | scoped managed transaction 하나와 exact real count | Task 3 재개방, fresh container에서 Task 6 재실행 |
| Adapter ElementID is confused with opaque ID | endpoint lookup 또는 output ID drift | backend ID는 topology에만 map하고 domain output은 validated property 사용 | Task 2와 shuffle/integration test 재개방 |
| Limit query silently truncates | 정확히 256/1024는 pass하지만 더 큰 graph가 success를 보고 | query max+1과 analysis 전 reject | Task 3 boundary test 재개방 |
| NoAuth URI reaches remote host | hostname/userinfo/scheme matrix가 예상 밖으로 pass | driver 전 strict parsed loopback Bolt validation | Task 5 config test/security lens 재개방 |
| Cleanup failure follows successful output | stdout event가 close보다 앞서거나 close error에도 JSON 출력 | buffer, explicit close, 그다음 checked stdout write | Tasks 4-5 lifecycle test 재개방 |
| Two processes race on fixed namespace | inconsistent vertices/edges 또는 missing endpoint | explicit single-run boundary, test의 unique namespace | concurrent run 중단, Task 6 serial 재실행 |
| Docker host contention creates flaky CI | readiness timeout 또는 retry-only pass | serial container 하나, connectivity proof, bounded cleanup | 진단, isolated rerun, 이후 full Task 9 gate |
| Diagram arrows pass scripts but render reversed/colliding | PNG가 intended receiver 또는 bend와 다름 | PNG를 authoritative로 보고 port/bend 이동 후 one-asset loop 재실행 | owning Task 8 asset으로 복귀 |
| Scope grows into graph abstraction or Memgraph | map 밖의 new package/interface/dependency | 중단하고 new scope 요청, issue #50 boundary 보존 | task-owned drift만 revert |

## Repository Hazard Decisions

- New module/CI/Nightly/Kover/BOM/catalog: N/A. existing Go module 안의 example directory이며 current workflow가 `./...`를 discover한다.
- Dependency change: trigger됨. upstream v0.18.0 version에 pin한다. Task 1과 `tidy-check`, license/source review, no-extra-dependency diff가 이를 증명한다.
- HTTP/auth/TLS: HTTP server 없음. strict local NoAuth Bolt configuration은 의도적으로 좁고 non-production으로 문서화한다.
- Testcontainers: trigger됨. 한 번에 Neo4j container 하나를 사용하고 connectivity, cancellation, scoped cleanup, normal/race evidence를 남긴다.
- README locales and diagrams: trigger됨. Tasks 7-8이 parity와 visual ledger를 소유한다.
- Benchmark: N/A. throughput claim이 없고 fixed tiny fixture와 bounded operation만 있으며, 이 issue는 performance evidence가 아니라 teaching example이다.
- Release/changelog: N/A. workshop delivery는 issue/README/PR 기반이며 library artifact를 publish하지 않는다.
