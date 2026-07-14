// Package main runs the graph abuse-cluster example against a loopback Neo4j instance.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-abuse-cluster/internal/abusecluster"
	neo4jgraph "github.com/bluetape4k/bluetape-go/graph/neo4j"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

const (
	neo4jURIEnvironment = "NEO4J_URI"
	operationTimeout    = 15 * time.Second
	cleanupTimeout      = 3 * time.Second
)

type appConfig struct {
	neo4jURI string
}

type application interface {
	VerifyConnectivity(context.Context) error
	Execute(context.Context) (abusecluster.Report, error)
	Close(context.Context) error
}

type openApplicationFunc func(appConfig) (application, error)
type encodeReportFunc func(abusecluster.Report) ([]byte, error)

type neo4jApplication struct {
	client  *neo4jgraph.Client
	store   *abusecluster.Store
	fixture abusecluster.Fixture
}

type stageFailure struct {
	stage string
	class string
	cause error
}

func (e *stageFailure) Error() string { return e.stage + ": " + e.class }
func (e *stageFailure) Unwrap() error { return e.cause }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := realMain(ctx, os.Getenv, os.Stdout, os.Stderr, openNeo4jApplication, abusecluster.EncodeReport)
	stop()
	os.Exit(exitCode)
}

func loadConfig(getenv func(string) string) (appConfig, error) {
	if getenv == nil {
		return appConfig{}, newFailure("configuration", "invalid", abusecluster.ErrConfiguration)
	}
	raw := getenv(neo4jURIEnvironment)
	if raw == "" || strings.TrimSpace(raw) != raw || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return appConfig{}, newFailure("configuration", "invalid", abusecluster.ErrConfiguration)
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "bolt" || parsed.Opaque != "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery ||
		parsed.Fragment != "" || strings.ContainsAny(raw, "?#") {
		return appConfig{}, newFailure("configuration", "invalid", abusecluster.ErrConfiguration)
	}

	host := parsed.Hostname()
	portText := parsed.Port()
	if host == "" || portText == "" || !decimalDigits(portText) {
		return appConfig{}, newFailure("configuration", "invalid", abusecluster.ErrConfiguration)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || !loopbackHost(host) {
		return appConfig{}, newFailure("configuration", "invalid", abusecluster.ErrConfiguration)
	}
	return appConfig{neo4jURI: raw}, nil
}

func run(
	ctx context.Context,
	cfg appConfig,
	stdout io.Writer,
	open openApplicationFunc,
	encode encodeReportFunc,
) (result error) {
	if ctx == nil || isNilValue(stdout) || open == nil || encode == nil {
		return newFailure("configuration", "invalid", abusecluster.ErrConfiguration)
	}

	operationCtx, cancelOperation := context.WithTimeout(ctx, operationTimeout)
	defer cancelOperation()
	if err := operationCtx.Err(); err != nil {
		return contextFailure("operation", err)
	}

	app, err := open(cfg)
	if err != nil || isNilValue(app) {
		return newFailure("open", "unavailable", err)
	}
	closeAttempted := false
	closeApplication := func() error {
		closeAttempted = true
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancelCleanup()
		return app.Close(cleanupCtx)
	}
	defer func() {
		if closeAttempted {
			return
		}
		if closeErr := closeApplication(); result == nil && closeErr != nil {
			result = newFailure("close", "failed", closeErr)
		}
	}()

	if err := app.VerifyConnectivity(operationCtx); err != nil {
		return classifyFailure("verify", "unavailable", err)
	}
	if err := operationCtx.Err(); err != nil {
		return contextFailure("operation", err)
	}
	report, err := app.Execute(operationCtx)
	if err != nil {
		return classifyFailure("workflow", "failed", err)
	}
	if err := operationCtx.Err(); err != nil {
		return contextFailure("operation", err)
	}
	encoded, err := encode(report)
	if err != nil || len(encoded) == 0 {
		return newFailure("encode", "failed", err)
	}
	if err := closeApplication(); err != nil {
		return newFailure("close", "failed", err)
	}
	if err := writeFull(stdout, encoded); err != nil {
		return newFailure("write", "failed", err)
	}
	return nil
}

func realMain(
	ctx context.Context,
	getenv func(string) string,
	stdout io.Writer,
	stderr io.Writer,
	open openApplicationFunc,
	encode encodeReportFunc,
) int {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := loadConfig(getenv)
	if err == nil {
		err = run(ctx, cfg, stdout, open, encode)
	}
	if err == nil {
		return 0
	}
	failure, ok := errorStageClass(err)
	if !ok {
		failure = stageFailure{stage: "application", class: "failed"}
	}
	if !isNilValue(stderr) {
		_, _ = fmt.Fprintf(stderr, "application failed stage=%s class=%s\n", failure.stage, failure.class)
	}
	return 1
}

func openNeo4jApplication(cfg appConfig) (application, error) {
	driver, err := neo4jdriver.NewDriver(cfg.neo4jURI, neo4jdriver.NoAuth())
	if err != nil {
		return nil, err
	}
	client, err := neo4jgraph.NewClient(driver)
	if err != nil {
		closeDriver(driver)
		return nil, err
	}
	store, err := abusecluster.NewStore(client)
	if err != nil {
		closeClient(client)
		return nil, err
	}
	fixture, err := abusecluster.DefaultFixture()
	if err != nil {
		closeClient(client)
		return nil, err
	}
	return &neo4jApplication{client: client, store: store, fixture: fixture}, nil
}

func (a *neo4jApplication) VerifyConnectivity(ctx context.Context) error {
	return a.client.VerifyConnectivity(ctx)
}

func (a *neo4jApplication) Execute(ctx context.Context) (abusecluster.Report, error) {
	return abusecluster.Execute(ctx, a.store, a.fixture)
}

func (a *neo4jApplication) Close(ctx context.Context) error {
	return a.client.Close(ctx)
}

func closeDriver(driver neo4jdriver.Driver) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_ = driver.Close(ctx)
}

func closeClient(client *neo4jgraph.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_ = client.Close(ctx)
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeFull(writer io.Writer, data []byte) error {
	written, err := writer.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func newFailure(stage, class string, cause error) error {
	return &stageFailure{stage: stage, class: class, cause: cause}
}

func classifyFailure(stage, fallbackClass string, err error) error {
	if errors.Is(err, context.Canceled) {
		return contextFailure(stage, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return contextFailure(stage, context.DeadlineExceeded)
	}
	return newFailure(stage, fallbackClass, err)
}

func contextFailure(stage string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return newFailure(stage, "deadline", context.DeadlineExceeded)
	}
	return newFailure(stage, "canceled", context.Canceled)
}

func errorStageClass(err error) (stageFailure, bool) {
	var failure *stageFailure
	if !errors.As(err, &failure) || failure == nil {
		return stageFailure{}, false
	}
	return *failure, true
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
