---
phase: "04"
plan: "01"
subsystem: "build-parallel"
tags: [errgroup, concurrency, output-buffering, atomic-progress]

dependency-graph:
  requires: [03-incremental-builds]
  provides: [parallel-compilation, bounded-concurrency, buffered-output]
  affects: [04-02, 04-03]

tech-stack:
  added:
    - name: "golang.org/x/sync/errgroup"
      version: "v0.17.0"
      purpose: "Parallel task execution with error propagation"
  patterns:
    - "Worker pool with bounded concurrency via errgroup.SetLimit"
    - "Output buffering per compilation to prevent interleaving"
    - "Atomic progress tracking with sync/atomic.Int64"

file-tracking:
  key-files:
    created:
      - internal/build/parallel.go
      - internal/build/parallel_test.go
    modified:
      - go.mod

decisions:
  - id: "parallel-errgroup"
    choice: "errgroup.WithContext + SetLimit for parallel execution"
    rationale: "Standard Go pattern with automatic context cancellation and bounded concurrency"
  - id: "output-buffering"
    choice: "bytes.Buffer per compilation, print atomically after completion"
    rationale: "Prevents interleaved output from concurrent compilations"
  - id: "keep-going-nil"
    choice: "Return nil from g.Go when keepGoing is true"
    rationale: "Allows other goroutines to continue without context cancellation"

metrics:
  duration: "3.5min"
  completed: "2026-01-23"
---

# Phase 04 Plan 01: Parallel Compilation Infrastructure Summary

Parallel compilation orchestrator with errgroup, bounded concurrency, and buffered output for non-interleaved progress display.

## What Was Built

### ParallelCompiler (internal/build/parallel.go)

Core parallel compilation engine that wraps the sequential Compiler:

1. **ParallelCompiler struct**: Holds compiler reference, jobs limit, keepGoing flag, verbose setting
2. **ParallelResult struct**: Captures source path, object path, dep file, buffered output, error, duration
3. **CompileParallel method**: Uses errgroup.WithContext for parallel execution with SetLimit for bounded concurrency
4. **Output buffering**: Each compilation captures its output to bytes.Buffer, printed atomically after completion
5. **Progress tracking**: Atomic Int64 counter for completed files, mutex-protected active file list

### Key Implementation Details

```go
// Bounded concurrency with errgroup
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(p.jobs)

// Launch workers for each source
for _, opts := range sources {
    opts := opts // Capture
    g.Go(func() error {
        result := p.compileWithBuffering(ctx, opts)
        results <- result
        if result.Error != nil && !p.keepGoing {
            return result.Error // Fail fast
        }
        return nil
    })
}
```

### Tests (internal/build/parallel_test.go)

Comprehensive test coverage (533 lines):
- Single file and multiple file compilation
- Concurrency limit verification
- Keep-going mode (continues after errors)
- Fail-fast mode (stops on first error)
- Context cancellation handling
- Empty sources handling
- Progress tracking unit tests
- Active file tracking unit tests
- Output non-interleaving verification

## Commits

| Commit | Type | Description |
|--------|------|-------------|
| 1d344b9 | feat | Create ParallelCompiler with errgroup-based execution |
| 5095f59 | feat | Implement buffered compilation for output isolation |
| 911ee39 | test | Add parallel compilation unit tests |

## Verification Results

1. `go build ./...` - PASS (no compilation errors)
2. `go test -mod=mod -v ./internal/build/... -run TestParallelCompiler` - PASS (10/10 tests)
3. Exports verified: ParallelCompiler, CompileParallel, ParallelResult
4. Output buffering prevents interleaving (verified by TestParallelCompiler_OutputNotInterleaved)

## Deviations from Plan

### Design Clarification: Compilation Capture

**Found during:** Task 2
**Issue:** Plan suggested using `compiler.CompileSource` directly but this streams output. To capture output, needed to implement compilation logic with non-streaming executor.
**Solution:** Created `compileSourceWithCapture` method that replicates compilation logic but captures stdout/stderr to buffer via executor's non-streaming mode.
**Impact:** More code but proper output isolation. Still uses `compiler.compilerCmd` for toolchain selection.

## Success Criteria Met

- [x] ParallelCompiler struct with errgroup-based execution
- [x] CompileParallel method with SetLimit for bounded concurrency
- [x] Output buffering via bytes.Buffer per compilation
- [x] Atomic progress tracking with sync/atomic
- [x] Unit tests for concurrency limit, keep-going mode, and cancellation

## Next Steps

This plan provides the foundation for:
- **04-02**: Signal handling with graceful shutdown (uses context cancellation from errgroup)
- **04-03**: Builder integration with --jobs flag and progress updates
