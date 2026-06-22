package customermigration

import (
	"context"
	"fmt"
	"time"
)

// RunScheduler executes one scheduled tick per received tick value.
func RunScheduler(ctx context.Context, service *Service, ticks <-chan time.Time, runIDPrefix string) error {
	if service == nil {
		return fmt.Errorf("service must not be nil")
	}
	if runIDPrefix == "" {
		runIDPrefix = "scheduled"
	}
	for round := 1; ; round++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ticks:
			if !ok {
				return nil
			}
			_, _ = service.RunScheduledTick(ctx, ScheduleRequest{
				RunID: fmt.Sprintf("%s-%03d", runIDPrefix, round),
			})
		}
	}
}
