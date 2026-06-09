package ticketworker

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

func TestRunDemoCompletesWithRetryAndDeadLetter(t *testing.T) {
	result, err := RunDemo(context.Background())
	if err != nil {
		t.Fatalf("RunDemo failed: %v", err)
	}
	if result.Status != batch.StatusCompleted {
		t.Fatalf("status = %s, want completed; report=%+v", result.Status, result.Report)
	}
	if result.ReadCount != 4 || result.WriteCount != 3 || result.RetryCount != 1 || result.SkipCount != 1 {
		t.Fatalf("counts = read:%d write:%d retry:%d skip:%d, want 4/3/1/1", result.ReadCount, result.WriteCount, result.RetryCount, result.SkipCount)
	}

	gotIDs := ids(result.Processed)
	if !slices.Equal(gotIDs, []string{"ticket-1001", "ticket-1002", "ticket-1004"}) {
		t.Fatalf("processed ids = %v", gotIDs)
	}
	if len(result.DeadLetters) != 1 {
		t.Fatalf("dead letters = %#v, want one", result.DeadLetters)
	}
	dead := result.DeadLetters[0]
	if dead.TicketID != "ticket-1003" || dead.Reason != "blocked address" || dead.Attempts != 1 {
		t.Fatalf("dead letter = %#v", dead)
	}

	transient := result.Processed[1]
	if transient.ID != "ticket-1002" || transient.Attempts != 2 {
		t.Fatalf("transient processed = %#v, want second attempt", transient)
	}
}

func TestPermanentTicketIsSkippedAndDeadLetteredOnce(t *testing.T) {
	store := NewDeadLetterStore()
	result, err := Run(context.Background(), Options{
		Tickets: []Ticket{
			{ID: "ticket-permanent", Channel: "email", Address: "blocked@example.com", Scenario: ScenarioPermanent, Reason: "blocked address"},
		},
		DeadLetters: store,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Status != batch.StatusCompleted || result.SkipCount != 1 || result.WriteCount != 0 {
		t.Fatalf("result = %#v, want completed skip without write", result)
	}
	if got := store.List(); len(got) != 1 || got[0].TicketID != "ticket-permanent" || got[0].Attempts != 1 {
		t.Fatalf("dead letters = %#v", got)
	}
}

func TestSkipBudgetExhaustionFailsWithPermanentError(t *testing.T) {
	result, err := Run(context.Background(), Options{
		Tickets: []Ticket{
			{ID: "ticket-a", Channel: "email", Address: "a@example.com", Scenario: ScenarioPermanent, Reason: "blocked address"},
			{ID: "ticket-b", Channel: "email", Address: "b@example.com", Scenario: ScenarioPermanent, Reason: "blocked address"},
		},
		MaxSkips: 1,
	})
	if err != nil {
		t.Fatalf("Run setup failed: %v", err)
	}
	if result.Status != batch.StatusFailed {
		t.Fatalf("status = %s, want failed; report=%+v", result.Status, result.Report)
	}
	if !strings.Contains(result.Report.Error, ErrPermanentTicket.Error()) {
		t.Fatalf("error = %q, want permanent ticket", result.Report.Error)
	}
	if result.SkipCount != 1 || len(result.DeadLetters) != 2 {
		t.Fatalf("skip/dead letters = %d/%#v, want one skip and two DLT records", result.SkipCount, result.DeadLetters)
	}
}

func TestDuplicateWriterFailureIsNotDeadLettered(t *testing.T) {
	result, err := Run(context.Background(), Options{
		Tickets: []Ticket{
			{ID: "ticket-dup", Channel: "email", Address: "first@example.com", Scenario: ScenarioSuccess},
			{ID: "ticket-dup", Channel: "email", Address: "second@example.com", Scenario: ScenarioSuccess},
		},
		ChunkSize: 1,
	})
	if err != nil {
		t.Fatalf("Run setup failed: %v", err)
	}
	if result.Status != batch.StatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
	if !strings.Contains(result.Report.Error, ErrDuplicateTicket.Error()) {
		t.Fatalf("error = %q, want duplicate writer error", result.Report.Error)
	}
	if len(result.DeadLetters) != 0 {
		t.Fatalf("dead letters = %#v, want none", result.DeadLetters)
	}
}

func TestCancellationBeforeWorkClosesNoOpenedResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reader := NewTicketReader(DefaultTickets())
	writer := NewTicketWriter(NewProcessedSink())
	step, err := batch.NewStep(batch.StepOptions[Ticket, ProcessedTicket]{
		Name:      StepName,
		ChunkSize: DefaultChunkSize,
		Reader:    reader,
		Processor: NewTicketProcessor(NewDeadLetterStore()),
		Writer:    writer,
	})
	if err != nil {
		t.Fatalf("NewStep failed: %v", err)
	}

	report := step.Run(ctx)
	if report.Status != batch.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", report.Status)
	}
	if reader.Closed() || writer.Closed() {
		t.Fatalf("reader/writer should not close because open never happened")
	}
}

func TestCancellationDuringRetryIsNotRetriedOrDeadLettered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	processor := batch.ProcessorFunc[Ticket, ProcessedTicket](func(context.Context, Ticket) (ProcessedTicket, bool, error) {
		cancel()
		return ProcessedTicket{}, false, context.Canceled
	})
	retry, err := batch.RetryErrors(3, func(err error) bool {
		return errors.Is(err, ErrTransientTicket)
	})
	if err != nil {
		t.Fatalf("RetryErrors failed: %v", err)
	}
	skip, err := batch.SkipErrors(2, func(err error) bool {
		return errors.Is(err, ErrPermanentTicket)
	})
	if err != nil {
		t.Fatalf("SkipErrors failed: %v", err)
	}
	store := NewDeadLetterStore()
	step, err := batch.NewStep(batch.StepOptions[Ticket, ProcessedTicket]{
		Name:        StepName,
		ChunkSize:   DefaultChunkSize,
		Reader:      NewTicketReader([]Ticket{{ID: "ticket-cancel", Channel: "email", Address: "cancel@example.com"}}),
		Processor:   processor,
		Writer:      NewTicketWriter(NewProcessedSink()),
		RetryPolicy: retry,
		SkipPolicy:  skip,
	})
	if err != nil {
		t.Fatalf("NewStep failed: %v", err)
	}

	report := step.Run(ctx)
	if report.Status != batch.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", report.Status)
	}
	if report.RetryCount != 0 || report.SkipCount != 0 {
		t.Fatalf("retry/skip = %d/%d, want 0/0", report.RetryCount, report.SkipCount)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("dead letters = %#v, want none", got)
	}
}

func TestReportProjectionOmitsRuntimeTimestamps(t *testing.T) {
	result, err := RunDemo(context.Background())
	if err != nil {
		t.Fatalf("RunDemo failed: %v", err)
	}
	if reflect.ValueOf(result.Report).FieldByName("StartedAt").IsValid() {
		t.Fatal("ReportNode must not expose StartedAt")
	}
	if reflect.ValueOf(result.Report).FieldByName("EndedAt").IsValid() {
		t.Fatal("ReportNode must not expose EndedAt")
	}
}

func TestConcurrentRunsRemainIsolatedAndDeterministic(t *testing.T) {
	const (
		workers    = 16
		iterations = 12
	)

	errs := make(chan string, workers*iterations)
	var wg sync.WaitGroup
	for workerID := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range iterations {
				result, err := Run(context.Background(), Options{ChunkSize: 1 + iteration%3})
				if err != nil {
					errs <- fmt.Sprintf("worker %d iteration %d: Run failed: %v", workerID, iteration, err)
					continue
				}
				if err := verifyDefaultResult(result); err != nil {
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

func TestStoresStressConcurrentWritesAndReads(t *testing.T) {
	const (
		workers    = 8
		iterations = 50
	)

	sink := NewProcessedSink()
	deadLetters := NewDeadLetterStore()
	errs := make(chan string, workers*iterations)

	var wg sync.WaitGroup
	for workerID := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range iterations {
				id := fmt.Sprintf("ticket-%02d-%03d", workerID, iteration)
				ticket := ProcessedTicket{ID: id, Channel: "email", Address: id + "@example.com", Attempts: 1}
				if err := sink.Put(ticket); err != nil {
					errs <- fmt.Sprintf("put %s: %v", id, err)
				}
				deadLetters.Record(DeadLetter{TicketID: id, Reason: "stress", Attempts: 1})

				_ = sink.List()
				_ = deadLetters.List()
			}
		}()
	}
	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
	processed := sink.List()
	dead := deadLetters.List()
	want := workers * iterations
	if len(processed) != want || len(dead) != want {
		t.Fatalf("processed/dead = %d/%d, want %d/%d", len(processed), len(dead), want, want)
	}
	if err := verifyUniqueProcessed(processed); err != nil {
		t.Fatal(err)
	}
	if err := verifyUniqueDeadLetters(dead); err != nil {
		t.Fatal(err)
	}
	if err := sink.Put(processed[0]); !errors.Is(err, ErrDuplicateTicket) {
		t.Fatalf("duplicate write error = %v, want ErrDuplicateTicket", err)
	}
}

func ids(tickets []ProcessedTicket) []string {
	values := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		values = append(values, ticket.ID)
	}
	return values
}

func verifyDefaultResult(result Result) error {
	if result.Status != batch.StatusCompleted {
		return fmt.Errorf("status = %s, want completed", result.Status)
	}
	if result.ReadCount != 4 || result.WriteCount != 3 || result.RetryCount != 1 || result.SkipCount != 1 {
		return fmt.Errorf("counts = read:%d write:%d retry:%d skip:%d, want 4/3/1/1", result.ReadCount, result.WriteCount, result.RetryCount, result.SkipCount)
	}
	if got := ids(result.Processed); !slices.Equal(got, []string{"ticket-1001", "ticket-1002", "ticket-1004"}) {
		return fmt.Errorf("processed ids = %v", got)
	}
	if len(result.DeadLetters) != 1 || result.DeadLetters[0].TicketID != "ticket-1003" {
		return fmt.Errorf("dead letters = %#v, want ticket-1003", result.DeadLetters)
	}
	if result.Processed[1].Attempts != 2 {
		return fmt.Errorf("transient attempts = %d, want 2", result.Processed[1].Attempts)
	}
	return nil
}

func verifyUniqueProcessed(tickets []ProcessedTicket) error {
	seen := make(map[string]struct{}, len(tickets))
	for _, ticket := range tickets {
		if _, exists := seen[ticket.ID]; exists {
			return fmt.Errorf("duplicate processed ticket %q", ticket.ID)
		}
		seen[ticket.ID] = struct{}{}
	}
	return nil
}

func verifyUniqueDeadLetters(records []DeadLetter) error {
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if _, exists := seen[record.TicketID]; exists {
			return fmt.Errorf("duplicate dead letter %q", record.TicketID)
		}
		seen[record.TicketID] = struct{}{}
	}
	return nil
}
