package documentworkflow

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestFlociSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_DOCUMENT_WORKFLOW_SMOKE") != "1" {
		t.Skip("set BLUETAPE_DOCUMENT_WORKFLOW_SMOKE=1 to run the Docker-backed Floci document workflow smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(
		ctx,
		t,
		flocitestcontainer.WithS3Config(flocitestcontainer.DefaultS3Config()),
		flocitestcontainer.WithSQSConfig(flocitestcontainer.DefaultSQSConfig()),
		flocitestcontainer.WithDynamoDBConfig(flocitestcontainer.DefaultDynamoDBConfig()),
	)
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	s3Client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	sqsClient := sqs.NewFromConfig(cfg)
	dynamoClient := dynamodb.NewFromConfig(cfg)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	bucket := "documents-" + suffix
	queueURL := createDocumentQueue(ctx, t, sqsClient, "document-events-"+suffix)
	table := "document_processing_" + suffix
	createDocumentBucket(ctx, t, s3Client, bucket)
	createDocumentTable(ctx, t, dynamoClient, table)

	workflow, err := NewWorkflow(bucket, queueURL, table, 1, 2, 0)
	if err != nil {
		t.Fatalf("NewWorkflow() error = %v", err)
	}
	doc := SampleDocuments()[0]
	submitted, err := workflow.Submit(ctx, s3Client, sqsClient, doc)
	if err != nil {
		t.Fatalf("Submit() smoke error = %v", err)
	}
	if submitted.IdempotencyKey != idempotencyKey(doc.TenantID, doc.DocumentID) {
		t.Fatalf("submitted = %#v", submitted)
	}

	result, err := workflow.ProcessOnce(ctx, s3Client, sqsClient, dynamoClient)
	if err != nil {
		t.Fatalf("ProcessOnce() smoke error = %v", err)
	}
	if !result.Acked || result.Duplicate || result.BytesRead != submitted.Size {
		t.Fatalf("ProcessOnce() = %#v, want successful ack", result)
	}
	if messages := receiveDocumentEvents(ctx, t, sqsClient, queueURL); len(messages) != 0 {
		t.Fatalf("received %d messages after ack, want 0", len(messages))
	}

	if _, err := workflow.Submit(ctx, s3Client, sqsClient, doc); err != nil {
		t.Fatalf("duplicate Submit() smoke error = %v", err)
	}
	duplicate, err := workflow.ProcessOnce(ctx, s3Client, sqsClient, dynamoClient)
	if err != nil {
		t.Fatalf("duplicate ProcessOnce() smoke error = %v", err)
	}
	if !duplicate.Acked || !duplicate.Duplicate {
		t.Fatalf("duplicate result = %#v, want duplicate ack", duplicate)
	}
}

func createDocumentBucket(ctx context.Context, t *testing.T, client *s3.Client, bucket string) {
	t.Helper()
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, doc := range SampleDocuments() {
			_, _ = client.DeleteObject(cleanupCtx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(objectKey(doc.TenantID, doc.DocumentID, doc.FileName)),
			})
		}
		_, _ = client.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})
}

func createDocumentQueue(ctx context.Context, t *testing.T, client *sqs.Client, name string) string {
	t.Helper()
	out, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(name),
		Attributes: map[string]string{
			string(sqstypes.QueueAttributeNameVisibilityTimeout): "2",
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

func createDocumentTable(ctx context.Context, t *testing.T, client *dynamodb.Client, table string) {
	t.Helper()
	if _, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModePayPerRequest,
	}); err != nil {
		t.Fatalf("create dynamodb table: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.DeleteTable(cleanupCtx, &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})
}

func receiveDocumentEvents(ctx context.Context, t *testing.T, client *sqs.Client, queueURL string) []sqstypes.Message {
	t.Helper()
	out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(queueURL),
		MaxNumberOfMessages:   1,
		WaitTimeSeconds:       1,
		VisibilityTimeout:     1,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		t.Fatalf("receive document events: %v", err)
	}
	return out.Messages
}
