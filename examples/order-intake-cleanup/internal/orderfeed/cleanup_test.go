package orderfeed_test

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go-workshop/examples/order-intake-cleanup/internal/orderfeed"
	"github.com/bluetape4k/bluetape-go/core"
)

func TestNormalizeRowsCleansPartnerFeed(t *testing.T) {
	coupon := "WELCOME"
	result, err := orderfeed.NormalizeRows([]orderfeed.PartnerRow{
		{
			OrderID:  "ord-1",
			Tenant:   "tenant-a",
			Channel:  "mobile",
			Customer: "customer-1",
			Coupon:   core.Ptr(coupon),
			Items: []orderfeed.PartnerItem{
				{SKU: "sku-1", Quantity: 2},
				{SKU: "", Quantity: 1},
				{SKU: "sku-2", Quantity: 0},
			},
			Tags: []string{"VIP", "vip", "Flash"},
		},
		{
			OrderID:  "ord-2",
			Tenant:   "tenant-a",
			Customer: "customer-2",
			Items:    []orderfeed.PartnerItem{{SKU: "sku-3", Quantity: 1}},
		},
	})
	if err != nil {
		t.Fatalf("normalize rows: %v", err)
	}

	if len(result.Orders) != 2 {
		t.Fatalf("orders = %+v", result.Orders)
	}
	first := result.Orders[0]
	if first.Coupon != coupon || first.Notes != "no notes" || len(first.Items) != 1 {
		t.Fatalf("first order = %+v", first)
	}
	if len(first.Tags) != 2 {
		t.Fatalf("tags should be case-insensitive distinct, got %+v", first.Tags)
	}
	if len(result.ByChannel["mobile"]) != 1 || len(result.ByChannel["web"]) != 1 {
		t.Fatalf("grouped = %+v", result.ByChannel)
	}
}

func TestNormalizeRowsRejectsInvalidPartnerFeed(t *testing.T) {
	_, err := orderfeed.NormalizeRows([]orderfeed.PartnerRow{{OrderID: "ord-1", Tenant: "tenant-a"}})
	if err == nil || !strings.Contains(err.Error(), "customer") {
		t.Fatalf("expected customer validation error, got %v", err)
	}

	_, err = orderfeed.NormalizeRows([]orderfeed.PartnerRow{{
		OrderID:  "ord-2",
		Tenant:   "tenant-a",
		Customer: "customer-2",
		Items:    []orderfeed.PartnerItem{{SKU: "", Quantity: -1}},
	}})
	if err == nil || !strings.Contains(err.Error(), "items") {
		t.Fatalf("expected items validation error, got %v", err)
	}
}
