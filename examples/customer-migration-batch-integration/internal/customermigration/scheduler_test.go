package customermigration

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunSchedulerTriggersTicksAndStopsOnCancellation(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceOptions{LeaderGate: NewStaticLeaderGate(true)})
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunScheduler(ctx, service, ticks, "scheduled")
	}()

	ticks <- time.Now()
	ticks <- time.Now()
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunScheduler() error = %v, want context.Canceled", err)
	}
	if service.Status().Latest == nil {
		t.Fatalf("scheduler did not run any tick")
	}
}

func TestRunSchedulerRejectsNilService(t *testing.T) {
	t.Parallel()

	err := RunScheduler(context.Background(), nil, make(chan time.Time), "scheduled")
	if err == nil {
		t.Fatalf("RunScheduler() error = nil, want error")
	}
}
