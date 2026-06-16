# Fix Review Findings

**Goal:** Fix all 10 confirmed findings from the June 2026 code review: 3 production correctness bugs, 4 test bugs, 2 performance/resource issues, and 1 code quality cleanup.

**Architecture:** All changes are in two files — `main.go` (production code) and `main_test.go` (tests). No new files are introduced. Some changes are breaking API changes (`WithCleanup` signature).

**Tech Stack:** Go 1.20+, `sync.RWMutex`, `time.Ticker`, `context.Context`, `github.com/stretchr/testify/assert`

---

## Context (from discovery)

- files involved: `main.go` (173 lines), `main_test.go` (198 lines), `bench_test.go`
- `Cache[T]` is a generic in-memory key-value store with TTL, using `sync.RWMutex` for concurrency
- `Cacher[T]` is the public interface — `Set`, `Get`, `Has`, `Del`, `Cleanup`, `Clear`
- `WithCleanup` is a functional option that starts a background goroutine
- No existing `docs/plans/` directory (created by this task)

## Development Approach

- **testing approach**: TDD — for each production code bug, write the failing test first, confirm it fails, then implement the fix
- complete each task fully before moving to the next
- make small, focused changes
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task** — run `go test -race ./...`
- **CRITICAL: update this plan file when scope changes during implementation**
- **CRITICAL: single summary commit at the end** — one commit covers all implementation + plan move
- **CRITICAL: run `golangci-lint run ./...` before committing** — fix all linter issues first

## Solution Overview

The 10 findings break down into 4 groups, implemented in dependency order:

1. **Direct test fixes** (no production change needed): fix reversed `assert.ErrorIs` args and wrong TTL in `TestWithCleanup`
2. **Production correctness bugs** (TDD order: failing test → fix): negative TTL, `Del` TOCTOU race, `Del` returning `ErrExpired`
3. **Performance / resource fixes** (TDD: concurrent benchmark → fix): `Get`/`Has` write-lock serialization, `WithCleanup` goroutine leak
4. **Code quality** (no TDD needed): simplify `expired()`, fix redundant map lookup in `Get`

The `Del` TOCTOU fix changes its implementation from calling `Has` to owning a single write lock for the whole check-and-delete operation. The `Get`/`Has` lock upgrade changes from always holding a write lock to RLock-for-reads with a write-lock upgrade only when deletion is needed. The `WithCleanup` signature changes from `WithCleanup[T](ttl time.Duration)` to `WithCleanup[T](ctx context.Context, interval time.Duration)`.

## Technical Details

### New error sentinel
```go
ErrInvalidTTL = errors.New("invalid ttl")
```

### Del — new implementation (single lock, no TOCTOU)
```go
func (c *Cache[T]) Del(key string) error {
    c.mx.Lock()
    defer c.mx.Unlock()
    _, ok := c.data[key]
    if !ok {
        return ErrKeyNotFound
    }
    delete(c.data, key)
    return nil
}
```
Expired keys are deleted and `nil` is returned — the deletion succeeded, regardless of expiry state. This eliminates the TOCTOU and the confusing `ErrExpired` return from `Del`.

### Get — RLock-with-upgrade
```go
func (c *Cache[T]) Get(key string) (T, error) {
    var none T

    c.mx.RLock()
    item, ok := c.data[key]
    c.mx.RUnlock()

    if !ok {
        return none, ErrKeyNotFound
    }
    if !item.expired() {
        return item.value, nil  // hot path: no write lock needed
    }

    // Expired: upgrade to write lock for deletion
    c.mx.Lock()
    defer c.mx.Unlock()
    item, ok = c.data[key]
    if !ok {
        return none, ErrKeyNotFound
    }
    if item.expired() {
        delete(c.data, key)
        return none, ErrExpired
    }
    // Another goroutine refreshed the key between RUnlock and Lock
    return item.value, nil
}
```

### Has — same RLock-with-upgrade pattern
```go
func (c *Cache[T]) Has(key string) (bool, error) {
    c.mx.RLock()
    item, ok := c.data[key]
    c.mx.RUnlock()

    if !ok {
        return false, ErrKeyNotFound
    }
    if !item.expired() {
        return true, nil  // hot path
    }

    c.mx.Lock()
    defer c.mx.Unlock()
    item, ok = c.data[key]
    if !ok {
        return false, ErrKeyNotFound
    }
    if item.expired() {
        delete(c.data, key)
        return false, ErrExpired
    }
    return true, nil
}
```

### WithCleanup — context + ticker
```go
func WithCleanup[T any](ctx context.Context, interval time.Duration) func(*Cache[T]) {
    return func(c *Cache[T]) {
        ticker := time.NewTicker(interval)
        go func() {
            defer ticker.Stop()
            for {
                select {
                case <-ticker.C:
                    c.Cleanup()
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
}
```

---

## Progress Tracking

Mark completed items with `[x]` immediately when done. Add newly discovered tasks with ➕. Document blockers with ⚠️.

---

## Implementation Steps

### Task 1: Fix reversed `assert.ErrorIs` arguments

These are test code bugs — no TDD needed, just fix directly.

**Background:** Testify's signature is `assert.ErrorIs(t TestingT, err error, target error)` — actual error first, sentinel second. All 5 call sites in `main_test.go` have the arguments swapped.

**Files:**
- Modify: `main_test.go`

- [ ] Fix line 50 — `c.Get(noSuchKey)` returns `ErrKeyNotFound`:
  ```go
  // Before:
  assert.ErrorIs(t, ErrKeyNotFound, err)
  // After:
  assert.ErrorIs(t, err, ErrKeyNotFound)
  ```

- [ ] Fix line 62 — expired key returns `ErrExpired`:
  ```go
  // Before:
  assert.ErrorIs(t, ErrExpired, err)
  // After:
  assert.ErrorIs(t, err, ErrExpired)
  ```

- [ ] Fix line 67 — `Has` on expired key returns `ErrExpired`:
  ```go
  // Before:
  assert.ErrorIs(t, ErrExpired, err)
  // After:
  assert.ErrorIs(t, err, ErrExpired)
  ```

- [ ] Fix line 80 — `Has` on deleted key returns `ErrKeyNotFound`:
  ```go
  // Before:
  assert.ErrorIs(t, ErrKeyNotFound, err)
  // After:
  assert.ErrorIs(t, err, ErrKeyNotFound)
  ```

- [ ] Fix line 95 — `Set` on existing key returns `ErrKeyExists`:
  ```go
  // Before:
  assert.ErrorIs(t, ErrKeyExists, err)
  // After:
  assert.ErrorIs(t, err, ErrKeyExists)
  ```

- [ ] Run tests to confirm they still pass:
  ```
  go test -race ./... -run Test_SimpleTest_Mcache -v
  ```
  Expected: PASS

---

### Task 2: Fix negative TTL creating immortal entries (TDD)

**Background:** `Set` with `ttl < 0` silently creates a never-expiring entry because `ttl > time.Duration(0)` skips the expiration assignment.

**Files:**
- Modify: `main.go`, `main_test.go`

- [ ] **Add failing test** in `main_test.go` as a new top-level function:
  ```go
  func TestSet_NegativeTTL(t *testing.T) {
      cache := NewCache[string]()
      err := cache.Set("key", "value", -1*time.Second)
      assert.ErrorIs(t, err, ErrInvalidTTL)
  }
  ```

- [ ] **Run test to confirm it fails:**
  ```
  go test -race ./... -run TestSet_NegativeTTL -v
  ```
  Expected: FAIL (ErrInvalidTTL is not defined yet)

- [ ] **Add `ErrInvalidTTL` to the error block in `main.go`:**
  ```go
  var (
      ErrKeyNotFound = errors.New("key not found")
      ErrKeyExists   = errors.New("key already exists")
      ErrExpired     = errors.New("key expired")
      ErrInvalidTTL  = errors.New("invalid ttl")
  )
  ```

- [ ] **Add validation at the top of `Set` in `main.go`**, before the lock:
  ```go
  func (c *Cache[T]) Set(key string, value T, ttl time.Duration) error {
      if ttl < 0 {
          return ErrInvalidTTL
      }
      c.mx.Lock()
      defer c.mx.Unlock()
      // ... rest of Set unchanged
  ```

- [ ] **Also change `ttl > time.Duration(0)` to `ttl > 0`** (idiomatic, `time.Duration(0)` cast is redundant):
  ```go
  if ttl > 0 {
      expiration = time.Now().Add(ttl)
  }
  ```

- [ ] **Run tests to confirm they pass:**
  ```
  go test -race ./... -v
  ```
  Expected: PASS

---

### Task 3: Fix `Del` TOCTOU race and `ErrExpired` contract (TDD)

**Background:** `Del` calls `Has()` (acquires+releases write lock) then re-acquires the lock to delete — a window where another goroutine can Set a fresh key that Del then silently destroys. Additionally, `Del` returns `ErrExpired` even though `Has` already deleted the key as a side-effect.

**Fix:** Inline the check and delete under a single write lock. Treat expired keys as "successfully deleted" (return `nil`) since the end result is the same — the key is gone.

**Files:**
- Modify: `main.go`, `main_test.go`

- [ ] **Add failing test** for the `ErrExpired` contract bug.

  `main_test.go` imports `sync/atomic` but not `sync`. Add `"sync"` to the import block — it is needed for `sync.WaitGroup` below.

  ```go
  func TestDel_ExpiredKey(t *testing.T) {
      cache := NewCache[string]()
      if err := cache.Set("key", "value", time.Millisecond); err != nil {
          t.Fatalf("Set failed: %v", err)
      }

      time.Sleep(time.Millisecond * 10) // ensure expired

      // Deleting an expired key should succeed, not return ErrExpired
      err := cache.Del("key")
      assert.NoError(t, err)

      // Key should be gone
      _, err = cache.Get("key")
      assert.ErrorIs(t, err, ErrKeyNotFound)
  }
  ```

  Also add a stress test for the TOCTOU race (run under `-race`):
  ```go
  func TestDel_TOCTOU(t *testing.T) {
      cache := NewCache[string]()
      var wg sync.WaitGroup

      for i := 0; i < 1000; i++ {
          cache.Set("key", "original", 0)
          wg.Add(1)
          go func() {
              defer wg.Done()
              // Concurrently: attempt delete, then immediately re-set
              cache.Del("key")
              cache.Set("key", "fresh", 0)
          }()
          cache.Del("key")
          wg.Wait()
          cache.Clear()
      }
  }
  ```
  The TOCTOU test is primarily a race-detector test (`-race`). With old code the race detector may surface non-atomic access patterns.

- [ ] **Run tests to confirm the ErrExpired test fails:**
  ```
  go test -race ./... -run TestDel_ExpiredKey -v
  ```
  Expected: FAIL (old code returns ErrExpired)

- [ ] **Replace the `Del` implementation in `main.go`:**
  ```go
  // Del deletes a key-value pair.
  // Returns ErrKeyNotFound if the key does not exist.
  // Expired keys are deleted and nil is returned — the key is gone either way.
  func (c *Cache[T]) Del(key string) error {
      c.mx.Lock()
      defer c.mx.Unlock()
      _, ok := c.data[key]
      if !ok {
          return ErrKeyNotFound
      }
      delete(c.data, key)
      return nil
  }
  ```

- [ ] **Run all tests:**
  ```
  go test -race ./... -v
  ```
  Expected: PASS

  Note: the `Test_SimpleTest_Mcache` test at line 113 calls `c.Del("noSuchKey")` and asserts an error — that still returns `ErrKeyNotFound`. ✓
  The Del loop at line 71 deletes live keys — those still return nil. ✓

---

### Task 4: Fix `Get` and `Has` write-lock serialization (TDD)

**Background:** `Get` and `Has` both hold `c.mx.Lock()` (exclusive write lock) for the entire operation. The non-expired hot path only reads the map, so `RLock` suffices. Write lock is only needed when deleting an expired entry.

**Fix:** RLock for the initial read; if the entry is expired, upgrade to a write lock (with re-check under the write lock, since state may change between the two acquisitions).

**Files:**
- Modify: `main.go`, `main_test.go`

- [ ] **Add concurrent read test** that verifies reads proceed in parallel (not just that they return correct values — that's already tested). This test is mainly a `-race` detector sanity check.

  If `"sync"` is not already in `main_test.go` imports (added in Task 3), add it now.

  ```go
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
  ```

- [ ] **Run the test to confirm it passes with old code** (not a failing test — this is a correctness verification):
  ```
  go test -race ./... -run TestConcurrentReads -v
  ```
  Expected: PASS (correctness is fine; this test will still pass after the lock change)

- [ ] **Replace `Get` in `main.go`:**
  ```go
  // Get returns the value for key.
  // Returns ErrKeyNotFound if key does not exist.
  // Returns ErrExpired and deletes the key if it has expired.
  func (c *Cache[T]) Get(key string) (T, error) {
      var none T

      c.mx.RLock()
      item, ok := c.data[key]
      c.mx.RUnlock()

      if !ok {
          return none, ErrKeyNotFound
      }
      if !item.expired() {
          return item.value, nil
      }

      // Expired path: upgrade to write lock for deletion.
      // Re-check under write lock — another goroutine may have replaced the key.
      c.mx.Lock()
      defer c.mx.Unlock()
      item, ok = c.data[key]
      if !ok {
          return none, ErrKeyNotFound
      }
      if item.expired() {
          delete(c.data, key)
          return none, ErrExpired
      }
      return item.value, nil
  }
  ```

- [ ] **Replace `Has` in `main.go`:**
  ```go
  // Has checks if key exists and is not expired.
  // Returns ErrKeyNotFound if key does not exist.
  // Returns ErrExpired and deletes the key if it has expired.
  func (c *Cache[T]) Has(key string) (bool, error) {
      c.mx.RLock()
      item, ok := c.data[key]
      c.mx.RUnlock()

      if !ok {
          return false, ErrKeyNotFound
      }
      if !item.expired() {
          return true, nil
      }

      c.mx.Lock()
      defer c.mx.Unlock()
      item, ok = c.data[key]
      if !ok {
          return false, ErrKeyNotFound
      }
      if item.expired() {
          delete(c.data, key)
          return false, ErrExpired
      }
      return true, nil
  }
  ```

- [ ] **Run all tests:**
  ```
  go test -race ./... -v
  ```
  Expected: PASS

---

### Task 5: Fix `WithCleanup` goroutine leak (TDD)

**Background:** The goroutine in `WithCleanup` runs forever — no context, no stop channel. Also uses `time.Sleep` which drifts. The existing test passes a `ttl=1` (1 nanosecond) which causes the key to expire via lazy deletion in `Get`, not via the cleanup goroutine — so the cleanup goroutine is never actually verified.

**Fix:** Change signature to `WithCleanup[T](ctx context.Context, interval time.Duration)`, use `time.Ticker`, stop on `ctx.Done()`. Rewrite the test to distinguish lazy deletion from proactive cleanup.

**Key insight for the test:** After the cleanup goroutine deletes an expired key, `Get` returns `ErrKeyNotFound` (key not in map). With only lazy deletion, `Get` returns `ErrExpired` (key still in map but expired). These are different error values — the test can assert `ErrKeyNotFound` to prove the goroutine ran.

**Files:**
- Modify: `main.go`, `main_test.go`

- [ ] **Rewrite `TestWithCleanup` in `main_test.go`** (fix both the TTL and the assertion, and update for the new `WithCleanup` signature).

  Add `"context"` to imports in `main_test.go`.

  ```go
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
  ```

- [ ] **Run test to confirm it fails with old code:**
  ```
  go test -race ./... -run TestWithCleanup -v
  ```
  Expected: FAIL (old code returns `ErrExpired` not `ErrKeyNotFound`, and doesn't compile with new signature)

- [ ] **Update `WithCleanup` in `main.go`:**

  Add `"context"` to imports.

  Replace the function:
  ```go
  // WithCleanup is a functional option that starts a background goroutine to
  // periodically delete expired keys. The goroutine stops when ctx is cancelled.
  func WithCleanup[T any](ctx context.Context, interval time.Duration) func(*Cache[T]) {
      return func(c *Cache[T]) {
          ticker := time.NewTicker(interval)
          go func() {
              defer ticker.Stop()
              for {
                  select {
                  case <-ticker.C:
                      c.Cleanup()
                  case <-ctx.Done():
                      return
                  }
              }
          }()
      }
  }
  ```

- [ ] **Run all tests:**
  ```
  go test -race ./... -v
  ```
  Expected: PASS

---

### Task 6: Fix `TestConcurrentSetAndDel` (test redesign)

**Background:** Two bugs in this test:
1. The detection condition `err == nil && v == ""` is structurally impossible — `Get` only returns `""` when `err != nil`, so `cnt` can never increment regardless of any race.
2. 1000 goroutines are launched but not waited for before the assertion.

**Fix:** Rewrite the test with a `sync.WaitGroup` and a meaningful condition: when `Get` succeeds (`err == nil`), the returned value must be the expected string.

**Files:**
- Modify: `main_test.go`

- [ ] **Replace `TestConcurrentSetAndDel` in `main_test.go`:**
  ```go
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
              // If Get succeeded, the value must be the one that was Set
              if err == nil && v != "will be deleted" {
                  t.Errorf("Get returned wrong value: got %q, want %q", v, "will be deleted")
              }
          }()
          cache.Del("key")
      }
      wg.Wait()
  }
  ```

- [ ] **Run test:**
  ```
  go test -race ./... -run TestConcurrentSetAndDel -v
  ```
  Expected: PASS

---

### Task 7: Code quality cleanup

These are simple in-place fixes with no behavior change.

**Files:**
- Modify: `main.go`

- [ ] **Simplify `expired()` — remove redundant if/return:**
  ```go
  // Before:
  func (cacheItem CacheItem[T]) expired() bool {
      if !cacheItem.expiration.IsZero() && cacheItem.expiration.Before(time.Now()) {
          return true
      }
      return false
  }

  // After:
  func (cacheItem CacheItem[T]) expired() bool {
      return !cacheItem.expiration.IsZero() && cacheItem.expiration.Before(time.Now())
  }
  ```

- [ ] **Fix redundant map lookup in `Get`** — this was already fixed in Task 4 by returning `item.value` directly. Verify the new `Get` implementation uses `item.value`, not `c.data[key].value`. ✓ (done as part of Task 4)

- [ ] **Run all tests:**
  ```
  go test -race ./... -v
  ```
  Expected: PASS

---

### Task 8: Verify acceptance criteria

- [ ] All 10 findings are addressed:
  - [ ] Finding 1 — `Del` TOCTOU (Task 3)
  - [ ] Finding 2 — negative TTL immortal entry (Task 2)
  - [ ] Finding 3 — `Del` returns `ErrExpired` (Task 3)
  - [ ] Finding 4 — `TestConcurrentSetAndDel` impossible condition (Task 6)
  - [ ] Finding 5 — `assert.ErrorIs` args reversed (Task 1)
  - [ ] Finding 6 — `TestWithCleanup` wrong TTL (Task 5)
  - [ ] Finding 7 — `TestConcurrentSetAndDel` unwaited goroutines (Task 6)
  - [ ] Finding 8 — `Get`/`Has` write-lock serialization (Task 4)
  - [ ] Finding 9 — `WithCleanup` goroutine leak (Task 5)
  - [ ] Finding 10 — `Get` redundant map lookup (Task 4 + Task 7)

- [ ] Run the full test suite including race detector:
  ```
  go test -race -coverprofile=coverage.txt -covermode=atomic ./...
  ```
  Expected: PASS, no data races

- [ ] Run the linter:
  ```
  golangci-lint run ./...
  ```
  Expected: no issues

- [ ] Confirm `bench_test.go` does not use `WithCleanup` (it uses the old signature if it does):
  ```
  grep -n WithCleanup bench_test.go
  ```
  Expected: no output (bench_test.go does not call `WithCleanup`)

- [ ] Verify examples still compile (they don't use `WithCleanup`, so no changes needed):
  ```
  go build ./examples/...
  ```
  Expected: success

---

### Task 9: Wrap up and commit

- [ ] Move this plan to `docs/plans/completed/`:
  ```
  mkdir -p docs/plans/completed
  mv docs/plans/20260616-fix-review-findings.md docs/plans/completed/
  ```

- [ ] Single summary commit — stage all changed files:
  ```
  git add main.go main_test.go docs/plans/completed/20260616-fix-review-findings.md
  git commit -m "$(cat <<'EOF'
fix: address 10 code review findings

- Del: inline check-and-delete under single write lock (fixes TOCTOU race)
- Del: return nil for expired keys (was returning ErrExpired while key was already deleted)
- Set: reject negative ttl with ErrInvalidTTL
- Get/Has: use RLock for hot path, upgrade to Lock only when deleting expired entry
- WithCleanup: accept context.Context for cancellation, use time.Ticker to fix drift
- expired(): simplify to single boolean return expression
- Tests: fix 5 reversed assert.ErrorIs argument pairs
- Tests: rewrite TestConcurrentSetAndDel with WaitGroup and reachable condition
- Tests: rewrite TestWithCleanup to verify proactive cleanup via ErrKeyNotFound

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
  )"
  ```

- [ ] Open draft PR — invoke `planning:pr`

---

## Post-Completion

- The `WithCleanup` signature is a breaking API change: callers must now pass a `context.Context` as the first argument. If this is a public library, document the change in the README and consider a semver major bump.
- `ErrInvalidTTL` is a new public sentinel — mention it in the README's error reference.
