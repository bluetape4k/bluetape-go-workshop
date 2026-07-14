package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
)

func TestDocumentationParity(t *testing.T) {
	directory := exampleDirectory(t)
	requests, err := os.ReadFile(filepath.Join(directory, "requests.http"))
	if err != nil {
		t.Fatal(err)
	}
	english, err := os.ReadFile(filepath.Join(directory, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	korean, err := os.ReadFile(filepath.Join(directory, "README.ko.md"))
	if err != nil {
		t.Fatal(err)
	}
	scenarios := parseHTTPScenarios(t, requests)
	if len(scenarios) != 9 {
		t.Fatalf("scenarios = %d, want 9", len(scenarios))
	}
	for _, scenario := range scenarios {
		for locale, document := range map[string][]byte{"English": english, "Korean": korean} {
			if !bytes.Contains(document, []byte(scenario.body)) || !bytes.Contains(document, []byte(strings.TrimPrefix(scenario.path, "{{baseURL}}"))) {
				t.Fatalf("%s README is missing scenario %q", locale, scenario.name)
			}
		}
	}
	for _, marker := range []string{"replayed: true", "next_from_revision: 2", "next_from_revision: null", "invalid_transition", "too_many_requests", "at-least-once", "30-second", "30초"} {
		if marker == "30초" {
			if !bytes.Contains(korean, []byte(marker)) {
				t.Fatalf("Korean README missing %q", marker)
			}
			continue
		}
		if !bytes.Contains(english, []byte(marker)) {
			t.Fatalf("English README missing %q", marker)
		}
	}
}

func TestRequestsHTTPSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	postgresURL := postgrestestcontainer.Start(ctx, t)
	redisAddress := redistestcontainer.Start(ctx, t)
	httpAddress := unusedLoopbackAddress(t)
	config := appConfig{
		databaseURL: postgresURL, redisAddress: redisAddress,
		redisStream: "workshop:requests-http-smoke", httpAddress: httpAddress,
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- run(runCtx, config, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	waitForReady(t, ctx, "http://"+httpAddress+"/readyz")

	requests, err := os.ReadFile(filepath.Join(exampleDirectory(t), "requests.http"))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	for _, scenario := range parseHTTPScenarios(t, requests) {
		url := strings.ReplaceAll(scenario.path, "{{baseURL}}", "http://"+httpAddress)
		request, err := http.NewRequestWithContext(ctx, scenario.method, url, strings.NewReader(scenario.body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("%s: %v", scenario.name, err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("%s response read/close = (%v, %v)", scenario.name, readErr, closeErr)
		}
		if response.StatusCode != scenario.status {
			t.Fatalf("%s status = %d, body = %s", scenario.name, response.StatusCode, body)
		}
		assertScenarioResponse(t, scenario, body)
	}
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() shutdown error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("application did not stop")
	}
}

type httpScenario struct {
	name      string
	method    string
	path      string
	body      string
	status    int
	replayed  bool
	hasReplay bool
	next      string
	errorCode string
}

func parseHTTPScenarios(t *testing.T, content []byte) []httpScenario {
	t.Helper()
	blocks := strings.Split(string(content), "### ")
	scenarios := make([]httpScenario, 0, len(blocks)-1)
	for _, block := range blocks[1:] {
		scanner := bufio.NewScanner(strings.NewReader(block))
		if !scanner.Scan() {
			continue
		}
		scenario := httpScenario{name: strings.TrimSpace(scanner.Text())}
		bodyLines := make([]string, 0, 1)
		readingBody := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			switch {
			case strings.HasPrefix(line, "# @expect-status "):
				scenario.status, _ = strconv.Atoi(strings.TrimPrefix(line, "# @expect-status "))
			case strings.HasPrefix(line, "# @expect-replayed "):
				scenario.hasReplay = true
				scenario.replayed = strings.TrimPrefix(line, "# @expect-replayed ") == "true"
			case strings.HasPrefix(line, "# @expect-next-from-revision "):
				scenario.next = strings.TrimPrefix(line, "# @expect-next-from-revision ")
			case strings.HasPrefix(line, "# @expect-error-code "):
				scenario.errorCode = strings.TrimPrefix(line, "# @expect-error-code ")
			case strings.HasPrefix(line, "POST "):
				scenario.method = http.MethodPost
				scenario.path = strings.TrimPrefix(line, "POST ")
			case line == "" && scenario.method != "":
				readingBody = true
			case readingBody && line != "":
				bodyLines = append(bodyLines, line)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		scenario.body = strings.Join(bodyLines, "\n")
		if scenario.method == "" || scenario.path == "" || scenario.body == "" || scenario.status == 0 {
			t.Fatalf("invalid HTTP scenario %q: %+v", scenario.name, scenario)
		}
		if !json.Valid([]byte(scenario.body)) {
			t.Fatalf("invalid JSON body for %q: %s", scenario.name, scenario.body)
		}
		scenarios = append(scenarios, scenario)
	}
	return scenarios
}

func assertScenarioResponse(t *testing.T, scenario httpScenario, body []byte) {
	t.Helper()
	if scenario.hasReplay && !bytes.Contains(body, []byte(fmt.Sprintf(`"replayed":%t`, scenario.replayed))) {
		t.Fatalf("%s body missing replay marker: %s", scenario.name, body)
	}
	if scenario.next != "" && !bytes.Contains(body, []byte(`"next_from_revision":`+scenario.next)) {
		t.Fatalf("%s body missing cursor %s: %s", scenario.name, scenario.next, body)
	}
	if scenario.errorCode != "" && !bytes.Contains(body, []byte(`"code":"`+scenario.errorCode+`"`)) {
		t.Fatalf("%s body missing error code: %s", scenario.name, body)
	}
}

func unusedLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func waitForReady(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("application did not become ready")
}

func exampleDirectory(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return directory
}
