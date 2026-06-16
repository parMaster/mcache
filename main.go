// Package provides simple, fast, thread-safe in-memory cache with by-key TTL expiration.
// Supporting generic value types.
package mcache

import (
	"context"
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

func (cacheItem CacheItem[T]) expired() bool {
	return !cacheItem.expiration.IsZero() && cacheItem.expiration.Before(time.Now())
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

// Set is a method for setting key-value pair.
// If key already exists, and it's not expired, return false.
// If key already exists, but it's expired, set new value and return true.
// If key doesn't exist, set new value and return true.
// If ttl is 0, set value without expiration.
// If ttl is negative, return false.
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) bool {
	if ttl < 0 {
		return false
	}
	c.Lock()
	defer c.Unlock()
	cached, ok := c.data[key]
	if ok {
		if !cached.expired() {
			return false
		}
	}
	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}
	c.data[key] = &CacheItem[T]{
		value:      value,
		expiration: expiration,
	}
	return true
}

// Get returns the value for key.
// Returns ErrKeyNotFound if key does not exist.
// Returns ErrExpired and deletes the key if it has expired.
func (c *Cache[T]) Get(key string) (T, error) {
	var none T

	c.RLock()
	item, ok := c.data[key]
	c.RUnlock()

	if !ok {
		return none, ErrKeyNotFound
	}
	if !item.expired() {
		return item.value, nil
	}

	// Expired path: upgrade to write lock for deletion.
	// Re-check under write lock — another goroutine may have replaced the key.
	c.Lock()
	defer c.Unlock()
	item, ok = c.data[key]
	if !ok {
		return none, ErrKeyNotFound
	}
	if item.expired() {
		delete(c.data, key)
		return none, ErrExpired
	}
	return item.value, nil
}

// Has checks if key exists and is not expired.
// Returns ErrKeyNotFound if key does not exist.
// Returns ErrExpired and deletes the key if it has expired.
func (c *Cache[T]) Has(key string) (bool, error) {
	c.RLock()
	item, ok := c.data[key]
	c.RUnlock()

	if !ok {
		return false, ErrKeyNotFound
	}
	if !item.expired() {
		return true, nil
	}

	c.Lock()
	defer c.Unlock()
	item, ok = c.data[key]
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
// Returns ErrKeyNotFound if the key does not exist.
// Expired keys are deleted and nil is returned — the key is gone either way.
func (c *Cache[T]) Del(key string) error {
	c.Lock()
	defer c.Unlock()
	_, ok := c.data[key]
	if !ok {
		return ErrKeyNotFound
	}
	delete(c.data, key)
	return nil
}

// Clear replaces the cache with a clean one.
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

// WithCleanup is a functional option that starts a background goroutine to
// periodically delete expired keys. The goroutine stops when ctx is cancelled.
func WithCleanup[T any](ctx context.Context, interval time.Duration) func(*Cache[T]) {
	return func(c *Cache[T]) {
		ticker := time.NewTicker(interval)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					c.Cleanup()
				case <-ctx.Done():
					return
				}
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
