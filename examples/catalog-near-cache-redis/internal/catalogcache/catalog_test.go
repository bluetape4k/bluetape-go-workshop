package catalogcache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestPeerWriteInvalidatesPeerLocalCacheAndReloadsStore(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)
	namespace := namespace(t)
	store := newStore(t, Product{SKU: "sku-1", Name: "Original", Version: 1})

	peerA := newPeer(ctx, t, "peer-a", clientA, namespace, store)
	peerB := newPeer(ctx, t, "peer-b", clientB, namespace, store)

	initial, err := peerB.GetProduct(ctx, "sku-1")
	if err != nil {
		t.Fatalf("initial peer b load: %v", err)
	}
	if initial.Version != 1 {
		t.Fatalf("initial product = %+v", initial)
	}
	if loads := store.LoadCount("sku-1"); loads != 1 {
		t.Fatalf("initial load count = %d", loads)
	}

	if err := peerA.PutProduct(ctx, Product{SKU: "sku-1", Name: "Updated", Version: 2}); err != nil {
		t.Fatalf("peer a put: %v", err)
	}

	eventually(t, 2*time.Second, 10*time.Millisecond, func() bool {
		product, err := peerB.GetProduct(ctx, "sku-1")
		return err == nil && product.Name == "Updated" && product.Version == 2
	})
	if loads := store.LoadCount("sku-1"); loads != 2 {
		t.Fatalf("peer b should reload once after invalidation, load count = %d", loads)
	}
}

func TestColdMissBurstAcrossPeersRunsBackingLoaderOnce(t *testing.T) {
	ctx := context.Background()
	clientA, clientB := redisClients(ctx, t)
	namespace := namespace(t)
	store := newStore(t, Product{SKU: "sku-2", Name: "Coordinated", Version: 1})

	releaseLoader := make(chan struct{})
	loaderStarted := make(chan struct{}, 1)
	store.setLoadHook(func(ctx context.Context, sku string) error {
		if sku != "sku-2" {
			return nil
		}
		select {
		case loaderStarted <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseLoader:
			return nil
		}
	})

	peerA := newPeer(ctx, t, "peer-a", clientA, namespace, store)
	peerB := newPeer(ctx, t, "peer-b", clientB, namespace, store)

	results := make(chan loadResult, 2)
	go func() {
		product, err := peerA.GetProduct(ctx, "sku-2")
		results <- loadResult{product: product, err: err}
	}()
	go func() {
		product, err := peerB.GetProduct(ctx, "sku-2")
		results <- loadResult{product: product, err: err}
	}()

	select {
	case <-loaderStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first backing load")
	}
	eventually(t, 2*time.Second, 10*time.Millisecond, func() bool {
		return store.LoadCount("sku-2") == 1
	})
	time.Sleep(50 * time.Millisecond)
	close(releaseLoader)

	assertLoadResult(t, <-results, "sku-2", 1)
	assertLoadResult(t, <-results, "sku-2", 1)
	if loads := store.LoadCount("sku-2"); loads != 1 {
		t.Fatalf("backing loader should run once across peers, load count = %d", loads)
	}
}

func TestMissingProductIsNotCached(t *testing.T) {
	ctx := context.Background()
	clientA, _ := redisClients(ctx, t)
	store := newStore(t)
	peer := newPeer(ctx, t, "peer-a", clientA, namespace(t), store)

	_, err := peer.GetProduct(ctx, "missing")
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected product not found, got %v", err)
	}
	if loads := store.LoadCount("missing"); loads != 1 {
		t.Fatalf("missing product load count = %d", loads)
	}

	if err := store.Put(ctx, Product{SKU: "missing", Name: "Now Present", Version: 1}); err != nil {
		t.Fatalf("put missing product: %v", err)
	}
	product, err := peer.GetProduct(ctx, "missing")
	if err != nil {
		t.Fatalf("reload formerly missing product: %v", err)
	}
	if product.Name != "Now Present" {
		t.Fatalf("product = %+v", product)
	}
	if loads := store.LoadCount("missing"); loads != 2 {
		t.Fatalf("missing product should not be cached, load count = %d", loads)
	}
}

func TestPeerRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	clientA, _ := redisClients(ctx, t)
	store := newStore(t)

	if _, err := NewPeer(ctx, "", clientA, "catalog", store, Options{}); err == nil {
		t.Fatal("expected blank peer name to fail")
	}
	if _, err := NewPeer(ctx, "peer", nil, "catalog", store, Options{}); err == nil {
		t.Fatal("expected nil redis client to fail")
	}
	if _, err := NewPeer(ctx, "peer", clientA, " ", store, Options{}); err == nil {
		t.Fatal("expected blank namespace to fail")
	}
	if _, err := NewPeer(ctx, "peer", clientA, "catalog", nil, Options{}); err == nil {
		t.Fatal("expected nil store to fail")
	}
	if _, err := NewPeer(ctx, "peer", clientA, "catalog", store, Options{TTL: -time.Second}); err == nil {
		t.Fatal("expected negative ttl to fail")
	}

	peer := newPeer(ctx, t, "peer", clientA, namespace(t), store)
	if _, err := peer.GetProduct(ctx, " "); err == nil {
		t.Fatal("expected blank sku to fail")
	}
	if err := peer.PutProduct(ctx, Product{}); err == nil {
		t.Fatal("expected blank product sku to fail")
	}
	if err := peer.Close(); err != nil {
		t.Fatalf("close peer: %v", err)
	}
	if err := peer.Close(); err != nil {
		t.Fatalf("close peer twice: %v", err)
	}

	var nilPeer *Peer
	if _, err := nilPeer.GetProduct(ctx, "sku"); err == nil {
		t.Fatal("expected nil peer get to fail")
	}
	if err := nilPeer.PutProduct(ctx, Product{SKU: "sku"}); err == nil {
		t.Fatal("expected nil peer put to fail")
	}
	if err := nilPeer.Close(); err != nil {
		t.Fatalf("nil peer close: %v", err)
	}
}

func TestStoreRespectsContextCancellation(t *testing.T) {
	store := newStore(t, Product{SKU: "sku-1", Name: "Item", Version: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := store.Load(ctx, "sku-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled load, got %v", err)
	}
	if err := store.Put(ctx, Product{SKU: "sku-2", Name: "Other", Version: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled put, got %v", err)
	}
	if _, err := store.Load(context.Background(), " "); err == nil {
		t.Fatal("expected blank sku load to fail")
	}
}

func redisClients(ctx context.Context, t *testing.T) (*redis.Client, *redis.Client) {
	t.Helper()
	addr := redistestcontainer.Start(ctx, t)
	clientA := redis.NewClient(&redis.Options{Addr: addr})
	clientB := redis.NewClient(&redis.Options{Addr: addr})
	eventually(t, 2*time.Second, 10*time.Millisecond, func() bool {
		return clientA.Ping(ctx).Err() == nil && clientB.Ping(ctx).Err() == nil
	})
	t.Cleanup(func() { _ = clientB.Close() })
	t.Cleanup(func() { _ = clientA.Close() })
	return clientA, clientB
}

func newStore(t *testing.T, products ...Product) *Store {
	t.Helper()
	store, err := NewStore(products...)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func newPeer(ctx context.Context, t *testing.T, name string, client *redis.Client, namespace string, store *Store) *Peer {
	t.Helper()
	peer, err := NewPeer(ctx, name, client, namespace, store, Options{
		TTL:          time.Hour,
		LockTTL:      time.Second,
		ResultTTL:    time.Second,
		PollInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new peer: %v", err)
	}
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Fatalf("close peer: %v", err)
		}
	})
	return peer
}

func namespace(t *testing.T) string {
	t.Helper()
	return "catalog-near-cache-redis:" + strings.ReplaceAll(t.Name(), "/", ":")
}

func eventually(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	if condition() {
		return
	}
	t.Fatalf("condition did not pass within %s", timeout)
}

type loadResult struct {
	product Product
	err     error
}

func assertLoadResult(t *testing.T, result loadResult, sku string, version int) {
	t.Helper()
	if result.err != nil {
		t.Fatalf("load error: %v", result.err)
	}
	if result.product.SKU != sku || result.product.Version != version {
		t.Fatalf("product = %+v", result.product)
	}
}
