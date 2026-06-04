// Package orderfeed demonstrates partner order feed cleanup.
package orderfeed

import (
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/collections"
	"github.com/bluetape4k/bluetape-go/core"
)

// PartnerRow is the raw order shape received from a partner feed.
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

// PartnerItem is one raw feed line item.
type PartnerItem struct {
	SKU      string
	Quantity int
}

// Order is the normalized internal order shape.
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

// LineItem is a validated order line.
type LineItem struct {
	SKU      string
	Quantity int
}

// CleanupResult contains accepted orders and the downstream batching view.
type CleanupResult struct {
	Orders    []Order
	ByChannel map[string][]Order
}

// NormalizeRows validates and normalizes partner feed rows.
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
