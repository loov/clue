# Project Research Summary

**Project:** Clue Build System v0.2.0
**Domain:** C/C++ Build System (Go-based)
**Researched:** 2026-01-24
**Confidence:** HIGH

## Executive Summary

Clue v0.2.0 adds three capabilities to the existing Linux/macOS GCC/Clang build system: Windows MSVC support, watch mode for auto-rebuild, and build profiling. The research confirms that MSVC integration is the highest-complexity addition, requiring a new toolchain abstraction layer since MSVC uses fundamentally different flag syntax (`/flag` vs `-flag`), separate tools (`cl.exe`, `link.exe`, `lib.exe`), and Windows-specific environment setup via `vcvarsall.bat`. Watch mode and profiling are lower-risk additions that integrate cleanly with existing infrastructure.

The recommended approach is to implement a `ToolchainDriver` interface that abstracts compiler-specific differences, allowing the existing build logic to work unchanged across GCC, Clang, and MSVC. Watch mode should use `fsnotify` (already recommended for v0.1.0) with leading-edge debouncing to handle editor save patterns. Build profiling should output Chrome Trace JSON format, which is the de facto standard used by Ninja and Bazel.

Key risks center on MSVC integration: command line length limits (8191 characters requires response file support), localized `/showIncludes` output requiring runtime prefix detection, and the separate linker/archiver tools. These are well-documented problems with known solutions. Watch mode risks are mainly platform-specific fsnotify behaviors on Windows. Profiling is lowest risk as it is purely additive with no changes to existing code paths.

## Key Findings

### Recommended Stack Additions

No new Go dependencies are required for v0.2.0. MSVC support uses standard library (`os/exec`, `encoding/json`) to invoke `vswhere.exe` and parse its output. fsnotify was already recommended in v0.1.0 research.

**Core additions:**

| Component | Purpose | Rationale |
|-----------|---------|-----------|
| `github.com/fsnotify/fsnotify` v1.9.0 | File watching | Cross-platform, 12K+ importers, actively maintained |
| Chrome Trace JSON (stdlib) | Profiling output | Standard format, viewable in chrome://tracing and Perfetto |
| `vswhere.exe` (external) | MSVC discovery | Microsoft's official tool for locating Visual Studio |

**Not recommended:**

- No separate debounce library (simple to implement inline)
- No cgo for Windows APIs (os/exec is simpler)
- No MinGW for v0.2.0 (focus on MSVC; MinGW can come later)
- No polling-based watchers (OS notifications work fine)

### Expected Features

**Must have (table stakes):**

| Feature | Complexity | Notes |
|---------|------------|-------|
| Visual Studio detection via vswhere | MEDIUM | Required for any Windows user |
| cl.exe/link.exe/lib.exe invocation | MEDIUM | Different tools unlike GCC/Clang |
| MSVC flag translation (/O2, /W3, /Zi) | MEDIUM | Cannot translate; must generate from scratch |
| Response file support (@file.rsp) | MEDIUM | Windows 8KB command line limit |
| Directory watching with debounce | LOW-MEDIUM | 100ms window prevents duplicate builds |
| Per-file compilation timing | LOW | Already captured in CompileResult |
| Slowest files report | LOW | Sort and print top N |

**Should have (differentiators):**

| Feature | Complexity | Notes |
|---------|------------|-------|
| Automatic vcvars environment capture | HIGH | Zero-config Windows experience |
| Chrome Trace profiling output | MEDIUM | Visualize parallelism, find bottlenecks |
| Smart header-change rebuild | MEDIUM | Parse .d files for transitive deps |
| Configurable debounce interval | LOW | --debounce 200ms flag |
| MSVC incremental linking | LOW | /INCREMENTAL flag |

**Defer (v2+):**

- Cross-architecture MSVC (x86_amd64, amd64_arm64)
- MinGW/Clang-on-Windows support
- Network filesystem watch support
- Deep compiler phase analysis (template instantiation)
- MSBuild/vcxproj generation

### Architecture Approach

The existing Clue architecture supports v0.2.0 with minimal refactoring. The key change is extracting a `ToolchainDriver` interface from the current `Toolchain` struct, allowing GCC, Clang, and MSVC implementations to coexist. Watch mode is a new top-level orchestrator (`internal/watch`) that wraps existing `build.Builder`. Profiling extends `internal/build` with timing collection and output formatting.

**Major components:**

1. **ToolchainDriver interface** (new) — Abstracts CC/CXX/Linker/Archiver with toolchain-specific flag generation
2. **MSVCToolchain** (new) — MSVC implementation with vswhere discovery and vcvarsall environment capture
3. **internal/watch** (new package) — fsnotify wrapper with debouncing and rebuild orchestration
4. **build.Profiler** (new) — Event collection and Chrome Trace JSON output
5. **flags_msvc.go** (new) — Semantic flag to MSVC flag translation

**Build tag strategy:**

```
toolchain.go          // Interface (all platforms)
toolchain_unix.go     // +build linux darwin
toolchain_windows.go  // +build windows
toolchain_msvc.go     // MSVC implementation
```

### Critical Pitfalls

1. **MSVC flags are not translatable** — Never attempt `-O2` to `/O2` string replacement. The flag systems have different semantics. Create separate `CompilerFlagsForMSVC()` that constructs MSVC flags from scratch based on semantic configuration.

2. **Windows command line limit (8191 chars)** — Builds with many files or long paths fail silently. Implement response file support from day one: detect when approaching 7KB, write args to temp `.rsp` file, pass `@response.rsp` to cl.exe/link.exe.

3. **MSVC /showIncludes is localized** — Prefix varies by Windows language (English: "Note: including file:", German: "Hinweis: Einlesen der Datei:"). Probe the compiler at discovery time to capture the actual prefix, or use `/sourceDependencies` (JSON output, VS2019 16.7+).

4. **MSVC uses separate tools** — Unlike GCC/Clang where the compiler driver also links, MSVC requires calling `link.exe` and `lib.exe` directly with different flag syntax. Extend Toolchain struct with Linker and Lib fields.

5. **Editor atomic saves trigger multiple events** — VSCode and other editors save via write-to-temp + rename, causing 2-5 fsnotify events per save. Use leading-edge debounce (100-200ms window) and track actual content changes.

6. **fsnotify lacks recursive watching** — Must walk directory tree at startup and add each directory explicitly. Watch for CREATE events on directories to add new subdirectories dynamically.

## Implications for Roadmap

Based on dependency analysis and risk assessment, v0.2.0 should be structured as four phases.

### Phase 1: Toolchain Abstraction

**Rationale:** MSVC support requires refactoring the core toolchain abstraction. This is foundation work that must precede Windows support and carries the highest risk of breaking existing Linux/macOS functionality.

**Delivers:**
- `ToolchainDriver` interface extracted from current code
- `GCCToolchain` and `ClangToolchain` implementations (existing behavior, new types)
- Toolchain-aware flag generation via interface methods
- All existing tests passing with new abstraction

**Addresses features:** Enables compiler abstraction, paves way for MSVC

**Avoids pitfall:** Flag translation failures by designing semantic-to-toolchain translation correctly from the start

**Risk:** HIGH (core abstraction change affecting all builds)

**Research needed:** LOW (standard Go interface patterns)

### Phase 2: Windows MSVC Support

**Rationale:** With toolchain abstraction in place, MSVC implementation proceeds in isolation. Must complete before watch mode or profiling to enable Windows CI testing.

**Delivers:**
- MSVC toolchain discovery via vswhere.exe
- vcvarsall.bat environment capture
- cl.exe, link.exe, lib.exe invocation
- MSVC flag mappings
- Response file support for long command lines
- Ninja rules for MSVC
- Windows CI integration

**Addresses features:** Visual Studio detection, cl.exe compilation, link.exe linking, debug symbols, optimization flags, warning levels, response files, windows-amd64 platform

**Avoids pitfalls:** Command line limit, localized /showIncludes, separate tools, Go exec.Command quoting

**Uses:** Standard library (os/exec, encoding/json)

**Risk:** MEDIUM (Windows-specific, isolated from Unix code paths)

**Research needed:** LOW (MSVC documentation is comprehensive)

### Phase 3: Build Profiling

**Rationale:** Independent of MSVC and watch mode. Low complexity, provides immediate diagnostic value. Having profiling available before watch mode aids debugging.

**Delivers:**
- `Profiler` type with event collection
- Integration with Builder and ParallelCompiler
- Timing summary output (default)
- JSON output (--profile=json)
- Chrome Trace output (--profile=trace)
- Cache hit/miss reporting
- Slowest files report

**Addresses features:** Per-file compilation timing, total build time, slowest files, parallelism visualization, link time tracking

**Avoids pitfalls:** Profiling overhead (opt-in only), JSON file aggregation

**Uses:** Standard library (encoding/json, time)

**Risk:** LOW (purely additive, no existing code changes)

**Research needed:** NONE (Chrome Trace format is simple and well-documented)

### Phase 4: Watch Mode

**Rationale:** Most complex new feature. Benefits from having profiling available for debugging. Less critical than MSVC support for user adoption.

**Delivers:**
- `internal/watch` package
- fsnotify integration with platform-specific handling
- Leading-edge debouncing (100ms default, configurable)
- Source file filtering (.c, .cpp, .h, .cue)
- Directory watching with dynamic subdirectory addition
- Incremental rebuild triggering
- Graceful shutdown on Ctrl+C
- `clue watch` command

**Addresses features:** Directory watching, file change detection, event debouncing, incremental rebuild, graceful shutdown, source file filtering

**Avoids pitfalls:** Editor atomic saves, fsnotify recursion, Windows behavior differences

**Uses:** fsnotify v1.9.0

**Risk:** MEDIUM (new subsystem, platform-specific edge cases)

**Research needed:** LOW (fsnotify is well-documented; debounce patterns established)

### Phase Ordering Rationale

1. **Toolchain abstraction first:** All subsequent features depend on proper toolchain abstraction. Breaking this out ensures the foundation is solid before adding complexity.

2. **MSVC before profiling/watch:** Windows CI must be operational to test profiling and watch mode cross-platform. MSVC is the stated priority for v0.2.0.

3. **Profiling before watch:** Profiling is simpler and provides debugging tools useful when developing watch mode.

4. **Watch mode last:** Highest new-code complexity, benefits from all other features being stable, and is least critical for initial v0.2.0 adoption.

### Research Flags

**Phases needing deeper research during planning:**

- **Phase 1 (Toolchain Abstraction):** May need to research interface patterns for other build systems (CMake's toolchain files, Meson's cross files) to ensure the abstraction is sufficiently flexible for future expansion.

**Phases with standard patterns (skip research-phase):**

- **Phase 2 (MSVC):** Microsoft documentation is comprehensive; vswhere usage is well-documented; flag mappings verified against multiple sources.
- **Phase 3 (Profiling):** Chrome Trace format is a simple, stable spec with many reference implementations.
- **Phase 4 (Watch):** fsnotify is mature with good documentation; debouncing is a solved problem.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | No new dependencies needed; fsnotify already vetted; Chrome Trace is standard |
| Features | HIGH | Based on Microsoft Learn docs, verified against CMake/Meson/xmake feature sets |
| Architecture | HIGH | Follows established patterns; interface extraction is standard Go |
| Pitfalls | HIGH | Cross-referenced with Ninja, CMake, fsnotify GitHub issues; MSVC docs complete |

**Overall confidence:** HIGH

All research areas have authoritative primary sources. MSVC documentation is particularly comprehensive. fsnotify has extensive community experience. Chrome Trace format is stable and widely implemented.

### Gaps to Address

1. **MSVC /showIncludes prefix detection:** Need to implement and test prefix probing across multiple Windows language installations. Consider falling back to `/sourceDependencies` (JSON) if probing fails.

2. **Debounce timing validation:** 100ms is a reasonable default, but may need tuning based on real-world editor testing. Make configurable with `--debounce` flag.

3. **Windows long path handling:** Research suggests using `\\?\` prefix, but this needs explicit testing in deep directory structures. May require `filepath.EvalSymlinks` workaround.

4. **MSVC on ARM64:** Windows ARM64 support exists but is less common; may defer to v2.1 if testing resources are limited.

## Sources

### Primary (HIGH confidence)

**MSVC:**
- [MSVC Compiler Options](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options?view=msvc-170) - Flag syntax and semantics
- [MSVC Linker Options](https://learn.microsoft.com/en-us/cpp/build/reference/linker-options?view=msvc-170) - link.exe usage
- [Building on Command Line](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line?view=msvc-170) - vcvarsall.bat, vswhere
- [Windows Command Line Limitation](https://learn.microsoft.com/en-us/troubleshoot/windows-client/shell-experience/command-line-string-limitation) - 8191 char limit

**Watch Mode:**
- [fsnotify Go Package](https://pkg.go.dev/github.com/fsnotify/fsnotify) - API documentation (v1.9.0, Apr 2025)
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify) - Platform support, known issues

**Profiling:**
- [Chrome Trace Format](https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU) - Trace event format specification
- [Bazel JSON Trace Profile](https://bazel.build/advanced/performance/json-trace-profile) - Reference implementation

### Secondary (MEDIUM confidence)

- [Ninja MSVC /showIncludes localization](https://github.com/ninja-build/ninja/issues/1766) - Known issue with prefix detection
- [Debouncing File Events in Go](https://dev.to/asoseil/from-chaos-to-signal-taming-high-frequency-os-events-in-go-4p8k) - Debounce patterns
- [ClangBuildAnalyzer](https://github.com/aras-p/ClangBuildAnalyzer) - Profiling aggregation reference

### Tertiary (LOW confidence)

- Debounce timing (100ms) may need adjustment based on editor testing
- Windows ARM64 MSVC testing limited

---
*Research completed: 2026-01-24*
*Ready for roadmap: yes*
