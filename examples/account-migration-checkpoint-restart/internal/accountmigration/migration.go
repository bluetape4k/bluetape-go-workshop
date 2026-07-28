// Package accountmigration은 local migration에서 batch checkpoint restart를 보여준다.
package accountmigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/bluetape4k/bluetape-go/batch"
)

const (
	// JobName은 report에 표시되는 batch job name이다.
	JobName = "account-migration"
	// StepName은 report에 표시되는 checkpoint-aware step name이다.
	StepName = "legacy-account-migration"
	// DefaultCheckpointKey는 first run과 restart run이 공유하는 checkpoint key다.
	DefaultCheckpointKey = "account-migration-v1"
	// DefaultChunkSize는 checkpoint 이동을 쉽게 검사할 수 있게 작게 유지한다.
	DefaultChunkSize = 2
)

var (
	// ErrInvalidAccount는 migration할 수 없는 legacy account를 나타낸다.
	ErrInvalidAccount = errors.New("invalid legacy account")
	// ErrMigrationCrash는 교육용으로 고정된 deterministic failure를 나타낸다.
	ErrMigrationCrash = errors.New("simulated migration crash")
	// ErrInvalidCheckpoint는 restart에 사용할 수 없는 checkpoint를 나타낸다.
	ErrInvalidCheckpoint = errors.New("invalid migration checkpoint")
	// ErrDuplicateAccount는 target account 중복 write를 나타낸다.
	ErrDuplicateAccount = errors.New("duplicate target account")
)

// LegacyAccount는 migration fixture의 source row 하나다.
type LegacyAccount struct {
	Index     int
	AccountID string
	Email     string
	Plan      string
	Region    string
}

// TargetAccount는 migration이 write하는 normalized row다.
type TargetAccount struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Plan   string `json:"plan"`
	Region string `json:"region"`
}

// MigrationCheckpoint는 다음으로 읽을 source index를 저장한다.
type MigrationCheckpoint struct {
	NextIndex int `json:"next_index"`
}

// CheckpointOperation은 checkpoint load/save behavior에 대한 stable evidence다.
type CheckpointOperation struct {
	Kind       string               `json:"kind"`
	Key        string               `json:"key"`
	Checkpoint *MigrationCheckpoint `json:"checkpoint,omitempty"`
	Found      bool                 `json:"found,omitempty"`
}

// RunOptions는 account migration run 하나를 설정한다.
type RunOptions struct {
	Accounts        []LegacyAccount
	ChunkSize       int
	CheckpointKey   string
	CheckpointStore batch.CheckpointStore
	TargetStore     *TargetAccountStore
	CrashOnAccount  string
	Processor       batch.Processor[LegacyAccount, TargetAccount]
}

// MigrationRun은 batch run 하나의 stable output이다.
type MigrationRun struct {
	Report          ReportNode           `json:"report"`
	Checkpoint      *MigrationCheckpoint `json:"checkpoint,omitempty"`
	ReadIDs         []string             `json:"read_ids"`
	WrittenIDs      []string             `json:"written_ids"`
	Targets         []TargetAccount      `json:"targets"`
	ReaderWasClosed bool                 `json:"-"`
	WriterWasClosed bool                 `json:"-"`
}

// DemoResult는 실행 가능한 fail-and-restart scenario output이다.
type DemoResult struct {
	CheckpointKey      string                `json:"checkpoint_key"`
	ChunkSize          int                   `json:"chunk_size"`
	FirstRun           MigrationRun          `json:"first_run"`
	RestartRun         MigrationRun          `json:"restart_run"`
	FinalCheckpoint    *MigrationCheckpoint  `json:"final_checkpoint,omitempty"`
	AccountsMigrated   int                   `json:"accounts_migrated"`
	Targets            []TargetAccount       `json:"targets"`
	CheckpointActivity []CheckpointOperation `json:"checkpoint_activity"`
}

// ReportNode는 timestamp를 제거한 batch report projection이다.
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

// DefaultAccounts는 deterministic legacy account fixture를 반환한다.
func DefaultAccounts() []LegacyAccount {
	return []LegacyAccount{
		{Index: 0, AccountID: "acct-1001", Email: "Alpha@Example.com", Plan: "Gold", Region: "NA"},
		{Index: 1, AccountID: "acct-1002", Email: "Beta@Example.com", Plan: "Silver", Region: "EU"},
		{Index: 2, AccountID: "acct-1003", Email: "Gamma@Example.com", Plan: "Gold", Region: "APAC"},
		{Index: 3, AccountID: "acct-1004", Email: "Delta@Example.com", Plan: "Bronze", Region: "NA"},
		{Index: 4, AccountID: "acct-1005", Email: "Omega@Example.com", Plan: "Silver", Region: "EU"},
	}
}

// RunDemo는 의도된 first-run failure와 restart sequence를 실행한다.
func RunDemo(ctx context.Context) (DemoResult, error) {
	store := NewRecordingCheckpointStore()
	targets := NewTargetAccountStore()

	first, err := RunMigration(ctx, RunOptions{
		CheckpointStore: store,
		TargetStore:     targets,
		CrashOnAccount:  "acct-1003",
	})
	if err != nil {
		return DemoResult{}, err
	}
	restart, err := RunMigration(ctx, RunOptions{
		CheckpointStore: store,
		TargetStore:     targets,
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

	finalCheckpoint, err := loadMigrationCheckpoint(context.WithoutCancel(normalizeContext(ctx)), store, DefaultCheckpointKey)
	if err != nil {
		return DemoResult{}, err
	}
	return DemoResult{
		CheckpointKey:      DefaultCheckpointKey,
		ChunkSize:          DefaultChunkSize,
		FirstRun:           first,
		RestartRun:         restart,
		FinalCheckpoint:    finalCheckpoint,
		AccountsMigrated:   len(targets.Snapshot()),
		Targets:            targets.Snapshot(),
		CheckpointActivity: store.Operations(),
	}, nil
}

// RunMigration은 checkpoint-aware account migration batch job 하나를 실행한다.
func RunMigration(ctx context.Context, options RunOptions) (MigrationRun, error) {
	ctx = normalizeContext(ctx)
	if len(options.Accounts) == 0 {
		options.Accounts = DefaultAccounts()
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
	if options.TargetStore == nil {
		options.TargetStore = NewTargetAccountStore()
	}
	if options.Processor == nil {
		options.Processor = NewAccountProcessor(options.CrashOnAccount)
	}

	reader := NewMigrationReader(options.Accounts)
	writer := NewTargetAccountWriter(options.TargetStore)
	step, err := batch.NewStep(batch.StepOptions[LegacyAccount, TargetAccount]{
		Name:            StepName,
		ChunkSize:       options.ChunkSize,
		Reader:          reader,
		Processor:       options.Processor,
		Writer:          writer,
		CheckpointStore: options.CheckpointStore,
		CheckpointKey:   options.CheckpointKey,
	})
	if err != nil {
		return MigrationRun{}, fmt.Errorf("create migration step: %w", err)
	}
	job, err := batch.NewJob(JobName, step)
	if err != nil {
		return MigrationRun{}, fmt.Errorf("create migration job: %w", err)
	}

	report := job.Run(ctx)
	checkpoint, err := loadMigrationCheckpoint(context.WithoutCancel(ctx), options.CheckpointStore, options.CheckpointKey)
	if err != nil && (!report.IsFailure() || !errors.Is(err, ErrInvalidCheckpoint)) {
		return MigrationRun{}, err
	}
	return MigrationRun{
		Report:          projectReport(report),
		Checkpoint:      checkpoint,
		ReadIDs:         reader.ReadIDs(),
		WrittenIDs:      writer.WrittenIDs(),
		Targets:         options.TargetStore.Snapshot(),
		ReaderWasClosed: reader.Closed(),
		WriterWasClosed: writer.Closed(),
	}, nil
}

// MarshalResult는 demo CLI를 위한 deterministic indented JSON을 반환한다.
func MarshalResult(result DemoResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// MigrationReader는 legacy account를 읽고 checkpoint restore를 지원한다.
type MigrationReader struct {
	accounts []LegacyAccount
	next     int
	readIDs  []string
	opened   bool
	closed   bool
}

// NewMigrationReader는 account defensive copy를 읽는 reader를 만든다.
func NewMigrationReader(accounts []LegacyAccount) *MigrationReader {
	return &MigrationReader{accounts: append([]LegacyAccount(nil), accounts...)}
}

// Open은 deterministic source fixture를 검증한다.
func (r *MigrationReader) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("migration reader must not be nil")
	}
	seen := make(map[string]struct{}, len(r.accounts))
	for index, account := range r.accounts {
		if account.Index != index {
			return fmt.Errorf("%w: account %q has index %d, want %d", ErrInvalidAccount, account.AccountID, account.Index, index)
		}
		id := strings.TrimSpace(account.AccountID)
		if id == "" {
			return fmt.Errorf("%w: account at index %d has blank id", ErrInvalidAccount, index)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%w: duplicate source account %q", ErrInvalidAccount, id)
		}
		seen[id] = struct{}{}
	}
	r.next = 0
	r.readIDs = nil
	r.opened = true
	r.closed = false
	return nil
}

// Read는 다음 legacy account를 반환한다.
func (r *MigrationReader) Read(ctx context.Context) (LegacyAccount, bool, error) {
	var zero LegacyAccount
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	if r == nil || !r.opened {
		return zero, false, fmt.Errorf("migration reader is not open")
	}
	if r.next >= len(r.accounts) {
		return zero, false, nil
	}
	account := r.accounts[r.next]
	r.next++
	r.readIDs = append(r.readIDs, account.AccountID)
	return account, true, nil
}

// Restore는 reader cursor를 저장된 checkpoint로 이동한다.
func (r *MigrationReader) Restore(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || !r.opened {
		return fmt.Errorf("migration reader is not open")
	}
	checkpoint, err := asMigrationCheckpoint(value)
	if err != nil {
		return err
	}
	if checkpoint.NextIndex < 0 || checkpoint.NextIndex > len(r.accounts) {
		return fmt.Errorf("%w: next_index %d outside 0..%d", ErrInvalidCheckpoint, checkpoint.NextIndex, len(r.accounts))
	}
	r.next = checkpoint.NextIndex
	return nil
}

// Checkpoint는 다음 unread source index를 반환한다.
func (r *MigrationReader) Checkpoint(ctx context.Context) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if r == nil || !r.opened {
		return nil, false, fmt.Errorf("migration reader is not open")
	}
	return MigrationCheckpoint{NextIndex: r.next}, true, nil
}

// Close는 reader cleanup을 기록한다.
func (r *MigrationReader) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.closed = true
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// Closed는 Close 호출 여부를 보고한다.
func (r *MigrationReader) Closed() bool {
	return r != nil && r.closed
}

// ReadIDs는 이 reader가 읽은 source account ID를 반환한다.
func (r *MigrationReader) ReadIDs() []string {
	if r == nil {
		return nil
	}
	return slices.Clone(r.readIDs)
}

// NewAccountProcessor는 deterministic migration processor를 만든다.
func NewAccountProcessor(crashOnAccount string) batch.Processor[LegacyAccount, TargetAccount] {
	return batch.ProcessorFunc[LegacyAccount, TargetAccount](func(ctx context.Context, account LegacyAccount) (TargetAccount, bool, error) {
		if err := ctx.Err(); err != nil {
			return TargetAccount{}, false, err
		}
		id := strings.TrimSpace(account.AccountID)
		email := strings.ToLower(strings.TrimSpace(account.Email))
		plan := strings.ToLower(strings.TrimSpace(account.Plan))
		region := strings.ToUpper(strings.TrimSpace(account.Region))
		if id == "" || email == "" || plan == "" || region == "" {
			return TargetAccount{}, false, fmt.Errorf("%w: account %q has blank field", ErrInvalidAccount, id)
		}
		if crashOnAccount != "" && id == crashOnAccount {
			return TargetAccount{}, false, fmt.Errorf("%w: account %s", ErrMigrationCrash, id)
		}
		return TargetAccount{ID: id, Email: email, Plan: plan, Region: region}, true, nil
	})
}

// TargetAccountStore는 account ID 기준 uniqueness를 지키며 migrated account를 저장한다.
type TargetAccountStore struct {
	mu      sync.RWMutex
	targets map[string]TargetAccount
	order   []string
	writes  []string
}

// NewTargetAccountStore는 비어 있는 target store를 만든다.
func NewTargetAccountStore() *TargetAccountStore {
	return &TargetAccountStore{targets: make(map[string]TargetAccount)}
}

// Put은 migrated account 하나를 저장한다.
func (s *TargetAccountStore) Put(account TargetAccount) error {
	if s == nil {
		return fmt.Errorf("target account store must not be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.targets[account.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateAccount, account.ID)
	}
	s.targets[account.ID] = account
	s.order = append(s.order, account.ID)
	s.writes = append(s.writes, account.ID)
	return nil
}

// Snapshot은 account를 first-write order로 반환한다.
func (s *TargetAccountStore) Snapshot() []TargetAccount {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]TargetAccount, 0, len(s.order))
	for _, id := range s.order {
		result = append(result, s.targets[id])
	}
	return result
}

// WriteIDs는 target ID를 write order로 반환한다.
func (s *TargetAccountStore) WriteIDs() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.writes)
}

// TargetAccountWriter는 migrated chunk를 target store에 persist한다.
type TargetAccountWriter struct {
	store      *TargetAccountStore
	writtenIDs []string
	opened     bool
	closed     bool
}

// NewTargetAccountWriter는 chunk writer를 만든다.
func NewTargetAccountWriter(store *TargetAccountStore) *TargetAccountWriter {
	if store == nil {
		store = NewTargetAccountStore()
	}
	return &TargetAccountWriter{store: store}
}

// Open은 writer startup을 기록한다.
func (w *TargetAccountWriter) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil || w.store == nil {
		return fmt.Errorf("target writer must not be nil")
	}
	w.opened = true
	w.closed = false
	w.writtenIDs = nil
	return nil
}

// Write는 migrated chunk를 commit한다.
func (w *TargetAccountWriter) Write(ctx context.Context, chunk []TargetAccount) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w == nil || !w.opened {
		return fmt.Errorf("target writer is not open")
	}
	for _, account := range chunk {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.store.Put(account); err != nil {
			return err
		}
		w.writtenIDs = append(w.writtenIDs, account.ID)
	}
	return nil
}

// Close는 writer cleanup을 기록한다.
func (w *TargetAccountWriter) Close(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.closed = true
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// Closed는 Close 호출 여부를 보고한다.
func (w *TargetAccountWriter) Closed() bool {
	return w != nil && w.closed
}

// WrittenIDs는 이 writer가 쓴 account ID를 반환한다.
func (w *TargetAccountWriter) WrittenIDs() []string {
	if w == nil {
		return nil
	}
	return slices.Clone(w.writtenIDs)
}

// RecordingCheckpointStore는 교체 가능한 작은 in-memory checkpoint store다.
type RecordingCheckpointStore struct {
	mu         sync.RWMutex
	values     map[string]any
	operations []CheckpointOperation
}

// NewRecordingCheckpointStore는 비어 있는 checkpoint store를 만든다.
func NewRecordingCheckpointStore() *RecordingCheckpointStore {
	return &RecordingCheckpointStore{values: make(map[string]any)}
}

// Load는 key로 checkpoint를 반환하고 load를 기록한다.
func (s *RecordingCheckpointStore) Load(ctx context.Context, key string) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if s == nil {
		return nil, false, fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[key]
	s.operations = append(s.operations, CheckpointOperation{
		Kind:       "load",
		Key:        key,
		Checkpoint: checkpointPointer(value),
		Found:      ok,
	})
	return value, ok, nil
}

// Save는 key로 checkpoint를 저장하고 save를 기록한다.
func (s *RecordingCheckpointStore) Save(ctx context.Context, key string, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = checkpoint
	s.operations = append(s.operations, CheckpointOperation{
		Kind:       "save",
		Key:        key,
		Checkpoint: checkpointPointer(checkpoint),
		Found:      true,
	})
	return nil
}

// Operations는 checkpoint activity를 call order로 반환한다.
func (s *RecordingCheckpointStore) Operations() []CheckpointOperation {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.operations)
}

func loadMigrationCheckpoint(ctx context.Context, store batch.CheckpointStore, key string) (*MigrationCheckpoint, error) {
	if store == nil {
		return nil, nil
	}
	value, ok, err := store.Load(ctx, key)
	if err != nil || !ok {
		return nil, err
	}
	checkpoint, err := asMigrationCheckpoint(value)
	if err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

func asMigrationCheckpoint(value any) (MigrationCheckpoint, error) {
	switch checkpoint := value.(type) {
	case MigrationCheckpoint:
		return checkpoint, nil
	case *MigrationCheckpoint:
		if checkpoint == nil {
			return MigrationCheckpoint{}, fmt.Errorf("%w: nil checkpoint", ErrInvalidCheckpoint)
		}
		return *checkpoint, nil
	default:
		return MigrationCheckpoint{}, fmt.Errorf("%w: got %T", ErrInvalidCheckpoint, value)
	}
}

func checkpointPointer(value any) *MigrationCheckpoint {
	checkpoint, err := asMigrationCheckpoint(value)
	if err != nil {
		return nil
	}
	return &checkpoint
}

func projectReport(report batch.Report) ReportNode {
	node := ReportNode{
		Name:        report.Name,
		Status:      report.Status,
		Error:       "",
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
