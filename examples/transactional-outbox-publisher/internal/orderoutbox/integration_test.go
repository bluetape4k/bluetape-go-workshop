package orderoutbox

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func TestRedisStreamsIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	postgresURL := postgrestestcontainer.Start(ctx, t)
	redisAddr := redistestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", postgresURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close postgres: %v", err)
		}
	})
	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close redis: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	now := time.Date(2026, 7, 14, 11, 12, 13, 456000000, time.UTC)
	store := newClockedStore(t, func() time.Time { return now })
	service := newClockedService(t, store, func() time.Time { return now })
	if err := service.CreateSchema(ctx, db); err != nil {
		t.Fatalf("CreateSchema() error = %v", err)
	}
	command := relayCommand("redis-streams")
	placeRelayOrder(ctx, t, service, db, command)

	const stream = "workshop:transactional-outbox"
	publisher, err := redisstreams.New(redisstreams.Options{Client: client, Stream: stream})
	if err != nil {
		t.Fatalf("redisstreams.New() error = %v", err)
	}
	if publisher.Stream() != stream {
		t.Fatalf("publisher stream = %q, want %q", publisher.Stream(), stream)
	}
	relay := newTestRelay(t, store, publisher, 1, func() time.Time { return now })
	result, err := relay.RunOnce(ctx, db)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	assertRelayResult(t, result, sqloutbox.RelayResult{Claimed: 1, Published: 1})

	messages, err := client.XRangeN(ctx, stream, "-", "+", 2).Result()
	if err != nil {
		t.Fatalf("XRangeN() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("stream messages = %d, want 1", len(messages))
	}
	values := messages[0].Values
	if len(values) != 13 {
		t.Fatalf("stream fields = %d, want 13: %#v", len(values), values)
	}

	var recordID int64
	var status sqloutbox.Status
	var attempts int
	if err := db.QueryRowContext(ctx, `select id, status, attempts from transactional_outbox_records`).Scan(&recordID, &status, &attempts); err != nil {
		t.Fatalf("read published outbox row: %v", err)
	}
	if status != sqloutbox.StatusPublished || attempts != 1 {
		t.Fatalf("SQL outbox state = (%s, %d), want (published, 1)", status, attempts)
	}

	wantFields := map[string]string{
		"record_id":       strconv.FormatInt(recordID, 10),
		"status":          string(sqloutbox.StatusClaimed),
		"aggregate_type":  aggregateType,
		"aggregate_id":    command.OrderID,
		"revision":        strconv.FormatUint(uint64(audit.InitialRevision()), 10),
		"event_id":        command.CommandID,
		"idempotency_key": command.CommandID,
		"event_type":      string(eventType),
		"occurred_at":     command.CreatedAt.UTC().Format(time.RFC3339Nano),
		"recorded_at":     now.UTC().Format(time.RFC3339Nano),
		"schema_version":  strconv.Itoa(audit.SchemaVersion),
		"attempts":        "1",
	}
	for field, want := range wantFields {
		got, ok := values[field]
		if !ok {
			t.Fatalf("stream field %q is missing", field)
		}
		if fmt.Sprint(got) != want {
			t.Fatalf("stream field %q = %q, want %q", field, got, want)
		}
	}
	entryJSON, ok := values["entry_json"]
	if !ok {
		t.Fatal("stream field entry_json is missing")
	}
	entry, err := audit.DecodeEntryJSON([]byte(fmt.Sprint(entryJSON)))
	if err != nil {
		t.Fatalf("audit.DecodeEntryJSON() error = %v", err)
	}
	if entry.Aggregate.Type != wantFields["aggregate_type"] || entry.Aggregate.ID != wantFields["aggregate_id"] {
		t.Fatalf("decoded aggregate = %s:%s, want %s:%s", entry.Aggregate.Type, entry.Aggregate.ID, wantFields["aggregate_type"], wantFields["aggregate_id"])
	}
	if strconv.FormatUint(uint64(entry.Revision), 10) != wantFields["revision"] ||
		string(entry.Event.EventID) != wantFields["event_id"] ||
		entry.Event.IdempotencyKey != wantFields["idempotency_key"] ||
		string(entry.Event.EventType) != wantFields["event_type"] ||
		entry.SchemaVersion != audit.SchemaVersion {
		t.Fatalf("decoded entry does not match stream scalar fields: %#v", entry)
	}
}
