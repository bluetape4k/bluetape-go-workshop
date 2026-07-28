package abusecluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
	neo4jgraph "github.com/bluetape4k/bluetape-go/graph/neo4j"
)

const (
	replaceFixtureCypher = `OPTIONAL MATCH (stale {fixture_id: $fixture_id})
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

	readVerticesCypher = `MATCH (n {fixture_id: $fixture_id}) WHERE n:User OR n:Identifier RETURN n ORDER BY elementId(n) LIMIT $limit`
	readEdgesCypher    = `MATCH ()-[r:USES_IDENTIFIER {fixture_id: $fixture_id}]->() RETURN r ORDER BY elementId(r) LIMIT $limit`

	vertexReadLimit = MaxVertices + 1
	edgeReadLimit   = MaxEdges + 1
)

// Store 는 제한된 graph fixture namespace 하나를 Neo4j에 영속화한다.
type Store struct {
	client *neo4jgraph.Client
}

// NewStore 는 호출자 소유 Neo4j client를 감싸는 Store를 만든다.
func NewStore(client *neo4jgraph.Client) (*Store, error) {
	if client == nil {
		return nil, ErrBackend
	}
	return &Store{client: client}, nil
}

// ReplaceFixture 는 fixture namespace를 검증된 값으로 원자적으로 교체한다.
func (s *Store) ReplaceFixture(ctx context.Context, fixture Fixture) error {
	if s == nil || s.client == nil {
		return ErrBackend
	}
	params, err := fixtureParameters(fixture)
	if err != nil {
		return err
	}
	return storeError(s.client.ExecuteWrite(ctx, replaceFixtureCypher, params))
}

// LoadFixture 는 제한된 fixture namespace 하나를 읽고 검증한다.
func (s *Store) LoadFixture(ctx context.Context, fixtureID string) ([]graph.Vertex, []graph.Edge, error) {
	if s == nil || s.client == nil {
		return nil, nil, ErrBackend
	}
	if strings.TrimSpace(fixtureID) == "" {
		return nil, nil, fmt.Errorf("%w: blank id", ErrInvalidFixture)
	}

	vertices, err := s.client.ReadVertices(ctx, readVerticesCypher, map[string]any{
		"fixture_id": fixtureID,
		"limit":      vertexReadLimit,
	}, "n")
	if err != nil {
		return nil, nil, storeError(err)
	}
	if len(vertices) > MaxVertices {
		return nil, nil, fmt.Errorf("%w: vertex limit exceeded", ErrGraphTooLarge)
	}

	edges, err := s.client.ReadEdges(ctx, readEdgesCypher, map[string]any{
		"fixture_id": fixtureID,
		"limit":      edgeReadLimit,
	}, "r")
	if err != nil {
		return nil, nil, storeError(err)
	}
	if len(edges) > MaxEdges {
		return nil, nil, fmt.Errorf("%w: edge limit exceeded", ErrGraphTooLarge)
	}
	if err := validateLoadedGraph(vertices, edges); err != nil {
		return nil, nil, err
	}
	return vertices, edges, nil
}

func fixtureParameters(fixture Fixture) (map[string]any, error) {
	if err := ValidateFixture(fixture); err != nil {
		return nil, err
	}

	users := make([]map[string]any, 0, len(fixture.Vertices))
	identifiers := make([]map[string]any, 0, len(fixture.Vertices))
	for _, vertex := range fixture.Vertices {
		switch vertex.Label().String() {
		case labelUser:
			users = append(users, map[string]any{
				propertyOpaqueID: vertex.ID().String(),
			})
		case labelIdentifier:
			identifiers = append(identifiers, map[string]any{
				propertyOpaqueID: vertex.ID().String(),
				propertyKind:     vertex.Properties()[propertyKind],
			})
		}
	}

	edges := make([]map[string]any, 0, len(fixture.Edges))
	for _, edge := range fixture.Edges {
		edges = append(edges, map[string]any{
			propertyOpaqueID: edge.ID().String(),
			"start":          edge.StartID().String(),
			"end":            edge.EndID().String(),
		})
	}

	return map[string]any{
		"fixture_id":  fixture.ID,
		"users":       users,
		"identifiers": identifiers,
		"edges":       edges,
	}, nil
}

func validateLoadedGraph(vertices []graph.Vertex, edges []graph.Edge) error {
	if len(vertices) > MaxVertices || len(edges) > MaxEdges {
		return fmt.Errorf("%w: limit exceeded", ErrGraphTooLarge)
	}
	loaded, fixtureID, err := loadVertices(vertices)
	if err != nil {
		return err
	}
	_, err = loadEdges(edges, loaded, fixtureID)
	return err
}

func storeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %w", ErrBackend, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrBackend, context.DeadlineExceeded)
	}
	return ErrBackend
}
