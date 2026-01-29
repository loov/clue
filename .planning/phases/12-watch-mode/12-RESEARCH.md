# Phase 12: Watch Mode - Research

**Researched:** 2026-01-29
**Domain:** File system watching and incremental rebuilds in Go
**Confidence:** HIGH

## Summary

Watch mode for build systems requires three key components: file system event monitoring, event debouncing to handle rapid changes, and incremental rebuild triggering. The standard approach in Go uses fsnotify for cross-platform filesystem notifications, timer-based debouncing to batch rapid events, and the existing context cancellation infrastructure for interrupting in-progress builds.

The research confirms that fsnotify (github.com/fsnotify/fsnotify) is the de-facto standard for file watching in Go, with comprehensive platform support (Linux/inotify, macOS/kqueue, Windows/ReadDirectoryChangesW). Event debouncing is typically implemented using Go's time.Timer with a 300-500ms window, matching the phase requirements. Build cancellation leverages existing context.Context patterns that clue already uses for signal handling.

**Primary recommendation:** Use fsnotify v1.7+ with manual debouncing via time.Timer, watch parent directories (not individual files), filter events by extension and path, and integrate with existing Builder.Build() context cancellation for interrupt-and-restart behavior.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/fsnotify/fsnotify | v1.7+ | Cross-platform filesystem notifications | Industry standard, mature (10+ years), supports all target platforms (Linux/macOS/Windows), 9k+ GitHub stars |
| time.Timer (stdlib) | Go 1.25 | Debouncing rapid events | Built-in, no dependencies, proven pattern for event batching |
| context.Context (stdlib) | Go 1.25 | Build cancellation and restart | Already used in clue for signal handling, standard cancellation pattern |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| path/filepath (stdlib) | Go 1.25 | Path matching and filtering | Filter events to relevant source files |
| os/signal (stdlib) | Go 1.25 | Ctrl+C handling | Already integrated in internal/build/signal.go |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| fsnotify | github.com/Q1-Energie-AG/fswatch | fswatch adds debouncing wrapper but less mature (66 stars vs 9k+), fsnotify + manual debounce is more transparent |
| Manual debounce | State machine debouncer | State machine approach still in development per fsnotify roadmap, timer-based is simpler and proven |
| Recursive watching | kqueue/inotify directly | fsnotify abstracts platform differences, manual implementation would require per-OS code paths |

**Installation:**
```bash
go get github.com/fsnotify/fsnotify@latest
```

Already in clue if needed (check go.mod). If not present, add with `go get`.

## Architecture Patterns

### Recommended Project Structure
```
internal/build/
├── watcher.go           # Watch mode orchestration
├── watcher_test.go      # Unit tests for debouncing logic
├── builder.go           # Existing - Build() method already context-aware
└── signal.go            # Existing - Ctrl+C handling pattern to reuse
```

### Pattern 1: Watch Loop with Debounced Rebuilds
**What:** Main event loop that receives fsnotify events, debounces them, and triggers rebuilds
**When to use:** Core watch mode implementation
**Example:**
```go
// Simplified pattern based on fsnotify documentation
watcher, err := fsnotify.NewWatcher()
defer watcher.Close()

// Watch source directories (not individual files)
for _, dir := range sourceDirs {
    watcher.Add(dir)
}

// Debounce timer
var debounceTimer *time.Timer
debounceDuration := 300 * time.Millisecond

for {
    select {
    case event := <-watcher.Events:
        // Filter to relevant extensions (.c, .cpp, .h, .hpp, .cue)
        if !isRelevantFile(event.Name) {
            continue
        }

        // Reset debounce timer
        if debounceTimer != nil {
            debounceTimer.Stop()
        }
        debounceTimer = time.AfterFunc(debounceDuration, func() {
            triggerRebuild()
        })

    case err := <-watcher.Errors:
        log.Println("watcher error:", err)
    }
}
```

### Pattern 2: Cancel-and-Restart Build Pattern
**What:** If file changes occur during active build, cancel current build and start fresh
**When to use:** Prevents wasted work on stale builds
**Example:**
```go
// Reuse existing pattern from internal/build/signal.go
var currentBuildCancel context.CancelFunc

func triggerRebuild() {
    // Cancel any in-progress build
    if currentBuildCancel != nil {
        currentBuildCancel()
    }

    // Create new cancellable context for this build
    ctx, cancel := context.WithCancel(context.Background())
    currentBuildCancel = cancel

    // Build (existing Builder.Build is already context-aware)
    builder.Build(ctx, opts)
}
```

### Pattern 3: Watch Directory, Filter by Name
**What:** Watch parent directories and filter events by filename, not individual file watches
**When to use:** Always - fsnotify best practice due to editor atomic save behavior
**Example:**
```go
// Source: https://pkg.go.dev/github.com/fsnotify/fsnotify
// Watch parent directory
watcher.Add("/path/to/src")

// Filter in event handler
if event.Has(fsnotify.Write) && filepath.Base(event.Name) == "target.cpp" {
    // Handle specific file
}
```

### Pattern 4: Terminal Screen Clearing
**What:** Clear screen before each rebuild for fresh output
**When to use:** Per phase requirement for clean visual feedback
**Example:**
```go
// ANSI escape codes work on Linux, macOS, and modern Windows (10+)
func clearScreen() {
    fmt.Print("\033[H\033[2J") // Home cursor + clear screen
}
```

### Anti-Patterns to Avoid
- **Watching individual files:** Editors use atomic saves (write temp, rename), breaking file watches. Watch directories instead.
- **Processing Chmod events:** Filesystem indexers (Spotlight, Windows Search) trigger excessive Chmod events. Filter them out.
- **No debouncing:** Single user save can trigger 5+ events (VSCode example from research). Always debounce.
- **Blocking on rebuild:** Run rebuilds in goroutines to keep event loop responsive during compilation.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform file watching | Custom inotify/kqueue/ReadDirectoryChangesW | fsnotify | Handles platform differences, edge cases (buffer overflows, watch limits), tested across systems |
| Recursive directory watching | Walk filesystem + watch all | Manual Add() per directory | fsnotify is non-recursive by design for resource control; build systems need explicit source paths anyway |
| Debouncing state machine | Complex event deduplication logic | time.Timer + counter | Timer approach is proven, simpler, and sufficient for 300-500ms debounce window |
| Config file change detection | Custom file hashing | fsnotify events on .cue files | Filesystem events are immediate, hashing requires polling |

**Key insight:** File watching has many platform-specific edge cases (watch limits, buffer sizes, symlinks, network filesystems). fsnotify has handled these for 10+ years across thousands of projects. Don't reimplement.

## Common Pitfalls

### Pitfall 1: Watch Limit Exhaustion (Linux)
**What goes wrong:** On Linux, hitting `fs.inotify.max_user_watches` limit causes "no space left on device" errors
**Why it happens:** Each watched directory consumes one inotify watch. Large codebases can exceed default limit (typically 8192-524288 depending on distro)
**How to avoid:**
- Only watch source directories explicitly listed in build.cue (don't auto-discover)
- Document system limit in error messages
- Watch parent directories rather than individual files (reduces watch count)
**Warning signs:** Error message containing "too many open files" or "no space left on device" when adding watches

### Pitfall 2: Editor Atomic Saves Breaking Watches
**What goes wrong:** Editors write to temp file then rename/move, causing watch on original file to be lost
**Why it happens:** After rename, fsnotify automatically removes the watch (documented behavior)
**How to avoid:** Watch parent directories and filter by basename (Pattern 3 above)
**Warning signs:** First save triggers rebuild, subsequent saves don't

### Pitfall 3: Infinite Rebuild Loops
**What goes wrong:** Build process writes to watched directory, triggering rebuild, which writes again, etc.
**Why it happens:** Build outputs (.o files, binaries) in source tree get watched too
**How to avoid:**
- Filter events to only source extensions (.c, .cpp, .h, .hpp, .cue)
- Don't watch .build/ directory
- Use explicit path filtering in event handler
**Warning signs:** Rapid continuous rebuilds without file edits

### Pitfall 4: No Debouncing on Config Changes
**What goes wrong:** Editing build.cue triggers rebuild for every keystroke if watching config file
**Why it happens:** Config files get Write events during editing just like source files
**How to avoid:** Use same debounce logic for config changes as source changes
**Warning signs:** Multiple rebuilds while editing single config file

### Pitfall 5: Race Between Event and File State
**What goes wrong:** Receiving Write event but file content not fully flushed to disk yet
**Why it happens:** Filesystem events are asynchronous, fsync may not have completed
**How to avoid:**
- Debounce window (300-500ms) naturally handles this by waiting
- Builder already reads files with proper error handling
**Warning signs:** Intermittent "file not found" or "permission denied" during rebuilds

## Code Examples

Verified patterns from official sources:

### Creating and Using Watcher
```go
// Source: https://pkg.go.dev/github.com/fsnotify/fsnotify
import "github.com/fsnotify/fsnotify"

watcher, err := fsnotify.NewWatcher()
if err != nil {
    log.Fatal(err)
}
defer watcher.Close()

// Add paths
err = watcher.Add("/path/to/watch")
if err != nil {
    log.Fatal(err)
}
```

### Event Handling with Select
```go
// Source: https://pkg.go.dev/github.com/fsnotify/fsnotify
go func() {
    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            log.Println("event:", event)
            if event.Has(fsnotify.Write) {
                log.Println("modified file:", event.Name)
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            log.Println("error:", err)
        }
    }
}()
```

### Filtering Events
```go
// Source: https://pkg.go.dev/github.com/fsnotify/fsnotify (best practices)
// Ignore Chmod events (recommended)
if event.Op&fsnotify.Chmod == 0 {
    // Process event (skips Chmod)
}

// Check specific operations
if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
    // Handle write/create
}
```

### Debouncing with Timer
```go
// Pattern from wgo (https://github.com/bokwoon95/wgo)
// Default 300ms debounce
var debounceTimer *time.Timer
const debounceDuration = 300 * time.Millisecond

// In event handler
if debounceTimer != nil {
    debounceTimer.Stop()
}
debounceTimer = time.AfterFunc(debounceDuration, func() {
    // Trigger action after quiet period
    performRebuild()
})
```

### Context Cancellation for Interrupts
```go
// Source: https://pkg.go.dev/context (standard pattern)
// Reuse existing signal handling pattern from internal/build/signal.go
ctx, cancel := context.WithCancel(context.Background())

// Cancel on new event
go func() {
    <-newEventChan
    cancel() // Stops in-progress build
}()

// Pass to Builder.Build (already context-aware)
builder.Build(ctx, opts)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| fsnotify v1.4 string-based Op | fsnotify v1.5+ Has() method | 2021 | Cleaner event checking: `event.Has(fsnotify.Write)` vs bitwise ops |
| Polling filesystem | Event-driven (inotify/kqueue) | ~2014 (fsnotify matured) | Instant notifications vs 1s+ latency |
| Manual per-OS code | fsnotify abstraction | 2014 (fsnotify v1.0) | Cross-platform with single API |
| signal.Notify | signal.NotifyContext | Go 1.16 (2021) | Built-in context integration for signals |

**Deprecated/outdated:**
- gopkg.in/fsnotify.v1: Use github.com/fsnotify/fsnotify (canonical import path since 2019)
- Manual signal channel handling: Use signal.NotifyContext (available since Go 1.16)

## Open Questions

Things that couldn't be fully resolved:

1. **Header dependency triggering**
   - What we know: clue tracks header dependencies in .d files (see cache_manager.go ParseDepFile)
   - What's unclear: How to efficiently determine "affected compilation units" when a header changes without parsing all .d files
   - Recommendation: On header change, trigger full rebuild initially (simple, correct). Optimize later if performance issue arises.

2. **Config reload granularity**
   - What we know: build.cue changes should trigger full rebuild per phase requirements
   - What's unclear: Should we reload and re-validate entire config, or just trigger rebuild with existing config?
   - Recommendation: Full reload (call loadConfig again) to catch validation errors and schema changes immediately.

3. **Verbosity level in watch mode**
   - What we know: clue has VerbosityQuiet/Normal/Verbose modes
   - What's unclear: Should watch mode have different default verbosity than regular builds (less noisy on incremental)?
   - Recommendation: Respect user's verbosity flag, but default to Normal (not Quiet) so users see rebuild confirmations.

## Sources

### Primary (HIGH confidence)
- https://pkg.go.dev/github.com/fsnotify/fsnotify - Official API documentation
- https://github.com/fsnotify/fsnotify - Official README and best practices
- https://pkg.go.dev/context - Go standard library context documentation
- https://go.dev/blog/context - Official Go blog on context patterns
- Internal codebase: /workspace/internal/build/signal.go (existing cancellation patterns)
- Internal codebase: /workspace/internal/build/builder.go (context-aware Build method)
- Internal codebase: /workspace/internal/build/cache_manager.go (header dependency tracking)

### Secondary (MEDIUM confidence)
- https://github.com/bokwoon95/wgo - Live reload tool with 300ms debounce pattern
- https://medium.com/@matryer/make-ctrl-c-cancel-the-context-context-bd006a8ad6bf - Context cancellation with Ctrl+C
- https://github.com/Q1-Energie-AG/fswatch - Debounce wrapper patterns
- https://dev.to/asoseil/building-a-cross-platform-file-watcher-in-go-what-i-learned-from-scratch-1dbj - File watcher implementation lessons

### Tertiary (LOW confidence)
- https://medium.com/@adamszpilewicz/efficient-file-system-monitoring-in-go-with-fsnotify-and-push-mechanism-9fa59342d6a4 - Monitoring patterns
- Various search results on ANSI terminal codes (cross-platform clearing)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - fsnotify is industry standard with 10+ years maturity, used by thousands of projects
- Architecture: HIGH - Patterns verified from official docs and existing clue codebase (signal.go, builder.go already context-aware)
- Pitfalls: HIGH - Documented in fsnotify official docs and real-world usage examples
- Debounce timing: MEDIUM - 300-500ms is common pattern (wgo uses 300ms default) but not formally standardized

**Research date:** 2026-01-29
**Valid until:** ~60 days (2026-03-30) - fsnotify is stable, Go stdlib patterns unlikely to change
