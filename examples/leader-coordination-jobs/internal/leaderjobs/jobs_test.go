package leaderjobs_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/leader-coordination-jobs/internal/leaderjobs"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestMigrationGateRunsOnlyForLeader(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = client.Close() })

	first := newElector(t, client, "migration-gate", "instance-1")
	second := newElector(t, client, "migration-gate", "instance-2")

	var migrations atomic.Int32
	if err := leaderjobs.MigrationGate(ctx, first, func(context.Context) error {
		migrations.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("first gate: %v", err)
	}
	if err := leaderjobs.MigrationGate(ctx, second, func(context.Context) error {
		migrations.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("second gate after resign: %v", err)
	}
	if migrations.Load() != 2 {
		t.Fatalf("migrations = %d", migrations.Load())
	}
}

func TestCacheWarmerStopsOnCancellationWithAsyncJobTester(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = client.Close() })

	var sequence atomic.Int32
	job := func(ctx context.Context) error {
		id := sequence.Add(1)
		elector := newElector(t, client, "cache-warmer-"+fmt.Sprint(id), "instance")
		warmer := leaderjobs.NewCacheWarmer(elector, func(context.Context) error { return nil })
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
		defer cancel()
		err := warmer.Run(runCtx, 10*time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline, got %v", err)
		}
		return nil
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{Workers: 2, RoundsPerTask: 2, Timeout: time.Second})
	report := tester.RunT(t, job, job)
	if report.Completed != 4 {
		t.Fatalf("report = %+v", report)
	}
}

func newElector(t *testing.T, client redis.Cmdable, group, member string) *redisleader.Elector {
	t.Helper()
	elector, err := redisleader.New(client, leader.Options{
		Group:         group,
		MemberID:      member,
		Lease:         time.Second,
		RenewInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}
	return elector
}
