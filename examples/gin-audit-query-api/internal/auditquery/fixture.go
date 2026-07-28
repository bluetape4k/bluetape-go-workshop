// Package auditquery 는 Gin audit query API 워크숍 예제를 구현한다.
package auditquery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

type fixtureTransition struct {
	eventType audit.EventType
	from      string
	to        string
	reason    string
}

// SeedRepository 는 결정적인 order history를 repository에 추가한다.
func SeedRepository(ctx context.Context, repository audit.Repository) error {
	if repository == nil || isNilInterface(repository) {
		return fmt.Errorf("%w: repository is required", ErrInvalidConfig)
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	fixtures := []struct {
		orderID     string
		transitions []fixtureTransition
	}{
		{
			orderID: "order-1001",
			transitions: []fixtureTransition{
				{eventType: "order.created", to: "pending"},
				{eventType: "order.confirmed", from: "pending", to: "confirmed"},
				{eventType: "order.packed", from: "confirmed", to: "packed"},
				{eventType: "order.shipped", from: "packed", to: "shipped"},
			},
		},
		{
			orderID: "order-2001",
			transitions: []fixtureTransition{
				{eventType: "order.created", to: "pending"},
				{eventType: "order.cancelled", from: "pending", to: "cancelled", reason: "customer request"},
			},
		},
	}
	base := time.Date(2026, time.July, 13, 9, 0, 0, 0, time.UTC)
	for fixtureIndex, fixture := range fixtures {
		entries := make([]audit.Entry, len(fixture.transitions))
		for index, transition := range fixture.transitions {
			recordedAt := base.Add(time.Duration(fixtureIndex*10+index) * time.Minute)
			entry, err := newFixtureEntry(fixture.orderID, audit.Revision(index+1), transition, recordedAt)
			if err != nil {
				return fmt.Errorf("build %s revision %d: %w", fixture.orderID, index+1, err)
			}
			entries[index] = entry
		}
		if err := repository.Append(ctx, entries...); err != nil {
			return fmt.Errorf("append %s history: %w", fixture.orderID, err)
		}
	}
	return nil
}

func newFixtureEntry(orderID string, revision audit.Revision, transition fixtureTransition, recordedAt time.Time) (audit.Entry, error) {
	aggregate, err := audit.NewAggregateID("order", orderID)
	if err != nil {
		return audit.Entry{}, err
	}
	payload, err := json.Marshal(struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
		Reason  string `json:"reason,omitempty"`
	}{OrderID: orderID, Status: transition.to, Reason: transition.reason})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("marshal payload: %w", err)
	}
	eventID := fmt.Sprintf("%s-event-%d", orderID, revision)
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID:        audit.EventID(eventID),
		EventType:      transition.eventType,
		AggregateID:    aggregate,
		Revision:       revision,
		OccurredAt:     recordedAt,
		RecordedAt:     recordedAt,
		IdempotencyKey: fmt.Sprintf("%s-command-%d", orderID, revision),
		Metadata:       audit.Metadata{"source": "fixture"},
		Payload:        payload,
	})
	if err != nil {
		return audit.Entry{}, err
	}
	attributes := audit.Metadata{"to": transition.to}
	if transition.from != "" {
		attributes["from"] = transition.from
	}
	if transition.reason != "" {
		attributes["reason"] = transition.reason
	}
	change, err := audit.NewChangeMetadata([]string{"status"}, string(transition.eventType), attributes)
	if err != nil {
		return audit.Entry{}, err
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: "workshop", Event: event, Change: &change})
	if err != nil {
		return audit.Entry{}, err
	}
	return entry, nil
}
