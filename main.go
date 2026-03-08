package main

import (
	"fmt"

	"github.com/AyoubBoulahtar/go-cache/cache"
)

func main() {
	newCache := cache.NewCache[string, string]()
	newCache.Set("foo", "bar")
	cacheResolved, exists := newCache.Get("foo")
	if exists {
		fmt.Printf("result : %v\n", cacheResolved)
	}
}
