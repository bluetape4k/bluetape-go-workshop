package abusecluster

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
)

type analyzedVertex struct {
	backendID string
	opaqueID  string
	label     string
	kind      IdentifierKind
}

type logicalRelationship struct {
	start string
	end   string
	label string
}

// Analyze validates backend-loaded graph values and returns deterministic abuse clusters.
func Analyze(vertices []graph.Vertex, edges []graph.Edge) (Report, error) {
	if len(vertices) > MaxVertices || len(edges) > MaxEdges {
		return Report{}, fmt.Errorf("%w: limit exceeded", ErrGraphTooLarge)
	}

	loaded, fixtureID, err := loadVertices(vertices)
	if err != nil {
		return Report{}, err
	}
	adjacency, err := loadEdges(edges, loaded, fixtureID)
	if err != nil {
		return Report{}, err
	}

	return analyzeComponents(loaded, adjacency), nil
}

func loadVertices(vertices []graph.Vertex) (map[string]analyzedVertex, string, error) {
	loaded := make(map[string]analyzedVertex, len(vertices))
	opaqueIDs := make(map[string]struct{}, len(vertices))
	fixtureID := ""

	for _, vertex := range vertices {
		if err := vertex.Validate(); err != nil {
			return nil, "", invalidGraphError("invalid vertex")
		}
		backendID := vertex.ID().String()
		if _, duplicate := loaded[backendID]; duplicate {
			return nil, "", invalidGraphError("duplicate vertex id")
		}

		properties := vertex.Properties()
		opaqueID, ok := nonBlankString(properties, propertyOpaqueID)
		if !ok {
			return nil, "", invalidGraphError("invalid vertex properties")
		}
		vertexFixtureID, ok := nonBlankString(properties, propertyFixtureID)
		if !ok {
			return nil, "", invalidGraphError("invalid vertex properties")
		}
		if fixtureID == "" {
			fixtureID = vertexFixtureID
		} else if vertexFixtureID != fixtureID {
			return nil, "", invalidGraphError("inconsistent fixture namespace")
		}
		if _, duplicate := opaqueIDs[opaqueID]; duplicate {
			return nil, "", invalidGraphError("duplicate vertex opaque id")
		}

		value := analyzedVertex{
			backendID: backendID,
			opaqueID:  opaqueID,
			label:     vertex.Label().String(),
		}
		switch value.label {
		case labelUser:
			if len(properties) != 2 {
				return nil, "", invalidGraphError("invalid user properties")
			}
		case labelIdentifier:
			kind, kindOK := properties[propertyKind].(string)
			value.kind = IdentifierKind(kind)
			if len(properties) != 3 || !kindOK || !validIdentifierKind(value.kind) {
				return nil, "", invalidGraphError("invalid identifier properties")
			}
		default:
			return nil, "", invalidGraphError("invalid vertex label")
		}

		loaded[backendID] = value
		opaqueIDs[opaqueID] = struct{}{}
	}
	return loaded, fixtureID, nil
}

func loadEdges(edges []graph.Edge, vertices map[string]analyzedVertex, fixtureID string) (map[string][]string, error) {
	adjacency := make(map[string][]string, len(vertices))
	backendIDs := make(map[string]struct{}, len(edges))
	opaqueIDs := make(map[string]struct{}, len(edges))
	logicalEdges := make(map[logicalRelationship]struct{}, len(edges))

	for _, edge := range edges {
		if err := edge.Validate(); err != nil {
			return nil, invalidGraphError("invalid edge")
		}
		backendID := edge.ID().String()
		if _, duplicate := backendIDs[backendID]; duplicate {
			return nil, invalidGraphError("duplicate edge id")
		}
		backendIDs[backendID] = struct{}{}

		label := edge.Label().String()
		if label != labelUsesIdentifier {
			return nil, invalidGraphError("invalid edge label")
		}
		properties := edge.Properties()
		opaqueID, opaqueOK := nonBlankString(properties, propertyOpaqueID)
		edgeFixtureID, fixtureOK := nonBlankString(properties, propertyFixtureID)
		if len(properties) != 2 || !opaqueOK || !fixtureOK {
			return nil, invalidGraphError("invalid edge properties")
		}
		if fixtureID == "" || edgeFixtureID != fixtureID {
			return nil, invalidGraphError("inconsistent fixture namespace")
		}
		if _, duplicate := opaqueIDs[opaqueID]; duplicate {
			return nil, invalidGraphError("duplicate edge opaque id")
		}
		opaqueIDs[opaqueID] = struct{}{}

		startID := edge.StartID().String()
		endID := edge.EndID().String()
		start, startExists := vertices[startID]
		end, endExists := vertices[endID]
		if !startExists || !endExists {
			return nil, invalidGraphError("missing edge endpoint")
		}
		if start.label != labelUser || end.label != labelIdentifier {
			return nil, invalidGraphError("invalid edge direction")
		}

		logical := logicalRelationship{start: startID, end: endID, label: label}
		if _, duplicate := logicalEdges[logical]; duplicate {
			return nil, invalidGraphError("duplicate logical edge")
		}
		logicalEdges[logical] = struct{}{}
		adjacency[startID] = append(adjacency[startID], endID)
		adjacency[endID] = append(adjacency[endID], startID)
	}

	for backendID := range adjacency {
		sort.Slice(adjacency[backendID], func(i, j int) bool {
			return vertices[adjacency[backendID][i]].opaqueID < vertices[adjacency[backendID][j]].opaqueID
		})
	}
	return adjacency, nil
}

func analyzeComponents(vertices map[string]analyzedVertex, adjacency map[string][]string) Report {
	report := Report{
		Clusters:      make([]Cluster, 0),
		IsolatedUsers: make([]string, 0),
	}
	users := make([]analyzedVertex, 0)
	for _, vertex := range vertices {
		if vertex.label == labelUser {
			users = append(users, vertex)
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].opaqueID < users[j].opaqueID })

	visited := make(map[string]bool, len(vertices))
	for _, start := range users {
		if visited[start.backendID] {
			continue
		}
		componentUsers, identifiers := walkComponent(start.backendID, vertices, adjacency, visited)
		sort.Strings(componentUsers)
		if len(componentUsers) < 2 {
			report.IsolatedUsers = append(report.IsolatedUsers, componentUsers...)
			continue
		}

		cluster := Cluster{
			ClusterID: "cluster:" + componentUsers[0],
			Users:     componentUsers,
			Evidence:  make([]Evidence, 0),
		}
		for backendID := range identifiers {
			userCount := distinctUserCount(adjacency[backendID], vertices)
			if userCount < 2 {
				continue
			}
			identifier := vertices[backendID]
			weight := identifierWeight(identifier.kind)
			cluster.Evidence = append(cluster.Evidence, Evidence{
				Kind:      identifier.kind,
				OpaqueID:  identifier.opaqueID,
				UserCount: userCount,
				Weight:    weight,
			})
			cluster.RiskScore += weight
		}
		sort.Slice(cluster.Evidence, func(i, j int) bool {
			if cluster.Evidence[i].Kind != cluster.Evidence[j].Kind {
				return cluster.Evidence[i].Kind < cluster.Evidence[j].Kind
			}
			return cluster.Evidence[i].OpaqueID < cluster.Evidence[j].OpaqueID
		})
		report.Clusters = append(report.Clusters, cluster)
	}

	sort.Slice(report.Clusters, func(i, j int) bool {
		left, right := report.Clusters[i], report.Clusters[j]
		if left.RiskScore != right.RiskScore {
			return left.RiskScore > right.RiskScore
		}
		if len(left.Users) != len(right.Users) {
			return len(left.Users) > len(right.Users)
		}
		return left.Users[0] < right.Users[0]
	})
	sort.Strings(report.IsolatedUsers)
	return report
}

func walkComponent(start string, vertices map[string]analyzedVertex, adjacency map[string][]string, visited map[string]bool) ([]string, map[string]struct{}) {
	queue := []string{start}
	visited[start] = true
	users := make([]string, 0)
	identifiers := make(map[string]struct{})

	for head := 0; head < len(queue); head++ {
		backendID := queue[head]
		vertex := vertices[backendID]
		if vertex.label == labelUser {
			users = append(users, vertex.opaqueID)
		} else {
			identifiers[backendID] = struct{}{}
		}
		for _, neighbor := range adjacency[backendID] {
			if visited[neighbor] {
				continue
			}
			visited[neighbor] = true
			queue = append(queue, neighbor)
		}
	}
	return users, identifiers
}

func distinctUserCount(backendIDs []string, vertices map[string]analyzedVertex) int {
	users := make(map[string]struct{}, len(backendIDs))
	for _, backendID := range backendIDs {
		vertex := vertices[backendID]
		if vertex.label == labelUser {
			users[backendID] = struct{}{}
		}
	}
	return len(users)
}

func identifierWeight(kind IdentifierKind) int {
	switch kind {
	case IdentifierPaymentToken:
		return 5
	case IdentifierDevice:
		return 3
	case IdentifierIP:
		return 1
	default:
		return 0
	}
}

func nonBlankString(properties graph.Properties, key string) (string, bool) {
	value, ok := properties[key].(string)
	return value, ok && strings.TrimSpace(value) != ""
}

func invalidGraphError(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidGraph, reason)
}
