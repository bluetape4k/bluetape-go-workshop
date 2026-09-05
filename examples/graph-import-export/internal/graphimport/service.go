package graphimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
	"github.com/bluetape4k/bluetape-go/graph/graphio/graphml"
)

// Import 는 bounded reader를 format별 graphio parser에 전달하고 domain snapshot을 만든다.
func Import(ctx context.Context, partner string, format Format, reader io.Reader, options Options) (Result, graphio.Report, error) {
	result := Result{Partner: strings.TrimSpace(partner), Format: format}
	report := emptyReport(format)
	if ctx == nil {
		return result, report, ErrInvalidContext
	}
	if result.Partner == "" {
		return result, report, fmt.Errorf("%w: partner is blank", ErrInvalidPartner)
	}
	if !isSupportedFormat(format) {
		return result, report, ErrInvalidFormat
	}
	normalizedOptions, err := normalizeOptions(options)
	if err != nil {
		return result, report, err
	}
	if reader == nil {
		return result, report, fmt.Errorf("%w: reader must not be nil", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return result, report, err
	}

	data, err := readBounded(ctx, reader, normalizedOptions.MaxInputBytes)
	if err != nil {
		return result, report, err
	}
	if err := ctx.Err(); err != nil {
		return result, report, err
	}

	records, report, err := readRecords(ctx, format, data, normalizedOptions)
	if err != nil {
		return result, report, err
	}
	if err := ctx.Err(); err != nil {
		return result, report, err
	}
	normalizedRecords, summary, snapshot, err := normalizeDomain(result.Partner, records, format == FormatNDJSON)
	if err != nil {
		return result, report, err
	}
	if err := ctx.Err(); err != nil {
		return result, report, err
	}
	result.Summary = summary
	result.Snapshot = snapshot
	result.records = normalizedRecords
	return result, report, nil
}

// Export 는 record를 canonical ID 순서로 정렬한 뒤 선택한 format으로 기록한다.
func Export(ctx context.Context, format Format, writer io.Writer, records []graphio.Record) (graphio.Report, error) {
	report := emptyReport(format)
	if ctx == nil {
		return report, ErrInvalidContext
	}
	if !isSupportedFormat(format) {
		return report, ErrInvalidFormat
	}
	if writer == nil {
		return report, fmt.Errorf("%w: writer must not be nil", ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	canonical, err := canonicalRecords(records, false)
	if err != nil {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}

	switch format {
	case FormatNDJSON:
		return graphio.WriteNDJSON(ctx, writer, canonical, graphio.WriteOptions{})
	case FormatGraphML:
		return graphml.Write(ctx, writer, canonical, graphml.WriteOptions{})
	default:
		return report, ErrInvalidFormat
	}
}

// RunDemo 는 하나의 named partner fixture를 두 interchange로 비교한다.
func RunDemo(ctx context.Context) (DemoResult, error) {
	if ctx == nil {
		return DemoResult{}, ErrInvalidContext
	}
	fixture, err := DefaultFixture()
	if err != nil {
		return DemoResult{}, err
	}
	runs := make([]Result, 0, 2)
	for _, format := range []Format{FormatNDJSON, FormatGraphML} {
		if err := ctx.Err(); err != nil {
			return DemoResult{}, err
		}
		var encoded bytes.Buffer
		if _, err := Export(ctx, format, &encoded, fixture.Records); err != nil {
			return DemoResult{}, err
		}
		result, _, err := Import(ctx, fixture.Partner, format, bytes.NewReader(encoded.Bytes()), DefaultOptions())
		if err != nil {
			return DemoResult{}, err
		}
		runs = append(runs, result)
	}
	if len(runs) != 2 || !snapshotsEqual(runs[0].Snapshot, runs[1].Snapshot) {
		return DemoResult{}, ErrSnapshotMismatch
	}
	return DemoResult{Partner: fixture.Partner, Equivalent: true, Runs: runs}, nil
}

// EncodeDemo 는 runtime duration을 포함하지 않는 stable JSON report를 만든다.
func EncodeDemo(demo DemoResult) ([]byte, error) {
	if demo.Runs == nil {
		demo.Runs = []Result{}
	}
	encoded, err := json.MarshalIndent(demo, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func normalizeOptions(options Options) (Options, error) {
	defaults := DefaultOptions()
	if options.MaxInputBytes == 0 {
		options.MaxInputBytes = defaults.MaxInputBytes
	}
	if options.MaxRecords == 0 {
		options.MaxRecords = defaults.MaxRecords
	}
	if options.MaxLineBytes == 0 {
		options.MaxLineBytes = defaults.MaxLineBytes
	}
	if options.MaxRecordBytes == 0 {
		options.MaxRecordBytes = defaults.MaxRecordBytes
	}
	if options.MaxInputBytes < 0 || options.MaxLineBytes < 0 || options.MaxRecordBytes < 0 {
		return options, fmt.Errorf("%w: limits must not be negative", graphio.ErrInvalidOptions)
	}
	if options.MaxInputBytes > math.MaxInt64-1 {
		return options, fmt.Errorf("%w: max input bytes is too large", graphio.ErrInvalidOptions)
	}
	if options.MaxRecords < 0 && options.MaxRecords != graphio.UnlimitedRecords {
		return options, fmt.Errorf("%w: max records must be positive or UnlimitedRecords", graphio.ErrInvalidOptions)
	}
	return options, nil
}

func readRecords(ctx context.Context, format Format, data []byte, options Options) ([]graphio.Record, graphio.Report, error) {
	readOptions := graphio.ReadOptions{
		DuplicateVertexPolicy: graphio.DuplicateVertexFail,
		MissingEndpointPolicy: graphio.MissingEndpointFail,
		MaxLineBytes:          options.MaxLineBytes,
		MaxRecordBytes:        options.MaxRecordBytes,
		MaxRecords:            options.MaxRecords,
	}
	switch format {
	case FormatNDJSON:
		return graphio.ReadNDJSON(ctx, bytes.NewReader(data), readOptions)
	case FormatGraphML:
		return graphml.Read(ctx, bytes.NewReader(data), graphml.ReadOptions{
			ReadOptions:   readOptions,
			MaxInputBytes: options.MaxInputBytes,
		})
	default:
		return nil, emptyReport(format), ErrInvalidFormat
	}
}

func readBounded(ctx context.Context, reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(contextReader{ctx: ctx, reader: reader}, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: %w", ErrInputTooLarge, graphio.ErrMalformedInput)
	}
	return data, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func canonicalRecords(records []graphio.Record, coerceJSONNumbers bool) ([]graphio.Record, error) {
	vertices := make([]graphio.Record, 0, len(records))
	edges := make([]graphio.Record, 0, len(records))
	seenIDs := make(map[string]struct{}, len(records))
	vertexIDs := make(map[string]struct{})
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidGraph, err)
		}
		id := recordID(record)
		if _, exists := seenIDs[id]; exists {
			return nil, ErrDuplicateRecord
		}
		seenIDs[id] = struct{}{}
		normalized, err := normalizeRecord(record, coerceJSONNumbers)
		if err != nil {
			return nil, err
		}
		if normalized.Kind == graphio.RecordVertex {
			vertexIDs[id] = struct{}{}
			vertices = append(vertices, normalized)
		} else {
			edges = append(edges, normalized)
		}
	}
	for _, record := range edges {
		if _, ok := vertexIDs[record.Edge.StartID().String()]; !ok {
			return nil, graphio.ErrMissingEndpoint
		}
		if _, ok := vertexIDs[record.Edge.EndID().String()]; !ok {
			return nil, graphio.ErrMissingEndpoint
		}
	}
	sort.Slice(vertices, func(i, j int) bool {
		return vertices[i].Vertex.ID().String() < vertices[j].Vertex.ID().String()
	})
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].Edge.ID().String() < edges[j].Edge.ID().String()
	})
	return append(vertices, edges...), nil
}

func normalizeRecord(record graphio.Record, coerceJSONNumbers bool) (graphio.Record, error) {
	if record.Kind == graphio.RecordVertex {
		properties, err := normalizeProperties(record.Vertex.Properties(), coerceJSONNumbers)
		if err != nil {
			return graphio.Record{}, err
		}
		vertex, err := graph.ParseVertex(record.Vertex.ID().String(), record.Vertex.Label().String(), properties)
		if err != nil {
			return graphio.Record{}, fmt.Errorf("%w: %w", ErrInvalidGraph, err)
		}
		return graphio.VertexRecord(vertex)
	}
	properties, err := normalizeProperties(record.Edge.Properties(), coerceJSONNumbers)
	if err != nil {
		return graphio.Record{}, err
	}
	edge, err := graph.ParseEdge(record.Edge.ID().String(), record.Edge.Label().String(), graph.RawEdgeEndpoints{
		Start: record.Edge.StartID().String(),
		End:   record.Edge.EndID().String(),
	}, properties)
	if err != nil {
		return graphio.Record{}, fmt.Errorf("%w: %w", ErrInvalidGraph, err)
	}
	return graphio.EdgeRecord(edge)
}

func normalizeProperties(properties graph.Properties, coerceJSONNumbers bool) (graph.Properties, error) {
	if len(properties) == 0 {
		return nil, nil
	}
	normalized := make(graph.Properties, len(properties))
	for key, value := range properties {
		if strings.TrimSpace(key) == "" {
			return nil, ErrInvalidGraph
		}
		scalar, err := normalizeScalar(value, coerceJSONNumbers)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNonScalarProperty, err)
		}
		normalized[key] = scalar
	}
	return normalized, nil
}

func normalizeScalar(value any, coerceJSONNumbers bool) (any, error) {
	switch typed := value.(type) {
	case string, bool:
		return typed, nil
	case int:
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		return uint64ToInt64(uint64(typed))
	case uint8:
		return int64(typed), nil
	case uint16:
		return int64(typed), nil
	case uint32:
		return int64(typed), nil
	case uint64:
		return uint64ToInt64(typed)
	case float32:
		return normalizeFloat(float64(typed), coerceJSONNumbers)
	case float64:
		return normalizeFloat(typed, coerceJSONNumbers)
	default:
		return nil, errors.New("value is not a supported scalar")
	}
}

func normalizeFloat(value float64, coerceJSONNumbers bool) (any, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, errors.New("non-finite number")
	}
	const maxSafeJSONInteger = float64(1 << 53)
	if coerceJSONNumbers && math.Trunc(value) == value && value >= -maxSafeJSONInteger && value <= maxSafeJSONInteger {
		return int64(value), nil
	}
	return value, nil
}

func uint64ToInt64(value uint64) (any, error) {
	if value > math.MaxInt64 {
		return nil, errors.New("unsigned integer exceeds int64")
	}
	return int64(value), nil
}

func normalizeDomain(partner string, records []graphio.Record, coerceJSONNumbers bool) ([]graphio.Record, Summary, Snapshot, error) {
	canonical, err := canonicalRecords(records, coerceJSONNumbers)
	if err != nil {
		return nil, Summary{}, Snapshot{}, err
	}
	vertices := make(map[string]graph.Vertex)
	summary := Summary{
		Directed:     true,
		VertexLabels: map[string]int{},
		EdgeLabels:   map[string]int{},
	}
	for _, record := range canonical {
		if record.Kind != graphio.RecordVertex {
			continue
		}
		vertex := record.Vertex
		label := vertex.Label().String()
		if label != labelAccount && label != labelDevice {
			return nil, Summary{}, Snapshot{}, ErrUnsupportedRecord
		}
		if err := validatePartner(vertex.Properties(), partner); err != nil {
			return nil, Summary{}, Snapshot{}, err
		}
		id := vertex.ID().String()
		vertices[id] = vertex
		summary.Vertices++
		summary.VertexLabels[label]++
		switch label {
		case labelAccount:
			summary.Accounts++
		case labelDevice:
			summary.Devices++
		}
	}

	snapshot := Snapshot{
		Vertices: make([]SnapshotVertex, 0, summary.Vertices),
		Edges:    make([]SnapshotEdge, 0),
	}
	for _, record := range canonical {
		if record.Kind == graphio.RecordVertex {
			vertex := record.Vertex
			snapshot.Vertices = append(snapshot.Vertices, SnapshotVertex{
				ID:         vertex.ID().String(),
				Label:      vertex.Label().String(),
				Properties: cloneProperties(vertex.Properties()),
			})
			continue
		}
		edge := record.Edge
		if edge.Label().String() != labelUsesDevice {
			return nil, Summary{}, Snapshot{}, ErrUnsupportedRecord
		}
		from, fromOK := vertices[edge.StartID().String()]
		to, toOK := vertices[edge.EndID().String()]
		if !fromOK || !toOK {
			return nil, Summary{}, Snapshot{}, graphio.ErrMissingEndpoint
		}
		if from.Label().String() != labelAccount || to.Label().String() != labelDevice {
			return nil, Summary{}, Snapshot{}, ErrInvalidGraph
		}
		summary.Edges++
		summary.EdgeLabels[edge.Label().String()]++
		summary.UsesDeviceEdges++
		snapshot.Edges = append(snapshot.Edges, SnapshotEdge{
			ID:         edge.ID().String(),
			Label:      edge.Label().String(),
			From:       edge.StartID().String(),
			To:         edge.EndID().String(),
			Properties: cloneProperties(edge.Properties()),
		})
	}
	return canonical, summary, snapshot, nil
}

func validatePartner(properties graph.Properties, partner string) error {
	value, ok := properties["partner"]
	if !ok {
		return fmt.Errorf("%w: partner property is required", ErrInvalidGraph)
	}
	if value != partner {
		return fmt.Errorf("%w: partner property mismatch", ErrInvalidGraph)
	}
	return nil
}

func cloneProperties(properties graph.Properties) map[string]any {
	if len(properties) == 0 {
		return nil
	}
	clone := make(map[string]any, len(properties))
	for key, value := range properties {
		clone[key] = value
	}
	return clone
}

func snapshotsEqual(left, right Snapshot) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func recordID(record graphio.Record) string {
	if record.Kind == graphio.RecordVertex {
		return record.Vertex.ID().String()
	}
	return record.Edge.ID().String()
}

func emptyReport(format Format) graphio.Report {
	switch format {
	case FormatNDJSON:
		return graphio.Report{Format: graphio.FormatNDJSON}
	case FormatGraphML:
		return graphio.Report{Format: graphml.FormatGraphML}
	default:
		return graphio.Report{}
	}
}

func isSupportedFormat(format Format) bool {
	return format == FormatNDJSON || format == FormatGraphML
}
