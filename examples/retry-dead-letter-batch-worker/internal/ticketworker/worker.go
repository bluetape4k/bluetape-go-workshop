// Package ticketworker demonstrates batch retry and dead-letter handling.
package ticketworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/bluetape4k/bluetape-go/batch"
)

const (
	// JobName is the batch job name shown in the demo report.
	JobName = "ticket-batch-worker"
	// StepName is the batch step name shown in the demo report.
	StepName = "ticket-retry-dead-letter-worker"
	// DefaultChunkSize keeps the workshop output small enough to inspect.
	DefaultChunkSize = 2
)

var (
	// ErrTransientTicket reports a ticket failure that can be retried safely.
	ErrTransientTicket = errors.New("transient ticket failure")
	// ErrPermanentTicket reports a ticket failure that should be dead-lettered.
	ErrPermanentTicket = errors.New("permanent ticket failure")
	// ErrDuplicateTicket reports a duplicate processed ticket write.
	ErrDuplicateTicket = errors.New("duplicate processed ticket")
)

// FailureScenario controls deterministic processor behavior for the example.
type FailureScenario string

const (
	// ScenarioSuccess processes a ticket without an injected failure.
	ScenarioSuccess FailureScenario = "success"
	// ScenarioTransientOnce fails the first processor attempt and succeeds on retry.
	ScenarioTransientOnce FailureScenario = "transient_once"
	// ScenarioPermanent records a dead-letter entry and returns a skippable error.
	ScenarioPermanent FailureScenario = "permanent"
)

// Ticket is one queued support ticket.
type Ticket struct {
	ID       string
	Channel  string
	Address  string
	Scenario FailureScenario
	Reason   string
}

// ProcessedTicket is the successful output written by the batch writer.
type ProcessedTicket struct {
	ID       string `json:"id"`
	Channel  string `json:"channel"`
	Address  string `json:"address"`
	Attempts int    `json:"attempts"`
}

// DeadLetter preserves a skipped permanent item and the reason it was skipped.
type DeadLetter struct {
	TicketID string `json:"ticket_id"`
	Reason   string `json:"reason"`
	Attempts int    `json:"attempts"`
}

// Options configures the local batch worker demo.
type Options struct {
	Tickets     []Ticket
	ChunkSize   int
	MaxAttempts int
	MaxSkips    int
	Sink        *ProcessedSink
	DeadLetters *DeadLetterStore
}

// Result is the deterministic JSON projection printed by the demo.
type Result struct {
	JobName     string            `json:"job_name"`
	StepName    string            `json:"step_name"`
	ChunkSize   int               `json:"chunk_size"`
	Status      batch.Status      `json:"status"`
	ReadCount   int               `json:"read_count"`
	WriteCount  int               `json:"write_count"`
	RetryCount  int               `json:"retry_count"`
	SkipCount   int               `json:"skip_count"`
	Processed   []ProcessedTicket `json:"processed"`
	DeadLetters []DeadLetter      `json:"dead_letters"`
	Report      ReportNode        `json:"report"`
}

// ReportNode is a timestamp-free batch report projection.
type ReportNode struct {
	Name        string       `json:"name"`
	Status      batch.Status `json:"status"`
	Error       string       `json:"error,omitempty"`
	ReadCount   int          `json:"read_count,omitempty"`
	WriteCount  int          `json:"write_count,omitempty"`
	FilterCount int          `json:"filter_count,omitempty"`
	SkipCount   int          `json:"skip_count,omitempty"`
	RetryCount  int          `json:"retry_count,omitempty"`
	Failure     bool         `json:"failure"`
	Children    []ReportNode `json:"children,omitempty"`
}

// DefaultTickets returns the deterministic support-ticket fixture.
func DefaultTickets() []Ticket {
	return []Ticket{
		{ID: "ticket-1001", Channel: "email", Address: "vip@example.com", Scenario: ScenarioSuccess},
		{ID: "ticket-1002", Channel: "sms", Address: "+15550100", Scenario: ScenarioTransientOnce},
		{ID: "ticket-1003", Channel: "email", Address: "blocked@example.com", Scenario: ScenarioPermanent, Reason: "blocked address"},
		{ID: "ticket-1004", Channel: "push", Address: "device-42", Scenario: ScenarioSuccess},
	}
}

// RunDemo executes the default retry/dead-letter batch worker scenario.
func RunDemo(ctx context.Context) (Result, error) {
	return Run(ctx, Options{})
}

// Run executes one configured ticket worker batch job.
func Run(ctx context.Context, options Options) (Result, error) {
	ctx = normalizeContext(ctx)
	if len(options.Tickets) == 0 {
		options.Tickets = DefaultTickets()
	}
	if options.ChunkSize == 0 {
		options.ChunkSize = DefaultChunkSize
	}
	if options.MaxAttempts == 0 {
		options.MaxAttempts = 3
	}
	if options.MaxSkips == 0 {
		options.MaxSkips = 2
	}
	if options.Sink == nil {
		options.Sink = NewProcessedSink()
	}
	if options.DeadLetters == nil {
		options.DeadLetters = NewDeadLetterStore()
	}

	retry, err := batch.RetryErrors(options.MaxAttempts, func(err error) bool {
		return errors.Is(err, ErrTransientTicket)
	})
	if err != nil {
		return Result{}, err
	}
	skip, err := batch.SkipErrors(options.MaxSkips, func(err error) bool {
		return errors.Is(err, ErrPermanentTicket)
	})
	if err != nil {
		return Result{}, err
	}

	reader := NewTicketReader(options.Tickets)
	processor := NewTicketProcessor(options.DeadLetters)
	writer := NewTicketWriter(options.Sink)

	step, err := batch.NewStep(batch.StepOptions[Ticket, ProcessedTicket]{
		Name:        StepName,
		ChunkSize:   options.ChunkSize,
		Reader:      reader,
		Processor:   processor,
		Writer:      writer,
		RetryPolicy: retry,
		SkipPolicy:  skip,
	})
	if err != nil {
		return Result{}, err
	}
	job, err := batch.NewJob(JobName, step)
	if err != nil {
		return Result{}, err
	}

	report := job.Run(ctx)
	return Result{
		JobName:     JobName,
		StepName:    StepName,
		ChunkSize:   options.ChunkSize,
		Status:      report.Status,
		ReadCount:   report.ReadCount,
		WriteCount:  report.WriteCount,
		RetryCount:  report.RetryCount,
		SkipCount:   report.SkipCount,
		Processed:   options.Sink.List(),
		DeadLetters: options.DeadLetters.List(),
		Report:      projectReport(report),
	}, nil
}

// MarshalResult returns deterministic indented JSON for the demo CLI.
func MarshalResult(result Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// TicketReader reads tickets from an in-memory queue.
type TicketReader struct {
	tickets []Ticket
	next    int
	opened  bool
	closed  bool
}

// NewTicketReader creates a reader over a defensive copy of tickets.
func NewTicketReader(tickets []Ticket) *TicketReader {
	return &TicketReader{tickets: append([]Ticket(nil), tickets...)}
}

// Open validates the deterministic queue fixture.
func (r *TicketReader) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("ticket reader must not be nil")
	}
	for index, ticket := range r.tickets {
		if ticket.ID == "" {
			return fmt.Errorf("%w: ticket at index %d has blank id", ErrPermanentTicket, index)
		}
	}
	r.next = 0
	r.opened = true
	r.closed = false
	return nil
}

// Read returns the next ticket.
func (r *TicketReader) Read(ctx context.Context) (Ticket, bool, error) {
	var zero Ticket
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	if r == nil || !r.opened {
		return zero, false, fmt.Errorf("ticket reader is not open")
	}
	if r.next >= len(r.tickets) {
		return zero, false, nil
	}
	ticket := r.tickets[r.next]
	r.next++
	return ticket, true, nil
}

// Close records reader cleanup.
func (r *TicketReader) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.closed = true
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// Closed reports whether Close was called.
func (r *TicketReader) Closed() bool {
	return r != nil && r.closed
}

// TicketProcessor classifies ticket failures and creates dead-letter records.
type TicketProcessor struct {
	mu          sync.Mutex
	attempts    map[string]int
	deadLetters *DeadLetterStore
}

// NewTicketProcessor creates a processor that writes permanent failures to store.
func NewTicketProcessor(store *DeadLetterStore) *TicketProcessor {
	if store == nil {
		store = NewDeadLetterStore()
	}
	return &TicketProcessor{attempts: make(map[string]int), deadLetters: store}
}

// Process turns a valid ticket into a processed ticket or a classified error.
func (p *TicketProcessor) Process(ctx context.Context, ticket Ticket) (ProcessedTicket, bool, error) {
	var zero ProcessedTicket
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	if p == nil {
		return zero, false, fmt.Errorf("ticket processor must not be nil")
	}

	attempt := p.nextAttempt(ticket.ID)
	if ticket.Channel == "" || ticket.Address == "" {
		p.deadLetters.Record(DeadLetter{
			TicketID: ticket.ID,
			Reason:   "missing delivery target",
			Attempts: attempt,
		})
		return zero, false, fmt.Errorf("%w: ticket %s missing delivery target", ErrPermanentTicket, ticket.ID)
	}

	switch ticket.Scenario {
	case ScenarioSuccess, "":
		return processedTicket(ticket, attempt), true, nil
	case ScenarioTransientOnce:
		if attempt == 1 {
			return zero, false, fmt.Errorf("%w: ticket %s provider throttled", ErrTransientTicket, ticket.ID)
		}
		return processedTicket(ticket, attempt), true, nil
	case ScenarioPermanent:
		reason := ticket.Reason
		if reason == "" {
			reason = "permanent ticket failure"
		}
		p.deadLetters.Record(DeadLetter{
			TicketID: ticket.ID,
			Reason:   reason,
			Attempts: attempt,
		})
		return zero, false, fmt.Errorf("%w: ticket %s: %s", ErrPermanentTicket, ticket.ID, reason)
	default:
		p.deadLetters.Record(DeadLetter{
			TicketID: ticket.ID,
			Reason:   "unknown failure scenario",
			Attempts: attempt,
		})
		return zero, false, fmt.Errorf("%w: ticket %s unknown scenario %q", ErrPermanentTicket, ticket.ID, ticket.Scenario)
	}
}

func (p *TicketProcessor) nextAttempt(ticketID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts[ticketID]++
	return p.attempts[ticketID]
}

func processedTicket(ticket Ticket, attempt int) ProcessedTicket {
	return ProcessedTicket{
		ID:       ticket.ID,
		Channel:  ticket.Channel,
		Address:  ticket.Address,
		Attempts: attempt,
	}
}

// TicketWriter persists processed tickets into an in-memory sink.
type TicketWriter struct {
	sink   *ProcessedSink
	opened bool
	closed bool
}

// NewTicketWriter creates a writer for sink.
func NewTicketWriter(sink *ProcessedSink) *TicketWriter {
	if sink == nil {
		sink = NewProcessedSink()
	}
	return &TicketWriter{sink: sink}
}

// Open records writer startup.
func (w *TicketWriter) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("ticket writer must not be nil")
	}
	w.opened = true
	w.closed = false
	return nil
}

// Write persists one chunk.
func (w *TicketWriter) Write(ctx context.Context, tickets []ProcessedTicket) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil || !w.opened {
		return fmt.Errorf("ticket writer is not open")
	}
	for _, ticket := range tickets {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.sink.Put(ticket); err != nil {
			return err
		}
	}
	return nil
}

// Close records writer cleanup.
func (w *TicketWriter) Close(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.closed = true
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// Closed reports whether Close was called.
func (w *TicketWriter) Closed() bool {
	return w != nil && w.closed
}

// ProcessedSink stores successful processed-ticket writes.
type ProcessedSink struct {
	mu      sync.RWMutex
	tickets map[string]ProcessedTicket
	order   []string
}

// NewProcessedSink creates an empty processed-ticket sink.
func NewProcessedSink() *ProcessedSink {
	return &ProcessedSink{tickets: make(map[string]ProcessedTicket)}
}

// Put stores ticket and rejects duplicate IDs.
func (s *ProcessedSink) Put(ticket ProcessedTicket) error {
	if s == nil {
		return fmt.Errorf("processed sink must not be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tickets[ticket.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateTicket, ticket.ID)
	}
	s.tickets[ticket.ID] = ticket
	s.order = append(s.order, ticket.ID)
	return nil
}

// List returns processed tickets in write order.
func (s *ProcessedSink) List() []ProcessedTicket {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProcessedTicket, 0, len(s.order))
	for _, id := range s.order {
		result = append(result, s.tickets[id])
	}
	return result
}

// DeadLetterStore stores permanent failures in deterministic order.
type DeadLetterStore struct {
	mu      sync.RWMutex
	records []DeadLetter
	seen    map[string]struct{}
}

// NewDeadLetterStore creates an empty dead-letter store.
func NewDeadLetterStore() *DeadLetterStore {
	return &DeadLetterStore{seen: make(map[string]struct{})}
}

// Record stores the first dead-letter record for a ticket.
func (s *DeadLetterStore) Record(record DeadLetter) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.seen[record.TicketID]; exists {
		return
	}
	s.seen[record.TicketID] = struct{}{}
	s.records = append(s.records, record)
}

// List returns dead letters in insertion order.
func (s *DeadLetterStore) List() []DeadLetter {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.records)
}

func projectReport(report batch.Report) ReportNode {
	node := ReportNode{
		Name:        report.Name,
		Status:      report.Status,
		ReadCount:   report.ReadCount,
		WriteCount:  report.WriteCount,
		FilterCount: report.FilterCount,
		SkipCount:   report.SkipCount,
		RetryCount:  report.RetryCount,
		Failure:     report.IsFailure(),
	}
	if report.Err != nil {
		node.Error = report.Err.Error()
	}
	if len(report.Children) > 0 {
		node.Children = make([]ReportNode, 0, len(report.Children))
		for _, child := range report.Children {
			node.Children = append(node.Children, projectReport(child))
		}
	}
	return node
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
