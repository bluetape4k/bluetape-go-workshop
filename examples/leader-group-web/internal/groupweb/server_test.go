package groupweb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/leader-group-web/internal/groupweb"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestGroupWebCampaignStatusAndResign(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = client.Close() })

	elector := newGroupElector(t, client, "group-web", "member-1", 2)
	server := groupweb.NewServer(elector, "member-1")

	campaign := httptest.NewRecorder()
	server.ServeHTTP(campaign, httptest.NewRequestWithContext(ctx, http.MethodPost, "/campaign", nil))
	if campaign.Code != http.StatusOK {
		t.Fatalf("campaign status = %d body = %s", campaign.Code, campaign.Body.String())
	}

	status := httptest.NewRecorder()
	server.ServeHTTP(status, httptest.NewRequestWithContext(ctx, http.MethodGet, "/group", nil))
	var body struct {
		Active    int  `json:"active"`
		Available int  `json:"available"`
		IsLeader  bool `json:"is_leader"`
	}
	if err := json.NewDecoder(status.Body).Decode(&body); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if body.Active != 1 || body.Available != 1 || !body.IsLeader {
		t.Fatalf("group body = %+v", body)
	}

	resign := httptest.NewRecorder()
	server.ServeHTTP(resign, httptest.NewRequestWithContext(ctx, http.MethodPost, "/resign", nil))
	if resign.Code != http.StatusNoContent {
		t.Fatalf("resign status = %d", resign.Code)
	}
}

func TestGroupWebStressBoundsCampaignContention(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = client.Close() })

	const maxLeaders = 2
	tasks := make([]concurrencytest.Task, 0, 6)
	for i := 0; i < 6; i++ {
		index := i
		tasks = append(tasks, func(ctx context.Context) error {
			member := fmt.Sprintf("member-%d", index)
			elector := newGroupElector(t, client, "group-web-stress", member, maxLeaders)
			campaignCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			if err := elector.Campaign(campaignCtx); err != nil {
				return err
			}
			defer func() {
				_ = elector.Resign(context.Background())
			}()
			active, err := elector.ActiveCount(ctx)
			if err != nil {
				return err
			}
			if active > maxLeaders {
				return fmt.Errorf("active leaders exceeded max: %d", active)
			}
			time.Sleep(5 * time.Millisecond)
			return nil
		})
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 6, RoundsPerTask: 2, Timeout: 5 * time.Second})
	report := tester.RunT(t, tasks...)
	if report.Completed != 12 {
		t.Fatalf("report = %+v", report)
	}
}

func newGroupElector(t *testing.T, client redis.Cmdable, group, member string, maxLeaders int) *redisleader.GroupElector {
	t.Helper()
	elector, err := redisleader.NewGroup(client, leader.GroupOptions{
		Options: leader.Options{
			Group:         group,
			MemberID:      member,
			Lease:         time.Second,
			RenewInterval: 100 * time.Millisecond,
		},
		MaxLeaders: maxLeaders,
	})
	if err != nil {
		t.Fatalf("new group elector: %v", err)
	}
	return elector
}
