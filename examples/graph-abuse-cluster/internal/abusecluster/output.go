package abusecluster

import (
	"encoding/json"
	"errors"
)

var errNilMarshalIndent = errors.New("graph abuse cluster: JSON marshaler unavailable")

type marshalIndentFunc func(any, string, string) ([]byte, error)

// EncodeReport 는 trailing newline 하나가 붙은 완전한 indented JSON report를 반환한다.
func EncodeReport(report Report) ([]byte, error) {
	return encodeReport(report, json.MarshalIndent)
}

func encodeReport(report Report, marshal marshalIndentFunc) ([]byte, error) {
	if marshal == nil {
		return nil, errNilMarshalIndent
	}
	normalized := report
	if normalized.Clusters == nil {
		normalized.Clusters = []Cluster{}
	}
	if normalized.IsolatedUsers == nil {
		normalized.IsolatedUsers = []string{}
	}

	encoded, err := marshal(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	output := make([]byte, len(encoded)+1)
	copy(output, encoded)
	output[len(encoded)] = '\n'
	return output, nil
}
