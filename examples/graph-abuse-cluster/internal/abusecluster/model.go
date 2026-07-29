package abusecluster

import "github.com/bluetape4k/bluetape-go/graph"

// IdentifierKind 는 user들이 공유하는 opaque identifier를 분류한다.
type IdentifierKind string

// IdentifierDevice 와 관련 상수들은 제한된 fixture 계약을 정의한다.
const (
	IdentifierDevice       IdentifierKind = "device"
	IdentifierIP           IdentifierKind = "ip"
	IdentifierPaymentToken IdentifierKind = "payment_token"

	FixtureID = "graph-abuse-cluster-v1"

	MaxVertices = 256
	MaxEdges    = 1024
)

// Fixture 는 제한된 graph namespace와 검증된 graph value 묶음이다.
type Fixture struct {
	ID       string
	Vertices []graph.Vertex
	Edges    []graph.Edge
}

// Evidence 는 cluster risk에 기여하는 shared identifier 하나를 설명한다.
type Evidence struct {
	Kind      IdentifierKind `json:"kind"`
	OpaqueID  string         `json:"opaque_id"`
	UserCount int            `json:"user_count"`
	Weight    int            `json:"weight"`
}

// Cluster 는 전이적으로 연결된 user와 그 shared evidence를 묶는다.
type Cluster struct {
	ClusterID string     `json:"cluster_id"`
	Users     []string   `json:"users"`
	Evidence  []Evidence `json:"evidence"`
	RiskScore int        `json:"risk_score"`
}

// Report 는 결정적으로 정렬된 cluster와 isolated user를 담는다.
type Report struct {
	Clusters      []Cluster `json:"clusters"`
	IsolatedUsers []string  `json:"isolated_users"`
}
