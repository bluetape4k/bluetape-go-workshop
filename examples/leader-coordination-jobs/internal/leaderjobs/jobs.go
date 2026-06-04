// Package leaderjobs demonstrates Redis leader election for backend jobs.
package leaderjobs

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

// MigrationGate runs a one-shot task only after this instance becomes leader.
func MigrationGate(ctx context.Context, elector leader.Elector, migrate func(context.Context) error) error {
	if err := elector.Campaign(ctx); err != nil {
		if errors.Is(err, leader.ErrAlreadyLeader) {
			return migrate(ctx)
		}
		return err
	}
	defer func() {
		_ = elector.Resign(context.Background())
	}()
	return migrate(ctx)
}

// CacheWarmer runs periodic work while leadership is held.
type CacheWarmer struct {
	elector leader.Elector
	warm    func(context.Context) error
	count   atomic.Int32
}

// NewCacheWarmer creates a leader-owned cache warmer.
func NewCacheWarmer(elector leader.Elector, warm func(context.Context) error) *CacheWarmer {
	return &CacheWarmer{elector: elector, warm: warm}
}

// Runs returns how many warm cycles completed.
func (w *CacheWarmer) Runs() int {
	return int(w.count.Load())
}

// Run starts the warmer and stops on cancellation, resign, or lost leadership.
func (w *CacheWarmer) Run(ctx context.Context, interval time.Duration) error {
	if err := w.elector.Campaign(ctx); err != nil && !errors.Is(err, leader.ErrAlreadyLeader) {
		return err
	}
	defer func() {
		_ = w.elector.Resign(context.Background())
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if !w.elector.IsLeader() {
			return leader.ErrNotLeader
		}
		if err := w.warm(ctx); err != nil {
			return err
		}
		w.count.Add(1)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
