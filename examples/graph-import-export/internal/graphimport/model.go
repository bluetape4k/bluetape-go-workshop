package graphimport

import (
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

const (
	// PartnerName 은 README와 CLI가 공유하는 named partner다.
	PartnerName = "acme-payments"
	// FixtureID 는 deterministic fixture의 namespace다.
	FixtureID = "risk-partner-acme-v1"
	// DefaultMaxInputBytes 는 untrusted import의 전체 입력 기본 상한이다.
	DefaultMaxInputBytes int64 = 64 << 10
	// DefaultMaxRecords 는 하나의 import에서 허용하는 vertex와 edge 합계 기본 상한이다.
	DefaultMaxRecords int64 = 64
	// DefaultMaxLineBytes 는 NDJSON logical line의 기본 상한이다.
	DefaultMaxLineBytes = 16 << 10
	// DefaultMaxRecordBytes 는 NDJSON record의 기본 상한이다.
	DefaultMaxRecordBytes = 16 << 10
)

const (
	labelAccount    = "Account"
	labelDevice     = "Device"
	labelUsesDevice = "USES_DEVICE"
)

// Format 은 graph interchange encoding을 식별한다.
type Format string

const (
	// FormatNDJSON 는 graphio의 line-oriented JSON encoding이다.
	FormatNDJSON Format = "ndjson"
	// FormatGraphML 는 graphio/graphml의 bounded directed subset이다.
	FormatGraphML Format = "graphml"
)

// Options 는 import 경계의 byte, line, record 상한을 정의한다.
type Options struct {
	MaxInputBytes  int64
	MaxRecords     int64
	MaxLineBytes   int
	MaxRecordBytes int
}

// DefaultOptions 는 untrusted partner input에 적용할 bounded 기본값을 반환한다.
func DefaultOptions() Options {
	return Options{
		MaxInputBytes:  DefaultMaxInputBytes,
		MaxRecords:     DefaultMaxRecords,
		MaxLineBytes:   DefaultMaxLineBytes,
		MaxRecordBytes: DefaultMaxRecordBytes,
	}
}

// Fixture 는 partner graph record와 named partner를 함께 보관한다.
type Fixture struct {
	Partner string
	Records []graphio.Record
}

// Summary 는 normalized graph의 inspectable 집계를 제공한다.
type Summary struct {
	Vertices        int            `json:"vertices"`
	Edges           int            `json:"edges"`
	Accounts        int            `json:"accounts"`
	Devices         int            `json:"devices"`
	UsesDeviceEdges int            `json:"uses_device_edges"`
	Directed        bool           `json:"directed"`
	VertexLabels    map[string]int `json:"vertex_labels"`
	EdgeLabels      map[string]int `json:"edge_labels"`
}

// Snapshot 은 ID 순서로 정렬된 portable graph view다.
type Snapshot struct {
	Vertices []SnapshotVertex `json:"vertices"`
	Edges    []SnapshotEdge   `json:"edges"`
}

// SnapshotVertex 는 JSON으로 확인할 수 있는 vertex projection이다.
type SnapshotVertex struct {
	ID         string         `json:"id"`
	Label      string         `json:"label"`
	Properties map[string]any `json:"properties,omitempty"`
}

// SnapshotEdge 는 방향과 scalar properties를 보존한 edge projection이다.
type SnapshotEdge struct {
	ID         string         `json:"id"`
	Label      string         `json:"label"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	Properties map[string]any `json:"properties,omitempty"`
}

// Result 는 하나의 format에서 import한 normalized graph와 집계 결과다.
type Result struct {
	Partner  string   `json:"partner"`
	Format   Format   `json:"format"`
	Summary  Summary  `json:"summary"`
	Snapshot Snapshot `json:"snapshot"`

	records []graphio.Record
}

// Records 는 후속 export에서 사용할 수 있도록 normalized record를 복사해 반환한다.
func (r Result) Records() []graphio.Record {
	return append([]graphio.Record(nil), r.records...)
}

// DemoResult 는 같은 fixture를 NDJSON와 GraphML로 읽은 비교 결과다.
type DemoResult struct {
	Partner    string   `json:"partner"`
	Equivalent bool     `json:"equivalent"`
	Runs       []Result `json:"runs"`
}
