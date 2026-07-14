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
