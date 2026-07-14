package orderworkflow

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestNormalizeIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "trim", value: " order-1001 ", want: "order-1001"},
		{name: "allowed separators", value: "order:west.1_test", want: "order:west.1_test"},
		{name: "blank", value: " ", wantErr: true},
		{name: "leading separator", value: "-order", wantErr: true},
		{name: "newline", value: "order\nforged", wantErr: true},
		{name: "non ascii", value: "주문", wantErr: true},
		{name: "oversized", value: strings.Repeat("a", 129), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeIdentifier(tt.value)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCommand) {
					t.Fatalf("normalizeIdentifier(%q) error = %v", tt.value, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("normalizeIdentifier(%q) = (%q, %v), want (%q, nil)", tt.value, got, err, tt.want)
			}
		})
	}
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata audit.Metadata
		wantErr  bool
	}{
		{name: "nil"},
		{name: "valid", metadata: audit.Metadata{"channel": "workshop"}},
		{name: "blank key", metadata: audit.Metadata{" ": "value"}, wantErr: true},
		{name: "oversized key", metadata: audit.Metadata{strings.Repeat("k", 65): "value"}, wantErr: true},
		{name: "oversized value", metadata: audit.Metadata{"key": strings.Repeat("v", 513)}, wantErr: true},
		{name: "too many", metadata: metadataWithEntries(33), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateMetadata(tt.metadata)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCommand) {
					t.Fatalf("validateMetadata() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateMetadata() error = %v", err)
			}
			if len(got) != len(tt.metadata) {
				t.Fatalf("validateMetadata() length = %d, want %d", len(got), len(tt.metadata))
			}
		})
	}
}

func TestValidateCommands(t *testing.T) {
	create, err := (CreateCommand{
		OrderID:   " order-1001 ",
		CommandID: " cmd-create-1001 ",
		Metadata:  audit.Metadata{"channel": "workshop"},
	}).validate()
	if err != nil {
		t.Fatalf("CreateCommand.validate() error = %v", err)
	}
	if create.OrderID != "order-1001" || create.CommandID != "cmd-create-1001" {
		t.Fatalf("CreateCommand.validate() = %#v", create)
	}

	transition, err := (TransitionCommand{
		OrderID:   "order-1001",
		CommandID: "cmd-cancel-1001",
		Action:    ActionCancel,
		Reason:    " customer request ",
	}).validate()
	if err != nil {
		t.Fatalf("TransitionCommand.validate() error = %v", err)
	}
	if transition.Reason != "customer request" {
		t.Fatalf("TransitionCommand.validate() reason = %q", transition.Reason)
	}

	invalid := []TransitionCommand{
		{OrderID: "order-1001", CommandID: "cmd-1", Action: "ship"},
		{OrderID: "order-1001", CommandID: "cmd-1", Action: ActionConfirm, Reason: "not allowed"},
		{OrderID: "order-1001", CommandID: "cmd-1", Action: ActionCancel, Reason: strings.Repeat("가", 501)},
	}
	for _, command := range invalid {
		if _, err := command.validate(); !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("TransitionCommand.validate(%#v) error = %v", command, err)
		}
	}
}

func TestNormalizeTimestampUsesUTCPostgreSQLPrecision(t *testing.T) {
	value := time.Date(2026, 7, 14, 12, 0, 0, 123456789, time.FixedZone("KST", 9*60*60))
	got := normalizeTimestamp(value)
	want := time.Date(2026, 7, 14, 3, 0, 0, 123456000, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("normalizeTimestamp() = %v, want %v", got, want)
	}
}

func TestBuildEntryPreservesCanonicalIntentAndOrder(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 0, 0, 123456000, time.UTC)
	command := TransitionCommand{
		OrderID:   "order-1001",
		CommandID: "cmd-confirm-1001",
		Action:    ActionConfirm,
		Metadata:  audit.Metadata{"operator": "demo"},
	}
	order := Order{OrderID: command.OrderID, Status: StatusConfirmed, Revision: 2, UpdatedAt: now}

	entry, err := buildTransitionEntry("workshop", now, command, order)
	if err != nil {
		t.Fatalf("buildTransitionEntry() error = %v", err)
	}
	if entry.Aggregate != (audit.AggregateID{Type: orderAggregateType, ID: command.OrderID}) {
		t.Fatalf("entry aggregate = %#v", entry.Aggregate)
	}
	if entry.Event.EventID != audit.EventID(command.CommandID) || entry.Event.IdempotencyKey != command.CommandID {
		t.Fatalf("entry identity = (%q, %q)", entry.Event.EventID, entry.Event.IdempotencyKey)
	}
	if entry.Event.EventType != audit.EventType("order.confirmed") || entry.Revision != 2 {
		t.Fatalf("entry type/revision = (%q, %d)", entry.Event.EventType, entry.Revision)
	}

	payload, err := decodeEventPayload(entry)
	if err != nil {
		t.Fatalf("decodeEventPayload() error = %v", err)
	}
	if !payload.Intent.matchesTransition(command) || payload.Order != order {
		t.Fatalf("payload = %#v", payload)
	}

	command.Metadata["operator"] = "mutated"
	if entry.Event.Metadata["operator"] != "demo" {
		t.Fatalf("entry metadata changed with caller map: %#v", entry.Event.Metadata)
	}
}

func metadataWithEntries(count int) audit.Metadata {
	metadata := make(audit.Metadata, count)
	for index := range count {
		metadata["key"+string(rune('A'+index))] = "value"
	}
	return metadata
}
