package graphimport

import (
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

// DefaultFixture 는 README와 CLI가 공유하는 작은 directed account/device graph를 만든다.
func DefaultFixture() (Fixture, error) {
	return NewFixture(PartnerName)
}

// NewFixture 는 partner 이름을 properties에 넣은 deterministic graph를 만든다.
func NewFixture(partner string) (Fixture, error) {
	partner = strings.TrimSpace(partner)
	if partner == "" {
		return Fixture{}, fmt.Errorf("%w: blank partner", ErrInvalidPartner)
	}

	vertices := []graph.Vertex{
		mustVertex("account-a", labelAccount, graph.Properties{
			"partner":    partner,
			"risk_score": int64(72),
			"active":     true,
			"region":     "kr",
		}),
		mustVertex("account-b", labelAccount, graph.Properties{
			"partner":    partner,
			"risk_score": int64(18),
			"active":     true,
			"region":     "us",
		}),
		mustVertex("device-01", labelDevice, graph.Properties{
			"partner": partner,
			"kind":    "mobile",
			"trusted": false,
		}),
		mustVertex("device-02", labelDevice, graph.Properties{
			"partner": partner,
			"kind":    "browser",
			"trusted": true,
		}),
	}
	edges := []graph.Edge{
		mustEdge("use-001", "account-a", "device-01", 0.98),
		mustEdge("use-002", "account-b", "device-01", 0.87),
		mustEdge("use-003", "account-b", "device-02", 0.65),
	}

	records := make([]graphio.Record, 0, len(vertices)+len(edges))
	for _, vertex := range vertices {
		record, err := graphio.VertexRecord(vertex)
		if err != nil {
			return Fixture{}, fmt.Errorf("%w: vertex record", ErrInvalidGraph)
		}
		records = append(records, record)
	}
	for _, edge := range edges {
		record, err := graphio.EdgeRecord(edge)
		if err != nil {
			return Fixture{}, fmt.Errorf("%w: edge record", ErrInvalidGraph)
		}
		records = append(records, record)
	}
	return Fixture{Partner: partner, Records: records}, nil
}

func mustVertex(id, label string, properties graph.Properties) graph.Vertex {
	vertex, err := graph.ParseVertex(id, label, properties)
	if err != nil {
		panic(err)
	}
	return vertex
}

func mustEdge(id, from, to string, confidence float64) graph.Edge {
	edge, err := graph.ParseEdge(id, labelUsesDevice, graph.RawEdgeEndpoints{Start: from, End: to}, graph.Properties{
		"confidence": confidence,
	})
	if err != nil {
		panic(err)
	}
	return edge
}
