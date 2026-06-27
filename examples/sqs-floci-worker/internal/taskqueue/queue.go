// Package taskqueue demonstrates a small SQS-backed worker boundary.
package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

const (
	defaultWaitTimeSeconds       int32 = 2
	defaultVisibilitySeconds     int32 = 30
	defaultFailureVisibleSeconds int32 = 0
)

var (
	// ErrInvalidTask reports an unsafe or incomplete task message.
	ErrInvalidTask = errors.New("taskqueue: invalid task")
	// ErrNoMessages reports that a poll returned no visible SQS messages.
	ErrNoMessages = errors.New("taskqueue: no messages")
)

// Client is the narrow SQS client surface needed by Queue.
type Client interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

// Queue owns the queue URL and worker receive policy.
type Queue struct {
	QueueURL              string
	WaitTimeSeconds       int32
	VisibilitySeconds     int32
	FailureVisibleSeconds int32
}

// FulfillmentTask is the application-owned work payload.
type FulfillmentTask struct {
	TenantID       string `json:"tenant_id"`
	OrderID        string `json:"order_id"`
	Step           string `json:"step"`
	IdempotencyKey string `json:"idempotency_key"`
}

// EnqueuedTask describes a sent SQS message.
type EnqueuedTask struct {
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// WorkResult describes one worker poll attempt.
type WorkResult struct {
	Task              FulfillmentTask `json:"task,omitempty"`
	MessageID         string          `json:"message_id,omitempty"`
	ReceiptHandle     string          `json:"receipt_handle,omitempty"`
	Acked             bool            `json:"acked"`
	RetryVisible      bool            `json:"retry_visible"`
	Failure           string          `json:"failure,omitempty"`
	NoMessage         bool            `json:"no_message"`
	VisibilitySeconds int32           `json:"visibility_seconds,omitempty"`
}

// Preview describes the example contract without contacting SQS.
type Preview struct {
	QueueName         string   `json:"queue_name"`
	Operations        []string `json:"operations"`
	DeliverySemantics []string `json:"delivery_semantics"`
	SmokeTest         string   `json:"smoke_test"`
}

// Handler processes one decoded task.
type Handler func(context.Context, FulfillmentTask) error

// NewQueue creates a worker queue with bounded receive defaults.
func NewQueue(queueURL string, waitTimeSeconds, visibilitySeconds, failureVisibleSeconds int32) (Queue, error) {
	if queueURL == "" {
		return Queue{}, fmt.Errorf("%w: queue_url is required", ErrInvalidTask)
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
	return Queue{
		QueueURL:              queueURL,
		WaitTimeSeconds:       waitTimeSeconds,
		VisibilitySeconds:     visibilitySeconds,
		FailureVisibleSeconds: failureVisibleSeconds,
	}, nil
}

// NewPreview builds a local preview for README and go run output.
func NewPreview(queueName string) Preview {
	return Preview{
		QueueName: queueName,
		Operations: []string{
			"Enqueue serializes a fulfillment task and idempotency key",
			"PollOnce receives one visible message with long polling",
			"handler success deletes the SQS message",
			"handler failure changes visibility so the message can retry",
		},
		DeliverySemantics: []string{
			"SQS is at-least-once: handlers must be idempotent",
			"delete is the acknowledgement boundary",
			"visibility timeout is retry control, not a transaction",
			"Floci supplies only local endpoint and test credentials",
		},
		SmokeTest: "BLUETAPE_SQS_FLOCI_WORKER_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/sqs-floci-worker/...",
	}
}

// Enqueue sends one task to SQS.
func (q Queue) Enqueue(ctx context.Context, client Client, task FulfillmentTask) (EnqueuedTask, error) {
	if err := ctx.Err(); err != nil {
		return EnqueuedTask{}, err
	}
	if err := q.validate(); err != nil {
		return EnqueuedTask{}, err
	}
	if err := validateTask(task); err != nil {
		return EnqueuedTask{}, err
	}
	body, err := json.Marshal(task)
	if err != nil {
		return EnqueuedTask{}, fmt.Errorf("marshal task %s: %w", task.IdempotencyKey, err)
	}
	out, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.QueueURL),
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"content-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("application/json"),
			},
			"event-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("fulfillment.task"),
			},
			"idempotency-key": {
				DataType:    aws.String("String"),
				StringValue: aws.String(task.IdempotencyKey),
			},
		},
	})
	if err != nil {
		return EnqueuedTask{}, fmt.Errorf("send task %s: %w", task.IdempotencyKey, err)
	}
	return EnqueuedTask{MessageID: aws.ToString(out.MessageId), IdempotencyKey: task.IdempotencyKey}, nil
}

// PollOnce receives at most one task and acknowledges only after handler success.
func (q Queue) PollOnce(ctx context.Context, client Client, handler Handler) (WorkResult, error) {
	if err := ctx.Err(); err != nil {
		return WorkResult{}, err
	}
	if err := q.validate(); err != nil {
		return WorkResult{}, err
	}
	if handler == nil {
		return WorkResult{}, fmt.Errorf("%w: handler is required", ErrInvalidTask)
	}
	out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(q.QueueURL),
		MaxNumberOfMessages:   1,
		WaitTimeSeconds:       q.WaitTimeSeconds,
		VisibilityTimeout:     q.VisibilitySeconds,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return WorkResult{}, fmt.Errorf("receive task: %w", err)
	}
	if len(out.Messages) == 0 {
		return WorkResult{NoMessage: true}, ErrNoMessages
	}
	message := out.Messages[0]
	result := WorkResult{
		MessageID:     aws.ToString(message.MessageId),
		ReceiptHandle: aws.ToString(message.ReceiptHandle),
	}
	task, err := decodeTask(message)
	if err != nil {
		result.Failure = err.Error()
		if retryErr := q.retrySoon(ctx, client, result.ReceiptHandle); retryErr != nil {
			return result, retryErr
		}
		result.RetryVisible = true
		result.VisibilitySeconds = q.FailureVisibleSeconds
		return result, err
	}
	result.Task = task
	if err := handler(ctx, task); err != nil {
		result.Failure = err.Error()
		if retryErr := q.retrySoon(ctx, client, result.ReceiptHandle); retryErr != nil {
			return result, retryErr
		}
		result.RetryVisible = true
		result.VisibilitySeconds = q.FailureVisibleSeconds
		return result, fmt.Errorf("handle task %s: %w", task.IdempotencyKey, err)
	}
	if err := q.ack(ctx, client, result.ReceiptHandle); err != nil {
		return result, err
	}
	result.Acked = true
	return result, nil
}

// SampleTasks returns scenario-shaped work for preview and tests.
func SampleTasks() []FulfillmentTask {
	return []FulfillmentTask{
		{TenantID: "tenant-alpha", OrderID: "order-1001", Step: "reserve-inventory", IdempotencyKey: "tenant-alpha/order-1001/reserve-inventory"},
		{TenantID: "tenant-alpha", OrderID: "order-1002", Step: "capture-payment", IdempotencyKey: "tenant-alpha/order-1002/capture-payment"},
	}
}

func (q Queue) validate() error {
	if q.QueueURL == "" {
		return fmt.Errorf("%w: queue_url is required", ErrInvalidTask)
	}
	return nil
}

func validateTask(task FulfillmentTask) error {
	if task.TenantID == "" || task.OrderID == "" || task.Step == "" || task.IdempotencyKey == "" {
		return fmt.Errorf("%w: tenant_id, order_id, step, and idempotency_key are required", ErrInvalidTask)
	}
	return nil
}

func decodeTask(message types.Message) (FulfillmentTask, error) {
	var task FulfillmentTask
	body := aws.ToString(message.Body)
	if body == "" {
		return task, fmt.Errorf("%w: message body is empty", ErrInvalidTask)
	}
	if err := json.Unmarshal([]byte(body), &task); err != nil {
		return task, fmt.Errorf("%w: decode message body: %w", ErrInvalidTask, err)
	}
	if err := validateTask(task); err != nil {
		return task, err
	}
	return task, nil
}

func (q Queue) ack(ctx context.Context, client Client, receiptHandle string) error {
	if receiptHandle == "" {
		return fmt.Errorf("%w: receipt handle is required for ack", ErrInvalidTask)
	}
	_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete task message: %w", err)
	}
	return nil
}

func (q Queue) retrySoon(ctx context.Context, client Client, receiptHandle string) error {
	if receiptHandle == "" {
		return fmt.Errorf("%w: receipt handle is required for retry", ErrInvalidTask)
	}
	_, err := client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(q.QueueURL),
		ReceiptHandle:     aws.String(receiptHandle),
		VisibilityTimeout: q.FailureVisibleSeconds,
	})
	if err != nil {
		return fmt.Errorf("change failed task visibility: %w", err)
	}
	return nil
}

// RetryDelay returns the local retry visibility duration for documentation.
func (q Queue) RetryDelay() time.Duration {
	return time.Duration(q.FailureVisibleSeconds) * time.Second
}
