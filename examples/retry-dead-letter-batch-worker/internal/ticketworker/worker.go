// Package ticketworker 는 배치 재시도와 dead-letter 처리를 보여준다.
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
	// JobName 은 데모 리포트에 표시되는 배치 작업 이름이다.
	JobName = "ticket-batch-worker"
	// StepName 은 데모 리포트에 표시되는 배치 단계 이름이다.
	StepName = "ticket-retry-dead-letter-worker"
	// DefaultChunkSize 는 워크숍 출력을 검토하기에 충분히 작게 유지한다.
	DefaultChunkSize = 2
)

var (
	// ErrTransientTicket 은 안전하게 재시도할 수 있는 티켓 실패를 나타낸다.
	ErrTransientTicket = errors.New("transient ticket failure")
	// ErrPermanentTicket 은 dead-letter 로 보내야 하는 티켓 실패를 나타낸다.
	ErrPermanentTicket = errors.New("permanent ticket failure")
	// ErrDuplicateTicket 은 처리 완료 티켓 중복 쓰기를 나타낸다.
	ErrDuplicateTicket = errors.New("duplicate processed ticket")
)

// FailureScenario 는 예제의 결정적 processor 동작을 제어한다.
type FailureScenario string

const (
	// ScenarioSuccess 는 주입된 실패 없이 티켓을 처리한다.
	ScenarioSuccess FailureScenario = "success"
	// ScenarioTransientOnce 는 첫 processor 시도를 실패시키고 재시도에서 성공한다.
	ScenarioTransientOnce FailureScenario = "transient_once"
	// ScenarioPermanent 는 dead-letter 항목을 기록하고 건너뛸 수 있는 오류를 반환한다.
	ScenarioPermanent FailureScenario = "permanent"
)

// Ticket 은 큐에 들어간 지원 티켓 하나다.
type Ticket struct {
	ID       string
	Channel  string
	Address  string
	Scenario FailureScenario
	Reason   string
}

// ProcessedTicket 은 배치 writer 가 기록한 성공 출력이다.
type ProcessedTicket struct {
	ID       string `json:"id"`
	Channel  string `json:"channel"`
	Address  string `json:"address"`
	Attempts int    `json:"attempts"`
}

// DeadLetter 는 건너뛴 영구 실패 항목과 그 이유를 보존한다.
type DeadLetter struct {
	TicketID string `json:"ticket_id"`
	Reason   string `json:"reason"`
	Attempts int    `json:"attempts"`
}

// Options 는 로컬 배치 worker 데모를 설정한다.
type Options struct {
	Tickets     []Ticket
	ChunkSize   int
	MaxAttempts int
	MaxSkips    int
	Sink        *ProcessedSink
	DeadLetters *DeadLetterStore
}

// Result 는 데모가 출력하는 결정적 JSON 표현이다.
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

// ReportNode 는 타임스탬프 없는 배치 리포트 표현이다.
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

// DefaultTickets 는 결정적인 지원 티켓 fixture를 반환한다.
func DefaultTickets() []Ticket {
	return []Ticket{
		{ID: "ticket-1001", Channel: "email", Address: "vip@example.com", Scenario: ScenarioSuccess},
		{ID: "ticket-1002", Channel: "sms", Address: "+15550100", Scenario: ScenarioTransientOnce},
		{ID: "ticket-1003", Channel: "email", Address: "blocked@example.com", Scenario: ScenarioPermanent, Reason: "blocked address"},
		{ID: "ticket-1004", Channel: "push", Address: "device-42", Scenario: ScenarioSuccess},
	}
}

// RunDemo 는 기본 재시도/dead-letter 배치 worker 시나리오를 실행한다.
func RunDemo(ctx context.Context) (Result, error) {
	return Run(ctx, Options{})
}

// Run 은 설정된 티켓 worker 배치 작업 하나를 실행한다.
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

// MarshalResult 는 데모 CLI를 위한 결정적 들여쓰기 JSON을 반환한다.
func MarshalResult(result Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// TicketReader 는 인메모리 큐에서 티켓을 읽는다.
type TicketReader struct {
	tickets []Ticket
	next    int
	opened  bool
	closed  bool
}

// NewTicketReader 는 티켓의 방어적 복사본 위에서 동작하는 reader를 생성한다.
func NewTicketReader(tickets []Ticket) *TicketReader {
	return &TicketReader{tickets: append([]Ticket(nil), tickets...)}
}

// Open 은 결정적 큐 fixture를 검증한다.
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

// Read 는 다음 티켓을 반환한다.
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

// Close 는 reader 정리를 기록한다.
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

// Closed 는 Close 호출 여부를 보고한다.
func (r *TicketReader) Closed() bool {
	return r != nil && r.closed
}

// TicketProcessor 는 티켓 실패를 분류하고 dead-letter 레코드를 생성한다.
type TicketProcessor struct {
	mu          sync.Mutex
	attempts    map[string]int
	deadLetters *DeadLetterStore
}

// NewTicketProcessor 는 영구 실패를 저장소에 쓰는 processor를 생성한다.
func NewTicketProcessor(store *DeadLetterStore) *TicketProcessor {
	if store == nil {
		store = NewDeadLetterStore()
	}
	return &TicketProcessor{attempts: make(map[string]int), deadLetters: store}
}

// Process 는 유효한 티켓을 처리 완료 티켓 또는 분류된 오류로 변환한다.
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

// TicketWriter 는 처리 완료 티켓을 인메모리 sink에 저장한다.
type TicketWriter struct {
	sink   *ProcessedSink
	opened bool
	closed bool
}

// NewTicketWriter 는 sink용 writer를 생성한다.
func NewTicketWriter(sink *ProcessedSink) *TicketWriter {
	if sink == nil {
		sink = NewProcessedSink()
	}
	return &TicketWriter{sink: sink}
}

// Open 은 writer 시작을 기록한다.
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

// Write 는 chunk 하나를 저장한다.
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

// Close 는 writer 정리를 기록한다.
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

// Closed 는 Close 호출 여부를 보고한다.
func (w *TicketWriter) Closed() bool {
	return w != nil && w.closed
}

// ProcessedSink 는 성공한 처리 완료 티켓 쓰기를 저장한다.
type ProcessedSink struct {
	mu      sync.RWMutex
	tickets map[string]ProcessedTicket
	order   []string
}

// NewProcessedSink 는 빈 처리 완료 티켓 sink를 생성한다.
func NewProcessedSink() *ProcessedSink {
	return &ProcessedSink{tickets: make(map[string]ProcessedTicket)}
}

// Put 은 티켓을 저장하고 중복 ID를 거부한다.
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

// List 는 처리 완료 티켓을 쓰기 순서대로 반환한다.
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

// DeadLetterStore 는 영구 실패를 결정적 순서로 저장한다.
type DeadLetterStore struct {
	mu      sync.RWMutex
	records []DeadLetter
	seen    map[string]struct{}
}

// NewDeadLetterStore 는 빈 dead-letter 저장소를 생성한다.
func NewDeadLetterStore() *DeadLetterStore {
	return &DeadLetterStore{seen: make(map[string]struct{})}
}

// Record 는 티켓의 첫 dead-letter 레코드를 저장한다.
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

// List 는 dead letter 를 삽입 순서대로 반환한다.
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
