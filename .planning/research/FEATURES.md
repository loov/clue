# Feature Research: C/C++ Build Systems

**Domain:** C/C++ build systems
**Researched:** 2026-01-22
**Confidence:** HIGH (based on official documentation, multiple authoritative sources)

## Executive Summary

This research surveys the feature landscape of modern C/C++ build systems (CMake, Meson, Bazel, Ninja, xmake, build2) to identify table stakes, differentiators, and anti-features for the Clue project. The C++ build ecosystem is mature but fragmented, with CMake as the de facto standard despite usability complaints. Clue's opportunity lies in combining CUE's type-safe configuration with the simplicity of newer tools like xmake while avoiding the complexity traps of CMake.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that users consider baseline. Missing any of these means the build system feels incomplete or broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Multi-file compilation** | Basic functionality - compile multiple .c/.cpp files into executables/libraries | Low | Core requirement |
| **Dependency tracking** | Incremental builds require knowing what changed | Medium | Header dependency scanning essential |
| **Parallel compilation** | Single-threaded builds unacceptable on modern hardware | Medium | All major tools do this (-j flag for make, default for Ninja) |
| **Static/shared library support** | Projects need both types | Low | .a/.lib and .so/.dll/.dylib generation |
| **Debug/Release configurations** | Every project needs these variants | Low | Different optimization flags, debug symbols |
| **Include path management** | Projects have internal/external headers | Low | -I flag abstraction |
| **Link library specification** | External dependencies need linking | Low | -L and -l flag abstraction |
| **Compiler detection** | Must find GCC/Clang/MSVC automatically | Medium | Platform-specific toolchain discovery |
| **Out-of-source builds** | Don't pollute source tree with build artifacts | Low | Separate build directory support |
| **Executable generation** | Primary output for most projects | Low | Core requirement |
| **Clean target** | Remove build artifacts | Low | Standard hygiene |
| **Install target** | Deploy to system paths | Medium | Prefix handling, DESTDIR support |

### Table Stakes - Platform Support

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Linux support** | Primary development platform | Low | GCC/Clang, ELF binaries |
| **macOS support** | Common development platform | Medium | Clang, Mach-O, frameworks |
| **Windows support** | Enterprise requirement | High | MSVC differs significantly from GCC/Clang |
| **Cross-compilation support** | Embedded, mobile, different architectures | High | Toolchain files, sysroot handling |

### Table Stakes - Developer Experience

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Error messages with source location** | Debug build failures | Low | Show file:line for issues |
| **Verbose mode** | See actual compiler commands | Low | Essential for debugging build issues |
| **Dry-run mode** | Preview what would be built | Low | Confidence before long builds |
| **Incremental builds** | Only rebuild what changed | Medium | Timestamp or content-based tracking |

---

### Differentiators (Competitive Advantage)

Features that set a build system apart. Not expected by default, but valued when present.

| Feature | Value Proposition | Complexity | Who Has It | Clue Opportunity |
|---------|-------------------|------------|------------|------------------|
| **Type-safe configuration** | Catch config errors before build time | Medium | None (Clue unique via CUE) | **Primary differentiator** |
| **Minimal boilerplate** | Less time writing build files | Low | xmake, Meson | Match or beat xmake simplicity |
| **C++20 modules support** | Modern C++ feature | High | CMake 3.28+, xmake, build2 | Important for modern projects |
| **Unified dependency handling** | git/tarball/vendored all work same | Medium | build2, xmake | User explicitly wants this |
| **Watch mode / auto-rebuild** | Faster iteration cycles | Medium | Not native to most | User explicitly wants this |
| **Compiler flag abstraction** | Portable optimization/warning levels | Medium | xmake (best), CMake (partial) | Valuable DX improvement |
| **Built-in package manager** | No external tool needed | High | xmake, build2 | Nice to have, not MVP |
| **Remote build cache** | Shared cache across team/CI | High | Bazel (native), CMake (icecream) | Enterprise feature |
| **Hermetic builds** | Reproducible regardless of host | High | Bazel | Important for reliability |
| **IDE project generation** | VS/Xcode/etc projects | Medium | CMake (excellent), Meson | Important for adoption |
| **Test integration** | Run tests from build system | Low | All (CTest, Meson test) | Standard expectation |
| **Compile commands JSON** | LSP/IDE integration | Low | CMake, Meson, xmake | Essential for modern tooling |

### Differentiators - xmake's Unique Strengths (Reference)

xmake stands out for features Clue should consider matching:

| Feature | xmake Approach | Clue Consideration |
|---------|----------------|-------------------|
| **Abstraction layer** | OpenMP, fast-math, optimization level work across compilers | Very valuable - abstract away compiler differences |
| **Multiple package sources** | vcpkg, conan, system packages all accessible | Consider for Phase 2+ |
| **All-in-one** | Build + generate + package manager | Focus on build first |

### Differentiators - build2's Unique Strengths (Reference)

| Feature | build2 Approach | Clue Consideration |
|---------|-----------------|-------------------|
| **Hermetic by default** | Precise change detection | Good goal for correctness |
| **No external deps** | Pure C++, no Python/Java | Go binary is similarly portable |
| **Package repository** | cppget.org ecosystem | Not MVP priority |

---

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem useful but cause more problems than they solve. Deliberately NOT build these.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Auto-dependency detection** | "Build with feature if dep found" | Non-reproducible builds, magic behavior | Explicit dependency declaration with clear errors |
| **Wildcard source globs** | "Add all .cpp files automatically" | Non-deterministic builds when files added, Meson explicitly forbids this | Explicit file lists or directory-based conventions |
| **Turing-complete config language** | "I need complex logic" | Unmaintainable configs, debugging nightmares (CMake's problem) | CUE's constrained language is a feature |
| **GUI configuration** | "Interactive setup" | State not in version control, reproducibility issues | Good defaults + simple config files |
| **Automatic flag inference** | "Detect optimal flags" | Host-dependent binaries, cross-compile breaks | Explicit optimization profiles |
| **Multi-generator support** | "Generate Make AND Ninja AND VS" | Complexity explosion, testing burden | Pick one backend (Ninja) and do it well |
| **Legacy compatibility modes** | "Support old CMake syntax" | Maintenance burden, confusion | Clean break with good migration docs |
| **Plugin ecosystem** | "Extensibility for everything" | Fragmentation, version hell | Core features done right |

### Anti-Features - Common Build System Mistakes

| Mistake | Examples | Impact | Clue Should |
|---------|----------|--------|-------------|
| **Rolling own language** | CMake's scripting | Poor DX, learning curve | Use CUE (proven language) |
| **Implicit behavior** | Auto-deps, magic variables | Debugging nightmares | Explicit is better than implicit |
| **Configuration complexity** | CMake toolchain files | High barrier to cross-compile | Simple toolchain specification |
| **Poor error messages** | "Target not found" without context | User frustration | Invest in error quality |
| **Mixing concerns** | Build logic + package management + deployment | Bloat, bugs | Do build well first |

---

## Feature Dependencies

```
Core Foundation (Phase 1)
    |
    +-- Multi-file compilation
    |       |
    |       +-- Dependency tracking (requires file graph)
    |       |       |
    |       |       +-- Incremental builds (requires dep tracking)
    |       |
    |       +-- Parallel compilation (requires file independence analysis)
    |
    +-- Compiler detection
    |       |
    |       +-- Platform support (Linux -> macOS -> Windows)
    |               |
    |               +-- Cross-compilation (requires toolchain abstraction)
    |
    +-- Library support (static/shared)

Configuration (Phase 2)
    |
    +-- Debug/Release variants
    |       |
    |       +-- Custom build types (requires variant system)
    |
    +-- Include/link path management
    |
    +-- Preprocessor defines
            |
            +-- Conditional compilation via env vars (user request)

Dependencies (Phase 3)
    |
    +-- Vendored/local deps (simplest)
    |       |
    |       +-- Git submodule deps (requires git integration)
    |       |       |
    |       |       +-- Tarball deps (requires download/extract)
    |       |               |
    |       |               +-- Unified dep interface (user request)
    |
    +-- System library detection (pkg-config integration)

Advanced (Phase 4+)
    |
    +-- C++ modules (requires DAG analysis, complex)
    |
    +-- Watch mode (requires file system events)
    |
    +-- Compile commands JSON (requires command capture)
    |
    +-- Test integration (requires test discovery)
```

---

## Competitor Feature Analysis

### Feature Matrix

| Feature | CMake | Meson | Bazel | xmake | build2 | Ninja |
|---------|-------|-------|-------|-------|--------|-------|
| **Learning curve** | High | Low | Very High | Low | Medium | N/A (low-level) |
| **Parallel builds** | Yes | Yes | Yes | Yes | Yes | Yes (core strength) |
| **Incremental builds** | Yes | Yes | Excellent | Yes | Yes | Yes |
| **C++ modules** | Yes (3.28+) | Experimental | Yes | Yes | Yes | N/A |
| **Cross-compilation** | Yes (toolchain files) | Yes (cross files) | Yes (platforms) | Yes (simple SDK) | Yes | N/A |
| **Windows/MSVC** | Excellent | Good | Improved | Good | Good | Yes |
| **Package management** | External (vcpkg/conan) | Wrap files | Bzlmod | Built-in | Built-in | N/A |
| **IDE generation** | Excellent | Good | Limited | Yes | Limited | N/A |
| **Config language** | CMake (Turing-complete) | Python-like DSL | Starlark | Lua | Declarative | None |
| **Type safety in config** | None | None | Some (Starlark) | None | None | N/A |
| **Hermetic builds** | No | No | Yes | No | Yes | N/A |
| **Remote caching** | No (external) | No | Yes | No | No | N/A |
| **Watch mode** | No | No | No | No | No | No |
| **Compiler abstraction** | Limited | Some | Limited | Excellent | Some | N/A |
| **Adoption** | Dominant | Growing | Enterprise | Growing | Niche | Backend only |

### Detailed Competitor Notes

**CMake (v4.2, 2026)**
- De facto standard, ~40% of open source C++ projects
- Excellent IDE integration (VS, CLion, VSCode)
- C++ modules support mature in 4.0
- Pain points: Complex language, poor error messages, verbose configs
- Generates: Make, Ninja, VS, Xcode

**Meson (v1.10, 2026)**
- Fast, user-friendly, Python-like syntax
- Generates Ninja only (simplicity)
- Used by GNOME, Git 2.48+
- Pain points: No wildcard globs (by design), Windows shell requirement
- Experimental C++ modules and `import std` support

**Bazel (v9.0, 2025)**
- Designed for massive codebases (Google scale)
- Hermetic, reproducible builds
- Remote execution and caching
- Pain points: High complexity, steep learning curve, poor Windows history
- C++ rules now in separate rules_cc module

**xmake (v3.0, 2026)**
- Lua-based, all-in-one (build + package manager)
- Best compiler flag abstraction
- Strong C++ modules support
- Pain points: Smaller community, less IDE integration
- Can also generate CMake/Meson/VS projects

**build2 (current)**
- Cargo-like experience for C++
- Built-in package manager (cppget.org)
- Hermetic by default, no external deps
- Pain points: Smaller ecosystem, learning curve
- Excellent C++ modules support with GCC

**Ninja (v1.13)**
- Low-level build executor only
- Fastest incremental builds
- Not meant for direct use - needs generator
- Used by Chrome, LLVM, many via CMake/Meson

---

## MVP Feature Prioritization for Clue

Based on user requirements and competitive analysis:

### Phase 1 - Foundation (Must Work)
1. Multi-file compilation (C and C++)
2. Static and shared library targets
3. Executable targets
4. Dependency tracking (header scanning)
5. Parallel compilation
6. Incremental builds
7. Debug/Release configurations
8. Linux support (primary platform)
9. Basic CUE configuration validation

### Phase 2 - Usability (Must Be Pleasant)
1. Compiler flag abstraction (optimization levels, warnings)
2. Include/link path management
3. Preprocessor defines from config
4. Environment variable conditional compilation
5. macOS support
6. compile_commands.json generation
7. Verbose/dry-run modes

### Phase 3 - Dependencies (User Request)
1. Vendored dependencies
2. Git repository dependencies
3. Tarball dependencies
4. Unified dependency interface
5. System library detection (pkg-config)

### Phase 4 - Advanced (Competitive Edge)
1. C++ modules support
2. Watch mode / auto-rebuild
3. Windows/MSVC support
4. Cross-compilation
5. Test integration

### Defer (Post-MVP)
- Built-in package manager (too complex)
- Remote caching (enterprise feature)
- IDE project generation (compile_commands.json covers most needs)
- Plugin system (avoid fragmentation)

---

## Clue's Unique Value Proposition

Based on this analysis, Clue's differentiators should be:

1. **Type-safe configuration (CUE)** - No other build system catches config errors before build time
2. **Minimal boilerplate** - Match or beat xmake's simplicity
3. **Unified dependency handling** - git/tarball/vendored all work the same way
4. **Compiler flag abstraction** - Portable optimization/warning levels like xmake
5. **Watch mode** - Developer experience feature missing from most tools
6. **Single binary** - Like build2, no Python/Java dependencies

### What NOT to Compete On

- Ecosystem size (CMake has too much momentum)
- Enterprise scale (Bazel's niche)
- Package repository (build2/xmake already doing this)
- IDE project generation (compile_commands.json is sufficient)

---

## Sources

### Official Documentation
- [CMake Features](https://cmake.org/features/)
- [Meson Build System](https://mesonbuild.com/)
- [Bazel C++ Documentation](https://bazel.build/docs/bazel-and-cpp)
- [xmake Official Site](https://xmake.io/)
- [build2 Build Toolchain](https://www.build2.org/)
- [Ninja Build System](https://ninja-build.org/)

### Comparative Analysis
- [Choosing a Build System for C++](https://gist.github.com/MangaD/26ef92a1e1efd967c3e0188dc0591e83)
- [6 Months of Testing C++ Build Systems](https://keasigmadelta.com/blog/6-months-of-testing-c-build-systems-heres-what-you-need-to-know/)
- [CMake vs Meson Real Life Comparison](https://keasigmadelta.com/blog/cmake-vs-meson-a-real-life-comparison-with-actual-code/)
- [Performance Showdown: Building Real-World C++ Projects](https://slaptijack.com/programming/best-build-system-for-cpp-performance.html)
- [How to Compile C++ in 2025](https://sysdev.me/2025/01/20/how-to-compile-c-in-2025-bazel-or-cmake/)

### Feature-Specific Sources
- [The State of C++ Modules in 2025](https://tech-champion.com/programming/cpp/the-state-of-c-modules-in-2025-production-ready-with-cmake-4-0/)
- [C++ Package Managers Roundup](https://moderncppdevops.com/pkg-mngr-roundup/)
- [The State of C++ Package Management](https://twdev.blog/2024/09/cpp_pkgmng3/)
- [Best Practices for Build Options (Meson)](https://blogs.gnome.org/mcatanzaro/2022/07/15/best-practices-for-build-options/)

### Hot Reload / Watch Mode
- [jet-live C++ Hot Reload](https://github.com/ddovod/jet-live)
- [Visual Studio Hot Reload for C++](https://learn.microsoft.com/en-us/visualstudio/debugger/hot-reload)

### Version Information
- CMake 4.2.2 (January 2026)
- Meson 1.10.1 (January 2026)
- Bazel 9.0 (Late 2025)
- xmake 3.0.6 (January 2026)
