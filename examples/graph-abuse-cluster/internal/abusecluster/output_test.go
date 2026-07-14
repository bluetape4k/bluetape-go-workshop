package abusecluster

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncodeReportMatchesApprovedIndentedJSON(t *testing.T) {
	report := mustDefaultWorkflowReport(t)
	want := []byte(`{
  "clusters": [
    {
      "cluster_id": "cluster:usr-001",
      "users": [
        "usr-001",
        "usr-002",
        "usr-003"
      ],
      "evidence": [
        {
          "kind": "device",
          "opaque_id": "dev-001",
          "user_count": 2,
          "weight": 3
        },
        {
          "kind": "ip",
          "opaque_id": "ip-001",
          "user_count": 2,
          "weight": 1
        }
      ],
      "risk_score": 4
    },
    {
      "cluster_id": "cluster:usr-004",
      "users": [
        "usr-004",
        "usr-005"
      ],
      "evidence": [
        {
          "kind": "device",
          "opaque_id": "dev-002",
          "user_count": 2,
          "weight": 3
        },
        {
          "kind": "ip",
          "opaque_id": "ip-002",
          "user_count": 2,
          "weight": 1
        }
      ],
      "risk_score": 4
    }
  ],
  "isolated_users": [
    "usr-006"
  ]
}
`)

	got, err := EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeReport() =\n%s\nwant\n%s", got, want)
	}
	if !bytes.HasSuffix(got, []byte("\n")) || bytes.HasSuffix(got, []byte("\n\n")) {
		t.Fatalf("EncodeReport() trailing bytes = %q, want exactly one newline", got[len(got)-2:])
	}

	again, err := EncodeReport(report)
	if err != nil {
		t.Fatalf("second EncodeReport() error = %v", err)
	}
	if !bytes.Equal(again, got) {
		t.Fatalf("repeated EncodeReport() differs:\nfirst=%q\nsecond=%q", got, again)
	}
}

func TestEncodeReportReturnsNoBytesOnMarshalFailure(t *testing.T) {
	report := mustDefaultWorkflowReport(t)
	cause := errors.New("injected marshal failure")
	marshal := func(any, string, string) ([]byte, error) {
		return []byte("partial secret JSON"), cause
	}

	got, err := encodeReport(report, marshal)
	if got != nil {
		t.Fatalf("encodeReport() bytes = %q, want nil", got)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("encodeReport() error = %v, want injected cause", err)
	}
}

func TestEncodeReportRejectsNilMarshaler(t *testing.T) {
	report := mustDefaultWorkflowReport(t)

	got, err := encodeReport(report, nil)
	if got != nil {
		t.Fatalf("encodeReport(nil marshaler) bytes = %q, want nil", got)
	}
	if !errors.Is(err, errNilMarshalIndent) {
		t.Fatalf("encodeReport(nil marshaler) error = %v, want fixed errNilMarshalIndent", err)
	}
}

func mustDefaultWorkflowReport(t *testing.T) Report {
	t.Helper()
	fixture, err := DefaultFixture()
	if err != nil {
		t.Fatalf("DefaultFixture() error = %v", err)
	}
	report, err := Analyze(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("Analyze(default fixture) error = %v", err)
	}
	return report
}
