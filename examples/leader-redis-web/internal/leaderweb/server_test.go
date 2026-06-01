package leaderweb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/leader-redis-web/internal/leaderweb"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestLeaderWebCampaignLeaderAndResign(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:         "workshop-leader-web",
		MemberID:      "web-1",
		Lease:         2 * time.Second,
		RenewInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}

	server := leaderweb.NewServer(elector, "web-1")

	campaign := httptest.NewRequestWithContext(ctx, http.MethodPost, "/campaign", nil)
	campaignResponse := httptest.NewRecorder()
	server.ServeHTTP(campaignResponse, campaign)
	if campaignResponse.Code != http.StatusOK {
		t.Fatalf("campaign status = %d, body = %s", campaignResponse.Code, campaignResponse.Body.String())
	}

	leaderRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/leader", nil)
	leaderResponse := httptest.NewRecorder()
	server.ServeHTTP(leaderResponse, leaderRequest)
	if leaderResponse.Code != http.StatusOK {
		t.Fatalf("leader status = %d, body = %s", leaderResponse.Code, leaderResponse.Body.String())
	}

	var body struct {
		Leader   string `json:"leader"`
		IsLeader bool   `json:"is_leader"`
		Member   string `json:"member"`
	}
	if err := json.NewDecoder(leaderResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode leader response: %v", err)
	}
	if !body.IsLeader {
		t.Fatal("expected local server to be leader")
	}
	if body.Member != "web-1" {
		t.Fatalf("member = %q", body.Member)
	}
	if !strings.HasPrefix(body.Leader, "web-1:") {
		t.Fatalf("leader token = %q", body.Leader)
	}

	resign := httptest.NewRequestWithContext(ctx, http.MethodPost, "/resign", nil)
	resignResponse := httptest.NewRecorder()
	server.ServeHTTP(resignResponse, resign)
	if resignResponse.Code != http.StatusNoContent {
		t.Fatalf("resign status = %d, body = %s", resignResponse.Code, resignResponse.Body.String())
	}
}
