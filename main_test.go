package mcache

import (
	"context"
	"fmt"
	"sync"
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
	assert.ErrorIs(t, err, ErrKeyNotFound)

	for _, item := range testItems {
		has, err := c.Has(item.key)
		assert.NoError(t, err)
		assert.True(t, has)
	}

	time.Sleep(time.Second * 2)

	item, err := c.Get(testItems[1].key)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrExpired)
	assert.Empty(t, item)

	has, err := c.Has(testItems[2].key)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrExpired)
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
		assert.ErrorIs(t, err, ErrKeyNotFound)
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
		assert.ErrorIs(t, err, ErrKeyExists)
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

// TestConcurrentSetAndDel verifies that when Get succeeds, it never returns
// a zero value — ensuring the value is always consistent with what was Set.
func TestConcurrentSetAndDel(t *testing.T) {
	cache := NewCache[string]()
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		cache.Clear()
		if err := cache.Set("key", "will be deleted", 0); err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := cache.Get("key")
			if err == nil && v != "will be deleted" {
				t.Errorf("Get returned wrong value: got %q, want %q", v, "will be deleted")
			}
		}()
		cache.Del("key")
	}
	wg.Wait()
}

func TestWithCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cleanup runs every 50ms, key expires in 10ms
	cache := NewCache(WithCleanup[string](ctx, time.Millisecond*50))

	err := cache.Set("key", "value", time.Millisecond*10)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait long enough for: expiry (10ms) + cleanup tick (50ms) + margin.
	// 500ms gives ~10 tick opportunities; keeps the test stable under -race overhead.
	time.Sleep(time.Millisecond * 500)

	// Proactive cleanup deleted the key → ErrKeyNotFound (not ErrExpired from lazy delete)
	_, err = cache.Get("key")
	assert.ErrorIs(t, err, ErrKeyNotFound,
		"WithCleanup goroutine should have proactively deleted the expired key (ErrKeyNotFound), not lazy-deleted it (ErrExpired)")

	// Cancel the context so the goroutine exits cleanly before the test ends.
	cancel()
	time.Sleep(time.Millisecond * 100)
}

func TestConcurrentReads(t *testing.T) {
	cache := NewCache[string]()
	if err := cache.Set("key", "value", time.Hour); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := cache.Get("key")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if v != "value" {
				t.Errorf("expected 'value', got %q", v)
			}
		}()
	}
	wg.Wait()
}

func TestDel_ExpiredKey(t *testing.T) {
	cache := NewCache[string]()
	if err := cache.Set("key", "value", time.Millisecond); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	time.Sleep(time.Millisecond * 10)

	err := cache.Del("key")
	assert.NoError(t, err)

	_, err = cache.Get("key")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestDel_TOCTOU(t *testing.T) {
	cache := NewCache[string]()
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		cache.Set("key", "original", 0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Del("key")
			cache.Set("key", "fresh", 0)
		}()
		cache.Del("key")
		wg.Wait()
		cache.Clear()
	}
}

func TestSet_NegativeTTL(t *testing.T) {
	cache := NewCache[string]()
	err := cache.Set("key", "value", -1*time.Second)
	assert.ErrorIs(t, err, ErrInvalidTTL)
}

func TestMain(m *testing.M) {
	// Enable the race detector
	m.Run()
}
