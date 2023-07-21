package main

import (
	"fmt"

	"github.com/parMaster/mcache"
)

func main() {

	cache := mcache.NewCache()

	cache.Set("key", "value", 5*60) // set value with expiration in 5 minutes

	v, err := cache.Get("key")
	if err != nil {
		// either error can be checked
		fmt.Println(err)
	}
	if v != nil {
		// or value can be checked for nil
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
