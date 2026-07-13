package orderhistory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestBuildPreviewContrastsCurrentAndHistory(t *testing.T) {
	t.Parallel()

	preview, err := BuildPreview(context.Background(), newTestService(t, audit.NewMemoryRepository()))
	if err != nil {
		t.Fatalf("BuildPreview() error = %v", err)
	}
	if preview.Current.OrderID != "order-1001" || preview.Current.Status != StatusShipped || preview.Current.Revision != 3 {
		t.Fatalf("Current = %+v", preview.Current)
	}
	if len(preview.History) != 3 || len(preview.RecentHistory) != 2 {
		t.Fatalf("history lengths = %d/%d", len(preview.History), len(preview.RecentHistory))
	}
	if preview.History[0].EventType != "order.created" || preview.History[2].EventType != "order.shipped" {
		t.Fatalf("History = %+v", preview.History)
	}
	if preview.RecentHistory[0].Revision != 3 || preview.RecentHistory[1].Revision != 2 {
		t.Fatalf("RecentHistory = %+v", preview.RecentHistory)
	}
	wantIDs := []string{"command-create-1001", "command-confirm-1001", "command-ship-1001"}
	wantTypes := []string{"order.created", "order.confirmed", "order.shipped"}
	wantBefore := []Status{"", StatusPending, StatusConfirmed}
	wantAfter := []Status{StatusPending, StatusConfirmed, StatusShipped}
	for i, entry := range preview.History {
		wantTime := time.Date(2026, 7, 13, 1, i, 0, 0, time.UTC)
		if entry.EventID != wantIDs[i] || entry.EventType != wantTypes[i] || entry.Revision != audit.Revision(i+1) ||
			!entry.OccurredAt.Equal(wantTime) || entry.Author != "workshop" ||
			entry.StatusBefore != wantBefore[i] || entry.StatusAfter != wantAfter[i] {
			t.Fatalf("History[%d] = %+v", i, entry)
		}
	}
}

func TestPreviewMarshalsDeterministically(t *testing.T) {
	t.Parallel()

	first, err := BuildPreview(context.Background(), newTestService(t, audit.NewMemoryRepository()))
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPreview(context.Background(), newTestService(t, audit.NewMemoryRepository()))
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("preview JSON differs:\n%s\n%s", firstJSON, secondJSON)
	}
}
