package recommendation

import (
	"github.com/bluetape4k/bluetape-go/graph"
)

const (
	// FixtureID 는 문서와 CLI가 함께 사용하는 결정론적 namespace입니다.
	FixtureID = "graph-recommendation-v1"
	// MaxVertices 는 예제 입력의 정점 메모리 경계를 고정합니다.
	MaxVertices = 256
	// MaxEdges 는 예제 입력의 엣지 메모리 경계를 고정합니다.
	MaxEdges = 1024
)

// Fixture 는 검증된 graph model 값과 namespace를 묶습니다.
type Fixture struct {
	ID       string
	Vertices []graph.Vertex
	Edges    []graph.Edge
}

// ProductRecommendation 은 공동구매자 수로 정렬된 상품 후보입니다.
type ProductRecommendation struct {
	ProductID string            `json:"product_id"`
	Score     int               `json:"score"`
	Evidence  []ProductEvidence `json:"evidence"`
}

// ProductEvidence 는 후보 상품으로 이어지는 한 개의 공동구매 경로입니다.
type ProductEvidence struct {
	CoBuyerID       string   `json:"co_buyer_id"`
	SharedProductID string   `json:"shared_product_id"`
	Path            []string `json:"path"`

	graphPath graph.Path
}

// GraphPath 는 출력에서 제외한 graph.Path를 검증 코드가 확인하도록 반환합니다.
func (e ProductEvidence) GraphPath() graph.Path {
	return e.graphPath
}

// FollowRecommendation 은 두 홉 FOLLOWS 증거로 정렬된 사용자 후보입니다.
type FollowRecommendation struct {
	UserID   string           `json:"user_id"`
	Score    int              `json:"score"`
	Evidence []FollowEvidence `json:"evidence"`
}

// FollowEvidence 는 후보 사용자로 이어지는 한 개의 FOAF 경로입니다.
type FollowEvidence struct {
	ViaUserID string   `json:"via_user_id"`
	Path      []string `json:"path"`

	graphPath graph.Path
}

// GraphPath 는 출력에서 제외한 graph.Path를 검증 코드가 확인하도록 반환합니다.
func (e FollowEvidence) GraphPath() graph.Path {
	return e.graphPath
}

// Report 는 한 seed user에 대한 두 추천 결과를 담습니다.
type Report struct {
	SeedUser               string                  `json:"seed_user"`
	ProductRecommendations []ProductRecommendation `json:"product_recommendations"`
	FollowRecommendations  []FollowRecommendation  `json:"follow_recommendations"`
}

type purchase struct {
	edge      graph.Edge
	userID    string
	productID string
	rating    int
}

type follow struct {
	edge     graph.Edge
	follower string
	followee string
}

// Graph 는 caller가 소유한 graph model을 검증한 뒤 읽기 전용 index로 보관합니다.
type Graph struct {
	fixtureID string
	vertices  map[string]graph.Vertex
	users     map[string]graph.Vertex
	products  map[string]graph.Vertex

	purchasesByUser    map[string][]purchase
	purchasesByProduct map[string][]purchase
	followsByUser      map[string][]follow
}
