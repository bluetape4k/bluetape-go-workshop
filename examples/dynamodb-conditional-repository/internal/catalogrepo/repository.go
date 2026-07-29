// Package catalogrepo 는 DynamoDB conditional repository write를 보여준다.
package catalogrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	// ErrInvalidItem 은 catalog row로 저장할 수 없는 item을 나타낸다.
	ErrInvalidItem = errors.New("catalogrepo: invalid item")
	// ErrConditionalConflict 는 DynamoDB conditional write conflict를 나타낸다.
	ErrConditionalConflict = errors.New("catalogrepo: conditional conflict")
	// ErrDecodeItem 은 repository 계약과 더 이상 맞지 않는 DynamoDB item shape를 나타낸다.
	ErrDecodeItem = errors.New("catalogrepo: decode item")
)

// Client 는 Repository에 필요한 좁은 DynamoDB client surface다.
type Client interface {
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// CatalogItem 은 DynamoDB에 저장되는 애플리케이션 소유 item shape다.
type CatalogItem struct {
	TenantID  string `json:"tenant_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
}

// Repository 는 catalog conditional-write expression과 item mapping을 소유한다.
type Repository struct {
	Table string
}

// Preview 는 DynamoDB에 접속하지 않고 repository 계약을 설명한다.
type Preview struct {
	Table         string   `json:"table"`
	ItemCount     int      `json:"item_count"`
	PartitionKey  string   `json:"partition_key"`
	SortKeyPrefix string   `json:"sort_key_prefix"`
	Operations    []string `json:"operations"`
	ConflictRules []string `json:"conflict_rules"`
	SmokeTest     string   `json:"smoke_test"`
}

// NewRepository 는 DynamoDB catalog repository를 만든다.
func NewRepository(table string) (Repository, error) {
	if table == "" {
		return Repository{}, fmt.Errorf("%w: table is required", ErrInvalidItem)
	}
	return Repository{Table: table}, nil
}

// NewPreview 는 README와 go run 출력에서 확인할 수 있는 local preview를 만든다.
func NewPreview(table string, items []CatalogItem) (Preview, error) {
	if _, err := NewRepository(table); err != nil {
		return Preview{}, err
	}
	for _, item := range items {
		if err := validateItem(item); err != nil {
			return Preview{}, err
		}
	}
	return Preview{
		Table:         table,
		ItemCount:     len(items),
		PartitionKey:  "TENANT#<tenant_id>",
		SortKeyPrefix: "ITEM#<sku>",
		Operations: []string{
			"CreateIfAbsent uses attribute_not_exists(pk) and attribute_not_exists(sk)",
			"UpdateName uses version = expectedVersion for optimistic updates",
			"QueryTenant reads a tenant partition with begins_with(sk, ITEM#)",
		},
		ConflictRules: []string{
			"ConditionalCheckFailedException becomes ErrConditionalConflict",
			"the typed AWS SDK error is preserved for errors.As",
			"context cancellation is returned before client calls",
		},
		SmokeTest: "BLUETAPE_DYNAMODB_CONDITIONAL_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/dynamodb-conditional-repository/...",
	}, nil
}

// CreateIfAbsent 는 pk/sk 쌍이 없을 때만 item을 쓴다.
func (r Repository) CreateIfAbsent(ctx context.Context, client Client, item CatalogItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.Table == "" {
		return fmt.Errorf("%w: table is required", ErrInvalidItem)
	}
	if err := validateItem(item); err != nil {
		return err
	}
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: stringPtr(r.Table),
		Item:      encodeItem(item),
		ConditionExpression: stringPtr(
			"attribute_not_exists(#pk) AND attribute_not_exists(#sk)",
		),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
	})
	if err == nil {
		return nil
	}
	if isConditionalConflict(err) {
		return fmt.Errorf("%w: create catalog item %s/%s: %w", ErrConditionalConflict, item.TenantID, item.SKU, err)
	}
	return fmt.Errorf("create catalog item %s/%s: %w", item.TenantID, item.SKU, err)
}

// UpdateName 은 expected version을 사용해 optimistic item update를 수행한다.
func (r Repository) UpdateName(ctx context.Context, client Client, tenantID, sku string, expectedVersion int, name, updatedBy string) (CatalogItem, error) {
	if err := ctx.Err(); err != nil {
		return CatalogItem{}, err
	}
	if r.Table == "" {
		return CatalogItem{}, fmt.Errorf("%w: table is required", ErrInvalidItem)
	}
	if tenantID == "" || sku == "" || expectedVersion <= 0 || name == "" || updatedBy == "" {
		return CatalogItem{}, fmt.Errorf("%w: update requires tenant_id, sku, positive expected version, name, and updated_by", ErrInvalidItem)
	}
	nextVersion := expectedVersion + 1
	output, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: stringPtr(r.Table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: tenantKey(tenantID)},
			"sk": &types.AttributeValueMemberS{Value: itemKey(sku)},
		},
		UpdateExpression:    stringPtr("SET #name = :name, #version = :next_version, #updated_by = :updated_by"),
		ConditionExpression: stringPtr("#version = :expected_version"),
		ExpressionAttributeNames: map[string]string{
			"#name":       "name",
			"#version":    "version",
			"#updated_by": "updated_by",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":             &types.AttributeValueMemberS{Value: name},
			":next_version":     &types.AttributeValueMemberN{Value: strconv.Itoa(nextVersion)},
			":updated_by":       &types.AttributeValueMemberS{Value: updatedBy},
			":expected_version": &types.AttributeValueMemberN{Value: strconv.Itoa(expectedVersion)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		if isConditionalConflict(err) {
			return CatalogItem{}, fmt.Errorf("%w: update catalog item %s/%s at version %d: %w", ErrConditionalConflict, tenantID, sku, expectedVersion, err)
		}
		return CatalogItem{}, fmt.Errorf("update catalog item %s/%s: %w", tenantID, sku, err)
	}
	item, err := decodeItem(output.Attributes)
	if err != nil {
		return CatalogItem{}, err
	}
	return item, nil
}

// QueryTenant 는 tenant partition 하나의 모든 catalog item을 읽는다.
func (r Repository) QueryTenant(ctx context.Context, client Client, tenantID string) ([]CatalogItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Table == "" {
		return nil, fmt.Errorf("%w: table is required", ErrInvalidItem)
	}
	if tenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", ErrInvalidItem)
	}
	output, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              stringPtr(r.Table),
		KeyConditionExpression: stringPtr("#pk = :pk AND begins_with(#sk, :item_prefix)"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "pk",
			"#sk": "sk",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":          &types.AttributeValueMemberS{Value: tenantKey(tenantID)},
			":item_prefix": &types.AttributeValueMemberS{Value: "ITEM#"},
		},
		ConsistentRead: boolPtr(true),
	})
	if err != nil {
		return nil, fmt.Errorf("query catalog tenant %s: %w", tenantID, err)
	}
	items := make([]CatalogItem, 0, len(output.Items))
	for _, raw := range output.Items {
		item, err := decodeItem(raw)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// SampleItems 는 partition-key query 동작을 보여주기에 충분한 데이터를 반환한다.
func SampleItems() []CatalogItem {
	return []CatalogItem{
		{TenantID: "tenant-alpha", SKU: "sku-1001", Name: "Road bike", Version: 1, UpdatedBy: "seed"},
		{TenantID: "tenant-alpha", SKU: "sku-1002", Name: "Helmet", Version: 1, UpdatedBy: "seed"},
		{TenantID: "tenant-beta", SKU: "sku-2001", Name: "Touring bag", Version: 1, UpdatedBy: "seed"},
	}
}

func validateItem(item CatalogItem) error {
	switch {
	case item.TenantID == "":
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidItem)
	case item.SKU == "":
		return fmt.Errorf("%w: sku is required", ErrInvalidItem)
	case item.Name == "":
		return fmt.Errorf("%w: name is required", ErrInvalidItem)
	case item.Version <= 0:
		return fmt.Errorf("%w: version must be positive", ErrInvalidItem)
	case item.UpdatedBy == "":
		return fmt.Errorf("%w: updated_by is required", ErrInvalidItem)
	default:
		return nil
	}
}

func encodeItem(item CatalogItem) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: tenantKey(item.TenantID)},
		"sk":         &types.AttributeValueMemberS{Value: itemKey(item.SKU)},
		"tenant_id":  &types.AttributeValueMemberS{Value: item.TenantID},
		"sku":        &types.AttributeValueMemberS{Value: item.SKU},
		"name":       &types.AttributeValueMemberS{Value: item.Name},
		"version":    &types.AttributeValueMemberN{Value: strconv.Itoa(item.Version)},
		"updated_by": &types.AttributeValueMemberS{Value: item.UpdatedBy},
	}
}

func decodeItem(raw map[string]types.AttributeValue) (CatalogItem, error) {
	item := CatalogItem{}
	var err error
	if item.TenantID, err = stringAttr(raw, "tenant_id"); err != nil {
		return CatalogItem{}, err
	}
	if item.SKU, err = stringAttr(raw, "sku"); err != nil {
		return CatalogItem{}, err
	}
	if item.Name, err = stringAttr(raw, "name"); err != nil {
		return CatalogItem{}, err
	}
	if item.Version, err = intAttr(raw, "version"); err != nil {
		return CatalogItem{}, err
	}
	if item.UpdatedBy, err = stringAttr(raw, "updated_by"); err != nil {
		return CatalogItem{}, err
	}
	return item, nil
}

func stringAttr(raw map[string]types.AttributeValue, name string) (string, error) {
	value, ok := raw[name].(*types.AttributeValueMemberS)
	if !ok || value.Value == "" {
		return "", fmt.Errorf("%w: %s string attribute is required", ErrDecodeItem, name)
	}
	return value.Value, nil
}

func intAttr(raw map[string]types.AttributeValue, name string) (int, error) {
	value, ok := raw[name].(*types.AttributeValueMemberN)
	if !ok || value.Value == "" {
		return 0, fmt.Errorf("%w: %s number attribute is required", ErrDecodeItem, name)
	}
	parsed, err := strconv.Atoi(value.Value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s number attribute: %w", ErrDecodeItem, name, err)
	}
	return parsed, nil
}

func isConditionalConflict(err error) bool {
	var conflict *types.ConditionalCheckFailedException
	return errors.As(err, &conflict)
}

func tenantKey(tenantID string) string {
	return "TENANT#" + tenantID
}

func itemKey(sku string) string {
	return "ITEM#" + sku
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
