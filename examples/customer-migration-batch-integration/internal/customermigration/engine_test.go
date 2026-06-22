package customermigration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/batch"
)

func TestRunBatchRestartsAfterWriterCrash(t *testing.T) {
	t.Parallel()

	stores := NewStores()

	failed, err := RunBatch(context.Background(), RunOptions{
		RunID:               "manual-001",
		Trigger:             TriggerManual,
		Stores:              stores,
		CrashAfterNewWrites: 3,
	})
	if !errors.Is(err, ErrWriterCrash) {
		t.Fatalf("RunBatch() error = %v, want ErrWriterCrash", err)
	}
	if failed.Status != batch.StatusFailed {
		t.Fatalf("failed.Status = %s, want %s", failed.Status, batch.StatusFailed)
	}
	if got := failed.Checkpoint.NextIndex; got != 2 {
		t.Fatalf("failed checkpoint NextIndex = %d, want 2", got)
	}
	if !contains(failed.MigratedIDs, "cust-1003") {
		t.Fatalf("failed run migrated IDs = %v, want partial cust-1003 write", failed.MigratedIDs)
	}

	completed, err := RunBatch(context.Background(), RunOptions{
		RunID:   "scheduled-001",
		Trigger: TriggerSchedule,
		Stores:  stores,
	})
	if err != nil {
		t.Fatalf("restart RunBatch() error = %v", err)
	}
	if completed.Status != batch.StatusCompleted {
		t.Fatalf("completed.Status = %s, want %s", completed.Status, batch.StatusCompleted)
	}
	if got := completed.Checkpoint.NextIndex; got != 5 {
		t.Fatalf("completed checkpoint NextIndex = %d, want 5", got)
	}
	assertStringSlicesEqual(t, completed.ReadIDs, []string{"cust-1003", "cust-1004", "cust-1005"})
	assertStringSlicesEqual(t, completed.AcceptedIDs, []string{"cust-1003", "cust-1005"})
	assertStringSlicesEqual(t, completed.NewWrittenIDs, []string{"cust-1005"})
	assertStringSlicesEqual(t, completed.MigratedIDs, []string{"cust-1001", "cust-1002", "cust-1003", "cust-1005"})
	if got := completed.Summary.DuplicateSkipCount; got != 1 {
		t.Fatalf("DuplicateSkipCount = %d, want 1", got)
	}
	if got := completed.Summary.RetryCount; got != 1 {
		t.Fatalf("RetryCount = %d, want 1", got)
	}
	if got := completed.Summary.SkipCount; got != 1 {
		t.Fatalf("SkipCount = %d, want 1", got)
	}
	if got := completed.Summary.WriteCount; got != 2 {
		t.Fatalf("WriteCount = %d, want 2", got)
	}
	if got := completed.Summary.ChunkSize; got != 2 {
		t.Fatalf("ChunkSize = %d, want 2", got)
	}
	if len(completed.DeadLetters) != 1 {
		t.Fatalf("DeadLetters len = %d, want 1: %#v", len(completed.DeadLetters), completed.DeadLetters)
	}
	if completed.DeadLetters[0].CustomerID != "cust-1004" {
		t.Fatalf("dead letter customer = %s, want cust-1004", completed.DeadLetters[0].CustomerID)
	}
	if containsEmail(completed) {
		t.Fatalf("public response leaked fixture email: %#v", completed)
	}
	if completed.Report.StartedAt != "" || completed.Report.EndedAt != "" {
		t.Fatalf("report projection must omit timestamps: %#v", completed.Report)
	}
}

func TestRunBatchRejectsInvalidCheckpoint(t *testing.T) {
	t.Parallel()

	stores := NewStores()
	if err := stores.Checkpoints.Save(context.Background(), CheckpointKey, Checkpoint{NextIndex: 99}); err != nil {
		t.Fatalf("Save checkpoint: %v", err)
	}

	_, err := RunBatch(context.Background(), RunOptions{
		RunID:   "manual-001",
		Trigger: TriggerManual,
		Stores:  stores,
	})
	if !errors.Is(err, ErrInvalidCheckpoint) {
		t.Fatalf("RunBatch() error = %v, want ErrInvalidCheckpoint", err)
	}
}

func TestRunBatchCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response, err := RunBatch(ctx, RunOptions{
		RunID:   "manual-001",
		Trigger: TriggerManual,
		Stores:  NewStores(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunBatch() error = %v, want context.Canceled", err)
	}
	if response.Status != batch.StatusCancelled {
		t.Fatalf("Status = %s, want %s", response.Status, batch.StatusCancelled)
	}
	if response.ErrorCode != ErrorCodeRequestCancelled {
		t.Fatalf("ErrorCode = %s, want %s", response.ErrorCode, ErrorCodeRequestCancelled)
	}
}

func TestDefaultChunkSize(t *testing.T) {
	t.Parallel()

	if DefaultChunkSize != 2 {
		t.Fatalf("DefaultChunkSize = %d, want 2", DefaultChunkSize)
	}
}

func assertStringSlicesEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d; got=%v want=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d = %q, want %q; got=%v want=%v", i, got[i], want[i], got, want)
		}
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsEmail(response RunResponse) bool {
	rendered := response.String()
	return strings.Contains(rendered, "@example.test")
}
