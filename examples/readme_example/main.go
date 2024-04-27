package main

import (
	"fmt"
	"time"

	"github.com/parMaster/mcache"
)

func main() {

	cache := mcache.NewCache[string]()

	cache.Set("key", "value", time.Minute*5) // set value with expiration in 5 minutes

	v, err := cache.Get("key")
	if err != nil {
		// either error can be checked
		fmt.Println(err)
	}
	if v != "" {
		// or value can be checked for "empty" type value
		fmt.Println("key =", v)
	}

	// check if key exists
	exists, err := cache.Has("key")
	if err != nil {
		// possible errors:
		// mcache.ErrKeyNotFound - key doesn't exist
		// mcache.ErrExpired     - key expired
		fmt.Println(err)
	}
	if exists {
		fmt.Println("key exists")
	}

	// delete key
	cache.Del("key")
}
