package documentworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func TestPreviewShowsIntegratedBoundary(t *testing.T) {
	preview, err := NewPreview("documents", "document-events", "document_processing")
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.ObjectPrefix != "tenants/<tenant_id>/documents/<document_id>/<file_name>" {
		t.Fatalf("ObjectPrefix = %q", preview.ObjectPrefix)
	}
	if len(preview.Operations) != 5 || len(preview.IdempotencyRules) != 3 || len(preview.Prerequisites) != 3 {
		t.Fatalf("preview = %#v, want operations, rules, and prerequisites", preview)
	}
}

func TestSubmitStoresObjectAndSendsDocumentEvent(t *testing.T) {
	storage := newFakeStorage()
	queue := &fakeQueue{}
	workflow := newTestWorkflow(t)
	doc := SampleDocuments()[0]

	submitted, err := workflow.Submit(context.Background(), storage, queue, doc)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if submitted.ObjectKey != "tenants/tenant-alpha/documents/doc-1001/contract-1001.txt" {
		t.Fatalf("ObjectKey = %q", submitted.ObjectKey)
	}
	if submitted.IdempotencyKey != "tenant-alpha/doc-1001/process" {
		t.Fatalf("IdempotencyKey = %q", submitted.IdempotencyKey)
	}
	if storage.putInput == nil {
		t.Fatal("PutObject input = nil")
	}
	if got := aws.ToString(storage.putInput.Bucket); got != workflow.Bucket {
		t.Fatalf("bucket = %q", got)
	}
	if got := storage.putInput.Metadata["idempotency_key"]; got != submitted.IdempotencyKey {
		t.Fatalf("metadata idempotency_key = %q", got)
	}
	if queue.sendInput == nil {
		t.Fatal("SendMessage input = nil")
	}
	var event DocumentEvent
	if err := json.Unmarshal([]byte(aws.ToString(queue.sendInput.MessageBody)), &event); err != nil {
		t.Fatalf("event JSON: %v", err)
	}
	if event.ObjectKey != submitted.ObjectKey || event.IdempotencyKey != submitted.IdempotencyKey {
		t.Fatalf("event = %#v, submitted = %#v", event, submitted)
	}
	if got := aws.ToString(queue.sendInput.MessageAttributes["event-type"].StringValue); got != "document.submitted" {
		t.Fatalf("event-type attribute = %q", got)
	}
}

func TestProcessOnceDownloadsRecordsStateAndDeletesMessage(t *testing.T) {
	storage := newFakeStorage()
	queue := &fakeQueue{}
	state := &fakeState{}
	workflow := newTestWorkflow(t)
	submitted, err := workflow.Submit(context.Background(), storage, queue, SampleDocuments()[0])
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result, err := workflow.ProcessOnce(context.Background(), storage, queue, state)
	if err != nil {
		t.Fatalf("ProcessOnce() error = %v", err)
	}
	if !result.Acked || result.Duplicate || result.RetryVisible {
		t.Fatalf("ProcessOnce() = %#v, want success ack only", result)
	}
	if result.BytesRead != submitted.Size {
		t.Fatalf("BytesRead = %d, want %d", result.BytesRead, submitted.Size)
	}
	if !storage.lastBody.closed {
		t.Fatal("S3 GetObject body was not closed")
	}
	if state.putInput == nil {
		t.Fatal("PutItem input = nil")
	}
	if got := deref(state.putInput.ConditionExpression); got != "attribute_not_exists(#pk) AND attribute_not_exists(#sk)" {
		t.Fatalf("ConditionExpression = %q", got)
	}
	if got := stringValue(state.putInput.Item["pk"]); got != "TENANT#tenant-alpha" {
		t.Fatalf("pk = %q", got)
	}
	if got := stringValue(state.putInput.Item["sk"]); got != "DOCUMENT#doc-1001#PROCESSING" {
		t.Fatalf("sk = %q", got)
	}
	if queue.deleteInput == nil {
		t.Fatal("DeleteMessage input = nil")
	}
	if queue.visibilityInput != nil {
		t.Fatal("ChangeMessageVisibility should not run after success")
	}
}

func TestProcessOnceAcksDuplicateConditionalConflict(t *testing.T) {
	storage := newFakeStorage()
	queue := &fakeQueue{}
	state := &fakeState{putErr: &types.ConditionalCheckFailedException{Message: aws.String("already processed")}}
	workflow := newTestWorkflow(t)
	if _, err := workflow.Submit(context.Background(), storage, queue, SampleDocuments()[0]); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result, err := workflow.ProcessOnce(context.Background(), storage, queue, state)
	if err != nil {
		t.Fatalf("ProcessOnce() duplicate error = %v", err)
	}
	if !result.Acked || !result.Duplicate || result.RetryVisible {
		t.Fatalf("ProcessOnce() = %#v, want duplicate ack", result)
	}
	if queue.deleteInput == nil {
		t.Fatal("DeleteMessage input = nil for duplicate")
	}
	if queue.visibilityInput != nil {
		t.Fatal("duplicate should not be made visible for retry")
	}
}

func TestProcessOnceMakesTransientFailureVisibleForRetry(t *testing.T) {
	storage := newFakeStorage()
	queue := &fakeQueue{}
	state := &fakeState{putErr: errors.New("dynamodb timeout")}
	workflow := newTestWorkflow(t)
	if _, err := workflow.Submit(context.Background(), storage, queue, SampleDocuments()[0]); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result, err := workflow.ProcessOnce(context.Background(), storage, queue, state)
	if err == nil {
		t.Fatal("ProcessOnce() error = nil, want transient failure")
	}
	if result.Acked || !result.RetryVisible || result.VisibilitySeconds != workflow.FailureVisibleSeconds {
		t.Fatalf("ProcessOnce() = %#v, want retry visible", result)
	}
	if queue.deleteInput != nil {
		t.Fatal("DeleteMessage should not run after transient failure")
	}
	if queue.visibilityInput == nil {
		t.Fatal("ChangeMessageVisibility input = nil")
	}
}

func TestProcessOnceRejectsForgedEventConsistency(t *testing.T) {
	storage := newFakeStorage()
	queue := &fakeQueue{}
	state := &fakeState{}
	workflow := newTestWorkflow(t)
	event := DocumentEvent{
		TenantID:       "tenant-alpha",
		DocumentID:     "doc-1001",
		FileName:       "contract-1001.txt",
		ObjectKey:      "tenants/tenant-alpha/documents/doc-1002/claim-1002.txt",
		ContentType:    "text/plain; charset=utf-8",
		IdempotencyKey: "tenant-alpha/doc-1002/process",
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	queue.messages = append(queue.messages, sqstypes.Message{
		MessageId:     aws.String("msg-forged"),
		ReceiptHandle: aws.String("receipt-forged"),
		Body:          aws.String(string(body)),
	})

	result, err := workflow.ProcessOnce(context.Background(), storage, queue, state)
	if !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("ProcessOnce() error = %v, want ErrInvalidDocument", err)
	}
	if result.Acked || !result.RetryVisible {
		t.Fatalf("ProcessOnce() = %#v, want retry-visible invalid message", result)
	}
	if storage.getInput != nil || state.putInput != nil {
		t.Fatal("forged event should not read S3 or write DynamoDB")
	}
	if queue.deleteInput != nil || queue.visibilityInput == nil {
		t.Fatal("forged event should change visibility without deleting the message")
	}
}

func TestProcessOnceMapsNoMessages(t *testing.T) {
	workflow := newTestWorkflow(t)

	result, err := workflow.ProcessOnce(context.Background(), newFakeStorage(), &fakeQueue{}, &fakeState{})
	if !errors.Is(err, ErrNoMessages) || !result.NoMessage {
		t.Fatalf("ProcessOnce() result=%#v error=%v, want ErrNoMessages", result, err)
	}
}

func TestSubmitPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := newFakeStorage()
	queue := &fakeQueue{}
	workflow := newTestWorkflow(t)

	_, err := workflow.Submit(ctx, storage, queue, SampleDocuments()[0])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit() error = %v, want context.Canceled", err)
	}
	if storage.putInput != nil || queue.sendInput != nil {
		t.Fatal("AWS clients should not be called after context cancellation")
	}
}

func TestSubmitRejectsUnsafeObjectKeys(t *testing.T) {
	workflow := newTestWorkflow(t)

	_, err := workflow.Submit(context.Background(), newFakeStorage(), &fakeQueue{}, DocumentUpload{
		TenantID:   "tenant-alpha",
		DocumentID: "doc-1001",
		FileName:   "../secret.txt",
		Body:       []byte("secret"),
	})
	if !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("Submit() error = %v, want ErrInvalidDocument", err)
	}
}

func newTestWorkflow(t *testing.T) Workflow {
	t.Helper()
	workflow, err := NewWorkflow("documents", "https://sqs.local/000000000000/document-events", "document_processing", 1, 5, 0)
	if err != nil {
		t.Fatalf("NewWorkflow() error = %v", err)
	}
	return workflow
}

type fakeStorage struct {
	putInput *s3.PutObjectInput
	getInput *s3.GetObjectInput
	putErr   error
	getErr   error
	objects  map[string][]byte
	lastBody *trackingReadCloser
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: make(map[string][]byte)}
}

func (s *fakeStorage) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	s.putInput = input
	if s.putErr != nil {
		return nil, s.putErr
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	s.objects[aws.ToString(input.Key)] = body
	return &s3.PutObjectOutput{}, nil
}

func (s *fakeStorage) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s.getInput = input
	if s.getErr != nil {
		return nil, s.getErr
	}
	body := s.objects[aws.ToString(input.Key)]
	s.lastBody = &trackingReadCloser{Reader: bytes.NewReader(body)}
	return &s3.GetObjectOutput{Body: s.lastBody}, nil
}

type fakeQueue struct {
	sendInput       *sqs.SendMessageInput
	receiveInput    *sqs.ReceiveMessageInput
	deleteInput     *sqs.DeleteMessageInput
	visibilityInput *sqs.ChangeMessageVisibilityInput
	sendErr         error
	receiveErr      error
	deleteErr       error
	visibilityErr   error
	messages        []sqstypes.Message
}

func (q *fakeQueue) SendMessage(_ context.Context, input *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	q.sendInput = input
	if q.sendErr != nil {
		return nil, q.sendErr
	}
	q.messages = append(q.messages, sqstypes.Message{
		MessageId:     aws.String("msg-1"),
		ReceiptHandle: aws.String("receipt-1"),
		Body:          input.MessageBody,
	})
	return &sqs.SendMessageOutput{MessageId: aws.String("msg-1")}, nil
}

func (q *fakeQueue) ReceiveMessage(_ context.Context, input *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	q.receiveInput = input
	if q.receiveErr != nil {
		return nil, q.receiveErr
	}
	if len(q.messages) == 0 {
		return &sqs.ReceiveMessageOutput{}, nil
	}
	return &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{q.messages[0]}}, nil
}

func (q *fakeQueue) DeleteMessage(_ context.Context, input *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	q.deleteInput = input
	if q.deleteErr != nil {
		return nil, q.deleteErr
	}
	q.messages = nil
	return &sqs.DeleteMessageOutput{}, nil
}

func (q *fakeQueue) ChangeMessageVisibility(_ context.Context, input *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	q.visibilityInput = input
	if q.visibilityErr != nil {
		return nil, q.visibilityErr
	}
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

type fakeState struct {
	putInput *dynamodb.PutItemInput
	putErr   error
}

func (s *fakeState) PutItem(_ context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	s.putInput = input
	if s.putErr != nil {
		return nil, s.putErr
	}
	return &dynamodb.PutItemOutput{}, nil
}

type trackingReadCloser struct {
	*bytes.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringValue(value types.AttributeValue) string {
	attr, ok := value.(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return attr.Value
}

func TestSampleDocumentsStable(t *testing.T) {
	want := []DocumentUpload{
		{TenantID: "tenant-alpha", DocumentID: "doc-1001", FileName: "contract-1001.txt", Body: []byte("contract 1001\n"), Metadata: map[string]string{"source": "portal"}},
		{TenantID: "tenant-alpha", DocumentID: "doc-1002", FileName: "claim-1002.txt", Body: []byte("claim 1002\n"), Metadata: map[string]string{"source": "batch"}},
	}
	if got := SampleDocuments(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SampleDocuments() = %#v, want %#v", got, want)
	}
}
