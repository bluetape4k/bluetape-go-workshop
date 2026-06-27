package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func TestPreviewShowsWorkerBoundary(t *testing.T) {
	preview := NewPreview("fulfillment-tasks")
	if preview.QueueName != "fulfillment-tasks" {
		t.Fatalf("QueueName = %q", preview.QueueName)
	}
	if len(preview.Operations) != 4 || len(preview.DeliverySemantics) != 4 {
		t.Fatalf("preview = %#v, want operations and delivery semantics", preview)
	}
}

func TestEnqueueMapsMessageBodyAndAttributes(t *testing.T) {
	client := &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("msg-1")}}
	queue := newTestQueue(t)
	task := SampleTasks()[0]

	enqueued, err := queue.Enqueue(context.Background(), client, task)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if enqueued.MessageID != "msg-1" || enqueued.IdempotencyKey != task.IdempotencyKey {
		t.Fatalf("Enqueue() = %#v", enqueued)
	}
	if got := aws.ToString(client.sendInput.QueueUrl); got != queue.QueueURL {
		t.Fatalf("QueueUrl = %q", got)
	}
	var decoded FulfillmentTask
	if err := json.Unmarshal([]byte(aws.ToString(client.sendInput.MessageBody)), &decoded); err != nil {
		t.Fatalf("message body json: %v", err)
	}
	if !reflect.DeepEqual(decoded, task) {
		t.Fatalf("message body = %#v, want %#v", decoded, task)
	}
	if got := aws.ToString(client.sendInput.MessageAttributes["idempotency-key"].StringValue); got != task.IdempotencyKey {
		t.Fatalf("idempotency-key attribute = %q", got)
	}
}

func TestPollOnceDeletesAfterHandlerSuccess(t *testing.T) {
	task := SampleTasks()[0]
	client := &fakeSQSClient{receiveOutput: messageOutput(t, task)}
	queue := newTestQueue(t)
	var handled FulfillmentTask

	result, err := queue.PollOnce(context.Background(), client, func(_ context.Context, task FulfillmentTask) error {
		handled = task
		return nil
	})
	if err != nil {
		t.Fatalf("PollOnce() error = %v", err)
	}
	if !result.Acked || result.RetryVisible {
		t.Fatalf("result = %#v, want acked only", result)
	}
	if !reflect.DeepEqual(handled, task) {
		t.Fatalf("handled = %#v, want %#v", handled, task)
	}
	if client.deleteInput == nil {
		t.Fatal("DeleteMessage input = nil")
	}
	if client.visibilityInput != nil {
		t.Fatal("ChangeMessageVisibility should not be called after success")
	}
}

func TestPollOnceMakesFailureVisibleForRetry(t *testing.T) {
	task := SampleTasks()[0]
	client := &fakeSQSClient{receiveOutput: messageOutput(t, task)}
	queue := newTestQueue(t)
	wantErr := errors.New("inventory timeout")

	result, err := queue.PollOnce(context.Background(), client, func(context.Context, FulfillmentTask) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("PollOnce() error = %v, want %v", err, wantErr)
	}
	if result.Acked || !result.RetryVisible || result.VisibilitySeconds != queue.FailureVisibleSeconds {
		t.Fatalf("result = %#v, want retry visible", result)
	}
	if client.deleteInput != nil {
		t.Fatal("DeleteMessage should not be called after handler failure")
	}
	if got := client.visibilityInput.VisibilityTimeout; got != queue.FailureVisibleSeconds {
		t.Fatalf("VisibilityTimeout = %d", got)
	}
}

func TestPollOnceMapsNoMessages(t *testing.T) {
	client := &fakeSQSClient{receiveOutput: &sqs.ReceiveMessageOutput{}}
	queue := newTestQueue(t)

	result, err := queue.PollOnce(context.Background(), client, func(context.Context, FulfillmentTask) error {
		t.Fatal("handler should not run")
		return nil
	})
	if !errors.Is(err, ErrNoMessages) || !result.NoMessage {
		t.Fatalf("PollOnce() result=%#v error=%v, want ErrNoMessages", result, err)
	}
}

func TestPollOnceRejectsInvalidBodyAndRetries(t *testing.T) {
	client := &fakeSQSClient{receiveOutput: &sqs.ReceiveMessageOutput{Messages: []types.Message{
		{MessageId: aws.String("msg-1"), ReceiptHandle: aws.String("receipt-1"), Body: aws.String("{broken")},
	}}}
	queue := newTestQueue(t)

	result, err := queue.PollOnce(context.Background(), client, func(context.Context, FulfillmentTask) error {
		t.Fatal("handler should not run for invalid body")
		return nil
	})
	if !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("PollOnce() error = %v, want ErrInvalidTask", err)
	}
	if !result.RetryVisible || client.visibilityInput == nil {
		t.Fatalf("result = %#v, want retry visibility call", result)
	}
}

func TestEnqueuePropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeSQSClient{}
	queue := newTestQueue(t)

	_, err := queue.Enqueue(ctx, client, SampleTasks()[0])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Enqueue() error = %v, want context.Canceled", err)
	}
	if client.sendInput != nil {
		t.Fatal("SendMessage should not be called after context cancellation")
	}
}

func newTestQueue(t *testing.T) Queue {
	t.Helper()
	queue, err := NewQueue("https://sqs.local/000000000000/fulfillment-tasks", 2, 30, 0)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	return queue
}

func messageOutput(t *testing.T, task FulfillmentTask) *sqs.ReceiveMessageOutput {
	t.Helper()
	body, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	return &sqs.ReceiveMessageOutput{Messages: []types.Message{
		{
			MessageId:     aws.String("msg-1"),
			ReceiptHandle: aws.String("receipt-1"),
			Body:          aws.String(string(body)),
		},
	}}
}

type fakeSQSClient struct {
	sendInput       *sqs.SendMessageInput
	receiveInput    *sqs.ReceiveMessageInput
	deleteInput     *sqs.DeleteMessageInput
	visibilityInput *sqs.ChangeMessageVisibilityInput
	sendOutput      *sqs.SendMessageOutput
	receiveOutput   *sqs.ReceiveMessageOutput
	sendErr         error
	receiveErr      error
	deleteErr       error
	visibilityErr   error
}

func (c *fakeSQSClient) SendMessage(_ context.Context, input *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	c.sendInput = input
	if c.sendErr != nil {
		return nil, c.sendErr
	}
	if c.sendOutput != nil {
		return c.sendOutput, nil
	}
	return &sqs.SendMessageOutput{MessageId: aws.String("msg-1")}, nil
}

func (c *fakeSQSClient) ReceiveMessage(_ context.Context, input *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	c.receiveInput = input
	if c.receiveErr != nil {
		return nil, c.receiveErr
	}
	if c.receiveOutput != nil {
		return c.receiveOutput, nil
	}
	return &sqs.ReceiveMessageOutput{}, nil
}

func (c *fakeSQSClient) DeleteMessage(_ context.Context, input *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	c.deleteInput = input
	if c.deleteErr != nil {
		return nil, c.deleteErr
	}
	return &sqs.DeleteMessageOutput{}, nil
}

func (c *fakeSQSClient) ChangeMessageVisibility(_ context.Context, input *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	c.visibilityInput = input
	if c.visibilityErr != nil {
		return nil, c.visibilityErr
	}
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}
