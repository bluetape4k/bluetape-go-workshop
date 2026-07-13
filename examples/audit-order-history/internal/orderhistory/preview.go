package orderhistory

import (
	"context"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

// Preview contrasts current order state with full and filtered audit history.
type Preview struct {
	Current       Order       `json:"current"`
	History       []EntryView `json:"history"`
	RecentHistory []EntryView `json:"recent_history"`
}

// EntryView is the stable JSON projection of one audit entry.
type EntryView struct {
	EventID      string         `json:"event_id"`
	EventType    string         `json:"event_type"`
	Revision     audit.Revision `json:"revision"`
	OccurredAt   time.Time      `json:"occurred_at"`
	Author       string         `json:"author"`
	StatusBefore Status         `json:"status_before,omitempty"`
	StatusAfter  Status         `json:"status_after"`
}

// BuildPreview executes a deterministic create-confirm-ship lifecycle.
func BuildPreview(ctx context.Context, service *Service) (Preview, error) {
	if service == nil {
		return Preview{}, fmt.Errorf("%w: service is required", ErrInvalidConfig)
	}
	if _, err := service.Create(ctx, CreateCommand{OrderID: "order-1001", CommandID: "command-create-1001"}); err != nil {
		return Preview{}, fmt.Errorf("create preview order: %w", err)
	}
	if _, err := service.Confirm(ctx, TransitionCommand{OrderID: "order-1001", CommandID: "command-confirm-1001"}); err != nil {
		return Preview{}, fmt.Errorf("confirm preview order: %w", err)
	}
	current, err := service.Ship(ctx, TransitionCommand{OrderID: "order-1001", CommandID: "command-ship-1001"})
	if err != nil {
		return Preview{}, fmt.Errorf("ship preview order: %w", err)
	}
	history, ok, err := service.History(ctx, "order-1001")
	if err != nil {
		return Preview{}, err
	}
	if !ok {
		return Preview{}, fmt.Errorf("preview history missing after commit")
	}
	aggregate, err := audit.NewAggregateID(aggregateType, "order-1001")
	if err != nil {
		return Preview{}, fmt.Errorf("create preview aggregate: %w", err)
	}
	recent, err := service.Find(ctx, audit.Query{
		Aggregate: &aggregate, FromRevision: 2, NewestFirst: true, Limit: 2,
	})
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		Current: current, History: projectEntries(history.Entries()), RecentHistory: projectEntries(recent),
	}, nil
}

func projectEntries(entries []audit.Entry) []EntryView {
	views := make([]EntryView, 0, len(entries))
	for _, entry := range entries {
		view := EntryView{
			EventID: string(entry.Event.EventID), EventType: string(entry.Event.EventType),
			Revision: entry.Revision, OccurredAt: entry.Event.OccurredAt, Author: entry.Author,
		}
		if entry.Change != nil {
			view.StatusBefore = Status(entry.Change.Attributes["status_before"])
			view.StatusAfter = Status(entry.Change.Attributes["status_after"])
		}
		views = append(views, view)
	}
	return views
}
