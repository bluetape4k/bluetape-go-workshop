// Package main 은 Redis 리더 그룹 웹 예제를 실행한다.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/leader-group-web/internal/groupweb"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	redisAddr := env("REDIS_ADDR", "localhost:6379")
	memberID := env("MEMBER_ID", "group-web-1")
	group := env("ELECTION_GROUP", "leader-group-web")
	httpAddr := env("HTTP_ADDR", ":8082")
	maxLeaders := envInt("MAX_LEADERS", 2)

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close redis client: %v", err)
		}
	}()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("ping redis at %s: %v", redisAddr, err)
	}

	elector, err := redisleader.NewGroup(client, leader.GroupOptions{
		Options: leader.Options{
			Group:         group,
			MemberID:      memberID,
			Lease:         10 * time.Second,
			RenewInterval: 2 * time.Second,
		},
		MaxLeaders: maxLeaders,
	})
	if err != nil {
		log.Fatalf("create group elector: %v", err)
	}

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           groupweb.NewServer(elector, memberID),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("leader group web example listening on %s, redis=%s, member=%s, group=%s max=%d", httpAddr, redisAddr, memberID, group, maxLeaders)
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

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
