# Stack Research: v0.2.0 Additions

**Project:** Clue Build System
**Researched:** 2026-01-24
**Scope:** Windows MSVC support, watch mode, build profiling
**Confidence:** HIGH

## Executive Summary

For v0.2.0, Clue needs three capability additions: (1) MSVC toolchain integration requiring no new Go dependencies but significant code changes to handle MSVC's different flag syntax and environment discovery via `vswhere.exe`, (2) file watching using the existing `fsnotify` recommendation with custom debouncing logic, and (3) build profiling via Chrome Trace JSON output, implementable with the standard library. All three integrate cleanly with the existing Go/CUE/errgroup stack.

## Recommended Additions

### Windows MSVC Support

**No new dependencies required.** MSVC integration is primarily code changes to existing packages.

#### Environment Discovery

| Component | Approach | Rationale |
|-----------|----------|-----------|
| **vswhere.exe** | Execute via `os/exec`, parse JSON output | Microsoft's official tool for locating Visual Studio installations. Standard location: `C:\Program Files (x86)\Microsoft Visual Studio\Installer\vswhere.exe`. Use `-format json` for structured output. |
| **vcvarsall.bat** | Locate via vswhere, execute to capture environment | Sets PATH, INCLUDE, LIB environment variables needed by cl.exe/link.exe. Located at `VC\Auxiliary\Build\vcvarsall.bat` relative to VS installation. |

**Confidence: HIGH** - vswhere is Microsoft's official discovery tool, documented on [Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line?view=msvc-170).

#### Toolchain Components

| Tool | MSVC | GCC/Clang Equivalent |
|------|------|---------------------|
| C/C++ Compiler | `cl.exe` | `gcc`/`clang` |
| Linker | `link.exe` | `ld` (via compiler driver) |
| Archiver | `lib.exe` | `ar` |

#### Flag Translation (Existing Code Changes)

The current `internal/build/flags.go` uses GCC/Clang flags. MSVC requires translation:

| Semantic | GCC/Clang | MSVC |
|----------|-----------|------|
| Compile only | `-c` | `/c` |
| Output object | `-o file.o` | `/Fo:file.obj` |
| Output executable | `-o file` | `/Fe:file.exe` |
| Optimization none | `-O0` | `/Od` |
| Optimization fast | `-O2` | `/O2` |
| Optimization aggressive | `-O3` | `/Ox` |
| Optimization size | `-Os` | `/Os` |
| Warnings default | `-Wall` | `/W3` |
| Warnings strict | `-Wall -Wextra` | `/W4` |
| Warnings all | `-Wall -Wextra -Wpedantic` | `/Wall` |
| Warnings as errors | `-Werror` | `/WX` |
| Debug info | `-g` | `/Zi` |
| Debug minimal | `-g1` | `/Z7` |
| Include path | `-I/path` | `/I/path` |
| Define | `-DFOO` | `/DFOO` |
| Language standard | `-std=c++20` | `/std:c++20` |
| PIC | `-fPIC` | (not needed, DLLs are always position-independent) |
| Shared library | `-shared` | `/LD` |
| Dependency output | `-MMD -MF file.d` | `/showIncludes` (parse stdout) |

**Confidence: HIGH** - Verified against [MSVC Compiler Options by Category](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options-listed-by-category?view=msvc-170).

#### Linker Flag Translation

| Semantic | GCC/Clang | MSVC link.exe |
|----------|-----------|---------------|
| Output | `-o file` | `/OUT:file` |
| Library path | `-L/path` | `/LIBPATH:/path` |
| Link library | `-lfoo` | `foo.lib` |
| Debug | `-g` | `/DEBUG` |
| Shared | `-shared` | `/DLL` |

**Confidence: HIGH** - Verified against [MSVC Linker Options](https://learn.microsoft.com/en-us/cpp/build/reference/linker-options?view=msvc-170).

#### Archiver Translation

| Operation | Unix ar | MSVC lib.exe |
|-----------|---------|--------------|
| Create archive | `ar crs lib.a obj.o` | `lib /OUT:lib.lib obj.obj` |

**Note:** MSVC `.lib` files use AR format internally, but the command syntax differs.

#### Implementation Notes

1. **Toolchain struct extension**: Add `LinkerCmd` and `ArchiverCmd` fields (currently AR assumed to be `ar`)
2. **Platform detection**: Extend `internal/build/platform.go` to support `windows-amd64` and `windows-arm64`
3. **Flag abstraction**: Create toolchain-aware flag generation in `flags.go`
4. **Environment capture**: On Windows, need to capture environment variables set by `vcvarsall.bat` since they're required for cl.exe to find headers/libs
5. **Dependency parsing**: MSVC's `/showIncludes` outputs to stdout in a different format than GCC's `.d` files - need parser

#### vswhere Discovery Pattern

```go
// Example: Discover Visual Studio installation
func discoverMSVC() (*MSVCToolchain, error) {
    vswhere := `C:\Program Files (x86)\Microsoft Visual Studio\Installer\vswhere.exe`

    cmd := exec.Command(vswhere,
        "-latest",
        "-products", "*",
        "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
        "-property", "installationPath",
        "-format", "json")

    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("vswhere failed: %w", err)
    }

    var installations []struct {
        InstallationPath string `json:"installationPath"`
    }
    if err := json.Unmarshal(output, &installations); err != nil {
        return nil, err
    }

    if len(installations) == 0 {
        return nil, errors.New("no Visual Studio installation with C++ tools found")
    }

    // vcvarsall.bat is at: {installPath}\VC\Auxiliary\Build\vcvarsall.bat
    vcvarsall := filepath.Join(installations[0].InstallationPath,
        "VC", "Auxiliary", "Build", "vcvarsall.bat")

    return &MSVCToolchain{
        InstallPath: installations[0].InstallationPath,
        VCVarsAll:   vcvarsall,
    }, nil
}
```

### Watch Mode

**Use existing recommendation:** `github.com/fsnotify/fsnotify` v1.9.0 (already in v0.1.0 STACK.md)

| Library | Version | Purpose |
|---------|---------|---------|
| **fsnotify** | v1.9.0 | Cross-platform file system notifications |

#### Why fsnotify (Reconfirmed)

- **Maturity:** 12,768+ importers, actively maintained (latest release Apr 2025)
- **Platform support:** Windows (ReadDirectoryChangesW), Linux (inotify), macOS (kqueue), BSD, illumos
- **Minimal API:** Simple Watcher with Events and Errors channels
- **Go version:** Requires Go 1.17+ (Clue uses Go 1.25, compatible)
- **Already recommended:** In v0.1.0 STACK.md

**Confidence: HIGH** - Verified via [pkg.go.dev](https://pkg.go.dev/github.com/fsnotify/fsnotify).

#### Debouncing Strategy (NEW for v0.2.0)

File operations often trigger multiple events (editor saves trigger WRITE, CHMOD, etc.). The "double-fire" problem is well-documented.

**Leading-edge debounce with per-file cooldown:**

```go
const debounceDelay = 100 * time.Millisecond

type Debouncer struct {
    mu       sync.Mutex
    lastSeen map[string]time.Time
}

func (d *Debouncer) ShouldProcess(path string) bool {
    d.mu.Lock()
    defer d.mu.Unlock()

    now := time.Now()
    if last, ok := d.lastSeen[path]; ok {
        if now.Sub(last) < debounceDelay {
            return false // Suppress echo
        }
    }
    d.lastSeen[path] = now
    return true // Process immediately
}
```

**Why leading-edge:** Acts immediately on first event, suppresses subsequent echoes. More responsive than trailing-edge (waiting for quiet period).

**Confidence: MEDIUM** - Pattern verified from [community sources](https://dev.to/asoseil/from-chaos-to-signal-taming-high-frequency-os-events-in-go-4p8k), exact timing (100ms) may need tuning.

#### Watch Mode Implementation Notes

1. **Watch directories, not files:** fsnotify recommends watching parent directories because editors often use atomic saves (write to temp, rename to target)
2. **Source file filtering:** Filter events to only source files (`.c`, `.cpp`, `.cc`, `.h`, `.hpp`) and CUE config (`.cue`)
3. **Rebuild trigger:** On valid change, trigger incremental rebuild using existing `internal/build` infrastructure
4. **Graceful handling:** Integrate with existing signal handling (double Ctrl+C pattern from v0.1.0)
5. **Ignore build outputs:** Filter out events from `build/` output directory

### Build Profiling

**No new dependencies.** Use standard library + Chrome Trace JSON format.

#### Chrome Trace Format

Output timing data as Chrome Trace JSON for visualization in:
- Chrome DevTools (chrome://tracing)
- [Perfetto UI](https://ui.perfetto.dev) (modern alternative)
- VSCode extensions

**Confidence: HIGH** - Chrome Trace format is stable and widely adopted by build systems including [Ninja](https://github.com/nico/ninjatracing) and [Bazel](https://bazel.build/advanced/performance/json-trace-profile).

#### JSON Structure

```go
type TraceEvent struct {
    Name string                 `json:"name"`           // Event name
    Cat  string                 `json:"cat,omitempty"`  // Category
    Ph   string                 `json:"ph"`             // Phase: "X" for complete event
    Ts   float64                `json:"ts"`             // Timestamp in microseconds
    Dur  float64                `json:"dur,omitempty"`  // Duration in microseconds
    Pid  int                    `json:"pid"`            // Process ID
    Tid  int                    `json:"tid"`            // Thread ID (goroutine identifier)
    Args map[string]interface{} `json:"args,omitempty"` // Custom data
}

type TraceOutput struct {
    TraceEvents []TraceEvent `json:"traceEvents"`
}
```

**Key fields:**
- `ph: "X"` - Complete event (timestamp + duration)
- `ts` / `dur` - Microseconds (float64 for precision)
- `tid` - Use sequential IDs for parallel compilation threads
- `args` - Attach source file path, compiler flags, etc.

#### Event Categories

| Category | Events | Example Name |
|----------|--------|--------------|
| `compile` | Per-file compilation | `compile src/main.cpp` |
| `link` | Executable/library linking | `link myapp` |
| `deps` | Dependency fetching | `git clone libfoo` |
| `config` | CUE loading and validation | `load config` |
| `cache` | Cache lookups, hash computation | `hash src/main.cpp` |

#### Implementation Approach

1. **Timing collection:** Already have `time.Duration` in `CompileResult` and `LinkResult`
2. **Start time tracking:** Add `StartTime time.Time` to results
3. **Event accumulation:** Collect events during build in a slice
4. **Output options:**
   - `--profile=build-trace.json` writes Chrome Trace JSON
   - `--profile=-` writes to stdout
4. **Parallel tracking:** Use sequential thread IDs (0, 1, 2...) for each parallel goroutine to show parallelism in trace viewer

#### Example Output

```json
{"traceEvents": [
  {"name":"load config","cat":"config","ph":"X","ts":0,"dur":15000,"pid":1,"tid":0},
  {"name":"compile src/main.cpp","cat":"compile","ph":"X","ts":20000,"dur":150000,"pid":1,"tid":1,"args":{"source":"src/main.cpp"}},
  {"name":"compile src/util.cpp","cat":"compile","ph":"X","ts":25000,"dur":120000,"pid":1,"tid":2,"args":{"source":"src/util.cpp"}},
  {"name":"link myapp","cat":"link","ph":"X","ts":175000,"dur":50000,"pid":1,"tid":0}
]}
```

#### Usage

```bash
clue build --profile=build-trace.json
# Open in chrome://tracing or https://ui.perfetto.dev
```

## Integration Notes

### Existing Stack Compatibility

| Component | Integration |
|-----------|-------------|
| **Go 1.25** | All additions compatible. fsnotify needs 1.17+, standard library for rest. |
| **CUE v0.15.3** | No changes. Watch mode watches `.cue` files too. |
| **xxh3** | No changes. Content hashing unchanged. |
| **errgroup** | Watch mode uses errgroup for parallel watch + build coordination. |

### Code Changes by Package

| Package | Changes for v0.2.0 |
|---------|---------|
| `internal/build/toolchain.go` | Add MSVC toolchain discovery, extend Toolchain struct with LinkerCmd/ArchiverCmd |
| `internal/build/platform.go` | Add `windows-amd64`, `windows-arm64` to supported platforms |
| `internal/build/flags.go` | Add MSVC flag mappings, make flag generation toolchain-aware |
| `internal/build/compiler.go` | Handle MSVC-specific compilation (different flags, `/showIncludes` parsing) |
| `internal/build/linker.go` | Handle MSVC link.exe invocation, different library linking syntax |
| `internal/build/watch.go` (new) | Watch mode with fsnotify, debouncing, rebuild triggering |
| `internal/build/profile.go` (new) | Chrome Trace event collection and JSON output |
| `main.go` | Add `--watch`, `--profile` flags |

### Windows-Specific Go Packages

Already in go.mod via transitive dependencies:
- `github.com/Microsoft/go-winio` v0.6.2 (via go-git) - Windows named pipe support
- `golang.org/x/sys` v0.37.0 - Windows syscalls

No additional Windows packages needed. Use `os/exec` for vswhere/cl.exe invocation.

## Not Recommended

### What NOT to Add

| Don't Add | Reason |
|-----------|--------|
| **CMake/Meson integration** | Clue IS a build system, not a meta-build system |
| **Separate debounce library** | Simple to implement inline (~20 lines), avoids dependency |
| **pprof for build profiling** | pprof profiles Go runtime, not build process timing. Chrome Trace is for build performance. |
| **cgo for Windows APIs** | `os/exec` calling vswhere.exe is simpler, avoids cross-compilation complexity |
| **MSVC path hardcoding** | Visual Studio installations vary; always use vswhere for discovery |
| **MinGW on Windows** | MSVC is the priority for v0.2.0. MinGW/Clang-on-Windows can come later. |
| **rjeczalik/notify** | More complex API; recursive watching not needed since Clue knows source files from CUE config |
| **Polling-based watchers** | Higher CPU usage; OS notifications work fine for this use case |

### Common Pitfalls to Avoid

1. **Don't assume MSVC is in PATH:** Always discover via vswhere, set environment via vcvarsall.bat capture
2. **Don't forget `/showIncludes` parsing:** MSVC generates dependencies differently than GCC's `-MMD` output to `.d` files
3. **Don't watch individual files:** Watch directories and filter events by extension
4. **Don't skip debouncing:** Editors (especially VSCode) trigger 3-5 events per save
5. **Don't use trailing-edge debounce:** Leading-edge with cooldown feels more responsive
6. **Don't forget PDB files:** MSVC generates `.pdb` debug databases alongside executables

## Dependency Summary

### New Direct Dependencies for v0.2.0

| Dependency | Version | Purpose | Status |
|------------|---------|---------|--------|
| `github.com/fsnotify/fsnotify` | v1.9.0 | Watch mode file notifications | Already recommended in v0.1.0 STACK |

### No Changes Needed

| Existing | Status |
|----------|--------|
| Go version | 1.25 (compatible) |
| CUE | v0.15.3 (unchanged) |
| xxh3 | v1.0.2 (unchanged) |
| go-git | v5.16.4 (unchanged) |
| golang.org/x/sync | v0.17.0 (unchanged, use for watch mode) |

## Sources

### MSVC/Windows (HIGH confidence)
- [MSVC Compiler Options](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options?view=msvc-170) - Microsoft Learn
- [MSVC Compiler Options by Category](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options-listed-by-category?view=msvc-170) - Microsoft Learn
- [MSVC Linker Options](https://learn.microsoft.com/en-us/cpp/build/reference/linker-options?view=msvc-170) - Microsoft Learn
- [MSVC Command Line Build Tools](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line?view=msvc-170) - Microsoft Learn
- [vswhere Usage](https://commandmasters.com/commands/vswhere-windows/) - Command Masters

### Watch Mode (HIGH confidence)
- [fsnotify Package](https://pkg.go.dev/github.com/fsnotify/fsnotify) - Go Packages (v1.9.0, Apr 2025)
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify) - Platform support, limitations

### Watch Mode Debouncing (MEDIUM confidence)
- [Debouncing File Events in Go](https://dev.to/asoseil/from-chaos-to-signal-taming-high-frequency-os-events-in-go-4p8k) - DEV Community

### Build Profiling (HIGH confidence)
- [Chrome Trace Format](https://aras-p.info/blog/2017/01/23/Chrome-Tracing-as-Profiler-Frontend/) - Aras Pranckevicius
- [Introduction to Chrome Tracing](https://markdewing.github.io/blog/posts/2024/introduction-to-chrome-tracing/) - Mark Dewing
- [ninjatracing](https://github.com/nico/ninjatracing) - Reference implementation for ninja_log to Chrome trace
- [Bazel JSON Trace Profile](https://bazel.build/advanced/performance/json-trace-profile) - Similar approach in Bazel

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| fsnotify recommendation | HIGH | Official docs verified, version confirmed via pkg.go.dev |
| MSVC flag mappings | HIGH | Verified against Microsoft Learn documentation |
| MSVC environment discovery | HIGH | vswhere is Microsoft's official tool |
| Chrome Trace format | HIGH | Cross-verified with multiple sources, widely used |
| Debouncing approach | MEDIUM | Pattern verified, exact timing (100ms) may need tuning |
| MSVC dependency parsing | MEDIUM | `/showIncludes` is documented but parser needs implementation |
