package abusecluster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	neo4jgraph "github.com/bluetape4k/bluetape-go/graph/neo4j"
)

func TestFixtureParametersBuildsFreshOpaqueRows(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}

	params, err := fixtureParameters(fixture)
	if err != nil {
		t.Fatalf("fixtureParameters() error = %v", err)
	}
	assertMapKeys(t, params, "fixture_id", "users", "identifiers", "edges")
	if got, want := params["fixture_id"], fixture.ID; got != want {
		t.Fatalf("fixture_id = %#v, want %q", got, want)
	}

	users, ok := params["users"].([]map[string]any)
	if !ok {
		t.Fatalf("users type = %T, want []map[string]any", params["users"])
	}
	identifiers, ok := params["identifiers"].([]map[string]any)
	if !ok {
		t.Fatalf("identifiers type = %T, want []map[string]any", params["identifiers"])
	}
	edges, ok := params["edges"].([]map[string]any)
	if !ok {
		t.Fatalf("edges type = %T, want []map[string]any", params["edges"])
	}
	if got, want := len(users), len(fixtureUsers); got != want {
		t.Fatalf("len(users) = %d, want %d", got, want)
	}
	if got, want := len(identifiers), len(fixtureIdentifiers); got != want {
		t.Fatalf("len(identifiers) = %d, want %d", got, want)
	}
	if got, want := len(edges), len(fixtureEdges); got != want {
		t.Fatalf("len(edges) = %d, want %d", got, want)
	}
	for index, row := range users {
		assertMapKeys(t, row, "opaque_id")
		if got, want := row["opaque_id"], fixtureUsers[index]; got != want {
			t.Errorf("users[%d].opaque_id = %#v, want %q", index, got, want)
		}
	}
	for index, row := range identifiers {
		assertMapKeys(t, row, "opaque_id", "kind")
		if got, want := row["opaque_id"], fixtureIdentifiers[index].opaqueID; got != want {
			t.Errorf("identifiers[%d].opaque_id = %#v, want %q", index, got, want)
		}
		if got, want := row["kind"], string(fixtureIdentifiers[index].kind); got != want {
			t.Errorf("identifiers[%d].kind = %#v, want %q", index, got, want)
		}
	}
	for index, row := range edges {
		assertMapKeys(t, row, "opaque_id", "start", "end")
		want := fixtureEdges[index]
		if got := row["opaque_id"]; got != want.opaqueID {
			t.Errorf("edges[%d].opaque_id = %#v, want %q", index, got, want.opaqueID)
		}
		if got := row["start"]; got != want.userID {
			t.Errorf("edges[%d].start = %#v, want %q", index, got, want.userID)
		}
		if got := row["end"]; got != want.identifierID {
			t.Errorf("edges[%d].end = %#v, want %q", index, got, want.identifierID)
		}
	}

	users[0]["opaque_id"] = "mutated-user"
	identifiers[0]["kind"] = "mutated-kind"
	delete(edges[0], "start")
	users = append(users, map[string]any{"opaque_id": "mutated-extra"})
	params["users"] = users

	fresh, err := fixtureParameters(fixture)
	if err != nil {
		t.Fatalf("second fixtureParameters() error = %v", err)
	}
	freshUsers := fresh["users"].([]map[string]any)
	freshIdentifiers := fresh["identifiers"].([]map[string]any)
	freshEdges := fresh["edges"].([]map[string]any)
	if got, want := freshUsers[0]["opaque_id"], fixtureUsers[0]; got != want {
		t.Fatalf("fresh users[0].opaque_id = %#v, want %q", got, want)
	}
	if got, want := freshIdentifiers[0]["kind"], string(fixtureIdentifiers[0].kind); got != want {
		t.Fatalf("fresh identifiers[0].kind = %#v, want %q", got, want)
	}
	if got, want := freshEdges[0]["start"], fixtureEdges[0].userID; got != want {
		t.Fatalf("fresh edges[0].start = %#v, want %q", got, want)
	}
	if got, want := len(freshUsers), len(fixtureUsers); got != want {
		t.Fatalf("len(fresh users) = %d, want %d", got, want)
	}
}

func TestFixtureParametersValidatesBeforeBuildingRows(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	fixture.ID = "provider://secret-host/secret-fixture"

	params, err := fixtureParameters(fixture)
	if params != nil {
		t.Fatalf("fixtureParameters(invalid) params = %#v, want nil", params)
	}
	if !errors.Is(err, ErrInvalidFixture) {
		t.Fatalf("fixtureParameters(invalid) error = %v, want ErrInvalidFixture", err)
	}
	for _, secret := range []string{"provider://", "secret-host", "secret-fixture", fixtureUsers[0]} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("fixtureParameters(invalid) error leaked %q: %v", secret, err)
		}
	}
}

func TestValidateLoadedGraphReusesBoundedBackendValidation(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	if err := validateLoadedGraph(fixture.Vertices, fixture.Edges); err != nil {
		t.Fatalf("validateLoadedGraph(default) error = %v", err)
	}
	if err := validateLoadedGraph(nil, nil); err != nil {
		t.Fatalf("validateLoadedGraph(nil, nil) error = %v", err)
	}

	malformed := analyzerRawVertex(t, "secret-backend-id", "Account", graph.Properties{
		"opaque_id":  "secret-opaque-id",
		"fixture_id": "secret-fixture-id",
	})
	err = validateLoadedGraph([]graph.Vertex{malformed}, nil)
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("validateLoadedGraph(malformed) error = %v, want ErrInvalidGraph", err)
	}
	for _, secret := range []string{"secret-backend-id", "secret-opaque-id", "secret-fixture-id"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("validateLoadedGraph(malformed) error leaked %q: %v", secret, err)
		}
	}

	boundary := boundaryFixture(t)
	overVertices := append([]graph.Vertex(nil), boundary.Vertices...)
	overVertices = append(overVertices, analyzerVertex(t, "overflow-backend", labelUser, "overflow-user", boundary.ID, ""))
	if err := validateLoadedGraph(overVertices, boundary.Edges); !errors.Is(err, ErrGraphTooLarge) {
		t.Fatalf("validateLoadedGraph(max vertices + 1) error = %v, want ErrGraphTooLarge", err)
	}
	overEdges := append([]graph.Edge(nil), boundary.Edges...)
	overEdges = append(overEdges, analyzerEdge(t, "overflow-edge-backend", "overflow-edge", "user-000", "identifier-008", boundary.ID))
	if err := validateLoadedGraph(boundary.Vertices, overEdges); !errors.Is(err, ErrGraphTooLarge) {
		t.Fatalf("validateLoadedGraph(max edges + 1) error = %v, want ErrGraphTooLarge", err)
	}
}

func TestStoreRejectsNilClientAndReceiver(t *testing.T) {
	store, err := NewStore(nil)
	if store != nil {
		t.Fatalf("NewStore(nil) = %#v, want nil", store)
	}
	assertStableStoreError(t, err, ErrBackend)

	var nilStore *Store
	assertStableStoreError(t, nilStore.ReplaceFixture(context.Background(), Fixture{}), ErrBackend)
	vertices, edges, err := nilStore.LoadFixture(context.Background(), FixtureID)
	if vertices != nil || edges != nil {
		t.Fatalf("nil Store.LoadFixture() = (%#v, %#v), want nil slices", vertices, edges)
	}
	assertStableStoreError(t, err, ErrBackend)

	missingClient := &Store{}
	assertStableStoreError(t, missingClient.ReplaceFixture(context.Background(), Fixture{}), ErrBackend)
	_, _, err = missingClient.LoadFixture(context.Background(), FixtureID)
	assertStableStoreError(t, err, ErrBackend)
}

func TestStoreRejectsBlankLoadFixtureIDBeforeIO(t *testing.T) {
	store, err := NewStore(&neo4jgraph.Client{})
	if err != nil {
		t.Fatalf("NewStore(non-nil) error = %v", err)
	}
	for _, fixtureID := range []string{"", " ", "\t\n"} {
		vertices, edges, err := store.LoadFixture(context.Background(), fixtureID)
		if vertices != nil || edges != nil {
			t.Fatalf("LoadFixture(%q) = (%#v, %#v), want nil slices", fixtureID, vertices, edges)
		}
		if !errors.Is(err, ErrInvalidFixture) {
			t.Fatalf("LoadFixture(%q) error = %v, want ErrInvalidFixture", fixtureID, err)
		}
		if got, want := err.Error(), ErrInvalidFixture.Error()+": blank id"; got != want {
			t.Fatalf("LoadFixture(%q) error text = %q, want stable %q", fixtureID, got, want)
		}
	}
}

func TestStoreQueriesAreFixedScopedAndBounded(t *testing.T) {
	const wantReplace = `OPTIONAL MATCH (stale {fixture_id: $fixture_id})
WITH [node IN collect(stale) WHERE node IS NOT NULL] AS stale_nodes
FOREACH (node IN stale_nodes | DETACH DELETE node)
WITH size(stale_nodes) AS deleted
FOREACH (user IN $users |
  CREATE (:User {opaque_id: user.opaque_id, fixture_id: $fixture_id}))
FOREACH (identifier IN $identifiers |
  CREATE (:Identifier {opaque_id: identifier.opaque_id, kind: identifier.kind, fixture_id: $fixture_id}))
WITH deleted
UNWIND $edges AS edge
MATCH (user:User {opaque_id: edge.start, fixture_id: $fixture_id})
MATCH (identifier:Identifier {opaque_id: edge.end, fixture_id: $fixture_id})
CREATE (user)-[:USES_IDENTIFIER {opaque_id: edge.opaque_id, fixture_id: $fixture_id}]->(identifier)`
	const wantVertices = `MATCH (n {fixture_id: $fixture_id}) WHERE n:User OR n:Identifier RETURN n ORDER BY elementId(n) LIMIT $limit`
	const wantEdges = `MATCH ()-[r:USES_IDENTIFIER {fixture_id: $fixture_id}]->() RETURN r ORDER BY elementId(r) LIMIT $limit`

	if replaceFixtureCypher != wantReplace {
		t.Fatalf("replaceFixtureCypher =\n%s\nwant\n%s", replaceFixtureCypher, wantReplace)
	}
	if readVerticesCypher != wantVertices {
		t.Fatalf("readVerticesCypher = %q, want %q", readVerticesCypher, wantVertices)
	}
	if readEdgesCypher != wantEdges {
		t.Fatalf("readEdgesCypher = %q, want %q", readEdgesCypher, wantEdges)
	}
	if got, want := vertexReadLimit, MaxVertices+1; got != want {
		t.Fatalf("vertexReadLimit = %d, want %d", got, want)
	}
	if got, want := edgeReadLimit, MaxEdges+1; got != want {
		t.Fatalf("edgeReadLimit = %d, want %d", got, want)
	}

	queries := []string{replaceFixtureCypher, readVerticesCypher, readEdgesCypher}
	if got, want := len(queries), 3; got != want {
		t.Fatalf("query count = %d, want one write and two reads", got)
	}
	for index, query := range queries {
		for _, forbidden := range []string{"%s", "%q", "fmt.", FixtureID, fixtureUsers[0], "provider://"} {
			if strings.Contains(query, forbidden) {
				t.Errorf("queries[%d] contains dynamic/raw value %q: %s", index, forbidden, query)
			}
		}
		if strings.Contains(query, ";") {
			t.Errorf("queries[%d] contains multiple-statement delimiter: %s", index, query)
		}
	}
	if strings.Contains(replaceFixtureCypher, "MATCH (stale)\n") ||
		!strings.Contains(replaceFixtureCypher, "OPTIONAL MATCH (stale {fixture_id: $fixture_id})") ||
		!strings.Contains(replaceFixtureCypher, "DETACH DELETE node") {
		t.Fatalf("replace query is not namespace-scoped with an empty-namespace guard:\n%s", replaceFixtureCypher)
	}
}

func TestStoreErrorRedactsBackendFailuresAndPreservesContext(t *testing.T) {
	if err := storeError(nil); err != nil {
		t.Fatalf("storeError(nil) = %v, want nil", err)
	}

	provider := &neo4jgraph.Error{
		Kind:      neo4jgraph.ErrDriver,
		Operation: "secret query operation",
		Column:    "secret_column",
		Cause:     errors.New("provider://secret-host/?token=secret-token opaque-secret"),
	}
	assertStableStoreError(t, storeError(provider), ErrBackend)

	for _, contextErr := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(contextErr.Error(), func(t *testing.T) {
			adapterErr := &neo4jgraph.Error{
				Kind:      neo4jgraph.ErrDriver,
				Operation: "read secret query",
				Cause:     fmt.Errorf("provider secret: %w", contextErr),
			}
			err := storeError(adapterErr)
			if !errors.Is(err, ErrBackend) {
				t.Fatalf("storeError(context) error = %v, want ErrBackend", err)
			}
			if !errors.Is(err, contextErr) {
				t.Fatalf("storeError(context) error = %v, want %v", err, contextErr)
			}
			for _, secret := range []string{"provider secret", "read secret query", "secret-host", "secret-token", "opaque-secret"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("storeError(context) leaked %q: %v", secret, err)
				}
			}
		})
	}
}

func assertMapKeys(t *testing.T, values map[string]any, want ...string) {
	t.Helper()
	got := make([]string, 0, len(values))
	for key := range values {
		got = append(got, key)
	}
	if !reflect.DeepEqual(stringSet(got), stringSet(want)) {
		t.Fatalf("map keys = %v, want %v", got, want)
	}
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func assertStableStoreError(t *testing.T, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if got := err.Error(); got != want.Error() {
		t.Fatalf("error text = %q, want stable %q", got, want.Error())
	}
	for _, secret := range []string{"provider://", "secret-host", "secret-token", "opaque-secret", "secret query", "$fixture_id"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked %q: %v", secret, err)
		}
	}
}
