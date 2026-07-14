package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-abuse-cluster/internal/abusecluster"
)

func TestLoadConfigAcceptsStrictLoopbackBoltURIs(t *testing.T) {
	accepted := []string{
		"bolt://localhost:7687",
		"bolt://LOCALHOST:7687",
		"bolt://127.0.0.1:7687",
		"bolt://127.9.8.7:7687",
		"bolt://[::1]:7687",
	}
	for _, uri := range accepted {
		t.Run(uri, func(t *testing.T) {
			cfg, err := loadConfig(func(key string) string {
				if key != neo4jURIEnvironment {
					t.Fatalf("getenv key = %q", key)
				}
				return uri
			})
			if err != nil {
				t.Fatalf("loadConfig() error = %v", err)
			}
			if cfg.neo4jURI != uri {
				t.Fatalf("neo4jURI = %q, want %q", cfg.neo4jURI, uri)
			}
		})
	}
}

func TestLoadConfigRejectsUnsafeURIWithoutDisclosure(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{name: "missing"},
		{name: "routing scheme", uri: "neo4j://localhost:7687"},
		{name: "http scheme", uri: "http://localhost:7687"},
		{name: "dns host", uri: "bolt://neo4j.internal:7687"},
		{name: "remote ipv4", uri: "bolt://192.0.2.10:7687"},
		{name: "userinfo", uri: "bolt://secret:password@localhost:7687"},
		{name: "path", uri: "bolt://localhost:7687/db"},
		{name: "query", uri: "bolt://localhost:7687?token=secret"},
		{name: "fragment", uri: "bolt://localhost:7687#secret"},
		{name: "missing port", uri: "bolt://localhost"},
		{name: "non numeric port", uri: "bolt://localhost:abc"},
		{name: "zero port", uri: "bolt://localhost:0"},
		{name: "large port", uri: "bolt://localhost:65536"},
		{name: "leading whitespace", uri: " bolt://localhost:7687"},
		{name: "embedded whitespace", uri: "bolt://local host:7687"},
		{name: "trailing whitespace", uri: "bolt://localhost:7687\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(func(string) string { return tt.uri })
			if err == nil {
				t.Fatal("loadConfig() error = nil")
			}
			if !errors.Is(err, abusecluster.ErrConfiguration) {
				t.Fatalf("loadConfig() error = %v, want ErrConfiguration", err)
			}
			if tt.uri != "" && strings.Contains(err.Error(), tt.uri) {
				t.Fatalf("error disclosed URI %q: %v", tt.uri, err)
			}
		})
	}
}

func TestLoadConfigRejectsNilGetenv(t *testing.T) {
	_, err := loadConfig(nil)
	if !errors.Is(err, abusecluster.ErrConfiguration) {
		t.Fatalf("loadConfig(nil) error = %v, want ErrConfiguration", err)
	}
}

func TestRunOrdersLifecycleAndUsesBoundedContexts(t *testing.T) {
	events := []string{}
	app := &fakeApplication{events: &events}
	open := func(appConfig) (application, error) {
		events = append(events, "open")
		return app, nil
	}
	encode := func(abusecluster.Report) ([]byte, error) {
		events = append(events, "encode")
		return []byte("{\"ok\":true}\n"), nil
	}
	stdout := &eventWriter{events: &events}

	if err := run(context.Background(), appConfig{neo4jURI: "bolt://localhost:7687"}, stdout, open, encode); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := "open,verify,replace,load,analyze,encode,close,stdout"
	if got := strings.Join(events, ","); got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	if got := stdout.String(); got != "{\"ok\":true}\n" {
		t.Fatalf("stdout = %q", got)
	}
	assertDeadlineNear(t, app.operationDeadline, operationTimeout)
	assertDeadlineNear(t, app.closeDeadline, cleanupTimeout)
}

func TestRunClassifiesFailuresAndAlwaysClosesAfterOpen(t *testing.T) {
	secret := "bolt://secret:password@localhost:7687"
	opaque := "usr-private-123"
	cause := errors.New(secret + " " + opaque)
	tests := []struct {
		name       string
		configure  func(*fakeApplication)
		openErr    error
		encodeErr  error
		writer     io.Writer
		wantStage  string
		wantClass  string
		wantClosed bool
	}{
		{name: "open", openErr: cause, wantStage: "open", wantClass: "unavailable"},
		{name: "verify", configure: func(a *fakeApplication) { a.verifyErr = cause }, wantStage: "verify", wantClass: "unavailable", wantClosed: true},
		{name: "workflow", configure: func(a *fakeApplication) { a.executeErr = cause }, wantStage: "workflow", wantClass: "failed", wantClosed: true},
		{name: "encode", encodeErr: cause, wantStage: "encode", wantClass: "failed", wantClosed: true},
		{name: "close", configure: func(a *fakeApplication) { a.closeErr = cause }, wantStage: "close", wantClass: "failed", wantClosed: true},
		{name: "write", writer: errorWriter{err: cause}, wantStage: "write", wantClass: "failed", wantClosed: true},
		{name: "short write", writer: shortWriter{}, wantStage: "write", wantClass: "failed", wantClosed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			app := &fakeApplication{events: &events}
			stdout := tt.writer
			buffer := &bytes.Buffer{}
			if stdout == nil {
				stdout = buffer
			}
			if tt.configure != nil {
				tt.configure(app)
			}
			open := func(appConfig) (application, error) {
				events = append(events, "open")
				if tt.openErr != nil {
					return nil, tt.openErr
				}
				return app, nil
			}
			encode := func(abusecluster.Report) ([]byte, error) {
				events = append(events, "encode")
				if tt.encodeErr != nil {
					return nil, tt.encodeErr
				}
				return []byte("report\n"), nil
			}
			err := run(context.Background(), appConfig{}, stdout, open, encode)
			assertRunFailure(t, err, tt.wantStage, tt.wantClass, secret, opaque)
			if buffer.Len() != 0 {
				t.Fatalf("stdout on %s failure = %q, want empty", tt.name, buffer.String())
			}
			if got := app.closeCalls == 1; got != tt.wantClosed {
				t.Fatalf("close exactly once = %v, want %v (calls=%d)", got, tt.wantClosed, app.closeCalls)
			}
		})
	}
}

func TestRunUsesFreshCleanupContextAfterCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := []string{}
	app := &fakeApplication{events: &events, executeHook: cancel, executeErr: context.Canceled}
	err := run(ctx, appConfig{}, io.Discard, func(appConfig) (application, error) {
		events = append(events, "open")
		return app, nil
	}, abusecluster.EncodeReport)
	assertRunFailure(t, err, "workflow", "canceled", "", "")
	if app.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", app.closeCalls)
	}
	if app.closeContextErr != nil {
		t.Fatalf("fresh close context error = %v", app.closeContextErr)
	}
	assertDeadlineNear(t, app.closeDeadline, cleanupTimeout)
}

func TestRunReturnsShortWriteIdentityAndNoOutputBeforeSuccessfulClose(t *testing.T) {
	app := &fakeApplication{}
	err := run(context.Background(), appConfig{}, shortWriter{}, func(appConfig) (application, error) {
		return app, nil
	}, func(abusecluster.Report) ([]byte, error) { return []byte("report\n"), nil })
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("run() error = %v, want io.ErrShortWrite identity", err)
	}
	if app.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", app.closeCalls)
	}

	stdout := &bytes.Buffer{}
	app = &fakeApplication{closeErr: errors.New("close failed")}
	err = run(context.Background(), appConfig{}, stdout, func(appConfig) (application, error) {
		return app, nil
	}, func(abusecluster.Report) ([]byte, error) { return []byte("report\n"), nil })
	assertRunFailure(t, err, "close", "failed", "", "")
	if stdout.Len() != 0 {
		t.Fatalf("stdout before successful close = %q", stdout.String())
	}
}

func TestRunRejectsCanceledCallerBeforeOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opened := false
	err := run(ctx, appConfig{}, io.Discard, func(appConfig) (application, error) {
		opened = true
		return &fakeApplication{}, nil
	}, abusecluster.EncodeReport)
	assertRunFailure(t, err, "operation", "canceled", "", "")
	if opened {
		t.Fatal("open called for canceled context")
	}
}

func TestRealMainOwnsExitCodeAndRedactedLog(t *testing.T) {
	secret := "bolt://secret:password@localhost:7687"
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := realMain(context.Background(), func(string) string { return secret }, stdout, stderr,
		func(appConfig) (application, error) { t.Fatal("open called"); return nil, nil }, abusecluster.EncodeReport)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if strings.Contains(stderr.String(), secret) {
		t.Fatalf("stderr disclosed URI: %q", stderr.String())
	}
	if got := stderr.String(); got != "application failed stage=configuration class=invalid\n" {
		t.Fatalf("stderr = %q", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRealMainReturnsZeroOnlyAfterCompleteRun(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &fakeApplication{}
	exitCode := realMain(context.Background(), func(string) string { return "bolt://localhost:7687" }, stdout, stderr,
		func(appConfig) (application, error) { return app, nil },
		func(abusecluster.Report) ([]byte, error) { return []byte("{}\n"), nil })
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != "{}\n" {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRealMainReturnsOneForShortWrite(t *testing.T) {
	stderr := &bytes.Buffer{}
	exitCode := realMain(context.Background(), func(string) string { return "bolt://localhost:7687" }, shortWriter{}, stderr,
		func(appConfig) (application, error) { return &fakeApplication{}, nil },
		func(abusecluster.Report) ([]byte, error) { return []byte("{}\n"), nil })
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if got := stderr.String(); got != "application failed stage=write class=failed\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestRealMainRedactsBackendFailureFromStderr(t *testing.T) {
	secret := "bolt://secret:password@localhost:7687"
	opaque := "usr-private-123"
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &fakeApplication{verifyErr: errors.New(secret + " " + opaque)}
	exitCode := realMain(context.Background(), func(string) string { return "bolt://localhost:7687" }, stdout, stderr,
		func(appConfig) (application, error) { return app, nil }, abusecluster.EncodeReport)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if got := stderr.String(); got != "application failed stage=verify class=unavailable\n" {
		t.Fatalf("stderr = %q", got)
	}
	for _, value := range []string{secret, opaque} {
		if strings.Contains(stderr.String(), value) {
			t.Fatalf("stderr disclosed %q: %q", value, stderr.String())
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if app.closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", app.closeCalls)
	}
}

type fakeApplication struct {
	events            *[]string
	verifyErr         error
	executeErr        error
	closeErr          error
	executeHook       func()
	operationDeadline time.Duration
	closeDeadline     time.Duration
	closeContextErr   error
	closeCalls        int
}

func (a *fakeApplication) record(event string) {
	if a.events != nil {
		*a.events = append(*a.events, event)
	}
}

func (a *fakeApplication) VerifyConnectivity(ctx context.Context) error {
	a.record("verify")
	a.operationDeadline = remainingDeadline(ctx)
	return a.verifyErr
}

func (a *fakeApplication) Execute(ctx context.Context) (abusecluster.Report, error) {
	a.record("replace")
	a.record("load")
	a.record("analyze")
	a.operationDeadline = remainingDeadline(ctx)
	if a.executeHook != nil {
		a.executeHook()
	}
	return abusecluster.Report{}, a.executeErr
}

func (a *fakeApplication) Close(ctx context.Context) error {
	a.record("close")
	a.closeCalls++
	a.closeContextErr = ctx.Err()
	a.closeDeadline = remainingDeadline(ctx)
	return a.closeErr
}

type eventWriter struct {
	bytes.Buffer
	events *[]string
}

func (w *eventWriter) Write(p []byte) (int, error) {
	*w.events = append(*w.events, "stdout")
	return w.Buffer.Write(p)
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func assertRunFailure(t *testing.T, err error, stage, class, secrets, opaque string) {
	t.Helper()
	if err == nil {
		t.Fatal("run() error = nil")
	}
	failure, ok := errorStageClass(err)
	if !ok || failure.stage != stage || failure.class != class {
		t.Fatalf("failure = %#v, %v; want stage=%q class=%q", failure, err, stage, class)
	}
	for _, secret := range []string{secrets, opaque} {
		if secret != "" && strings.Contains(err.Error(), secret) {
			t.Fatalf("error disclosed %q: %v", secret, err)
		}
	}
}

func remainingDeadline(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	return time.Until(deadline)
}

func assertDeadlineNear(t *testing.T, got, want time.Duration) {
	t.Helper()
	if got <= 0 || got > want || got < want-time.Second {
		t.Fatalf("remaining deadline = %v, want near %v", got, want)
	}
}
