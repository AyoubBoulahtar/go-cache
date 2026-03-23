package cache

import (
	"sync"
	"time"
)

const DefaultExpiration = 10 * time.Minute

type item[V any] struct {
	value V
	ttl   time.Time
}

type Cache[K comparable, V any] struct {
	mu         sync.RWMutex
	items      map[K]item[V]
	defaultTTL time.Duration
}

func NewCache[K comparable, V any](cleanupInterval time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		items:      make(map[K]item[V]),
		defaultTTL: DefaultExpiration,
	}

	go c.janitor(cleanupInterval)
	return c
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]

	if !ok {
		var zero V
		return zero, false
	}
	if now.After(item.ttl) {
		c.Delete(key)
		var zero V
		return zero, false
	}
	return item.value, true
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithExpiration(key, value, c.defaultTTL)
}

func (c *Cache[K, V]) SetWithExpiration(key K, value V, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = item[V]{value: value, ttl: time.Now().Add(expiration)}
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

	for key, item := range c.items {
		if now.After(item.ttl) {
			delete(c.items, key)
		}
	}
}
