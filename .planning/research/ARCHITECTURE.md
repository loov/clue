# Architecture Research: v0.2.0

**Domain:** C++ Build System (Go-based)
**Features:** Windows MSVC, Watch Mode, Build Profiling
**Researched:** 2026-01-24
**Confidence:** HIGH

## Executive Summary

The existing Clue architecture is well-designed for extensibility. Windows MSVC support requires a new toolchain abstraction layer since MSVC uses fundamentally different flag syntax (`/flag` vs `-flag`) and tools (`cl.exe`, `link.exe`, `lib.exe`). Watch mode fits cleanly as a new top-level orchestrator that wraps existing build logic. Build profiling integrates naturally with the existing `Progress` type, adding timing instrumentation and output formatting.

The recommended approach creates three isolated feature additions: (1) toolchain interface abstraction with MSVC implementation, (2) `internal/watch` package using fsnotify, and (3) profiling extensions to `internal/build` with JSON/Chrome Trace output. No major refactoring of existing code is required.

## Existing Architecture Analysis

### Current Component Structure

```
internal/
  build/
    builder.go      -- Build orchestrator, entry point
    toolchain.go    -- GCC/Clang discovery (Unix-only)
    platform.go     -- Platform detection (linux/darwin/amd64/arm64)
    compiler.go     -- CompileSource(), flag construction
    linker.go       -- LinkExecutable(), CreateStaticLibrary(), LinkSharedLibrary()
    executor.go     -- RunCommand() subprocess execution
    parallel.go     -- ParallelCompiler, errgroup-based concurrency
    progress.go     -- Progress tracking, output formatting
    cache.go        -- Build cache key computation
    cache_manager.go -- Cache hit/miss logic
    flags.go        -- Semantic flag to compiler flag translation

  config/
    loader.go       -- CUE configuration parsing
    schema.cue      -- CUE schema definitions
    variants.go     -- Variant application

  generate/
    ninja.go        -- build.ninja generation
    compdb.go       -- compile_commands.json generation

  graph/
    builder.go      -- Target dependency graph
    file_graph.go   -- File-level dependency graph

  deps/
    manager.go      -- External dependency management
    fetcher.go      -- Dependency fetching

  errors/
    formatter.go    -- Rich error formatting with colors
```

### Key Abstractions

**Toolchain:** Currently a simple struct with `CC`, `CXX`, `AR`, `Name` fields. Discovery is Unix-specific (GCC/Clang only). This is the primary integration point for MSVC.

**Compiler/Linker:** Take `CompileOptions`/`LinkOptions` and construct command-line arguments. Flag construction uses `CompilerFlagsWithToolchain()` which already has toolchain parameter but only distinguishes "gcc" vs "clang".

**Executor:** Generic subprocess execution, already cross-platform (uses `syscall.SysProcAttr` but conditionally).

**Progress:** Already has atomic counters, timing, and output formatting. Natural extension point for profiling.

## Feature Integration Analysis

---

## 1. Windows MSVC Support

### Modified Components

| Package | File | Change Type | Description |
|---------|------|-------------|-------------|
| `internal/build` | `toolchain.go` | **MAJOR** | Extract interface, add MSVC implementation |
| `internal/build` | `platform.go` | **MINOR** | Add "windows-amd64" to supported platforms |
| `internal/build` | `flags.go` | **MAJOR** | Add MSVC flag mappings (`-O2` -> `/O2`) |
| `internal/build` | `compiler.go` | **MODERATE** | Handle MSVC-specific dependency output (`/showIncludes`) |
| `internal/build` | `linker.go` | **MAJOR** | MSVC uses `link.exe` and `lib.exe`, different flags |
| `internal/build` | `executor.go` | **MINOR** | Windows process group handling differs |
| `internal/config` | `schema.cue` | **MINOR** | Add "msvc" to toolchain.compiler options |
| `internal/generate` | `ninja.go` | **MODERATE** | Add MSVC rules with `/` flags |

### New Components

| Package | File | Purpose |
|---------|------|---------|
| `internal/build` | `toolchain_unix.go` | Build-tagged GCC/Clang discovery |
| `internal/build` | `toolchain_windows.go` | Build-tagged MSVC discovery |
| `internal/build` | `toolchain_msvc.go` | MSVC-specific toolchain implementation |
| `internal/build` | `flags_msvc.go` | MSVC semantic flag translation |
| `internal/build` | `executor_windows.go` | Windows-specific process handling |

### Architecture Pattern: Toolchain Interface

```go
// toolchain.go - New interface abstraction
type ToolchainDriver interface {
    // Tool paths
    CC() string      // C compiler (gcc, clang, cl.exe)
    CXX() string     // C++ compiler (g++, clang++, cl.exe)
    Archiver() string // Static lib (ar, lib.exe)
    Linker() string   // Link tool (ld via compiler, link.exe)

    // Flag translation
    CompilerFlags(config Config) []string
    LinkerFlags(config Config) []string

    // Dependency output format
    DependencyFlags(source, depFile string) []string
    ParseDependencies(output []byte) ([]string, error)

    // Platform characteristics
    ObjectExtension() string     // ".o" or ".obj"
    ExecutableExtension() string // "" or ".exe"
    StaticLibPrefix() string     // "lib" or ""
    StaticLibExtension() string  // ".a" or ".lib"
    SharedLibExtension() string  // ".so", ".dylib", ".dll"

    // Identity for cache keys
    Identity() (CompilerIdentity, error)
}
```

### MSVC Flag Mapping

| Semantic | GCC/Clang | MSVC |
|----------|-----------|------|
| `optimize: "none"` | `-O0` | `/Od` |
| `optimize: "size"` | `-Os` | `/O1` |
| `optimize: "fast"` | `-O2` | `/O2` |
| `optimize: "aggressive"` | `-O3` | `/Ox` |
| `debug: "full"` | `-g` | `/Zi` |
| `debug: "minimal"` | `-g1` | `/Z7` |
| `warnings: "default"` | `-Wall` | `/W3` |
| `warnings: "strict"` | `-Wall -Wextra` | `/W4` |
| `warnings: "pedantic"` | `-Wall -Wextra -Wpedantic` | `/W4 /permissive-` |
| `warningsAsErrors: true` | `-Werror` | `/WX` |

### MSVC Detection Strategy

```go
// toolchain_windows.go
func DiscoverMSVC() (*MSVCToolchain, error) {
    // 1. Check VCINSTALLDIR environment variable (set by vcvarsall.bat)
    if vcdir := os.Getenv("VCINSTALLDIR"); vcdir != "" {
        return findMSVCInVC(vcdir)
    }

    // 2. Use vswhere.exe to locate Visual Studio
    // Located at: %ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe
    vswhereOutput, err := exec.Command("vswhere.exe",
        "-latest", "-products", "*",
        "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
        "-property", "installationPath").Output()
    if err == nil {
        return findMSVCInInstall(strings.TrimSpace(string(vswhereOutput)))
    }

    // 3. Check well-known paths
    return findMSVCInDefaultPaths()
}
```

### Data Flow Changes (MSVC)

**Before:**
```
Config -> BuildCompilerFlags() -> [-O2, -Wall, ...] -> exec(clang, args)
```

**After:**
```
Config -> toolchain.CompilerFlags() ->
    GCC/Clang: [-O2, -Wall, ...]
    MSVC:      [/O2, /W3, ...]
-> exec(toolchain.CC(), args)
```

---

## 2. Watch Mode

### Modified Components

| Package | File | Change Type | Description |
|---------|------|-------------|-------------|
| `main.go` | | **MINOR** | Add `watch` command |
| `internal/build` | `builder.go` | **NONE** | Reused as-is |

### New Components

| Package | File | Purpose |
|---------|------|---------|
| `internal/watch` | `watcher.go` | fsnotify wrapper with debouncing |
| `internal/watch` | `rebuild.go` | Incremental rebuild orchestration |
| `internal/watch` | `filter.go` | Source file filtering (.c, .cpp, .h, etc.) |

### Architecture Pattern: Watch Orchestrator

```go
// internal/watch/watcher.go
type Watcher struct {
    fsWatcher   *fsnotify.Watcher
    config      *config.Config
    builder     *build.Builder
    opts        build.Options

    // Debouncing
    pending     map[string]time.Time
    debounce    time.Duration  // Default 100ms

    // State
    lastBuild   time.Time
    building    atomic.Bool
}

func (w *Watcher) Run(ctx context.Context) error {
    // 1. Add watches for all source directories
    for _, target := range w.config.Targets {
        for _, source := range target.Sources {
            dir := filepath.Dir(source)
            w.fsWatcher.Add(dir)
        }
        for _, include := range target.Includes {
            w.fsWatcher.Add(include)
        }
    }

    // 2. Event loop with debouncing
    for {
        select {
        case event := <-w.fsWatcher.Events:
            if w.isRelevant(event) {
                w.pending[event.Name] = time.Now()
                w.scheduleRebuild()
            }
        case err := <-w.fsWatcher.Errors:
            // Log but continue
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

### fsnotify Integration

**Library:** `github.com/fsnotify/fsnotify` (cross-platform, widely used)

**Key Considerations:**
1. **Debouncing:** Editors save files in multiple steps; wait 100ms for events to settle
2. **Recursive watching:** fsnotify doesn't support recursive watching; must add each directory
3. **Event filtering:** Ignore `.o`, `.d` files in build directory
4. **Platform limits:** inotify has watch limits (~65K default on Linux)

### Data Flow (Watch Mode)

```
User runs: clue watch

1. Load config, create Builder
2. Initial build (full)
3. Add watches for source directories
4. Event loop:
   - File change detected
   - Debounce (100ms window)
   - Determine affected targets
   - Incremental rebuild
   - Report results
   - Continue watching
```

### Incremental Rebuild Strategy

```go
// internal/watch/rebuild.go
func (w *Watcher) affectedTargets(changedFiles []string) []string {
    affected := make(map[string]bool)

    for _, file := range changedFiles {
        // Check each target's sources and includes
        for name, target := range w.config.Targets {
            if containsFile(target.Sources, file) ||
               containsInIncludePath(target.Includes, file) {
                affected[name] = true
                // Also add dependents
                for _, dep := range w.config.Targets {
                    if slices.Contains(dep.Depends, name) {
                        affected[dep.Name] = true
                    }
                }
            }
        }
    }

    return maps.Keys(affected)
}
```

---

## 3. Build Profiling

### Modified Components

| Package | File | Change Type | Description |
|---------|------|-------------|-------------|
| `main.go` | | **MINOR** | Add `--profile` flag |
| `internal/build` | `builder.go` | **MODERATE** | Add profiling hooks |
| `internal/build` | `parallel.go` | **MINOR** | Capture per-compilation timing |
| `internal/build` | `progress.go` | **MINOR** | Extend with profiling data collection |

### New Components

| Package | File | Purpose |
|---------|------|---------|
| `internal/build` | `profiler.go` | Timing collection and aggregation |
| `internal/build` | `profile_output.go` | JSON and Chrome Trace output |

### Architecture Pattern: Profiler

```go
// internal/build/profiler.go
type Profiler struct {
    enabled     bool
    startTime   time.Time
    events      []ProfileEvent
    mu          sync.Mutex
}

type ProfileEvent struct {
    Name      string        `json:"name"`
    Category  string        `json:"cat"`   // "compile", "link", "cache"
    Phase     string        `json:"ph"`    // "B"/"E" for begin/end, "X" for complete
    Timestamp int64         `json:"ts"`    // Microseconds from start
    Duration  int64         `json:"dur"`   // For "X" phase
    PID       int           `json:"pid"`
    TID       int           `json:"tid"`   // Goroutine ID for parallel
    Args      map[string]any `json:"args,omitempty"`
}

func (p *Profiler) BeginPhase(name, category string) func() {
    if !p.enabled {
        return func() {}
    }
    start := time.Now()
    return func() {
        p.recordEvent(ProfileEvent{
            Name:      name,
            Category:  category,
            Phase:     "X",
            Timestamp: p.usFromStart(start),
            Duration:  time.Since(start).Microseconds(),
        })
    }
}
```

### Output Formats

**1. Summary (default with --profile):**
```
Build Profile Summary
=====================
Total:     2.45s
Compile:   1.82s (74%)
  foo.cpp: 0.45s
  bar.cpp: 0.38s
  ...
Link:      0.51s (21%)
Cache:     0.12s (5%)
  Hits: 15, Misses: 8
```

**2. JSON (--profile=json):**
```json
{
  "total_ms": 2450,
  "phases": {
    "compile": {"duration_ms": 1820, "files": 23, "cache_hits": 15},
    "link": {"duration_ms": 510, "targets": 3},
    "cache": {"duration_ms": 120, "operations": 46}
  },
  "slowest_files": [
    {"path": "foo.cpp", "duration_ms": 450},
    {"path": "bar.cpp", "duration_ms": 380}
  ]
}
```

**3. Chrome Trace (--profile=trace):**
```json
[
  {"name": "foo.cpp", "cat": "compile", "ph": "X", "ts": 0, "dur": 450000, "pid": 1, "tid": 1},
  {"name": "bar.cpp", "cat": "compile", "ph": "X", "ts": 50000, "dur": 380000, "pid": 1, "tid": 2}
]
```

The Chrome Trace format can be visualized in `chrome://tracing` or Perfetto.

### Data Flow (Profiling)

```
Build starts
  |
  v
Profiler.Begin("build")
  |
  +-> For each target:
  |     Profiler.Begin("compile:target")
  |       +-> For each source (parallel):
  |       |     Profiler.Begin("compile:source")
  |       |       -> Compilation
  |       |     Profiler.End()
  |       Profiler.End()
  |     Profiler.Begin("link:target")
  |       -> Linking
  |     Profiler.End()
  |
Profiler.End()
  |
  v
Output profile (summary/JSON/trace)
```

---

## Cross-Platform Considerations

### Build Tags Strategy

```
internal/build/
  toolchain.go          // Interface definition (all platforms)
  toolchain_unix.go     // +build linux darwin
  toolchain_windows.go  // +build windows
  executor.go           // Common code
  executor_unix.go      // +build linux darwin (process groups)
  executor_windows.go   // +build windows (job objects)
```

### Windows-Specific Issues

| Issue | Impact | Mitigation |
|-------|--------|------------|
| Long paths (>260 chars) | Build fails in deep directories | Use `\\?\` prefix via `filepath.EvalSymlinks` |
| File locking | Can't delete .obj during rebuild | Use `gofrs/flock` for cache, handle EBUSY |
| Case-insensitive FS | Include path mismatches | Normalize paths with `filepath.Clean` |
| Process groups | Signal handling differs | Use Windows Job Objects |
| Line endings | Source file hashing inconsistent | Normalize to LF before hashing |

### Maintaining Linux/macOS Compatibility

1. **All new code uses build tags** - Platform-specific code isolated
2. **CI matrix includes all platforms** - Linux, macOS, Windows
3. **Interface-based abstraction** - Toolchain interface hides platform differences
4. **Feature flags** - MSVC features only available on Windows

---

## Suggested Phase Order

Based on dependency analysis and risk assessment:

### Phase 1: Toolchain Abstraction (Foundation)

**Rationale:** MSVC support requires refactoring the toolchain abstraction. This is the foundation for everything else and has the highest risk of breaking existing functionality.

**Components:**
1. Extract `ToolchainDriver` interface
2. Implement `GCCToolchain` and `ClangToolchain` (existing behavior)
3. Add platform detection for Windows
4. Update `flags.go` to use interface

**Risk:** HIGH - Core abstraction change
**Dependencies:** None (enables MSVC)

### Phase 2: Windows Build Support (Platform)

**Rationale:** With toolchain abstraction in place, MSVC implementation can proceed. Must be done before watch mode or profiling to ensure cross-platform testing.

**Components:**
1. `MSVCToolchain` implementation
2. MSVC flag mappings
3. Windows executor adjustments
4. Ninja MSVC rules

**Risk:** MEDIUM - Windows-specific, isolated
**Dependencies:** Phase 1 (Toolchain Abstraction)

### Phase 3: Build Profiling (Enhancement)

**Rationale:** Profiling is independent of watch mode and provides immediate value for debugging builds. Lower complexity than watch mode.

**Components:**
1. `Profiler` type with event collection
2. Integration with `Builder` and `ParallelCompiler`
3. Output formats (summary, JSON, trace)
4. CLI flag (`--profile`)

**Risk:** LOW - Additive, no existing code changes
**Dependencies:** None (can parallel with Phase 2)

### Phase 4: Watch Mode (Feature)

**Rationale:** Watch mode is the most complex new feature. Best implemented after profiling is available for debugging.

**Components:**
1. `internal/watch` package
2. fsnotify integration with debouncing
3. Incremental rebuild logic
4. CLI command (`clue watch`)

**Risk:** MEDIUM - New subsystem, edge cases with file events
**Dependencies:** None (can parallel with Phase 2-3)

---

## Component Dependency Diagram

```
                    +-------------------+
                    |     main.go       |
                    |  (CLI commands)   |
                    +-------------------+
                           |
        +------------------+------------------+
        |                  |                  |
        v                  v                  v
+---------------+  +---------------+  +---------------+
| clue build    |  | clue watch    |  | --profile     |
+---------------+  +---------------+  +---------------+
        |                  |                  |
        v                  v                  v
+---------------+  +---------------+  +---------------+
| build.Builder |  | watch.Watcher |  | build.Profiler|
+---------------+  +---------------+  +---------------+
        |                  |
        +--------+---------+
                 |
                 v
        +-------------------+
        | ToolchainDriver   |  <-- NEW INTERFACE
        +-------------------+
               |
    +----------+----------+
    |          |          |
    v          v          v
+-------+  +-------+  +------+
| GCC   |  | Clang |  | MSVC |  <-- NEW
+-------+  +-------+  +------+
```

---

## Testing Strategy

### MSVC Testing

1. **CI Matrix:** Add Windows runner with MSVC
2. **vcvarsall.bat:** Run before tests to set environment
3. **Mock toolchain:** Unit tests use interface mocking
4. **Integration:** Real MSVC builds on Windows CI only

### Watch Mode Testing

1. **Unit tests:** Mock fsnotify events
2. **Integration:** Real file changes with short timeout
3. **Platform tests:** Linux inotify, macOS FSEvents, Windows ReadDirectoryChanges

### Profiling Testing

1. **Unit tests:** Verify timing collection
2. **Format tests:** Validate JSON/trace output
3. **Integration:** Full build with profiling enabled

---

## Risk Assessment

| Feature | Risk | Mitigation |
|---------|------|------------|
| Toolchain Interface | Breaking existing builds | Extensive test coverage before merge |
| MSVC Flag Mapping | Incorrect translations | Side-by-side comparison with CMake/Meson |
| Windows Long Paths | Unexpected failures | Use `\\?\` prefix, test deep directories |
| fsnotify Limits | Too many watches | Implement directory batching, document limits |
| Watch Debouncing | Missed or duplicate builds | Configurable debounce, thorough edge case testing |
| Profile Overhead | Slowdown when profiling | Conditional instrumentation, benchmark overhead |

---

## Sources

### fsnotify / Watch Mode
- [fsnotify/fsnotify GitHub](https://github.com/fsnotify/fsnotify) - Cross-platform filesystem notifications
- [fsnotify Go Package](https://pkg.go.dev/github.com/fsnotify/fsnotify) - API documentation

### Profiling
- [Go runtime/pprof](https://pkg.go.dev/runtime/pprof) - Built-in profiling
- [Go Diagnostics](https://go.dev/doc/diagnostics) - Official profiling guide
- [Chrome Trace Format](https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU) - Trace event format

### MSVC
- [MSVC Compiler Options](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options?view=msvc-170) - Microsoft documentation
- [Clang MSVC Compatibility](https://clang.llvm.org/docs/MSVCCompatibility.html) - Clang's MSVC support
- [Visual Studio Build Tools](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line?view=msvc-170) - Command-line usage

### File Locking
- [gofrs/flock](https://github.com/gofrs/flock) - Cross-platform file locking
- [Go internal filelock](https://pkg.go.dev/cmd/go/internal/lockedfile/internal/filelock) - Go's internal implementation
