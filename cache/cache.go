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

func NewCache[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{
		items:      make(map[K]item[V]),
		defaultTTL: DefaultExpiration,
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]

	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(item.ttl) {
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
