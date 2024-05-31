package mcache

import (
	"fmt"
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
		{"key0", "value0", time.Second * 0},
		{"key1", "value1", time.Second * 1},
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

	has, err := c.Has(testItems[1].key)
	assert.Error(t, err)
	assert.ErrorIs(t, ErrExpired, err)
	assert.False(t, has)

	testItems = append(testItems[2:], testItems[0])
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
