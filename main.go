// Package provides simple, fast, thread-safe in-memory cache with by-key TTL expiration.
// Supporting generic value types.
package mcache

import (
	"errors"
	"sync"
	"time"
)

// Errors for cache
var (
	ErrKeyNotFound = errors.New("key not found")
	ErrExpired     = errors.New("key expired")
)

// CacheItem is a struct for cache item.
type CacheItem[T any] struct {
	value      T
	expiration time.Time
}

// Cache is a struct for cache.
type Cache[T any] struct {
	initialSize int
	data        map[string]*CacheItem[T]
	sync.RWMutex
}

// Cacher is an interface for cache.
type Cacher[T any] interface {
	Set(key string, value T, ttl time.Duration) bool
	Get(key string) (T, error)
	Has(key string) (bool, error)
	Del(key string) error
	Cleanup()
	Clear() error
}

// NewCache is a constructor for Cache.
func NewCache[T any](options ...func(*Cache[T])) *Cache[T] {
	c := &Cache[T]{
		data: make(map[string]*CacheItem[T]),
	}

	for _, option := range options {
		option(c)
	}

	return c
}

// common method for checking if item is expired
func (cacheItem CacheItem[T]) expired() bool {
	if !cacheItem.expiration.IsZero() && cacheItem.expiration.Before(time.Now()) {
		return true
	}
	return false
}

// Set is a method for setting key-value pair.
// If key already exists, and it's not expired, return false.
// If key already exists, but it's expired, set new value and return true.
// If key doesn't exist, set new value and return true.
// If ttl is 0, set value without expiration.
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) bool {
	c.Lock()
	defer c.Unlock()
	cached, ok := c.data[key]
	if ok {
		if !cached.expired() {
			return false
		}
	}

	var expiration time.Time

	if ttl > time.Duration(0) {
		expiration = time.Now().Add(ttl)
	}

	c.data[key] = &CacheItem[T]{
		value:      value,
		expiration: expiration,
	}
	return true
}

// Get is a method for getting value by key.
// If key doesn't exist, return error.
// If key exists, but it's expired, delete key, return zero value and error.
// If key exists and it's not expired, return value.
func (c *Cache[T]) Get(key string) (T, error) {
	var none T

	c.Lock()
	defer c.Unlock()

	item, ok := c.data[key]
	if !ok {
		return none, ErrKeyNotFound
	}

	if item.expired() {
		delete(c.data, key)
		return none, ErrExpired
	}

	return c.data[key].value, nil
}

// Has checks if key exists and if it's expired.
// If key doesn't exist, return false.
// If key exists, but it's expired, return false and delete key.
// If key exists and it's not expired, return true.
func (c *Cache[T]) Has(key string) (bool, error) {
	c.Lock()
	defer c.Unlock()

	item, ok := c.data[key]
	if !ok {
		return false, ErrKeyNotFound
	}

	if item.expired() {
		delete(c.data, key)
		return false, ErrExpired
	}

	return true, nil
}

// Del deletes a key-value pair.
func (c *Cache[T]) Del(key string) error {
	_, err := c.Has(key)
	if err != nil {
		return err
	}

	// parallel goroutine can delete key right here
	// or even perform Clear() operation
	// but it doen't matter

	c.Lock()
	delete(c.data, key)
	c.Unlock()
	return nil
}

// Clears cache by replacing it with a clean one.
func (c *Cache[T]) Clear() error {
	c.Lock()
	c.data = make(map[string]*CacheItem[T], c.initialSize)
	c.Unlock()
	return nil
}

// Cleanup deletes expired keys from cache by copying non-expired keys to a new map.
func (c *Cache[T]) Cleanup() {
	c.Lock()
	defer c.Unlock()
	data := make(map[string]*CacheItem[T], c.initialSize)
	for k, v := range c.data {
		if !v.expired() {
			data[k] = v
		}
	}
	c.data = data
}

// WithCleanup is a functional option for setting interval to run Cleanup goroutine.
func WithCleanup[T any](ttl time.Duration) func(*Cache[T]) {
	return func(c *Cache[T]) {
		go func() {
			for {
				c.Cleanup()
				time.Sleep(ttl)
			}
		}()
	}
}

// WithSize is a functional option for setting cache initial size. So it won't grow dynamically,
// go will allocate appropriate number of buckets.
func WithSize[T any](size int) func(*Cache[T]) {
	return func(c *Cache[T]) {
		c.data = make(map[string]*CacheItem[T], size)
		c.initialSize = size
	}
}
