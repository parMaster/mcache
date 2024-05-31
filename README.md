# mcache [![codecov](https://codecov.io/gh/parMaster/mcache/branch/main/graph/badge.svg?token=K6685ZN3YS)](https://codecov.io/gh/parMaster/mcache) ![GitHub](https://img.shields.io/github/license/parMaster/mcache) [![Go Report Card](https://goreportcard.com/badge/github.com/parMaster/mcache)](https://goreportcard.com/report/github.com/parMaster/mcache) [![Go](https://github.com/parMaster/zoomrs/actions/workflows/go.yml/badge.svg)](https://github.com/parMaster/zoomrs/actions/workflows/go.yml)

`mcache` is a simple, fast, thread-safe in-memory cache library with by-key TTL written in Go.

## Features

- Thread-safe cache operations
- Set key-value pairs with optional expiration time
- Get values by key
- Check if a key exists
- Delete key-value pairs
- Clear the entire cache
- Cleanup expired key-value pairs

## Installation

Use `go get` to install the package:

```shell
go get github.com/parMaster/mcache
```

## Usage

Import the `mcache` package in your Go code:

```go
import "github.com/parMaster/mcache"
```

Create a new cache instance using the `NewCache` constructor, and use it to perform cache operations:

```go
cache := mcache.NewCache[string]()
data, err := cache.Get("key")
if err != nil {
	data = ExpensiveFunctionCall()
	cache.Set("key", data, 5*time.Minute) // cache data for 5 minutes
}
```
## Examples

See the [examples](https://github.com/parMaster/mcache/tree/main/examples) directory for more examples.

## API Reference
### Set

Set a key-value pair in the cache. The key must be a `string`, value type defined during cache creation, ttl is `time.Duration` type. If ttl is 0, the key-value pair will not expire.:

```go
err := cache.Set("key", "value", time.Duration(0))
if err != nil {
    // handle error
}
```

If the key already exists and is not expired, an error `mcache.ErrKeyExists` will be returned. If the key exists but is expired, the value will be updated.

You can also set a key-value pair with an expiration time (in seconds):

```go
err := cache.Set("key", "value", time.Minute)
if err != nil {
    // handle error
}
```

The value will automatically expire after the specified duration.

### Get

Retrieve a value from the cache by key:

```go
value, err := cache.Get("key")
if err != nil {
    // handle error
}
```

If the key does not exist, an error `mcache.ErrKeyNotFound` will be returned. If the key exists but is expired, an error `mcache.ErrExpired` will be returned, and the key-value pair will be deleted.

Either error or value could be checked to determine if the key exists.

### Has

Check if a key exists in the cache:

```go
exists, err := cache.Has("key")
if err != nil {
    // handle error
}

if exists {
    // key exists
} else {
    // key does not exist
}
```

If the key exists but is expired, an error `mcache.ErrExpired` will be returned, and the key-value pair will be deleted.

### Delete

Delete a key-value pair from the cache:

```go
err := cache.Del("key")
if err != nil {
    // handle error
}
```

### Clear

Clear the entire cache:

```go
err := cache.Clear()
if err != nil {
    // handle error
}
```

### Cleanup

Cleanup expired key-value pairs in the cache. You can call this method periodically to remove expired key-value pairs from the cache:

```go
cache.Cleanup()
```

`WithCleanup` is a functional option to the `NewCache` constructor that allows you to specify a cleanup interval:

```go
cache := mcache.NewCache(mcache.WithCleanup[string](time.Minute)) // cleanup every 60 seconds
```
It will basically run a `Cleanup` method in a goroutine with a time interval.

## Tests and Benchmarks

100% test coverage:

```shell
$ go test -cover -race .

ok      github.com/parMaster/mcache     8.239s  coverage: 100.0% of statements
```
Blinding fast and efficient:

```shell
$ go test -bench . -benchmem
goos: darwin
goarch: amd64
pkg: github.com/parMaster/mcache
cpu: Intel(R) Core(TM) i5-5257U CPU @ 2.70GHz
BenchmarkWrite-4   	 1333594	       772.8 ns/op	     196 B/op	       2 allocs/op
BenchmarkRead-4    	 2578328	       490.7 ns/op	      15 B/op	       1 allocs/op
BenchmarkRWD-4     	 1277389	       939.7 ns/op	      47 B/op	       5 allocs/op
PASS
ok  	github.com/parMaster/mcache	19.769s
```

## Contributing

Contributions are welcome! If you find any issues or have suggestions for improvements, please open an issue or submit a pull request.

## License

This project is licensed under the [MIT](https://choosealicense.com/licenses/mit/) license.
