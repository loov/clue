# Quick Task 013: Fix go vet issues

## Objective

Run `go vet ./...` and fix all reported issues.

## Issues Found

```
internal/build/parallel_test.go:230:6: assignment copies lock value to _: sync/atomic.Int32 contains sync/atomic.noCopy
internal/build/parallel_test.go:231:6: assignment copies lock value to _: sync/atomic.Int32 contains sync/atomic.noCopy
```

## Root Cause

The test `TestParallelCompiler_ConcurrencyLimit` declared two `atomic.Int32` variables (`maxConcurrent` and `currentConcurrent`) that were intended for concurrency tracking but never actually used. To silence the "unused variable" warning, they were assigned to blank identifiers (`_ = maxConcurrent`), but this pattern doesn't work with atomic types because they contain `sync/atomic.noCopy`.

## Fix

1. Remove the unused `maxConcurrent` and `currentConcurrent` variable declarations
2. Remove the `_ = maxConcurrent` and `_ = currentConcurrent` assignments
3. Remove the now-unused `sync/atomic` import

## Files Modified

- `internal/build/parallel_test.go`
