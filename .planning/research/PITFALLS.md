# Pitfalls Research

**Domain:** Build systems (Go implementation for C/C++)
**Researched:** 2026-01-22
**Confidence:** HIGH (multiple authoritative sources cross-referenced)

## Critical Pitfalls

These mistakes cause rewrites, major delays, or fundamental architectural issues.

### Pitfall 1: Incorrect Incremental Build Cache Invalidation

**What goes wrong:** Build system fails to recompile when it should (stale artifacts) or recompiles too much (thrashing). Users get corrupted builds or lose incremental build benefits entirely.

**Why it happens:**
- Dependency tracking misses header changes (especially transitive headers)
- Timestamp-based invalidation breaks with clock drift, timezone changes, or file copy operations
- Cache keys don't include all inputs (compiler flags, environment variables, toolchain version)
- "Direct mode" caching doesn't detect when a NEW header file would have been included if it existed

**How to avoid:**
- Use content hashing, not timestamps, as primary invalidation signal
- Include ALL inputs in cache key: source content, compiler version, flags, system headers
- Track "negative dependencies" - files that didn't exist but would affect compilation if created
- Test cache correctness explicitly: modify file, verify rebuild; revert, verify cache hit
- Consider ccache's lessons: set `CCACHE_NODIRECT=true` during testing to catch direct-mode misses

**Warning signs:**
- "Clean build works, incremental build is broken"
- Different results from same source depending on build history
- Users reporting "I have to clean rebuild every time"
- Spurious rebuilds after unrelated changes

**Phase to address:** Core Compilation & Caching (early phase - foundational)

---

### Pitfall 2: C++ Module Dependency Scanning Order Violations

**What goes wrong:** Parallel builds fail intermittently because module compilation order isn't respected. Binary Module Interface (BMI) files aren't available when needed.

**Why it happens:**
- C++20 modules break the "embarrassingly parallel" compilation model
- Source files must be scanned BEFORE compilation to discover `import` statements
- Build graph must be dynamically updated based on scan results
- Traditional dependency tracking (Makefiles, .d files) can't express module dependencies

**How to avoid:**
- Implement two-phase build: scan phase (extract module dependencies) then compile phase
- Follow P1689R5 format for dependency information (standardized JSON format)
- Ensure scan results feed into dependency graph before scheduling compilation
- Test with module-heavy codebases early
- Consider CMake's approach: collate per-source scan results to infer ordering

**Warning signs:**
- "error: module 'X' not found" in parallel builds but not serial builds
- Race conditions that only appear at high parallelism
- Build succeeds after retry without changes

**Phase to address:** Module Support (dedicated phase after basic compilation works)

---

### Pitfall 3: Parallel Build Race Conditions from Missing Dependencies

**What goes wrong:** Builds fail intermittently. Same source produces different results. Archive files get corrupted when multiple `ar` processes write simultaneously.

**Why it happens:**
- Dependencies declared in build config don't match actual file dependencies
- Multiple targets produce same output file without synchronization
- Build rules read/write shared resources without proper ordering
- Missing dependencies allow parallel execution of conflicting operations

**How to avoid:**
- Sandbox each build step (like Bazel) - fail if undeclared files are accessed
- Validate that declared dependencies match actual file access patterns
- Use file locks for shared resources (archives, databases)
- Implement "pool" mechanism (like Ninja) to limit concurrent resource-heavy operations
- Test builds at both high parallelism (-j1 for correctness, -jN for races)

**Warning signs:**
- Build failures that disappear on retry
- "File not found" errors for files that clearly exist
- Corrupt archive files or binaries
- Works on developer machine, fails in CI (different timing)

**Phase to address:** Parallel Execution (after single-threaded build is solid)

---

### Pitfall 4: Cross-Platform Compiler Flag Abstraction Leakage

**What goes wrong:** Build configuration that works on one platform fails mysteriously on another. Users must learn three different flag syntaxes (GCC/Clang/MSVC).

**Why it happens:**
- GCC/Clang use `-flag` syntax, MSVC uses `/flag` syntax
- Same concept has different flags: `-O2` vs `/O2`, `-std=c++20` vs `/std:c++20`
- ABI differences between compilers (e.g., `long double` size)
- Preprocessor defines differ (`__GNUC__` vs `_MSC_VER`)
- Two-phase template lookup works differently (MSVC defers to instantiation)

**How to avoid:**
- Abstract flags to semantic concepts: `optimization: "fast"` not `-O2`
- Map concepts to platform-specific flags internally
- Test on all three platforms in CI from day one
- Document which features have platform-specific behavior
- Consider Clang's approach: `-fms-compatibility` for Windows targeting

**Warning signs:**
- Users copy-pasting compiler flags directly into config
- "Works on Linux, fails on Windows" reports
- Template errors that only appear on one compiler

**Phase to address:** Compiler Abstraction (early - before users depend on flag format)

---

### Pitfall 5: External Dependency Resolution Brittleness

**What goes wrong:** Builds fail because external dependency changed, disappeared, or conflicts with another dependency. Network failures break builds entirely.

**Why it happens:**
- Git repositories move, get deleted, or force-push breaking changes
- Version ranges resolve to incompatible versions over time
- No offline/cached fallback for network dependencies
- Diamond dependency problem: A needs C@1.0, B needs C@2.0
- Mixing dependency sources (git, tarball, system package) with inconsistent behavior

**How to avoid:**
- Pin exact versions with content hashes, not just tags/branches
- Implement local caching with integrity verification
- Support lockfiles that record resolved dependency graph
- Detect and report version conflicts early, before compilation
- Allow offline builds from cached dependencies
- Consider vendoring strategy for reproducibility

**Warning signs:**
- "Build worked yesterday, fails today" without source changes
- Network timeouts breaking CI
- Different machines resolving different dependency versions

**Phase to address:** Dependency Management (dedicated phase with careful design)

---

### Pitfall 6: Filesystem Watch Mode Platform Inconsistencies

**What goes wrong:** Watch mode misses file changes, reports phantom changes, or crashes when watching large directories.

**Why it happens:**
- inotify (Linux): queue overflow under high event rate; resource limits
- kqueue (macOS/BSD): requires file descriptor per watched file; doesn't scale
- FSEvents (macOS): coarse-grained; misses rapid changes
- ReadDirectoryChangesW (Windows): different event semantics
- Editors use atomic save (write to temp, rename) which breaks per-file watches
- Network filesystems emit no events

**How to avoid:**
- Watch directories, not individual files
- Handle "file replaced" (atomic save) as modification event
- Implement graceful degradation to polling when native watch fails
- Set appropriate debounce/coalesce window for rapid changes
- Test with editors that use atomic save (VSCode, Vim, etc.)
- Warn users about network filesystem limitations

**Warning signs:**
- "Watch mode doesn't detect my changes" on specific platforms
- High CPU usage during idle watch
- "Too many open files" errors on macOS/BSD

**Phase to address:** Watch Mode (late phase after core build works)

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term maintenance burden.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Timestamp-only invalidation | Simple to implement | Breaks with clock drift, file copies, CI caching | Never for primary cache key; OK as optimization hint |
| Shell-out for everything | Quick prototyping | Platform differences, quoting hell, performance | Early prototyping only; replace with native calls |
| Global mutable state | Easy data sharing | Parallelism bugs, test pollution, hard to reason about | Never; use explicit parameter passing |
| String-based flag handling | Flexible | Injection vulnerabilities, parsing bugs | OK for user-provided raw flags; sanitize carefully |
| Implicit current directory | Less typing | Breaks when invoked from different paths | Never; always use absolute or explicitly relative paths |
| Single monolithic cache file | Simple implementation | Corruption affects all projects, grows unbounded | Temporary; split by project/target early |
| Hardcoded compiler paths | Works on dev machine | Fails when compiler location differs | Never; always discover or configure compiler location |
| Synchronous dependency fetch | Simple control flow | Blocks build start; can't parallelize fetches | Acceptable for MVP; async/parallel for production |

---

## Performance Traps

Issues that don't break builds but make them unacceptably slow.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Linking bottleneck | Compilation fast, linking takes minutes | Incremental linking, `pool` to limit concurrent links, split-dwarf for debug | Large binaries (>100MB), debug builds |
| Dependency file parsing | Slow incremental build startup | Binary cache of parsed deps (like Ninja's .ninja_deps) | Projects with >10K source files |
| Over-broad cache invalidation | Rebuilds everything when one header changes | Fine-grained dependency tracking, PCH strategy | Header-heavy C++ projects |
| Sequential dependency resolution | Build doesn't start until all deps fetched | Parallel fetching, incremental availability | Many external dependencies |
| Repeated header parsing | Same header parsed thousands of times | Precompiled headers, modules, or header caching | STL-heavy code, Boost users |
| Graph construction overhead | Noticeable delay before "Compiling..." | Lazy graph construction, only resolve what's needed | Monorepos with many targets |
| Excessive file stat calls | Slow on network drives, Windows | Batch stat operations, cache results during single build | Network filesystems, Windows |
| Memory-hungry link phase | OOM during link | Limit concurrent links via pool, use `thin` archives | Debug builds, address sanitizer |

---

## "Looks Done But Isn't" Checklist

Features that appear complete in happy-path testing but fail in real-world usage.

### Compilation

- [ ] Does it handle spaces in file paths?
- [ ] Does it handle Unicode in file paths (especially Windows)?
- [ ] Does it handle symlinks correctly (both in sources and dependencies)?
- [ ] Does it detect when compiler version changes?
- [ ] Does it work when source and build directories are on different filesystems?
- [ ] Does it handle read-only source directories?
- [ ] Does it work when build directory is on a network mount?

### Caching

- [ ] Does cache invalidation work when system headers change (e.g., macOS SDK update)?
- [ ] Does it handle clock going backward (VM snapshots, NTP corrections)?
- [ ] Does it invalidate when environment variables affecting compilation change?
- [ ] Does it work when cache is on different machine (shared cache scenario)?
- [ ] Does cache eviction work correctly under disk pressure?

### Cross-Platform

- [ ] Do line ending differences (CRLF vs LF) cause spurious rebuilds?
- [ ] Does it handle case-insensitive filesystems (Windows, macOS default)?
- [ ] Does it work with long paths on Windows (>260 characters)?
- [ ] Does it handle Windows file locking (can't delete open files)?
- [ ] Does it work under WSL accessing Windows paths?

### Error Handling

- [ ] Are compiler error messages passed through clearly, with correct paths?
- [ ] Does build stop immediately on first error (fail-fast) or continue?
- [ ] Does user understand WHY rebuild happened (dependency tracking)?
- [ ] Are configuration errors reported before build starts (not mid-compilation)?
- [ ] Is CUE validation error traceable to the exact config line?

### Dependencies

- [ ] Does it handle circular dependencies (detect and report)?
- [ ] Does it work when dependency repo is force-pushed?
- [ ] Does it handle diamond dependencies (A->C@1, B->C@2)?
- [ ] Does it work fully offline after first fetch?
- [ ] Does it verify integrity of downloaded dependencies?

### Parallel Builds

- [ ] Does it work correctly with -j1? (Baseline correctness)
- [ ] Does it work correctly with -j(2*cores)? (Race detection)
- [ ] Does it handle interrupt (Ctrl+C) gracefully? (No corrupt state)
- [ ] Does it recover from crashed build? (No stale lock files)

---

## Domain-Specific Warnings for C/C++ Build Systems

### Header Dependency Tracking Complexities

**The problem:** C++ has notoriously complex header dependency patterns that trip up build systems.

**Specific traps:**
1. **Precompiled header dependency rot**: Adding a header to PCH silently creates dependency for all files
2. **Include guards vs `#pragma once`**: Different mechanisms, edge cases with symlinks
3. **Generated headers**: Must be generated BEFORE dependency scan, or scan misses them
4. **Header search order**: `-I` vs `-isystem` vs `-iquote` have different semantics
5. **Non-existent headers**: `#if __has_include(<foo>)` means file absence is also a dependency

**Prevention:**
- Use compiler's dependency output (`-MD`/`-MMD` for GCC/Clang, `/showIncludes` for MSVC)
- Don't try to parse `#include` directives manually - too many edge cases
- Track header search paths as cache inputs (adding `-I` path should invalidate)

### CUE Configuration Language Considerations

**The opportunity:** CUE's type system catches configuration errors at validation time, before build starts.

**Traps to avoid:**
1. **Overly strict schemas early**: Blocks experimentation; hard to evolve
2. **Under-documenting constraints**: Users don't understand why validation fails
3. **Verbose error messages**: CUE errors need translation to user-friendly build concepts
4. **No migration path**: Schema changes break existing configs without upgrade guidance

**Prevention:**
- Start with minimal schema, add constraints as patterns emerge
- Show which config file and line caused validation failure
- Provide "did you mean?" suggestions for common mistakes
- Consider schema versioning for backwards compatibility

### Go-Specific Implementation Considerations

**File locking on Windows:** Go's standard library doesn't provide cross-platform file locking. Use `golang.org/x/sys/windows` or third-party libraries like `github.com/gofrs/flock`.

**Network filesystem issues:** File locking may fail silently on network mounts, including WSL accessing Windows paths via `\\wsl$`.

**CGO cross-compilation:** If using C libraries, CGO is disabled during cross-compilation. Design core build logic without CGO dependency.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|----------------|------------|
| Config parsing (CUE) | Schema too rigid, error messages unclear | Start permissive, improve errors iteratively |
| Compiler detection | Hardcoded paths, missing Windows/macOS support | Auto-detect + user override + test all platforms |
| Dependency tracking | Miss header changes, over/under invalidate | Use compiler output, comprehensive test suite |
| Parallel execution | Race conditions, resource exhaustion | Sandbox builds, implement pools for expensive ops |
| External deps | Network fragility, version conflicts | Lockfiles, integrity hashes, offline mode |
| Module support | Build order violations | Two-phase scan-then-compile, P1689R5 format |
| Watch mode | Platform differences, atomic save handling | Test on all OSes, watch directories not files |
| Caching | Invalidation bugs, corruption | Content-hash keys, integrity verification |

---

## Sources

### Cache and Incremental Builds
- [Debugging ccache misses](https://interrupt.memfault.com/blog/ccache-debugging) - Detailed analysis of cache miss causes
- [FASTBuild Changelog](https://www.fastbuild.org/docs/changelog.html) - Build system fixes for cache invalidation edge cases
- [Bits'n'Bites: Faster C++ builds](https://www.bitsnbites.eu/faster-c-builds/) - Comprehensive C++ build optimization guide

### C++ Modules
- [CMake C++ Modules Documentation](https://cmake.org/cmake/help/latest/manual/cmake-cxxmodules.7.html) - Official CMake module support documentation
- [P1689R5: Format for describing dependencies](https://www.open-std.org/jtc1/sc22/wg21/docs/papers/2022/p1689r5.html) - C++ standard proposal for module dependencies
- [MSVC /scanDependencies](https://learn.microsoft.com/en-us/cpp/build/reference/scandependencies?view=msvc-170) - Microsoft's module dependency scanning

### Parallel Builds
- [CMake: Avoiding parallel-build race conditions](https://discourse.cmake.org/t/how-to-avoid-parallel-build-race-conditions/727)
- [MSBuild: Diagnose and resolve build race conditions](https://learn.microsoft.com/en-us/visualstudio/msbuild/fix-intermittent-build-failures?view=vs-2022)
- [Detecting Build Dependency Errors](https://arxiv.org/html/2404.13295v1) - Academic paper on build dependency verification

### Cross-Platform
- [Clang MSVC Compatibility](https://clang.llvm.org/docs/MSVCCompatibility.html) - Official Clang documentation on MSVC compatibility
- [Abseil Compiler Flags](https://abseil.io/docs/cpp/platforms/compilerflags) - Google's approach to cross-platform flags
- [CMake Cross Compiling Guide](https://cmake.org/cmake/help/book/mastering-cmake/chapter/Cross%20Compiling%20With%20CMake.html)

### Filesystem Watching
- [fsnotify (Go)](https://github.com/fsnotify/fsnotify) - Cross-platform filesystem notifications for Go
- [fswatch Wiki: Monitors](https://github.com/emcrisostomo/fswatch/wiki/Monitors) - Platform-specific watch API limitations

### Build System Performance
- [Ninja Performance Analysis](https://aosabook.org/en/posa/ninja.html) - Architecture of open source applications chapter
- [Ninja Benchmark](https://david.rothlis.net/ninja-benchmark/) - Performance comparison with Make

### Reproducible Builds
- [Reproducible Builds Documentation](https://reproducible-builds.org/docs/deterministic-build-systems/) - Comprehensive guide to deterministic builds

### CUE Language
- [CUE Introduction](https://cuelang.org/docs/introduction/) - Official documentation
- [How CUE enables configuration](https://cuelang.org/docs/concept/how-cue-enables-configuration/) - Design philosophy

### General Build Systems
- [Signals and Threads: Build Systems](https://signalsandthreads.com/build-systems/) - Podcast discussing build system design
- [6 Months Testing C++ Build Systems](https://keasigmadelta.com/blog/6-months-of-testing-c-build-systems-heres-what-you-need-to-know/) - Practical comparison
- [Build Code Needs Maintenance Too](https://arxiv.org/html/2504.01907v1) - Academic study on build system technical debt
