// Package catalogcache는 catalog read model을 위해 Redis near-cache invalidation과
// cross-peer stampede coordination을 보여준다.
package catalogcache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	btcache "github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/rediscoord"
	"github.com/bluetape4k/bluetape-go/cache/redisnear"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTTL          = time.Minute
	defaultLockTTL      = time.Second
	defaultResultTTL    = time.Second
	defaultPollInterval = 5 * time.Millisecond
)

// ErrProductNotFound는 authoritative catalog에 SKU가 없을 때 반환된다.
var ErrProductNotFound = errors.New("product not found")

// Product는 cached catalog projection이다.
type Product struct {
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// Options는 example peer의 cache timing을 제어한다.
type Options struct {
	TTL          time.Duration
	LockTTL      time.Duration
	ResultTTL    time.Duration
	PollInterval time.Duration
}

// Store는 example이 사용하는 authoritative in-memory product source다.
type Store struct {
	mu       sync.RWMutex
	products map[string]Product
	loads    map[string]int
	onLoad   func(context.Context, string) error
}

// NewStore는 optional seed product가 있는 authoritative store를 만든다.
func NewStore(products ...Product) (*Store, error) {
	store := &Store{
		products: make(map[string]Product, len(products)),
		loads:    make(map[string]int),
	}
	for _, product := range products {
		if err := store.Put(context.Background(), product); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// Put은 authoritative product projection을 쓴다.
func (s *Store) Put(ctx context.Context, product Product) error {
	if s == nil {
		return fmt.Errorf("store must not be nil")
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(product.SKU) == "" {
		return fmt.Errorf("product sku must not be blank")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMaps()
	s.products[product.SKU] = product
	return nil
}

// Load는 authoritative product를 읽고 backing load를 기록한다.
func (s *Store) Load(ctx context.Context, sku string) (Product, error) {
	var zero Product
	if s == nil {
		return zero, fmt.Errorf("store must not be nil")
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if strings.TrimSpace(sku) == "" {
		return zero, fmt.Errorf("sku must not be blank")
	}

	hook := s.recordLoad(sku)
	if hook != nil {
		if err := hook(ctx, sku); err != nil {
			return zero, err
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	product, ok := s.products[sku]
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrProductNotFound, sku)
	}
	return product, nil
}

// LoadCount는 해당 SKU에 대해 authoritative loader가 몇 번 실행됐는지 반환한다.
func (s *Store) LoadCount(sku string) int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loads[sku]
}

func (s *Store) recordLoad(sku string) func(context.Context, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMaps()
	s.loads[sku]++
	return s.onLoad
}

func (s *Store) setLoadHook(hook func(context.Context, string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onLoad = hook
}

func (s *Store) ensureMaps() {
	if s.products == nil {
		s.products = make(map[string]Product)
	}
	if s.loads == nil {
		s.loads = make(map[string]int)
	}
}

// Peer는 local cache와 Redis coordination을 가진 catalog service instance 하나다.
type Peer struct {
	name  string
	store *Store
	ttl   time.Duration
	near  *redisnear.NearCache[Product]
	cache *rediscoord.StampedeCache[Product]
}

// NewPeer는 memory, Redis near-cache invalidation, Redis stampede coordination으로
// backing되는 catalog peer를 만든다.
func NewPeer(
	ctx context.Context,
	name string,
	client *redis.Client,
	namespace string,
	store *Store,
	options Options,
) (*Peer, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("peer name must not be blank")
	}
	if client == nil {
		return nil, fmt.Errorf("redis client must not be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("namespace must not be blank")
	}
	if store == nil {
		return nil, fmt.Errorf("store must not be nil")
	}

	cfg, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}

	local := btcache.NewMemory[string, Product]()
	near, err := redisnear.NewPubSub[Product](ctx, redisnear.Options[Product]{
		Client:    client,
		Namespace: namespace,
		OriginID:  name,
		Local:     local,
	})
	if err != nil {
		return nil, fmt.Errorf("new near cache: %w", err)
	}

	coordinated, err := rediscoord.NewStampedeCache[Product](rediscoord.Options[Product]{
		Client:       client,
		Cache:        near,
		Namespace:    namespace,
		Codec:        rediscoord.JSONCodec[Product]{},
		LockTTL:      cfg.LockTTL,
		ResultTTL:    cfg.ResultTTL,
		PollInterval: cfg.PollInterval,
	})
	if err != nil {
		_ = near.Close()
		return nil, fmt.Errorf("new stampede cache: %w", err)
	}

	return &Peer{
		name:  name,
		store: store,
		ttl:   cfg.TTL,
		near:  near,
		cache: coordinated,
	}, nil
}

// GetProduct는 coordinated near-cache를 통해 product를 반환한다.
func (p *Peer) GetProduct(ctx context.Context, sku string) (Product, error) {
	var zero Product
	if p == nil {
		return zero, fmt.Errorf("peer must not be nil")
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if strings.TrimSpace(sku) == "" {
		return zero, fmt.Errorf("sku must not be blank")
	}

	product, err := p.cache.GetOrLoad(ctx, sku, p.ttl, func(ctx context.Context, key string) (Product, error) {
		return p.store.Load(ctx, key)
	})
	if err != nil {
		return zero, fmt.Errorf("get product %s on %s: %w", sku, p.name, err)
	}
	return product, nil
}

// PutProduct는 authoritative store에 쓰고 peer invalidation을 publish한다.
func (p *Peer) PutProduct(ctx context.Context, product Product) error {
	if p == nil {
		return fmt.Errorf("peer must not be nil")
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.store.Put(ctx, product); err != nil {
		return err
	}
	if err := p.cache.Set(ctx, product.SKU, product, p.ttl); err != nil {
		return fmt.Errorf("set product %s on %s: %w", product.SKU, p.name, err)
	}
	return nil
}

// Close는 peer의 near-cache subscriber를 멈춘다. Redis client ownership은 caller에게 남는다.
func (p *Peer) Close() error {
	if p == nil {
		return nil
	}
	return p.near.Close()
}

func normalizeOptions(options Options) (Options, error) {
	ttl, err := normalizeDuration(options.TTL, defaultTTL, "ttl")
	if err != nil {
		return Options{}, err
	}
	lockTTL, err := normalizeDuration(options.LockTTL, defaultLockTTL, "lock ttl")
	if err != nil {
		return Options{}, err
	}
	resultTTL, err := normalizeDuration(options.ResultTTL, defaultResultTTL, "result ttl")
	if err != nil {
		return Options{}, err
	}
	pollInterval, err := normalizeDuration(options.PollInterval, defaultPollInterval, "poll interval")
	if err != nil {
		return Options{}, err
	}
	return Options{
		TTL:          ttl,
		LockTTL:      lockTTL,
		ResultTTL:    resultTTL,
		PollInterval: pollInterval,
	}, nil
}

func normalizeDuration(value time.Duration, fallback time.Duration, name string) (time.Duration, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s must not be negative", name)
	}
	if value == 0 {
		return fallback, nil
	}
	return value, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
