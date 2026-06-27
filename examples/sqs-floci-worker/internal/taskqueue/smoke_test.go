package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestFlociSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_SQS_FLOCI_WORKER_SMOKE") != "1" {
		t.Skip("set BLUETAPE_SQS_FLOCI_WORKER_SMOKE=1 to run the Docker-backed Floci SQS worker smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t, flocitestcontainer.WithSQSConfig(flocitestcontainer.DefaultSQSConfig()))
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := sqs.NewFromConfig(cfg)

	queueURL := createQueue(ctx, t, client, "bluetape-sqs-worker-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	queue, err := NewQueue(queueURL, 1, 2, 0)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}

	first := SampleTasks()[0]
	enqueued, err := queue.Enqueue(ctx, client, first)
	if err != nil {
		t.Fatalf("Enqueue() smoke error = %v", err)
	}
	if enqueued.IdempotencyKey != first.IdempotencyKey {
		t.Fatalf("enqueued = %#v", enqueued)
	}
	result, err := queue.PollOnce(ctx, client, func(_ context.Context, task FulfillmentTask) error {
		if task != first {
			t.Fatalf("handled task = %#v, want %#v", task, first)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("PollOnce() success smoke error = %v", err)
	}
	if !result.Acked {
		t.Fatalf("success result = %#v, want acked", result)
	}
	empty, err := receiveRaw(ctx, client, queueURL)
	if err != nil {
		t.Fatalf("receive after ack: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("received %d messages after ack, want 0", len(empty))
	}

	second := SampleTasks()[1]
	if _, err := queue.Enqueue(ctx, client, second); err != nil {
		t.Fatalf("Enqueue() retry smoke error = %v", err)
	}
	retryErr := errors.New("payment gateway timeout")
	failed, err := queue.PollOnce(ctx, client, func(context.Context, FulfillmentTask) error {
		return retryErr
	})
	if !errors.Is(err, retryErr) {
		t.Fatalf("PollOnce() failure error = %v, want retryErr", err)
	}
	if !failed.RetryVisible || failed.Acked {
		t.Fatalf("failure result = %#v, want retry-visible without ack", failed)
	}
	retried, err := receiveUntil(ctx, client, queueURL, 1, 2, 0)
	if err != nil {
		t.Fatalf("receive retry-visible message: %v", err)
	}
	if len(retried) != 1 {
		t.Fatalf("retry-visible messages = %d, want 1", len(retried))
	}
	if err := deleteRaw(ctx, client, queueURL, aws.ToString(retried[0].ReceiptHandle)); err != nil {
		t.Fatalf("delete retry-visible message: %v", err)
	}
}

func createQueue(ctx context.Context, t *testing.T, client *sqs.Client, name string) string {
	t.Helper()
	out, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(name),
		Attributes: map[string]string{
			string(types.QueueAttributeNameVisibilityTimeout): "2",
		},
	})
	if err != nil {
		t.Fatalf("create sqs queue: %v", err)
	}
	queueURL := aws.ToString(out.QueueUrl)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.DeleteQueue(cleanupCtx, &sqs.DeleteQueueInput{QueueUrl: aws.String(queueURL)})
	})
	return queueURL
}

func receiveRaw(ctx context.Context, client *sqs.Client, queueURL string) ([]types.Message, error) {
	out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(queueURL),
		MaxNumberOfMessages:   1,
		WaitTimeSeconds:       1,
		VisibilityTimeout:     1,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return nil, fmt.Errorf("receive raw message: %w", err)
	}
	return out.Messages, nil
}

func receiveUntil(ctx context.Context, client *sqs.Client, queueURL string, maxMessages, waitSeconds, visibilitySeconds int32) ([]types.Message, error) {
	for {
		out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(queueURL),
			MaxNumberOfMessages:   maxMessages,
			WaitTimeSeconds:       waitSeconds,
			VisibilityTimeout:     visibilitySeconds,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			return nil, fmt.Errorf("receive messages: %w", err)
		}
		if len(out.Messages) > 0 {
			return out.Messages, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}

func deleteRaw(ctx context.Context, client *sqs.Client, queueURL, receiptHandle string) error {
	_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete raw message: %w", err)
	}
	return nil
}
