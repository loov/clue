# Phase 11: Build Profiling - Research

**Researched:** 2026-01-29
**Domain:** Build system profiling and timing data collection
**Confidence:** HIGH

## Summary

Build profiling for this phase involves collecting per-file compilation timing, aggregating build statistics, and persisting data in Chrome Trace JSON format for visualization. The existing codebase already captures `Duration` in `CommandResult` and `ParallelResult`, so the infrastructure for timing collection is largely in place. The primary work involves:

1. Aggregating timing data from existing compilation results
2. Formatting output with adaptive precision (seconds vs milliseconds)
3. Generating Chrome Trace JSON format for chrome://tracing visualization
4. Adding CLI flags and config options for opt-in profiling

The Chrome Trace Event Format is a well-documented JSON format that has been stable since 2016. Go's standard `encoding/json` package is sufficient for generating the output. No external libraries are required.

**Primary recommendation:** Extend existing `ParallelResult` and `CommandResult` timing data with a new `Profiler` type that aggregates results and can export to Chrome Trace JSON format.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `time` | 1.22+ | Duration tracking, formatting | Built-in, zero dependencies |
| Go stdlib `encoding/json` | 1.22+ | Chrome Trace JSON output | Standard library, well-tested |
| Go stdlib `sync` | 1.22+ | Thread-safe timing collection | Standard library |
| Go stdlib `sort` | 1.22+ | Sorting slowest files | Standard library |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Go stdlib `os` | 1.22+ | File writing | Profile persistence |
| Go stdlib `path/filepath` | 1.22+ | Output path handling | Profile file location |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib json | github.com/goccy/go-json | Faster but adds dependency, unnecessary for small output |
| Custom format | Chrome Trace JSON | Chrome Trace is standard, opens in Perfetto/chrome://tracing |
| Internal tracing | runtime/trace | Different purpose (Go runtime), not build profiling |

**No external dependencies needed.** Go stdlib provides everything required.

## Architecture Patterns

### Recommended Project Structure
```
internal/build/
├── profiler.go         # Profiler type, timing collection, aggregation
├── profiler_test.go    # Unit tests for profiler
├── chrome_trace.go     # Chrome Trace JSON format export
├── chrome_trace_test.go
├── parallel.go         # Already has Duration in ParallelResult
├── executor.go         # Already has Duration in CommandResult
└── builder.go          # Integration point, passes profiler to compilation
```

### Pattern 1: Centralized Timing Collector
**What:** A `Profiler` struct that receives timing events from parallel compilation
**When to use:** Always when profiling is enabled
**Example:**
```go
// Source: Derived from existing ParallelResult pattern in parallel.go
type Profiler struct {
    mu        sync.Mutex
    enabled   bool
    startTime time.Time
    events    []CompileEvent
}

type CompileEvent struct {
    Source    string
    StartTime time.Time
    Duration  time.Duration
    ThreadID  int  // Worker goroutine ID for Chrome Trace
}

func (p *Profiler) RecordCompilation(source string, start time.Time, duration time.Duration, threadID int) {
    if !p.enabled {
        return
    }
    p.mu.Lock()
    defer p.mu.Unlock()
    p.events = append(p.events, CompileEvent{
        Source:    source,
        StartTime: start,
        Duration:  duration,
        ThreadID:  threadID,
    })
}
```

### Pattern 2: Adaptive Duration Formatting
**What:** Format durations with appropriate precision based on magnitude
**When to use:** For user-facing timing output
**Example:**
```go
// Source: Derived from time.Duration.String() behavior
func formatDuration(d time.Duration) string {
    if d >= time.Second {
        // Show as seconds with one decimal: [2.3s]
        return fmt.Sprintf("[%.1fs]", d.Seconds())
    }
    // Show as milliseconds: [450ms]
    return fmt.Sprintf("[%dms]", d.Milliseconds())
}
```

### Pattern 3: Chrome Trace Export Structure
**What:** JSON structure matching Chrome Trace Event Format
**When to use:** For `--save-profile` output
**Example:**
```go
// Source: Chrome Trace Event Format specification
// https://chromium.googlesource.com/catapult/+/HEAD/docs/trace-event-format.md
type ChromeTrace struct {
    TraceEvents []ChromeEvent `json:"traceEvents"`
}

type ChromeEvent struct {
    Name      string                 `json:"name"`
    Category  string                 `json:"cat,omitempty"`
    Phase     string                 `json:"ph"`           // "X" for complete events
    Timestamp int64                  `json:"ts"`           // Microseconds
    Duration  int64                  `json:"dur"`          // Microseconds
    ProcessID int                    `json:"pid"`
    ThreadID  int                    `json:"tid"`
    Args      map[string]interface{} `json:"args,omitempty"`
}
```

### Anti-Patterns to Avoid
- **Global state for profiling:** Use dependency injection, pass profiler to builder
- **Always-on detailed profiling:** Per-file timing has overhead, must be opt-in
- **Microsecond timestamps as float64:** Use int64 to avoid precision loss
- **Blocking on profile writes:** Write profile at end of build, not during

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON serialization | Custom string building | `encoding/json` | Edge cases with escaping, unicode |
| Duration formatting | Custom formatting | `time.Duration` methods | Handles nanoseconds, edge cases |
| Thread-safe collection | Channels without mutex | `sync.Mutex` with slice | Simpler, ParallelCompiler already uses this pattern |
| Sorting by duration | Custom sort | `sort.Slice` with closure | Standard library, efficient |

**Key insight:** The existing codebase already has the timing data collection pattern established in `parallel.go` and `executor.go`. The profiler should aggregate existing data, not re-collect it.

## Common Pitfalls

### Pitfall 1: Chrome Trace Timestamp Units
**What goes wrong:** Using milliseconds instead of microseconds for timestamps
**Why it happens:** The `dur` and `ts` fields are in microseconds, not milliseconds
**How to avoid:** Always multiply `Duration.Microseconds()` for `ts` and `dur` fields
**Warning signs:** Events appear as dots instead of bars in chrome://tracing

### Pitfall 2: Missing Args for Event Selection
**What goes wrong:** Events cannot be clicked/selected in chrome://tracing
**Why it happens:** Chrome UI requires non-empty `args` for event selection
**How to avoid:** Always include at least one arg: `"args": {"file": "foo.cpp"}`
**Warning signs:** Clicking events in UI does nothing

### Pitfall 3: Total Time vs Wall Time
**What goes wrong:** Sum of per-file times exceeds total build time
**Why it happens:** Parallel compilation means files compile concurrently
**How to avoid:** Track wall clock time separately from sum of durations
**Warning signs:** Percentages add up to more than 100%

### Pitfall 4: Profile File Write Failures Silently Ignored
**What goes wrong:** User thinks profile was saved but file is empty or missing
**Why it happens:** Ignoring write errors or not flushing before close
**How to avoid:** Check all write errors, explicitly close file with error check
**Warning signs:** Empty or truncated profile.json files

### Pitfall 5: Thread ID Confusion in Chrome Trace
**What goes wrong:** Overlapping events shown incorrectly in trace viewer
**Why it happens:** Using same thread ID for all events regardless of worker
**How to avoid:** Assign unique TID per goroutine worker in parallel compiler
**Warning signs:** Events overlap in strange ways in trace view

## Code Examples

Verified patterns from official sources:

### Chrome Trace Complete Event
```go
// Source: https://aras-p.info/blog/2017/01/23/Chrome-Tracing-as-Profiler-Frontend/
event := ChromeEvent{
    Name:      filepath.Base(source),
    Category:  "compile",
    Phase:     "X",  // Complete event
    Timestamp: startTime.Sub(buildStart).Microseconds(),
    Duration:  duration.Microseconds(),
    ProcessID: 1,
    ThreadID:  workerID,
    Args:      map[string]interface{}{"file": source},
}
```

### Writing Chrome Trace JSON
```go
// Source: Go standard library encoding/json
func (p *Profiler) WriteTrace(path string) error {
    f, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("failed to create profile: %w", err)
    }
    defer f.Close()

    trace := p.buildChromeTrace()
    encoder := json.NewEncoder(f)
    encoder.SetIndent("", "  ")  // Pretty print for readability
    if err := encoder.Encode(trace); err != nil {
        return fmt.Errorf("failed to write profile: %w", err)
    }
    return nil
}
```

### Slowest Files Summary
```go
// Source: Derived from existing Progress.Summary pattern
func (p *Profiler) PrintSlowestFiles(n int, totalWallTime time.Duration) {
    events := p.getSortedEvents()  // Sorted by duration descending
    if n > len(events) {
        n = len(events)
    }

    fmt.Printf("\nSlowest %d compilation units:\n", n)
    for i := 0; i < n; i++ {
        e := events[i]
        pct := float64(e.Duration) / float64(totalWallTime) * 100
        fmt.Printf("  %s  %s  (%.1f%%)\n",
            formatDuration(e.Duration),
            filepath.Base(e.Source),
            pct)
    }
}
```

### Activation Precedence
```go
// Source: Pattern from existing variant selector in config
func isProfilingEnabled(flagValue bool, envVar string, configValue bool) bool {
    // Flag > Env > Config precedence
    if flagValue {
        return true
    }
    if env := os.Getenv(envVar); env != "" {
        return env == "1" || strings.ToLower(env) == "true"
    }
    return configValue
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom profiler formats | Chrome Trace JSON | ~2017 | Portable, opens in any Chromium browser |
| Ninja .ninja_log parsing | Direct timing collection | Always | More accurate, no post-processing |
| External ClangBuildAnalyzer | Built-in summary | This phase | No additional tools needed |

**Deprecated/outdated:**
- Custom SVG flame graphs: Chrome Trace viewer has superior interactivity
- External ninjatracing tool: Only needed when not controlling the build system

## Open Questions

Things that couldn't be fully resolved:

1. **Goroutine to Thread ID mapping**
   - What we know: Chrome Trace requires unique tid per concurrent lane
   - What's unclear: Best way to assign stable IDs to goroutines in errgroup
   - Recommendation: Use worker index from parallel compiler (0 to jobs-1)

2. **Profile file atomicity**
   - What we know: Should write to temp file then rename for atomicity
   - What's unclear: Whether this complexity is needed for profile.json
   - Recommendation: Direct write is acceptable; profile isn't critical data

3. **Additional stats for verbose summary**
   - What we know: User context allows total wall time, CPU time, parallelism efficiency
   - What's unclear: Which stats are most useful
   - Recommendation: Include: total wall time, sum of compile times, parallelism ratio (sum/wall)

## Sources

### Primary (HIGH confidence)
- Chrome Trace Event Format specification: https://chromium.googlesource.com/catapult/+/HEAD/docs/trace-event-format.md
- Aras Pranckevicius blog on Chrome Tracing: https://aras-p.info/blog/2017/01/23/Chrome-Tracing-as-Profiler-Frontend/
- Go time package: https://pkg.go.dev/time
- Go encoding/json package: https://pkg.go.dev/encoding/json
- Existing codebase: `/workspace/internal/build/parallel.go`, `/workspace/internal/build/executor.go`

### Secondary (MEDIUM confidence)
- rai-project/tracer Go implementation: https://github.com/rai-project/tracer/blob/master/chrome/chrome.go
- Profilerpedia format reference: https://profilerpedia.markhansen.co.nz/formats/trace-event-format/

### Tertiary (LOW confidence)
- None required; domain is well-documented

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go stdlib is well-documented, Chrome Trace format is stable
- Architecture: HIGH - Extends existing patterns in codebase
- Pitfalls: HIGH - Verified against official Chrome Trace documentation

**Research date:** 2026-01-29
**Valid until:** 90 days (stable domain, no rapid changes expected)
