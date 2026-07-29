package orderworkflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func TestIntegrationPostgreSQLThenRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	postgresURL := postgrestestcontainer.Start(ctx, t)
	db, err := sql.Open("pgx", postgresURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ConfigureDatabase(db)
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 3, 0, 0, 123456000, time.UTC)
	outbox := newClockedWorkflowOutbox(t, func() time.Time { return now })
	history, err := NewHistoryStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateSchema(ctx, db, outbox); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, history, outbox, Config{Author: "workshop", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := service.Create(ctx, CreateCommand{OrderID: "order-integration", CommandID: "cmd-create-integration"})
	if err != nil || replayed || created.Revision != 1 {
		t.Fatalf("Create() = (%+v, %v, %v)", created, replayed, err)
	}
	now = now.Add(time.Second)
	confirmed, replayed, err := service.Transition(ctx, TransitionCommand{OrderID: created.OrderID, CommandID: "cmd-confirm-integration", Action: ActionConfirm})
	if err != nil || replayed || confirmed.Revision != 2 {
		t.Fatalf("Transition() = (%+v, %v, %v)", confirmed, replayed, err)
	}

	// Redis 가 존재하기 전에 동일한 PostgreSQL 데이터베이스 위에서 모든 애플리케이션 객체를 재구성한다.
	restartedOutbox := newClockedWorkflowOutbox(t, func() time.Time { return now })
	restartedHistory, err := NewHistoryStore(db)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(db, restartedHistory, restartedOutbox, Config{Author: "workshop", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	replayedOrder, replayed, err := restarted.Transition(ctx, TransitionCommand{OrderID: created.OrderID, CommandID: "cmd-confirm-integration", Action: ActionConfirm})
	if err != nil || !replayed || replayedOrder != confirmed {
		t.Fatalf("Transition(restart replay) = (%+v, %v, %v)", replayedOrder, replayed, err)
	}
	aggregate, _ := audit.NewAggregateID(orderAggregateType, created.OrderID)
	entries, err := restartedHistory.Find(ctx, audit.Query{Aggregate: &aggregate})
	if err != nil || len(entries) != 2 {
		t.Fatalf("persisted history = (%d, %v)", len(entries), err)
	}
	assertWorkflowTableCount(ctx, t, db, outboxTable, 2)
	if db.Stats().MaxOpenConnections != 8 {
		t.Fatalf("MaxOpenConnections = %d", db.Stats().MaxOpenConnections)
	}

	// 위의 영속 PostgreSQL 및 재시작 검증이 끝난 뒤에만 Redis를 시작한다.
	redisAddress := redistestcontainer.Start(ctx, t)
	client := redis.NewClient(NewRedisOptions(redisAddress))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	const stream = "workshop:audited-order-workflow"
	publisher, err := redisstreams.New(redisstreams.Options{Client: client, Stream: stream})
	if err != nil {
		t.Fatal(err)
	}
	relay := newWorkflowRelay(t, restartedOutbox, publisher, func() time.Time { return now })
	for published := 0; published < 2; {
		result, err := relay.RunOnce(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		published += result.Published
	}
	messages, err := client.XRange(ctx, stream, "-", "+").Result()
	if err != nil || len(messages) != 2 {
		t.Fatalf("stream messages = (%d, %v)", len(messages), err)
	}
	for index, message := range messages {
		if len(message.Values) != 13 {
			t.Fatalf("message %d fields = %d", index, len(message.Values))
		}
		entry, err := audit.DecodeEntryJSON([]byte(fmt.Sprint(message.Values["entry_json"])))
		if err != nil {
			t.Fatalf("decode message %d: %v", index, err)
		}
		wantRevision := audit.Revision(index + 1)
		wantEventID := []string{"cmd-create-integration", "cmd-confirm-integration"}[index]
		if entry.Revision != wantRevision || string(entry.Event.EventID) != wantEventID ||
			fmt.Sprint(message.Values["revision"]) != strconv.Itoa(index+1) ||
			fmt.Sprint(message.Values["event_id"]) != wantEventID ||
			fmt.Sprint(message.Values["idempotency_key"]) != wantEventID {
			t.Fatalf("message %d identity = %#v", index, message.Values)
		}
	}

	t.Run("concurrent writers and relay stay within the pool ceiling", func(t *testing.T) {
		const writerCount = 24
		runCtx, stopRelay := context.WithCancel(ctx)
		relayDone := make(chan error, 1)
		go func() { relayDone <- relay.Run(runCtx, db) }()
		var peak atomic.Int64
		sampleDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-sampleDone:
					return
				case <-ticker.C:
					inUse := int64(db.Stats().InUse)
					for current := peak.Load(); inUse > current; current = peak.Load() {
						if peak.CompareAndSwap(current, inUse) {
							break
						}
					}
				}
			}
		}()
		before := db.Stats()
		errorsCh := make(chan error, writerCount)
		var writers sync.WaitGroup
		writers.Add(writerCount)
		for index := range writerCount {
			go func(index int) {
				defer writers.Done()
				operationCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				_, _, err := restarted.Create(operationCtx, CreateCommand{
					OrderID: fmt.Sprintf("order-backlog-%02d", index), CommandID: fmt.Sprintf("cmd-backlog-%02d", index),
				})
				errorsCh <- err
			}(index)
		}
		writers.Wait()
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Fatalf("writer error = %v", err)
			}
		}
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			status, err := restartedHistory.DeliveryStatus(ctx, now)
			if err != nil {
				t.Fatal(err)
			}
			if status.Pending == 0 && status.Retrying == 0 && status.Claimed == 0 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		stopRelay()
		if err := <-relayDone; err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Relay.Run() error = %v", err)
		}
		close(sampleDone)
		after := db.Stats()
		if peak.Load() > 8 || after.MaxOpenConnections != 8 {
			t.Fatalf("pool peak/max = %d/%d", peak.Load(), after.MaxOpenConnections)
		}
		waitDuration := after.WaitDuration - before.WaitDuration
		if waitDuration >= 2*time.Second*writerCount {
			t.Fatalf("pool WaitDuration = %s", waitDuration)
		}
		status, err := restartedHistory.DeliveryStatus(ctx, now)
		if err != nil || status.Pending != 0 || status.Retrying != 0 || status.Claimed != 0 {
			t.Fatalf("final delivery status = (%+v, %v)", status, err)
		}
	})
}
