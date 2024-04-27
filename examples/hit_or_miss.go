package main

import (
	"fmt"
	"time"

	"github.com/parMaster/mcache"
)

func expensive_func() string {
	time.Sleep(1 * time.Second)
	return "expensive result"
}

func demo_hit_or_miss() {
	fmt.Println("\r\n Hit or Miss:")
	fmt.Println("------------------------------------------------------------")

	cache := mcache.NewCache[string]()
	// expensive_func will be called only once
	// because result will be saved in cache
	for i := 0; i < 10; i++ {
		v, err := cache.Get("expensive value")
		if err != nil {
			fmt.Println("cache miss, calling expensive_func")
			v = expensive_func()
			cache.Set("expensive value", v, 0)
			continue
		}
		fmt.Println("cache hit - " + v)
	}
}
