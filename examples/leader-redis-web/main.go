// Package main runs the Redis leader election web example.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/leader-redis-web/internal/leaderweb"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	redisAddr := env("REDIS_ADDR", "localhost:6379")
	memberID := env("MEMBER_ID", "leader-web-1")
	group := env("ELECTION_GROUP", "leader-redis-web")
	httpAddr := env("HTTP_ADDR", ":8080")

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close redis client: %v", err)
		}
	}()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("ping redis at %s: %v", redisAddr, err)
	}

	elector, err := redisleader.New(client, leader.Options{
		Group:         group,
		MemberID:      memberID,
		Lease:         10 * time.Second,
		RenewInterval: 2 * time.Second,
	})
	if err != nil {
		log.Fatalf("create elector: %v", err)
	}

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           leaderweb.NewServer(elector, memberID),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("leader Redis web example listening on %s, redis=%s, member=%s, group=%s", httpAddr, redisAddr, memberID, group)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
