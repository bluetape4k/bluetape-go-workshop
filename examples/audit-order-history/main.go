// Command audit-order-history는 현재 order state와 immutable history를 함께 출력한다.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/audit-order-history/internal/orderhistory"
	"github.com/bluetape4k/bluetape-go/audit"
)

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "audit order history: %v\n", err)
		os.Exit(1)
	}
}

func run(output io.Writer) error {
	timestamps := []time.Time{
		time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 13, 9, 5, 0, 0, time.UTC),
		time.Date(2026, 7, 13, 9, 10, 0, 0, time.UTC),
	}
	next := 0
	service, err := orderhistory.NewService(audit.NewMemoryRepository(), orderhistory.Options{
		Author: "workshop-preview",
		Now: func() time.Time {
			value := timestamps[next]
			next++
			return value
		},
	})
	if err != nil {
		return err
	}
	preview, err := orderhistory.BuildPreview(context.Background(), service)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preview: %w", err)
	}
	if _, err := fmt.Fprintln(output, string(data)); err != nil {
		return fmt.Errorf("write preview: %w", err)
	}
	return nil
}
