package redisadmission

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestServiceAdmitsFirstInsertAndRejectsRepeatedValue(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	service := newTestService(ctx, t, client, "instance-a", namespace(t), 500)

	first, err := service.Admit(ctx, EventRequest{EventID: " evt-1001 ", Source: "checkout"})
	if err != nil {
		t.Fatalf("first Admit() error = %v", err)
	}
	second, err := service.Admit(ctx, EventRequest{EventID: "evt-1001", Source: "checkout"})
	if err != nil {
		t.Fatalf("second Admit() error = %v", err)
	}

	if first.EventID != "evt-1001" || first.Decision != DecisionAdmit || !first.Accepted || first.Reason != ReasonDefinitelyNew {
		t.Fatalf("first response = %+v, want definitely-new admit", first)
	}
	if second.Decision != DecisionProbablySeen || second.Accepted || second.Reason != ReasonMightBeDuplicate {
		t.Fatalf("second response = %+v, want probably-seen rejection", second)
	}
	if second.Stats.ApproximateElementCount == 0 || !second.Stats.SharedRedisState {
		t.Fatalf("stats = %+v, want shared non-empty filter", second.Stats)
	}
}

func TestServiceSharesVisibilityAcrossInstances(t *testing.T) {
	ctx := context.Background()
	addr := redistestcontainer.Start(ctx, t)
	clientA := redis.NewClient(&redis.Options{Addr: addr})
	clientB := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = clientB.Close() })
	t.Cleanup(func() { _ = clientA.Close() })
	waitForRedis(ctx, t, clientA, clientB)

	namespace := namespace(t)
	instanceA := newTestService(ctx, t, clientA, "instance-a", namespace, 500)
	instanceB := newTestService(ctx, t, clientB, "instance-b", namespace, 500)

	first, err := instanceA.Admit(ctx, EventRequest{EventID: "evt-shared-1001", Source: "billing"})
	if err != nil {
		t.Fatalf("instance A Admit() error = %v", err)
	}
	second, err := instanceB.Admit(ctx, EventRequest{EventID: "evt-shared-1001", Source: "billing"})
	if err != nil {
		t.Fatalf("instance B Admit() error = %v", err)
	}

	if first.Decision != DecisionAdmit || !first.Accepted {
		t.Fatalf("first response = %+v, want admit", first)
	}
	if second.Instance != "instance-b" || second.Decision != DecisionProbablySeen || second.Accepted {
		t.Fatalf("second response = %+v, want shared probably seen from instance-b", second)
	}
}

func TestServiceRejectsConfigMismatch(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	namespace := namespace(t)
	_ = newTestService(ctx, t, client, "instance-a", namespace, 500)

	_, err := NewService(ctx, client, Config{
		Namespace:                namespace,
		InstanceID:               "instance-b",
		ExpectedInsertions:       600,
		FalsePositiveProbability: 0.01,
	})

	if !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("NewService() error = %v, want ErrConfigMismatch", err)
	}
}

func TestServiceMapsRedisFailure(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	service := newTestService(ctx, t, client, "instance-a", namespace(t), 500)
	if err := client.Close(); err != nil {
		t.Fatalf("close redis client: %v", err)
	}

	_, err := service.Admit(ctx, EventRequest{EventID: "evt-redis-down", Source: "checkout"})

	if !errors.Is(err, ErrFilterUnavailable) {
		t.Fatalf("Admit() error = %v, want ErrFilterUnavailable", err)
	}
}

func TestServicePreservesCancellation(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	service := newTestService(ctx, t, client, "instance-a", namespace(t), 500)
	canceled, cancel := context.WithCancel(ctx)
	cancel()

	_, err := service.Admit(canceled, EventRequest{EventID: "evt-canceled", Source: "checkout"})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Admit() error = %v, want context.Canceled", err)
	}
}

func TestServiceRejectsInvalidEventID(t *testing.T) {
	ctx := context.Background()
	client := redisClient(ctx, t)
	service := newTestService(ctx, t, client, "instance-a", namespace(t), 500)

	_, err := service.Admit(ctx, EventRequest{EventID: "  ", Source: "checkout"})

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Admit() error = %v, want ErrInvalidRequest", err)
	}
}

func redisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = client.Close() })
	waitForRedis(ctx, t, client)
	return client
}

func newTestService(ctx context.Context, t *testing.T, client redis.Cmdable, instanceID string, namespace string, expectedInsertions uint64) *Service {
	t.Helper()
	service, err := NewService(ctx, client, Config{
		Namespace:                namespace,
		InstanceID:               instanceID,
		ScenarioName:             "test-shared-redis-bloom-admission",
		ExpectedInsertions:       expectedInsertions,
		FalsePositiveProbability: 0.01,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func namespace(t *testing.T) string {
	t.Helper()
	return "shared-redis-bloom-admission:" + strings.ReplaceAll(t.Name(), "/", ":")
}

func waitForRedis(ctx context.Context, t *testing.T, clients ...*redis.Client) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for _, client := range clients {
			if client.Ping(ctx).Err() != nil {
				ready = false
				break
			}
		}
		if ready {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, client := range clients {
		if err := client.Ping(ctx).Err(); err != nil {
			t.Fatalf("redis ping: %v", err)
		}
	}
}
