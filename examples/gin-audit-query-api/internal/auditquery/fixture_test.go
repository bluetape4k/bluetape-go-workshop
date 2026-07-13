package auditquery

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestSeedRepositoryCreatesDeterministicOrderHistory(t *testing.T) {
	repository := audit.NewMemoryRepository()
	if err := SeedRepository(context.Background(), repository); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		id         string
		eventTypes []audit.EventType
	}{
		{id: "order-1001", eventTypes: []audit.EventType{"order.created", "order.confirmed", "order.packed", "order.shipped"}},
		{id: "order-2001", eventTypes: []audit.EventType{"order.created", "order.cancelled"}},
	}
	seenEvents := make(map[audit.EventID]struct{})
	seenKeys := make(map[string]struct{})
	for _, test := range tests {
		aggregate, err := audit.NewAggregateID("order", test.id)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := repository.Find(context.Background(), audit.Query{Aggregate: &aggregate})
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != len(test.eventTypes) {
			t.Fatalf("%s entry count = %d", test.id, len(entries))
		}
		for index, entry := range entries {
			if entry.Revision != audit.Revision(index+1) || entry.Event.EventType != test.eventTypes[index] {
				t.Fatalf("%s entry[%d] = %+v", test.id, index, entry)
			}
			if entry.Author != "workshop" || entry.Event.Metadata["source"] != "fixture" || entry.Event.RecordedAt.IsZero() {
				t.Fatalf("%s metadata = %+v", test.id, entry)
			}
			if entry.Change == nil || len(entry.Change.ChangedFields) == 0 {
				t.Fatalf("%s change = %+v", test.id, entry.Change)
			}
			if _, duplicate := seenEvents[entry.Event.EventID]; duplicate {
				t.Fatalf("duplicate event id %q", entry.Event.EventID)
			}
			seenEvents[entry.Event.EventID] = struct{}{}
			if _, duplicate := seenKeys[entry.Event.IdempotencyKey]; duplicate {
				t.Fatalf("duplicate idempotency key %q", entry.Event.IdempotencyKey)
			}
			seenKeys[entry.Event.IdempotencyKey] = struct{}{}
		}
	}
}

func TestSeedRepositoryHonorsCancellationWithoutPartialWrite(t *testing.T) {
	repository := audit.NewMemoryRepository()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SeedRepository(ctx, repository); !errors.Is(err, context.Canceled) {
		t.Fatalf("SeedRepository() err = %v", err)
	}
	entries, err := repository.Find(context.Background(), audit.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial entries = %d", len(entries))
	}
}

func TestSeedRepositoryReturnsDefensiveCopies(t *testing.T) {
	repository := audit.NewMemoryRepository()
	if err := SeedRepository(context.Background(), repository); err != nil {
		t.Fatal(err)
	}
	aggregate, err := audit.NewAggregateID("order", "order-1001")
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.Find(context.Background(), audit.Query{Aggregate: &aggregate, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	first[0].Event.Metadata["source"] = "mutated"
	first[0].Event.Payload[0] = '['
	second, err := repository.Find(context.Background(), audit.Query{Aggregate: &aggregate, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Event.Metadata["source"] != "fixture" || second[0].Event.Payload[0] != '{' {
		t.Fatalf("stored entry mutated: %+v", second[0])
	}
}
