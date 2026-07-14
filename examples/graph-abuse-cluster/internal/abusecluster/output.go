package abusecluster

import (
	"encoding/json"
	"errors"
)

var errNilMarshalIndent = errors.New("graph abuse cluster: JSON marshaler unavailable")

type marshalIndentFunc func(any, string, string) ([]byte, error)

// EncodeReport returns the complete indented JSON report with one trailing newline.
func EncodeReport(report Report) ([]byte, error) {
	return encodeReport(report, json.MarshalIndent)
}

func encodeReport(report Report, marshal marshalIndentFunc) ([]byte, error) {
	if marshal == nil {
		return nil, errNilMarshalIndent
	}
	encoded, err := marshal(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
