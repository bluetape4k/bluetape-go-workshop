package catalogrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestFlociSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_DYNAMODB_CONDITIONAL_SMOKE") != "1" {
		t.Skip("set BLUETAPE_DYNAMODB_CONDITIONAL_SMOKE=1 to run the Docker-backed Floci DynamoDB conditional repository smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t, flocitestcontainer.WithDynamoDBConfig(flocitestcontainer.DefaultDynamoDBConfig()))
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := dynamodb.NewFromConfig(cfg)

	table := "catalog_items_" + fmt.Sprintf("%d", time.Now().UnixNano())
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

	repository, err := NewRepository(table)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	item := SampleItems()[0]
	if err := repository.CreateIfAbsent(ctx, client, item); err != nil {
		t.Fatalf("CreateIfAbsent() smoke error = %v", err)
	}
	if err := repository.CreateIfAbsent(ctx, client, item); !errors.Is(err, ErrConditionalConflict) {
		t.Fatalf("duplicate CreateIfAbsent() error = %v, want ErrConditionalConflict", err)
	}

	updated, err := repository.UpdateName(ctx, client, item.TenantID, item.SKU, 1, "Road bike pro", "smoke")
	if err != nil {
		t.Fatalf("UpdateName() smoke error = %v", err)
	}
	if updated.Version != 2 || updated.Name != "Road bike pro" {
		t.Fatalf("updated item = %#v, want version=2 name=Road bike pro", updated)
	}
	if _, err := repository.UpdateName(ctx, client, item.TenantID, item.SKU, 1, "Stale name", "smoke"); !errors.Is(err, ErrConditionalConflict) {
		t.Fatalf("stale UpdateName() error = %v, want ErrConditionalConflict", err)
	}

	items, err := repository.QueryTenant(ctx, client, item.TenantID)
	if err != nil {
		t.Fatalf("QueryTenant() smoke error = %v", err)
	}
	if len(items) != 1 || items[0].Version != 2 {
		t.Fatalf("QueryTenant() items = %#v, want one version=2 item", items)
	}
}
