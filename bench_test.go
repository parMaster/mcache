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

// global var mutex:
// BenchmarkConcurrentRWD-4   	  293641	      5057 ns/op	     437 B/op	      13 allocs/op
// struct field mutex:
// BenchmarkConcurrentRWD-4   	  368404	      2837 ns/op	     207 B/op	      16 allocs/op
func BenchmarkConcurrentRWD(b *testing.B) {
	c1 := NewCache[int]()
	c2 := NewCache[int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go func(i int) {
			c1.Set(fmt.Sprintf("%d", i), i, time.Hour)
			c1.Get(fmt.Sprintf("%d", i))
			c1.Del(fmt.Sprintf("%d", i))
		}(i)
		go func(i int) {
			c2.Set(fmt.Sprintf("%d", i), i, time.Hour)
			c2.Get(fmt.Sprintf("%d", i))
			c2.Del(fmt.Sprintf("%d", i))
		}(i)
	}
	b.StopTimer()
	c1.Clear()
	c2.Clear()
}
