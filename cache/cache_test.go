package cache

import (
	"sync"
	"testing"
	"time"
)

func withLockedItems[K comparable, V any](t *testing.T, c *Cache[K, V], fn func(map[K]item[V])) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c.items)
}

func getLenLocked[K comparable, V any](t *testing.T, c *Cache[K, V]) int {
	t.Helper()
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func mustNotPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic, got: %v", r)
		}
	}()
	f()
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not satisfied within %s", timeout)
}

func TestSetGet_HappyPath(t *testing.T) {
	c := NewCache[string, string](WithoutJanitor())
	defer c.Close()

	c.Set("foo", "bar")
	got, ok := c.Get("foo")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "bar" {
		t.Fatalf("expected value %q, got %q", "bar", got)
	}
}

func TestGet_MissingKey(t *testing.T) {
	c := NewCache[string, int](WithoutJanitor())
	defer c.Close()

	got, ok := c.Get("missing")
	if ok {
		t.Fatalf("expected ok=false")
	}
	if got != 0 {
		t.Fatalf("expected zero value, got %v", got)
	}
}

func TestDelete_RemovesKey(t *testing.T) {
	c := NewCache[string, string](WithoutJanitor())
	defer c.Close()

	c.Set("foo", "bar")
	c.Delete("foo")

	got, ok := c.Get("foo")
	if ok {
		t.Fatalf("expected ok=false")
	}
	if got != "" {
		t.Fatalf("expected zero value, got %q", got)
	}
}

func TestClear_RemovesAllKeys(t *testing.T) {
	c := NewCache[string, int](WithoutJanitor())
	defer c.Close()

	for i := 0; i < 10; i++ {
		c.Set("k"+string(rune('a'+i)), i)
	}
	if c.Len() != 10 {
		t.Fatalf("expected len=10, got %d", c.Len())
	}

	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("expected len=0 after clear, got %d", c.Len())
	}
	for i := 0; i < 10; i++ {
		_, ok := c.Get("k" + string(rune('a'+i)))
		if ok {
			t.Fatalf("expected key %d to be missing after clear", i)
		}
	}
}

func TestExpiration_ExpiresAfterTTL(t *testing.T) {
	ttl := 30 * time.Millisecond
	c := NewCache[string, string](WithoutJanitor())
	defer c.Close()

	c.SetWithExpiration("foo", "bar", ttl)

	// Still valid shortly after.
	got, ok := c.Get("foo")
	if !ok || got != "bar" {
		t.Fatalf("expected value before expiry, got (%v, %v)", got, ok)
	}

	time.Sleep(ttl + 25*time.Millisecond)
	_, ok = c.Get("foo")
	if ok {
		t.Fatalf("expected key to be expired after TTL")
	}
}

func TestExpiration_BoundaryAroundEquality(t *testing.T) {
	ttl := 20 * time.Millisecond
	c := NewCache[string, int](WithoutJanitor())
	defer c.Close()

	c.SetWithExpiration("k", 1, ttl)

	// Give a small buffer that should still be before expiry.
	time.Sleep(ttl - 8*time.Millisecond)
	_, ok := c.Get("k")
	if !ok {
		t.Fatalf("expected key to still exist near boundary before expiry")
	}

	// Now after expiry.
	time.Sleep(15 * time.Millisecond)
	_, ok = c.Get("k")
	if ok {
		t.Fatalf("expected key to be expired after expiry")
	}
}

func TestSetWithExpiration_NonPositiveExpiration_UsesDefaultTTL(t *testing.T) {
	defaultTTL := 40 * time.Millisecond
	c := NewCache[string, int](WithTTL(defaultTTL), WithoutJanitor())
	defer c.Close()

	c.SetWithExpiration("k0", 0, 0)
	c.SetWithExpiration("kneg", 1, -1)

	// Inspect expiresAt directly under lock to avoid flakiness.
	c.mu.RLock()
	e0, ok0 := c.items["k0"]
	e1, ok1 := c.items["kneg"]
	c.mu.RUnlock()
	if !ok0 || !ok1 {
		t.Fatalf("expected both keys to be present immediately after SetWithExpiration")
	}

	now := time.Now()
	// expiresAt should be about now + defaultTTL (within a reasonable scheduling delta).
	lower := now.Add(defaultTTL - 15*time.Millisecond)
	upper := now.Add(defaultTTL + 60*time.Millisecond)
	if e0.expiresAt.Before(lower) || e0.expiresAt.After(upper) {
		t.Fatalf("expiresAt for k0 out of expected range: %v (expected between %v and %v)", e0.expiresAt, lower, upper)
	}
	if e1.expiresAt.Before(lower) || e1.expiresAt.After(upper) {
		t.Fatalf("expiresAt for kneg out of expected range: %v (expected between %v and %v)", e1.expiresAt, lower, upper)
	}
}

func TestDefaultTTL_UsedBySet(t *testing.T) {
	defaultTTL := 25 * time.Millisecond
	c := NewCache[string, string](WithTTL(defaultTTL), WithoutJanitor())
	defer c.Close()

	c.Set("foo", "bar")

	c.mu.RLock()
	e, ok := c.items["foo"]
	c.mu.RUnlock()
	if !ok {
		t.Fatalf("expected key to exist")
	}

	now := time.Now()
	lower := now.Add(defaultTTL - 15*time.Millisecond)
	upper := now.Add(defaultTTL + 60*time.Millisecond)
	if e.expiresAt.Before(lower) || e.expiresAt.After(upper) {
		t.Fatalf("expiresAt out of expected range: %v (expected between %v and %v)", e.expiresAt, lower, upper)
	}
}

func TestLen_IncludesExpiredUntilCollected(t *testing.T) {
	ttl := 15 * time.Millisecond
	c := NewCache[string, int](WithoutJanitor())
	defer c.Close()

	c.SetWithExpiration("k", 1, ttl)
	time.Sleep(ttl + 25*time.Millisecond)

	if got := c.Len(); got != 1 {
		t.Fatalf("expected Len() to include expired entry until collected, got %d", got)
	}

	// A Get triggers lazy deletion.
	_, _ = c.Get("k")
	if got := c.Len(); got != 0 {
		t.Fatalf("expected Len() to drop after lazy collection, got %d", got)
	}
}

func TestDeleteExpired_RemovesAllExpired(t *testing.T) {
	c := NewCache[string, int](WithoutJanitor())
	defer c.Close()

	withLockedItems(t, c, func(m map[string]item[int]) {
		now := time.Now()
		m["expired1"] = item[int]{value: 1, expiresAt: now.Add(-1 * time.Second)}
		m["expired2"] = item[int]{value: 2, expiresAt: now.Add(-500 * time.Millisecond)}
		m["live"] = item[int]{value: 3, expiresAt: now.Add(10 * time.Second)}
	})

	c.DeleteExpired()

	if got := c.Len(); got != 1 {
		t.Fatalf("expected only 1 live entry after DeleteExpired, got %d", got)
	}
	if _, ok := c.Get("live"); !ok {
		t.Fatalf("expected live entry to remain")
	}
	if _, ok := c.Get("expired1"); ok {
		t.Fatalf("expected expired1 to be removed")
	}
	if _, ok := c.Get("expired2"); ok {
		t.Fatalf("expected expired2 to be removed")
	}
}

func TestJanitor_DeletesExpiredEntries(t *testing.T) {
	// Use very small intervals for tests.
	cleanup := 5 * time.Millisecond
	ttl := 20 * time.Millisecond
	// Janitor is enabled by default unless WithoutJanitor() is passed.
	c := NewCache[string, int](WithCleanupInterval(cleanup))
	defer c.Close()

	c.SetWithExpiration("k", 123, ttl)

	// Do not call Get: ensure janitor does the deletion.
	waitUntil(t, 500*time.Millisecond, func() bool {
		return getLenLocked(t, c) == 0
	})
}

func TestWithoutJanitor_NoBackgroundCleanup(t *testing.T) {
	ttl := 15 * time.Millisecond
	c := NewCache[string, int](WithoutJanitor(), WithCleanupInterval(5*time.Millisecond))
	defer c.Close()

	c.SetWithExpiration("k", 1, ttl)
	time.Sleep(ttl + 40*time.Millisecond)

	// Expired entries remain stored if janitor is disabled.
	if got := getLenLocked(t, c); got != 1 {
		t.Fatalf("expected expired entry to still be stored when janitor is disabled; got len=%d", got)
	}

	_, ok := c.Get("k")
	if ok {
		t.Fatalf("expected Get to return ok=false for expired entry")
	}
	if got := c.Len(); got != 0 {
		t.Fatalf("expected lazy deletion to remove expired entry; got len=%d", got)
	}
}

func TestClose_Idempotent(t *testing.T) {
	c := NewCache[string, int](WithCleanupInterval(10 * time.Millisecond))

	// Populate a bit, then close twice.
	c.Set("k", 1)
	mustNotPanic(t, func() { c.Close() })
	mustNotPanic(t, func() { c.Close() })
	if got := c.Len(); got != 0 {
		t.Fatalf("expected cache cleared on Close, got len=%d", got)
	}
}

func TestHas_ConsistentWithGet(t *testing.T) {
	c := NewCache[string, int](WithoutJanitor())
	defer c.Close()

	c.Set("k", 1)
	if !c.Has("k") {
		t.Fatalf("expected Has to be true for present key")
	}

	// Inject an expired entry, then validate Has == Get.
	withLockedItems(t, c, func(m map[string]item[int]) {
		m["expired"] = item[int]{value: 2, expiresAt: time.Now().Add(-time.Second)}
	})

	has := c.Has("expired")
	_, ok := c.Get("expired")
	if has || ok {
		t.Fatalf("expected both Has and Get to report missing/expired: Has=%v GetOk=%v", has, ok)
	}
}

func TestConcurrency_GetSetDelete_NoRaces(t *testing.T) {
	cleanup := 5 * time.Millisecond
	c := NewCache[int, int](WithCleanupInterval(cleanup), WithTTL(50*time.Millisecond))
	defer c.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	workerSet := func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.Set(i%100, i)
			i++
		}
	}

	workerGet := func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_, _ = c.Get(time.Now().Nanosecond() % 100)
		}
	}

	workerDelete := func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.Delete(time.Now().Nanosecond() % 100)
		}
	}

	wg.Add(3)
	go workerSet()
	go workerGet()
	go workerDelete()

	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()
}
