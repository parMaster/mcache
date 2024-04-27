package mcache

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkWrite
func BenchmarkWrite(b *testing.B) {
	mcache := NewCache[int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcache.Set(fmt.Sprintf("%d", i), i, time.Second)
	}
	b.StopTimer()
	mcache.Cleanup()
}

// BenchmarkRead
func BenchmarkRead(b *testing.B) {
	mcache := NewCache[int]()

	for i := 0; i < b.N; i++ {
		mcache.Set(fmt.Sprintf("%d", i), i, time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcache.Get(fmt.Sprintf("%d", i))
	}
	b.StopTimer()
	mcache.Clear()
}

// BenchmarkRW
func BenchmarkRWD(b *testing.B) {
	mcache := NewCache[int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mcache.Set(fmt.Sprintf("%d", i), i, time.Hour)
		mcache.Get(fmt.Sprintf("%d", i))
		mcache.Del(fmt.Sprintf("%d", i))
	}
	b.StopTimer()
	mcache.Clear()
}
