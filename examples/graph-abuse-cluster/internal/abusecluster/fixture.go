package abusecluster

import (
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
)

const (
	labelUser           = "User"
	labelIdentifier     = "Identifier"
	labelUsesIdentifier = "USES_IDENTIFIER"

	propertyOpaqueID  = "opaque_id"
	propertyFixtureID = "fixture_id"
	propertyKind      = "kind"
)

type identifierSpec struct {
	opaqueID string
	kind     IdentifierKind
}

type edgeSpec struct {
	opaqueID     string
	userID       string
	identifierID string
}

var fixtureUsers = [...]string{
	"usr-001",
	"usr-002",
	"usr-003",
	"usr-004",
	"usr-005",
	"usr-006",
}

var fixtureIdentifiers = [...]identifierSpec{
	{opaqueID: "dev-001", kind: IdentifierDevice},
	{opaqueID: "ip-001", kind: IdentifierIP},
	{opaqueID: "pay-001", kind: IdentifierPaymentToken},
	{opaqueID: "dev-002", kind: IdentifierDevice},
	{opaqueID: "ip-002", kind: IdentifierIP},
	{opaqueID: "ip-003", kind: IdentifierIP},
}

var fixtureEdges = [...]edgeSpec{
	{opaqueID: "uses-001", userID: "usr-001", identifierID: "dev-001"},
	{opaqueID: "uses-002", userID: "usr-002", identifierID: "dev-001"},
	{opaqueID: "uses-003", userID: "usr-002", identifierID: "ip-001"},
	{opaqueID: "uses-004", userID: "usr-003", identifierID: "ip-001"},
	{opaqueID: "uses-005", userID: "usr-003", identifierID: "pay-001"},
	{opaqueID: "uses-006", userID: "usr-004", identifierID: "dev-002"},
	{opaqueID: "uses-007", userID: "usr-005", identifierID: "dev-002"},
	{opaqueID: "uses-008", userID: "usr-004", identifierID: "ip-002"},
	{opaqueID: "uses-009", userID: "usr-005", identifierID: "ip-002"},
	{opaqueID: "uses-010", userID: "usr-006", identifierID: "ip-003"},
}

// NewFixture 는 fixtureID 안에 안정적인 logical abuse-cluster graph를 만든다.
func NewFixture(fixtureID string) (Fixture, error) {
	if strings.TrimSpace(fixtureID) == "" {
		return Fixture{}, fmt.Errorf("%w: blank id", ErrInvalidFixture)
	}

	fixture := Fixture{
		ID:       fixtureID,
		Vertices: make([]graph.Vertex, 0, len(fixtureUsers)+len(fixtureIdentifiers)),
		Edges:    make([]graph.Edge, 0, len(fixtureEdges)),
	}
	for index, opaqueID := range fixtureUsers {
		vertex, err := graph.ParseVertex(opaqueID, labelUser, graph.Properties{
			propertyOpaqueID:  opaqueID,
			propertyFixtureID: fixtureID,
		})
		if err != nil {
			return Fixture{}, fixtureBuildError("vertex", index)
		}
		fixture.Vertices = append(fixture.Vertices, vertex)
	}
	for offset, spec := range fixtureIdentifiers {
		vertex, err := graph.ParseVertex(spec.opaqueID, labelIdentifier, graph.Properties{
			propertyOpaqueID:  spec.opaqueID,
			propertyFixtureID: fixtureID,
			propertyKind:      string(spec.kind),
		})
		if err != nil {
			return Fixture{}, fixtureBuildError("vertex", len(fixtureUsers)+offset)
		}
		fixture.Vertices = append(fixture.Vertices, vertex)
	}
	for index, spec := range fixtureEdges {
		edge, err := graph.ParseEdge(
			spec.opaqueID,
			labelUsesIdentifier,
			graph.RawEdgeEndpoints{Start: spec.userID, End: spec.identifierID},
			graph.Properties{
				propertyOpaqueID:  spec.opaqueID,
				propertyFixtureID: fixtureID,
			},
		)
		if err != nil {
			return Fixture{}, fixtureBuildError("edge", index)
		}
		fixture.Edges = append(fixture.Edges, edge)
	}
	if err := ValidateFixture(fixture); err != nil {
		return Fixture{}, fmt.Errorf("%w: validation failed", ErrInvalidFixture)
	}
	return fixture, nil
}

// DefaultFixture 는 FixtureID 안에 안정적인 graph를 만든다.
func DefaultFixture() (Fixture, error) {
	return NewFixture(FixtureID)
}

// ValidateFixture 는 fixture 크기, schema, endpoint, uniqueness를 검증한다.
func ValidateFixture(fixture Fixture) error {
	if len(fixture.Vertices) > MaxVertices || len(fixture.Edges) > MaxEdges {
		return fmt.Errorf("%w: limit exceeded", ErrGraphTooLarge)
	}
	if strings.TrimSpace(fixture.ID) == "" {
		return fmt.Errorf("%w: blank id", ErrInvalidFixture)
	}

	vertexLabels := make(map[string]string, len(fixture.Vertices))
	for index, vertex := range fixture.Vertices {
		if err := vertex.Validate(); err != nil {
			return fixtureValidationError("vertex", index, "invalid value")
		}
		id := vertex.ID().String()
		if _, exists := vertexLabels[id]; exists {
			return fixtureValidationError("vertex", index, "duplicate id")
		}

		properties := vertex.Properties()
		switch vertex.Label().String() {
		case labelUser:
			if len(properties) != 2 ||
				!matchingStringProperty(properties, propertyOpaqueID, id) ||
				!matchingStringProperty(properties, propertyFixtureID, fixture.ID) {
				return fixtureValidationError("vertex", index, "invalid properties")
			}
		case labelIdentifier:
			kind, ok := properties[propertyKind].(string)
			if len(properties) != 3 ||
				!matchingStringProperty(properties, propertyOpaqueID, id) ||
				!matchingStringProperty(properties, propertyFixtureID, fixture.ID) ||
				!ok || !validIdentifierKind(IdentifierKind(kind)) {
				return fixtureValidationError("vertex", index, "invalid properties")
			}
		default:
			return fixtureValidationError("vertex", index, "invalid label")
		}
		vertexLabels[id] = vertex.Label().String()
	}

	type logicalEdge struct {
		start string
		end   string
		label string
	}
	edgeIDs := make(map[string]struct{}, len(fixture.Edges))
	logicalEdges := make(map[logicalEdge]struct{}, len(fixture.Edges))
	for index, edge := range fixture.Edges {
		if err := edge.Validate(); err != nil {
			return fixtureValidationError("edge", index, "invalid value")
		}
		id := edge.ID().String()
		if _, exists := edgeIDs[id]; exists {
			return fixtureValidationError("edge", index, "duplicate id")
		}
		edgeIDs[id] = struct{}{}
		if edge.Label().String() != labelUsesIdentifier {
			return fixtureValidationError("edge", index, "invalid label")
		}
		properties := edge.Properties()
		if len(properties) != 2 ||
			!matchingStringProperty(properties, propertyOpaqueID, id) ||
			!matchingStringProperty(properties, propertyFixtureID, fixture.ID) {
			return fixtureValidationError("edge", index, "invalid properties")
		}

		start := edge.StartID().String()
		end := edge.EndID().String()
		startLabel, startExists := vertexLabels[start]
		endLabel, endExists := vertexLabels[end]
		if !startExists || !endExists {
			return fixtureValidationError("edge", index, "missing endpoint")
		}
		if startLabel != labelUser || endLabel != labelIdentifier {
			return fixtureValidationError("edge", index, "invalid endpoints")
		}
		key := logicalEdge{start: start, end: end, label: edge.Label().String()}
		if _, exists := logicalEdges[key]; exists {
			return fixtureValidationError("edge", index, "duplicate logical edge")
		}
		logicalEdges[key] = struct{}{}
	}
	return nil
}

func fixtureBuildError(element string, index int) error {
	return fmt.Errorf("%w: %s %d", ErrInvalidFixture, element, index)
}

func fixtureValidationError(element string, index int, reason string) error {
	return fmt.Errorf("%w: %s %d: %s", ErrInvalidFixture, element, index, reason)
}

func matchingStringProperty(properties graph.Properties, key, want string) bool {
	value, ok := properties[key].(string)
	return ok && strings.TrimSpace(value) != "" && value == want
}

func validIdentifierKind(kind IdentifierKind) bool {
	switch kind {
	case IdentifierDevice, IdentifierIP, IdentifierPaymentToken:
		return true
	default:
		return false
	}
}
