package abusecluster

import "github.com/bluetape4k/bluetape-go/graph"

// IdentifierKind classifies an opaque identifier shared by users.
type IdentifierKind string

const (
	IdentifierDevice       IdentifierKind = "device"
	IdentifierIP           IdentifierKind = "ip"
	IdentifierPaymentToken IdentifierKind = "payment_token"

	FixtureID = "graph-abuse-cluster-v1"

	MaxVertices = 256
	MaxEdges    = 1024
)

// Fixture is a bounded graph namespace and its validated graph values.
type Fixture struct {
	ID       string
	Vertices []graph.Vertex
	Edges    []graph.Edge
}

// Evidence describes one shared identifier contributing to cluster risk.
type Evidence struct {
	Kind      IdentifierKind `json:"kind"`
	OpaqueID  string         `json:"opaque_id"`
	UserCount int            `json:"user_count"`
	Weight    int            `json:"weight"`
}

// Cluster groups transitively linked users and their shared evidence.
type Cluster struct {
	ClusterID string     `json:"cluster_id"`
	Users     []string   `json:"users"`
	Evidence  []Evidence `json:"evidence"`
	RiskScore int        `json:"risk_score"`
}

// Report contains deterministically ordered clusters and isolated users.
type Report struct {
	Clusters      []Cluster `json:"clusters"`
	IsolatedUsers []string  `json:"isolated_users"`
}
