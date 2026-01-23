# Phase 4: Parallel Execution - Research

**Researched:** 2026-01-23
**Domain:** Go concurrency patterns for parallel build systems
**Confidence:** HIGH

## Summary

Parallel compilation in Go is best implemented using the worker pool pattern with controlled concurrency. The existing codebase already has the sequential compilation loop in `builder.go` (lines 144-213) which compiles sources one-by-one; this needs to be parallelized while maintaining output grouping and proper error handling.

The standard approach uses `golang.org/x/sync/errgroup` for parallel execution with automatic error propagation and context cancellation. Output buffering prevents interleaving (Ninja-style: buffer per file, print complete results), while signal handling provides graceful shutdown (SIGTERM first, SIGKILL fallback) with double Ctrl+C for immediate exit.

**Primary recommendation:** Use errgroup.Group with SetLimit() for bounded concurrency, buffer all compiler output per file, and use signal.NotifyContext() for graceful cancellation with process group cleanup.

## Standard Stack

The established libraries/tools for parallel execution in Go build systems:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sync/errgroup` | v0.17.0 (already in deps) | Parallel execution with error propagation | Official extended library, handles context cancellation and first-error-wins semantics |
| `golang.org/x/sync/semaphore` | v0.17.0 (already in deps) | Bounded concurrency control | Official extended library for limiting concurrent operations |
| `os/signal` + `signal.NotifyContext` | stdlib (Go 1.16+) | Signal handling | Standard way to handle Ctrl+C since Go 1.16 |
| `sync/atomic` | stdlib | Lock-free progress counters | Standard library, fast atomic operations for concurrent progress tracking |
| `context.Context` | stdlib | Cancellation propagation | Standard for propagating cancellation through goroutines |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os.File.IsTerminal()` | stdlib (Go 1.24+) | TTY detection | Detect if stdout is a terminal for progress bar vs line-by-line output |
| `github.com/mattn/go-isatty` | v0.0.20 | Cross-platform TTY detection | Fallback if targeting older Go versions (not needed for Go 1.24) |
| `bytes.Buffer` | stdlib | Output buffering | Buffer compiler output per file to prevent interleaving |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| errgroup | Manual WaitGroup + channels | errgroup provides context cancellation and error propagation automatically; manual approach requires more boilerplate |
| semaphore.Weighted | Buffered channel as semaphore | semaphore.Weighted supports context-aware acquisition; channels require select statements |
| signal.NotifyContext | os/signal.Notify + manual context | NotifyContext is simpler (one call) and automatically stops signal delivery |

**Installation:**
Dependencies already present in go.mod (`golang.org/x/sync v0.17.0`). No additional packages needed.

## Architecture Patterns

### Recommended Project Structure
```
internal/build/
├── builder.go          # Orchestration, now calls parallel compiler
├── executor.go         # Process execution with cancellation support
├── compiler.go         # Sequential compile (unchanged)
├── parallel.go         # NEW: Parallel compilation orchestrator
└── progress.go         # Enhanced with concurrent-safe progress tracking
```

### Pattern 1: Worker Pool with errgroup
**What:** Limited concurrency with automatic error propagation and cancellation
**When to use:** Default pattern for parallel compilation - combines bounded concurrency with fail-fast error handling
**Example:**
```go
// Source: golang.org/x/sync/errgroup package documentation
g, ctx := errgroup.WithContext(parentCtx)
g.SetLimit(runtime.NumCPU() / 2) // Bounded concurrency

for _, source := range sources {
    source := source // Capture for closure
    g.Go(func() error {
        return compileFile(ctx, source)
    })
}

if err := g.Wait(); err != nil {
    return err // First error cancels all in-flight work
}
```

### Pattern 2: Buffered Output Collection
**What:** Capture each goroutine's output separately, print complete chunks to avoid interleaving
**When to use:** When parallelizing commands that produce stdout/stderr (compilers, linkers)
**Example:**
```go
// Inspired by Ninja build system behavior
type CompileResult struct {
    Source string
    Output bytes.Buffer // Buffered stdout/stderr
    Error  error
}

// In worker goroutine:
var buf bytes.Buffer
cmd := exec.CommandContext(ctx, compiler, args...)
cmd.Stdout = &buf
cmd.Stderr = &buf
err := cmd.Run()

// Later, print complete results atomically
fmt.Fprint(os.Stdout, result.Output.String())
```

### Pattern 3: Graceful Shutdown with Double Ctrl+C
**What:** First Ctrl+C triggers graceful cancellation, second Ctrl+C exits immediately
**When to use:** Always, for build systems and long-running CLI tools
**Example:**
```go
// Source: Multiple Go blog posts and VictoriaMetrics graceful shutdown guide
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

// Goroutine to handle double Ctrl+C
go func() {
    <-ctx.Done() // First signal
    stop()       // Stop signal delivery

    // Wait for second signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    fmt.Println("\nForce exit")
    os.Exit(1)
}()
```

### Pattern 4: Process Group Cleanup (Linux/Unix)
**What:** Kill child compiler processes when parent is cancelled
**When to use:** When spawning subprocesses that may run longer than parent context
**Example:**
```go
// Source: Multiple sources on Go process management
cmd := exec.CommandContext(ctx, compiler, args...)
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true, // Create new process group
}

// On cancellation, kill process group gracefully then forcefully
if cmd.Process != nil {
    pgid, _ := syscall.Getpgid(cmd.Process.Pid)
    syscall.Kill(-pgid, syscall.SIGTERM) // Negative PID = process group

    time.Sleep(100 * time.Millisecond) // Brief grace period

    syscall.Kill(-pgid, syscall.SIGKILL) // Force kill if still running
}
```

### Pattern 5: Atomic Progress Tracking
**What:** Use sync/atomic for lock-free progress counters accessible from multiple goroutines
**When to use:** When multiple workers need to update a shared counter (files compiled, files remaining)
**Example:**
```go
// Source: Go by Example: Atomic Counters
type Progress struct {
    completed atomic.Int64
    total     int
}

// In worker goroutine:
progress.completed.Add(1)

// In display goroutine:
current := progress.completed.Load()
fmt.Printf("[%d/%d]\n", current, progress.total)
```

### Pattern 6: TTY-Aware Output
**What:** Detect terminal vs pipe to choose between live progress updates (with \r) or line-by-line output
**When to use:** Always, to avoid mangling output when piped to files or other tools
**Example:**
```go
// Source: os package documentation (Go 1.24+)
isTTY := os.Stdout.IsTerminal()

if isTTY {
    // Use \r to overwrite current line
    fmt.Printf("\r[%d/%d] Compiling...", current, total)
} else {
    // Use \n for each update (pipe-friendly)
    fmt.Printf("[%d/%d] Compiling...\n", current, total)
}
```

### Anti-Patterns to Avoid
- **Unbounded goroutines:** Don't spawn goroutine per file without limit - will exhaust system resources on large projects. Use SetLimit() or semaphore.
- **Shared state without synchronization:** Don't update progress counters with regular ints. Use atomic operations or channels.
- **Streamed output in parallel:** Don't write directly to os.Stdout from multiple goroutines - output will interleave. Buffer per goroutine.
- **Ignoring context cancellation:** Always pass context to subprocesses and check ctx.Err() in loops. Respect cancellation signals.
- **SIGKILL without SIGTERM:** Don't immediately SIGKILL child processes. Send SIGTERM first, wait briefly, then SIGKILL if needed.
- **Process without process group:** If not using Setpgid, killing parent won't kill compiler child processes, leading to zombie processes.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parallel execution with error handling | Manual WaitGroup + error channels + context cancellation | `golang.org/x/sync/errgroup` | Handles first-error-wins, context cancellation, and cleanup automatically. Easy to get wrong (missed errors, goroutine leaks). |
| Bounded concurrency | Buffered channel as counting semaphore | `golang.org/x/sync/semaphore` or `errgroup.SetLimit()` | Semaphore supports context-aware acquisition (cancellable waits). SetLimit() is simpler if using errgroup. |
| Signal handling | Raw `signal.Notify()` + manual context | `signal.NotifyContext()` (Go 1.16+) | Automatically stops signal delivery, ties signal to context lifecycle. One function call vs multiple. |
| Progress tracking across goroutines | Mutex-protected counters | `sync/atomic` package | Lock-free, faster, simpler code. Mutex adds contention. |
| Process group management | Manual syscall setup and tracking | Standard pattern with `SysProcAttr{Setpgid: true}` | Easy to get PID/PGID math wrong. Standard pattern is battle-tested. |

**Key insight:** Go's extended sync libraries (errgroup, semaphore) solve 80% of parallel build problems. Don't rebuild these primitives. The complexity is in cancellation edge cases and race conditions.

## Common Pitfalls

### Pitfall 1: Output Interleaving
**What goes wrong:** Multiple goroutines write to stdout simultaneously, creating garbled output: "Compi[3/20ling] foo.[4/cpp20] bar.cpp"
**Why it happens:** os.Stdout is not goroutine-safe for formatting operations. Multiple Printf calls can interleave mid-format.
**How to avoid:** Buffer all output per compilation unit. Only print complete chunks. Alternatively, serialize all output through a single goroutine via channel.
**Warning signs:** Output looks corrupted, progress indicators appear in middle of compiler errors, line breaks in wrong places.

### Pitfall 2: Context Cancelled But Process Still Running
**What goes wrong:** User presses Ctrl+C, program exits, but compiler processes continue running in background
**Why it happens:** `exec.CommandContext` kills process but not child processes. Without process group, only parent is terminated.
**How to avoid:** Always set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and kill the process group (negative PID).
**Warning signs:** After Ctrl+C, `ps aux | grep clang` shows compiler processes still running.

### Pitfall 3: Deadlock with Buffered Channels
**What goes wrong:** Program hangs during parallel compilation
**Why it happens:** If using channels for work distribution, wrong buffer size can cause deadlock (producer blocks, consumer waiting for other condition).
**How to avoid:** Use errgroup instead of manual channels. If using channels, ensure producer doesn't block indefinitely (use select with context).
**Warning signs:** Build hangs with no progress, no CPU activity, goroutines stuck in channel send/receive (goroutine dump shows channel operations).

### Pitfall 4: Race on Progress Stats
**What goes wrong:** Progress counter occasionally shows wrong numbers or doesn't increment
**Why it happens:** Non-atomic operations on shared counter. Increment (`count++`) is read-modify-write, not atomic.
**How to avoid:** Use `sync/atomic` package: `atomic.AddInt64()` for increments, `atomic.LoadInt64()` for reads. Run with `go test -race` to detect.
**Warning signs:** `go test -race` reports data races on progress variables. Progress occasionally stuck or jumps values.

### Pitfall 5: Keep-Going Flag Implementation
**What goes wrong:** `--keep-going` continues after errors, but some files that could have compiled are skipped
**Why it happens:** errgroup cancels all goroutines on first error. Need different error accumulation strategy for keep-going mode.
**How to avoid:** When keep-going is enabled, don't use errgroup's auto-cancel. Use semaphore directly + manual error collection. Continue submitting work even after errors.
**Warning signs:** `--keep-going` flag stops build on first error like normal mode. Not all independent files are attempted.

### Pitfall 6: Manifest Corruption Under Parallel Writes
**What goes wrong:** Build cache manifest (manifest.json) becomes corrupted or loses entries
**Why it happens:** Multiple goroutines writing to cache manager manifest simultaneously. Even with atomic file writes, manifest is in-memory map that's not thread-safe.
**How to avoid:** Serialize cache writes through channel or mutex. Or collect all results, write manifest once at end (current code does this - good!).
**Warning signs:** Manifest file has JSON errors, cache entries mysteriously disappear, builds fail with "invalid JSON" errors.

### Pitfall 7: SIGTERM Timeout Too Short
**What goes wrong:** Compiler processes get SIGKILL before they can flush output or clean up temp files
**Why it happens:** SIGTERM → SIGKILL timeout too short (e.g., 10ms). Compilers need time to clean up.
**How to avoid:** Use 100-500ms timeout between SIGTERM and SIGKILL. This is "Claude's Discretion" per context - recommend 100ms as good default.
**Warning signs:** After Ctrl+C, temp files left in `/tmp`, partial object files with weird state, compiler warnings about interrupted writes.

## Code Examples

Verified patterns from official sources:

### Parallel Compilation Loop
```go
// Combines errgroup, bounded concurrency, and buffering
// Based on: golang.org/x/sync/errgroup examples

func (b *Builder) CompileParallel(ctx context.Context, sources []CompileOptions, jobs int) error {
    g, ctx := errgroup.WithContext(ctx)

    // Bounded concurrency
    g.SetLimit(jobs)

    // Channel to collect results in order
    results := make(chan CompileResult, len(sources))

    // Launch workers
    for _, opts := range sources {
        opts := opts // Capture
        g.Go(func() error {
            result := b.compileWithBuffering(ctx, opts)
            results <- result
            if result.Error != nil && !keepGoing {
                return result.Error // Fail fast
            }
            return nil
        })
    }

    // Wait for completion
    err := g.Wait()
    close(results)

    // Print all results in order
    for result := range results {
        fmt.Print(result.Output.String())
    }

    return err
}
```

### Buffered Compilation
```go
// Prevent output interleaving
func (b *Builder) compileWithBuffering(ctx context.Context, opts CompileOptions) CompileResult {
    var buf bytes.Buffer

    // Modified executor config to capture output
    tempExecutor := &Executor{
        config: ExecutorConfig{
            Verbose:      false,
            StreamOutput: false, // Capture, don't stream
        },
    }

    // Progress message goes to buffer
    fmt.Fprintf(&buf, "[%d/%d] Compiling: %s\n",
        atomic.AddInt64(&progress.current, 1),
        progress.total,
        filepath.Base(opts.Source))

    // Run compiler, capture stdout/stderr to buffer
    cmd := exec.CommandContext(ctx, compiler, args...)
    cmd.Stdout = &buf
    cmd.Stderr = &buf
    err := cmd.Run()

    return CompileResult{
        Source: opts.Source,
        Output: buf,
        Error:  err,
    }
}
```

### Signal Handling with Double Ctrl+C
```go
// Source: VictoriaMetrics blog, Mat Ryer's context patterns
func setupSignalHandling() context.Context {
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)

    // Handle double Ctrl+C for immediate exit
    go func() {
        <-ctx.Done()     // First signal received
        stop()            // Stop signal notifications

        // Setup for second signal
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

        select {
        case <-sigChan:
            fmt.Fprintf(os.Stderr, "\nForce exit\n")
            os.Exit(130) // 128 + SIGINT(2)
        case <-time.After(5 * time.Second):
            // Timeout waiting for graceful shutdown
            return
        }
    }()

    return ctx
}
```

### Process Group Cleanup
```go
// Source: Go process management guides
func (e *Executor) RunWithCleanup(ctx context.Context, name string, args []string) error {
    cmd := exec.CommandContext(ctx, name, args...)

    // Create process group
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
        Pgid:    0, // New process group
    }

    // Start process
    if err := cmd.Start(); err != nil {
        return err
    }

    // Wait with cleanup on cancellation
    done := make(chan error, 1)
    go func() {
        done <- cmd.Wait()
    }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        // Context cancelled, cleanup process group
        if cmd.Process != nil {
            pgid, _ := syscall.Getpgid(cmd.Process.Pid)

            // Graceful termination
            syscall.Kill(-pgid, syscall.SIGTERM)

            // Wait briefly
            select {
            case <-done:
                return ctx.Err()
            case <-time.After(100 * time.Millisecond):
                // Force kill
                syscall.Kill(-pgid, syscall.SIGKILL)
            }
        }
        return ctx.Err()
    }
}
```

### TTY Detection for Progress Output
```go
// Source: os package documentation
func (p *Progress) updateProgress(current, total int, activeFiles []string) {
    isTTY := os.Stdout.IsTerminal()

    if isTTY {
        // Overwrite current line (terminal only)
        fmt.Printf("\r\033[K[%d/%d] Compiling: %s",
            current, total, strings.Join(activeFiles, ", "))
    } else {
        // New line each time (pipe/file friendly)
        fmt.Printf("[%d/%d] Compiling: %s\n",
            current, total, strings.Join(activeFiles, ", "))
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| os/signal.Notify + manual context | signal.NotifyContext | Go 1.16 (2021) | Simplified signal handling, automatic cleanup |
| Manual WaitGroup + error channels | errgroup.WithContext | Always available (x/sync) | Automatic error propagation and cancellation |
| Mutex-protected counters | sync/atomic | Always available | Lock-free, better performance |
| github.com/mattn/go-isatty | os.File.IsTerminal() | Go 1.24 (2025) | No external dependency for TTY detection |

**Deprecated/outdated:**
- **golang.org/x/crypto/ssh/terminal.IsTerminal**: Moved to os.File.IsTerminal() in Go 1.24. Old location still works but use new one.
- **Manual buffering with time.Sleep polling**: Ninja popularized buffered output approach. Now standard pattern for parallel build tools.
- **GOMAXPROCS for concurrency limit**: Modern tools use explicit -j flag like Make. GOMAXPROCS is for Go runtime, not user control.

## Open Questions

Things that couldn't be fully resolved:

1. **Active files display during compilation**
   - What we know: Context says show "[3/20] Compiling: foo.cpp, bar.cpp, baz.cpp" with list of active files
   - What's unclear: How frequently to update this display (every completion? timer-based?), maximum number of files to show before truncation
   - Recommendation: Update on TTY every 100ms with timer. Show up to 3-5 files, rest as "... and N more". Non-TTY: print start/end events only.

2. **Error summary limiting**
   - What we know: Context says "show first few, summarize rest" with "and N more"
   - What's unclear: Exact threshold (3 errors? 5? 10?)
   - Recommendation: Show first 5 errors with full details, then "...and N more errors" summary. This matches typical terminal height and user attention span.

3. **Optimal wait time between SIGTERM and SIGKILL**
   - What we know: Context says "brief wait" for graceful shutdown, this is "Claude's Discretion"
   - What's unclear: Exact duration for C++ compilers
   - Recommendation: 100ms is sufficient for compilers to flush buffers. Longer (500ms+) is for servers with connections. Shorter (<50ms) risks incomplete cleanup.

4. **Keep-going mode interaction with dependencies**
   - What we know: --keep-going continues despite errors, files are compiled in dependency order
   - What's unclear: If library compilation fails, do we skip executables that depend on it, or try anyway?
   - Recommendation: Skip dependents of failed builds (can't link without library). Only continue independent branches.

## Sources

### Primary (HIGH confidence)
- [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) - Official documentation for parallel task execution with error handling
- [golang.org/x/sync/semaphore](https://pkg.go.dev/golang.org/x/sync/semaphore) - Official documentation for bounded concurrency control
- [Go by Example: Atomic Counters](https://gobyexample.com/atomic-counters) - Official examples for lock-free counter operations
- [os/signal package](https://pkg.go.dev/os/signal) - Standard library signal handling documentation
- [VictoriaMetrics: Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/) - Comprehensive guide to graceful shutdown patterns
- [How to Use errgroup for Parallel Operations in Go (2026)](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) - Recent detailed errgroup tutorial
- [How to Implement Graceful Shutdown in Go for Kubernetes (2026)](https://oneuptime.com/blog/post/2026-01-07-go-graceful-shutdown-kubernetes/view) - Production graceful shutdown patterns
- [How to Use Goroutines and Channels for Concurrent Processing (2026)](https://oneuptime.com/blog/post/2026-01-07-go-goroutines-channels-concurrency/view) - Recent concurrency patterns overview

### Secondary (MEDIUM confidence)
- [The Ninja build system](https://ninja-build.org/manual.html) - Build parallelism and output buffering patterns
- [Go Build System Optimized for Humans and Machines (2026)](https://blog.gaborkoos.com/posts/2026-01-08-The-Go-Build-System-Optimised-for-Humans-and-Machines/) - Modern Go build best practices
- [Efficient Concurrency in Go: Worker Pool Pattern](https://rksurwase.medium.com/efficient-concurrency-in-go-a-deep-dive-into-the-worker-pool-pattern-for-batch-processing-73cac5a5bdca) - Worker pool implementation details
- [Mastering Go Atomic Operations (2026)](https://jsschools.com/golang/mastering-go-atomic-operations-build-high-perform/) - Atomic operations guide
- [Killing a process and all descendants in Go](https://sigmoid.at/post/2023/08/kill_process_descendants_golang/) - Process group management patterns
- [signal.NotifyContext: handling cancelation with Unix signals](https://henvic.dev/posts/signal-notify-context/) - Deep dive into signal.NotifyContext

### Tertiary (LOW confidence - implementation details, not critical decisions)
- [Various Medium articles on errgroup patterns] - Code examples validated against official docs
- [Various GitHub code examples for process cleanup] - Pattern validation
- [Build system parallelism discussions] - Background on Make/Ninja conventions

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - golang.org/x/sync is official extended library, already in dependencies, well-documented
- Architecture: HIGH - errgroup pattern is standard, verified with official docs and recent 2026 tutorials
- Pitfalls: MEDIUM-HIGH - Based on common Go concurrency pitfalls (races, deadlocks, output interleaving) which are well-documented, plus build-system-specific issues from Make/Ninja experience

**Research date:** 2026-01-23
**Valid until:** 2026-04-23 (90 days - Go standard library and x/sync are stable, patterns are well-established)
