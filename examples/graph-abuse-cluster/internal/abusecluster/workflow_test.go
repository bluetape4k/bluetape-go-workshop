package abusecluster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestExecuteRejectsNilBackend(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)

	for _, test := range []struct {
		name    string
		backend WorkflowBackend
	}{
		{name: "nil interface"},
		{name: "typed nil", backend: (*workflowBackendFake)(nil)},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Execute(context.Background(), test.backend, fixture)
			if !reflect.DeepEqual(got, Report{}) {
				t.Fatalf("Execute() report = %#v, want zero Report", got)
			}
			if !errors.Is(err, ErrBackend) {
				t.Fatalf("Execute() error = %v, want ErrBackend", err)
			}
		})
	}
}

func TestExecuteValidatesFixtureBeforeBackendCalls(t *testing.T) {
	backend := &workflowBackendFake{}

	got, err := Execute(context.Background(), backend, Fixture{})
	if !reflect.DeepEqual(got, Report{}) {
		t.Fatalf("Execute() report = %#v, want zero Report", got)
	}
	if !errors.Is(err, ErrInvalidFixture) {
		t.Fatalf("Execute() error = %v, want ErrInvalidFixture", err)
	}
	if len(backend.events) != 0 {
		t.Fatalf("backend events = %v, want none", backend.events)
	}
}

func TestExecuteReturnsPreCanceledContextWithoutBackendCalls(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()

	for _, test := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "canceled", ctx: canceled, want: context.Canceled},
		{name: "deadline", ctx: expired, want: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &workflowBackendFake{}
			got, err := Execute(test.ctx, backend, fixture)
			if !reflect.DeepEqual(got, Report{}) {
				t.Fatalf("Execute() report = %#v, want zero Report", got)
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if len(backend.events) != 0 {
				t.Fatalf("backend events = %v, want none", backend.events)
			}
		})
	}
}

func TestExecuteRejectsNilContextBeforeBackendCalls(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)
	backend := &workflowBackendFake{}

	got, err := Execute(nil, backend, fixture)
	if !reflect.DeepEqual(got, Report{}) {
		t.Fatalf("Execute(nil context) report = %#v, want zero Report", got)
	}
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("Execute(nil context) error = %v, want ErrConfiguration", err)
	}
	if got, want := err.Error(), ErrConfiguration.Error()+": nil context"; got != want {
		t.Fatalf("Execute(nil context) error = %q, want stable %q", got, want)
	}
	if len(backend.events) != 0 {
		t.Fatalf("backend events = %v, want none", backend.events)
	}
}

func TestExecuteReplacesLoadsAndAnalyzesInOrder(t *testing.T) {
	fixture := mustWorkflowFixture(t, "exact-fixture-id")
	want, err := Analyze(fixture.Vertices, fixture.Edges)
	if err != nil {
		t.Fatalf("Analyze(fixture) error = %v", err)
	}
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("caller"), "unchanged")
	backend := &workflowBackendFake{
		wantContext: ctx,
		vertices:    fixture.Vertices,
		edges:       fixture.Edges,
	}

	got, err := Execute(ctx, backend, fixture)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Execute() report = %#v, want %#v", got, want)
	}
	if got := backend.events; !reflect.DeepEqual(got, []string{"replace", "load"}) {
		t.Fatalf("backend events = %v, want replace then load before returned analysis", got)
	}
	if got := backend.loadedFixtureID; got != fixture.ID {
		t.Fatalf("LoadFixture() id = %q, want exact fixture ID %q", got, fixture.ID)
	}
	if got := backend.replacedFixtureID; got != fixture.ID {
		t.Fatalf("ReplaceFixture() fixture ID = %q, want %q", got, fixture.ID)
	}
}

func TestExecuteChecksCancellationAfterReplaceBeforeLoad(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)
	ctx, cancel := context.WithCancel(context.Background())
	backend := &workflowBackendFake{
		replace: func(context.Context, Fixture) error {
			cancel()
			return nil
		},
	}

	got, err := Execute(ctx, backend, fixture)
	if !reflect.DeepEqual(got, Report{}) {
		t.Fatalf("Execute() report = %#v, want zero Report", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	if got := backend.events; !reflect.DeepEqual(got, []string{"replace"}) {
		t.Fatalf("backend events = %v, want no late load", got)
	}
}

func TestExecutePreservesReplaceFailureWithoutLoading(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)

	for _, cause := range []error{
		fmt.Errorf("safe backend category: %w", ErrBackend),
		fmt.Errorf("safe cancellation category: %w", context.Canceled),
	} {
		backend := &workflowBackendFake{replaceErr: cause}
		got, err := Execute(context.Background(), backend, fixture)
		if !reflect.DeepEqual(got, Report{}) {
			t.Fatalf("Execute() report = %#v, want zero Report", got)
		}
		if !errors.Is(err, cause) {
			t.Fatalf("Execute() error = %v, want preserved cause %v", err, cause)
		}
		if got := backend.events; !reflect.DeepEqual(got, []string{"replace"}) {
			t.Fatalf("backend events = %v, want one replace and no load", got)
		}
	}
}

func TestExecuteReturnsZeroReportOnLoadFailure(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)
	cause := fmt.Errorf("safe load category: %w", ErrBackend)
	backend := &workflowBackendFake{loadErr: cause}

	got, err := Execute(context.Background(), backend, fixture)
	if !reflect.DeepEqual(got, Report{}) {
		t.Fatalf("Execute() report = %#v, want zero Report", got)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("Execute() error = %v, want preserved cause %v", err, cause)
	}
	if got := backend.events; !reflect.DeepEqual(got, []string{"replace", "load"}) {
		t.Fatalf("backend events = %v, want one replace then one load", got)
	}
}

func TestExecutePropagatesMalformedLoadedGraph(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)
	backend := &workflowBackendFake{vertices: []graph.Vertex{{}}}

	got, err := Execute(context.Background(), backend, fixture)
	if !reflect.DeepEqual(got, Report{}) {
		t.Fatalf("Execute() report = %#v, want zero Report", got)
	}
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("Execute() error = %v, want ErrInvalidGraph", err)
	}
	if !errors.Is(err, graph.ErrInvalidVertex) {
		t.Fatalf("Execute() error = %v, want graph.ErrInvalidVertex cause", err)
	}
}

func TestExecuteDoesNotAnalyzeAfterLoadCancelsCaller(t *testing.T) {
	fixture := mustWorkflowFixture(t, FixtureID)
	ctx, cancel := context.WithCancel(context.Background())
	backend := &workflowBackendFake{
		load: func(context.Context, string) ([]graph.Vertex, []graph.Edge, error) {
			cancel()
			return fixture.Vertices, fixture.Edges, nil
		},
	}

	got, err := Execute(ctx, backend, fixture)
	if !reflect.DeepEqual(got, Report{}) {
		t.Fatalf("Execute() report = %#v, want zero Report", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
}

type workflowBackendFake struct {
	wantContext       context.Context
	events            []string
	replacedFixtureID string
	loadedFixtureID   string
	vertices          []graph.Vertex
	edges             []graph.Edge
	replaceErr        error
	loadErr           error
	replace           func(context.Context, Fixture) error
	load              func(context.Context, string) ([]graph.Vertex, []graph.Edge, error)
}

func (f *workflowBackendFake) ReplaceFixture(ctx context.Context, fixture Fixture) error {
	f.events = append(f.events, "replace")
	f.replacedFixtureID = fixture.ID
	if f.wantContext != nil && ctx != f.wantContext {
		return errors.New("replace received different context")
	}
	if f.replace != nil {
		return f.replace(ctx, fixture)
	}
	return f.replaceErr
}

func (f *workflowBackendFake) LoadFixture(ctx context.Context, fixtureID string) ([]graph.Vertex, []graph.Edge, error) {
	f.events = append(f.events, "load")
	f.loadedFixtureID = fixtureID
	if f.wantContext != nil && ctx != f.wantContext {
		return nil, nil, errors.New("load received different context")
	}
	if f.load != nil {
		return f.load(ctx, fixtureID)
	}
	return f.vertices, f.edges, f.loadErr
}

func mustWorkflowFixture(t *testing.T, fixtureID string) Fixture {
	t.Helper()
	fixture, err := NewFixture(fixtureID)
	if err != nil {
		t.Fatalf("NewFixture(%q) error = %v", fixtureID, err)
	}
	return fixture
}
