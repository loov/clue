# Project Research Summary

**Project:** Clue (Go-based C/C++ build system with CUE configuration)
**Domain:** Build Systems for C/C++ Compilation
**Researched:** 2026-01-22
**Confidence:** HIGH

## Executive Summary

Clue is a C/C++ build system written in Go with CUE configuration. Research across existing build systems (CMake, Meson, Bazel, Ninja, xmake, build2) reveals a mature but fragmented ecosystem where CMake dominates despite significant usability issues. The recommended approach follows proven patterns: bipartite dependency graphs (files and commands as separate node types), two-phase architecture (configuration then execution), and parallel task execution with topological sorting. Clue's unique value proposition lies in CUE's type-safe configuration validation (no other build system catches config errors before build time) and unified dependency handling (git/tarball/vendored).

The primary technical risks are cache invalidation bugs (causing stale builds or unnecessary recompilation), C++20 module dependency ordering violations (requiring two-phase scan-then-compile), and cross-platform compiler flag abstraction leakage. These are mitigated by: content-hash-based invalidation with all inputs in cache keys, following the P1689R5 standard for module dependencies, and semantic flag abstraction (e.g., `optimization: "fast"` instead of `-O2`). The Go ecosystem provides excellent support with CUE v0.15.3, fsnotify for watch mode, errgroup for parallel execution, and gonum for dependency graph operations.

Build systems follow a well-established architecture: parse configuration, build dependency DAG, perform topological sort, execute in parallel with bounded concurrency, and track discovered dependencies for incremental builds. The critical insight from Ninja is that dependency graphs should be bipartite (file nodes connect to command nodes, which connect back to file nodes), enabling efficient change propagation. Success depends on getting dependency tracking correct from the start—incorrect invalidation leads to either broken incremental builds or complete loss of build speed benefits.

## Key Findings

### Recommended Stack

Go 1.25.x with CUE v0.15.3 provides the foundation, offering static binaries, excellent concurrency primitives, and mature configuration validation. The Go ecosystem has all necessary building blocks: CUE's official Go API for config parsing/validation, fsnotify for cross-platform file watching, errgroup for bounded parallel execution, and gonum for dependency graph operations. For C++ modules support, Clang 18+ is recommended due to superior clang-scan-deps tooling and P1689 format support. Ninja 1.13.x is the optional backend target if generating build files rather than direct execution.

**Core technologies:**
- **Go 1.25.x:** Core language — current stable, excellent cross-compilation, static binaries, concurrency via goroutines
- **CUE v0.15.3:** Configuration — type validation catches errors at parse time before build starts, mature Go API with excellent error messages
- **Clang 18+:** Primary compiler target — best C++20 modules support via clang-scan-deps, P1689 format for dependency scanning
- **fsnotify v1.9.0:** Watch mode — cross-platform file monitoring for auto-rebuild, stable API, widely used
- **errgroup:** Parallel execution — bounded concurrency with context cancellation, standard library extension
- **gonum/graph/topo:** Dependency graphs — mature graph library with topological sort, deterministic ordering

### Expected Features

Build systems have well-defined table stakes that users expect as baseline. Missing any of these makes the system feel broken or incomplete. Differentiators set tools apart but aren't universally expected.

**Must have (table stakes):**
- Multi-file compilation with dependency tracking — users expect incremental builds
- Parallel compilation — single-threaded builds are unacceptable on modern hardware
- Static and shared library generation — projects need both .a/.so and .dll output
- Debug/Release configurations — different optimization levels and debug symbols
- Linux, macOS, Windows support — cross-platform is baseline expectation
- Include/link path management — abstract -I and -L flags
- Compiler detection — automatically find GCC/Clang/MSVC
- Out-of-source builds — don't pollute source tree with artifacts
- Clean target — remove build outputs
- Verbose mode — see actual compiler commands for debugging

**Should have (competitive advantage):**
- **Type-safe configuration via CUE** — primary differentiator, no other build system has this
- **Unified dependency handling** — git/tarball/vendored all work identically (user explicit requirement)
- **Watch mode / auto-rebuild** — developer experience improvement missing from most tools
- Compiler flag abstraction — portable optimization levels like xmake's approach
- C++20 modules support — important for modern C++ adoption
- compile_commands.json generation — required for LSP/IDE integration
- Minimal boilerplate — match or beat xmake's simplicity

**Defer (v2+):**
- Built-in package manager — too complex for MVP, defer to post-launch
- Remote build cache — enterprise feature, not needed for initial adoption
- IDE project generation — compile_commands.json covers most needs
- Plugin system — avoid fragmentation, prefer well-designed core features

### Architecture Approach

Build systems universally follow a two-phase architecture: configuration (parse files, validate, detect toolchains) followed by generation/execution (build DAG, sort, run/generate). The dependency graph should be bipartite with file nodes and command nodes as distinct types—this pattern from Ninja better captures build semantics where commands are out-of-date if any input changes. Topological sorting with Kahn's algorithm enables parallel execution by grouping independent tasks into "levels" that can run concurrently. Incremental builds require persisting discovered dependencies (headers found during compilation) in a deps log, with content hashing preferred over timestamps to avoid clock drift and file copy issues.

**Major components:**
1. **CUE Parser & Loader** — parse configuration files, validate against schema, decode to Go structs (uses cuelang.org/go)
2. **Dependency Graph** — bipartite DAG with file nodes and command nodes, path interning for performance, topological sort for execution order
3. **Source Scanner** — discover C++ dependencies via #include and import statements, use compiler output (-MD flag) not manual parsing
4. **Build Planner** — topological sort into parallelizable task levels, change detection for incremental builds, task queue management
5. **Parallel Executor** — worker pool with bounded concurrency (errgroup.SetLimit), context-aware cancellation, buffered output
6. **External Dependency Manager** — fetch from git/tarball/vendor, parallel fetching, local caching with integrity verification
7. **File Watcher (Watch Mode)** — fsnotify-based, watch directories not files, debounce rapid changes, handle atomic saves
8. **Backend Generator (Optional)** — generate Ninja or Make files for systems that prefer explicit build scripts

**Key architectural patterns:**
- **Bipartite graph:** File nodes point to command nodes, commands point back to files (Ninja pattern)
- **Path canonicalization/interning:** Every path maps to unique in-memory object, use pointer comparison for performance
- **Two-phase build:** Separate configuration analysis from execution, enables caching and backend generation
- **Topological levels:** Group independent tasks for parallel execution while respecting dependencies
- **Dependency logs:** Persist discovered headers for incremental builds, content-hash for invalidation

### Critical Pitfalls

Research identified six critical pitfalls that cause rewrites or fundamental failures, plus several moderate technical debt patterns.

1. **Incorrect cache invalidation** — stale builds (didn't recompile when needed) or thrashing (recompiled too much). Prevention: content-hash based keys, include ALL inputs (source, flags, compiler version, system headers), track "negative dependencies" (files that didn't exist but would affect build), test explicitly with modification and reversion
2. **C++ module dependency ordering violations** — parallel builds fail because BMI files aren't available when needed. Prevention: two-phase scan-then-compile, follow P1689R5 format, update DAG from scan results before scheduling compilation, test with module-heavy code early
3. **Parallel build race conditions** — archive corruption from simultaneous ar processes, missing dependencies allow conflicting operations. Prevention: validate declared dependencies match actual file access, use pools to limit concurrent resource-heavy operations (linking, archiving), test at high parallelism (-jN) to surface races
4. **Cross-platform compiler flag abstraction leakage** — GCC uses `-flag`, MSVC uses `/flag`; same concept has different syntax. Prevention: abstract to semantic concepts (optimization: "fast" not -O2), map internally to platform flags, test all platforms in CI from day one
5. **External dependency resolution brittleness** — git repos move/disappear, version ranges resolve differently over time, network failures break builds. Prevention: pin exact versions with content hashes, implement local caching with integrity verification, support lockfiles, allow offline builds from cache
6. **Filesystem watch mode platform inconsistencies** — inotify queue overflow on Linux, kqueue file descriptor limits on macOS, atomic save (temp file then rename) breaks per-file watches. Prevention: watch directories not files, handle "file replaced" as modification, debounce rapid changes, test with editors using atomic save

**Additional warnings:**
- Header dependency tracking is complex (precompiled headers, generated headers, non-existent headers with `__has_include`)
- CUE schema evolution requires migration path for existing configs
- Linking is often the bottleneck in large projects (use pools to limit concurrent links)
- Windows has unique challenges (file locking, long paths >260 chars, case-insensitive filesystem)

## Implications for Roadmap

Research reveals clear phase dependencies based on component architecture and pitfall timing. Foundation (config + graph) must come first, followed by basic compilation, then incremental builds with correct invalidation, then advanced features. External dependencies and C++ modules are complex enough to warrant dedicated phases.

### Phase 1: Foundation (Config & Graph)
**Rationale:** Everything depends on configuration model and dependency graph. These are the core data structures. Get them right before building on top.

**Delivers:**
- CUE configuration parsing and validation
- Basic target model (executable, static library, shared library)
- Dependency graph data structure (bipartite: file nodes + command nodes)
- Path canonicalization/interning
- Topological sort implementation

**Addresses:**
- Type-safe configuration (FEATURES: primary differentiator)
- Clear error messages with source locations
- Foundation for all subsequent phases

**Avoids:**
- Rolling own config language (use proven CUE instead)
- Implicit behavior and magic (explicit dependencies in config)

**Implementation notes:**
- Use cuelang.org/go for CUE integration
- Internal packages: internal/config, internal/graph
- Test schema validation thoroughly before proceeding

### Phase 2: Core Compilation
**Rationale:** Prove the build system can compile before adding complexity. Start with single-threaded, full rebuilds. Correctness first, speed later.

**Delivers:**
- Compiler detection (GCC, Clang, MSVC)
- Single-file compilation
- Multi-file compilation into executables
- Static library generation (ar/lib.exe)
- Linux support as primary platform
- Verbose mode (show commands being run)

**Addresses:**
- Multi-file compilation (FEATURES: table stakes)
- Compiler detection (FEATURES: table stakes)
- Executable generation (FEATURES: table stakes)

**Avoids:**
- Hardcoded compiler paths (detect via PATH)
- In-source builds (enforce out-of-source from start)
- Premature parallelization (get correctness first)

**Implementation notes:**
- Use exec.CommandContext for cancellable compiler invocation
- Internal packages: internal/toolchain, internal/exec
- Test on Linux with GCC and Clang

### Phase 3: Incremental Builds & Caching
**Rationale:** This is the CRITICAL phase. Incorrect cache invalidation is the #1 pitfall. Spend significant time getting this right with comprehensive tests.

**Delivers:**
- Header dependency scanning (use compiler -MD output)
- Content-hash based invalidation (not timestamps)
- Dependency log persistence
- Change detection (what needs rebuilding)
- Incremental rebuild correctness

**Addresses:**
- Dependency tracking (FEATURES: table stakes)
- Incremental builds (FEATURES: table stakes)

**Avoids:**
- **Incorrect cache invalidation (PITFALL #1)** — extensive testing required
- Timestamp-only invalidation (breaks with clock drift)
- Missing cache key inputs (compiler version, flags, env vars)

**Implementation notes:**
- Internal packages: internal/scanner, internal/plan
- Test suite MUST include: modify file & verify rebuild, revert & verify cache hit, change flags & verify rebuild
- Consider ccache's lessons learned for edge cases

**Research flag:** HIGH priority for additional research during this phase. Cache invalidation has many edge cases (system headers, clock changes, negative dependencies). Plan for /gsd:research-phase when implementing.

### Phase 4: Parallel Execution
**Rationale:** Now that correctness is proven, add speed via parallelization. Build on solid incremental build foundation.

**Delivers:**
- Parallel compilation with bounded concurrency
- Worker pool implementation
- Task queue with dependency-aware scheduling
- Context-aware cancellation (Ctrl+C handling)
- Buffered output (avoid interleaved compiler messages)

**Addresses:**
- Parallel compilation (FEATURES: table stakes)
- Efficient builds on modern hardware

**Avoids:**
- **Parallel build race conditions (PITFALL #3)** — validate dependencies
- Unbounded goroutines (use errgroup.SetLimit)
- Missing context cancellation (use exec.CommandContext)

**Implementation notes:**
- Use errgroup for bounded concurrency
- Test at -j1 (correctness) and -j(2*cores) (race detection)
- Implement pools for expensive operations (future linking bottleneck)

### Phase 5: Cross-Platform Support
**Rationale:** Expand from Linux to macOS and Windows. This is where compiler abstraction pays off.

**Delivers:**
- macOS support (Clang, Mach-O binaries)
- Windows support (MSVC, PE binaries)
- Compiler flag abstraction (semantic concepts not raw flags)
- Cross-compilation basics (toolchain specification)

**Addresses:**
- Platform support (FEATURES: table stakes for all three OSes)
- Compiler flag abstraction (FEATURES: competitive advantage)

**Avoids:**
- **Cross-platform flag leakage (PITFALL #4)** — abstract early
- Platform-specific code scattered throughout (centralize in toolchain package)

**Implementation notes:**
- Map semantic flags (optimization: "fast") to platform-specific flags internally
- Test on all three platforms in CI from this phase onward
- Handle Windows-specific issues: long paths, file locking, case-insensitive filesystem

### Phase 6: External Dependencies
**Rationale:** Complex enough to deserve dedicated focus. User explicitly requested unified git/tarball/vendor handling.

**Delivers:**
- Vendored dependencies (simplest case)
- Git repository dependencies (clone, fetch, checkout)
- Tarball dependencies (download, verify, extract)
- Unified dependency interface
- Dependency caching and offline mode
- Lockfile generation for reproducibility

**Addresses:**
- Unified dependency handling (FEATURES: user requirement, competitive advantage)
- Dependency management (needed for real-world projects)

**Avoids:**
- **Dependency resolution brittleness (PITFALL #5)** — pin versions, hash verification
- Synchronous fetching (fetch in parallel)
- Network-only operation (implement caching)

**Implementation notes:**
- Internal package: internal/deps (resolver, git, tarball, vendor)
- Use go-git for pure-Go git operations (no CGO)
- Content-hash verification for integrity
- Support lockfiles to record resolved versions

**Research flag:** MEDIUM priority. Dependency resolution has complex edge cases (version conflicts, diamond dependencies). May need targeted research for resolution strategies.

### Phase 7: Watch Mode
**Rationale:** Differentiating feature for developer experience. Build on solid incremental build foundation.

**Delivers:**
- File system watching via fsnotify
- Change detection and automatic rebuild
- Debouncing for rapid changes
- Graceful handling of atomic saves (temp file + rename)

**Addresses:**
- Watch mode / auto-rebuild (FEATURES: competitive advantage, user request)
- Developer experience improvement

**Avoids:**
- **Watch mode platform inconsistencies (PITFALL #6)** — test on all platforms
- Watching individual files (watch directories instead)
- Missing debounce (rapid changes cause rebuild storms)

**Implementation notes:**
- Internal package: internal/watch
- Watch directories, not individual files (fsnotify best practice)
- Test with VSCode, Vim, and other editors that use atomic save
- Handle inotify queue overflow on Linux, kqueue fd limits on macOS

### Phase 8: C++ Modules
**Rationale:** Advanced feature requiring careful implementation. Should come after core compilation is solid.

**Delivers:**
- C++20 module dependency scanning (clang-scan-deps)
- Two-phase build (scan modules, then compile)
- P1689R5 format support
- BMI file handling

**Addresses:**
- C++20 modules support (FEATURES: competitive advantage for modern C++)

**Avoids:**
- **Module dependency ordering violations (PITFALL #2)** — two-phase approach required
- Traditional dependency assumptions (modules break embarrassingly parallel model)

**Implementation notes:**
- Internal package: internal/scanner/modules.go
- Follow P1689R5 standard for module dependency information
- Scan phase must complete before compilation scheduling
- Test with module-heavy codebases early

**Research flag:** HIGH priority. C++20 modules are complex with sparse real-world documentation. Definitely plan for /gsd:research-phase when implementing this.

### Phase Ordering Rationale

The suggested ordering follows component dependency analysis from ARCHITECTURE.md:
1. **Foundation first** — config and graph are prerequisites for everything
2. **Correctness before speed** — single-threaded compilation before parallel execution
3. **Incremental builds before watch mode** — watch mode builds on change detection
4. **Core features before advanced features** — basic compilation before modules
5. **Platform expansion after core works** — Linux first, then macOS/Windows
6. **Complex features isolated** — external deps and modules in dedicated phases

This ordering also reflects pitfall avoidance strategy:
- Cache invalidation addressed in Phase 3 (early, foundational)
- Parallel races addressed in Phase 4 (after correctness proven)
- Cross-platform issues addressed in Phase 5 (centralized effort)
- Dependency brittleness addressed in Phase 6 (dedicated focus)
- Watch mode inconsistencies addressed in Phase 7 (builds on working incremental builds)
- Module ordering violations addressed in Phase 8 (advanced, isolated)

### Research Flags

**Phases needing deeper research during planning:**
- **Phase 3 (Incremental Builds & Caching):** HIGH — cache invalidation has many edge cases; plan for additional research on ccache lessons learned, ninja deps log format, content-hash strategies
- **Phase 6 (External Dependencies):** MEDIUM — dependency resolution strategies (especially version conflict handling) may need targeted research; check how cargo/npm handle this
- **Phase 8 (C++ Modules):** HIGH — C++20 modules implementation details are complex; will definitely need /gsd:research-phase for P1689R5 format details, clang-scan-deps usage, BMI file handling

**Phases with standard patterns (skip deep research):**
- **Phase 1 (Foundation):** Config parsing and DAG structures are well-documented in CUE docs and graph theory
- **Phase 2 (Core Compilation):** Compiler invocation via exec is straightforward Go pattern
- **Phase 4 (Parallel Execution):** errgroup usage is well-documented, topological sort is standard algorithm
- **Phase 5 (Cross-Platform):** Compiler differences are well-documented (Clang, GCC, MSVC docs)
- **Phase 7 (Watch Mode):** fsnotify has excellent documentation and examples

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All recommended libraries verified via pkg.go.dev with recent publication dates; CUE v0.15.3, fsnotify v1.9.0, errgroup, gonum all confirmed available and maintained |
| Features | HIGH | Based on comprehensive analysis of CMake, Meson, Bazel, xmake, build2, Ninja official documentation; feature categorization matches industry consensus on table stakes vs differentiators |
| Architecture | HIGH | Patterns verified from authoritative sources: Ninja (AOSA book chapter), CMake architecture, Bazel concepts; bipartite graph and two-phase build are proven industry patterns |
| Pitfalls | HIGH | Cache invalidation, module ordering, parallel races, cross-platform issues all cross-referenced from multiple sources (ccache debugging, CMake docs, MSVC docs, fsnotify limitations) |

**Overall confidence:** HIGH

Research is based on authoritative sources (official documentation, AOSA book chapters, standardization documents like P1689R5) cross-referenced with practical experience reports (6-month build system comparison, ccache debugging posts). The recommended stack is conservative and proven rather than experimental. Architecture patterns come from production build systems handling million-line codebases.

### Gaps to Address

While research confidence is high, several areas need validation during implementation:

- **CUE schema evolution strategy** — no established pattern for versioning build system schemas; will need to develop migration approach as schema evolves
- **Go-specific file locking for Windows** — standard library doesn't provide cross-platform file locking; need to evaluate golang.org/x/sys/windows or third-party alternatives during Windows implementation
- **Optimal debounce timing for watch mode** — research suggests 100-500ms range but optimal value depends on typical project compile times; will need tuning based on real-world testing
- **C++ module scanning performance** — clang-scan-deps is known to work but performance characteristics at scale are not well-documented; may need optimization during Phase 8
- **Dependency resolution for version conflicts** — while strategies exist (cargo/npm patterns), specific approach for C++ dependencies (which lack semantic versioning discipline) needs design during Phase 6

These gaps are manageable and can be addressed during respective phase planning with targeted research (/gsd:research-phase) or iterative implementation.

## Sources

### Primary (HIGH confidence)
- **STACK.md** — CUE Go packages (pkg.go.dev), fsnotify GitHub, errgroup documentation, Clang Standard C++ Modules documentation, gonum/graph/topo, go-git; all verified with version numbers and recent publication dates
- **FEATURES.md** — Official documentation for CMake 4.2, Meson 1.10, Bazel 9.0, xmake 3.0, build2, Ninja 1.13; comparative analyses from authoritative sources
- **ARCHITECTURE.md** — The Performance of Open Source Software - Ninja chapter (AOSA book), The Architecture of Open Source Applications - CMake chapter, Bazel Dependencies Documentation, Ninja Manual, CMake C++ Modules Documentation, CUE Go Integration
- **PITFALLS.md** — ccache debugging documentation, CMake C++ Modules docs, P1689R5 standard, MSVC /scanDependencies docs, fsnotify GitHub limitations, Ninja performance analysis, Clang MSVC compatibility docs

### Secondary (MEDIUM confidence)
- Meson Build System Overview, Buck performance documentation, Go Project Layout conventions, Fuchsia Ninja documentation
- Comparative analyses: "Choosing a Build System for C++", "6 Months of Testing C++ Build Systems", "CMake vs Meson Real Life Comparison"
- Feature-specific: "The State of C++ Modules in 2025", "C++ Package Managers Roundup"

### Tertiary (LOW confidence - validate during implementation)
- WebSearch results for logging library comparisons, CLI framework comparisons
- Gradle incremental build patterns (different domain, may not transfer directly)
- Community blog posts on build optimization techniques

---
*Research completed: 2026-01-22*
*Ready for roadmap: yes*
