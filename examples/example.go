package main

import (
	"fmt"
	"time"

	"github.com/parMaster/mcache"
)

func demo() {

	fmt.Println("\r\n Demo 1:")
	fmt.Println("------------------------------------------------------------")

	cache := mcache.NewCache()

	cache.Set("save indefinitely", "value without expiration", 0)      // set value without expiration
	cache.Set("save for 1 second", "value will expire in 1 second", 1) // set value with expiration in 1 second

	exists, err := cache.Has("no such key")
	// either exists or error can be checked
	if err != nil {
		// possible errors:
		// mcache.ErrKeyNotFound
		// mcache.ErrExpired
		fmt.Println(err)
	}
	if !exists {
		fmt.Println("key doen't exist or expired")
	}

	v, _ := cache.Get("save indefinitely")
	fmt.Println(v)

	time.Sleep(1 * time.Second)
	v, _ = cache.Get("save for 1 second")
	fmt.Println(v) // <nil> because key expired

}

func main() {
	demo()
	demo_hit_or_miss()
}
