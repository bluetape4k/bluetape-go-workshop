// Package docmaterializer demonstrates DynamoDB batch-write materialization.
package docmaterializer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bluetape4k/bluetape-go/dynamodb/batchwrite"
)

var (
	// ErrInvalidEvent reports a document event that cannot be indexed.
	ErrInvalidEvent = errors.New("docmaterializer: invalid event")
	// ErrRetryExhausted reports that DynamoDB returned unprocessed items after retry budget exhaustion.
	ErrRetryExhausted = errors.New("docmaterializer: batch write retry exhausted")
)

// DocumentEvent is the application event projected into the search index table.
type DocumentEvent struct {
	DocumentID string `json:"document_id"`
	TenantID   string `json:"tenant_id"`
	Version    int    `json:"version"`
	Title      string `json:"title"`
	BodyHash   string `json:"body_hash"`
}

// Preview describes the write shape without contacting DynamoDB.
type Preview struct {
	Table          string   `json:"table"`
	EventCount     int      `json:"event_count"`
	RequestCount   int      `json:"request_count"`
	ChunkCount     int      `json:"chunk_count"`
	ChunkLimit     int      `json:"chunk_limit"`
	RetryBudget    int      `json:"retry_budget"`
	BoundaryNotes  []string `json:"boundary_notes"`
	SmokeTest      string   `json:"smoke_test"`
	ExampleEventID string   `json:"example_event_id"`
}

// WriteReport summarizes one materialization run.
type WriteReport struct {
	Table     string `json:"table"`
	Submitted int    `json:"submitted"`
	Attempts  int    `json:"attempts"`
	Processed int    `json:"processed"`
	Remaining int    `json:"remaining,omitempty"`
	Exhausted bool   `json:"exhausted,omitempty"`
}

// Materializer writes document index projections to DynamoDB.
type Materializer struct {
	Table       string
	MaxAttempts int
	Backoff     batchwrite.Backoff
}

// NewMaterializer creates a DynamoDB document materializer.
func NewMaterializer(table string, options ...Option) (Materializer, error) {
	m := Materializer{
		Table:       table,
		MaxAttempts: batchwrite.DefaultMaxAttempts,
		Backoff: func(int) time.Duration {
			return 0
		},
	}
	for _, option := range options {
		if option != nil {
			option(&m)
		}
	}
	if m.Table == "" {
		return Materializer{}, fmt.Errorf("%w: table is required", ErrInvalidEvent)
	}
	if m.MaxAttempts <= 0 {
		return Materializer{}, batchwrite.ErrInvalidMaxAttempts
	}
	if m.Backoff == nil {
		m.Backoff = func(int) time.Duration {
			return 0
		}
	}
	return m, nil
}

// Option configures a Materializer.
type Option func(*Materializer)

// WithMaxAttempts sets the WriteAll retry budget.
func WithMaxAttempts(maxAttempts int) Option {
	return func(m *Materializer) {
		m.MaxAttempts = maxAttempts
	}
}

// WithBackoff sets the delay before WriteAll retries unprocessed items.
func WithBackoff(backoff batchwrite.Backoff) Option {
	return func(m *Materializer) {
		m.Backoff = backoff
	}
}

// NewPreview builds an inspectable local preview for README and go run output.
func NewPreview(table string, events []DocumentEvent) (Preview, error) {
	m, err := NewMaterializer(table)
	if err != nil {
		return Preview{}, err
	}
	requests, err := m.RequestItems(events)
	if err != nil {
		return Preview{}, err
	}
	count := countWriteRequests(requests)
	exampleID := ""
	if len(events) > 0 {
		exampleID = events[0].DocumentID
	}
	return Preview{
		Table:        table,
		EventCount:   len(events),
		RequestCount: count,
		ChunkCount:   int(math.Ceil(float64(count) / float64(batchwrite.MaxItemsPerBatch))),
		ChunkLimit:   batchwrite.MaxItemsPerBatch,
		RetryBudget:  m.MaxAttempts,
		BoundaryNotes: []string{
			"application maps domain events into DynamoDB WriteRequest values",
			"batchwrite.WriteAll owns 25-item chunking and UnprocessedItems retry",
			"retry exhaustion is separated from typed AWS service errors",
			"Floci smoke tests stay opt-in so normal CI uses deterministic fakes",
		},
		SmokeTest:      "BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/dynamodb-batchwrite-materializer/...",
		ExampleEventID: exampleID,
	}, nil
}

// Write writes all document index projections through batchwrite.WriteAll.
func (m Materializer) Write(ctx context.Context, client batchwrite.Client, events []DocumentEvent) (WriteReport, error) {
	requests, err := m.RequestItems(events)
	if err != nil {
		return WriteReport{}, err
	}
	submitted := countWriteRequests(requests)
	result, err := batchwrite.WriteAll(ctx, client, requests,
		batchwrite.WithMaxAttempts(m.MaxAttempts),
		batchwrite.WithBackoff(m.Backoff),
		batchwrite.WithReturnConsumedCapacity(types.ReturnConsumedCapacityTotal),
	)
	report := WriteReport{
		Table:     m.Table,
		Submitted: submitted,
		Attempts:  result.Attempts,
		Processed: result.Processed,
	}
	if err == nil {
		return report, nil
	}

	var unprocessed batchwrite.UnprocessedItemsError
	if errors.As(err, &unprocessed) {
		report.Exhausted = true
		report.Remaining = countWriteRequests(unprocessed.UnprocessedItems)
		return report, fmt.Errorf("%w: %w", ErrRetryExhausted, err)
	}
	return report, fmt.Errorf("write document index to DynamoDB: %w", err)
}

// RequestItems maps domain events to the AWS SDK request shape.
func (m Materializer) RequestItems(events []DocumentEvent) (map[string][]types.WriteRequest, error) {
	if m.Table == "" {
		return nil, fmt.Errorf("%w: table is required", ErrInvalidEvent)
	}
	if len(events) == 0 {
		return nil, batchwrite.ErrEmptyRequestItems
	}

	requests := make([]types.WriteRequest, 0, len(events))
	for _, event := range events {
		if err := validateEvent(event); err != nil {
			return nil, err
		}
		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: map[string]types.AttributeValue{
					"pk":          &types.AttributeValueMemberS{Value: "TENANT#" + event.TenantID},
					"sk":          &types.AttributeValueMemberS{Value: fmt.Sprintf("DOC#%s#v%06d", event.DocumentID, event.Version)},
					"document_id": &types.AttributeValueMemberS{Value: event.DocumentID},
					"tenant_id":   &types.AttributeValueMemberS{Value: event.TenantID},
					"version":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", event.Version)},
					"title":       &types.AttributeValueMemberS{Value: event.Title},
					"body_hash":   &types.AttributeValueMemberS{Value: event.BodyHash},
				},
			},
		})
	}
	return map[string][]types.WriteRequest{m.Table: requests}, nil
}

// SampleEvents returns enough events to demonstrate DynamoDB 25-item chunking.
func SampleEvents() []DocumentEvent {
	events := make([]DocumentEvent, 0, 30)
	for i := 1; i <= 30; i++ {
		events = append(events, DocumentEvent{
			DocumentID: fmt.Sprintf("doc-%03d", i),
			TenantID:   "tenant-alpha",
			Version:    i,
			Title:      fmt.Sprintf("Document %03d", i),
			BodyHash:   fmt.Sprintf("sha256:%064d", i),
		})
	}
	return events
}

func validateEvent(event DocumentEvent) error {
	switch {
	case event.DocumentID == "":
		return fmt.Errorf("%w: document_id is required", ErrInvalidEvent)
	case event.TenantID == "":
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidEvent)
	case event.Version <= 0:
		return fmt.Errorf("%w: version must be positive", ErrInvalidEvent)
	case event.Title == "":
		return fmt.Errorf("%w: title is required", ErrInvalidEvent)
	case event.BodyHash == "":
		return fmt.Errorf("%w: body_hash is required", ErrInvalidEvent)
	default:
		return nil
	}
}

func countWriteRequests(requestItems map[string][]types.WriteRequest) int {
	var count int
	for _, items := range requestItems {
		count += len(items)
	}
	return count
}

func requestItemID(request types.WriteRequest) string {
	if request.PutRequest == nil {
		return ""
	}
	value, ok := request.PutRequest.Item["document_id"].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return value.Value
}
