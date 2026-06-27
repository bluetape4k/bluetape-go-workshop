package catalogrepo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestPreviewShowsConditionalBoundary(t *testing.T) {
	preview, err := NewPreview("catalog_items", SampleItems())
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.ItemCount != 3 {
		t.Fatalf("ItemCount = %d, want 3", preview.ItemCount)
	}
	if preview.PartitionKey != "TENANT#<tenant_id>" {
		t.Fatalf("PartitionKey = %q", preview.PartitionKey)
	}
	if len(preview.Operations) != 3 || len(preview.ConflictRules) != 3 {
		t.Fatalf("preview = %#v, want operations and conflict rules", preview)
	}
}

func TestRepositoryCreateUsesCreateIfAbsentCondition(t *testing.T) {
	client := &fakeDynamoClient{}
	repository := newTestRepository(t)

	err := repository.CreateIfAbsent(context.Background(), client, SampleItems()[0])
	if err != nil {
		t.Fatalf("CreateIfAbsent() error = %v", err)
	}
	if client.putInput == nil {
		t.Fatal("PutItem input = nil")
	}
	if got := deref(client.putInput.ConditionExpression); got != "attribute_not_exists(#pk) AND attribute_not_exists(#sk)" {
		t.Fatalf("ConditionExpression = %q", got)
	}
	if got := stringValue(client.putInput.Item["pk"]); got != "TENANT#tenant-alpha" {
		t.Fatalf("pk = %q", got)
	}
	if got := stringValue(client.putInput.Item["sk"]); got != "ITEM#sku-1001" {
		t.Fatalf("sk = %q", got)
	}
}

func TestRepositoryCreateReportsConditionalConflict(t *testing.T) {
	cause := &types.ConditionalCheckFailedException{Message: stringPtr("already exists")}
	client := &fakeDynamoClient{putErr: cause}
	repository := newTestRepository(t)

	err := repository.CreateIfAbsent(context.Background(), client, SampleItems()[0])
	if err == nil {
		t.Fatal("CreateIfAbsent() error = nil, want conflict")
	}
	if !errors.Is(err, ErrConditionalConflict) {
		t.Fatalf("error = %v, want ErrConditionalConflict", err)
	}
	var typed *types.ConditionalCheckFailedException
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T %[1]v, want ConditionalCheckFailedException", err)
	}
}

func TestRepositoryUpdateUsesOptimisticVersionCondition(t *testing.T) {
	updated := CatalogItem{TenantID: "tenant-alpha", SKU: "sku-1001", Name: "Road bike pro", Version: 2, UpdatedBy: "operator"}
	client := &fakeDynamoClient{updateOutput: &dynamodb.UpdateItemOutput{Attributes: encodeItem(updated)}}
	repository := newTestRepository(t)

	got, err := repository.UpdateName(context.Background(), client, "tenant-alpha", "sku-1001", 1, "Road bike pro", "operator")
	if err != nil {
		t.Fatalf("UpdateName() error = %v", err)
	}
	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("UpdateName() item = %#v, want %#v", got, updated)
	}
	if client.updateInput == nil {
		t.Fatal("UpdateItem input = nil")
	}
	if got := deref(client.updateInput.ConditionExpression); got != "#version = :expected_version" {
		t.Fatalf("ConditionExpression = %q", got)
	}
	if got := numberValue(client.updateInput.ExpressionAttributeValues[":expected_version"]); got != "1" {
		t.Fatalf("expected version = %q", got)
	}
	if got := numberValue(client.updateInput.ExpressionAttributeValues[":next_version"]); got != "2" {
		t.Fatalf("next version = %q", got)
	}
	if client.updateInput.ReturnValues != types.ReturnValueAllNew {
		t.Fatalf("ReturnValues = %s, want ALL_NEW", client.updateInput.ReturnValues)
	}
}

func TestRepositoryUpdateReportsConditionalConflict(t *testing.T) {
	cause := &types.ConditionalCheckFailedException{Message: stringPtr("stale version")}
	client := &fakeDynamoClient{updateErr: cause}
	repository := newTestRepository(t)

	_, err := repository.UpdateName(context.Background(), client, "tenant-alpha", "sku-1001", 1, "Road bike pro", "operator")
	if err == nil {
		t.Fatal("UpdateName() error = nil, want conflict")
	}
	if !errors.Is(err, ErrConditionalConflict) {
		t.Fatalf("error = %v, want ErrConditionalConflict", err)
	}
	var typed *types.ConditionalCheckFailedException
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T %[1]v, want ConditionalCheckFailedException", err)
	}
}

func TestRepositoryQueryTenantMapsItems(t *testing.T) {
	want := []CatalogItem{
		{TenantID: "tenant-alpha", SKU: "sku-1001", Name: "Road bike", Version: 1, UpdatedBy: "seed"},
		{TenantID: "tenant-alpha", SKU: "sku-1002", Name: "Helmet", Version: 1, UpdatedBy: "seed"},
	}
	client := &fakeDynamoClient{queryOutput: &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{
		encodeItem(want[0]),
		encodeItem(want[1]),
	}}}
	repository := newTestRepository(t)

	got, err := repository.QueryTenant(context.Background(), client, "tenant-alpha")
	if err != nil {
		t.Fatalf("QueryTenant() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("QueryTenant() items = %#v, want %#v", got, want)
	}
	if got := deref(client.queryInput.KeyConditionExpression); got != "#pk = :pk AND begins_with(#sk, :item_prefix)" {
		t.Fatalf("KeyConditionExpression = %q", got)
	}
	if got := stringValue(client.queryInput.ExpressionAttributeValues[":pk"]); got != "TENANT#tenant-alpha" {
		t.Fatalf(":pk = %q", got)
	}
	if client.queryInput.ConsistentRead == nil || !*client.queryInput.ConsistentRead {
		t.Fatal("ConsistentRead = false, want true for repository example")
	}
}

func TestRepositoryPropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeDynamoClient{}
	repository := newTestRepository(t)

	err := repository.CreateIfAbsent(ctx, client, SampleItems()[0])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateIfAbsent() error = %v, want context.Canceled", err)
	}
	if client.putInput != nil {
		t.Fatal("PutItem should not be called after context cancellation")
	}
}

func TestRepositoryRejectsInvalidItems(t *testing.T) {
	repository := newTestRepository(t)
	err := repository.CreateIfAbsent(context.Background(), &fakeDynamoClient{}, CatalogItem{TenantID: "tenant-alpha", SKU: "sku-1001", Version: 1, UpdatedBy: "seed"})
	if !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("CreateIfAbsent() error = %v, want ErrInvalidItem", err)
	}
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repository, err := NewRepository("catalog_items")
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	return repository
}

type fakeDynamoClient struct {
	putInput     *dynamodb.PutItemInput
	updateInput  *dynamodb.UpdateItemInput
	queryInput   *dynamodb.QueryInput
	putErr       error
	updateErr    error
	queryErr     error
	updateOutput *dynamodb.UpdateItemOutput
	queryOutput  *dynamodb.QueryOutput
}

func (c *fakeDynamoClient) PutItem(_ context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	c.putInput = input
	if c.putErr != nil {
		return nil, c.putErr
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (c *fakeDynamoClient) UpdateItem(_ context.Context, input *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	c.updateInput = input
	if c.updateErr != nil {
		return nil, c.updateErr
	}
	if c.updateOutput == nil {
		return &dynamodb.UpdateItemOutput{}, nil
	}
	return c.updateOutput, nil
}

func (c *fakeDynamoClient) Query(_ context.Context, input *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	c.queryInput = input
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.queryOutput == nil {
		return &dynamodb.QueryOutput{}, nil
	}
	return c.queryOutput, nil
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

func numberValue(value types.AttributeValue) string {
	attr, ok := value.(*types.AttributeValueMemberN)
	if !ok {
		return ""
	}
	return attr.Value
}
