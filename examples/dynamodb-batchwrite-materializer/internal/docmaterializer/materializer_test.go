package docmaterializer

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bluetape4k/bluetape-go/dynamodb/batchwrite"
)

func TestPreviewShowsChunkingBoundary(t *testing.T) {
	preview, err := NewPreview("document-index", SampleEvents())
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.RequestCount != 30 {
		t.Fatalf("RequestCount = %d, want 30", preview.RequestCount)
	}
	if preview.ChunkCount != 2 {
		t.Fatalf("ChunkCount = %d, want 2", preview.ChunkCount)
	}
	if preview.ChunkLimit != batchwrite.MaxItemsPerBatch {
		t.Fatalf("ChunkLimit = %d, want %d", preview.ChunkLimit, batchwrite.MaxItemsPerBatch)
	}
}

func TestMaterializerChunksRequests(t *testing.T) {
	client := &fakeBatchClient{outputs: []*dynamodb.BatchWriteItemOutput{{}, {}}}
	m := newTestMaterializer(t, WithMaxAttempts(1))

	report, err := m.Write(context.Background(), client, SampleEvents())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if report.Processed != 30 || report.Attempts != 2 {
		t.Fatalf("report = %#v, want processed=30 attempts=2", report)
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	if got := countWriteRequests(client.calls[0]); got != batchwrite.MaxItemsPerBatch {
		t.Fatalf("first chunk size = %d, want %d", got, batchwrite.MaxItemsPerBatch)
	}
	if got := countWriteRequests(client.calls[1]); got != 5 {
		t.Fatalf("second chunk size = %d, want 5", got)
	}
}

func TestMaterializerRetriesUnprocessedItemsOnly(t *testing.T) {
	m := newTestMaterializer(t)
	requests, err := m.RequestItems(SampleEvents()[:3])
	if err != nil {
		t.Fatalf("RequestItems() error = %v", err)
	}
	unprocessed := map[string][]types.WriteRequest{
		m.Table: requests[m.Table][1:3],
	}
	client := &fakeBatchClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: unprocessed},
			{},
		},
	}

	report, err := m.Write(context.Background(), client, SampleEvents()[:3])
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if report.Processed != 3 || report.Attempts != 2 {
		t.Fatalf("report = %#v, want processed=3 attempts=2", report)
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	if got, want := requestIDs(client.calls[1][m.Table]), requestIDs(unprocessed[m.Table]); !reflect.DeepEqual(got, want) {
		t.Fatalf("retry request ids = %v, want %v", got, want)
	}
}

func TestMaterializerReportsRetryExhaustion(t *testing.T) {
	m := newTestMaterializer(t, WithMaxAttempts(2))
	requests, err := m.RequestItems(SampleEvents()[:3])
	if err != nil {
		t.Fatalf("RequestItems() error = %v", err)
	}
	client := &fakeBatchClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: map[string][]types.WriteRequest{m.Table: requests[m.Table]}},
			{UnprocessedItems: map[string][]types.WriteRequest{m.Table: requests[m.Table][1:3]}},
		},
	}

	report, err := m.Write(context.Background(), client, SampleEvents()[:3])
	if err == nil {
		t.Fatal("Write() error = nil, want retry exhaustion")
	}
	if !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("error = %v, want ErrRetryExhausted", err)
	}
	if !errors.Is(err, batchwrite.ErrUnprocessedItems) {
		t.Fatalf("error = %v, want batchwrite.ErrUnprocessedItems", err)
	}
	var unprocessed batchwrite.UnprocessedItemsError
	if !errors.As(err, &unprocessed) {
		t.Fatalf("error type = %T, want UnprocessedItemsError", err)
	}
	if !report.Exhausted || report.Remaining != 2 || report.Processed != 1 {
		t.Fatalf("report = %#v, want exhausted remaining=2 processed=1", report)
	}
}

func TestMaterializerPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := newTestMaterializer(t, WithMaxAttempts(2))
	client := &fakeBatchClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: map[string][]types.WriteRequest{m.Table: oneRequest(t, m)}},
		},
		afterCall: cancel,
	}

	_, err := m.Write(ctx, client, SampleEvents()[:1])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Write() error = %v, want context.Canceled", err)
	}
}

func TestMaterializerPreservesTypedAWSServiceError(t *testing.T) {
	cause := &types.ProvisionedThroughputExceededException{
		Message: stringPtr("write capacity exceeded"),
	}
	m := newTestMaterializer(t)
	client := &fakeBatchClient{err: cause}

	_, err := m.Write(context.Background(), client, SampleEvents()[:1])
	if err == nil {
		t.Fatal("Write() error = nil, want service error")
	}
	var typed *types.ProvisionedThroughputExceededException
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T %[1]v, want ProvisionedThroughputExceededException", err)
	}
	if errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("service error = %v, should not look like retry exhaustion", err)
	}
}

func TestMaterializerRejectsInvalidEvents(t *testing.T) {
	m := newTestMaterializer(t)
	_, err := m.Write(context.Background(), &fakeBatchClient{}, []DocumentEvent{{TenantID: "tenant", Version: 1, Title: "missing id", BodyHash: "hash"}})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Write() error = %v, want ErrInvalidEvent", err)
	}
}

func newTestMaterializer(t *testing.T, options ...Option) Materializer {
	t.Helper()
	m, err := NewMaterializer("document-index", options...)
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}
	return m
}

func oneRequest(t *testing.T, m Materializer) []types.WriteRequest {
	t.Helper()
	requests, err := m.RequestItems(SampleEvents()[:1])
	if err != nil {
		t.Fatalf("RequestItems() error = %v", err)
	}
	return requests[m.Table]
}

type fakeBatchClient struct {
	calls     []map[string][]types.WriteRequest
	outputs   []*dynamodb.BatchWriteItemOutput
	err       error
	afterCall func()
}

func (c *fakeBatchClient) BatchWriteItem(_ context.Context, input *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	c.calls = append(c.calls, cloneRequestItems(input.RequestItems))
	if c.afterCall != nil {
		c.afterCall()
	}
	if c.err != nil {
		return nil, c.err
	}
	if len(c.outputs) == 0 {
		return &dynamodb.BatchWriteItemOutput{}, nil
	}
	out := c.outputs[0]
	c.outputs = c.outputs[1:]
	return out, nil
}

func cloneRequestItems(requestItems map[string][]types.WriteRequest) map[string][]types.WriteRequest {
	if len(requestItems) == 0 {
		return nil
	}
	cloned := make(map[string][]types.WriteRequest, len(requestItems))
	for table, items := range requestItems {
		cloned[table] = append([]types.WriteRequest(nil), items...)
	}
	return cloned
}

func requestIDs(requests []types.WriteRequest) []string {
	ids := make([]string, 0, len(requests))
	for _, request := range requests {
		ids = append(ids, requestItemID(request))
	}
	return ids
}

func stringPtr(value string) *string {
	return &value
}
