package cache

import (
	"sync"
	"time"
)

const DefaultExpiration = 10 * time.Minute

type item[V any] struct {
	value     V
	expiresAt time.Time
}

type Cache[K comparable, V any] struct {
	mu          sync.RWMutex
	items       map[K]item[V]
	defaultTTL  time.Duration
	stopJanitor chan struct{}
}

func NewCache[K comparable, V any](cleanupInterval time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		items:       make(map[K]item[V]),
		defaultTTL:  DefaultExpiration,
		stopJanitor: make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go c.janitor(cleanupInterval)
	}

	return c
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	if now.After(entry.expiresAt) {
		delete(c.items, key)
		var zero V
		return zero, false
	}

	return entry.value, true
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithExpiration(key, value, c.defaultTTL)
}

func (c *Cache[K, V]) SetWithExpiration(key K, value V, expiration time.Duration) {
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
		if now.After(entry.expiresAt) {
			delete(c.items, key)
		}
	}
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

func (c *Cache[K, V]) Close() {
	close(c.stopJanitor)
}
