// Package accountmigration demonstrates batch checkpoint restart for a local migration.
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
	// JobName is the batch job name shown in reports.
	JobName = "account-migration"
	// StepName is the checkpoint-aware step name shown in reports.
	StepName = "legacy-account-migration"
	// DefaultCheckpointKey is shared by the first run and restart run.
	DefaultCheckpointKey = "account-migration-v1"
	// DefaultChunkSize keeps the checkpoint movement easy to inspect.
	DefaultChunkSize = 2
)

var (
	// ErrInvalidAccount reports a legacy account that cannot be migrated.
	ErrInvalidAccount = errors.New("invalid legacy account")
	// ErrMigrationCrash reports the deterministic teaching failure.
	ErrMigrationCrash = errors.New("simulated migration crash")
	// ErrInvalidCheckpoint reports an unusable restart checkpoint.
	ErrInvalidCheckpoint = errors.New("invalid migration checkpoint")
	// ErrDuplicateAccount reports a duplicate target account write.
	ErrDuplicateAccount = errors.New("duplicate target account")
)

// LegacyAccount is one source row in the migration fixture.
type LegacyAccount struct {
	Index     int
	AccountID string
	Email     string
	Plan      string
	Region    string
}

// TargetAccount is the normalized row written by the migration.
type TargetAccount struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Plan   string `json:"plan"`
	Region string `json:"region"`
}

// MigrationCheckpoint stores the next unread source index.
type MigrationCheckpoint struct {
	NextIndex int `json:"next_index"`
}

// CheckpointOperation is stable evidence for checkpoint load/save behavior.
type CheckpointOperation struct {
	Kind       string               `json:"kind"`
	Key        string               `json:"key"`
	Checkpoint *MigrationCheckpoint `json:"checkpoint,omitempty"`
	Found      bool                 `json:"found,omitempty"`
}

// RunOptions configures one account migration run.
type RunOptions struct {
	Accounts        []LegacyAccount
	ChunkSize       int
	CheckpointKey   string
	CheckpointStore batch.CheckpointStore
	TargetStore     *TargetAccountStore
	CrashOnAccount  string
	Processor       batch.Processor[LegacyAccount, TargetAccount]
}

// MigrationRun is stable output for one batch run.
type MigrationRun struct {
	Report          ReportNode           `json:"report"`
	Checkpoint      *MigrationCheckpoint `json:"checkpoint,omitempty"`
	ReadIDs         []string             `json:"read_ids"`
	WrittenIDs      []string             `json:"written_ids"`
	Targets         []TargetAccount      `json:"targets"`
	ReaderWasClosed bool                 `json:"-"`
	WriterWasClosed bool                 `json:"-"`
}

// DemoResult is the runnable fail-and-restart scenario output.
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

// DefaultAccounts returns the deterministic legacy account fixture.
func DefaultAccounts() []LegacyAccount {
	return []LegacyAccount{
		{Index: 0, AccountID: "acct-1001", Email: "Alpha@Example.com", Plan: "Gold", Region: "NA"},
		{Index: 1, AccountID: "acct-1002", Email: "Beta@Example.com", Plan: "Silver", Region: "EU"},
		{Index: 2, AccountID: "acct-1003", Email: "Gamma@Example.com", Plan: "Gold", Region: "APAC"},
		{Index: 3, AccountID: "acct-1004", Email: "Delta@Example.com", Plan: "Bronze", Region: "NA"},
		{Index: 4, AccountID: "acct-1005", Email: "Omega@Example.com", Plan: "Silver", Region: "EU"},
	}
}

// RunDemo executes the expected first-run failure and restart sequence.
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

// RunMigration executes one checkpoint-aware account migration batch job.
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

// MarshalResult returns deterministic indented JSON for the demo CLI.
func MarshalResult(result DemoResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// MigrationReader reads legacy accounts and supports checkpoint restore.
type MigrationReader struct {
	accounts []LegacyAccount
	next     int
	readIDs  []string
	opened   bool
	closed   bool
}

// NewMigrationReader creates a reader over a defensive copy of accounts.
func NewMigrationReader(accounts []LegacyAccount) *MigrationReader {
	return &MigrationReader{accounts: append([]LegacyAccount(nil), accounts...)}
}

// Open validates the deterministic source fixture.
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

// Read returns the next legacy account.
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

// Restore moves the reader cursor to a stored checkpoint.
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

// Checkpoint returns the next unread source index.
func (r *MigrationReader) Checkpoint(ctx context.Context) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if r == nil || !r.opened {
		return nil, false, fmt.Errorf("migration reader is not open")
	}
	return MigrationCheckpoint{NextIndex: r.next}, true, nil
}

// Close records reader cleanup.
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

// Closed reports whether Close was called.
func (r *MigrationReader) Closed() bool {
	return r != nil && r.closed
}

// ReadIDs returns source account IDs read by this reader.
func (r *MigrationReader) ReadIDs() []string {
	if r == nil {
		return nil
	}
	return slices.Clone(r.readIDs)
}

// NewAccountProcessor creates the deterministic migration processor.
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

// TargetAccountStore stores migrated accounts with uniqueness by account ID.
type TargetAccountStore struct {
	mu      sync.RWMutex
	targets map[string]TargetAccount
	order   []string
	writes  []string
}

// NewTargetAccountStore creates an empty target store.
func NewTargetAccountStore() *TargetAccountStore {
	return &TargetAccountStore{targets: make(map[string]TargetAccount)}
}

// Put stores one migrated account.
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

// Snapshot returns accounts in first-write order.
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

// WriteIDs returns target IDs in write order.
func (s *TargetAccountStore) WriteIDs() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.writes)
}

// TargetAccountWriter persists migrated chunks into a target store.
type TargetAccountWriter struct {
	store      *TargetAccountStore
	writtenIDs []string
	opened     bool
	closed     bool
}

// NewTargetAccountWriter creates a chunk writer.
func NewTargetAccountWriter(store *TargetAccountStore) *TargetAccountWriter {
	if store == nil {
		store = NewTargetAccountStore()
	}
	return &TargetAccountWriter{store: store}
}

// Open records writer startup.
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

// Write commits a migrated chunk.
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

// Close records writer cleanup.
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

// Closed reports whether Close was called.
func (w *TargetAccountWriter) Closed() bool {
	return w != nil && w.closed
}

// WrittenIDs returns account IDs written by this writer.
func (w *TargetAccountWriter) WrittenIDs() []string {
	if w == nil {
		return nil
	}
	return slices.Clone(w.writtenIDs)
}

// RecordingCheckpointStore is a small replaceable in-memory checkpoint store.
type RecordingCheckpointStore struct {
	mu         sync.RWMutex
	values     map[string]any
	operations []CheckpointOperation
}

// NewRecordingCheckpointStore creates an empty checkpoint store.
func NewRecordingCheckpointStore() *RecordingCheckpointStore {
	return &RecordingCheckpointStore{values: make(map[string]any)}
}

// Load returns a checkpoint by key and records the load.
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

// Save stores a checkpoint by key and records the save.
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

// Operations returns checkpoint activity in call order.
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
