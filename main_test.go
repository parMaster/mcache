package mcache

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testItem struct {
	key   string
	value interface{}
	ttl   int64
}

func Test_SimpleTest_Mcache(t *testing.T) {
	c := NewCache()

	assert.NotNil(t, c)
	assert.IsType(t, &Cache{}, c)
	assert.NotNil(t, c.data)

	testItems := []testItem{
		{"key0", "value0", 0},
		{"key1", "value1", 1},
		{"key2", "value2", 20},
		{"key3", "value3", 30},
		{"key4", "value4", 40},
		{"key5", "value5", 50},
		{"key6", "value6", 60},
		{"key7", "value7", 70000000},
	}
	noSuchKey := "noSuchKey"

	for _, item := range testItems {
		err := c.Set(item.key, item.value, item.ttl)
		assert.NoError(t, err)
	}

	for _, item := range testItems {
		value, err := c.Get(item.key)
		assert.NoError(t, err)
		assert.Equal(t, item.value, value)
	}

	_, err := c.Get(noSuchKey)
	assert.Error(t, err)
	assert.Equal(t, ErrKeyNotFound, err.Error())

	for _, item := range testItems {
		has, err := c.Has(item.key)
		assert.NoError(t, err)
		assert.True(t, has)
	}

	time.Sleep(time.Second * 2)

	has, err := c.Has(testItems[1].key)
	assert.Error(t, err)
	assert.Equal(t, ErrExpired, err.Error())
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
		assert.Equal(t, ErrKeyNotFound, err.Error())
	}

	c.Set("key", "value", 1)
	time.Sleep(time.Second * 2)
	err = c.Set("key", "newvalue", 1)
	assert.NoError(t, err)

	// old value should be rewritten
	value, err := c.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "newvalue", value)

	err = c.Set("key", "not a newer value", 1)
	assert.Equal(t, ErrKeyExists, err.Error())

	time.Sleep(time.Second * 2)
	err = c.Set("key", "even newer value", 1)
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

	// del should return error if key doesn't exist
	err = c.Del("noSuchKey")
	assert.Error(t, err)

	err = c.Clear()
	assert.NoError(t, err)
}

func TestConcurrentSetAndGet(t *testing.T) {
	cache := NewCache()

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

func TestMain(m *testing.M) {
	// Enable the race detector
	m.Run()
}
