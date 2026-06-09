package accountmigration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/batch"
)

func TestRunDemoRestartsFromCheckpoint(t *testing.T) {
	result, err := RunDemo(context.Background())
	if err != nil {
		t.Fatalf("RunDemo failed: %v", err)
	}
	if result.FirstRun.Report.Status != batch.StatusFailed {
		t.Fatalf("first status = %s, want failed", result.FirstRun.Report.Status)
	}
	if !strings.Contains(result.FirstRun.Report.Error, ErrMigrationCrash.Error()) {
		t.Fatalf("first error = %q, want migration crash", result.FirstRun.Report.Error)
	}
	if result.RestartRun.Report.Status != batch.StatusCompleted {
		t.Fatalf("restart status = %s, want completed", result.RestartRun.Report.Status)
	}
	if result.FirstRun.Checkpoint == nil || result.FirstRun.Checkpoint.NextIndex != 2 {
		t.Fatalf("first checkpoint = %#v, want next_index=2", result.FirstRun.Checkpoint)
	}
	if result.FinalCheckpoint == nil || result.FinalCheckpoint.NextIndex != 5 {
		t.Fatalf("final checkpoint = %#v, want next_index=5", result.FinalCheckpoint)
	}
	if !slices.Equal(result.FirstRun.ReadIDs, []string{"acct-1001", "acct-1002", "acct-1003"}) {
		t.Fatalf("first read ids = %v", result.FirstRun.ReadIDs)
	}
	if !slices.Equal(result.FirstRun.WrittenIDs, []string{"acct-1001", "acct-1002"}) {
		t.Fatalf("first written ids = %v", result.FirstRun.WrittenIDs)
	}
	if !slices.Equal(result.RestartRun.ReadIDs, []string{"acct-1003", "acct-1004", "acct-1005"}) {
		t.Fatalf("restart read ids = %v", result.RestartRun.ReadIDs)
	}
	if !slices.Equal(result.RestartRun.WrittenIDs, []string{"acct-1003", "acct-1004", "acct-1005"}) {
		t.Fatalf("restart written ids = %v", result.RestartRun.WrittenIDs)
	}
	if result.AccountsMigrated != 5 || len(result.Targets) != 5 {
		t.Fatalf("targets = %d/%d, want 5/5", result.AccountsMigrated, len(result.Targets))
	}
	if containsAny(result.RestartRun.ReadIDs, "acct-1001", "acct-1002") {
		t.Fatalf("restart reprocessed completed chunk: %v", result.RestartRun.ReadIDs)
	}
}

func TestRunMigrationRejectsInvalidCheckpointTypeAndRange(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name  string
		value any
	}{
		{name: "wrong type", value: "bad-checkpoint"},
		{name: "negative index", value: MigrationCheckpoint{NextIndex: -1}},
		{name: "outside range", value: MigrationCheckpoint{NextIndex: 99}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := NewRecordingCheckpointStore()
			if err := store.Save(ctx, DefaultCheckpointKey, tt.value); err != nil {
				t.Fatalf("Save failed: %v", err)
			}
			result, err := RunMigration(ctx, RunOptions{CheckpointStore: store})
			if err != nil {
				t.Fatalf("RunMigration setup failed: %v", err)
			}
			if result.Report.Status != batch.StatusFailed {
				t.Fatalf("status = %s, want failed", result.Report.Status)
			}
			if !strings.Contains(result.Report.Error, ErrInvalidCheckpoint.Error()) {
				t.Fatalf("error = %q, want invalid checkpoint", result.Report.Error)
			}
			if len(result.Targets) != 0 {
				t.Fatalf("targets = %#v, want none", result.Targets)
			}
		})
	}
}

func TestDuplicateTargetWriteFails(t *testing.T) {
	store := NewTargetAccountStore()
	account := TargetAccount{ID: "acct-dup", Email: "dup@example.com", Plan: "gold", Region: "NA"}
	if err := store.Put(account); err != nil {
		t.Fatalf("first Put failed: %v", err)
	}
	if err := store.Put(account); !errors.Is(err, ErrDuplicateAccount) {
		t.Fatalf("duplicate error = %v, want ErrDuplicateAccount", err)
	}
}

func TestCancellationBeforeWorkLeavesNoWrites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := NewRecordingCheckpointStore()
	targets := NewTargetAccountStore()
	result, err := RunMigration(ctx, RunOptions{CheckpointStore: store, TargetStore: targets})
	if err != nil {
		t.Fatalf("RunMigration setup failed: %v", err)
	}
	if result.Report.Status != batch.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", result.Report.Status)
	}
	if len(result.Targets) != 0 || result.Checkpoint != nil {
		t.Fatalf("targets/checkpoint = %#v/%#v, want none", result.Targets, result.Checkpoint)
	}
	if result.ReaderWasClosed || result.WriterWasClosed {
		t.Fatalf("reader/writer should not close because open never happened")
	}
}

func TestCancellationDuringProcessingDoesNotAdvanceCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	processor := batch.ProcessorFunc[LegacyAccount, TargetAccount](func(context.Context, LegacyAccount) (TargetAccount, bool, error) {
		cancel()
		return TargetAccount{}, false, context.Canceled
	})

	store := NewRecordingCheckpointStore()
	targets := NewTargetAccountStore()
	result, err := RunMigration(ctx, RunOptions{
		CheckpointStore: store,
		TargetStore:     targets,
		Processor:       processor,
	})
	if err != nil {
		t.Fatalf("RunMigration setup failed: %v", err)
	}
	if result.Report.Status != batch.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", result.Report.Status)
	}
	if result.Report.ReadCount != 1 || len(result.Targets) != 0 || result.Checkpoint != nil {
		t.Fatalf("read/targets/checkpoint = %d/%#v/%#v, want 1/none/nil", result.Report.ReadCount, result.Targets, result.Checkpoint)
	}
}

func TestReportProjectionOmitsRuntimeTimestamps(t *testing.T) {
	result, err := RunDemo(context.Background())
	if err != nil {
		t.Fatalf("RunDemo failed: %v", err)
	}
	if reflect.ValueOf(result.RestartRun.Report).FieldByName("StartedAt").IsValid() {
		t.Fatal("ReportNode must not expose StartedAt")
	}
	if reflect.ValueOf(result.RestartRun.Report).FieldByName("EndedAt").IsValid() {
		t.Fatal("ReportNode must not expose EndedAt")
	}
}

func TestConcurrentDemoRunsRemainDeterministic(t *testing.T) {
	const (
		workers    = 16
		iterations = 10
	)

	errs := make(chan string, workers*iterations)
	var wg sync.WaitGroup
	for workerID := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range iterations {
				result, err := RunDemo(context.Background())
				if err != nil {
					errs <- fmt.Sprintf("worker %d iteration %d: RunDemo failed: %v", workerID, iteration, err)
					continue
				}
				if err := verifyDemoResult(result); err != nil {
					errs <- fmt.Sprintf("worker %d iteration %d: %v", workerID, iteration, err)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
}

func TestStoresStressConcurrentAccess(t *testing.T) {
	const (
		workers    = 8
		iterations = 40
	)

	ctx := context.Background()
	checkpoints := NewRecordingCheckpointStore()
	targets := NewTargetAccountStore()
	errs := make(chan string, workers*iterations)

	var wg sync.WaitGroup
	for workerID := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range iterations {
				key := fmt.Sprintf("migration-%02d", workerID)
				checkpoint := MigrationCheckpoint{NextIndex: iteration}
				if err := checkpoints.Save(ctx, key, checkpoint); err != nil {
					errs <- fmt.Sprintf("save %s: %v", key, err)
				}
				if _, _, err := checkpoints.Load(ctx, key); err != nil {
					errs <- fmt.Sprintf("load %s: %v", key, err)
				}

				id := fmt.Sprintf("acct-%02d-%03d", workerID, iteration)
				account := TargetAccount{ID: id, Email: id + "@example.com", Plan: "gold", Region: "NA"}
				if err := targets.Put(account); err != nil {
					errs <- fmt.Sprintf("put %s: %v", id, err)
				}
				_ = targets.Snapshot()
				_ = targets.WriteIDs()
				_ = checkpoints.Operations()
			}
		}()
	}
	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
	wantTargets := workers * iterations
	if got := len(targets.Snapshot()); got != wantTargets {
		t.Fatalf("target count = %d, want %d", got, wantTargets)
	}
	if err := verifyUniqueTargets(targets.Snapshot()); err != nil {
		t.Fatal(err)
	}
	if got := len(checkpoints.Operations()); got < workers*iterations*2 {
		t.Fatalf("checkpoint operations = %d, want at least %d", got, workers*iterations*2)
	}
}

func verifyDemoResult(result DemoResult) error {
	if result.FirstRun.Report.Status != batch.StatusFailed {
		return fmt.Errorf("first status = %s, want failed", result.FirstRun.Report.Status)
	}
	if result.RestartRun.Report.Status != batch.StatusCompleted {
		return fmt.Errorf("restart status = %s, want completed", result.RestartRun.Report.Status)
	}
	if result.FirstRun.Checkpoint == nil || result.FirstRun.Checkpoint.NextIndex != 2 {
		return fmt.Errorf("first checkpoint = %#v, want next_index=2", result.FirstRun.Checkpoint)
	}
	if result.FinalCheckpoint == nil || result.FinalCheckpoint.NextIndex != 5 {
		return fmt.Errorf("final checkpoint = %#v, want next_index=5", result.FinalCheckpoint)
	}
	if containsAny(result.RestartRun.ReadIDs, "acct-1001", "acct-1002") {
		return fmt.Errorf("restart reprocessed completed chunk: %v", result.RestartRun.ReadIDs)
	}
	if len(result.Targets) != 5 {
		return fmt.Errorf("targets = %d, want 5", len(result.Targets))
	}
	return verifyUniqueTargets(result.Targets)
}

func verifyUniqueTargets(targets []TargetAccount) error {
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if _, exists := seen[target.ID]; exists {
			return fmt.Errorf("duplicate target %q", target.ID)
		}
		seen[target.ID] = struct{}{}
	}
	return nil
}

func containsAny(values []string, needles ...string) bool {
	for _, needle := range needles {
		if slices.Contains(values, needle) {
			return true
		}
	}
	return false
}
