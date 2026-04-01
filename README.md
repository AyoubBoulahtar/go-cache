# go-cache

Small, generic, thread-safe in-memory TTL cache for Go.

## Features

- Generic API: `Cache[K comparable, V any]`
- Per-entry expiration (TTL)
- Optional background cleanup janitor
- Safe concurrent access with `sync.RWMutex`
- Lazy expiration cleanup on reads

## Install

```bash
go get github.com/AyoubBoulahtar/go-cache
```

## Quick start

```go
package main

import (
	"fmt"
	"time"

	"github.com/AyoubBoulahtar/go-cache/cache"
)

func main() {
	c := cache.NewCache[string, string](
		cache.WithTTL(5*time.Minute),
	)
	defer c.Close()

	c.Set("foo", "bar")
	if v, ok := c.Get("foo"); ok {
		fmt.Println(v)
	}
}
```

## Configuration options

`NewCache(opts ...Option)` supports:

- `WithTTL(ttl)` sets default TTL used by `Set`
- `WithCleanupInterval(d)` sets janitor sweep interval
- `WithoutJanitor()` disables the background janitor goroutine

Defaults:

- Default TTL: `10m`
- Cleanup interval: derived from TTL (`ttl/5`) and clamped to `[30s, 5m]`

## Expiration semantics

- Entries are considered expired when `now >= expiresAt`.
- `SetWithExpiration(key, value, expiration)`:
  - if `expiration > 0`, that value is used
  - if `expiration <= 0`, it falls back to the cache default TTL

## Cleanup behavior

- `Get` removes expired entries lazily when they are accessed.
- `DeleteExpired` removes all expired entries in one sweep.
- If janitor is enabled (default), it periodically calls `DeleteExpired`.

## API notes

- `Len()` returns number of stored entries (may include expired entries not yet collected).
- `LenUnexpired()` scans and returns the number of currently unexpired entries.
- `Has(key)` is equivalent to checking the boolean result of `Get(key)`.

## Lifecycle

- Call `Close()` to stop the janitor goroutine and clear cache memory.
- `Close()` is idempotent.

## Concurrency and value mutability

The cache is safe for concurrent map operations. If `V` is a mutable reference type
(for example, maps, slices, pointers to structs), mutating returned values from multiple
goroutines is the caller's responsibility.
