package graphimport

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

func TestDefaultFixtureRoundTripsThroughNDJSONAndGraphML(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	if got, want := len(fixture.Records), 7; got != want {
		t.Fatalf("fixture records = %d, want %d", got, want)
	}

	var baseline Snapshot
	for _, format := range []Format{FormatNDJSON, FormatGraphML} {
		t.Run(string(format), func(t *testing.T) {
			var encoded bytes.Buffer
			writeReport, err := Export(context.Background(), format, &encoded, fixture.Records)
			if err != nil {
				t.Fatalf("Export() error = %v", err)
			}
			if writeReport.VerticesWritten != 4 || writeReport.EdgesWritten != 3 {
				t.Fatalf("write report = %+v", writeReport)
			}

			result, readReport, err := Import(context.Background(), fixture.Partner, format, strings.NewReader(encoded.String()), DefaultOptions())
			if err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			if readReport.VerticesRead != 4 || readReport.EdgesRead != 3 {
				t.Fatalf("read report = %+v", readReport)
			}
			if result.Summary.Vertices != 4 || result.Summary.Edges != 3 || result.Summary.Accounts != 2 || result.Summary.Devices != 2 {
				t.Fatalf("summary = %+v", result.Summary)
			}
			if !result.Summary.Directed {
				t.Fatalf("summary.Directed = false, want true")
			}
			if format == FormatNDJSON {
				baseline = result.Snapshot
			} else if !reflect.DeepEqual(result.Snapshot, baseline) {
				t.Fatalf("GraphML snapshot = %#v, NDJSON snapshot = %#v", result.Snapshot, baseline)
			}
			assertCanonicalSnapshot(t, result.Snapshot)
		})
	}
}

func TestGraphMLPreservesIntegralFloatPropertyType(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	records := make([]graphio.Record, 0, len(fixture.Records))
	for _, record := range fixture.Records {
		if record.Kind != graphio.RecordEdge {
			records = append(records, record)
			continue
		}
		edge := record.Edge
		properties := edge.Properties()
		properties["confidence"] = float64(1)
		replacement, err := graph.ParseEdge(edge.ID().String(), edge.Label().String(), graph.RawEdgeEndpoints{
			Start: edge.StartID().String(),
			End:   edge.EndID().String(),
		}, properties)
		if err != nil {
			t.Fatalf("ParseEdge() error = %v", err)
		}
		record, err = graphio.EdgeRecord(replacement)
		if err != nil {
			t.Fatalf("EdgeRecord() error = %v", err)
		}
		records = append(records, record)
	}

	var encoded bytes.Buffer
	if _, err := Export(context.Background(), FormatGraphML, &encoded, records); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	result, _, err := Import(context.Background(), fixture.Partner, FormatGraphML, strings.NewReader(encoded.String()), DefaultOptions())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	value := result.Snapshot.Edges[0].Properties["confidence"]
	if got, ok := value.(float64); !ok || got != 1 {
		t.Fatalf("GraphML integral float property = %#v (%T), want float64(1)", value, value)
	}
}

func TestExportCanonicalizesRecordOrder(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	reversed := reverseRecords(fixture.Records)

	var encoded bytes.Buffer
	if _, err := Export(context.Background(), FormatNDJSON, &encoded, reversed); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	wantPrefix := `{"type":"vertex","id":"account-a"`
	if !strings.HasPrefix(encoded.String(), wantPrefix) {
		t.Fatalf("canonical NDJSON = %s, want prefix %s", encoded.String(), wantPrefix)
	}
	a := strings.Index(encoded.String(), `"id":"account-a"`)
	b := strings.Index(encoded.String(), `"id":"account-b"`)
	if a < 0 || b <= a {
		t.Fatalf("account IDs are not sorted: %s", encoded.String())
	}
}

func TestImportRejectsDuplicateMissingAndNonScalarRecords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []error
	}{
		{
			name: "duplicate vertex",
			input: "{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"acme-payments"}` + "}\n" +
				"{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"acme-payments"}` + "}\n",
			want: []error{graphio.ErrDuplicateVertex},
		},
		{
			name: "missing endpoint",
			input: "{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"acme-payments"}` + "}\n" +
				"{" + `"type":"edge","id":"use-001","label":"USES_DEVICE","from":"account-a","to":"device-missing","properties":{}` + "}\n",
			want: []error{graphio.ErrMissingEndpoint},
		},
		{
			name: "duplicate edge",
			input: "{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"acme-payments"}` + "}\n" +
				"{" + `"type":"vertex","id":"device-01","label":"Device","properties":{"partner":"acme-payments"}` + "}\n" +
				"{" + `"type":"edge","id":"use-001","label":"USES_DEVICE","from":"account-a","to":"device-01","properties":{}` + "}\n" +
				"{" + `"type":"edge","id":"use-001","label":"USES_DEVICE","from":"account-a","to":"device-01","properties":{}` + "}\n",
			want: []error{ErrDuplicateRecord},
		},
		{
			name:  "non scalar property",
			input: "{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"acme-payments","tags":["risky"]}` + "}\n",
			want:  []error{ErrNonScalarProperty},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Import(context.Background(), "acme-payments", FormatNDJSON, strings.NewReader(tt.input), DefaultOptions())
			if err == nil {
				t.Fatal("Import() error = nil, want failure")
			}
			for _, want := range tt.want {
				if !errors.Is(err, want) {
					t.Fatalf("Import() error = %v, want %v", err, want)
				}
			}
		})
	}
}

func TestImportRejectsWrongPartnerAndDirection(t *testing.T) {
	wrongPartner := "{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"other"}` + "}\n"
	_, _, err := Import(context.Background(), "acme-payments", FormatNDJSON, strings.NewReader(wrongPartner), DefaultOptions())
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("wrong partner error = %v, want ErrInvalidGraph", err)
	}

	reversed := "{" + `"type":"vertex","id":"account-a","label":"Account","properties":{"partner":"acme-payments"}` + "}\n" +
		"{" + `"type":"vertex","id":"device-01","label":"Device","properties":{"partner":"acme-payments"}` + "}\n" +
		"{" + `"type":"edge","id":"use-001","label":"USES_DEVICE","from":"device-01","to":"account-a","properties":{}}` + "\n"
	_, _, err = Import(context.Background(), "acme-payments", FormatNDJSON, strings.NewReader(reversed), DefaultOptions())
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("reversed edge error = %v, want ErrInvalidGraph", err)
	}
}

func TestImportRejectsGraphMLUnknownKeyAndUnsupportedConstruct(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "unknown key",
			xml:  `<graphml><graph edgedefault="directed"><node id="account-a"><data key="missing">Account</data></node></graph></graphml>`,
		},
		{
			name: "nested graph",
			xml:  `<graphml><graph edgedefault="directed"><node id="account-a"><graph edgedefault="directed"/></node></graph></graphml>`,
		},
		{
			name: "xml directive",
			xml:  `<!DOCTYPE graphml [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><graphml/>`,
		},
		{
			name: "xml extension",
			xml:  `<graphml><graph edgedefault="directed"><extension/></graph></graphml>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Import(context.Background(), "acme-payments", FormatGraphML, strings.NewReader(tt.xml), DefaultOptions())
			if !errors.Is(err, graphio.ErrMalformedInput) {
				t.Fatalf("Import() error = %v, want ErrMalformedInput", err)
			}
		})
	}
}

func TestImportBoundsInputAndHonorsCancellation(t *testing.T) {
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	var encoded bytes.Buffer
	if _, err := Export(context.Background(), FormatNDJSON, &encoded, fixture.Records); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	options := DefaultOptions()
	options.MaxInputBytes = 8
	_, _, err = Import(context.Background(), fixture.Partner, FormatNDJSON, strings.NewReader(encoded.String()), options)
	if !errors.Is(err, ErrInputTooLarge) || !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("oversized Import() error = %v, want ErrInputTooLarge and ErrMalformedInput", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = Import(ctx, fixture.Partner, FormatNDJSON, strings.NewReader(encoded.String()), DefaultOptions())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Import() error = %v, want context.Canceled", err)
	}
	var output bytes.Buffer
	if _, err := Export(ctx, FormatNDJSON, &output, fixture.Records); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Export() error = %v, want context.Canceled", err)
	}

	options = DefaultOptions()
	options.MaxRecords = 2
	_, _, err = Import(context.Background(), fixture.Partner, FormatNDJSON, strings.NewReader(encoded.String()), options)
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("record-limit Import() error = %v, want ErrMalformedInput", err)
	}
}

func TestImportRejectsOverflowingInputLimit(t *testing.T) {
	options := DefaultOptions()
	options.MaxInputBytes = int64(^uint64(0) >> 1)
	_, _, err := Import(context.Background(), PartnerName, FormatNDJSON, strings.NewReader(""), options)
	if !errors.Is(err, graphio.ErrInvalidOptions) {
		t.Fatalf("overflowing input limit error = %v, want graphio.ErrInvalidOptions", err)
	}
}

func TestInvalidFormatErrorDoesNotEchoUntrustedValue(t *testing.T) {
	const untrusted = "secret-token-format"
	_, _, err := Import(context.Background(), PartnerName, Format(untrusted), strings.NewReader(""), DefaultOptions())
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("invalid format error = %v, want ErrInvalidFormat", err)
	}
	if strings.Contains(err.Error(), untrusted) {
		t.Fatalf("invalid format error echoed untrusted value: %v", err)
	}

	_, err = Export(context.Background(), Format(untrusted), &bytes.Buffer{}, nil)
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("invalid export format error = %v, want ErrInvalidFormat", err)
	}
	if strings.Contains(err.Error(), untrusted) {
		t.Fatalf("invalid export format error echoed untrusted value: %v", err)
	}
}

func TestExportRejectsNonFiniteAndOutOfRangeScalarProperties(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
	}{
		{name: "nan", value: math.NaN()},
		{name: "positive infinity", value: math.Inf(1)},
		{name: "uint64 overflow", value: uint64(math.MaxInt64) + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			vertex, err := graph.ParseVertex("account-a", labelAccount, graph.Properties{
				"partner": "acme-payments",
				"value":   test.value,
			})
			if err != nil {
				t.Fatalf("ParseVertex() error = %v", err)
			}
			record, err := graphio.VertexRecord(vertex)
			if err != nil {
				t.Fatalf("VertexRecord() error = %v", err)
			}
			_, err = Export(context.Background(), FormatNDJSON, &bytes.Buffer{}, []graphio.Record{record})
			if !errors.Is(err, ErrNonScalarProperty) {
				t.Fatalf("Export() error = %v, want ErrNonScalarProperty", err)
			}
		})
	}
}

func TestRunDemoProducesEquivalentInspectableSnapshots(t *testing.T) {
	demo, err := RunDemo(context.Background())
	if err != nil {
		t.Fatalf("RunDemo() error = %v", err)
	}
	if !demo.Equivalent || len(demo.Runs) != 2 {
		t.Fatalf("demo = %+v, want two equivalent runs", demo)
	}
	encoded, err := EncodeDemo(demo)
	if err != nil {
		t.Fatalf("EncodeDemo() error = %v", err)
	}
	for _, marker := range []string{`"partner": "acme-payments"`, `"format": "ndjson"`, `"format": "graphml"`, `"equivalent": true`, `"directed": true`} {
		if !strings.Contains(string(encoded), marker) {
			t.Errorf("encoded demo missing %q:\n%s", marker, encoded)
		}
	}
}

func assertCanonicalSnapshot(t *testing.T, snapshot Snapshot) {
	t.Helper()
	if got, want := len(snapshot.Vertices), 4; got != want {
		t.Fatalf("snapshot vertices = %d, want %d", got, want)
	}
	if got, want := len(snapshot.Edges), 3; got != want {
		t.Fatalf("snapshot edges = %d, want %d", got, want)
	}
	if snapshot.Vertices[0].ID != "account-a" || snapshot.Vertices[1].ID != "account-b" || snapshot.Vertices[2].ID != "device-01" || snapshot.Vertices[3].ID != "device-02" {
		t.Fatalf("vertex order = %#v", snapshot.Vertices)
	}
	if snapshot.Edges[0].ID != "use-001" || snapshot.Edges[0].From != "account-a" || snapshot.Edges[0].To != "device-01" {
		t.Fatalf("edge direction/order = %#v", snapshot.Edges)
	}
	if got, want := snapshot.Vertices[0].Properties["risk_score"], int64(72); got != want {
		t.Fatalf("risk_score = %#v (%T), want %v (%T)", got, got, want, want)
	}
	if got, want := snapshot.Vertices[0].Properties["active"], true; got != want {
		t.Fatalf("active = %#v, want %v", got, want)
	}
	if got, want := snapshot.Edges[0].Properties["confidence"], float64(0.98); got != want {
		t.Fatalf("confidence = %#v (%T), want %v (%T)", got, got, want, want)
	}
}

func reverseRecords(records []graphio.Record) []graphio.Record {
	reversed := append([]graphio.Record(nil), records...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}
