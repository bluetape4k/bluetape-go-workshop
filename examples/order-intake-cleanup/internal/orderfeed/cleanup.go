// Package orderfeed 는 파트너 주문 피드 정리 흐름을 보여준다.
package orderfeed

import (
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/collections"
	"github.com/bluetape4k/bluetape-go/core"
)

// PartnerRow 는 파트너 피드에서 받은 원본 주문 형태다.
type PartnerRow struct {
	OrderID  string
	Tenant   string
	Channel  string
	Customer string
	Coupon   *string
	Notes    string
	Items    []PartnerItem
	Tags     []string
}

// PartnerItem 은 원본 피드의 한 줄 품목이다.
type PartnerItem struct {
	SKU      string
	Quantity int
}

// Order 는 정규화된 내부 주문 형태다.
type Order struct {
	ID       string
	Tenant   string
	Channel  string
	Customer string
	Coupon   string
	Notes    string
	Items    []LineItem
	Tags     []string
}

// LineItem 은 검증된 주문 품목 줄이다.
type LineItem struct {
	SKU      string
	Quantity int
}

// CleanupResult 는 수락된 주문과 다운스트림 배치 관점을 담는다.
type CleanupResult struct {
	Orders    []Order
	ByChannel map[string][]Order
}

// NormalizeRows 는 파트너 피드 행을 검증하고 정규화한다.
func NormalizeRows(rows []PartnerRow) (CleanupResult, error) {
	orders, err := collections.MapErr(rows, normalizeRow)
	if err != nil {
		return CleanupResult{}, err
	}
	grouped, err := collections.GroupBy(orders, func(order Order) string {
		return order.Channel
	})
	if err != nil {
		return CleanupResult{}, err
	}
	return CleanupResult{Orders: orders, ByChannel: grouped}, nil
}

func normalizeRow(row PartnerRow) (Order, error) {
	if err := core.RequireNotBlank("order_id", row.OrderID); err != nil {
		return Order{}, err
	}
	if err := core.RequireNotBlank("tenant", row.Tenant); err != nil {
		return Order{}, err
	}
	if err := core.RequireNotBlank("customer", row.Customer); err != nil {
		return Order{}, err
	}

	items, err := collections.FilterMap(row.Items, func(item PartnerItem) (LineItem, bool) {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" || item.Quantity <= 0 {
			return LineItem{}, false
		}
		return LineItem{SKU: sku, Quantity: item.Quantity}, true
	})
	if err != nil {
		return Order{}, err
	}
	if len(items) == 0 {
		return Order{}, fmt.Errorf("%s: items must not be empty", row.OrderID)
	}

	tags, err := collections.DistinctBy(row.Tags, func(value string) string {
		return strings.ToLower(strings.TrimSpace(value))
	})
	if err != nil {
		return Order{}, err
	}

	return Order{
		ID:       row.OrderID,
		Tenant:   row.Tenant,
		Channel:  core.BlankToDefault(row.Channel, "web"),
		Customer: row.Customer,
		Coupon:   core.ValueOr(row.Coupon, "none"),
		Notes:    core.BlankToDefault(row.Notes, "no notes"),
		Items:    items,
		Tags:     tags,
	}, nil
}
