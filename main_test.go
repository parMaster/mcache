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

	c.Cleanup()

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
