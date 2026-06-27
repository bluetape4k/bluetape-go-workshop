package docmaterializer

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestFlociSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE") != "1" {
		t.Skip("set BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 to run the Docker-backed Floci DynamoDB batchwrite smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t, flocitestcontainer.WithDynamoDBConfig(flocitestcontainer.DefaultDynamoDBConfig()))
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := dynamodb.NewFromConfig(cfg)

	table := "document_index_" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: stringPtr(table),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: stringPtr("pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: stringPtr("sk"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: stringPtr("pk"), KeyType: types.KeyTypeHash},
			{AttributeName: stringPtr("sk"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModePayPerRequest,
	}); err != nil {
		t.Fatalf("create dynamodb table: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.DeleteTable(cleanupCtx, &dynamodb.DeleteTableInput{TableName: stringPtr(table)})
	})

	m, err := NewMaterializer(table)
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}
	report, err := m.Write(ctx, client, SampleEvents())
	if err != nil {
		t.Fatalf("Write() smoke error = %v", err)
	}
	if report.Processed != 30 || report.Attempts != 2 {
		t.Fatalf("report = %#v, want processed=30 attempts=2", report)
	}

	scan, err := client.Scan(ctx, &dynamodb.ScanInput{TableName: stringPtr(table)})
	if err != nil {
		t.Fatalf("scan dynamodb table: %v", err)
	}
	if got := len(scan.Items); got != 30 {
		t.Fatalf("scan item count = %d, want 30", got)
	}
}
