package recommendation

import "errors"

// 예제에서 호출자가 분류할 수 있는 안정적인 오류 sentinel입니다.
var (
	ErrInvalidFixture = errors.New("graph recommendation: invalid fixture")
	ErrInvalidGraph   = errors.New("graph recommendation: invalid graph")
	ErrGraphTooLarge  = errors.New("graph recommendation: graph too large")
	ErrUnknownSeed    = errors.New("graph recommendation: unknown seed user")
	ErrInvalidLimit   = errors.New("graph recommendation: invalid limit")
	ErrInvalidContext = errors.New("graph recommendation: invalid context")
)
