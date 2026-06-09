package csvimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/batch"
)

func TestRunImportCompletesInitialImport(t *testing.T) {
	store := batch.NewMemoryCheckpointStore()
	sink := NewCustomerSink()

	run, err := RunImport(t.Context(), RunOptions{
		CSVPath:         fixturePath(),
		CheckpointStore: store,
		Sink:            sink,
	})
	if err != nil {
		t.Fatalf("RunImport failed: %v", err)
	}

	if run.Report.Status != batch.StatusCompleted {
		t.Fatalf("status = %s, want completed; report=%+v", run.Report.Status, run.Report)
	}
	if run.Report.ReadCount != 5 || run.Report.WriteCount != 5 {
		t.Fatalf("report counts = read:%d write:%d, want 5/5", run.Report.ReadCount, run.Report.WriteCount)
	}
	assertCheckpoint(t, run.Checkpoint, 5)
	if run.NewCommits != 5 || run.DuplicateSkips != 0 || run.WriteAttempts != 5 {
		t.Fatalf("sink counts = new:%d duplicate:%d attempts:%d, want 5/0/5", run.NewCommits, run.DuplicateSkips, run.WriteAttempts)
	}
	if !run.ReaderWasClosed || !run.WriterWasClosed {
		t.Fatalf("reader/writer closed = %v/%v, want true/true", run.ReaderWasClosed, run.WriterWasClosed)
	}
	assertCustomerIDs(t, run.Customers, "cust-1001", "cust-1002", "cust-1003", "cust-1004", "cust-1005")
	if run.Customers[0].Email != "alice@example.com" || run.Customers[0].Tier != "gold" {
		t.Fatalf("first customer = %#v, want normalized email and tier", run.Customers[0])
	}
}

func TestRunDemoFailsMidRunThenRestartsFromCheckpoint(t *testing.T) {
	result, err := RunDemo(t.Context(), fixturePath())
	if err != nil {
		t.Fatalf("RunDemo failed: %v", err)
	}

	first := result.FirstRun
	if first.Report.Status != batch.StatusFailed {
		t.Fatalf("first status = %s, want failed", first.Report.Status)
	}
	if !strings.Contains(first.Report.Error, ErrSimulatedCrash.Error()) {
		t.Fatalf("first error = %q, want simulated crash", first.Report.Error)
	}
	if first.Report.ReadCount != 4 || first.Report.WriteCount != 2 {
		t.Fatalf("first report counts = read:%d write:%d, want 4/2", first.Report.ReadCount, first.Report.WriteCount)
	}
	assertCheckpoint(t, first.Checkpoint, 2)
	assertCustomerIDs(t, first.Customers, "cust-1001", "cust-1002", "cust-1003")

	restart := result.RestartRun
	if restart.Report.Status != batch.StatusCompleted {
		t.Fatalf("restart status = %s, want completed; report=%+v", restart.Report.Status, restart.Report)
	}
	if restart.Report.ReadCount != 3 || restart.Report.WriteCount != 3 {
		t.Fatalf("restart report counts = read:%d write:%d, want 3/3", restart.Report.ReadCount, restart.Report.WriteCount)
	}
	assertCheckpoint(t, restart.Checkpoint, 5)
	if result.CustomersImported != 5 || result.DuplicateSkips != 1 {
		t.Fatalf("result counts = imported:%d duplicate:%d, want 5/1", result.CustomersImported, result.DuplicateSkips)
	}
	if restart.NewCommits != 5 || restart.DuplicateSkips != 1 || restart.WriteAttempts != 6 {
		t.Fatalf("sink counts after restart = new:%d duplicate:%d attempts:%d, want 5/1/6", restart.NewCommits, restart.DuplicateSkips, restart.WriteAttempts)
	}
	assertCustomerIDs(t, result.Customers, "cust-1001", "cust-1002", "cust-1003", "cust-1004", "cust-1005")
	assertUniqueCustomerIDs(t, result.Customers)
}

func TestRunImportRejectsMalformedHeader(t *testing.T) {
	path := writeCSV(t, "id,email,tier\ncust-1,a@example.com,gold\n")

	run, err := RunImport(t.Context(), RunOptions{CSVPath: path})
	if err != nil {
		t.Fatalf("RunImport returned setup error: %v", err)
	}

	if run.Report.Status != batch.StatusFailed {
		t.Fatalf("status = %s, want failed", run.Report.Status)
	}
	if !strings.Contains(run.Report.Error, ErrInvalidCSV.Error()) {
		t.Fatalf("error = %q, want invalid csv", run.Report.Error)
	}
	if len(run.Customers) != 0 {
		t.Fatalf("customers = %#v, want none", run.Customers)
	}
	if run.Checkpoint != nil {
		t.Fatalf("checkpoint = %#v, want nil", run.Checkpoint)
	}
}

func TestRunImportRejectsInvalidCustomerRow(t *testing.T) {
	path := writeCSV(t, "customer_id,email,tier\ncust-1,a@example.com,gold\ncust-2,   ,silver\n")

	run, err := RunImport(t.Context(), RunOptions{CSVPath: path})
	if err != nil {
		t.Fatalf("RunImport returned setup error: %v", err)
	}

	if run.Report.Status != batch.StatusFailed {
		t.Fatalf("status = %s, want failed", run.Report.Status)
	}
	if !strings.Contains(run.Report.Error, ErrInvalidCustomer.Error()) || !strings.Contains(run.Report.Error, "line 3") {
		t.Fatalf("error = %q, want invalid customer row context", run.Report.Error)
	}
	if run.Report.ReadCount != 2 || run.Report.WriteCount != 0 {
		t.Fatalf("counts = read:%d write:%d, want 2/0", run.Report.ReadCount, run.Report.WriteCount)
	}
	if run.Checkpoint != nil {
		t.Fatalf("checkpoint = %#v, want nil because first chunk never committed", run.Checkpoint)
	}
}

func TestRunImportCancellationBeforeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	run, err := RunImport(ctx, RunOptions{CSVPath: fixturePath()})
	if err != nil {
		t.Fatalf("RunImport returned setup error: %v", err)
	}

	if run.Report.Status != batch.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", run.Report.Status)
	}
	if !strings.Contains(run.Report.Error, context.Canceled.Error()) {
		t.Fatalf("error = %q, want context canceled", run.Report.Error)
	}
	if run.Report.ReadCount != 0 || run.Report.WriteCount != 0 {
		t.Fatalf("counts = read:%d write:%d, want 0/0", run.Report.ReadCount, run.Report.WriteCount)
	}
	if run.ReaderWasClosed || run.WriterWasClosed {
		t.Fatalf("closed = reader:%v writer:%v, want false/false because open never happened", run.ReaderWasClosed, run.WriterWasClosed)
	}
	if run.Checkpoint != nil {
		t.Fatalf("checkpoint = %#v, want nil", run.Checkpoint)
	}
}

func TestStepCancellationAfterCommittedChunkPreservesCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	store := batch.NewMemoryCheckpointStore()
	sink := NewCustomerSink()
	reader := NewCSVReader(fixturePath())
	writer := NewCustomerWriter(sink, WriterOptions{})
	processor := batch.ProcessorFunc[CSVRow, Customer](func(ctx context.Context, row CSVRow) (Customer, bool, error) {
		if row.Index == 2 {
			cancel()
		}
		return CustomerProcessor().Process(ctx, row)
	})
	step, err := batch.NewStep(batch.StepOptions[CSVRow, Customer]{
		Name:            StepName,
		ChunkSize:       DefaultChunkSize,
		Reader:          reader,
		Processor:       processor,
		Writer:          writer,
		CheckpointStore: store,
		CheckpointKey:   DefaultCheckpointKey,
	})
	if err != nil {
		t.Fatalf("NewStep failed: %v", err)
	}

	report := step.Run(ctx)

	if report.Status != batch.StatusCancelled {
		t.Fatalf("status = %s, want cancelled; report=%+v", report.Status, report)
	}
	if !errors.Is(report.Err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", report.Err)
	}
	if report.ReadCount != 3 || report.WriteCount != 2 {
		t.Fatalf("counts = read:%d write:%d, want 3/2", report.ReadCount, report.WriteCount)
	}
	value, exists, err := store.Load(context.Background(), DefaultCheckpointKey)
	if err != nil || !exists {
		t.Fatalf("checkpoint load exists=%v err=%v, want exists", exists, err)
	}
	checkpoint, ok := value.(Checkpoint)
	if !ok {
		t.Fatalf("checkpoint type = %T, want Checkpoint", value)
	}
	if checkpoint.NextRow != 2 {
		t.Fatalf("checkpoint next_row = %d, want 2", checkpoint.NextRow)
	}
	if !reader.Closed() || !writer.Closed() {
		t.Fatalf("reader/writer closed = %v/%v, want true/true", reader.Closed(), writer.Closed())
	}
	assertCustomerIDs(t, sink.Snapshot(), "cust-1001", "cust-1002")
}

func TestReportProjectionOmitsRuntimeTimestamps(t *testing.T) {
	result, err := RunDemo(t.Context(), fixturePath())
	if err != nil {
		t.Fatalf("RunDemo failed: %v", err)
	}

	if reflect.ValueOf(result.FirstRun.Report).FieldByName("StartedAt").IsValid() {
		t.Fatal("report projection unexpectedly exposes StartedAt")
	}
	if reflect.ValueOf(result.FirstRun.Report).FieldByName("EndedAt").IsValid() {
		t.Fatal("report projection unexpectedly exposes EndedAt")
	}
}

func fixturePath() string {
	return filepath.Join("..", "..", "testdata", "customers.csv")
}

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "customers.csv")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func assertCheckpoint(t *testing.T, checkpoint *Checkpoint, nextRow int) {
	t.Helper()
	if checkpoint == nil {
		t.Fatalf("checkpoint = nil, want next_row %d", nextRow)
	}
	if checkpoint.NextRow != nextRow {
		t.Fatalf("checkpoint next_row = %d, want %d", checkpoint.NextRow, nextRow)
	}
}

func assertCustomerIDs(t *testing.T, customers []Customer, expected ...string) {
	t.Helper()
	got := make([]string, 0, len(customers))
	for _, customer := range customers {
		got = append(got, customer.ID)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("customer IDs = %#v, want %#v", got, expected)
	}
}

func assertUniqueCustomerIDs(t *testing.T, customers []Customer) {
	t.Helper()
	seen := make(map[string]struct{}, len(customers))
	for _, customer := range customers {
		if _, exists := seen[customer.ID]; exists {
			t.Fatalf("duplicate customer ID committed: %s", customer.ID)
		}
		seen[customer.ID] = struct{}{}
	}
}
