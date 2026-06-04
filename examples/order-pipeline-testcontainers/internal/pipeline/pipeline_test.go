package pipeline_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	natstestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/nats"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

func TestOrderPipelineUsesRepositoryFixtures(t *testing.T) {
	ctx := context.Background()

	postgresURL := postgrestestcontainer.Start(ctx, t)
	redisAddr := redistestcontainer.Start(ctx, t)
	natsURL := natstestcontainer.Start(ctx, t)

	db, err := sql.Open("pgx", postgresURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = client.Close() })

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(nc.Close)

	if _, err := db.ExecContext(ctx, `create table order_projection (id text primary key, status text not null)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `insert into order_projection(id, status) values($1, $2)`, "ord-1", "accepted"); err != nil {
		t.Fatalf("insert projection: %v", err)
	}
	if ok, err := client.SetNX(ctx, "order:ord-1:processed", "1", time.Minute).Result(); err != nil {
		t.Fatalf("set idempotency marker: %v", err)
	} else if !ok {
		t.Fatal("expected idempotency marker to be created")
	}

	received := make(chan *nats.Msg, 1)
	sub, err := nc.ChanSubscribe("orders.accepted", received)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	if err := nc.Publish("orders.accepted", []byte("ord-1")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush nats: %v", err)
	}

	var status string
	if err := db.QueryRowContext(ctx, `select status from order_projection where id=$1`, "ord-1").Scan(&status); err != nil {
		t.Fatalf("read projection: %v", err)
	}
	if status != "accepted" {
		t.Fatalf("status = %s", status)
	}

	select {
	case msg := <-received:
		if string(msg.Data) != "ord-1" {
			t.Fatalf("event = %s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for nats event")
	}
}
