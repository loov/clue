# Phase 15: Supporting Extractions - Research

**Researched:** 2026-01-29
**Domain:** Go package refactoring and extraction patterns
**Confidence:** HIGH

## Summary

Phase 15 extracts three cohesive code modules from internal/build into dedicated packages: cache, profile, and watch. This is a pure refactoring operation with no new functionality. The research confirms this is a straightforward extraction following established Go package organization patterns.

The current codebase already has well-isolated code in internal/build:
- **Cache code**: 2 files (cache.go, cache_manager.go) with hashing and invalidation logic
- **Profile code**: 2 files (profiler.go, chrome_trace.go) with timing aggregation and Chrome Trace export
- **Watch code**: 1 file (watcher.go) with fsnotify integration and debouncing

Each module has clear responsibilities, minimal cross-dependencies, and comprehensive test coverage that can move with the code. The main challenge is managing the dependency direction to avoid circular imports while maintaining clean boundaries.

**Primary recommendation:** Extract packages in order (cache → profile → watch), move test files with implementation files, use concrete types (not interfaces), and let internal/build import the new packages.

## Standard Stack

The established libraries/tools for this domain:

### Core Dependencies (Already in Use)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/zeebo/xxh3 | v1.0.2 | Fast non-cryptographic hashing | Extremely fast hash algorithm for content-based caching, already integrated |
| github.com/fsnotify/fsnotify | v1.9.0 | Cross-platform filesystem notifications | De facto standard for file watching in Go, already integrated |
| golang.org/x/sync | v0.17.0 | Advanced concurrency primitives | Standard library extensions for synchronization, already available |

### Supporting Libraries (No New Dependencies)
| Library | Version | Purpose | When Used |
|---------|---------|---------|----------|
| encoding/json | stdlib | JSON marshaling | Cache manifest and Chrome Trace format |
| time | stdlib | Timing and debouncing | Profiler events and watcher debounce |
| sync | stdlib | Mutex/concurrency | Thread-safe access to shared state |

### No New Installation Required
All dependencies are already in go.mod. No additional libraries needed.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── cache/           # Hashing and cache invalidation
│   ├── cache.go           # Hashing functions (ComputeFileHash, ComputeCacheKey)
│   ├── manager.go         # CacheManager type
│   ├── cache_test.go      # Hash function tests
│   └── manager_test.go    # CacheManager tests
├── profile/         # Build timing and profiling
│   ├── profiler.go        # Profiler type, event recording
│   ├── chrome_trace.go    # Chrome Trace export
│   ├── profiler_test.go   # Profiler tests
│   └── chrome_trace_test.go  # Chrome Trace tests
├── watch/           # File change monitoring
│   ├── watcher.go         # Watcher type, fsnotify integration
│   └── watcher_test.go    # Watcher tests
└── build/           # Orchestrates cache, profile, watch
    ├── builder.go         # Creates and uses cache/profile/watch
    ├── compiler.go
    ├── linker.go
    └── ...
```

### Pattern 1: Leaf Package Extraction
**What:** Extract cohesive functionality into leaf packages that internal/build imports
**When to use:** When code has clear boundaries and no circular dependencies
**Structure:**
```
internal/build → internal/cache
internal/build → internal/profile
internal/build → internal/watch
```

**Why this works:**
- Build package remains the orchestrator
- New packages have no dependencies on build
- No circular import risk
- Clean dependency graph (DAG)

### Pattern 2: Move Tests With Implementation
**What:** Test files move to the same package as implementation
**When to use:** Always for unit tests of extracted code
**Example:**
```go
// Before: internal/build/cache_test.go
package build

// After: internal/cache/cache_test.go
package cache
```

**Benefits:**
- Tests verify extracted package works independently
- Test coverage stays with implementation
- No test import complexity

### Pattern 3: Concrete Types, Not Interfaces
**What:** Export concrete structs (Cache, Profiler, Watcher) not interfaces
**When to use:** For small, focused packages with clear responsibility
**Example:**
```go
// Good: Concrete type
type Profiler struct {
    mu      sync.Mutex
    enabled bool
    events  []CompileEvent
}

func NewProfiler(enabled bool) *Profiler { ... }

// Avoid: Interface abstraction (not needed for simple extraction)
type ProfileRecorder interface {
    RecordCompilation(...)
}
```

**Why concrete types:**
- Simpler API surface
- No interface tax for internal packages
- Can add methods without interface updates
- Go proverb: "Accept interfaces, return structs"

### Pattern 4: Shared Types in Common Package
**What:** If types are needed by multiple packages, create internal/meta or similar
**When to use:** When cache, profile, watch share file metadata types
**Example:**
```go
// internal/meta/types.go
package meta

type FileMetadata struct {
    Path  string
    Mtime int64
    Size  int64
}
```

**Note:** Based on current code inspection, shared types may not be needed immediately. Defer until actual need arises.

### Anti-Patterns to Avoid

- **Type aliases for backward compatibility:** Don't create `type CacheManager = cache.Manager` in internal/build. Force clean import migration.
- **Circular imports:** Never have cache import build or profile import build. Dependency direction must be acyclic.
- **Premature abstraction:** Don't create interfaces "for testability" if concrete types work fine.
- **God packages (util/common/helpers):** Don't create internal/util. Keep packages focused on specific domains.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File watching | Custom file polling | fsnotify.Watcher (already used) | Cross-platform, handles edge cases, battle-tested |
| Fast hashing | Custom hash function | github.com/zeebo/xxh3 (already used) | SIMD-optimized, consistent, non-cryptographic |
| Atomic file writes | os.WriteFile directly | Temp file + os.Rename pattern (already used) | Prevents partial writes, atomic on all platforms |
| JSON serialization | Custom format | encoding/json (stdlib) | Standard, debuggable, tooling support |
| Debouncing | Manual timer logic | time.AfterFunc + reset pattern (already used) | Correct timer cleanup, no resource leaks |

**Key insight:** All difficult problems are already solved in the current codebase. Extraction is code movement, not new implementation.

## Common Pitfalls

### Pitfall 1: Circular Import Dependencies
**What goes wrong:** Package A imports B, B imports A → compile error
**Why it happens:** Build needs cache types, cache needs build types
**How to avoid:**
- Establish clear dependency direction: build → cache (never cache → build)
- Move shared types to neutral package (internal/meta) if needed
- Use concrete types in method signatures (avoids interface shuffling)
**Warning signs:** "import cycle not allowed" compile error

### Pitfall 2: Incomplete Test Migration
**What goes wrong:** Tests stay in internal/build but test internal/cache types
**Why it happens:** Fear of breaking existing test structure
**How to avoid:**
- Move test files with implementation files
- Update imports in tests (build.CacheManager → cache.Manager)
- Run `go test ./...` after each extraction to verify
**Warning signs:** Tests importing multiple packages for single feature

### Pitfall 3: Breaking Backward Compatibility Unnecessarily
**What goes wrong:** Changing function signatures during extraction
**Why it happens:** Temptation to "improve" while refactoring
**How to avoid:**
- Keep function signatures identical
- Keep type names identical (CacheManager, Profiler, Watcher)
- Only change import paths (build.Profiler → profile.Profiler)
**Warning signs:** Main.go or other callers require changes beyond imports

### Pitfall 4: Forgetting go.mod Dependencies
**What goes wrong:** Extracted package uses library but doesn't have access
**Why it happens:** go.mod is at repo root, not per-package
**How to avoid:**
- Verify `go build ./internal/cache` works independently
- Check that test dependencies (testing, testdata) are accessible
- No action needed for this project (all deps already in go.mod)
**Warning signs:** "undefined: fsnotify" or similar errors

### Pitfall 5: Moving Too Much Code
**What goes wrong:** Extract helper functions that are only used by build package
**Why it happens:** Over-extracting for "completeness"
**How to avoid:**
- Only extract code directly related to cache/profile/watch
- Leave build-specific helpers in internal/build
- Example: Don't extract NormalizeFlags unless cache needs it (currently cache.go needs it, so extract)
**Warning signs:** Extracted package has many unexported functions

### Pitfall 6: Test File Package Confusion
**What goes wrong:** Test file has `package build_test` instead of `package build`
**Why it happens:** Attempting black-box testing during extraction
**How to avoid:**
- Use same package name as implementation (package cache, not cache_test)
- White-box testing is fine for internal packages
- Keep existing test package declarations when moving files
**Warning signs:** Tests can't access unexported fields/functions

## Code Examples

Verified patterns from the current codebase:

### Cache Package API (from internal/build/cache_manager.go)
```go
// Source: internal/build/cache_manager.go (lines 34-40, 43-63)
// After extraction: internal/cache/manager.go

package cache

import (
    "github.com/zeebo/xxh3"
    "github.com/loov/clue/internal/toolchain"
)

// Manager handles compilation caching for incremental builds
type Manager struct {
    cacheDir     string
    manifestPath string
    manifest     map[string]Entry
    verbosity    Verbosity  // May need to import from build or extract
}

// NewManager creates a cache manager for the given build directory
func NewManager(buildDir string, verbosity Verbosity) (*Manager, error) {
    cacheDir := filepath.Join(buildDir, "cache")
    if err := os.MkdirAll(cacheDir, 0o755); err != nil {
        return nil, fmt.Errorf("failed to create cache directory: %w", err)
    }

    cm := &Manager{
        cacheDir:     cacheDir,
        manifestPath: filepath.Join(cacheDir, "manifest.json"),
        manifest:     make(map[string]Entry),
        verbosity:    verbosity,
    }

    // Load existing manifest if present
    if err := cm.loadManifest(); err != nil {
        cm.manifest = make(map[string]Entry)
    }

    return cm, nil
}
```

### Profile Package API (from internal/build/profiler.go)
```go
// Source: internal/build/profiler.go (lines 20-35, 45-57)
// After extraction: internal/profile/profiler.go

package profile

import (
    "sync"
    "time"
)

// Profiler collects and aggregates build timing information
type Profiler struct {
    mu         sync.Mutex
    enabled    bool
    buildStart time.Time
    events     []CompileEvent
}

// NewProfiler creates a new Profiler instance
func NewProfiler(enabled bool) *Profiler {
    return &Profiler{
        enabled:    enabled,
        buildStart: time.Now(),
        events:     make([]CompileEvent, 0),
    }
}

// RecordCompilation records a compilation event
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

### Watch Package API (from internal/build/watcher.go)
```go
// Source: internal/build/watcher.go (lines 32-92)
// After extraction: internal/watch/watcher.go

package watch

import (
    "github.com/fsnotify/fsnotify"
    "time"
)

// Watcher monitors source directories for file changes and triggers
// rebuilds with debouncing to batch rapid consecutive changes.
type Watcher struct {
    watcher        *fsnotify.Watcher
    config         Config
    debounceTimer  *time.Timer
    mu             sync.Mutex
    done           chan struct{}
    pendingTrigger string
    isConfigChange bool
}

// Config holds configuration for the file watcher.
type Config struct {
    SourceDirs   []string
    BuildCuePath string
    DebounceDur  time.Duration
    OnRebuild    func(trigger string, isConfigChange bool)
}

// NewWatcher creates a new Watcher that monitors the specified directories.
func NewWatcher(cfg Config) (*Watcher, error) {
    // Apply defaults
    if cfg.DebounceDur == 0 {
        cfg.DebounceDur = DefaultDebounceDuration
    }

    // Create fsnotify watcher
    fsWatcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    w := &Watcher{
        watcher: fsWatcher,
        config:  cfg,
        done:    make(chan struct{}),
    }

    // Add directories to watch...
    return w, nil
}
```

### Calling Code Migration (from internal/build/builder.go)
```go
// Source: internal/build/builder.go (line 597)
// Before:
b.cacheManager, err = NewCacheManager(opts.BuildDir, opts.Verbosity)

// After extraction:
import "github.com/loov/clue/internal/cache"

b.cacheManager, err = cache.NewManager(opts.BuildDir, opts.Verbosity)
```

### Atomic File Write Pattern (from internal/build/cache_manager.go)
```go
// Source: internal/build/cache_manager.go (lines 86-101)
// This pattern should stay with cache package

func atomicWrite(path string, data []byte) error {
    dir := filepath.Dir(path)
    tempFile := filepath.Join(dir, fmt.Sprintf(".tmp.%d.%d", os.Getpid(), time.Now().UnixNano()))

    if err := os.WriteFile(tempFile, data, 0o644); err != nil {
        return err
    }

    if err := os.Rename(tempFile, path); err != nil {
        os.Remove(tempFile) // Clean up on failure
        return err
    }

    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| All code in internal/build | Extract to focused packages | Phase 15 (now) | Clearer responsibilities, better testability |
| Manual dep tracking | xxh3 content hashing | Phase 3 (implemented) | Fast, reliable cache invalidation |
| Polling for file changes | fsnotify event-based | Phase 12 (implemented) | Efficient, cross-platform watching |
| Custom profiling format | Chrome Trace JSON | Phase 11 (implemented) | Standard tooling, chrome://tracing support |

**Recent patterns (2024-2026):**
- **Shallow internal/ hierarchies** - Go community favors 1-2 level depth, not deep nesting
- **Concrete types over interfaces** - Interfaces at boundaries, structs internally
- **No util packages** - Descriptive names (cache, profile, watch) over generic names
- **Test colocation** - Tests live with implementation, same package

## Open Questions

Things that couldn't be fully resolved:

1. **Should Verbosity type be extracted?**
   - What we know: CacheManager uses build.Verbosity type
   - What's unclear: Should Verbosity move to internal/meta, or should cache accept an interface?
   - Recommendation: Keep build.Verbosity, have cache import "github.com/loov/clue/internal/build" just for this type, OR create internal/meta package if other shared types emerge

2. **Exact naming: Manager vs CacheManager?**
   - What we know: Current type is CacheManager
   - What's unclear: Should extracted package export cache.Manager or cache.CacheManager?
   - Recommendation: Use cache.Manager (package name provides context), but keeping CacheManager is also acceptable

3. **Should deps.go dependency parsing move to cache?**
   - What we know: ParseDepFile is in internal/build/deps.go, used by cache
   - What's unclear: Is it cache responsibility or build responsibility?
   - Recommendation: Keep in internal/build, cache imports it. Dependency parsing is broader than caching.

## Sources

### Primary (HIGH confidence)
- Internal codebase inspection: /workspace/internal/build/*.go - Current implementation state
- Go compiler behavior - Circular import prevention is enforced by toolchain
- Go standard library patterns - json, sync, time usage patterns

### Secondary (MEDIUM confidence)
- [Golang code refactoring: Best practices](https://codilime.com/blog/golang-code-refactoring-use-case/) - Refactoring methodologies
- [Eleven tips for structuring your Go projects](https://www.alexedwards.net/blog/11-tips-for-structuring-your-go-projects) - Internal package patterns
- [Organizing a Go module](https://go.dev/doc/modules/layout) - Official Go project layout guidance
- [GitHub: golang-standards/project-layout](https://github.com/golang-standards/project-layout) - Common Go project structures
- [Managing Circular Dependencies in Go](https://medium.com/@cosmicray001/managing-circular-dependencies-in-go-best-practices-and-solutions-723532f04dde) - Dependency direction patterns
- [How to break up circular dependencies](https://appliedgo.net/spotlight/circular-dependencies/) - Go circular import resolution
- [Import Cycles in Golang: How To Deal With Them](https://jogendra.dev/import-cycles-in-golang-and-how-to-deal-with-them) - Practical circular import solutions

### Tertiary (LOW confidence)
- [fsnotify package documentation](https://pkg.go.dev/github.com/fsnotify/fsnotify) - Library API reference
- [xxh3 package documentation](https://pkg.go.dev/github.com/zeebo/xxh3) - Library API reference
- [Chrome Trace Event Format](https://profilerpedia.markhansen.co.nz/formats/trace-event-format/) - Chrome trace specification
- [Bazel JSON Trace Profile](https://bazel.build/advanced/performance/json-trace-profile) - Build system profiling example

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already integrated, no new dependencies
- Architecture: HIGH - Standard Go package extraction pattern, well-documented
- Pitfalls: HIGH - Based on common Go refactoring mistakes and codebase structure

**Research date:** 2026-01-29
**Valid until:** 60 days (stable patterns, mature ecosystem)
