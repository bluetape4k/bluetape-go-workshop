// Package documentworkflow demonstrates a small S3, SQS, and DynamoDB workflow.
package documentworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

const (
	defaultWaitTimeSeconds       int32 = 2
	defaultVisibilitySeconds     int32 = 30
	defaultFailureVisibleSeconds int32 = 0
)

var (
	// ErrInvalidDocument reports an unsafe or incomplete document workflow request.
	ErrInvalidDocument = errors.New("documentworkflow: invalid document")
	// ErrNoMessages reports that SQS returned no visible messages.
	ErrNoMessages = errors.New("documentworkflow: no messages")
	// ErrConditionalConflict reports a duplicate DynamoDB processing record.
	ErrConditionalConflict = errors.New("documentworkflow: conditional conflict")
)

// StorageClient is the narrow S3 surface used by the workflow.
type StorageClient interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// QueueClient is the narrow SQS surface used by the workflow.
type QueueClient interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

// StateClient is the narrow DynamoDB surface used by the workflow.
type StateClient interface {
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

// Workflow owns resource names, key conventions, and retry policy.
type Workflow struct {
	Bucket                string
	QueueURL              string
	Table                 string
	WaitTimeSeconds       int32
	VisibilitySeconds     int32
	FailureVisibleSeconds int32
}

// DocumentUpload is the caller-owned document ingestion command.
type DocumentUpload struct {
	TenantID   string            `json:"tenant_id"`
	DocumentID string            `json:"document_id"`
	FileName   string            `json:"file_name"`
	Body       []byte            `json:"body,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// DocumentEvent is the SQS message payload created after S3 persistence.
type DocumentEvent struct {
	TenantID       string `json:"tenant_id"`
	DocumentID     string `json:"document_id"`
	FileName       string `json:"file_name"`
	ObjectKey      string `json:"object_key"`
	ContentType    string `json:"content_type"`
	IdempotencyKey string `json:"idempotency_key"`
}

// SubmittedDocument describes the stored object and queued processing event.
type SubmittedDocument struct {
	TenantID       string `json:"tenant_id"`
	DocumentID     string `json:"document_id"`
	ObjectKey      string `json:"object_key"`
	ContentType    string `json:"content_type"`
	Size           int64  `json:"size"`
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// ProcessResult describes one SQS processing attempt.
type ProcessResult struct {
	Event             DocumentEvent `json:"event,omitempty"`
	MessageID         string        `json:"message_id,omitempty"`
	ReceiptHandle     string        `json:"receipt_handle,omitempty"`
	BytesRead         int64         `json:"bytes_read,omitempty"`
	Acked             bool          `json:"acked"`
	Duplicate         bool          `json:"duplicate"`
	RetryVisible      bool          `json:"retry_visible"`
	Failure           string        `json:"failure,omitempty"`
	NoMessage         bool          `json:"no_message"`
	VisibilitySeconds int32         `json:"visibility_seconds,omitempty"`
}

// Preview describes the example contract without contacting AWS services.
type Preview struct {
	Bucket           string   `json:"bucket"`
	QueueName        string   `json:"queue_name"`
	Table            string   `json:"table"`
	ObjectPrefix     string   `json:"object_prefix"`
	StateKey         string   `json:"state_key"`
	Operations       []string `json:"operations"`
	IdempotencyRules []string `json:"idempotency_rules"`
	Prerequisites    []string `json:"prerequisites"`
	SmokeTest        string   `json:"smoke_test"`
}

// NewWorkflow creates a document workflow with bounded receive defaults.
func NewWorkflow(bucket, queueURL, table string, waitTimeSeconds, visibilitySeconds, failureVisibleSeconds int32) (Workflow, error) {
	switch {
	case bucket == "":
		return Workflow{}, fmt.Errorf("%w: bucket is required", ErrInvalidDocument)
	case queueURL == "":
		return Workflow{}, fmt.Errorf("%w: queue_url is required", ErrInvalidDocument)
	case table == "":
		return Workflow{}, fmt.Errorf("%w: table is required", ErrInvalidDocument)
	}
	if waitTimeSeconds <= 0 {
		waitTimeSeconds = defaultWaitTimeSeconds
	}
	if visibilitySeconds <= 0 {
		visibilitySeconds = defaultVisibilitySeconds
	}
	if failureVisibleSeconds < 0 {
		failureVisibleSeconds = defaultFailureVisibleSeconds
	}
	return Workflow{
		Bucket:                bucket,
		QueueURL:              queueURL,
		Table:                 table,
		WaitTimeSeconds:       waitTimeSeconds,
		VisibilitySeconds:     visibilitySeconds,
		FailureVisibleSeconds: failureVisibleSeconds,
	}, nil
}

// NewPreview builds an inspectable local preview for README and go run output.
func NewPreview(bucket, queueName, table string) (Preview, error) {
	if _, err := NewWorkflow(bucket, "https://sqs.local/000000000000/"+queueName, table, 2, 30, 0); err != nil {
		return Preview{}, err
	}
	return Preview{
		Bucket:       bucket,
		QueueName:    queueName,
		Table:        table,
		ObjectPrefix: "tenants/<tenant_id>/documents/<document_id>/<file_name>",
		StateKey:     "pk=TENANT#<tenant_id>, sk=DOCUMENT#<document_id>#PROCESSING",
		Operations: []string{
			"Submit stores the document body in S3 before publishing the SQS event",
			"Submit sends an SQS JSON event with content-type, event-type, and idempotency-key attributes",
			"ProcessOnce receives one message, downloads the S3 object, and closes the body",
			"ProcessOnce records processing state with a DynamoDB conditional PutItem",
			"Success and duplicate events delete the SQS message; transient failures become visible for retry",
		},
		IdempotencyRules: []string{
			"DynamoDB uses attribute_not_exists(pk) AND attribute_not_exists(sk)",
			"ConditionalCheckFailedException becomes ErrConditionalConflict and is acknowledged as duplicate work",
			"S3 or DynamoDB infrastructure errors do not delete the SQS message",
		},
		Prerequisites: []string{
			"#59 S3 Floci storage",
			"#60 SQS Floci worker",
			"#61 DynamoDB conditional repository",
		},
		SmokeTest: "BLUETAPE_DOCUMENT_WORKFLOW_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...",
	}, nil
}

// Submit persists the document object, then publishes one SQS processing event.
func (w Workflow) Submit(ctx context.Context, storage StorageClient, queue QueueClient, doc DocumentUpload) (SubmittedDocument, error) {
	if err := ctx.Err(); err != nil {
		return SubmittedDocument{}, err
	}
	if err := w.validate(); err != nil {
		return SubmittedDocument{}, err
	}
	if err := validateDocument(doc); err != nil {
		return SubmittedDocument{}, err
	}
	event := DocumentEvent{
		TenantID:       doc.TenantID,
		DocumentID:     doc.DocumentID,
		FileName:       path.Base(doc.FileName),
		ObjectKey:      objectKey(doc.TenantID, doc.DocumentID, doc.FileName),
		ContentType:    contentTypeForKey(doc.FileName, doc.Body),
		IdempotencyKey: idempotencyKey(doc.TenantID, doc.DocumentID),
	}
	_, err := storage.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(w.Bucket),
		Key:           aws.String(event.ObjectKey),
		Body:          bytes.NewReader(doc.Body),
		ContentLength: aws.Int64(int64(len(doc.Body))),
		ContentType:   aws.String(event.ContentType),
		Metadata:      objectMetadata(doc, event.IdempotencyKey),
	})
	if err != nil {
		return SubmittedDocument{}, fmt.Errorf("store document %s: %w", event.ObjectKey, err)
	}
	body, err := json.Marshal(event)
	if err != nil {
		return SubmittedDocument{}, fmt.Errorf("marshal document event %s: %w", event.IdempotencyKey, err)
	}
	out, err := queue.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(w.QueueURL),
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]sqstypes.MessageAttributeValue{
			"content-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("application/json"),
			},
			"event-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("document.submitted"),
			},
			"idempotency-key": {
				DataType:    aws.String("String"),
				StringValue: aws.String(event.IdempotencyKey),
			},
		},
	})
	if err != nil {
		return SubmittedDocument{}, fmt.Errorf("send document event %s: %w", event.IdempotencyKey, err)
	}
	return SubmittedDocument{
		TenantID:       event.TenantID,
		DocumentID:     event.DocumentID,
		ObjectKey:      event.ObjectKey,
		ContentType:    event.ContentType,
		Size:           int64(len(doc.Body)),
		MessageID:      aws.ToString(out.MessageId),
		IdempotencyKey: event.IdempotencyKey,
	}, nil
}

// ProcessOnce receives at most one SQS message and acknowledges only terminal outcomes.
func (w Workflow) ProcessOnce(ctx context.Context, storage StorageClient, queue QueueClient, state StateClient) (ProcessResult, error) {
	if err := ctx.Err(); err != nil {
		return ProcessResult{}, err
	}
	if err := w.validate(); err != nil {
		return ProcessResult{}, err
	}
	out, err := queue.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(w.QueueURL),
		MaxNumberOfMessages:   1,
		WaitTimeSeconds:       w.WaitTimeSeconds,
		VisibilityTimeout:     w.VisibilitySeconds,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return ProcessResult{}, fmt.Errorf("receive document event: %w", err)
	}
	if len(out.Messages) == 0 {
		return ProcessResult{NoMessage: true}, ErrNoMessages
	}
	message := out.Messages[0]
	result := ProcessResult{
		MessageID:     aws.ToString(message.MessageId),
		ReceiptHandle: aws.ToString(message.ReceiptHandle),
	}
	event, err := decodeEvent(message)
	if err != nil {
		return w.retry(ctx, queue, result, err)
	}
	result.Event = event
	size, err := w.readObjectSize(ctx, storage, event)
	if err != nil {
		return w.retry(ctx, queue, result, err)
	}
	result.BytesRead = size
	if err := w.recordProcessed(ctx, state, event, size); err != nil {
		if errors.Is(err, ErrConditionalConflict) {
			if ackErr := w.ack(ctx, queue, result.ReceiptHandle); ackErr != nil {
				return result, ackErr
			}
			result.Acked = true
			result.Duplicate = true
			return result, nil
		}
		return w.retry(ctx, queue, result, err)
	}
	if err := w.ack(ctx, queue, result.ReceiptHandle); err != nil {
		return result, err
	}
	result.Acked = true
	return result, nil
}

// SampleDocuments returns enough scenario data for preview and tests.
func SampleDocuments() []DocumentUpload {
	return []DocumentUpload{
		{TenantID: "tenant-alpha", DocumentID: "doc-1001", FileName: "contract-1001.txt", Body: []byte("contract 1001\n"), Metadata: map[string]string{"source": "portal"}},
		{TenantID: "tenant-alpha", DocumentID: "doc-1002", FileName: "claim-1002.txt", Body: []byte("claim 1002\n"), Metadata: map[string]string{"source": "batch"}},
	}
}

func (w Workflow) validate() error {
	switch {
	case w.Bucket == "":
		return fmt.Errorf("%w: bucket is required", ErrInvalidDocument)
	case w.QueueURL == "":
		return fmt.Errorf("%w: queue_url is required", ErrInvalidDocument)
	case w.Table == "":
		return fmt.Errorf("%w: table is required", ErrInvalidDocument)
	default:
		return nil
	}
}

func validateDocument(doc DocumentUpload) error {
	if err := validateSegment("tenant_id", doc.TenantID); err != nil {
		return err
	}
	if err := validateSegment("document_id", doc.DocumentID); err != nil {
		return err
	}
	if err := validateFileName(doc.FileName); err != nil {
		return err
	}
	if len(doc.Body) == 0 {
		return fmt.Errorf("%w: body is required", ErrInvalidDocument)
	}
	return nil
}

func decodeEvent(message sqstypes.Message) (DocumentEvent, error) {
	var event DocumentEvent
	body := aws.ToString(message.Body)
	if body == "" {
		return event, fmt.Errorf("%w: message body is empty", ErrInvalidDocument)
	}
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		return event, fmt.Errorf("%w: decode message body: %w", ErrInvalidDocument, err)
	}
	if err := validateSegment("tenant_id", event.TenantID); err != nil {
		return event, err
	}
	if err := validateSegment("document_id", event.DocumentID); err != nil {
		return event, err
	}
	if err := validateFileName(event.FileName); err != nil {
		return event, err
	}
	if event.ObjectKey == "" || event.ContentType == "" || event.IdempotencyKey == "" {
		return event, fmt.Errorf("%w: object_key, content_type, and idempotency_key are required", ErrInvalidDocument)
	}
	if want := objectKey(event.TenantID, event.DocumentID, event.FileName); event.ObjectKey != want {
		return event, fmt.Errorf("%w: object_key %q does not match %q", ErrInvalidDocument, event.ObjectKey, want)
	}
	if want := idempotencyKey(event.TenantID, event.DocumentID); event.IdempotencyKey != want {
		return event, fmt.Errorf("%w: idempotency_key %q does not match %q", ErrInvalidDocument, event.IdempotencyKey, want)
	}
	return event, nil
}

func (w Workflow) readObjectSize(ctx context.Context, storage StorageClient, event DocumentEvent) (int64, error) {
	output, err := storage.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(w.Bucket),
		Key:    aws.String(event.ObjectKey),
	})
	if err != nil {
		return 0, fmt.Errorf("download document %s: %w", event.ObjectKey, err)
	}
	if output.Body == nil {
		return 0, fmt.Errorf("%w: object body is empty", ErrInvalidDocument)
	}
	defer func() {
		_ = output.Body.Close()
	}()
	payload, err := io.ReadAll(output.Body)
	if err != nil {
		return 0, fmt.Errorf("read document %s: %w", event.ObjectKey, err)
	}
	return int64(len(payload)), nil
}

func (w Workflow) recordProcessed(ctx context.Context, state StateClient, event DocumentEvent, size int64) error {
	_, err := state.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(w.Table),
		Item: map[string]types.AttributeValue{
			"pk":              &types.AttributeValueMemberS{Value: tenantKey(event.TenantID)},
			"sk":              &types.AttributeValueMemberS{Value: processingKey(event.DocumentID)},
			"tenant_id":       &types.AttributeValueMemberS{Value: event.TenantID},
			"document_id":     &types.AttributeValueMemberS{Value: event.DocumentID},
			"object_key":      &types.AttributeValueMemberS{Value: event.ObjectKey},
			"content_type":    &types.AttributeValueMemberS{Value: event.ContentType},
			"idempotency_key": &types.AttributeValueMemberS{Value: event.IdempotencyKey},
			"status":          &types.AttributeValueMemberS{Value: "PROCESSED"},
			"bytes_read":      &types.AttributeValueMemberN{Value: strconv.FormatInt(size, 10)},
		},
		ConditionExpression: aws.String("attribute_not_exists(#pk) AND attribute_not_exists(#sk)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
	})
	if err == nil {
		return nil
	}
	if isConditionalConflict(err) {
		return fmt.Errorf("%w: document %s/%s: %w", ErrConditionalConflict, event.TenantID, event.DocumentID, err)
	}
	return fmt.Errorf("record document processing %s/%s: %w", event.TenantID, event.DocumentID, err)
}

func (w Workflow) ack(ctx context.Context, queue QueueClient, receiptHandle string) error {
	if receiptHandle == "" {
		return fmt.Errorf("%w: receipt handle is required for ack", ErrInvalidDocument)
	}
	_, err := queue.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(w.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete document event: %w", err)
	}
	return nil
}

func (w Workflow) retry(ctx context.Context, queue QueueClient, result ProcessResult, cause error) (ProcessResult, error) {
	if result.ReceiptHandle == "" {
		return result, cause
	}
	_, err := queue.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(w.QueueURL),
		ReceiptHandle:     aws.String(result.ReceiptHandle),
		VisibilityTimeout: w.FailureVisibleSeconds,
	})
	if err != nil {
		return result, fmt.Errorf("change failed document visibility: %w", err)
	}
	result.Failure = cause.Error()
	result.RetryVisible = true
	result.VisibilitySeconds = w.FailureVisibleSeconds
	return result, cause
}

func validateSegment(name, value string) error {
	if value == "" || strings.Contains(value, "/") || strings.Contains(value, "..") {
		return fmt.Errorf("%w: %s must be a non-empty safe key segment", ErrInvalidDocument, name)
	}
	return nil
}

func validateFileName(fileName string) error {
	if fileName == "" || path.Base(fileName) != fileName || strings.Contains(fileName, "..") {
		return fmt.Errorf("%w: file_name must be a safe base name", ErrInvalidDocument)
	}
	return nil
}

func objectKey(tenantID, documentID, fileName string) string {
	return path.Join("tenants", tenantID, "documents", documentID, path.Base(fileName))
}

func idempotencyKey(tenantID, documentID string) string {
	return tenantID + "/" + documentID + "/process"
}

func tenantKey(tenantID string) string {
	return "TENANT#" + tenantID
}

func processingKey(documentID string) string {
	return "DOCUMENT#" + documentID + "#PROCESSING"
}

func objectMetadata(doc DocumentUpload, idempotencyKey string) map[string]string {
	metadata := make(map[string]string, len(doc.Metadata)+3)
	for key, value := range doc.Metadata {
		metadata[key] = value
	}
	metadata["tenant_id"] = doc.TenantID
	metadata["document_id"] = doc.DocumentID
	metadata["idempotency_key"] = idempotencyKey
	return metadata
}

func contentTypeForKey(key string, sample []byte) string {
	if typ := mime.TypeByExtension(filepath.Ext(key)); typ != "" {
		return typ
	}
	if len(sample) == 0 {
		return "application/octet-stream"
	}
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return http.DetectContentType(sample)
}

func isConditionalConflict(err error) bool {
	var conflict *types.ConditionalCheckFailedException
	return errors.As(err, &conflict)
}
