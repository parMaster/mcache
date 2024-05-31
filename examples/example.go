package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/parMaster/mcache"
)

func demo() {

	fmt.Println("\r\n Demo 1:")
	fmt.Println("------------------------------------------------------------")

	cache := mcache.NewCache[string]()

	fmt.Println("Setting key \"save indefinitely\" with value \"value without expiration\" and ttl 0")
	cache.Set("save indefinitely", "value without expiration", 0) // set value without expiration

	fmt.Println("Setting key \"save for 1 second\" with value \"value will expire in 1 second\" and ttl 1 second")
	cache.Set("save for 1 second", "value will expire in 1 second", 1*time.Second) // set value with expiration in 1 second

	fmt.Printf("\nRetrieving \"no such key\":\n")
	exists, err := cache.Has("no such key")
	// either exists or error can be checked
	if err != nil {
		// possible errors:
		// mcache.ErrKeyNotFound
		// mcache.ErrExpired
		fmt.Printf("\tError retrieving \"no such key\": %v\n", err)
		fmt.Printf("\tError is \"mcache.ErrKeyNotFound\": %t\n", errors.Is(err, mcache.ErrKeyNotFound))
	}
	fmt.Printf("\t\"no such key\" exists: %t\n", exists)

	fmt.Printf("\nRetrieving \"save indefinitely\":\n")
	v, err := cache.Get("save indefinitely")
	fmt.Printf("\t\"save indefinitely\" = %v\n", v)
	fmt.Printf("\tError retrieving \"save indefinitely\": %v\n", err)

	time.Sleep(1 * time.Second)
	fmt.Printf("\nRetrieving \"save for 1 second\" after 1+ second pause:\n")
	v, err = cache.Get("save for 1 second")
	fmt.Printf("\t\"save for 1 second\" value = %v\n", v)
	fmt.Printf("\tError retrieving \"save for 1 second\": %v\n", err)
	fmt.Printf("\tError is \"mcache.ErrExpired\": %t\n", errors.Is(err, mcache.ErrExpired))

}

func main() {
	demo()
	demo_hit_or_miss()
}
