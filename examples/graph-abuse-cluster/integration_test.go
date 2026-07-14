package main

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-abuse-cluster/internal/abusecluster"
	"github.com/bluetape4k/bluetape-go/graph"
	neo4jgraph "github.com/bluetape4k/bluetape-go/graph/neo4j"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	tcneo4j "github.com/testcontainers/testcontainers-go/modules/neo4j"
)

const integrationCleanupTimeout = 30 * time.Second

func TestGraphAbuseClusterWithNeo4j(t *testing.T) {
	testCtx, cancelTest := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelTest()

	container, err := tcneo4j.Run(testCtx, "neo4j:5.26.0")
	if err != nil {
		t.Fatalf("start Neo4j container: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(testCtx), integrationCleanupTimeout)
		defer cancelCleanup()
		if err := container.Terminate(cleanupCtx); err != nil {
			t.Errorf("terminate Neo4j container: %v", err)
		}
	})

	boltURL, err := container.BoltUrl(testCtx)
	if err != nil {
		t.Fatalf("get Neo4j Bolt URL: %v", err)
	}
	parsed, err := url.Parse(boltURL)
	if err != nil {
		t.Fatalf("parse Neo4j Bolt URL: %v", err)
	}
	if parsed.Scheme != "neo4j" {
		t.Fatalf("Testcontainers Bolt URL scheme = %q, want neo4j", parsed.Scheme)
	}
	parsed.Scheme = "bolt"
	strictBoltURL := parsed.String()
	cfg, err := loadConfig(func(key string) string {
		if key != neo4jURIEnvironment {
			t.Fatalf("loadConfig getenv key = %q, want %q", key, neo4jURIEnvironment)
		}
		return strictBoltURL
	})
	if err != nil {
		t.Fatalf("load strict Testcontainers Bolt URL: %v", err)
	}

	driver, err := neo4jdriver.NewDriver(cfg.neo4jURI, neo4jdriver.NoAuth())
	if err != nil {
		t.Fatalf("create Neo4j driver: %v", err)
	}
	client, err := neo4jgraph.NewClient(driver)
	if err != nil {
		closeDriver(driver)
		t.Fatalf("create bluetape-go Neo4j client: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(testCtx), cleanupTimeout)
		defer cancelCleanup()
		if err := client.Close(cleanupCtx); err != nil {
			t.Errorf("close Neo4j client: %v", err)
		}
	})

	connectivityCtx, cancelConnectivity := context.WithTimeout(testCtx, operationTimeout)
	err = client.VerifyConnectivity(connectivityCtx)
	cancelConnectivity()
	if err != nil {
		t.Fatalf("verify real Neo4j connectivity: %v", err)
	}

	store, err := abusecluster.NewStore(client)
	if err != nil {
		t.Fatalf("create graph abuse cluster store: %v", err)
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	fixtureID := strings.ReplaceAll(t.Name(), "/", "-") + "-" + suffix
	sentinelID := fixtureID + "-sentinel"
	canceledID := fixtureID + "-canceled"
	fixture, err := abusecluster.NewFixture(fixtureID)
	if err != nil {
		t.Fatalf("build unique fixture: %v", err)
	}
	canceledFixture, err := abusecluster.NewFixture(canceledID)
	if err != nil {
		t.Fatalf("build canceled fixture: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(testCtx), cleanupTimeout)
		defer cancelCleanup()
		if err := deleteFixtureNamespaces(cleanupCtx, client, []string{fixtureID, sentinelID, canceledID}); err != nil {
			t.Errorf("clean test fixture namespaces: %v", err)
		}
	})

	operationCtx, cancelOperation := context.WithTimeout(testCtx, integrationCleanupTimeout)
	defer cancelOperation()
	if err := createSentinel(operationCtx, client, sentinelID); err != nil {
		t.Fatalf("create unrelated sentinel namespace: %v", err)
	}

	if err := store.ReplaceFixture(operationCtx, fixture); err != nil {
		t.Fatalf("first ReplaceFixture() error = %v", err)
	}
	vertices, edges, err := store.LoadFixture(operationCtx, fixture.ID)
	if err != nil {
		t.Fatalf("first LoadFixture() error = %v", err)
	}
	report, err := abusecluster.Analyze(vertices, edges)
	if err != nil {
		t.Fatalf("first Analyze() error = %v", err)
	}
	if want := approvedIntegrationReport(); !reflect.DeepEqual(report, want) {
		t.Fatalf("first report = %#v, want %#v", report, want)
	}
	assertBackendGraphValues(t, vertices, edges)
	assertNamespaceVertexCount(t, operationCtx, client, sentinelID, 1)

	if err := store.ReplaceFixture(operationCtx, fixture); err != nil {
		t.Fatalf("second ReplaceFixture() error = %v", err)
	}
	vertices, edges, err = store.LoadFixture(operationCtx, fixture.ID)
	if err != nil {
		t.Fatalf("second LoadFixture() error = %v", err)
	}
	if got, want := len(vertices), 12; got != want {
		t.Fatalf("vertices after second replace = %d, want %d without duplicates", got, want)
	}
	if got, want := len(edges), 10; got != want {
		t.Fatalf("edges after second replace = %d, want %d without duplicates", got, want)
	}
	secondReport, err := abusecluster.Analyze(vertices, edges)
	if err != nil {
		t.Fatalf("second Analyze() error = %v", err)
	}
	if want := approvedIntegrationReport(); !reflect.DeepEqual(secondReport, want) {
		t.Fatalf("second report = %#v, want %#v", secondReport, want)
	}
	assertNamespaceVertexCount(t, operationCtx, client, sentinelID, 1)

	canceledCtx, cancelCanceled := context.WithCancel(operationCtx)
	cancelCanceled()
	if err := store.ReplaceFixture(canceledCtx, canceledFixture); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReplaceFixture(pre-canceled) error = %v, want context.Canceled", err)
	}
	assertNamespaceVertexCount(t, operationCtx, client, canceledID, 0)
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-operationCtx.Done():
		t.Fatalf("waiting for late mutation proof: %v", operationCtx.Err())
	case <-timer.C:
	}
	assertNamespaceVertexCount(t, operationCtx, client, canceledID, 0)

	if err := deleteFixtureNamespaces(operationCtx, client, []string{fixtureID}); err != nil {
		t.Fatalf("delete only primary fixture namespace: %v", err)
	}
	assertNamespaceVertexCount(t, operationCtx, client, fixtureID, 0)
	assertNamespaceVertexCount(t, operationCtx, client, sentinelID, 1)
}

func approvedIntegrationReport() abusecluster.Report {
	return abusecluster.Report{
		Clusters: []abusecluster.Cluster{
			{
				ClusterID: "cluster:usr-001",
				Users:     []string{"usr-001", "usr-002", "usr-003"},
				Evidence: []abusecluster.Evidence{
					{Kind: abusecluster.IdentifierDevice, OpaqueID: "dev-001", UserCount: 2, Weight: 3},
					{Kind: abusecluster.IdentifierIP, OpaqueID: "ip-001", UserCount: 2, Weight: 1},
				},
				RiskScore: 4,
			},
			{
				ClusterID: "cluster:usr-004",
				Users:     []string{"usr-004", "usr-005"},
				Evidence: []abusecluster.Evidence{
					{Kind: abusecluster.IdentifierDevice, OpaqueID: "dev-002", UserCount: 2, Weight: 3},
					{Kind: abusecluster.IdentifierIP, OpaqueID: "ip-002", UserCount: 2, Weight: 1},
				},
				RiskScore: 4,
			},
		},
		IsolatedUsers: []string{"usr-006"},
	}
}

func assertBackendGraphValues(t *testing.T, vertices []graph.Vertex, edges []graph.Edge) {
	t.Helper()
	labels := map[string]int{}
	for _, vertex := range vertices {
		backendID := vertex.ID().String()
		opaqueID, _ := vertex.Properties()["opaque_id"].(string)
		if backendID == "" || backendID == opaqueID {
			t.Fatalf("vertex backend ElementID = %q, opaque ID = %q", backendID, opaqueID)
		}
		labels[vertex.Label().String()]++
	}
	if labels["User"] != 6 || labels["Identifier"] != 6 || len(labels) != 2 {
		t.Fatalf("vertex labels = %#v, want User=6 Identifier=6 only", labels)
	}
	for _, edge := range edges {
		backendID := edge.ID().String()
		opaqueID, _ := edge.Properties()["opaque_id"].(string)
		if backendID == "" || backendID == opaqueID {
			t.Fatalf("edge backend ElementID = %q, opaque ID = %q", backendID, opaqueID)
		}
		if edge.Label().String() != "USES_IDENTIFIER" {
			t.Fatalf("edge label = %q, want USES_IDENTIFIER", edge.Label())
		}
		if edge.StartID().String() == "" || edge.EndID().String() == "" {
			t.Fatalf("edge backend endpoints = %q -> %q, want populated ElementIDs", edge.StartID(), edge.EndID())
		}
	}
}

func assertNamespaceVertexCount(t *testing.T, ctx context.Context, client *neo4jgraph.Client, fixtureID string, want int) {
	t.Helper()
	vertices, err := client.ReadVertices(ctx,
		`MATCH (n {fixture_id: $fixture_id}) RETURN n ORDER BY elementId(n)`,
		map[string]any{"fixture_id": fixtureID},
		"n",
	)
	if err != nil {
		t.Fatalf("read namespace %q: %v", fixtureID, err)
	}
	if got := len(vertices); got != want {
		t.Fatalf("namespace %q vertex count = %d, want %d", fixtureID, got, want)
	}
}

func createSentinel(ctx context.Context, client *neo4jgraph.Client, fixtureID string) error {
	return client.ExecuteWrite(ctx,
		`CREATE (:Sentinel {opaque_id: $opaque_id, fixture_id: $fixture_id})`,
		map[string]any{"opaque_id": "sentinel", "fixture_id": fixtureID},
	)
}

func deleteFixtureNamespaces(ctx context.Context, client *neo4jgraph.Client, fixtureIDs []string) error {
	return client.ExecuteWrite(ctx,
		`MATCH (n) WHERE n.fixture_id IN $fixture_ids DETACH DELETE n`,
		map[string]any{"fixture_ids": fixtureIDs},
	)
}
