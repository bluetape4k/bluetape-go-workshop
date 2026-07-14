package orderworkflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
)

const (
	orderAggregateType    = "order"
	maxIdentifierBytes    = 128
	maxReasonRunes        = 500
	maxMetadataEntries    = 32
	maxMetadataKeyRunes   = 64
	maxMetadataValueRunes = 512
)

var (
	ErrInvalidConfig  = errors.New("orderworkflow: invalid config")
	ErrInvalidCommand = errors.New("orderworkflow: invalid command")
	ErrInvalidEntry   = errors.New("orderworkflow: invalid entry")
	ErrNotFound       = errors.New("orderworkflow: not found")
	ErrConflict       = errors.New("orderworkflow: conflict")

	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

type Action string

const (
	ActionConfirm Action = "confirm"
	ActionCancel  Action = "cancel"
)

type Order struct {
	OrderID   string         `json:"order_id"`
	Status    Status         `json:"status"`
	Revision  audit.Revision `json:"revision"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type CreateCommand struct {
	OrderID   string
	CommandID string
	Metadata  audit.Metadata
}

type TransitionCommand struct {
	OrderID   string
	CommandID string
	Action    Action
	Reason    string
	Metadata  audit.Metadata
}

type commandIntent struct {
	OrderID  string         `json:"order_id"`
	Action   string         `json:"action"`
	Reason   string         `json:"reason,omitempty"`
	Metadata audit.Metadata `json:"metadata,omitempty"`
}

type eventPayload struct {
	Intent commandIntent `json:"intent"`
	Order  Order         `json:"order"`
}

func normalizeIdentifier(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if len(normalized) == 0 || len(normalized) > maxIdentifierBytes || !identifierPattern.MatchString(normalized) {
		return "", fmt.Errorf("%w: identifier", ErrInvalidCommand)
	}
	return normalized, nil
}

func validateMetadata(metadata audit.Metadata) (audit.Metadata, error) {
	if len(metadata) > maxMetadataEntries {
		return nil, fmt.Errorf("%w: metadata count", ErrInvalidCommand)
	}
	validated := metadata.Clone()
	for key, value := range validated {
		if !utf8.ValidString(key) || strings.TrimSpace(key) == "" || utf8.RuneCountInString(key) > maxMetadataKeyRunes {
			return nil, fmt.Errorf("%w: metadata key", ErrInvalidCommand)
		}
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxMetadataValueRunes {
			return nil, fmt.Errorf("%w: metadata value", ErrInvalidCommand)
		}
	}
	return validated, nil
}

func (command CreateCommand) validate() (CreateCommand, error) {
	orderID, err := normalizeIdentifier(command.OrderID)
	if err != nil {
		return CreateCommand{}, err
	}
	commandID, err := normalizeIdentifier(command.CommandID)
	if err != nil {
		return CreateCommand{}, err
	}
	metadata, err := validateMetadata(command.Metadata)
	if err != nil {
		return CreateCommand{}, err
	}
	return CreateCommand{OrderID: orderID, CommandID: commandID, Metadata: metadata}, nil
}

func (command TransitionCommand) validate() (TransitionCommand, error) {
	orderID, err := normalizeIdentifier(command.OrderID)
	if err != nil {
		return TransitionCommand{}, err
	}
	commandID, err := normalizeIdentifier(command.CommandID)
	if err != nil {
		return TransitionCommand{}, err
	}
	reason := strings.TrimSpace(command.Reason)
	if !utf8.ValidString(reason) || utf8.RuneCountInString(reason) > maxReasonRunes {
		return TransitionCommand{}, fmt.Errorf("%w: reason", ErrInvalidCommand)
	}
	if command.Action != ActionConfirm && command.Action != ActionCancel {
		return TransitionCommand{}, fmt.Errorf("%w: action", ErrInvalidCommand)
	}
	if command.Action == ActionConfirm && reason != "" {
		return TransitionCommand{}, fmt.Errorf("%w: confirm reason", ErrInvalidCommand)
	}
	metadata, err := validateMetadata(command.Metadata)
	if err != nil {
		return TransitionCommand{}, err
	}
	return TransitionCommand{
		OrderID: orderID, CommandID: commandID, Action: command.Action,
		Reason: reason, Metadata: metadata,
	}, nil
}

func normalizeTimestamp(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func buildCreateEntry(author string, now time.Time, command CreateCommand, order Order) (audit.Entry, error) {
	return buildEntry(author, now, command.CommandID, "order.created", order, commandIntent{
		OrderID: command.OrderID, Action: "create", Metadata: command.Metadata,
	})
}

func buildTransitionEntry(author string, now time.Time, command TransitionCommand, order Order) (audit.Entry, error) {
	return buildEntry(author, now, command.CommandID, audit.EventType("order."+string(order.Status)), order, commandIntent{
		OrderID: command.OrderID, Action: string(command.Action), Reason: command.Reason, Metadata: command.Metadata,
	})
}

func buildEntry(author string, now time.Time, commandID string, eventType audit.EventType, order Order, intent commandIntent) (audit.Entry, error) {
	aggregate, err := audit.NewAggregateID(orderAggregateType, order.OrderID)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%w: aggregate: %w", ErrInvalidEntry, err)
	}
	payload, err := json.Marshal(eventPayload{Intent: intent, Order: order})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%w: payload: %w", ErrInvalidEntry, err)
	}
	event, err := audit.NewDomainEvent(audit.EventOptions{
		EventID: audit.EventID(commandID), EventType: eventType,
		AggregateID: aggregate, Revision: order.Revision,
		OccurredAt: now, RecordedAt: now, IdempotencyKey: commandID,
		Metadata: intent.Metadata, Payload: payload,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%w: event: %w", ErrInvalidEntry, err)
	}
	entry, err := audit.NewEntry(audit.EntryOptions{Author: author, Event: event})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%w: entry: %w", ErrInvalidEntry, err)
	}
	return entry, nil
}

func decodeEventPayload(entry audit.Entry) (eventPayload, error) {
	var payload eventPayload
	if err := json.Unmarshal(entry.Event.Payload, &payload); err != nil {
		return eventPayload{}, fmt.Errorf("%w: decode payload: %w", ErrInvalidEntry, err)
	}
	return payload, nil
}

func (intent commandIntent) matchesCreate(command CreateCommand) bool {
	return intent.OrderID == command.OrderID && intent.Action == "create" && intent.Reason == "" && metadataEqual(intent.Metadata, command.Metadata)
}

func (intent commandIntent) matchesTransition(command TransitionCommand) bool {
	return intent.OrderID == command.OrderID && intent.Action == string(command.Action) && intent.Reason == command.Reason && metadataEqual(intent.Metadata, command.Metadata)
}

func metadataEqual(left audit.Metadata, right audit.Metadata) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
