// Package customermigration implements the customer migration batch integration example.
package customermigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bluetape4k/bluetape-go/batch"
)

const (
	// BatchName is the stable batch job name used in reports.
	BatchName = "customer-migration-batch"
	// StepName is the stable step name used inside batch reports.
	StepName = "customer-migration-step"
	// CheckpointKey identifies this demo batch in the checkpoint store.
	CheckpointKey = "customer-migration"
	// DefaultChunkSize fixes the demo chunk size so restart behavior is predictable.
	DefaultChunkSize = 2
)

const (
	// TriggerManual labels operator-started runs.
	TriggerManual = "manual"
	// TriggerSchedule labels scheduler-started runs.
	TriggerSchedule = "schedule"
)

const (
	// ErrorCodeRequestCancelled is returned when caller cancellation stops work.
	ErrorCodeRequestCancelled = "request_cancelled"
	// ErrorCodeWriterCrash is returned when the demo writer crash is injected.
	ErrorCodeWriterCrash = "writer_crash"
	// ErrorCodeTransientCustomer is returned when transient retries are exhausted.
	ErrorCodeTransientCustomer = "transient_customer_exhausted"
	// ErrorCodePermanentCustomer is returned when a customer is dead-lettered.
	ErrorCodePermanentCustomer = "permanent_customer"
	// ErrorCodeInvalidCheckpoint is returned when checkpoint state is corrupt.
	ErrorCodeInvalidCheckpoint = "invalid_checkpoint"
	// ErrorCodeInvalidCrashAfter is returned when crash injection is malformed.
	ErrorCodeInvalidCrashAfter = "invalid_crash_after_new_writes"
	// ErrorCodeRunInProgress is returned when another run owns the stores.
	ErrorCodeRunInProgress = "run_in_progress"
	// ErrorCodeNotLeader is returned when the process does not hold leadership.
	ErrorCodeNotLeader = "not_leader"
	// ErrorCodeInvalidRunID is returned when a required run id is blank.
	ErrorCodeInvalidRunID = "invalid_run_id"
	// ErrorCodeInvalidRequest is returned when request JSON is malformed.
	ErrorCodeInvalidRequest = "invalid_request"
	// ErrorCodeRequestTooLarge is returned when request JSON exceeds the cap.
	ErrorCodeRequestTooLarge = "request_too_large"
	// ErrorCodeReportNotFound is returned when a run report is unavailable.
	ErrorCodeReportNotFound = "report_not_found"
	// ErrorCodeNoActiveRun is returned when cancel has no in-flight run to stop.
	ErrorCodeNoActiveRun = "no_active_run"
)

var (
	// ErrInvalidCustomer reports a malformed fixture customer.
	ErrInvalidCustomer = errors.New("invalid customer")
	// ErrTransientCustomer reports an exhausted transient fixture failure.
	ErrTransientCustomer = errors.New("transient customer")
	// ErrPermanentCustomer reports a deterministic dead-letter fixture failure.
	ErrPermanentCustomer = errors.New("permanent customer")
	// ErrWriterCrash reports the injected writer crash.
	ErrWriterCrash = errors.New("writer crash")
	// ErrDuplicateCustomer reports an unexpected duplicate sink write.
	ErrDuplicateCustomer = errors.New("duplicate customer")
	// ErrInvalidCheckpoint reports corrupt checkpoint state.
	ErrInvalidCheckpoint = errors.New("invalid checkpoint")
	// ErrRunInProgress reports concurrent mutation of the shared demo stores.
	ErrRunInProgress = errors.New("run in progress")
	// ErrNotLeader reports an operation attempted without leadership.
	ErrNotLeader = errors.New("not leader")
	// ErrInvalidRunID reports a blank or malformed run identifier.
	ErrInvalidRunID = errors.New("invalid run id")
	// ErrInvalidCrashAfter reports malformed crash injection configuration.
	ErrInvalidCrashAfter = errors.New("invalid crash_after_new_writes")
)

// FailureScenario describes deterministic per-customer behavior.
type FailureScenario string

const (
	// ScenarioSuccess migrates without injected failure.
	ScenarioSuccess = FailureScenario("success")
	// ScenarioTransientOnce fails once, then succeeds on retry.
	ScenarioTransientOnce FailureScenario = "transient_once"
	// ScenarioPermanent always moves the customer to the dead-letter store.
	ScenarioPermanent FailureScenario = "permanent"
)

// CustomerRecord is one source customer fixture.
type CustomerRecord struct {
	Index    int
	ID       string
	Email    string
	Segment  string
	Scenario FailureScenario
	Reason   string
}

// MigratedCustomer is the internal migrated-customer sink value.
type MigratedCustomer struct {
	ID       string `json:"id"`
	Email    string `json:"-"`
	Segment  string `json:"segment"`
	Attempts int    `json:"attempts"`
}

// Checkpoint stores the next source index to read.
type Checkpoint struct {
	NextIndex int `json:"next_index"`
}

// DeadLetter records a permanently skipped customer.
type DeadLetter struct {
	CustomerID string `json:"customer_id"`
	Reason     string `json:"reason"`
	Attempts   int    `json:"attempts"`
}

// Stores groups the shared in-memory batch stores.
type Stores struct {
	Checkpoints *RecordingCheckpointStore
	Sink        *CustomerSink
	DeadLetters *DeadLetterStore
}

// NewStores creates empty in-memory stores for one demo process.
func NewStores() *Stores {
	return &Stores{
		Checkpoints: NewRecordingCheckpointStore(),
		Sink:        NewCustomerSink(),
		DeadLetters: NewDeadLetterStore(),
	}
}

func (s *Stores) normalize() *Stores {
	if s == nil {
		return NewStores()
	}
	if s.Checkpoints == nil {
		s.Checkpoints = NewRecordingCheckpointStore()
	}
	if s.Sink == nil {
		s.Sink = NewCustomerSink()
	}
	if s.DeadLetters == nil {
		s.DeadLetters = NewDeadLetterStore()
	}
	return s
}

// RunOptions configures one batch run.
type RunOptions struct {
	RunID               string
	Trigger             string
	Stores              *Stores
	CrashAfterNewWrites int
	BeforeWrite         func(context.Context)
}

// RunResponse is the public run projection returned by HTTP handlers.
type RunResponse struct {
	RunID         string       `json:"run_id"`
	Trigger       string       `json:"trigger"`
	Status        batch.Status `json:"status"`
	Checkpoint    Checkpoint   `json:"checkpoint"`
	ReadIDs       []string     `json:"read_ids,omitempty"`
	AcceptedIDs   []string     `json:"accepted_ids,omitempty"`
	NewWrittenIDs []string     `json:"new_written_ids,omitempty"`
	MigratedIDs   []string     `json:"migrated_ids,omitempty"`
	DeadLetters   []DeadLetter `json:"dead_letters,omitempty"`
	Summary       Summary      `json:"summary"`
	Report        ReportNode   `json:"report"`
	ErrorCode     string       `json:"error_code,omitempty"`
	ErrorMessage  string       `json:"error_message,omitempty"`
	FailedPhase   string       `json:"failed_phase,omitempty"`
	Diagnostics   []string     `json:"diagnostics,omitempty"`
}

func (r RunResponse) String() string {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("%#v", r)
	}
	return string(data)
}

// Summary captures stable run counters.
type Summary struct {
	ReadCount          int  `json:"read_count"`
	WriteCount         int  `json:"write_count"`
	FilterCount        int  `json:"filter_count,omitempty"`
	RetryCount         int  `json:"retry_count"`
	SkipCount          int  `json:"skip_count"`
	DuplicateSkipCount int  `json:"duplicate_skip_count"`
	ChunkSize          int  `json:"chunk_size"`
	Failure            bool `json:"failure"`
}

// ReportNode is a timestamp-free batch report projection.
type ReportNode struct {
	Name        string       `json:"name"`
	Status      batch.Status `json:"status"`
	ReadCount   int          `json:"read_count,omitempty"`
	WriteCount  int          `json:"write_count,omitempty"`
	FilterCount int          `json:"filter_count,omitempty"`
	SkipCount   int          `json:"skip_count,omitempty"`
	RetryCount  int          `json:"retry_count,omitempty"`
	ErrorCode   string       `json:"error_code,omitempty"`
	Children    []ReportNode `json:"children,omitempty"`
	StartedAt   string       `json:"-"`
	EndedAt     string       `json:"-"`
}

// RunBatch executes one customer migration batch run.
func RunBatch(ctx context.Context, options RunOptions) (RunResponse, error) {
	ctx = normalizeContext(ctx)
	stores := options.Stores.normalize()
	if options.Trigger == "" {
		options.Trigger = TriggerManual
	}
	if err := validateCrashAfter(options.CrashAfterNewWrites); err != nil {
		response := baseResponse(options, stores, batch.StatusFailed, nil)
		response.ErrorCode = ErrorCodeInvalidCrashAfter
		response.ErrorMessage = publicMessage(response.ErrorCode)
		response.FailedPhase = "validation"
		return response, err
	}

	rollbackCheckpoint, hasRollback := stores.Checkpoints.Snapshot(CheckpointKey)
	reader := &customerReader{records: defaultRecords()}
	processor := &customerProcessor{deadLetters: stores.DeadLetters, attempts: make(map[string]int)}
	writer := &customerWriter{
		sink:                stores.Sink,
		crashAfterNewWrites: options.CrashAfterNewWrites,
		beforeWrite:         options.BeforeWrite,
	}
	retry, err := batch.RetryErrors(3, func(err error) bool {
		return errors.Is(err, ErrTransientCustomer)
	})
	if err != nil {
		return RunResponse{}, err
	}
	skip, err := batch.SkipErrors(2, func(err error) bool {
		return errors.Is(err, ErrPermanentCustomer)
	})
	if err != nil {
		return RunResponse{}, err
	}
	step, err := batch.NewStep(batch.StepOptions[CustomerRecord, MigratedCustomer]{
		Name:            StepName,
		ChunkSize:       DefaultChunkSize,
		Reader:          reader,
		Processor:       processor,
		Writer:          writer,
		RetryPolicy:     retry,
		SkipPolicy:      skip,
		CheckpointStore: stores.Checkpoints,
		CheckpointKey:   CheckpointKey,
	})
	if err != nil {
		return RunResponse{}, err
	}
	job, err := batch.NewJob(BatchName, step)
	if err != nil {
		return RunResponse{}, err
	}

	report := job.Run(ctx)
	if errors.Is(report.Err, ErrWriterCrash) {
		if hasRollback {
			_ = stores.Checkpoints.Save(context.WithoutCancel(ctx), CheckpointKey, rollbackCheckpoint)
		} else {
			_ = stores.Checkpoints.Save(context.WithoutCancel(ctx), CheckpointKey, Checkpoint{NextIndex: DefaultChunkSize})
		}
	}
	response := buildRunResponse(options, stores, reader.readIDs, writer, report)
	return response, report.Err
}

func validateCrashAfter(value int) error {
	if value < 0 || value > len(defaultRecords()) {
		return fmt.Errorf("%w: %d", ErrInvalidCrashAfter, value)
	}
	return nil
}

func baseResponse(options RunOptions, stores *Stores, status batch.Status, report *batch.Report) RunResponse {
	checkpoint, _ := stores.Checkpoints.Snapshot(CheckpointKey)
	response := RunResponse{
		RunID:       options.RunID,
		Trigger:     options.Trigger,
		Status:      status,
		Checkpoint:  checkpoint,
		MigratedIDs: stores.Sink.IDs(),
		DeadLetters: stores.DeadLetters.Snapshot(),
		Summary: Summary{
			ChunkSize: DefaultChunkSize,
			Failure:   status != batch.StatusCompleted,
		},
	}
	if report != nil {
		response.Report = projectReport(*report)
	}
	return response
}

func buildRunResponse(options RunOptions, stores *Stores, readIDs []string, writer *customerWriter, report batch.Report) RunResponse {
	response := baseResponse(options, stores, report.Status, &report)
	response.ReadIDs = copyStrings(readIDs)
	response.AcceptedIDs = writer.AcceptedIDs()
	response.NewWrittenIDs = writer.NewWrittenIDs()
	response.Summary = Summary{
		ReadCount:          report.ReadCount,
		WriteCount:         report.WriteCount,
		FilterCount:        report.FilterCount,
		RetryCount:         report.RetryCount,
		SkipCount:          report.SkipCount,
		DuplicateSkipCount: writer.DuplicateSkipCount(),
		ChunkSize:          DefaultChunkSize,
		Failure:            report.IsFailure(),
	}
	if report.Err != nil {
		response.ErrorCode = errorCodeFor(report.Err)
		response.ErrorMessage = publicMessage(response.ErrorCode)
		response.FailedPhase = failedPhaseFor(report.Err)
	}
	if errors.Is(report.Err, ErrWriterCrash) {
		checkpoint, _ := stores.Checkpoints.Snapshot(CheckpointKey)
		response.Checkpoint = checkpoint
	}
	return response
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
		ErrorCode:   errorCodeFor(report.Err),
	}
	if len(report.Children) > 0 {
		node.Children = make([]ReportNode, len(report.Children))
		for i, child := range report.Children {
			node.Children[i] = projectReport(child)
		}
	}
	return node
}

func errorCodeFor(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ErrorCodeRequestCancelled
	case errors.Is(err, ErrWriterCrash):
		return ErrorCodeWriterCrash
	case errors.Is(err, ErrTransientCustomer):
		return ErrorCodeTransientCustomer
	case errors.Is(err, ErrPermanentCustomer):
		return ErrorCodePermanentCustomer
	case errors.Is(err, ErrInvalidCheckpoint):
		return ErrorCodeInvalidCheckpoint
	case errors.Is(err, ErrRunInProgress):
		return ErrorCodeRunInProgress
	case errors.Is(err, ErrNotLeader):
		return ErrorCodeNotLeader
	case errors.Is(err, ErrInvalidRunID):
		return ErrorCodeInvalidRunID
	case errors.Is(err, ErrInvalidCrashAfter):
		return ErrorCodeInvalidCrashAfter
	default:
		return ErrorCodeInvalidRequest
	}
}

func publicMessage(code string) string {
	switch code {
	case ErrorCodeRequestCancelled:
		return "request was cancelled"
	case ErrorCodeWriterCrash:
		return "simulated writer crash"
	case ErrorCodeTransientCustomer:
		return "transient customer retries exhausted"
	case ErrorCodePermanentCustomer:
		return "permanent customer skip limit exceeded"
	case ErrorCodeInvalidCheckpoint:
		return "invalid checkpoint"
	case ErrorCodeRunInProgress:
		return "batch run already active"
	case ErrorCodeNotLeader:
		return "leadership is not held"
	case ErrorCodeInvalidRunID:
		return "invalid run id"
	case ErrorCodeInvalidCrashAfter:
		return "invalid crash_after_new_writes"
	case ErrorCodeRequestTooLarge:
		return "request body too large"
	case ErrorCodeReportNotFound:
		return "report not found"
	case ErrorCodeNoActiveRun:
		return "no active run"
	default:
		return "invalid request"
	}
}

func failedPhaseFor(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrWriterCrash):
		return "writer"
	case errors.Is(err, ErrTransientCustomer), errors.Is(err, ErrPermanentCustomer):
		return "processor"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "context"
	default:
		return "batch"
	}
}

// RecordingCheckpointStore stores checkpoints behind a lock.
type RecordingCheckpointStore struct {
	mu     sync.RWMutex
	values map[string]Checkpoint
}

// NewRecordingCheckpointStore creates an empty checkpoint store.
func NewRecordingCheckpointStore() *RecordingCheckpointStore {
	return &RecordingCheckpointStore{values: make(map[string]Checkpoint)}
}

// Load returns the checkpoint for key when present.
func (s *RecordingCheckpointStore) Load(ctx context.Context, key string) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if s == nil {
		return nil, false, fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	checkpoint, ok := s.values[key]
	return checkpoint, ok, nil
}

// Save stores a checkpoint for key.
func (s *RecordingCheckpointStore) Save(ctx context.Context, key string, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("checkpoint store must not be nil")
	}
	value, ok := checkpoint.(Checkpoint)
	if !ok {
		return fmt.Errorf("%w: %T", ErrInvalidCheckpoint, checkpoint)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

// Snapshot returns a typed defensive checkpoint snapshot.
func (s *RecordingCheckpointStore) Snapshot(key string) (Checkpoint, bool) {
	if s == nil {
		return Checkpoint{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	checkpoint, ok := s.values[key]
	return checkpoint, ok
}

// CustomerSink stores migrated customers idempotently.
type CustomerSink struct {
	mu       sync.RWMutex
	byID     map[string]MigratedCustomer
	order    []string
	newCount int
}

// NewCustomerSink creates an empty migrated customer sink.
func NewCustomerSink() *CustomerSink {
	return &CustomerSink{byID: make(map[string]MigratedCustomer)}
}

// Put stores customer if the ID has not already been migrated.
func (s *CustomerSink) Put(customer MigratedCustomer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[customer.ID]; exists {
		return false
	}
	s.byID[customer.ID] = customer
	s.order = append(s.order, customer.ID)
	s.newCount++
	return true
}

// IDs returns migrated customer IDs in insertion order.
func (s *CustomerSink) IDs() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyStrings(s.order)
}

// NewCount returns the number of newly inserted customers.
func (s *CustomerSink) NewCount() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.newCount
}

// DeadLetterStore stores unique dead letters by customer ID.
type DeadLetterStore struct {
	mu    sync.RWMutex
	byID  map[string]DeadLetter
	order []string
}

// NewDeadLetterStore creates an empty dead-letter store.
func NewDeadLetterStore() *DeadLetterStore {
	return &DeadLetterStore{byID: make(map[string]DeadLetter)}
}

// Record stores one dead letter if it has not already been recorded.
func (s *DeadLetterStore) Record(letter DeadLetter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[letter.CustomerID]; exists {
		return
	}
	s.byID[letter.CustomerID] = letter
	s.order = append(s.order, letter.CustomerID)
}

// Snapshot returns dead letters in insertion order.
func (s *DeadLetterStore) Snapshot() []DeadLetter {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	letters := make([]DeadLetter, 0, len(s.order))
	for _, id := range s.order {
		letters = append(letters, s.byID[id])
	}
	return letters
}

type customerReader struct {
	records []CustomerRecord
	index   int
	readIDs []string
}

func (r *customerReader) Open(ctx context.Context) error {
	return ctx.Err()
}

func (r *customerReader) Read(ctx context.Context) (CustomerRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return CustomerRecord{}, false, err
	}
	if r.index >= len(r.records) {
		return CustomerRecord{}, false, nil
	}
	record := r.records[r.index]
	r.index++
	r.readIDs = append(r.readIDs, record.ID)
	return record, true, nil
}

func (r *customerReader) Close(ctx context.Context) error {
	return ctx.Err()
}

func (r *customerReader) Restore(ctx context.Context, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	value, ok := checkpoint.(Checkpoint)
	if !ok {
		return fmt.Errorf("%w: %T", ErrInvalidCheckpoint, checkpoint)
	}
	if value.NextIndex < 0 || value.NextIndex > len(r.records) {
		return fmt.Errorf("%w: next_index=%d", ErrInvalidCheckpoint, value.NextIndex)
	}
	r.index = value.NextIndex
	return nil
}

func (r *customerReader) Checkpoint(ctx context.Context) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return Checkpoint{NextIndex: r.index}, true, nil
}

type customerProcessor struct {
	deadLetters *DeadLetterStore
	attempts    map[string]int
}

func (p *customerProcessor) Process(ctx context.Context, record CustomerRecord) (MigratedCustomer, bool, error) {
	if err := ctx.Err(); err != nil {
		return MigratedCustomer{}, false, err
	}
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Email) == "" {
		return MigratedCustomer{}, false, fmt.Errorf("%w: missing required customer fields", ErrInvalidCustomer)
	}
	p.attempts[record.ID]++
	attempt := p.attempts[record.ID]
	switch record.Scenario {
	case ScenarioTransientOnce:
		if attempt == 1 {
			return MigratedCustomer{}, false, fmt.Errorf("%w: enrichment unavailable", ErrTransientCustomer)
		}
	case ScenarioPermanent:
		p.deadLetters.Record(DeadLetter{CustomerID: record.ID, Reason: record.Reason, Attempts: attempt})
		return MigratedCustomer{}, false, fmt.Errorf("%w: %s", ErrPermanentCustomer, record.Reason)
	}
	return MigratedCustomer{
		ID:       record.ID,
		Email:    strings.ToLower(record.Email),
		Segment:  strings.ToLower(record.Segment),
		Attempts: attempt,
	}, true, nil
}

type customerWriter struct {
	sink                *CustomerSink
	crashAfterNewWrites int
	beforeWrite         func(context.Context)

	mu                 sync.Mutex
	acceptedIDs        []string
	newWrittenIDs      []string
	duplicateSkipCount int
}

func (w *customerWriter) Open(ctx context.Context) error {
	return ctx.Err()
}

func (w *customerWriter) Write(ctx context.Context, customers []MigratedCustomer) error {
	if w.beforeWrite != nil {
		w.beforeWrite(ctx)
	}
	for _, customer := range customers {
		if err := ctx.Err(); err != nil {
			return err
		}
		isNew := w.sink.Put(customer)
		w.mu.Lock()
		w.acceptedIDs = append(w.acceptedIDs, customer.ID)
		if isNew {
			w.newWrittenIDs = append(w.newWrittenIDs, customer.ID)
		} else {
			w.duplicateSkipCount++
		}
		w.mu.Unlock()
		if isNew && w.crashAfterNewWrites > 0 && w.sink.NewCount() >= w.crashAfterNewWrites {
			return fmt.Errorf("%w: after %d new writes", ErrWriterCrash, w.crashAfterNewWrites)
		}
	}
	return nil
}

func (w *customerWriter) Close(ctx context.Context) error {
	return ctx.Err()
}

func (w *customerWriter) AcceptedIDs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return copyStrings(w.acceptedIDs)
}

func (w *customerWriter) NewWrittenIDs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return copyStrings(w.newWrittenIDs)
}

func (w *customerWriter) DuplicateSkipCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.duplicateSkipCount
}

func defaultRecords() []CustomerRecord {
	records := []CustomerRecord{
		{Index: 0, ID: "cust-1001", Email: "cust-1001@example.test", Segment: "Retail", Scenario: ScenarioSuccess},
		{Index: 1, ID: "cust-1002", Email: "cust-1002@example.test", Segment: "Enterprise", Scenario: ScenarioSuccess},
		{Index: 2, ID: "cust-1003", Email: "cust-1003@example.test", Segment: "Retail", Scenario: ScenarioTransientOnce},
		{Index: 3, ID: "cust-1004", Email: "cust-1004@example.test", Segment: "Blocked", Scenario: ScenarioPermanent, Reason: "blocked customer"},
		{Index: 4, ID: "cust-1005", Email: "cust-1005@example.test", Segment: "Retail", Scenario: ScenarioSuccess},
	}
	return append([]CustomerRecord(nil), records...)
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
