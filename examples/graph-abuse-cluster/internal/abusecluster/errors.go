package abusecluster

import "errors"

// ErrInvalidFixture 와 관련 sentinel들은 안정적인 예제 실패를 분류한다.
var (
	ErrInvalidFixture = errors.New("graph abuse cluster: invalid fixture")
	ErrInvalidGraph   = errors.New("graph abuse cluster: invalid graph")
	ErrGraphTooLarge  = errors.New("graph abuse cluster: graph too large")
	ErrConfiguration  = errors.New("graph abuse cluster: invalid configuration")
	ErrBackend        = errors.New("graph abuse cluster: backend operation failed")
)
