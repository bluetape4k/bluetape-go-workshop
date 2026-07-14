package abusecluster

import (
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestAnalyzeDefaultFixture(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}

	got, err := Analyze(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	want := Report{
		Clusters: []Cluster{
			{
				ClusterID: "cluster:usr-001",
				Users:     []string{"usr-001", "usr-002", "usr-003"},
				Evidence: []Evidence{
					{Kind: IdentifierDevice, OpaqueID: "dev-001", UserCount: 2, Weight: 3},
					{Kind: IdentifierIP, OpaqueID: "ip-001", UserCount: 2, Weight: 1},
				},
				RiskScore: 4,
			},
			{
				ClusterID: "cluster:usr-004",
				Users:     []string{"usr-004", "usr-005"},
				Evidence: []Evidence{
					{Kind: IdentifierDevice, OpaqueID: "dev-002", UserCount: 2, Weight: 3},
					{Kind: IdentifierIP, OpaqueID: "ip-002", UserCount: 2, Weight: 1},
				},
				RiskScore: 4,
			},
		},
		IsolatedUsers: []string{"usr-006"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestAnalyzeUsesBackendElementIDsOnlyForTopology(t *testing.T) {
	const fixtureID = "backend-loaded"
	vertices := []graph.Vertex{
		analyzerVertex(t, "backend-user-z", "User", "usr-z", fixtureID, ""),
		analyzerVertex(t, "backend-user-a", "User", "usr-a", fixtureID, ""),
		analyzerVertex(t, "backend-identifier", "Identifier", "dev-shared", fixtureID, IdentifierDevice),
	}
	edges := []graph.Edge{
		analyzerEdge(t, "backend-edge-z", "uses-z", "backend-user-z", "backend-identifier", fixtureID),
		analyzerEdge(t, "backend-edge-a", "uses-a", "backend-user-a", "backend-identifier", fixtureID),
	}

	got, err := Analyze(vertices, edges)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	want := Report{
		Clusters: []Cluster{{
			ClusterID: "cluster:usr-a",
			Users:     []string{"usr-a", "usr-z"},
			Evidence:  []Evidence{{Kind: IdentifierDevice, OpaqueID: "dev-shared", UserCount: 2, Weight: 3}},
			RiskScore: 3,
		}},
		IsolatedUsers: []string{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestAnalyzeFindsTransitiveClustersAndScoresSharedEvidenceOnce(t *testing.T) {
	const fixtureID = "transitive"
	vertices := []graph.Vertex{
		analyzerVertex(t, "u-a", "User", "usr-a", fixtureID, ""),
		analyzerVertex(t, "u-b", "User", "usr-b", fixtureID, ""),
		analyzerVertex(t, "u-c", "User", "usr-c", fixtureID, ""),
		analyzerVertex(t, "i-device", "Identifier", "dev-shared", fixtureID, IdentifierDevice),
		analyzerVertex(t, "i-payment", "Identifier", "pay-shared", fixtureID, IdentifierPaymentToken),
		analyzerVertex(t, "i-unique", "Identifier", "ip-unique", fixtureID, IdentifierIP),
	}
	edges := []graph.Edge{
		analyzerEdge(t, "e-1", "uses-1", "u-a", "i-device", fixtureID),
		analyzerEdge(t, "e-2", "uses-2", "u-b", "i-device", fixtureID),
		analyzerEdge(t, "e-3", "uses-3", "u-b", "i-payment", fixtureID),
		analyzerEdge(t, "e-4", "uses-4", "u-c", "i-payment", fixtureID),
		analyzerEdge(t, "e-5", "uses-5", "u-c", "i-unique", fixtureID),
	}

	got, err := Analyze(vertices, edges)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	want := Report{
		Clusters: []Cluster{{
			ClusterID: "cluster:usr-a",
			Users:     []string{"usr-a", "usr-b", "usr-c"},
			Evidence: []Evidence{
				{Kind: IdentifierDevice, OpaqueID: "dev-shared", UserCount: 2, Weight: 3},
				{Kind: IdentifierPaymentToken, OpaqueID: "pay-shared", UserCount: 2, Weight: 5},
			},
			RiskScore: 8,
		}},
		IsolatedUsers: []string{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestAnalyzeSortsEqualClustersBySmallestUserID(t *testing.T) {
	const fixtureID = "tie-break"
	vertices := []graph.Vertex{
		analyzerVertex(t, "u-z1", "User", "usr-z1", fixtureID, ""),
		analyzerVertex(t, "u-z2", "User", "usr-z2", fixtureID, ""),
		analyzerVertex(t, "u-a1", "User", "usr-a1", fixtureID, ""),
		analyzerVertex(t, "u-a2", "User", "usr-a2", fixtureID, ""),
		analyzerVertex(t, "i-z", "Identifier", "ip-z", fixtureID, IdentifierIP),
		analyzerVertex(t, "i-a", "Identifier", "ip-a", fixtureID, IdentifierIP),
	}
	edges := []graph.Edge{
		analyzerEdge(t, "e-z1", "uses-z1", "u-z1", "i-z", fixtureID),
		analyzerEdge(t, "e-z2", "uses-z2", "u-z2", "i-z", fixtureID),
		analyzerEdge(t, "e-a1", "uses-a1", "u-a1", "i-a", fixtureID),
		analyzerEdge(t, "e-a2", "uses-a2", "u-a2", "i-a", fixtureID),
	}

	got, err := Analyze(vertices, edges)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got.Clusters[0].ClusterID != "cluster:usr-a1" || got.Clusters[1].ClusterID != "cluster:usr-z1" {
		t.Fatalf("cluster order = %v, want smallest user ID tie-break", got.Clusters)
	}
}

func TestAnalyzeIsIndependentOfRecordOrder(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	want, err := Analyze(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("Analyze() baseline error = %v", err)
	}

	random := rand.New(rand.NewSource(50))
	for iteration := 0; iteration < 20; iteration++ {
		vertices := append([]graph.Vertex(nil), fixture.Vertices...)
		edges := append([]graph.Edge(nil), fixture.Edges...)
		if iteration == 0 {
			reverseVertices(vertices)
			reverseEdges(edges)
		} else {
			random.Shuffle(len(vertices), func(i, j int) { vertices[i], vertices[j] = vertices[j], vertices[i] })
			random.Shuffle(len(edges), func(i, j int) { edges[i], edges[j] = edges[j], edges[i] })
		}

		got, err := Analyze(vertices, edges)
		if err != nil {
			t.Fatalf("Analyze() iteration %d error = %v", iteration, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Analyze() iteration %d = %#v, want %#v", iteration, got, want)
		}
	}
}

func TestAnalyzeAcceptsEmptyGraphWithJSONArrays(t *testing.T) {
	got, err := Analyze(nil, nil)
	if err != nil {
		t.Fatalf("Analyze(nil, nil) error = %v", err)
	}
	if got.Clusters == nil || got.IsolatedUsers == nil {
		t.Fatalf("Analyze(nil, nil) = %#v, want non-nil empty slices", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if got, want := string(encoded), `{"clusters":[],"isolated_users":[]}`; got != want {
		t.Fatalf("json.Marshal(Report) = %s, want %s", got, want)
	}
}

func TestAnalyzeRejectsMalformedBackendGraphs(t *testing.T) {
	const fixtureID = "safe-fixture"
	user := analyzerVertex(t, "backend-user", "User", "safe-user", fixtureID, "")
	user2 := analyzerVertex(t, "backend-user-2", "User", "safe-user-2", fixtureID, "")
	identifier := analyzerVertex(t, "backend-identifier", "Identifier", "safe-identifier", fixtureID, IdentifierDevice)
	edge := analyzerEdge(t, "backend-edge", "safe-edge", "backend-user", "backend-identifier", fixtureID)

	vertexCases := []struct {
		name     string
		vertices []graph.Vertex
	}{
		{name: "zero vertex", vertices: []graph.Vertex{{}}},
		{name: "duplicate backend vertex ID", vertices: []graph.Vertex{user, user}},
		{name: "duplicate opaque vertex ID", vertices: []graph.Vertex{user, analyzerVertex(t, "other-backend-user", "User", "safe-user", fixtureID, "")}},
		{name: "wrong vertex label", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-label", "Account", graph.Properties{"opaque_id": "secret-opaque-label", "fixture_id": fixtureID})}},
		{name: "missing vertex property", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-missing", "User", graph.Properties{"opaque_id": "secret-opaque-missing"})}},
		{name: "extra vertex property", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-extra", "User", graph.Properties{"opaque_id": "secret-opaque-extra", "fixture_id": fixtureID, "secret-extra-key": "secret-extra-value"})}},
		{name: "wrong vertex property type", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-type", "User", graph.Properties{"opaque_id": 42, "fixture_id": fixtureID})}},
		{name: "blank vertex opaque ID", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-blank", "User", graph.Properties{"opaque_id": "  ", "fixture_id": fixtureID})}},
		{name: "unknown identifier kind", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-kind", "Identifier", graph.Properties{"opaque_id": "secret-opaque-kind", "fixture_id": fixtureID, "kind": "secret-kind"})}},
		{name: "wrong identifier kind type", vertices: []graph.Vertex{analyzerRawVertex(t, "secret-backend-kind-type", "Identifier", graph.Properties{"opaque_id": "secret-opaque-kind-type", "fixture_id": fixtureID, "kind": 3})}},
		{name: "inconsistent fixture namespace", vertices: []graph.Vertex{user, analyzerVertex(t, "other-backend", "User", "other-safe-user", "secret-other-fixture", "")}},
	}
	for _, test := range vertexCases {
		t.Run(test.name, func(t *testing.T) {
			assertInvalidGraphRedacted(t, test.vertices, nil)
		})
	}

	edgeCases := []struct {
		name  string
		edges []graph.Edge
	}{
		{name: "zero edge", edges: []graph.Edge{{}}},
		{name: "duplicate backend edge ID", edges: []graph.Edge{edge, edge}},
		{name: "duplicate opaque edge ID", edges: []graph.Edge{edge, analyzerEdge(t, "other-backend-edge", "safe-edge", "backend-user-2", "backend-identifier", fixtureID)}},
		{name: "duplicate logical relationship", edges: []graph.Edge{edge, analyzerEdge(t, "other-backend-edge", "other-safe-edge", "backend-user", "backend-identifier", fixtureID)}},
		{name: "wrong edge direction", edges: []graph.Edge{analyzerEdge(t, "secret-direction-edge", "secret-direction-opaque", "backend-identifier", "backend-user", fixtureID)}},
		{name: "missing edge endpoint", edges: []graph.Edge{analyzerEdge(t, "secret-missing-edge", "secret-missing-opaque", "backend-user", "secret-missing-endpoint", fixtureID)}},
		{name: "wrong edge label", edges: []graph.Edge{analyzerRawEdge(t, "secret-label-edge", "LINKED", "backend-user", "backend-identifier", graph.Properties{"opaque_id": "secret-label-opaque", "fixture_id": fixtureID})}},
		{name: "missing edge property", edges: []graph.Edge{analyzerRawEdge(t, "secret-property-edge", labelUsesIdentifier, "backend-user", "backend-identifier", graph.Properties{"opaque_id": "secret-property-opaque"})}},
		{name: "extra edge property", edges: []graph.Edge{analyzerRawEdge(t, "secret-extra-edge", labelUsesIdentifier, "backend-user", "backend-identifier", graph.Properties{"opaque_id": "secret-extra-opaque", "fixture_id": fixtureID, "secret-extra-key": "secret-extra-value"})}},
		{name: "wrong edge property type", edges: []graph.Edge{analyzerRawEdge(t, "secret-type-edge", labelUsesIdentifier, "backend-user", "backend-identifier", graph.Properties{"opaque_id": 42, "fixture_id": fixtureID})}},
		{name: "blank edge opaque ID", edges: []graph.Edge{analyzerRawEdge(t, "secret-blank-edge", labelUsesIdentifier, "backend-user", "backend-identifier", graph.Properties{"opaque_id": "\t", "fixture_id": fixtureID})}},
		{name: "inconsistent edge namespace", edges: []graph.Edge{analyzerEdge(t, "secret-namespace-edge", "secret-namespace-opaque", "backend-user", "backend-identifier", "secret-other-fixture")}},
	}
	vertices := []graph.Vertex{user, user2, identifier}
	for _, test := range edgeCases {
		t.Run(test.name, func(t *testing.T) {
			assertInvalidGraphRedacted(t, vertices, test.edges)
		})
	}
}

func assertInvalidGraphRedacted(t *testing.T, vertices []graph.Vertex, edges []graph.Edge) {
	t.Helper()
	_, err := Analyze(vertices, edges)
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("Analyze() error = %v, want ErrInvalidGraph", err)
	}
	for _, secret := range []string{
		"secret-backend", "secret-opaque", "secret-kind", "secret-other-fixture",
		"secret-direction", "secret-missing", "secret-label", "secret-property",
		"secret-extra", "secret-type", "secret-blank", "secret-namespace",
	} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("Analyze() error retained sensitive value: %q", err)
		}
	}
}

func analyzerVertex(t *testing.T, backendID, label, opaqueID, fixtureID string, kind IdentifierKind) graph.Vertex {
	t.Helper()
	properties := graph.Properties{"opaque_id": opaqueID, "fixture_id": fixtureID}
	if label == labelIdentifier {
		properties["kind"] = string(kind)
	}
	return analyzerRawVertex(t, backendID, label, properties)
}

func analyzerRawVertex(t *testing.T, backendID, label string, properties graph.Properties) graph.Vertex {
	t.Helper()
	vertex, err := graph.ParseVertex(backendID, label, properties)
	if err != nil {
		t.Fatalf("graph.ParseVertex() error = %v", err)
	}
	return vertex
}

func analyzerEdge(t *testing.T, backendID, opaqueID, start, end, fixtureID string) graph.Edge {
	t.Helper()
	return analyzerRawEdge(t, backendID, labelUsesIdentifier, start, end, graph.Properties{
		"opaque_id":  opaqueID,
		"fixture_id": fixtureID,
	})
}

func analyzerRawEdge(t *testing.T, backendID, label, start, end string, properties graph.Properties) graph.Edge {
	t.Helper()
	edge, err := graph.ParseEdge(backendID, label, graph.RawEdgeEndpoints{Start: start, End: end}, properties)
	if err != nil {
		t.Fatalf("graph.ParseEdge() error = %v", err)
	}
	return edge
}

func reverseVertices(vertices []graph.Vertex) {
	for left, right := 0, len(vertices)-1; left < right; left, right = left+1, right-1 {
		vertices[left], vertices[right] = vertices[right], vertices[left]
	}
}

func reverseEdges(edges []graph.Edge) {
	for left, right := 0, len(edges)-1; left < right; left, right = left+1, right-1 {
		edges[left], edges[right] = edges[right], edges[left]
	}
}
