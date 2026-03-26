package cache

import (
	"sync"
	"time"
)

const DefaultExpiration = 10 * time.Minute

type Cache[K comparable, V any] struct {
	mu          sync.RWMutex
	items       map[K]item[V]
	defaultTTL  time.Duration
	stopJanitor chan struct{}
	closeOnce   sync.Once
}

type Option func(*config)

type config struct {
	defaultTTL              time.Duration
	cleanupInterval         time.Duration
	disableJanitor          bool
	cleanupIntervalExplicit bool
}

type item[V any] struct {
	value     V
	expiresAt time.Time
}

func NewCache[K comparable, V any](opts ...Option) *Cache[K, V] {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.defaultTTL <= 0 {
		cfg.defaultTTL = DefaultExpiration
	}
	if !cfg.cleanupIntervalExplicit || cfg.cleanupInterval <= 0 {
		cfg.cleanupInterval = deriveCleanupInterval(cfg.defaultTTL)
	}

	c := &Cache[K, V]{
		items:       make(map[K]item[V]),
		defaultTTL:  cfg.defaultTTL,
		stopJanitor: make(chan struct{}),
	}

	if !cfg.disableJanitor {
		go c.janitor(cfg.cleanupInterval)
	}

	return c
}

func WithTTL(ttl time.Duration) Option {
	return func(cfg *config) {
		if ttl > 0 {
			cfg.defaultTTL = ttl
		}
	}
}

func WithCleanupInterval(d time.Duration) Option {
	return func(cfg *config) {
		if d > 0 {
			cfg.cleanupInterval = d
			cfg.cleanupIntervalExplicit = true
		}
	}
}

func WithoutJanitor() Option {
	return func(cfg *config) {
		cfg.disableJanitor = true
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		var zero V
		return zero, false
	}

	if now.Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.value, true
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok = c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	if !now.Before(entry.expiresAt) {
		delete(c.items, key)
		var zero V
		return zero, false
	}

	return entry.value, true
}

func (c *Cache[K, V]) Has(key K) bool {
	_, ok := c.Get(key)
	return ok
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithExpiration(key, value, c.defaultTTL)
}

func (c *Cache[K, V]) SetWithExpiration(key K, value V, expiration time.Duration) {
	if expiration <= 0 {
		expiration = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = item[V]{
		value:     value,
		expiresAt: time.Now().Add(expiration),
	}
}

func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

func (c *Cache[K, V]) DeleteExpired() {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.items {
		if !now.Before(entry.expiresAt) {
			delete(c.items, key)
		}
	}
}

func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *Cache[K, V]) LenUnexpired() int {
	now := time.Now()
	count := 0
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, entry := range c.items {
		if now.Before(entry.expiresAt) {
			count++
		}
	}
	return count
}

func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]item[V])
}

func (c *Cache[K, V]) Close() {
	c.closeOnce.Do(func() {
		close(c.stopJanitor)

		c.mu.Lock()
		defer c.mu.Unlock()
		c.items = make(map[K]item[V])
	})
}

func (c *Cache[K, V]) janitor(cleanupInterval time.Duration) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-c.stopJanitor:
			return
		}
	}
}

func defaultConfig() config {
	ttl := DefaultExpiration
	return config{
		defaultTTL:      ttl,
		cleanupInterval: deriveCleanupInterval(ttl),
		disableJanitor:  false,
	}
}

func deriveCleanupInterval(ttl time.Duration) time.Duration {
	cleanupInterval := ttl / 5

	if cleanupInterval < 30*time.Second {
		cleanupInterval = 30 * time.Second
	}
	if cleanupInterval > 5*time.Minute {
		cleanupInterval = 5 * time.Minute
	}
	return cleanupInterval
}
