package mcache

import (
	"fmt"
	"testing"
)

var mcache *Cache

// BenchmarkWrite
func BenchmarkWrite(b *testing.B) {
	mcache = NewCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcache.Set(fmt.Sprintf("%d", i), i, int64(i))
	}
	b.StopTimer()
	mcache.Cleanup()
}

// BenchmarkRead
func BenchmarkRead(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mcache.Get(fmt.Sprintf("%d", i))
	}
	mcache.Clear()
}

// BenchmarkRW
func BenchmarkRWD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mcache.Set(fmt.Sprintf("%d", i), i, int64(i))
		mcache.Get(fmt.Sprintf("%d", i))
		mcache.Del(fmt.Sprintf("%d", i))
	}
	mcache.Clear()
}
