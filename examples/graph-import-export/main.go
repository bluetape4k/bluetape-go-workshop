// Package main 은 bounded graph import/export 예제를 실행합니다.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/bluetape4k/bluetape-go-workshop/examples/graph-import-export/internal/graphimport"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "graph import/export failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout io.Writer) error {
	if ctx == nil {
		return graphimport.ErrInvalidContext
	}
	if stdout == nil {
		return graphimport.ErrInvalidInput
	}
	demo, err := graphimport.RunDemo(ctx)
	if err != nil {
		return err
	}
	encoded, err := graphimport.EncodeDemo(demo)
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
