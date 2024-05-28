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

// Set is a method for setting key-value pair
// If key already exists, and it's not expired, return error
// If key already exists, but it's expired, set new value and return nil
// If key doesn't exist, set new value and return nil
// If ttl is 0, set value without expiration
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) error {
	var zeroTime time.Time

	c.mx.RLock()
	cached, ok := c.data[key]
	c.mx.RUnlock()
	if ok {
		if cached.expiration == zeroTime || cached.expiration.After(time.Now().Add(ttl)) {
			return fmt.Errorf(ErrKeyExists)
		}
	}

	var expiration time.Time

	if ttl > time.Duration(0) {
		expiration = time.Now().Add(ttl)
	}

	c.mx.Lock()
	c.data[key] = CacheItem[T]{
		value:      value,
		expiration: expiration,
	}
	c.mx.Unlock()
	return nil
}

// Get is a method for getting value by key
// If key doesn't exist, return error
// If key exists, but it's expired, delete key, return zeroa value and error
// If key exists and it's not expired, return value
func (c *Cache[T]) Get(key string) (T, error) {
	var none T

	_, err := c.Has(key)
	if err != nil {
		return none, err
	}

	// safe return?
	c.mx.RLock()
	defer c.mx.RUnlock()

	return c.data[key].value, nil
}

// Has is a method for checking if key exists.
// If key doesn't exist, return false.
// If key exists, but it's expired, return false and delete key.
// If key exists and it's not expired, return true.
func (c *Cache[T]) Has(key string) (bool, error) {
	c.mx.RLock()
	d, ok := c.data[key]
	c.mx.RUnlock()
	if !ok {
		return false, fmt.Errorf(ErrKeyNotFound)
	}

	var zeroTime time.Time
	if d.expiration != zeroTime && d.expiration.Before(time.Now()) {
		c.mx.Lock()
		delete(c.data, key)
		c.mx.Unlock()
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

	c.mx.Lock()
	delete(c.data, key)
	c.mx.Unlock()
	return nil
}

// Clear is a method for clearing cache
func (c *Cache[T]) Clear() error {
	c.mx.Lock()
	c.data = make(map[string]CacheItem[T])
	c.mx.Unlock()
	return nil
}

// Cleanup is a method for deleting expired keys
func (c *Cache[T]) Cleanup() {
	c.mx.Lock()
	var zeroTime time.Time
	for k, v := range c.data {
		if v.expiration != zeroTime && v.expiration.Before(time.Now()) {
			delete(c.data, k)
		}
	}
	c.mx.Unlock()
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
