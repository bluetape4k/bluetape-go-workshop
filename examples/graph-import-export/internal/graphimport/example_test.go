package graphimport_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-import-export/internal/graphimport"
)

func ExampleImport() {
	fixture, _ := graphimport.DefaultFixture()
	var encoded bytes.Buffer
	_, _ = graphimport.Export(context.Background(), graphimport.FormatNDJSON, &encoded, fixture.Records)
	result, _, _ := graphimport.Import(context.Background(), fixture.Partner, graphimport.FormatNDJSON, strings.NewReader(encoded.String()), graphimport.DefaultOptions())

	fmt.Println(result.Partner, result.Summary.Vertices, result.Summary.Edges)
	// Output: acme-payments 4 3
}

func ExampleExport() {
	fixture, _ := graphimport.DefaultFixture()
	var encoded bytes.Buffer
	report, _ := graphimport.Export(context.Background(), graphimport.FormatGraphML, &encoded, fixture.Records)

	fmt.Println(report.Format, report.VerticesWritten, report.EdgesWritten, strings.Contains(encoded.String(), `edgedefault="directed"`))
	// Output: graphml 4 3 true
}
