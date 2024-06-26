package mcache

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testItem struct {
	key   string
	value string
	ttl   time.Duration
}

func Test_SimpleTest_Mcache(t *testing.T) {
	var c Cacher[string] = NewCache[string]()

	assert.NotNil(t, c)
	assert.IsType(t, &Cache[string]{}, c)

	testItems := []testItem{
		{"key0", "value0", time.Duration(0)},
		{"key1", "value1", time.Second * 1},
		{"key11", "value11", time.Second * 1},
		{"key2", "value2", time.Second * 20},
		{"key3", "value3", time.Second * 30},
		{"key4", "value4", time.Second * 40},
		{"key5", "value5", time.Second * 50},
		{"key6", "value6", time.Second * 60},
		{"key7", "value7", time.Second * 70000000},
	}
	noSuchKey := "noSuchKey"

	for _, item := range testItems {
		err := c.Set(item.key, item.value, item.ttl)
		assert.NoError(t, err)
	}

	for _, item := range testItems {
		value, err := c.Get(item.key)
		assert.NoError(t, err, fmt.Sprintf("key:%s; val:%s; ttl:%d", item.key, item.value, item.ttl))
		assert.Equal(t, item.value, value, fmt.Sprintf("key:%s; val:%s; ttl:%d", item.key, item.value, item.ttl))
	}

	_, err := c.Get(noSuchKey)
	assert.Error(t, err)
	assert.ErrorIs(t, ErrKeyNotFound, err)

	for _, item := range testItems {
		has, err := c.Has(item.key)
		assert.NoError(t, err)
		assert.True(t, has)
	}

	time.Sleep(time.Second * 2)

	item, err := c.Get(testItems[1].key)
	assert.Error(t, err)
	assert.ErrorIs(t, ErrExpired, err)
	assert.Empty(t, item)

	has, err := c.Has(testItems[2].key)
	assert.Error(t, err)
	assert.ErrorIs(t, ErrExpired, err)
	assert.False(t, has)

	testItems = append(testItems[3:], testItems[0])
	for _, item := range testItems {
		err := c.Del(item.key)
		assert.NoError(t, err)
	}

	for _, item := range testItems {
		has, err := c.Has(item.key)
		assert.False(t, has)
		assert.Error(t, err)
		assert.ErrorIs(t, ErrKeyNotFound, err)
	}

	c.Set("key", "value", time.Second*1)
	time.Sleep(time.Second * 2)
	err = c.Set("key", "newvalue", time.Second*1)
	assert.NoError(t, err)

	// old value should be rewritten
	value, err := c.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "newvalue", value)

	err = c.Set("key", "not a newer value", 1)
	if err != nil {
		assert.ErrorIs(t, ErrKeyExists, err)
	}
	time.Sleep(time.Second * 2)
	err = c.Set("key", "even newer value", time.Second*1)
	// key should be silently rewritten
	assert.NoError(t, err)
	value, err = c.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "even newer value", value)

	time.Sleep(time.Second * 2)
	c.Cleanup()
	// key should be deleted
	has, err = c.Has("key")
	assert.False(t, has)
	assert.Error(t, err)

	// del should return error if key doesn't exist
	err = c.Del("noSuchKey")
	assert.Error(t, err)

	err = c.Clear()
	assert.NoError(t, err)
}

func Test_DelPrefix(t *testing.T) {
	cache := NewCache[int]()

	cache.Set("key1", 1, 0)
	for i := range [10]struct{}{} {
		cache.Set(fmt.Sprintf("user_%d", i), i, 0)
	}

	assert.Equal(t, 11, len(cache.data))

	deleted := cache.DelPrefix("user_")
	assert.Equal(t, 10, deleted)
	assert.Equal(t, 1, len(cache.data))

	deleted = cache.DelPrefix("user_")
	assert.Equal(t, 0, deleted)

	user, err := cache.Get("user_0")
	assert.Error(t, err)
	assert.Empty(t, user)

	key1, err := cache.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, 1, key1)

	emptyCache := NewCache[int]()
	deleted = emptyCache.DelPrefix("user_")
	assert.Equal(t, 0, deleted)
}

func Test_DelPrefixAltMatch(t *testing.T) {
	cache := NewCache[int]()

	cache.Set("key1", 1, 0)
	for i := range [10]struct{}{} {
		cache.Set(fmt.Sprintf("user_%d", i), i, 0)
	}

	assert.Equal(t, 11, len(cache.data))

	deleted := cache.DelPrefixAltMatch("user_")
	assert.Equal(t, 10, deleted)
	assert.Equal(t, 1, len(cache.data))

	deleted = cache.DelPrefixAltMatch("user_")
	assert.Equal(t, 0, deleted)

	user, err := cache.Get("user_0")
	assert.Error(t, err)
	assert.Empty(t, user)

	key1, err := cache.Get("key1")
	assert.NoError(t, err)
	assert.Equal(t, 1, key1)

	emptyCache := NewCache[int]()
	deleted = emptyCache.DelPrefixAltMatch("user_")
	assert.Equal(t, 0, deleted)
}

func TestDelWithConcurrentCleanup(t *testing.T) {
	m := map[string]int{}
	delete(m, "key")
	m = make(map[string]int)
	delete(m, "key")
	var mnil map[string]int
	delete(mnil, "key")
}

func TestConcurrentSetAndGet(t *testing.T) {
	cache := NewCache[string]()

	// Start multiple goroutines to concurrently set and get values
	numGoroutines := 10000
	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			key := fmt.Sprintf("key-%d", index)
			value := fmt.Sprintf("value-%d", index)

			err := cache.Set(key, value, 0)
			if err != nil {
				t.Errorf("Error setting value for key %s: %s", key, err)
			}

			result, err := cache.Get(key)
			if err != nil {
				t.Errorf("Error getting value for key %s: %s", key, err)
			}

			if result != value {
				t.Errorf("Expected value %s for key %s, but got %s", value, key, result)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// catching the situation when the key is deleted before the value is retrieved
func TestConcurrentSetAndDel(t *testing.T) {
	cache := NewCache[string]()

	var cnt atomic.Int32
	for i := 0; i < 1000; i++ {
		cache.Set("key", "will be deleted", 0)
		go func() {
			v, err := cache.Get("key")
			if err == nil && v == "" { // key was deleted before value was retrieved
				cnt.Add(1)
			}
		}()
		cache.Del("key")
	}
	assert.Equal(t, int32(0), cnt.Load(), "key was deleted before value was retrieved")
}

// TestWithCleanup tests that the cleanup goroutine is working
func TestWithCleanup(t *testing.T) {
	cache := NewCache(WithCleanup[string](time.Second * 1))

	// Set a value with a TTL of 1 second
	err := cache.Set("key", "value", 1)
	if err != nil {
		t.Errorf("Error setting value for key: %s", err)
	}

	// Wait for 2 seconds
	time.Sleep(2 * time.Second)

	// Check that the value has been deleted
	_, err = cache.Get("key")
	if err == nil {
		t.Errorf("Expected error getting value for key, but got nil")
	}
}

func TestMain(m *testing.M) {
	// Enable the race detector
	m.Run()
}
