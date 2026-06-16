# API Improvements Backlog

Captured after code review session (2026-06-16). Items ranked by impact.

---

## 1. Force-overwrite / Update operation (High impact)

`Set` silently refuses to overwrite a live key. Callers who need to update an
existing entry must `Del` then `Set`, which creates a window where the key is
absent — a real race under concurrent access.

**Proposed addition:**
```go
// Update sets key unconditionally, replacing any existing value or expiration.
// Returns false only for invalid TTL (< 0).
Update(key string, value T, ttl time.Duration) bool
```

---

## 2. `Has` return signature is redundant (Medium impact)

`Has(key string) (bool, error)` — when `err == nil` the bool is always `true`;
when `err != nil` it is always `false`. The bool is fully derivable from the
error, making it noise for callers.

**Options:**
- Return `error` only (`nil` = exists, sentinel = not-found/expired)
- Return `bool` only (loses the not-found vs expired distinction)
- Keep as-is but document the invariant explicitly

Breaking change — needs a major version bump if changed.

---

## 3. `Clear() error` promises nothing (Low impact)

The implementation always returns `nil`. The error return is misleading because
it implies the operation can fail.

**Proposed change:** drop the return value → `Clear()`.

Breaking change — requires updating the `Cacher` interface and all callers.

---

## 4. `Cacher` interface exposes `Cleanup` (Low impact)

`Cleanup` is a maintenance operation intended to be driven by `WithCleanup`
internally. Exposing it on the public interface implies callers are responsible
for invoking it, which conflicts with the background-goroutine design.

**Proposed change:** remove `Cleanup()` from `Cacher`. Keep the method on
`*Cache` for callers who manage cleanup manually.

Breaking change for any code typed against `Cacher`.

---

## 5. No `Len()` method (Low impact)

No way to observe cache occupancy. Useful for metrics, capacity decisions, and
debugging.

**Proposed addition:**
```go
// Len returns the number of keys currently in the cache, including expired
// keys that have not yet been cleaned up.
Len() int
```

---

## 6. Single mutex limits write throughput at scale (Performance)

A single `sync.RWMutex` is a bottleneck under heavy concurrent writes. A
sharded design (N sub-maps each with its own lock, key routed by hash) scales
write throughput roughly linearly with shard count.

Not a concern for typical single-instance usage. Natural next step if the
library targets high-throughput scenarios.

**Approach:** introduce a `ShardedCache[T]` type (separate file, same package)
implementing `Cacher[T]`, keeping `Cache[T]` as the simple default.
