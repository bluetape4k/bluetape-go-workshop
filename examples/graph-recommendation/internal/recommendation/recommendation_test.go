package recommendation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestDefaultFixtureProducesDeterministicRecommendations(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	if got, want := len(fixture.Vertices), 12; got != want {
		t.Fatalf("len(vertices) = %d, want %d", got, want)
	}
	if got, want := len(fixture.Edges), 25; got != want {
		t.Fatalf("len(edges) = %d, want %d", got, want)
	}

	graphValue, err := NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	report, err := graphValue.Recommend(context.Background(), "alice", 10)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	wantProducts := []ProductRecommendation{
		{ProductID: "headphones", Score: 3, Evidence: []ProductEvidence{
			{CoBuyerID: "bob", SharedProductID: "laptop", Path: []string{"alice", "laptop", "bob", "headphones"}},
			{CoBuyerID: "carol", SharedProductID: "phone", Path: []string{"alice", "phone", "carol", "headphones"}},
			{CoBuyerID: "dave", SharedProductID: "tablet", Path: []string{"alice", "tablet", "dave", "headphones"}},
		}},
		{ProductID: "keyboard", Score: 1, Evidence: []ProductEvidence{
			{CoBuyerID: "eve", SharedProductID: "laptop", Path: []string{"alice", "laptop", "eve", "keyboard"}},
		}},
		{ProductID: "mouse", Score: 1, Evidence: []ProductEvidence{
			{CoBuyerID: "frank", SharedProductID: "phone", Path: []string{"alice", "phone", "frank", "mouse"}},
		}},
	}
	wantFollows := []FollowRecommendation{
		{UserID: "dave", Score: 1, Evidence: []FollowEvidence{
			{ViaUserID: "bob", Path: []string{"alice", "bob", "dave"}},
		}},
		{UserID: "eve", Score: 1, Evidence: []FollowEvidence{
			{ViaUserID: "carol", Path: []string{"alice", "carol", "eve"}},
		}},
	}

	if got := report.ProductRecommendations; !reflect.DeepEqual(stripProductPaths(got), wantProducts) {
		t.Fatalf("product recommendations = %#v, want %#v", got, wantProducts)
	}
	if got := report.FollowRecommendations; !reflect.DeepEqual(stripFollowPaths(got), wantFollows) {
		t.Fatalf("follow recommendations = %#v, want %#v", got, wantFollows)
	}
	for _, recommendation := range report.ProductRecommendations {
		for _, evidence := range recommendation.Evidence {
			if path := evidence.GraphPath(); path.Length() != 3 || len(path.Vertices()) != 4 {
				t.Fatalf("product evidence path = %#v, want 3 edges and 4 vertices", path)
			}
		}
	}
	for _, recommendation := range report.FollowRecommendations {
		for _, evidence := range recommendation.Evidence {
			if path := evidence.GraphPath(); path.Length() != 2 || len(path.Vertices()) != 3 {
				t.Fatalf("follow evidence path = %#v, want 2 edges and 3 vertices", path)
			}
		}
	}
}

func TestRecommendationsExcludeSeedPurchasesAndDirectFollows(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	graphValue, err := NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	products, err := graphValue.RecommendProducts(context.Background(), "alice", 0)
	if err != nil {
		t.Fatalf("RecommendProducts() error = %v", err)
	}
	follows, err := graphValue.RecommendFollows(context.Background(), "alice", 0)
	if err != nil {
		t.Fatalf("RecommendFollows() error = %v", err)
	}

	for _, recommendation := range products {
		if recommendation.ProductID == "laptop" || recommendation.ProductID == "phone" || recommendation.ProductID == "tablet" {
			t.Errorf("already purchased product was recommended: %q", recommendation.ProductID)
		}
	}
	for _, recommendation := range follows {
		if recommendation.UserID == "alice" || recommendation.UserID == "bob" || recommendation.UserID == "carol" {
			t.Errorf("self or already-followed user was recommended: %q", recommendation.UserID)
		}
	}
}

func TestRecommendProductsExcludesEverySeedPurchaseBeforeScoring(t *testing.T) {
	const fixtureID = "seed-purchase-boundary"
	vertices := []graph.Vertex{
		mustVertex(t, "alice", labelUser, graph.Properties{propertyFixtureID: fixtureID, propertyName: "Alice"}),
		mustVertex(t, "bob", labelUser, graph.Properties{propertyFixtureID: fixtureID, propertyName: "Bob"}),
		mustVertex(t, "p-a", labelProduct, graph.Properties{propertyFixtureID: fixtureID, propertyName: "A", propertyCategory: "Test"}),
		mustVertex(t, "p-b", labelProduct, graph.Properties{propertyFixtureID: fixtureID, propertyName: "B", propertyCategory: "Test"}),
	}
	edges := []graph.Edge{
		mustEdge(t, "purchase-1", labelPurchased, "alice", "p-a", fixtureID, 5),
		mustEdge(t, "purchase-2", labelPurchased, "alice", "p-b", fixtureID, 5),
		mustEdge(t, "purchase-3", labelPurchased, "bob", "p-a", fixtureID, 5),
		mustEdge(t, "purchase-4", labelPurchased, "bob", "p-b", fixtureID, 5),
	}
	graphValue, err := NewGraph(vertices, edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	products, err := graphValue.RecommendProducts(context.Background(), "alice", 0)
	if err != nil {
		t.Fatalf("RecommendProducts() error = %v", err)
	}
	if len(products) != 0 {
		t.Fatalf("products = %#v, want no candidates after excluding all seed purchases", products)
	}
}

func TestRecommendSupportsEmptyCandidatesAndLimit(t *testing.T) {
	fixture, err := NewFixture("empty-candidates")
	if err != nil {
		t.Fatalf("NewFixture() error = %v", err)
	}
	fixture.Vertices = fixture.Vertices[:1]
	fixture.Edges = nil
	graphValue, err := NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	report, err := graphValue.Recommend(context.Background(), "alice", 1)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	if report.ProductRecommendations == nil || report.FollowRecommendations == nil {
		t.Fatalf("empty result slices must be non-nil: %#v", report)
	}
	if len(report.ProductRecommendations) != 0 || len(report.FollowRecommendations) != 0 {
		t.Fatalf("empty candidates report = %#v", report)
	}

	fixture, err = DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	graphValue, err = NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	products, err := graphValue.RecommendProducts(context.Background(), "alice", 2)
	if err != nil {
		t.Fatalf("RecommendProducts(limit=2) error = %v", err)
	}
	if got, want := len(products), 2; got != want {
		t.Fatalf("limited product count = %d, want %d", got, want)
	}
}

func TestNewGraphRejectsInvalidProperties(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	properties := fixture.Vertices[0].Properties()
	properties["unexpected"] = true
	fixture.Vertices[0] = mustVertex(t, fixture.Vertices[0].ID().String(), fixture.Vertices[0].Label().String(), properties)
	_, err = NewGraph(fixture.Vertices, fixture.Edges)
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("NewGraph() error = %v, want ErrInvalidGraph", err)
	}
	if err != nil && len(err.Error()) > 256 {
		t.Fatalf("invalid graph error is unexpectedly verbose: %q", err)
	}
}

func TestNewGraphAcceptsDistinctLogicalEdgesWithNULIDs(t *testing.T) {
	const fixtureID = "nul-key-boundary"
	vertices := []graph.Vertex{
		mustVertex(t, "user-a", labelUser, graph.Properties{propertyFixtureID: fixtureID, propertyName: "A"}),
		mustVertex(t, "user-a\x00", labelUser, graph.Properties{propertyFixtureID: fixtureID, propertyName: "A NUL"}),
		mustVertex(t, "\x00product-b", labelProduct, graph.Properties{propertyFixtureID: fixtureID, propertyName: "NUL B", propertyCategory: "Test"}),
		mustVertex(t, "product-b", labelProduct, graph.Properties{propertyFixtureID: fixtureID, propertyName: "B", propertyCategory: "Test"}),
	}
	edges := []graph.Edge{
		mustEdge(t, "purchase-1", labelPurchased, "user-a", "\x00product-b", fixtureID, 5),
		mustEdge(t, "purchase-2", labelPurchased, "user-a\x00", "product-b", fixtureID, 5),
	}
	if _, err := NewGraph(vertices, edges); err != nil {
		t.Fatalf("NewGraph() error = %v, want distinct logical edges to remain distinct", err)
	}
}

func TestRecommendHonorsCancellationAndInvalidArguments(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	graphValue, err := NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = graphValue.Recommend(ctx, "alice", 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Recommend() error = %v, want context.Canceled", err)
	}
	_, err = graphValue.Recommend(context.Background(), "alice", -1)
	if !errors.Is(err, ErrInvalidLimit) {
		t.Fatalf("Recommend(limit=-1) error = %v, want ErrInvalidLimit", err)
	}
	_, err = graphValue.Recommend(context.Background(), "missing", 10)
	if !errors.Is(err, ErrUnknownSeed) {
		t.Fatalf("Recommend(missing) error = %v, want ErrUnknownSeed", err)
	}
}

func TestRecommendationsAreIndependentOfRecordOrder(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	baselineGraph, err := NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("NewGraph() error = %v", err)
	}
	baseline, err := baselineGraph.Recommend(context.Background(), "alice", 0)
	if err != nil {
		t.Fatalf("baseline Recommend() error = %v", err)
	}

	reverseVertices(fixture.Vertices)
	reverseEdges(fixture.Edges)
	shuffledGraph, err := NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("reordered NewGraph() error = %v", err)
	}
	got, err := shuffledGraph.Recommend(context.Background(), "alice", 0)
	if err != nil {
		t.Fatalf("reordered Recommend() error = %v", err)
	}
	if !reflect.DeepEqual(stripReportPaths(got), stripReportPaths(baseline)) {
		t.Fatalf("reordered report = %#v, baseline = %#v", got, baseline)
	}
}

func mustVertex(t *testing.T, id, label string, properties graph.Properties) graph.Vertex {
	t.Helper()
	vertex, err := graph.ParseVertex(id, label, properties)
	if err != nil {
		t.Fatalf("graph.ParseVertex() error = %v", err)
	}
	return vertex
}

func mustEdge(t *testing.T, id, label, startID, endID, fixtureID string, rating int) graph.Edge {
	t.Helper()
	edge, err := graph.ParseEdge(
		id,
		label,
		graph.RawEdgeEndpoints{Start: startID, End: endID},
		graph.Properties{propertyFixtureID: fixtureID, propertyRating: rating},
	)
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

func stripProductPaths(recommendations []ProductRecommendation) []ProductRecommendation {
	cloned := make([]ProductRecommendation, len(recommendations))
	for i, recommendation := range recommendations {
		cloned[i] = recommendation
		cloned[i].Evidence = make([]ProductEvidence, len(recommendation.Evidence))
		for j, evidence := range recommendation.Evidence {
			cloned[i].Evidence[j] = evidence
			cloned[i].Evidence[j].graphPath = graph.EmptyPath()
		}
	}
	return cloned
}

func stripFollowPaths(recommendations []FollowRecommendation) []FollowRecommendation {
	cloned := make([]FollowRecommendation, len(recommendations))
	for i, recommendation := range recommendations {
		cloned[i] = recommendation
		cloned[i].Evidence = make([]FollowEvidence, len(recommendation.Evidence))
		for j, evidence := range recommendation.Evidence {
			cloned[i].Evidence[j] = evidence
			cloned[i].Evidence[j].graphPath = graph.EmptyPath()
		}
	}
	return cloned
}

func stripReportPaths(report Report) Report {
	report.ProductRecommendations = stripProductPaths(report.ProductRecommendations)
	report.FollowRecommendations = stripFollowPaths(report.FollowRecommendations)
	return report
}
