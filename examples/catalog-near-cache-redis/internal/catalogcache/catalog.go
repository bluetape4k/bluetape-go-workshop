// Package catalogcache demonstrates Redis near-cache invalidation and
// cross-peer stampede coordination for a catalog read model.
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

// ErrProductNotFound is returned when the authoritative catalog has no SKU.
var ErrProductNotFound = errors.New("product not found")

// Product is the cached catalog projection.
type Product struct {
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// Options controls cache timing for the example peer.
type Options struct {
	TTL          time.Duration
	LockTTL      time.Duration
	ResultTTL    time.Duration
	PollInterval time.Duration
}

// Store is the authoritative in-memory product source used by the example.
type Store struct {
	mu       sync.RWMutex
	products map[string]Product
	loads    map[string]int
	onLoad   func(context.Context, string) error
}

// NewStore creates an authoritative store with optional seed products.
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

// Put writes the authoritative product projection.
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

// Load reads an authoritative product and records a backing load.
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

// LoadCount returns how many times the authoritative loader ran for the SKU.
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

// Peer is one catalog service instance with a local cache and Redis
// coordination.
type Peer struct {
	name  string
	store *Store
	ttl   time.Duration
	near  *redisnear.NearCache[Product]
	cache *rediscoord.StampedeCache[Product]
}

// NewPeer creates a catalog peer backed by memory, Redis near-cache
// invalidation, and Redis stampede coordination.
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

// GetProduct returns a product through the coordinated near-cache.
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

// PutProduct writes the authoritative store and publishes peer invalidation.
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

// Close stops the peer's near-cache subscriber. Redis clients remain
// caller-owned.
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
