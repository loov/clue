# Feature Research: C/C++ Build Systems

**Domain:** C/C++ build systems
**Researched:** 2026-01-22 (v0.1.0), Updated 2026-01-24 (v0.2.0 additions)
**Confidence:** HIGH (based on official documentation, multiple authoritative sources)

## Executive Summary

This research surveys the feature landscape of modern C/C++ build systems (CMake, Meson, Bazel, Ninja, xmake, build2) to identify table stakes, differentiators, and anti-features for the Clue project. The C++ build ecosystem is mature but fragmented, with CMake as the de facto standard despite usability complaints. Clue's opportunity lies in combining CUE's type-safe configuration with the simplicity of newer tools like xmake while avoiding the complexity traps of CMake.

---

## v0.2.0 Feature Additions

This section covers features planned for v0.2.0: Windows MSVC support, watch mode, and build profiling.

### Windows MSVC Support

#### Table Stakes

| Feature | Description | Complexity | Notes |
|---------|-------------|------------|-------|
| **Visual Studio detection** | Use vswhere.exe to locate VS installations and vcvarsall.bat | MEDIUM | vswhere is at `%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe`; must parse JSON output |
| **Environment setup** | Execute vcvarsall.bat to set PATH, INCLUDE, LIB environment variables | MEDIUM | MSVC requires ~20 environment variables; cannot work without vcvars setup |
| **cl.exe compilation** | Invoke cl.exe with MSVC-style flags (/c, /Fo, /I, /D, etc.) | MEDIUM | Different flag syntax from GCC: forward slash prefixes, colon separators |
| **link.exe linking** | Invoke link.exe for executables and DLLs with /OUT, /LIBPATH, etc. | MEDIUM | Separate linker binary unlike GCC/Clang which use same binary |
| **lib.exe archiving** | Use lib.exe for static libraries instead of ar | LOW | Simpler than linking; /OUT flag for output |
| **Debug symbols (/Zi, /Z7)** | Generate PDB files for debugging | LOW | /Zi creates separate .pdb, /Z7 embeds in .obj |
| **Optimization flags** | Map debug/release to /Od and /O2 respectively | LOW | Direct mapping from existing semantic flags |
| **Warning levels (/W3, /W4, /Wall)** | Configure warning strictness | LOW | /W4 is roughly equivalent to -Wall -Wextra |
| **Response files (@file)** | Support long command lines via response files | MEDIUM | Windows command line limit is 32KB; response files essential |
| **windows-amd64 platform** | Add windows-amd64 to supported platforms | LOW | Extend existing Platform struct |

#### Differentiators

| Feature | Description | Complexity | Notes |
|---------|-------------|------------|-------|
| **Automatic vcvars detection** | Find and setup MSVC environment without manual configuration | HIGH | Parse vswhere JSON, locate vcvarsall.bat, capture environment |
| **Multi-version support** | Support VS 2019, 2022, and Build Tools editions | MEDIUM | vswhere can filter by version; `-latest` flag gets newest |
| **Cross-architecture (x86/x64/ARM64)** | Support x86_amd64, amd64_arm64 cross-compilation | HIGH | Different vcvars bat files per architecture |
| **Incremental linking** | Enable MSVC incremental linking for faster debug builds | LOW | /INCREMENTAL flag to link.exe |
| **Parallel compilation (/MP)** | Enable MSVC's built-in parallel compilation | LOW | /MP flag enables per-cl.exe parallelism |
| **PDB path control** | Allow customizing PDB output location | LOW | /Fd flag for compiler, /PDB for linker |
| **Edit and Continue (/ZI)** | Support Edit and Continue debugging | LOW | Requires /ZI flag; x86/x64 only |

#### Anti-features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **MSBuild integration** | Adds massive complexity; not a command-line build system | Use cl.exe/link.exe directly via command line |
| **Project file generation (.vcxproj)** | Outside scope; Visual Studio specific | Already have Ninja generation; IDE users can use compile_commands.json |
| **Windows SDK version management** | Complex versioning matrix; rarely needed | Use vcvarsall defaults; document manual override |
| **ATL/MFC support** | Niche; adds significant complexity | Document as unsupported; users can add raw flags |
| **Windows Store/UWP builds** | Different target platform; niche use case | Focus on desktop applications only |

---

### Watch Mode

#### Table Stakes

| Feature | Description | Complexity | Notes |
|---------|-------------|------------|-------|
| **Directory watching** | Monitor source directories for file changes | LOW | Use fsnotify; watch directories not individual files |
| **File change detection** | Detect Create, Write, Remove, Rename events | LOW | fsnotify provides these event types directly |
| **Event debouncing** | Coalesce rapid file changes (e.g., editor save) | MEDIUM | 100-200ms debounce window typical; prevents duplicate builds |
| **Incremental rebuild trigger** | Trigger rebuild only for changed files | LOW | Existing incremental build infrastructure handles this |
| **Graceful shutdown** | Handle Ctrl+C to stop watching cleanly | LOW | Existing signal handling can be extended |
| **Initial build** | Run full build before starting watch | LOW | Standard build invocation |
| **Source file filtering** | Only watch .c, .cpp, .h, .hpp files | LOW | Filter by extension in event handler |

#### Differentiators

| Feature | Description | Complexity | Notes |
|---------|-------------|------------|-------|
| **Smart dependency awareness** | Rebuild dependents when header changes | MEDIUM | Parse existing .d files to know what to rebuild |
| **Parallel watch + build** | Continue watching while build runs | MEDIUM | Run build in goroutine; queue changes during build |
| **Build success/failure notification** | Clear visual feedback (sound, system notification) | LOW | Optional; print colored success/failure |
| **Watch multiple directories** | Monitor src/, include/, and dependency dirs | LOW | fsnotify supports multiple watch paths |
| **Configurable debounce** | Allow tuning debounce interval | LOW | Flag: `--debounce 200ms` |
| **Exclude patterns** | Ignore build/, .git/, etc. | MEDIUM | Glob pattern matching on paths |
| **Run command on success** | Execute custom command after successful build | LOW | Similar to existing `clue run` |

#### Anti-features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Recursive subdirectory watching** | fsnotify doesn't support it natively; adds complexity | Explicitly add directories to watch list |
| **Network filesystem watching** | NFS/SMB don't support notifications | Document limitation; polling fallback too complex |
| **Hot reload / live patching** | C++ doesn't support this well; out of scope | Just rebuild and re-run |
| **Browser refresh integration** | Outside scope of C++ build system | Users can use external tools like browser-sync |

---

### Build Profiling

#### Table Stakes

| Feature | Description | Complexity | Notes |
|---------|-------------|------------|-------|
| **Per-file compilation timing** | Record duration for each source file | LOW | Already captured in CompileResult.Duration |
| **Total build time** | Report overall build duration | LOW | Simple timer around build |
| **Slowest files report** | List N slowest compilation units | LOW | Sort by duration; print top 10 |
| **Timing data persistence** | Save timing data to file for analysis | LOW | Write JSON to .clue/profile.json |
| **Human-readable summary** | Print timing summary at build end | LOW | "Build completed in 5.2s (slowest: foo.cpp 1.2s)" |

#### Differentiators

| Feature | Description | Complexity | Notes |
|---------|-------------|------------|-------|
| **Chrome Trace format** | Generate JSON viewable in chrome://tracing | MEDIUM | Standard format; good visualization |
| **Parallelism visualization** | Show which files compiled in parallel | MEDIUM | Track start/end times per file; timeline view |
| **Bottleneck detection** | Identify serialization points in build | HIGH | Analyze dependency graph for sequential chains |
| **Historical comparison** | Compare current build to previous | MEDIUM | Store history; diff timing data |
| **Compiler phase breakdown** | Frontend vs backend time (if -ftime-trace available) | HIGH | Parse Clang's JSON; integrate with our timeline |
| **Link time tracking** | Separate compilation from linking time | LOW | Already separate operations |
| **Cache hit/miss reporting** | Show how many files were rebuilt vs cached | LOW | Count cache lookups vs compiles |

#### Anti-features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Deep compiler integration** | Requires compiler modifications; not portable | Use compiler's own profiling flags (-ftime-trace) |
| **Template instantiation analysis** | Clang-specific; complex | Document how to use ClangBuildAnalyzer externally |
| **Always-on profiling** | Performance overhead; clutters output | Opt-in via `--profile` flag |
| **Network-based profiling dashboards** | Outside scope; over-engineering | JSON files work with existing tools |

---

### v0.2.0 Dependencies on Existing Features

| New Feature | Depends On | How It Uses It |
|-------------|------------|----------------|
| **Windows MSVC** | `Platform` struct | Extend with windows-amd64; add IsMSVC() method |
| **Windows MSVC** | `Toolchain` struct | New MSVC toolchain with CC=cl.exe, CXX=cl.exe, AR=lib.exe |
| **Windows MSVC** | `Compiler` | New branch for MSVC flag generation |
| **Windows MSVC** | `Linker` | New branch for link.exe invocation |
| **Windows MSVC** | Response files | New feature needed for Windows command line limits |
| **Watch Mode** | `Builder` | Trigger builds via existing Builder interface |
| **Watch Mode** | Incremental builds | Rely on cache invalidation to rebuild only changed files |
| **Watch Mode** | Signal handling | Extend existing SIGINT handling for clean shutdown |
| **Watch Mode** | Dependency tracking | Use .d files to know which sources depend on changed headers |
| **Build Profiling** | `CompileResult.Duration` | Already captures per-file timing |
| **Build Profiling** | `ParallelCompiler` | Already tracks start/end per goroutine |
| **Build Profiling** | Build output | Extend to include timing summary |

---

### v0.2.0 Feature Priority Matrix

| Feature | Priority | Effort | Rationale |
|---------|----------|--------|-----------|
| Windows MSVC (basic) | P0 | HIGH | Stated priority; unlocks Windows development |
| Watch mode (basic) | P1 | MEDIUM | High developer productivity gain |
| Build profiling (basic) | P2 | LOW | Low effort, useful diagnostics |
| MSVC auto-detection | P1 | MEDIUM | Required for good Windows DX |
| Watch debouncing | P0 | LOW | Required for correctness |
| Chrome trace output | P2 | MEDIUM | Nice visualization; existing format |
| MSVC cross-arch | P3 | HIGH | Niche use case |
| Parallelism visualization | P3 | MEDIUM | Power user feature |

---

### v0.2.0 Sources

#### Windows MSVC
- [MSVC Compiler Options (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options?view=msvc-170)
- [MSVC Compiler Command-Line Syntax (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-command-line-syntax?view=msvc-170)
- [Use the Microsoft C++ Build Tools from the command line (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line?view=msvc-170)
- [MSVC Linker Options (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/reference/linker-options?view=msvc-170)
- [/LIBPATH Linker Option (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/reference/libpath-additional-libpath?view=msvc-170)
- [Debug Information Format /Z7, /Zi, /ZI (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/reference/z7-zi-zi-debug-information-format?view=msvc-170)
- [Response Files @ Syntax (Microsoft Learn)](https://learn.microsoft.com/en-us/cpp/build/reference/at-specify-a-compiler-response-file?view=msvc-170)
- [vswhere GitHub Repository (Microsoft)](https://github.com/microsoft/vswhere)
- [Tools for detecting Visual Studio instances (Microsoft Learn)](https://learn.microsoft.com/en-us/visualstudio/install/tools-for-managing-visual-studio-instances?view=visualstudio)

#### Watch Mode
- [fsnotify GitHub Repository](https://github.com/fsnotify/fsnotify)
- [fsnotify Go Package Documentation](https://pkg.go.dev/github.com/fsnotify/fsnotify)
- [efsw C++ File Watcher (GitHub)](https://github.com/SpartanJ/efsw)
- [Ninja Build System Manual](https://ninja-build.org/manual.html)
- [Debounce Watch NPM Package](https://www.npmjs.com/package/@bscotch/debounce-watch)

#### Build Profiling
- [Introducing vcperf /timetrace (Microsoft C++ Blog)](https://devblogs.microsoft.com/cppblog/introducing-vcperf-timetrace-for-cpp-build-time-analysis/)
- [Finding build bottlenecks with C++ Build Insights (Microsoft C++ Blog)](https://devblogs.microsoft.com/cppblog/finding-build-bottlenecks-with-cpp-build-insights/)
- [Profiling Compilation Times (Adobe Lagrange)](https://opensource.adobe.com/lagrange-docs/dev/compilation-profiling/)
- [ClangBuildAnalyzer GitHub Repository](https://github.com/aras-p/ClangBuildAnalyzer)
- [Clang -ftime-trace Documentation](https://clang.llvm.org/docs/analyzer/developer-docs/PerformanceInvestigation.html)
- [time-trace: timeline / flame chart profiler for Clang (Aras' blog)](https://aras-p.info/blog/2019/01/16/time-trace-timeline-flame-chart-profiler-for-Clang/)

---

## Feature Landscape (v0.1.0 - Original Research)

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
