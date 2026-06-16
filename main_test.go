package mcache

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strconv"
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
		result := c.Set(item.key, item.value, item.ttl)
		assert.True(t, result)
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

	time.Sleep(200 * time.Millisecond)

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

	c.Set("key", "value", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	result := c.Set("key", "newvalue", 100*time.Millisecond)
	assert.True(t, result)

	// old value should be rewritten
	value, err := c.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "newvalue", value)

	result = c.Set("key", "not a newer value", 1)
	assert.False(t, result)

	time.Sleep(200 * time.Millisecond)
	result = c.Set("key", "even newer value", 100*time.Millisecond)
	// key should be silently rewritten
	assert.True(t, result)
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

	numGoroutines := 10000
	wg := sync.WaitGroup{}
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", index)
			value := fmt.Sprintf("value-%d", index)

			res := cache.Set(key, value, 0)
			if !res {
				t.Errorf("Error setting value for key %s", key)
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

// TestConcurrentSetAndDel verifies that when Get succeeds, it never returns
// a zero value — ensuring the value is always consistent with what was Set.
func TestConcurrentSetAndDel(t *testing.T) {
	cache := NewCache[string]()
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		cache.Clear()
		cache.Set("key", "will be deleted", 0)

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

	if !cache.Set("key", "value", time.Millisecond*10) {
		t.Fatalf("Set failed")
	}

	// Wait long enough for: expiry (10ms) + cleanup tick (50ms) + margin.
	// 500ms gives ~10 tick opportunities; keeps the test stable under -race overhead.
	time.Sleep(time.Millisecond * 500)

	// Proactive cleanup deleted the key → ErrKeyNotFound (not ErrExpired from lazy delete)
	_, err := cache.Get("key")
	assert.ErrorIs(t, err, ErrKeyNotFound,
		"WithCleanup goroutine should have proactively deleted the expired key (ErrKeyNotFound), not lazy-deleted it (ErrExpired)")

	cancel()
	time.Sleep(time.Millisecond * 100)
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

			res := cache.Set(key, value, 100*time.Millisecond)
			assert.True(t, res, "Expected successfuly set value for key")
		}
		printAlloc("After Set " + strconv.Itoa(size) + " entries")

		cache.Cleanup()
		printAlloc("After Cleanup")

		// Check that the value has been deleted
		_, err := cache.Get("key_1")
		assert.Error(t, err, "Expected the key to be deleted")
		time.Sleep(200 * time.Millisecond)
	}
	runtime.GC()
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

// TestValuesAfterCleanup verifies that unexpired keys retain correct values after Cleanup
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

func TestConcurrentReads(t *testing.T) {
	cache := NewCache[string]()
	if !cache.Set("key", "value", time.Hour) {
		t.Fatalf("Set failed")
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
	if !cache.Set("key", "value", time.Millisecond) {
		t.Fatalf("Set failed")
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
	result := cache.Set("key", "value", -1*time.Second)
	assert.False(t, result)
}

func TestMain(m *testing.M) {
	m.Run()
}
