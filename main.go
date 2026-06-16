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
	ErrKeyExists   = errors.New("key already exists")
	ErrExpired     = errors.New("key expired")
	ErrInvalidTTL  = errors.New("invalid ttl")
)

// CacheItem is a struct for cache item
type CacheItem[T any] struct {
	value      T
	expiration time.Time
}

func (cacheItem CacheItem[T]) expired() bool {
	return !cacheItem.expiration.IsZero() && cacheItem.expiration.Before(time.Now())
}

// Cache is a struct for cache
type Cache[T any] struct {
	data map[string]CacheItem[T]
	mx   sync.RWMutex
}

// Cacher is an interface for cache
type Cacher[T any] interface {
	Set(key string, value T, ttl time.Duration) error
	Get(key string) (T, error)
	Has(key string) (bool, error)
	Del(key string) error
	Cleanup()
	Clear() error
}

// NewCache is a constructor for Cache
func NewCache[T any](options ...func(*Cache[T])) *Cache[T] {
	c := &Cache[T]{
		data: make(map[string]CacheItem[T]),
	}

	for _, option := range options {
		option(c)
	}

	return c
}

// Set is a method for setting key-value pair.
// If key already exists, and it's not expired, return error.
// If key already exists, but it's expired, set new value and return nil.
// If key doesn't exist, set new value and return nil.
// If ttl is 0, set value without expiration
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) error {
	if ttl < 0 {
		return ErrInvalidTTL
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	cached, ok := c.data[key]
	if ok {
		if !cached.expired() {
			return ErrKeyExists
		}
	}

	var expiration time.Time

	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	c.data[key] = CacheItem[T]{
		value:      value,
		expiration: expiration,
	}
	return nil
}

// Get returns the value for key.
// Returns ErrKeyNotFound if key does not exist.
// Returns ErrExpired and deletes the key if it has expired.
func (c *Cache[T]) Get(key string) (T, error) {
	var none T

	c.mx.RLock()
	item, ok := c.data[key]
	c.mx.RUnlock()

	if !ok {
		return none, ErrKeyNotFound
	}
	if !item.expired() {
		return item.value, nil
	}

	// Expired path: upgrade to write lock for deletion.
	// Re-check under write lock — another goroutine may have replaced the key.
	c.mx.Lock()
	defer c.mx.Unlock()
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
	c.mx.RLock()
	item, ok := c.data[key]
	c.mx.RUnlock()

	if !ok {
		return false, ErrKeyNotFound
	}
	if !item.expired() {
		return true, nil
	}

	c.mx.Lock()
	defer c.mx.Unlock()
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
	c.mx.Lock()
	defer c.mx.Unlock()
	_, ok := c.data[key]
	if !ok {
		return ErrKeyNotFound
	}
	delete(c.data, key)
	return nil
}

// Clears cache by replacing it with a clean one
func (c *Cache[T]) Clear() error {
	c.mx.Lock()
	c.data = make(map[string]CacheItem[T])
	c.mx.Unlock()
	return nil
}

// Cleanup deletes expired keys from cache
func (c *Cache[T]) Cleanup() {
	c.mx.Lock()
	for k, v := range c.data {
		if v.expired() {
			delete(c.data, k)
		}
	}
	c.mx.Unlock()
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
