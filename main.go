package mcache

import (
	"fmt"
	"sync"
	"time"
)

// Errors for cache
const (
	ErrKeyNotFound = "key not found"
	ErrKeyExists   = "key already exists"
	ErrExpired     = "key expired"
)

// CacheItem is a struct for cache item
type CacheItem[T any] struct {
	value      T
	expiration time.Time
}

// Cache is a struct for cache
type Cache[T any] struct {
	sync.Map
}

// Cacher is an interface for cache
type Cacher interface {
	Set(key string, value interface{}, ttl int64) error
	Get(key string) (interface{}, error)
	Has(key string) (bool, error)
	Del(key string) error
	Cleanup()
	Clear() error
}

// NewCache is a constructor for Cache
func NewCache[T any](options ...func(*Cache[T])) *Cache[T] {
	c := &Cache[T]{
		sync.Map{},
	}

	for _, option := range options {
		option(c)
	}

	return c
}

// Set is a method for setting key-value pair
// If key already exists, and it's not expired, return error
// If key already exists, but it's expired, set new value and return nil
// If key doesn't exist, set new value and return nil
// If ttl is 0, set value without expiration
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) error {
	var zeroTime time.Time

	cachedItem, ok := c.Load(key)
	if ok {
		cached := cachedItem.(CacheItem[T])
		if cached.expiration == zeroTime || cached.expiration.After(time.Now().Add(ttl)) {
			c.Delete(key)
			return fmt.Errorf(ErrKeyExists)
		}
	}

	var expiration time.Time

	if ttl > time.Duration(0) {
		expiration = time.Now().Add(ttl)
	}

	c.Store(key, CacheItem[T]{
		value:      value,
		expiration: expiration,
	})

	return nil
}

// Get is a method for getting value by key
// If key doesn't exist, return error
// If key exists, but it's expired, delete key, return zeroa value and error
// If key exists and it's not expired, return value
func (c *Cache[T]) Get(key string) (T, error) {
	var none T

	cachedItem, ok := c.Load(key)
	if !ok {
		return none, fmt.Errorf(ErrKeyNotFound)
	}
	cached := cachedItem.(CacheItem[T])

	return cached.value, nil
}

// Has is a method for checking if key exists.
// If key doesn't exist, return false.
// If key exists, but it's expired, return false and delete key.
// If key exists and it's not expired, return true.
func (c *Cache[T]) Has(key string) (bool, error) {
	cachedItem, ok := c.Load(key)
	if !ok {
		return false, fmt.Errorf(ErrKeyNotFound)
	}
	cached := cachedItem.(CacheItem[T])

	var zeroTime time.Time
	if cached.expiration != zeroTime && cached.expiration.Before(time.Now()) {
		c.Delete(key)
		return false, fmt.Errorf(ErrExpired)
	}

	return true, nil
}

// Del is a method for deleting key-value pair
func (c *Cache[T]) Del(key string) error {
	_, err := c.Has(key)
	if err != nil {
		return err
	}

	c.Delete(key)
	return nil
}

// Clear is a method for clearing cache
func (c *Cache[T]) Clear() error {
	c.Map = sync.Map{}
	return nil
}

// Cleanup is a method for deleting expired keys
func (c *Cache[T]) Cleanup() {
	var zeroTime time.Time
	c.Range(func(key, value interface{}) bool {
		cached := value.(CacheItem[T])
		if cached.expiration != zeroTime && cached.expiration.Before(time.Now()) {
			c.Delete(key)
		}
		return true
	})
}

// WithCleanup is a functional option for setting interval to run Cleanup goroutine
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
