# Clue

## What This Is

A Go-based build system for C, C++, and similar languages that uses CUE as its configuration language. Clue replaces the complexity of CMake/Make with declarative, validated configuration that handles dependencies cleanly — whether system packages, vendored source, git repos, or tarballs. Ships with incremental builds, parallel compilation, and IDE integration out of the box.

## Core Value

Minimal configuration for common cases, with CUE's type system catching config errors before build time — not during.

## Requirements

### Validated

- ✓ CONF-01: Parse CUE configuration files with schema validation — v0.1.0
- ✓ CONF-02: Support build variants (debug/release) via CUE inheritance — v0.1.0
- ✓ CONF-03: Support conditional configuration based on environment variables — v0.1.0
- ✓ COMP-01: Compile C and C++ source files using configured toolchain — v0.1.0
- ✓ COMP-02: Track header dependencies to determine rebuild needs — v0.1.0
- ✓ COMP-03: Support incremental builds (content-hash based cache invalidation) — v0.1.0
- ✓ COMP-04: Support C++20 modules with compiler-driven dependency scanning — v0.1.0
- ✓ COMP-05: Abstract compiler flags with semantic names — v0.1.0
- ✓ DEPS-01: Link against system libraries via configuration — v0.1.0
- ✓ DEPS-02: Build vendored source dependencies in-project — v0.1.0
- ✓ DEPS-03: Clone and build git dependencies — v0.1.0
- ✓ OUTP-01: Build executable binaries — v0.1.0
- ✓ OUTP-02: Build static libraries (.a/.lib) — v0.1.0
- ✓ OUTP-03: Build shared libraries (.so/.dylib) — v0.1.0
- ✓ OUTP-04: Generate compile_commands.json for IDE integration — v0.1.0
- ✓ OUTP-05: Execute builds directly (invoke compilers) — v0.1.0
- ✓ OUTP-06: Generate Ninja build files — v0.1.0
- ✓ DEVX-01: CLI with build, clean, and run commands — v0.1.0
- ✓ DEVX-02: Configurable output verbosity (quiet/normal/verbose) — v0.1.0
- ✓ DEVX-03: Display build timing for each compilation step — v0.1.0
- ✓ PLAT-01: Support Linux with GCC and Clang toolchains — v0.1.0
- ✓ PLAT-02: Support macOS with Clang toolchain — v0.1.0
- ✓ PLAT-03: Support cross-compilation (build for different target than host) — v0.1.0

### Active

#### Current Milestone: v0.2.0 — Windows, Watch, Profiling

**Goal:** Add Windows MSVC support as priority, plus watch mode and build profiling for developer experience.

**Target features:**
- Windows platform support with MSVC toolchain (cl.exe, link.exe)
- Watch mode for automatic rebuilds on file changes
- Build profiling to identify compilation bottlenecks

### Out of Scope

- GUI/IDE — CLI only, though generate `compile_commands.json` for editor integration
- Package manager — Clue builds dependencies, doesn't host/distribute them
- Non-C-family languages — focus on C/C++ and similar (Objective-C, CUDA could come later)
- Wildcard source globs — leads to accidental inclusions; explicit source lists preferred

## Context

**Current state:** Shipped v0.1.0 with 21,932 LOC Go across 282 files.

**Tech stack:** Go 1.23, CUE for configuration, xxh3 for content hashing, errgroup for parallel compilation.

**Capabilities:**
- CUE-based validated configuration with schema inheritance
- Incremental builds with content-hash caching and header dependency tracking
- Parallel compilation with 3x speedup and graceful cancellation
- Cross-platform support (Linux/macOS) with GCC/Clang toolchains
- External dependency management (vendored, git, tarball)
- IDE integration (compile_commands.json, Ninja build files)
- C++20 module detection and dependency scanning infrastructure

**Known tech debt:**
- 9 orphaned test call sites using old Verbose bool API (test maintenance)
- Module BMI compilation not fully implemented (foundation only)
- Config loading output shows in --quiet mode (minor UX issue)

## Constraints

- **Language**: Go — matches the existing go.mod, good for CLI tools and concurrency
- **Primary toolchain**: Clang first, architecture supports adding gcc/MSVC later
- **Config language**: CUE — non-negotiable, core to the project's value proposition
- **Platforms**: Linux (GCC/Clang), macOS (Clang) in v0.1.0; Windows (MSVC) planned

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| CUE for configuration | Type validation, inheritance, catches errors before build | ✓ Good |
| Clang-first for modules | Most mature C++20 module support, delegate scanning to compiler | ✓ Good |
| Both direct and generated builds | Direct for simple projects, Ninja/Make for complex/IDE integration | ✓ Good |
| Uniform dependency model | Git, tarball, vendored all share similar schema shape where sensible | ✓ Good |
| xxh3.Hash128 for content hashing | Fast, excellent distribution, crypto not needed for caching | ✓ Good |
| errgroup.WithContext for parallelism | Standard Go pattern, automatic context cancellation | ✓ Good |
| Double Ctrl+C pattern for cancellation | First graceful, second forced — matches user expectations | ✓ Good |
| go:embed for CUE schema | Keeps schema in binary, no runtime file access needed | ✓ Good |
| Verbosity enum (Quiet/Normal/Verbose) | Three levels cleaner than boolean combinations | ✓ Good |

---
*Last updated: 2026-01-24 after v0.2.0 milestone start*
