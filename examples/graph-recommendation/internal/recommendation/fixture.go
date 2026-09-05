package recommendation

import (
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
)

type namedValue struct {
	id       string
	name     string
	category string
}

type purchaseSpec struct {
	userID    string
	productID string
	rating    int
}

type followSpec struct {
	follower string
	followee string
}

var fixtureUsers = [...]namedValue{
	{id: "alice", name: "Alice"},
	{id: "bob", name: "Bob"},
	{id: "carol", name: "Carol"},
	{id: "dave", name: "Dave"},
	{id: "eve", name: "Eve"},
	{id: "frank", name: "Frank"},
}

var fixtureProducts = [...]namedValue{
	{id: "laptop", name: "Laptop", category: "Electronics"},
	{id: "phone", name: "Phone", category: "Electronics"},
	{id: "tablet", name: "Tablet", category: "Electronics"},
	{id: "headphones", name: "Headphones", category: "Accessories"},
	{id: "keyboard", name: "Keyboard", category: "Accessories"},
	{id: "mouse", name: "Mouse", category: "Accessories"},
}

var fixturePurchases = [...]purchaseSpec{
	{userID: "alice", productID: "laptop", rating: 5},
	{userID: "alice", productID: "phone", rating: 4},
	{userID: "alice", productID: "tablet", rating: 3},
	{userID: "bob", productID: "laptop", rating: 4},
	{userID: "bob", productID: "headphones", rating: 5},
	{userID: "carol", productID: "phone", rating: 5},
	{userID: "carol", productID: "headphones", rating: 4},
	{userID: "dave", productID: "tablet", rating: 4},
	{userID: "dave", productID: "headphones", rating: 3},
	{userID: "eve", productID: "laptop", rating: 3},
	{userID: "eve", productID: "keyboard", rating: 5},
	{userID: "frank", productID: "phone", rating: 3},
	{userID: "frank", productID: "mouse", rating: 4},
}

var fixtureFollows = [...]followSpec{
	{follower: "alice", followee: "bob"},
	{follower: "alice", followee: "carol"},
	{follower: "bob", followee: "dave"},
	{follower: "bob", followee: "carol"},
	{follower: "carol", followee: "eve"},
	{follower: "carol", followee: "bob"},
	{follower: "dave", followee: "frank"},
	{follower: "dave", followee: "eve"},
	{follower: "eve", followee: "frank"},
	{follower: "eve", followee: "dave"},
	{follower: "frank", followee: "alice"},
	{follower: "frank", followee: "bob"},
}

// DefaultFixture 는 README와 CLI가 공유하는 6-user/6-product graph를 만듭니다.
func DefaultFixture() (Fixture, error) {
	return NewFixture(FixtureID)
}

// NewFixture 는 지정한 namespace로 독립적인 recommendation graph를 만듭니다.
func NewFixture(fixtureID string) (Fixture, error) {
	if strings.TrimSpace(fixtureID) == "" {
		return Fixture{}, fmt.Errorf("%w: blank namespace", ErrInvalidFixture)
	}

	fixture := Fixture{
		ID:       fixtureID,
		Vertices: make([]graph.Vertex, 0, len(fixtureUsers)+len(fixtureProducts)),
		Edges:    make([]graph.Edge, 0, len(fixturePurchases)+len(fixtureFollows)),
	}
	for _, value := range fixtureUsers {
		vertex, err := graph.ParseVertex(value.id, labelUser, graph.Properties{
			propertyFixtureID: fixtureID,
			propertyName:      value.name,
		})
		if err != nil {
			return Fixture{}, fmt.Errorf("%w: user vertex", ErrInvalidFixture)
		}
		fixture.Vertices = append(fixture.Vertices, vertex)
	}
	for _, value := range fixtureProducts {
		vertex, err := graph.ParseVertex(value.id, labelProduct, graph.Properties{
			propertyFixtureID: fixtureID,
			propertyName:      value.name,
			propertyCategory:  value.category,
		})
		if err != nil {
			return Fixture{}, fmt.Errorf("%w: product vertex", ErrInvalidFixture)
		}
		fixture.Vertices = append(fixture.Vertices, vertex)
	}
	for index, purchase := range fixturePurchases {
		edge, err := graph.ParseEdge(
			fmt.Sprintf("purchase-%03d", index+1),
			labelPurchased,
			graph.RawEdgeEndpoints{Start: purchase.userID, End: purchase.productID},
			graph.Properties{
				propertyFixtureID: fixtureID,
				propertyRating:    purchase.rating,
			},
		)
		if err != nil {
			return Fixture{}, fmt.Errorf("%w: purchased edge", ErrInvalidFixture)
		}
		fixture.Edges = append(fixture.Edges, edge)
	}
	for index, follow := range fixtureFollows {
		edge, err := graph.ParseEdge(
			fmt.Sprintf("follow-%03d", index+1),
			labelFollows,
			graph.RawEdgeEndpoints{Start: follow.follower, End: follow.followee},
			graph.Properties{propertyFixtureID: fixtureID},
		)
		if err != nil {
			return Fixture{}, fmt.Errorf("%w: follows edge", ErrInvalidFixture)
		}
		fixture.Edges = append(fixture.Edges, edge)
	}
	if _, err := NewGraph(fixture.Vertices, fixture.Edges); err != nil {
		return Fixture{}, fmt.Errorf("%w: validation failed", ErrInvalidFixture)
	}
	return fixture, nil
}
