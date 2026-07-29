// Package taskqueue 는 작은 SQS 기반 worker 경계를 보여준다.
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
	// ErrInvalidTask 는 안전하지 않거나 불완전한 task message 를 나타낸다.
	ErrInvalidTask = errors.New("taskqueue: invalid task")
	// ErrNoMessages 는 poll 이 visible SQS message 를 반환하지 않았음을 나타낸다.
	ErrNoMessages = errors.New("taskqueue: no messages")
)

// Client 는 Queue 에 필요한 좁은 SQS client 표면이다.
type Client interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

// Queue 는 queue URL 과 worker receive 정책을 소유한다.
type Queue struct {
	QueueURL              string
	WaitTimeSeconds       int32
	VisibilitySeconds     int32
	FailureVisibleSeconds int32
}

// FulfillmentTask 는 애플리케이션이 소유하는 작업 payload 다.
type FulfillmentTask struct {
	TenantID       string `json:"tenant_id"`
	OrderID        string `json:"order_id"`
	Step           string `json:"step"`
	IdempotencyKey string `json:"idempotency_key"`
}

// EnqueuedTask 는 전송된 SQS message 를 설명한다.
type EnqueuedTask struct {
	MessageID      string `json:"message_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// WorkResult 는 worker poll 시도 하나를 설명한다.
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

// Preview 는 SQS에 접속하지 않고 예제 계약을 설명한다.
type Preview struct {
	QueueName         string   `json:"queue_name"`
	Operations        []string `json:"operations"`
	DeliverySemantics []string `json:"delivery_semantics"`
	SmokeTest         string   `json:"smoke_test"`
}

// Handler 는 decode 된 task 하나를 처리한다.
type Handler func(context.Context, FulfillmentTask) error

// NewQueue 는 제한된 receive 기본값을 가진 worker queue 를 생성한다.
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

// NewPreview 는 README 와 go run 출력용 로컬 미리보기를 구성한다.
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

// Enqueue 는 task 하나를 SQS로 전송한다.
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

// PollOnce 는 최대 하나의 task 를 받고 handler 성공 후에만 acknowledge 한다.
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

// SampleTasks 는 미리보기와 테스트에 사용할 시나리오 형태 작업을 반환한다.
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

// RetryDelay 는 문서화를 위한 로컬 재시도 visibility 기간을 반환한다.
func (q Queue) RetryDelay() time.Duration {
	return time.Duration(q.FailureVisibleSeconds) * time.Second
}
