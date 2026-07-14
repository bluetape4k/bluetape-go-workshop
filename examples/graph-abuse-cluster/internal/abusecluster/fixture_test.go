package abusecluster

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestNewFixtureBuildsValidatedGraph(t *testing.T) {
	fixture, err := NewFixture(FixtureID)
	if err != nil {
		t.Fatalf("NewFixture() error = %v", err)
	}
	if err := ValidateFixture(fixture); err != nil {
		t.Fatalf("ValidateFixture() error = %v", err)
	}

	if got, want := fixture.ID, FixtureID; got != want {
		t.Fatalf("Fixture.ID = %q, want %q", got, want)
	}
	if got, want := len(fixture.Vertices), 12; got != want {
		t.Fatalf("len(Fixture.Vertices) = %d, want %d", got, want)
	}
	if got, want := len(fixture.Edges), 10; got != want {
		t.Fatalf("len(Fixture.Edges) = %d, want %d", got, want)
	}

	wantVertices := map[string]struct {
		label string
		kind  string
	}{
		"usr-001": {label: "User"},
		"usr-002": {label: "User"},
		"usr-003": {label: "User"},
		"usr-004": {label: "User"},
		"usr-005": {label: "User"},
		"usr-006": {label: "User"},
		"dev-001": {label: "Identifier", kind: string(IdentifierDevice)},
		"dev-002": {label: "Identifier", kind: string(IdentifierDevice)},
		"ip-001":  {label: "Identifier", kind: string(IdentifierIP)},
		"ip-002":  {label: "Identifier", kind: string(IdentifierIP)},
		"ip-003":  {label: "Identifier", kind: string(IdentifierIP)},
		"pay-001": {label: "Identifier", kind: string(IdentifierPaymentToken)},
	}
	for _, vertex := range fixture.Vertices {
		id := vertex.ID().String()
		want, ok := wantVertices[id]
		if !ok {
			t.Fatalf("unexpected vertex ID %q", id)
		}
		delete(wantVertices, id)
		if got := vertex.Label().String(); got != want.label {
			t.Errorf("vertex %q label = %q, want %q", id, got, want.label)
		}
		properties := vertex.Properties()
		if got := properties["opaque_id"]; got != id {
			t.Errorf("vertex %q opaque_id = %#v, want %q", id, got, id)
		}
		if got := properties["fixture_id"]; got != FixtureID {
			t.Errorf("vertex %q fixture_id = %#v, want %q", id, got, FixtureID)
		}
		if want.kind == "" {
			if got, ok := properties["kind"]; ok {
				t.Errorf("user vertex %q kind = %#v, want absent", id, got)
			}
			if got, want := len(properties), 2; got != want {
				t.Errorf("user vertex %q property count = %d, want %d", id, got, want)
			}
			continue
		}
		if got := properties["kind"]; got != want.kind {
			t.Errorf("identifier vertex %q kind = %#v, want %q", id, got, want.kind)
		}
		if got, want := len(properties), 3; got != want {
			t.Errorf("identifier vertex %q property count = %d, want %d", id, got, want)
		}
	}
	if len(wantVertices) != 0 {
		t.Fatalf("missing vertices: %v", wantVertices)
	}

	wantEdges := map[string]bool{
		"usr-001>dev-001": true,
		"usr-002>dev-001": true,
		"usr-002>ip-001":  true,
		"usr-003>ip-001":  true,
		"usr-003>pay-001": true,
		"usr-004>dev-002": true,
		"usr-005>dev-002": true,
		"usr-004>ip-002":  true,
		"usr-005>ip-002":  true,
		"usr-006>ip-003":  true,
	}
	seenOpaqueIDs := make(map[string]struct{}, len(fixture.Edges))
	for _, edge := range fixture.Edges {
		if got, want := edge.Label().String(), "USES_IDENTIFIER"; got != want {
			t.Errorf("edge label = %q, want %q", got, want)
		}
		logicalID := edge.StartID().String() + ">" + edge.EndID().String()
		if !wantEdges[logicalID] {
			t.Fatalf("unexpected logical edge %q", logicalID)
		}
		delete(wantEdges, logicalID)

		properties := edge.Properties()
		opaqueID, ok := properties["opaque_id"].(string)
		if !ok || strings.TrimSpace(opaqueID) == "" {
			t.Errorf("edge opaque_id must be a non-blank string")
		}
		if _, duplicate := seenOpaqueIDs[opaqueID]; duplicate {
			t.Errorf("duplicate edge opaque_id")
		}
		seenOpaqueIDs[opaqueID] = struct{}{}
		if got := properties["fixture_id"]; got != FixtureID {
			t.Errorf("edge fixture_id = %#v, want %q", got, FixtureID)
		}
		if got, want := len(properties), 2; got != want {
			t.Errorf("edge property count = %d, want %d", got, want)
		}
	}
	if len(wantEdges) != 0 {
		t.Fatalf("missing logical edges: %v", wantEdges)
	}
}

func TestNewFixtureUsesNamespaceAndReturnsDefensiveValues(t *testing.T) {
	fixture, err := NewFixture("test-namespace")
	if err != nil {
		t.Fatalf("NewFixture() error = %v", err)
	}
	if got, want := fixture.ID, "test-namespace"; got != want {
		t.Fatalf("Fixture.ID = %q, want %q", got, want)
	}
	for _, vertex := range fixture.Vertices {
		if got := vertex.Properties()["fixture_id"]; got != "test-namespace" {
			t.Fatalf("vertex fixture_id = %#v, want test namespace", got)
		}
	}
	for _, edge := range fixture.Edges {
		if got := edge.Properties()["fixture_id"]; got != "test-namespace" {
			t.Fatalf("edge fixture_id = %#v, want test namespace", got)
		}
	}

	vertexProperties := fixture.Vertices[0].Properties()
	originalVertexOpaqueID := vertexProperties["opaque_id"]
	vertexProperties["opaque_id"] = "caller-mutation"
	if got := fixture.Vertices[0].Properties()["opaque_id"]; got != originalVertexOpaqueID {
		t.Fatalf("vertex properties retained caller mutation: %#v", got)
	}
	edgeProperties := fixture.Edges[0].Properties()
	originalEdgeOpaqueID := edgeProperties["opaque_id"]
	edgeProperties["opaque_id"] = "caller-mutation"
	if got := fixture.Edges[0].Properties()["opaque_id"]; got != originalEdgeOpaqueID {
		t.Fatalf("edge properties retained caller mutation: %#v", got)
	}

	fixture.Vertices[0] = graph.Vertex{}
	fixture.Edges[0] = graph.Edge{}
	rebuilt, err := NewFixture("test-namespace")
	if err != nil {
		t.Fatalf("second NewFixture() error = %v", err)
	}
	if rebuilt.Vertices[0].ID().String() == "" || rebuilt.Edges[0].ID().String() == "" {
		t.Fatal("a later fixture retained caller slice mutation")
	}
}

func TestNewFixtureRejectsBlankNamespace(t *testing.T) {
	for _, fixtureID := range []string{"", " ", "\t\n"} {
		_, err := NewFixture(fixtureID)
		if !errors.Is(err, ErrInvalidFixture) {
			t.Errorf("NewFixture(%q) error = %v, want ErrInvalidFixture", fixtureID, err)
		}
	}
}

func TestDefaultFixtureDelegatesToNamedFixture(t *testing.T) {
	got, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	want, err := NewFixture(FixtureID)
	if err != nil {
		t.Fatalf("NewFixture(FixtureID) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultFixture() did not return NewFixture(FixtureID)")
	}
}

func TestValidateFixtureRejectsInvalidGraphs(t *testing.T) {
	valid, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}

	duplicateEdge := mustEdge(t, "duplicate-edge", "USES_IDENTIFIER", "usr-001", "dev-001", graph.Properties{
		"opaque_id":  "duplicate-edge",
		"fixture_id": FixtureID,
	})
	missingEndpoint := mustEdge(t, "missing-endpoint", "USES_IDENTIFIER", "usr-001", "absent-identifier", graph.Properties{
		"opaque_id":  "missing-endpoint",
		"fixture_id": FixtureID,
	})
	unknownKind := mustVertex(t, "identifier-unknown", "Identifier", graph.Properties{
		"opaque_id":  "identifier-unknown",
		"fixture_id": FixtureID,
		"kind":       "email",
	})
	wrongLabel := mustVertex(t, "account-001", "Account", graph.Properties{
		"opaque_id":  "account-001",
		"fixture_id": FixtureID,
	})
	missingProperty := mustVertex(t, "user-missing-property", "User", graph.Properties{
		"opaque_id": "user-missing-property",
	})
	wrongPropertyType := mustVertex(t, "user-wrong-property-type", "User", graph.Properties{
		"opaque_id":  42,
		"fixture_id": FixtureID,
	})
	extraProperty := mustVertex(t, "user-extra-property", "User", graph.Properties{
		"opaque_id":  "user-extra-property",
		"fixture_id": FixtureID,
		"unexpected": "value",
	})
	wrongEdgeLabel := mustEdge(t, "linked-edge", "LINKED", "usr-001", "dev-001", graph.Properties{
		"opaque_id":  "linked-edge",
		"fixture_id": FixtureID,
	})
	reversedEdge := mustEdge(t, "reversed-edge", "USES_IDENTIFIER", "dev-001", "usr-001", graph.Properties{
		"opaque_id":  "reversed-edge",
		"fixture_id": FixtureID,
	})
	wrongEdgePropertyType := mustEdge(t, "wrong-edge-property", "USES_IDENTIFIER", "usr-001", "dev-002", graph.Properties{
		"opaque_id":  99,
		"fixture_id": FixtureID,
	})

	tests := []struct {
		name    string
		fixture Fixture
		want    error
	}{
		{name: "blank fixture ID", fixture: Fixture{}, want: ErrInvalidFixture},
		{name: "whitespace fixture ID", fixture: Fixture{ID: "  "}, want: ErrInvalidFixture},
		{name: "duplicate vertex", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), valid.Vertices[0])), want: ErrInvalidFixture},
		{name: "duplicate logical edge", fixture: fixtureWithEdges(valid, append(cloneEdges(valid.Edges), duplicateEdge)), want: ErrInvalidFixture},
		{name: "missing endpoint", fixture: fixtureWithEdges(valid, append(cloneEdges(valid.Edges), missingEndpoint)), want: ErrInvalidFixture},
		{name: "unknown identifier kind", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), unknownKind)), want: ErrInvalidFixture},
		{name: "unknown vertex label", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), wrongLabel)), want: ErrInvalidFixture},
		{name: "missing vertex property", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), missingProperty)), want: ErrInvalidFixture},
		{name: "wrong vertex property type", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), wrongPropertyType)), want: ErrInvalidFixture},
		{name: "extra vertex property", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), extraProperty)), want: ErrInvalidFixture},
		{name: "zero vertex", fixture: fixtureWithVertices(valid, append(cloneVertices(valid.Vertices), graph.Vertex{})), want: ErrInvalidFixture},
		{name: "wrong edge label", fixture: fixtureWithEdges(valid, append(cloneEdges(valid.Edges), wrongEdgeLabel)), want: ErrInvalidFixture},
		{name: "reversed edge", fixture: fixtureWithEdges(valid, append(cloneEdges(valid.Edges), reversedEdge)), want: ErrInvalidFixture},
		{name: "wrong edge property type", fixture: fixtureWithEdges(valid, append(cloneEdges(valid.Edges), wrongEdgePropertyType)), want: ErrInvalidFixture},
		{name: "zero edge", fixture: fixtureWithEdges(valid, append(cloneEdges(valid.Edges), graph.Edge{})), want: ErrInvalidFixture},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateFixture(test.fixture)
			if !errors.Is(err, test.want) {
				t.Fatalf("ValidateFixture() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidateFixtureRedactsValuesFromErrors(t *testing.T) {
	const (
		secretOpaqueID = "secret-opaque-value"
		secretEndpoint = "secret-missing-endpoint"
		secretProperty = "secret-property-value"
	)
	malformedVertex := mustVertex(t, secretOpaqueID, "User", graph.Properties{
		"opaque_id":  secretOpaqueID,
		"fixture_id": secretProperty,
	})
	endpointVertex := mustVertex(t, secretOpaqueID, "User", graph.Properties{
		"opaque_id":  secretOpaqueID,
		"fixture_id": "safe-fixture",
	})
	edge := mustEdge(t, "secret-edge-value", "USES_IDENTIFIER", secretOpaqueID, secretEndpoint, graph.Properties{
		"opaque_id":  "secret-edge-value",
		"fixture_id": "safe-fixture",
	})

	for _, fixture := range []Fixture{
		{ID: "safe-fixture", Vertices: []graph.Vertex{malformedVertex}},
		{ID: "safe-fixture", Vertices: []graph.Vertex{endpointVertex}, Edges: []graph.Edge{edge}},
	} {
		err := ValidateFixture(fixture)
		if !errors.Is(err, ErrInvalidFixture) {
			t.Fatalf("ValidateFixture() error = %v, want ErrInvalidFixture", err)
		}
		for _, secret := range []string{secretOpaqueID, secretEndpoint, secretProperty, "secret-edge-value"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("ValidateFixture() error retained sensitive value: %q", err)
			}
		}
	}
}

func TestValidateFixtureAcceptsLimitsAndRejectsOverflow(t *testing.T) {
	fixture := boundaryFixture(t)
	if got, want := len(fixture.Vertices), MaxVertices; got != want {
		t.Fatalf("boundary vertex count = %d, want %d", got, want)
	}
	if got, want := len(fixture.Edges), MaxEdges; got != want {
		t.Fatalf("boundary edge count = %d, want %d", got, want)
	}
	if err := ValidateFixture(fixture); err != nil {
		t.Fatalf("ValidateFixture(boundary) error = %v", err)
	}

	overVertices := fixture
	overVertices.Vertices = append(cloneVertices(fixture.Vertices), mustVertex(t, "overflow-user", "User", graph.Properties{
		"opaque_id":  "overflow-user",
		"fixture_id": fixture.ID,
	}))
	if err := ValidateFixture(overVertices); !errors.Is(err, ErrGraphTooLarge) {
		t.Fatalf("ValidateFixture(max vertices + 1) error = %v, want ErrGraphTooLarge", err)
	}

	overEdges := fixture
	overEdges.Edges = append(cloneEdges(fixture.Edges), mustEdge(t, "overflow-edge", "USES_IDENTIFIER", "user-000", "identifier-008", graph.Properties{
		"opaque_id":  "overflow-edge",
		"fixture_id": fixture.ID,
	}))
	if err := ValidateFixture(overEdges); !errors.Is(err, ErrGraphTooLarge) {
		t.Fatalf("ValidateFixture(max edges + 1) error = %v, want ErrGraphTooLarge", err)
	}
}

func boundaryFixture(t *testing.T) Fixture {
	t.Helper()
	const fixtureID = "boundary-fixture"
	vertices := make([]graph.Vertex, 0, MaxVertices)
	for i := 0; i < MaxVertices/2; i++ {
		id := fmt.Sprintf("user-%03d", i)
		vertices = append(vertices, mustVertex(t, id, "User", graph.Properties{
			"opaque_id":  id,
			"fixture_id": fixtureID,
		}))
	}
	for i := 0; i < MaxVertices/2; i++ {
		id := fmt.Sprintf("identifier-%03d", i)
		vertices = append(vertices, mustVertex(t, id, "Identifier", graph.Properties{
			"opaque_id":  id,
			"fixture_id": fixtureID,
			"kind":       string(IdentifierDevice),
		}))
	}

	edges := make([]graph.Edge, 0, MaxEdges)
	for user := 0; user < MaxVertices/2; user++ {
		for identifier := 0; identifier < 8; identifier++ {
			id := fmt.Sprintf("edge-%03d-%03d", user, identifier)
			edges = append(edges, mustEdge(
				t,
				id,
				"USES_IDENTIFIER",
				fmt.Sprintf("user-%03d", user),
				fmt.Sprintf("identifier-%03d", identifier),
				graph.Properties{"opaque_id": id, "fixture_id": fixtureID},
			))
		}
	}
	return Fixture{ID: fixtureID, Vertices: vertices, Edges: edges}
}

func mustVertex(t *testing.T, id, label string, properties graph.Properties) graph.Vertex {
	t.Helper()
	vertex, err := graph.ParseVertex(id, label, properties)
	if err != nil {
		t.Fatalf("graph.ParseVertex() error = %v", err)
	}
	return vertex
}

func mustEdge(t *testing.T, id, label, start, end string, properties graph.Properties) graph.Edge {
	t.Helper()
	edge, err := graph.ParseEdge(id, label, graph.RawEdgeEndpoints{Start: start, End: end}, properties)
	if err != nil {
		t.Fatalf("graph.ParseEdge() error = %v", err)
	}
	return edge
}

func cloneVertices(vertices []graph.Vertex) []graph.Vertex {
	return append([]graph.Vertex(nil), vertices...)
}

func cloneEdges(edges []graph.Edge) []graph.Edge {
	return append([]graph.Edge(nil), edges...)
}

func fixtureWithVertices(fixture Fixture, vertices []graph.Vertex) Fixture {
	fixture.Vertices = vertices
	return fixture
}

func fixtureWithEdges(fixture Fixture, edges []graph.Edge) Fixture {
	fixture.Edges = edges
	return fixture
}
