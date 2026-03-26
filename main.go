package main

import (
	"fmt"
	"time"

	"github.com/AyoubBoulahtar/go-cache/cache"
)

func main() {
	newCache := cache.NewCache[string, string](cache.WithTTL(5 * time.Minute))

	newCache.Set("foo", "bar")

	cacheResolved, exists := newCache.Get("foo")
	if exists {
		fmt.Printf("result : %v\n", cacheResolved)
	}
}
