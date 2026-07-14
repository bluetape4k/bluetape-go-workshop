package abusecluster

import "errors"

// ErrInvalidFixture and the related sentinels classify stable example failures.
var (
	ErrInvalidFixture = errors.New("graph abuse cluster: invalid fixture")
	ErrInvalidGraph   = errors.New("graph abuse cluster: invalid graph")
	ErrGraphTooLarge  = errors.New("graph abuse cluster: graph too large")
	ErrConfiguration  = errors.New("graph abuse cluster: invalid configuration")
	ErrBackend        = errors.New("graph abuse cluster: backend operation failed")
)
