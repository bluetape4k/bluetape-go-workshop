// Package main 실행 가능한 로컬 graph recommendation 예제를 제공합니다.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-recommendation/internal/recommendation"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "graph recommendation failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout io.Writer) error {
	if ctx == nil || stdout == nil {
		return recommendation.ErrInvalidContext
	}
	fixture, err := recommendation.DefaultFixture()
	if err != nil {
		return err
	}
	graphValue, err := recommendation.NewGraph(fixture.Vertices, fixture.Edges)
	if err != nil {
		return err
	}
	report, err := graphValue.Recommend(ctx, "alice", 10)
	if err != nil {
		return err
	}
	encoded, err := recommendation.EncodeReport(report)
	if err != nil {
		return err
	}
	return writeFull(stdout, encoded)
}

func writeFull(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		written, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		payload = payload[written:]
	}
	return nil
}
