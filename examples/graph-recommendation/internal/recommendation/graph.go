package recommendation

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
)

const (
	labelUser      = "User"
	labelProduct   = "Product"
	labelPurchased = "PURCHASED"
	labelFollows   = "FOLLOWS"

	propertyFixtureID = "fixture_id"
	propertyName      = "name"
	propertyCategory  = "category"
	propertyRating    = "rating"
)

type logicalEdgeKey struct {
	label string
	start string
	end   string
}

// NewGraph 는 graph.Vertex/graph.Edge를 검증하고 읽기 전용 인덱스를 만듭니다.
func NewGraph(vertices []graph.Vertex, edges []graph.Edge) (*Graph, error) {
	if len(vertices) > MaxVertices || len(edges) > MaxEdges {
		return nil, fmt.Errorf("%w: limit exceeded", ErrGraphTooLarge)
	}

	result := &Graph{
		vertices:           make(map[string]graph.Vertex, len(vertices)),
		users:              make(map[string]graph.Vertex),
		products:           make(map[string]graph.Vertex),
		purchasesByUser:    make(map[string][]purchase),
		purchasesByProduct: make(map[string][]purchase),
		followsByUser:      make(map[string][]follow),
	}
	fixtureID := ""
	for _, vertex := range vertices {
		if err := vertex.Validate(); err != nil {
			return nil, invalidGraph("invalid vertex")
		}
		id := vertex.ID().String()
		if _, exists := result.vertices[id]; exists {
			return nil, invalidGraph("duplicate vertex")
		}
		properties := vertex.Properties()
		vertexFixtureID, ok := stringProperty(properties, propertyFixtureID)
		if !ok {
			return nil, invalidGraph("invalid vertex properties")
		}
		if fixtureID == "" {
			fixtureID = vertexFixtureID
		} else if fixtureID != vertexFixtureID {
			return nil, invalidGraph("inconsistent namespace")
		}
		switch vertex.Label().String() {
		case labelUser:
			if len(properties) != 2 || !nonBlankProperty(properties, propertyName) {
				return nil, invalidGraph("invalid user properties")
			}
			result.users[id] = vertex
		case labelProduct:
			if len(properties) != 3 || !nonBlankProperty(properties, propertyName) || !nonBlankProperty(properties, propertyCategory) {
				return nil, invalidGraph("invalid product properties")
			}
			result.products[id] = vertex
		default:
			return nil, invalidGraph("invalid vertex label")
		}
		result.vertices[id] = vertex
	}

	edgeIDs := make(map[string]struct{}, len(edges))
	logicalEdges := make(map[logicalEdgeKey]struct{}, len(edges))
	for _, edge := range edges {
		if err := edge.Validate(); err != nil {
			return nil, invalidGraph("invalid edge")
		}
		edgeID := edge.ID().String()
		if _, duplicate := edgeIDs[edgeID]; duplicate {
			return nil, invalidGraph("duplicate edge")
		}
		edgeIDs[edgeID] = struct{}{}
		properties := edge.Properties()
		edgeFixtureID, ok := stringProperty(properties, propertyFixtureID)
		if !ok || (fixtureID != "" && edgeFixtureID != fixtureID) {
			return nil, invalidGraph("invalid edge namespace")
		}
		startID := edge.StartID().String()
		endID := edge.EndID().String()
		if _, exists := result.vertices[startID]; !exists {
			return nil, invalidGraph("missing edge start")
		}
		if _, exists := result.vertices[endID]; !exists {
			return nil, invalidGraph("missing edge end")
		}
		logicalKey := logicalEdgeKey{label: edge.Label().String(), start: startID, end: endID}
		if _, duplicate := logicalEdges[logicalKey]; duplicate {
			return nil, invalidGraph("duplicate logical edge")
		}
		logicalEdges[logicalKey] = struct{}{}

		switch edge.Label().String() {
		case labelPurchased:
			if len(properties) != 2 || !isValidRating(properties[propertyRating]) {
				return nil, invalidGraph("invalid purchase properties")
			}
			if _, user := result.users[startID]; !user {
				return nil, invalidGraph("purchase start is not user")
			}
			if _, product := result.products[endID]; !product {
				return nil, invalidGraph("purchase end is not product")
			}
			result.purchasesByUser[startID] = append(result.purchasesByUser[startID], purchase{
				edge: edge, userID: startID, productID: endID, rating: properties[propertyRating].(int),
			})
			result.purchasesByProduct[endID] = append(result.purchasesByProduct[endID], purchase{
				edge: edge, userID: startID, productID: endID, rating: properties[propertyRating].(int),
			})
		case labelFollows:
			if len(properties) != 1 {
				return nil, invalidGraph("invalid follow properties")
			}
			if _, user := result.users[startID]; !user {
				return nil, invalidGraph("follow start is not user")
			}
			if _, user := result.users[endID]; !user || startID == endID {
				return nil, invalidGraph("invalid follow endpoints")
			}
			result.followsByUser[startID] = append(result.followsByUser[startID], follow{
				edge: edge, follower: startID, followee: endID,
			})
		default:
			return nil, invalidGraph("invalid edge label")
		}
	}

	result.fixtureID = fixtureID
	for userID := range result.purchasesByUser {
		sort.Slice(result.purchasesByUser[userID], func(i, j int) bool {
			left, right := result.purchasesByUser[userID][i], result.purchasesByUser[userID][j]
			if left.productID != right.productID {
				return left.productID < right.productID
			}
			return left.edge.ID().String() < right.edge.ID().String()
		})
	}
	for productID := range result.purchasesByProduct {
		sort.Slice(result.purchasesByProduct[productID], func(i, j int) bool {
			left, right := result.purchasesByProduct[productID][i], result.purchasesByProduct[productID][j]
			if left.userID != right.userID {
				return left.userID < right.userID
			}
			return left.edge.ID().String() < right.edge.ID().String()
		})
	}
	for userID := range result.followsByUser {
		sort.Slice(result.followsByUser[userID], func(i, j int) bool {
			left, right := result.followsByUser[userID][i], result.followsByUser[userID][j]
			if left.followee != right.followee {
				return left.followee < right.followee
			}
			return left.edge.ID().String() < right.edge.ID().String()
		})
	}
	return result, nil
}

func stringProperty(properties graph.Properties, key string) (string, bool) {
	value, ok := properties[key].(string)
	return value, ok && strings.TrimSpace(value) != ""
}

func nonBlankProperty(properties graph.Properties, key string) bool {
	value, ok := stringProperty(properties, key)
	return ok && strings.TrimSpace(value) != ""
}

func isValidRating(value any) bool {
	rating, ok := value.(int)
	return ok && rating >= 1 && rating <= 5
}

func invalidGraph(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidGraph, reason)
}

func (g *Graph) validateRequest(ctx context.Context, seedID string, limit int) error {
	if g == nil {
		return ErrInvalidGraph
	}
	if ctx == nil {
		return ErrInvalidContext
	}
	if limit < 0 {
		return ErrInvalidLimit
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := g.users[seedID]; !ok {
		return ErrUnknownSeed
	}
	return nil
}

func (g *Graph) productPurchase(userID, productID string) (purchase, bool) {
	for _, item := range g.purchasesByUser[userID] {
		if item.productID == productID {
			return item, true
		}
	}
	return purchase{}, false
}

func (g *Graph) followEdge(followerID, followeeID string) (follow, bool) {
	for _, item := range g.followsByUser[followerID] {
		if item.followee == followeeID {
			return item, true
		}
	}
	return follow{}, false
}

func (g *Graph) productEvidencePath(seedID, sharedProductID, coBuyerID, candidateID string) (graph.Path, error) {
	seedPurchase, seedOK := g.productPurchase(seedID, sharedProductID)
	coBuyerShared, sharedOK := g.productPurchase(coBuyerID, sharedProductID)
	coBuyerCandidate, candidateOK := g.productPurchase(coBuyerID, candidateID)
	if !seedOK || !sharedOK || !candidateOK {
		return graph.EmptyPath(), ErrInvalidGraph
	}
	steps := make([]graph.PathStep, 0, 7)
	for _, value := range []any{
		g.vertices[seedID], seedPurchase.edge, g.vertices[sharedProductID],
		coBuyerShared.edge, g.vertices[coBuyerID], coBuyerCandidate.edge, g.vertices[candidateID],
	} {
		step, err := pathStep(value)
		if err != nil {
			return graph.EmptyPath(), err
		}
		steps = append(steps, step)
	}
	return graph.NewPath(steps...)
}

func (g *Graph) followEvidencePath(seedID, viaUserID, candidateID string) (graph.Path, error) {
	first, firstOK := g.followEdge(seedID, viaUserID)
	second, secondOK := g.followEdge(viaUserID, candidateID)
	if !firstOK || !secondOK {
		return graph.EmptyPath(), ErrInvalidGraph
	}
	steps := make([]graph.PathStep, 0, 5)
	for _, value := range []any{
		g.vertices[seedID], first.edge, g.vertices[viaUserID], second.edge, g.vertices[candidateID],
	} {
		step, err := pathStep(value)
		if err != nil {
			return graph.EmptyPath(), err
		}
		steps = append(steps, step)
	}
	return graph.NewPath(steps...)
}

func pathStep(value any) (graph.PathStep, error) {
	switch typed := value.(type) {
	case graph.Vertex:
		return graph.VertexStep(typed)
	case graph.Edge:
		return graph.EdgeStep(typed)
	default:
		return graph.PathStep{}, ErrInvalidGraph
	}
}
