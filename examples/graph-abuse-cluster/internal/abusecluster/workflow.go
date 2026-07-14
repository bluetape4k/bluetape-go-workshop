package abusecluster

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bluetape4k/bluetape-go/graph"
)

// WorkflowBackend provides the two persistence operations required by Execute.
type WorkflowBackend interface {
	ReplaceFixture(context.Context, Fixture) error
	LoadFixture(context.Context, string) ([]graph.Vertex, []graph.Edge, error)
}

// Execute replaces, reloads, and analyzes one validated fixture.
func Execute(ctx context.Context, backend WorkflowBackend, fixture Fixture) (Report, error) {
	if isNilWorkflowBackend(backend) {
		return Report{}, ErrBackend
	}
	if err := workflowContextError(ctx); err != nil {
		return Report{}, err
	}
	if err := ValidateFixture(fixture); err != nil {
		return Report{}, err
	}
	if err := backend.ReplaceFixture(ctx, fixture); err != nil {
		return Report{}, err
	}
	if err := workflowContextError(ctx); err != nil {
		return Report{}, err
	}

	vertices, edges, err := backend.LoadFixture(ctx, fixture.ID)
	if err != nil {
		return Report{}, err
	}
	if err := workflowContextError(ctx); err != nil {
		return Report{}, err
	}
	return Analyze(vertices, edges)
}

func isNilWorkflowBackend(backend WorkflowBackend) bool {
	if backend == nil {
		return true
	}
	value := reflect.ValueOf(backend)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func workflowContextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", ErrConfiguration)
	}
	return ctx.Err()
}
