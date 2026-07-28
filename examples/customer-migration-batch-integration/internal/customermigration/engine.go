// Package customermigration 은 customer migration batch integration 예제를 구현한다.
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
	// BatchName 은 report에서 사용하는 안정적인 batch job 이름이다.
	BatchName = "customer-migration-batch"
	// StepName 은 batch report 내부에서 사용하는 안정적인 step 이름이다.
	StepName = "customer-migration-step"
	// CheckpointKey 는 checkpoint store 안에서 이 데모 batch를 식별한다.
	CheckpointKey = "customer-migration"
	// DefaultChunkSize 는 restart 동작을 예측 가능하게 만들도록 데모 chunk 크기를 고정한다.
	DefaultChunkSize = 2
)

const (
	// TriggerManual 은 운영자가 시작한 run에 붙이는 label이다.
	TriggerManual = "manual"
	// TriggerSchedule 은 scheduler가 시작한 run에 붙이는 label이다.
	TriggerSchedule = "schedule"
)

const (
	// ErrorCodeRequestCancelled 는 호출자 cancellation으로 작업이 중단될 때 반환된다.
	ErrorCodeRequestCancelled = "request_cancelled"
	// ErrorCodeWriterCrash 는 데모 writer crash가 주입될 때 반환된다.
	ErrorCodeWriterCrash = "writer_crash"
	// ErrorCodeTransientCustomer 는 transient retry가 모두 소진될 때 반환된다.
	ErrorCodeTransientCustomer = "transient_customer_exhausted"
	// ErrorCodePermanentCustomer 는 customer가 dead-letter 처리될 때 반환된다.
	ErrorCodePermanentCustomer = "permanent_customer"
	// ErrorCodeInvalidCheckpoint 는 checkpoint 상태가 손상되었을 때 반환된다.
	ErrorCodeInvalidCheckpoint = "invalid_checkpoint"
	// ErrorCodeInvalidCrashAfter 는 crash injection 설정이 잘못되었을 때 반환된다.
	ErrorCodeInvalidCrashAfter = "invalid_crash_after_new_writes"
	// ErrorCodeRunInProgress 는 다른 run이 store 소유권을 갖고 있을 때 반환된다.
	ErrorCodeRunInProgress = "run_in_progress"
	// ErrorCodeNotLeader 는 process가 leadership을 보유하지 않을 때 반환된다.
	ErrorCodeNotLeader = "not_leader"
	// ErrorCodeInvalidRunID 는 필수 run id가 비어 있을 때 반환된다.
	ErrorCodeInvalidRunID = "invalid_run_id"
	// ErrorCodeInvalidRequest 는 요청 JSON 형식이 잘못되었을 때 반환된다.
	ErrorCodeInvalidRequest = "invalid_request"
	// ErrorCodeRequestTooLarge 는 요청 JSON이 상한을 넘을 때 반환된다.
	ErrorCodeRequestTooLarge = "request_too_large"
	// ErrorCodeReportNotFound 는 run report를 사용할 수 없을 때 반환된다.
	ErrorCodeReportNotFound = "report_not_found"
	// ErrorCodeNoActiveRun 은 cancel할 in-flight run이 없을 때 반환된다.
	ErrorCodeNoActiveRun = "no_active_run"
)

var (
	// ErrInvalidCustomer 는 형식이 잘못된 fixture customer를 나타낸다.
	ErrInvalidCustomer = errors.New("invalid customer")
	// ErrTransientCustomer 는 transient fixture 실패가 모두 소진되었음을 나타낸다.
	ErrTransientCustomer = errors.New("transient customer")
	// ErrPermanentCustomer 는 결정적인 dead-letter fixture 실패를 나타낸다.
	ErrPermanentCustomer = errors.New("permanent customer")
	// ErrWriterCrash 는 주입된 writer crash를 나타낸다.
	ErrWriterCrash = errors.New("writer crash")
	// ErrDuplicateCustomer 는 예상하지 못한 중복 sink write를 나타낸다.
	ErrDuplicateCustomer = errors.New("duplicate customer")
	// ErrInvalidCheckpoint 는 손상된 checkpoint 상태를 나타낸다.
	ErrInvalidCheckpoint = errors.New("invalid checkpoint")
	// ErrRunInProgress 는 공유 데모 store의 동시 변경을 나타낸다.
	ErrRunInProgress = errors.New("run in progress")
	// ErrNotLeader 는 leadership 없이 작업을 시도했음을 나타낸다.
	ErrNotLeader = errors.New("not leader")
	// ErrInvalidRunID 는 비어 있거나 형식이 잘못된 run identifier를 나타낸다.
	ErrInvalidRunID = errors.New("invalid run id")
	// ErrInvalidCrashAfter 는 형식이 잘못된 crash injection 설정을 나타낸다.
	ErrInvalidCrashAfter = errors.New("invalid crash_after_new_writes")
)

// FailureScenario 는 customer별 결정적 동작을 설명한다.
type FailureScenario string

const (
	// ScenarioSuccess 는 실패 주입 없이 migration한다.
	ScenarioSuccess = FailureScenario("success")
	// ScenarioTransientOnce 는 한 번 실패한 뒤 retry에서 성공한다.
	ScenarioTransientOnce FailureScenario = "transient_once"
	// ScenarioPermanent 는 항상 customer를 dead-letter store로 보낸다.
	ScenarioPermanent FailureScenario = "permanent"
)

// CustomerRecord 는 source customer fixture 하나다.
type CustomerRecord struct {
	Index    int
	ID       string
	Email    string
	Segment  string
	Scenario FailureScenario
	Reason   string
}

// MigratedCustomer 는 내부 migrated-customer sink 값이다.
type MigratedCustomer struct {
	ID       string `json:"id"`
	Email    string `json:"-"`
	Segment  string `json:"segment"`
	Attempts int    `json:"attempts"`
}

// Checkpoint 는 다음으로 읽을 source index를 저장한다.
type Checkpoint struct {
	NextIndex int `json:"next_index"`
}

// DeadLetter 는 영구적으로 건너뛴 customer를 기록한다.
type DeadLetter struct {
	CustomerID string `json:"customer_id"`
	Reason     string `json:"reason"`
	Attempts   int    `json:"attempts"`
}

// Stores 는 공유 in-memory batch store들을 묶는다.
type Stores struct {
	Checkpoints *RecordingCheckpointStore
	Sink        *CustomerSink
	DeadLetters *DeadLetterStore
}

// NewStores 는 데모 process 하나를 위한 빈 in-memory store들을 만든다.
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

// RunOptions 는 batch run 하나를 설정한다.
type RunOptions struct {
	RunID               string
	Trigger             string
	Stores              *Stores
	CrashAfterNewWrites int
	BeforeWrite         func(context.Context)
}

// RunResponse 는 HTTP handler가 반환하는 공개 run 프로젝션이다.
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

// Summary 는 안정적인 run counter를 담는다.
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

// ReportNode 는 timestamp를 제거한 batch report 프로젝션이다.
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

// RunBatch 는 customer migration batch run 하나를 실행한다.
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

// RecordingCheckpointStore 는 lock 뒤에 checkpoint를 저장한다.
type RecordingCheckpointStore struct {
	mu     sync.RWMutex
	values map[string]Checkpoint
}

// NewRecordingCheckpointStore 는 빈 checkpoint store를 만든다.
func NewRecordingCheckpointStore() *RecordingCheckpointStore {
	return &RecordingCheckpointStore{values: make(map[string]Checkpoint)}
}

// Load 는 key에 해당하는 checkpoint가 있으면 반환한다.
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

// Save 는 key에 대한 checkpoint를 저장한다.
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

// Snapshot 은 타입이 보존된 defensive checkpoint snapshot을 반환한다.
func (s *RecordingCheckpointStore) Snapshot(key string) (Checkpoint, bool) {
	if s == nil {
		return Checkpoint{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	checkpoint, ok := s.values[key]
	return checkpoint, ok
}

// CustomerSink 는 migration된 customer를 idempotent하게 저장한다.
type CustomerSink struct {
	mu       sync.RWMutex
	byID     map[string]MigratedCustomer
	order    []string
	newCount int
}

// NewCustomerSink 는 빈 migrated customer sink를 만든다.
func NewCustomerSink() *CustomerSink {
	return &CustomerSink{byID: make(map[string]MigratedCustomer)}
}

// Put 은 해당 ID가 아직 migration되지 않았을 때만 customer를 저장한다.
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

// IDs 는 migration된 customer ID를 삽입 순서대로 반환한다.
func (s *CustomerSink) IDs() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyStrings(s.order)
}

// NewCount 는 새로 삽입된 customer 수를 반환한다.
func (s *CustomerSink) NewCount() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.newCount
}

// DeadLetterStore 는 customer ID 기준으로 고유한 dead letter를 저장한다.
type DeadLetterStore struct {
	mu    sync.RWMutex
	byID  map[string]DeadLetter
	order []string
}

// NewDeadLetterStore 는 빈 dead-letter store를 만든다.
func NewDeadLetterStore() *DeadLetterStore {
	return &DeadLetterStore{byID: make(map[string]DeadLetter)}
}

// Record 는 아직 기록되지 않은 경우 dead letter 하나를 저장한다.
func (s *DeadLetterStore) Record(letter DeadLetter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[letter.CustomerID]; exists {
		return
	}
	s.byID[letter.CustomerID] = letter
	s.order = append(s.order, letter.CustomerID)
}

// Snapshot 은 dead letter를 삽입 순서대로 반환한다.
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
