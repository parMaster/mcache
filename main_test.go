package mcache

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"sync"
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
		{"key1", "value1", time.Millisecond * 100},
		{"key11", "value11", time.Millisecond * 100},
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

	time.Sleep(200 * time.Millisecond)

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

	c.Set("key", "value", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	err = c.Set("key", "newvalue", 100*time.Millisecond)
	assert.NoError(t, err)

	// old value should be rewritten
	value, err := c.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "newvalue", value)

	err = c.Set("key", "not a newer value", 1)
	if err != nil {
		assert.ErrorIs(t, ErrKeyExists, err)
	}
	time.Sleep(200 * time.Millisecond)
	err = c.Set("key", "even newer value", 100*time.Millisecond)
	// key should be silently rewritten
	assert.NoError(t, err)
	value, err = c.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "even newer value", value)

	time.Sleep(200 * time.Millisecond)
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
	wg := sync.WaitGroup{}
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
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
		}(i)
	}

	wg.Wait()
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
	cache := NewCache(WithCleanup[string](time.Millisecond * 100))

	// Set a value with a TTL of 1 second
	err := cache.Set("key", "value", 1)
	assert.NoError(t, err, "Expected no error setting value for key")

	time.Sleep(time.Millisecond * 200)

	// Check that the value expired
	_, err = cache.Get("key")
	assert.Error(t, err, "Expected the key to expire and be deleted")
}

func getAlloc() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

func printAlloc(message string) {
	log.Printf("%s %d KB\n", message, getAlloc()/1024)
}

// TestWithSize tests that the memory doesn't leak after Cleanup
func TestWithSize(t *testing.T) {
	size := 10_000
	printAlloc("Before")
	cache := NewCache(WithSize[string](size))
	memBefore := getAlloc()
	for iter := 0; iter < 10; iter++ {
		printAlloc("After NewCache")

		for i := 0; i < size; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)

			err := cache.Set(key, value, 100*time.Millisecond)
			assert.NoError(t, err)
		}
		printAlloc("After Set " + strconv.Itoa(size) + " entries")

		cache.Cleanup()
		printAlloc("After Cleanup")

		// Check that the value has been deleted
		_, err := cache.Get("key_1")
		assert.Error(t, err, "Expected the key to be deleted")
		time.Sleep(200 * time.Millisecond)
	}
	runtime.GC() // force GC to clean up the cache, make sure it's not leaking
	printAlloc("After")
	memAfter := getAlloc()
	assert.Less(t, memAfter, memBefore*2, "Memory usage should not grow more than twice")
}

// TestThreadSafeCleanup tests that the cleanup goroutine is thread-safe
func TestThreadSafeCleanup(t *testing.T) {
	cache := NewCache[string]()
	for i := 0; i < 100; i++ {
		cache.Set("key_"+strconv.Itoa(i), "value", time.Duration(10)*time.Millisecond)
		go cache.Cleanup()
		cache.Set("key_"+strconv.Itoa(i), "value", time.Duration(20)*time.Millisecond)
		go cache.Cleanup()
		cache.Set("key_"+strconv.Itoa(i), "value", time.Duration(30)*time.Millisecond)
		go cache.Cleanup()
	}
}

// verify that after the Cleanup, the unexpired keys are moved with the right values
func TestValuesAfterCleanup(t *testing.T) {
	cache := NewCache[string]()
	for i := 0; i < 10; i++ {
		key := "key_" + strconv.Itoa(i)
		val := "value_" + strconv.Itoa(i)
		cache.Set(key, val, time.Second)
	}
	for i := 0; i < 10; i++ {
		key := "key_expired_" + strconv.Itoa(i)
		val := "value_expired_" + strconv.Itoa(i)
		cache.Set(key, val, time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	cache.Cleanup()
	for i := 0; i < 10; i++ {
		key := "key_" + strconv.Itoa(i)
		val := "value_" + strconv.Itoa(i)
		movedVal, err := cache.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, val, movedVal)
	}
	var emptyVal string
	for i := 0; i < 10; i++ {
		key := "key_expired_" + strconv.Itoa(i)
		movedVal, err := cache.Get(key)
		assert.Error(t, err)
		assert.Equal(t, emptyVal, movedVal)
	}
}

func TestMain(m *testing.M) {
	// Enable the race detector
	m.Run()
}
