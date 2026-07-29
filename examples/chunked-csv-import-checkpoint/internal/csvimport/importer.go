// Package csvimport 는 checkpoint와 restart를 사용하는 chunked CSV import를 보여준다.
package csvimport

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/bluetape4k/bluetape-go/batch"
)

const (
	// DefaultChunkSize 는 customer import step에서 사용하는 데모 chunk 크기다.
	DefaultChunkSize = 2
	// DefaultCheckpointKey 는 첫 실행과 restart 실행이 공유하는 checkpoint key다.
	DefaultCheckpointKey = "customer-csv-import"
	// StepName 은 report에 표시되는 batch step 이름이다.
	StepName = "chunked-customer-csv-import"
	// JobName 은 report에 표시되는 batch job 이름이다.
	JobName = "customer-csv-import"
)

var (
	// ErrInvalidCSV 는 CSV 형식 또는 header가 잘못되었음을 나타낸다.
	ErrInvalidCSV = errors.New("invalid customer csv")
	// ErrInvalidCustomer 는 customer로 가져올 수 없는 row를 나타낸다.
	ErrInvalidCustomer = errors.New("invalid customer row")
	// ErrSimulatedCrash 는 부분 chunk commit 뒤에 주입되는 교육용 실패를 나타낸다.
	ErrSimulatedCrash = errors.New("simulated writer crash")
)

// CSVRow 는 customer CSV fixture에서 parsing된 data row 하나다.
type CSVRow struct {
	Index      int
	LineNumber int
	CustomerID string
	Email      string
	Tier       string
}

// Customer 는 batch step이 기록하는 정규화된 domain 객체다.
type Customer struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

// Checkpoint 는 CSV header를 제외하고 다음에 읽을 data-row index를 저장한다.
type Checkpoint struct {
	NextRow int `json:"next_row"`
}

// CSVReader 는 CSV 파일에서 customer를 읽고 checkpoint restore를 지원한다.
type CSVReader struct {
	path   string
	rows   []CSVRow
	next   int
	opened bool
	closed bool
}

// NewCSVReader 는 path를 읽는 checkpoint-aware reader를 만든다.
func NewCSVReader(path string) *CSVReader {
	return &CSVReader{path: path}
}

// Open 은 CSV fixture를 parsing하고 검증한다.
func (r *CSVReader) Open(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("csv reader must not be nil")
	}
	file, err := os.Open(r.path)
	if err != nil {
		return fmt.Errorf("open customer csv: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close customer csv: %w", closeErr))
		}
	}()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return fmt.Errorf("%w: parse: %w", ErrInvalidCSV, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("%w: missing header", ErrInvalidCSV)
	}
	if !slices.Equal(records[0], []string{"customer_id", "email", "tier"}) {
		return fmt.Errorf("%w: header must be customer_id,email,tier", ErrInvalidCSV)
	}

	rows := make([]CSVRow, 0, len(records)-1)
	for i, record := range records[1:] {
		if len(record) != 3 {
			return fmt.Errorf("%w: line %d has %d fields", ErrInvalidCSV, i+2, len(record))
		}
		rows = append(rows, CSVRow{
			Index:      i,
			LineNumber: i + 2,
			CustomerID: record[0],
			Email:      record[1],
			Tier:       record[2],
		})
	}

	r.rows = rows
	r.next = 0
	r.opened = true
	r.closed = false
	return nil
}

// Read 는 다음 CSV row를 반환하고 checkpoint cursor를 앞으로 이동한다.
func (r *CSVReader) Read(ctx context.Context) (CSVRow, bool, error) {
	var zero CSVRow
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	if r == nil || !r.opened {
		return zero, false, fmt.Errorf("csv reader is not open")
	}
	if r.next >= len(r.rows) {
		return zero, false, nil
	}
	row := r.rows[r.next]
	r.next++
	return row, true, nil
}

// Restore 는 다음 unread row cursor를 저장된 checkpoint 위치로 이동한다.
func (r *CSVReader) Restore(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || !r.opened {
		return fmt.Errorf("csv reader is not open")
	}
	checkpoint, err := asCheckpoint(value)
	if err != nil {
		return err
	}
	if checkpoint.NextRow < 0 || checkpoint.NextRow > len(r.rows) {
		return fmt.Errorf("%w: next_row %d outside 0..%d", ErrInvalidCSV, checkpoint.NextRow, len(r.rows))
	}
	r.next = checkpoint.NextRow
	return nil
}

// Checkpoint 는 commit된 작업 이후 다음 unread row cursor를 반환한다.
func (r *CSVReader) Checkpoint(ctx context.Context) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if r == nil || !r.opened {
		return nil, false, fmt.Errorf("csv reader is not open")
	}
	return Checkpoint{NextRow: r.next}, true, nil
}

// Close 는 reader cleanup을 기록한다.
func (r *CSVReader) Close(ctx context.Context) error {
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
func (r *CSVReader) Closed() bool {
	return r != nil && r.closed
}

// CustomerProcessor 는 row 검증과 정규화를 수행하는 processor를 반환한다.
func CustomerProcessor() batch.Processor[CSVRow, Customer] {
	return batch.ProcessorFunc[CSVRow, Customer](func(ctx context.Context, row CSVRow) (Customer, bool, error) {
		if err := ctx.Err(); err != nil {
			return Customer{}, false, err
		}
		customer := Customer{
			ID:    strings.TrimSpace(row.CustomerID),
			Email: strings.ToLower(strings.TrimSpace(row.Email)),
			Tier:  strings.ToLower(strings.TrimSpace(row.Tier)),
		}
		if customer.ID == "" || customer.Email == "" || customer.Tier == "" {
			return Customer{}, false, fmt.Errorf("%w: line %d customer %q has blank field", ErrInvalidCustomer, row.LineNumber, customer.ID)
		}
		return customer, true, nil
	})
}

// CustomerSink 는 customer ID 기준 idempotency를 유지하며 import된 customer를 저장한다.
type CustomerSink struct {
	mu             sync.RWMutex
	customers      map[string]Customer
	order          []string
	newCommits     int
	duplicateSkips int
	writeAttempts  int
}

// NewCustomerSink 는 비어 있는 in-memory customer sink를 만든다.
func NewCustomerSink() *CustomerSink {
	return &CustomerSink{customers: make(map[string]Customer)}
}

func (s *CustomerSink) put(customer Customer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeAttempts++
	if _, exists := s.customers[customer.ID]; exists {
		s.duplicateSkips++
		return false
	}
	s.customers[customer.ID] = customer
	s.order = append(s.order, customer.ID)
	s.newCommits++
	return true
}

// Snapshot 은 최초 commit 순서대로 customer 목록을 반환한다.
func (s *CustomerSink) Snapshot() []Customer {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	customers := make([]Customer, 0, len(s.order))
	for _, id := range s.order {
		customers = append(customers, s.customers[id])
	}
	return customers
}

// NewCommits 는 고유하게 commit된 customer 수를 반환한다.
func (s *CustomerSink) NewCommits() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.newCommits
}

// DuplicateSkips 는 건너뛴 중복 customer write 수를 반환한다.
func (s *CustomerSink) DuplicateSkips() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.duplicateSkips
}

// WriteAttempts 는 row 단위 write 시도 횟수를 반환한다.
func (s *CustomerSink) WriteAttempts() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.writeAttempts
}

// CustomerWriter 는 customer chunk를 sink에 쓰며 한 번의 crash를 주입할 수 있다.
type CustomerWriter struct {
	sink                 *CustomerSink
	crashAfterNewCommits int
	crashed              bool
	opened               bool
	closed               bool
}

// WriterOptions 는 CustomerWriter 동작을 설정한다.
type WriterOptions struct {
	CrashAfterNewCommits int
}

// NewCustomerWriter 는 sink에 쓰는 chunk writer를 만든다.
func NewCustomerWriter(sink *CustomerSink, options WriterOptions) *CustomerWriter {
	if sink == nil {
		sink = NewCustomerSink()
	}
	return &CustomerWriter{
		sink:                 sink,
		crashAfterNewCommits: options.CrashAfterNewCommits,
	}
}

// Open 은 writer startup을 기록한다.
func (w *CustomerWriter) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil || w.sink == nil {
		return fmt.Errorf("customer writer must not be nil")
	}
	w.opened = true
	w.closed = false
	return nil
}

// Write 는 customer ID 기준 idempotency를 유지하며 chunk를 commit한다.
func (w *CustomerWriter) Write(ctx context.Context, chunk []Customer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil || !w.opened {
		return fmt.Errorf("customer writer is not open")
	}
	for _, customer := range chunk {
		if err := ctx.Err(); err != nil {
			return err
		}
		inserted := w.sink.put(customer)
		if inserted && w.crashAfterNewCommits > 0 && !w.crashed && w.sink.NewCommits() >= w.crashAfterNewCommits {
			w.crashed = true
			return fmt.Errorf("commit customer %s: %w", customer.ID, ErrSimulatedCrash)
		}
	}
	return nil
}

// Close 는 writer cleanup을 기록한다.
func (w *CustomerWriter) Close(ctx context.Context) error {
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
func (w *CustomerWriter) Closed() bool {
	return w != nil && w.closed
}

// RunOptions 는 import run 하나를 설정한다.
type RunOptions struct {
	CSVPath              string
	ChunkSize            int
	CheckpointKey        string
	CheckpointStore      batch.CheckpointStore
	Sink                 *CustomerSink
	CrashAfterNewCommits int
	Processor            batch.Processor[CSVRow, Customer]
}

// ImportRun 은 batch job run 하나에 대한 안정적인 출력이다.
type ImportRun struct {
	Report          ReportNode  `json:"report"`
	Checkpoint      *Checkpoint `json:"checkpoint,omitempty"`
	Customers       []Customer  `json:"customers"`
	NewCommits      int         `json:"new_commits"`
	DuplicateSkips  int         `json:"duplicate_skips"`
	WriteAttempts   int         `json:"write_attempts"`
	ReaderWasClosed bool        `json:"-"`
	WriterWasClosed bool        `json:"-"`
}

// ReportNode 는 timestamp를 제거한 batch report 프로젝션이다.
type ReportNode struct {
	Name        string       `json:"name"`
	Status      batch.Status `json:"status"`
	ReadCount   int          `json:"read_count"`
	WriteCount  int          `json:"write_count"`
	FilterCount int          `json:"filter_count,omitempty"`
	SkipCount   int          `json:"skip_count,omitempty"`
	RetryCount  int          `json:"retry_count,omitempty"`
	Error       string       `json:"error,omitempty"`
	Children    []ReportNode `json:"children,omitempty"`
}

// DemoResult 는 실행 가능한 예제 출력이다.
type DemoResult struct {
	CheckpointKey     string     `json:"checkpoint_key"`
	ChunkSize         int        `json:"chunk_size"`
	FirstRun          ImportRun  `json:"first_run"`
	RestartRun        ImportRun  `json:"restart_run"`
	CustomersImported int        `json:"customers_imported"`
	DuplicateSkips    int        `json:"duplicate_skips"`
	Customers         []Customer `json:"customers"`
}

// RunImport 는 checkpoint-aware customer import job 하나를 실행한다.
func RunImport(ctx context.Context, options RunOptions) (ImportRun, error) {
	ctx = normalizeContext(ctx)
	if options.CSVPath == "" {
		return ImportRun{}, fmt.Errorf("csv path must not be empty")
	}
	if options.ChunkSize == 0 {
		options.ChunkSize = DefaultChunkSize
	}
	if options.CheckpointKey == "" {
		options.CheckpointKey = DefaultCheckpointKey
	}
	if options.CheckpointStore == nil {
		options.CheckpointStore = batch.NewMemoryCheckpointStore()
	}
	if options.Sink == nil {
		options.Sink = NewCustomerSink()
	}
	if options.Processor == nil {
		options.Processor = CustomerProcessor()
	}

	reader := NewCSVReader(options.CSVPath)
	writer := NewCustomerWriter(options.Sink, WriterOptions{
		CrashAfterNewCommits: options.CrashAfterNewCommits,
	})
	step, err := batch.NewStep(batch.StepOptions[CSVRow, Customer]{
		Name:            StepName,
		ChunkSize:       options.ChunkSize,
		Reader:          reader,
		Processor:       options.Processor,
		Writer:          writer,
		CheckpointStore: options.CheckpointStore,
		CheckpointKey:   options.CheckpointKey,
	})
	if err != nil {
		return ImportRun{}, fmt.Errorf("create batch step: %w", err)
	}
	job, err := batch.NewJob(JobName, step)
	if err != nil {
		return ImportRun{}, fmt.Errorf("create batch job: %w", err)
	}

	report := job.Run(ctx)
	checkpoint, err := loadCheckpoint(context.WithoutCancel(ctx), options.CheckpointStore, options.CheckpointKey)
	if err != nil {
		return ImportRun{}, err
	}
	return ImportRun{
		Report:          projectReport(report),
		Checkpoint:      checkpoint,
		Customers:       options.Sink.Snapshot(),
		NewCommits:      options.Sink.NewCommits(),
		DuplicateSkips:  options.Sink.DuplicateSkips(),
		WriteAttempts:   options.Sink.WriteAttempts(),
		ReaderWasClosed: reader.Closed(),
		WriterWasClosed: writer.Closed(),
	}, nil
}

// RunDemo 는 의도된 fail-and-restart scenario를 실행한다.
func RunDemo(ctx context.Context, csvPath string) (DemoResult, error) {
	store := batch.NewMemoryCheckpointStore()
	sink := NewCustomerSink()

	first, err := RunImport(ctx, RunOptions{
		CSVPath:              csvPath,
		CheckpointStore:      store,
		Sink:                 sink,
		CrashAfterNewCommits: 3,
	})
	if err != nil {
		return DemoResult{}, err
	}
	restart, err := RunImport(ctx, RunOptions{
		CSVPath:         csvPath,
		CheckpointStore: store,
		Sink:            sink,
	})
	if err != nil {
		return DemoResult{}, err
	}
	if first.Report.Status != batch.StatusFailed {
		return DemoResult{}, fmt.Errorf("expected first run to fail, got %s", first.Report.Status)
	}
	if restart.Report.Status != batch.StatusCompleted {
		return DemoResult{}, fmt.Errorf("expected restart to complete, got %s", restart.Report.Status)
	}

	return DemoResult{
		CheckpointKey:     DefaultCheckpointKey,
		ChunkSize:         DefaultChunkSize,
		FirstRun:          first,
		RestartRun:        restart,
		CustomersImported: sink.NewCommits(),
		DuplicateSkips:    sink.DuplicateSkips(),
		Customers:         sink.Snapshot(),
	}, nil
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

func loadCheckpoint(ctx context.Context, store batch.CheckpointStore, key string) (*Checkpoint, error) {
	value, exists, err := store.Load(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}
	if !exists {
		return nil, nil
	}
	checkpoint, err := asCheckpoint(value)
	if err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

func asCheckpoint(value any) (Checkpoint, error) {
	switch checkpoint := value.(type) {
	case Checkpoint:
		return checkpoint, nil
	case *Checkpoint:
		if checkpoint == nil {
			return Checkpoint{}, fmt.Errorf("%w: nil checkpoint", ErrInvalidCSV)
		}
		return *checkpoint, nil
	default:
		return Checkpoint{}, fmt.Errorf("%w: unsupported checkpoint %T", ErrInvalidCSV, value)
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
