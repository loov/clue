# Clue

## What This Is

A Go-based build system for C, C++, and similar languages that uses CUE as its configuration language. Clue replaces the complexity of CMake/Make with declarative, validated configuration that handles dependencies cleanly -- whether system packages, vendored source, git repos, or tarballs. Ships with incremental builds, parallel compilation, Windows MSVC support, watch mode, build profiling, and IDE integration out of the box.

## Core Value

Minimal configuration for common cases, with CUE's type system catching config errors before build time -- not during.

## Requirements

### Validated

#### v0.1.0 MVP
- ✓ CONF-01: Parse CUE configuration files with schema validation -- v0.1.0
- ✓ CONF-02: Support build variants (debug/release) via CUE inheritance -- v0.1.0
- ✓ CONF-03: Support conditional configuration based on environment variables -- v0.1.0
- ✓ COMP-01: Compile C and C++ source files using configured toolchain -- v0.1.0
- ✓ COMP-02: Track header dependencies to determine rebuild needs -- v0.1.0
- ✓ COMP-03: Support incremental builds (content-hash based cache invalidation) -- v0.1.0
- ✓ COMP-04: Support C++20 modules with compiler-driven dependency scanning -- v0.1.0
- ✓ COMP-05: Abstract compiler flags with semantic names -- v0.1.0
- ✓ DEPS-01: Link against system libraries via configuration -- v0.1.0
- ✓ DEPS-02: Build vendored source dependencies in-project -- v0.1.0
- ✓ DEPS-03: Clone and build git dependencies -- v0.1.0
- ✓ OUTP-01: Build executable binaries -- v0.1.0
- ✓ OUTP-02: Build static libraries (.a/.lib) -- v0.1.0
- ✓ OUTP-03: Build shared libraries (.so/.dylib) -- v0.1.0
- ✓ OUTP-04: Generate compile_commands.json for IDE integration -- v0.1.0
- ✓ OUTP-05: Execute builds directly (invoke compilers) -- v0.1.0
- ✓ OUTP-06: Generate Ninja build files -- v0.1.0
- ✓ DEVX-01: CLI with build, clean, and run commands -- v0.1.0
- ✓ DEVX-02: Configurable output verbosity (quiet/normal/verbose) -- v0.1.0
- ✓ DEVX-03: Display build timing for each compilation step -- v0.1.0
- ✓ PLAT-01: Support Linux with GCC and Clang toolchains -- v0.1.0
- ✓ PLAT-02: Support macOS with Clang toolchain -- v0.1.0
- ✓ PLAT-03: Support cross-compilation (build for different target than host) -- v0.1.0

#### v0.2.0 Windows + DevEx
- ✓ MSVC-01: Detect Visual Studio installations using vswhere.exe -- v0.2.0
- ✓ MSVC-02: Execute vcvarsall.bat and capture MSVC environment variables -- v0.2.0
- ✓ MSVC-03: Compile C/C++ files using cl.exe with MSVC flag syntax -- v0.2.0
- ✓ MSVC-04: Link executables using link.exe with MSVC linker flags -- v0.2.0
- ✓ MSVC-05: Create static libraries using lib.exe -- v0.2.0
- ✓ MSVC-06: Create shared libraries (DLLs) using link.exe /DLL -- v0.2.0
- ✓ MSVC-07: Generate debug symbols using /Zi or /Z7 flags -- v0.2.0
- ✓ MSVC-08: Map debug/release variants to /Od and /O2 optimization flags -- v0.2.0
- ✓ MSVC-09: Configure warning levels using /W3, /W4, /Wall flags -- v0.2.0
- ✓ MSVC-10: Support response files (@file) for long command lines -- v0.2.0
- ✓ MSVC-11: Add windows-amd64 to supported platforms -- v0.2.0
- ✓ MSVC-12: Automatically detect and configure MSVC without manual paths -- v0.2.0
- ✓ WATCH-01: Monitor source directories for file changes using fsnotify -- v0.2.0
- ✓ WATCH-02: Detect Create, Write, Remove, and Rename file events -- v0.2.0
- ✓ WATCH-03: Debounce rapid file changes (100-200ms window) -- v0.2.0
- ✓ WATCH-04: Trigger incremental rebuild when source files change -- v0.2.0
- ✓ WATCH-05: Handle Ctrl+C gracefully to stop watching -- v0.2.0
- ✓ WATCH-06: Run initial full build before starting watch loop -- v0.2.0
- ✓ WATCH-07: Filter events to only .c, .cpp, .h, .hpp files -- v0.2.0
- ✓ PROF-01: Record compilation duration for each source file -- v0.2.0
- ✓ PROF-02: Report total build time at completion -- v0.2.0
- ✓ PROF-03: List the N slowest compilation units -- v0.2.0
- ✓ PROF-04: Persist timing data to file for analysis -- v0.2.0
- ✓ PROF-05: Print human-readable timing summary at build end -- v0.2.0

### Active

#### v0.3.0 Code Quality
- [ ] REFAC-01: Extract toolchain code from internal/build into internal/toolchain package
- [ ] REFAC-02: Extract caching code from internal/build into internal/cache package
- [ ] REFAC-03: Extract profiling code from internal/build into internal/profile package
- [ ] REFAC-04: Extract watch mode code from internal/build into internal/watch package
- [ ] REFAC-05: Reduce internal/build to core orchestration (builder, executor, parallel)
- [ ] TEST-01: Add testdata project using nlohmann/json as git dependency
- [ ] TEST-02: Add testdata project using Catch2 as header-only dependency
- [ ] TEST-03: Add testdata project with multiple interdependent external libraries
- [ ] TEST-04: Add integration tests exercising the new testdata projects

### Out of Scope

- GUI/IDE -- CLI only, though generate `compile_commands.json` for editor integration
- Package manager -- Clue builds dependencies, doesn't host/distribute them
- Non-C-family languages -- focus on C/C++ and similar (Objective-C, CUDA could come later)
- Wildcard source globs -- leads to accidental inclusions; explicit source lists preferred

## Context

**Current state:** Shipped v0.2.0 with 25,930 LOC Go across ~300 files. internal/build has grown to 55 files and 14,295 lines with mixed responsibilities (compilation, linking, caching, toolchains, profiling, watch mode).

**Tech stack:** Go 1.23, CUE for configuration, xxh3 for content hashing, errgroup for parallel compilation, fsnotify for watch mode.

**Capabilities:**
- CUE-based validated configuration with schema inheritance
- Incremental builds with content-hash caching and header dependency tracking
- Parallel compilation with 3x speedup and graceful cancellation
- Cross-platform support (Linux/macOS/Windows) with GCC/Clang/MSVC toolchains
- External dependency management (vendored, git, tarball)
- IDE integration (compile_commands.json, Ninja build files)
- C++20 module detection and dependency scanning infrastructure
- Build profiling with Chrome Trace export
- Watch mode for automatic rebuilds

**Known tech debt:**
- 9 orphaned test call sites using old Verbose bool API (test maintenance)
- Module BMI compilation not fully implemented (foundation only)
- Config loading output shows in --quiet mode (minor UX issue)

## Constraints

- **Language**: Go -- matches the existing go.mod, good for CLI tools and concurrency
- **Primary toolchain**: GCC, Clang, MSVC all supported via abstraction layer
- **Config language**: CUE -- non-negotiable, core to the project's value proposition
- **Platforms**: Linux (GCC/Clang), macOS (Clang), Windows (MSVC)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| CUE for configuration | Type validation, inheritance, catches errors before build | ✓ Good |
| Clang-first for modules | Most mature C++20 module support, delegate scanning to compiler | ✓ Good |
| Both direct and generated builds | Direct for simple projects, Ninja/Make for complex/IDE integration | ✓ Good |
| Uniform dependency model | Git, tarball, vendored all share similar schema shape where sensible | ✓ Good |
| xxh3.Hash128 for content hashing | Fast, excellent distribution, crypto not needed for caching | ✓ Good |
| errgroup.WithContext for parallelism | Standard Go pattern, automatic context cancellation | ✓ Good |
| Double Ctrl+C pattern for cancellation | First graceful, second forced -- matches user expectations | ✓ Good |
| go:embed for CUE schema | Keeps schema in binary, no runtime file access needed | ✓ Good |
| Verbosity enum (Quiet/Normal/Verbose) | Three levels cleaner than boolean combinations | ✓ Good |
| Toolchain interface with 9 methods | Clean abstraction for GCC/Clang/MSVC, enables future toolchains | ✓ Good |
| MSVC-first for Windows | Most common Windows toolchain, MinGW/Clang-cl can follow | ✓ Good |
| vswhere for MSVC detection | Official Microsoft tool, handles all VS versions | ✓ Good |
| Response file threshold at 8000 chars | Safe margin under Windows 32K limit | ✓ Good |
| fsnotify for watch mode | Standard Go library, cross-platform support | ✓ Good |
| 300ms debounce for watch | Balances responsiveness with preventing duplicate builds | ✓ Good |
| Chrome Trace JSON for profiling | Standard format, works in chrome://tracing | ✓ Good |

---
*Last updated: 2026-01-29 after v0.3.0 milestone start*
