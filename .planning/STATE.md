# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time — not during.

**Current focus:** Phase 7 - Output Generators (next up)

## Current Position

Phase: 7 of 8 (Output Generators)
Plan: 2 of 4 in phase 7 complete
Status: In progress
Last activity: 2026-01-23 — Completed 07-02-PLAN.md (compile_commands.json generation)

Progress: [████████████████████] 95% (40/42 plans complete across all phases)

## Performance Metrics

**Velocity:**
- Total plans completed: 38
- Average duration: 4.3min
- Total execution time: 2.86 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 8 | 40min | 5.0min |
| 02-core-compilation | 9 | 39min | 4.3min |
| 03-incremental-builds | 5 | 37min | 7.4min |
| 04-parallel-execution | 4 | 16.7min | 4.2min |
| 05-cross-platform-support | 7 | 24.0min | 3.4min |
| 06-external-dependencies | 7 | 27.7min | 4.0min |
| 07-output-generators | 2 | ~10min | ~5min |

**Recent Trend:**
- Last 5 plans: 07-02 (7.3min), 07-01+07-03 (~concurrent), 06-07 (8.0min), 06-06 (3.5min), 06-05 (6.2min)
- Trend: Phase 7 in progress - output generators being implemented

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Phase 1: CUE for configuration — type validation catches errors before build (rationale: catches errors at parse time)
- Phase 1: Clang-first for modules — most mature C++20 module support (rationale: delegate scanning to compiler)
- Phase 1: Both direct and generated builds — direct for simple, Ninja/Make for complex projects (rationale: flexibility)
- Phase 1: Uniform dependency model — git/tarball/vendored share schema shape (rationale: consistency)
- 01-02: graph.PreventCycles() over graph.Acyclic() — PreventCycles provides fail-fast cycle detection at edge insertion (rationale: required behavior for DAG enforcement)
- 01-02: Lexical stable sort for determinism — StableTopologicalSort with a < b ensures consistent build order (rationale: reproducible builds)
- 01-01: go:embed for CUE schema — keeps schema in binary, no runtime file access needed
- 01-01: CUE definitions (#Type) — closed by default, catches extra fields as errors
- 01-03: Raw ANSI codes for colors — simpler implementation without external dependencies
- 01-03: syscall for TTY detection — portable without golang.org/x/term
- 01-03: CUE stubs with JSON fallback — enables offline development when network unavailable
- 01-03: Buildable interface for CUE instances — clean separation between load and cue packages
- 01-04: CLI > env > default precedence — matches standard tool conventions for variant selection
- 01-04: CUE unification for variant merging — leverages CUE's built-in merging capabilities
- 01-04: Hidden _env field for injection — environment variables accessible via GetEnvValue
- 01-04: Mandatory defaults for env vars — fails early with clear error message
- 01-05: Target type to node type mapping — converts config target types to graph node types
- 01-05: Flag-before-command convention — Go's flag package requires flags before positional arguments
- 01-06: Real CUE library over shim — removed 561 lines of dead shim code
- 01-07: Truthy values for conditionals — 1, true, yes, on (case-insensitive) matches standard conventions
- 01-07: Global when_true application — conditionals apply to ALL targets, not per-target (rationale: env vars are typically global settings)
- 01-07: Deep copy pattern in ApplyEnvVars — preserves immutability, enables caching
- 01-07: CUE native lookup for conditionals — cue.ParsePath instead of JSON serialization (rationale: type-safe, preserves CUE semantics)
- 01-08: Command nodes as explicit vertices — enables tracking compile and link operations with metadata
- 01-08: Structured node IDs — src:target:path, obj:target:path, cmd:compile:target:source, cmd:link:target, out:target for predictable lookup
- 01-08: Cross-target dependencies at link level — link command depends on dependency output artifact
- 02-01: Semantic flags map to GCC/Clang options — none→-O0, size→-Os, fast→-O2, aggressive→-O3 (rationale: user-friendly configuration with compiler compatibility)
- 02-01: Warnings as errors by default — warningsAsErrors: true (rationale: fail-fast principle, early error detection)
- 02-01: Debug flag levels — none (no debug), minimal (-g1 line tables), full (-g complete) (rationale: fine-grained control over debug info size)
- 02-01: Raw flags as escape hatch — RawCompiler/RawLinker alongside semantic flags (rationale: unblock edge cases without losing semantic benefits)
- 02-02: ExecutorConfig struct for configurable execution — Verbose/StreamOutput/WorkDir enable different use cases (testing vs production)
- 02-02: Exit code extraction via syscall.WaitStatus — cross-platform compatibility with fallback to exitErr.ExitCode()
- 02-02: RunCompiler wrapper always streams — real-time feedback for long compilation processes (rationale: better UX than silent builds)
- 02-03: Compiler selection based on file extension — .cpp/.cc/.cxx/.C/.CPP trigger C++ compiler (rationale: standard convention across build systems)
- 02-03: Toolchain parameter with clang default — supports clang and gcc, defaults to clang if unknown (rationale: best C++20 module support)
- 02-03: Fail-fast batch compilation — CompileSources stops on first error, returns successful results (rationale: matches fail-fast principle)
- 02-03: Automatic output directory creation — creates filepath.Dir(opts.Output) before compilation (rationale: prevents blocking from missing directories)
- 02-04: ar crs for static libraries — single command creates archive with symbol table, no separate ranlib (rationale: simpler, reliable)
- 02-04: C++ linker selection via UseCPlusPlus flag — explicit control over clang++/g++ vs clang/gcc for linking (rationale: C++ std lib requirement)
- 02-04: System libraries handled separately — distinct from additional libraries for API clarity (rationale: common use case deserves clear semantics)
- 02-04: Automatic output directory creation for linker — link operations create directories as needed (rationale: reliability without manual setup)
- 02-05: Progress format [N/M] target: filename — clear compilation status during builds (rationale: user feedback for long builds)
- 02-05: Artifact organization by variant and type — build/variant/bin for executables, build/variant/lib for libraries (rationale: clean separation, predictable paths)
- 02-05: Dependency linking via -L and -l flags — static library dependencies automatically added to linker command (rationale: correct linking for multi-target projects)
- 02-05: Extracted loadConfig helper — shared between runValidate and runBuild (rationale: eliminate code duplication)
- 02-06: Require variant when not using --all — prevents accidental deletion of build artifacts (rationale: safety first for destructive operations)
- 02-06: Build directory relative to project directory — clean operates on build directory within --dir location (rationale: consistency with build command)
- 02-06: Graceful handling of non-existent directories — missing build directories return success with "Already clean" message (rationale: matches user expectation, no error for already-clean state)
- 02-07: CompileBytes for packageless configs — use ctx.CompileBytes() instead of load.Instances for JSON/CUE data files (rationale: data files shouldn't require package declarations)
- 02-07: Pointer for WarningsAsErrors — *bool allows distinguishing unset from false (rationale: nil = use default, explicit false = disabled)
- 02-07: Simplified schema variants — [string]: #Variant instead of forced debug/release definitions (rationale: users define their own variants)
- 02-08: Target semantic flag priority — defaults < target flags < variant flags in targetToBuildConfig (rationale: target-specific overrides with variant final precedence)
- 02-09: System libraries from target config — target.SysLibs passed to LinkOptions instead of empty array (rationale: enables linking against system libs like pthread, m, dl)
- 03-01: xxh3.Hash128 for content hashing — significantly faster than SHA256 while providing excellent distribution (rationale: performance critical for large codebases, cryptographic properties not needed)
- 03-01: Compiler identity via file stat — use mtime + size instead of version parsing (rationale: fast, reliable, version parsing is fragile and compiler-specific)
- 03-01: Absolute paths in cache keys — resolve relative -I paths to absolute in NormalizeFlags (rationale: cache keys work correctly regardless of working directory)
- 03-02: Dependency generation via -MMD -MP -MF flags — compiler generates .d files automatically during compilation (rationale: standard approach for header dependency tracking)
- 03-02: DepFile path computed from object path — replace .o extension with .d for consistency (rationale: keeps dependency files alongside object files with predictable naming)
- 03-03: Manifest keyed by source hash — fast O(1) lookup using source file hash as key (rationale: efficient cache checks without scanning entire manifest)
- 03-03: Atomic manifest writes — temp file + rename pattern prevents corruption on crash (rationale: POSIX atomic rename guarantee ensures cache integrity)
- 03-03: Absolute path normalization for comparisons — filepath.Abs before comparing source/headers (rationale: dep files may have relative paths, normalization ensures reliable comparison)
- 03-03: Header-level hash granularity — HeaderHashes map tracks individual header changes (rationale: more precise than combined DepsHash, enables reporting specific changed file)
- 03-03: Explicit rebuild reasons — NeedsRebuild returns specific reason enum (rationale: provides actionable user feedback on why recompilation needed)
- 03-04: Cache check per source file — NeedsRebuild called for each source before compilation (rationale: fine-grained caching, skip only unchanged files)
- 03-04: Progress built/cached distinction — separate counters for compiled vs skipped files (rationale: clear user feedback on cache effectiveness)
- 03-04: --rebuild-all flag — bypass cache and force recompilation of all files (rationale: escape hatch for cache issues or guaranteed clean builds)
- 03-05: Object mtime verification for tests — use modification times to verify rebuild behavior (rationale: reliable detection of whether files were recompiled)
- 03-05: Full project integration tests — create complete C++ projects in tests (rationale: tests full integration path from source to executable)
- 04-01: errgroup.WithContext + SetLimit for parallel execution — standard Go pattern with automatic context cancellation and bounded concurrency
- 04-01: Output buffering via bytes.Buffer per compilation — prevents interleaved output from concurrent compilations
- 04-01: Return nil from g.Go when keepGoing is true — allows other goroutines to continue without context cancellation
- 04-02: signal.NotifyContext for clean signal handling — Go 1.16+ pattern for signal-cancellable context (rationale: clean signal handling with automatic context cancellation)
- 04-02: Double Ctrl+C pattern — first cancels gracefully, second forces os.Exit(130) (rationale: allows graceful shutdown with escape hatch)
- 04-02: Process groups via Setpgid — all compiler processes in their own process group (rationale: clean termination of child processes on cancellation)
- 04-02: 100ms SIGTERM to SIGKILL timeout — brief window for graceful exit before forced kill (rationale: balance between responsiveness and cleanliness)
- 04-03: Default -j 0 means NumCPU/2 (minimum 1) — sensible default for parallel builds (rationale: leaves headroom for system processes)
- 04-03: -j -1 or negative means unlimited (all cores) — follows make convention
- 04-03: --keep-going follows make -k convention — continue building despite errors
- 04-03: atomic.Int64 for Progress counters — lock-free incrementing for performance
- 04-03: sync.Mutex for output serialization — prevents interleaved progress output
- 04-04: Use target names without hyphens — avoids CUE selector quoting issues with iter.Selector().String()
- 04-04: Absolute paths in test config — required since compiler runs from different working directory
- 04-04: Keep-going mode creates partial output — successfully compiled files produce output even when some fail
- 05-01: Go-style "os-arch" format — Use "linux-amd64" format for target platforms (rationale: familiar to Go developers, simpler than LLVM triples)
- 05-01: Runtime constants for detection — Use runtime.GOOS/GOARCH for platform detection (rationale: compile-time constants, no external dependencies, reliable)
- 05-01: Map-based platform validation — supportedPlatforms map for O(1) lookup (rationale: fast validation, easy to extend)
- 05-01: Go-style "os-arch" format — Use "linux-amd64" format for target platforms (rationale: familiar to Go developers, simpler than LLVM triples)
- 05-01: Runtime constants for detection — Use runtime.GOOS/GOARCH for platform detection (rationale: compile-time constants, no external dependencies, reliable)
- 05-01: Map-based platform validation — supportedPlatforms map for O(1) lookup (rationale: fast validation, easy to extend)
- 05-02: CC/CXX environment variables override configured toolchain — Standard Unix convention matching CMake/Make behavior (rationale: follows established build tool patterns)
- 05-02: crossPrefix checks host platform — Returns empty prefix for native compilation, GNU triplet for cross-compilation (rationale: same platform should use native compilers)
- 05-02: GNU triplet convention — aarch64-linux-gnu- for ARM64, x86_64-linux-gnu- for AMD64 (rationale: standard cross-compiler naming)
- 05-02: ValidateToolchain uses exec.LookPath — Validates all tools (CC/CXX/AR) exist in PATH before builds (rationale: fail-fast validation with clear error messages)
- 05-02: Separate gnuTripletPrefix for unit testing — Pure platform-to-prefix mapping for testing without host dependency (rationale: enables testing raw mapping logic)
- 05-03: Sanitizer GCC warning — Warn and skip MemorySanitizer on GCC (Clang-only feature) with user feedback (rationale: prevents build failure while informing user)
- 05-03: Coverage toolchain-specific flags — Clang uses source-based coverage (-fprofile-instr-generate), GCC uses gcov (-fprofile-arcs) (rationale: matches toolchain capabilities)
- 05-03: LTO in both phases — -flto added to both compiler and linker for correct whole-program optimization (rationale: LTO requires matching flags in both compilation and linking)
- 05-04: NewBuilder returns error — Changed signature to handle toolchain discovery and validation errors (rationale: fail-fast with clear error messages)
- 05-04: Platform parameter in NewBuilder — Added target Platform to enable toolchain discovery for cross-compilation (rationale: enables cross-compilation support)
- 05-04: SharedLibraryExtension as package function — Standalone function in linker.go rather than method (rationale: platform-dependent not linker-instance-dependent)
- 05-04: OutputPath supports shared_library — Extended Builder.OutputPath with platform-specific extensions (.dylib/.so/.dll) (rationale: enables shared library builds on all platforms)
- 05-05: Flag-before-command convention for --target — Flags must precede commands per Go flag package (e.g., --target=linux-arm64 build) (rationale: matches Go conventions, consistent with existing flags)
- 05-05: Platform display timing — Show "Building for X" or "Cross-compiling for X" immediately after flag parsing (rationale: early user feedback on target platform)
- 05-05: Default to HostPlatform — When --target not specified, use native platform for simplicity (rationale: common case should be simple)
- 05-07: Wire toolchain.Name to flag building — Compiler and linker pass toolchain.Name directly to WithToolchain functions (rationale: enables toolchain-specific flag logic for coverage, sanitizers, and cross-compilation)
- 06-01: Dependency name from map key — CUE uses dependencies map keys as dependency names, not separate name field (rationale: matches CUE's structural approach, avoids redundancy)
- 06-01: Cache path structure — .deps/{type}/{sanitized-name}-{short-ref} for predictable locations (rationale: enables future fetch operations)
- 06-01: Vendored deps return original path — no caching needed for local source tree dependencies (rationale: already in source tree)
- 06-01: InlineConfig for deps without clue.cue — enables building third-party libraries without configuration files (rationale: integration flexibility)
- 06-01: Validation at load time — Validate() called during config extraction for fail-fast (rationale: catch dependency errors early)
- 06-03: filepath.IsLocal + absolute path checks for security — Double-layered path validation prevents traversal attacks (rationale: defense in depth)
- 06-03: Silent symlink skipping — Skip symlinks/hardlinks without error for security (rationale: rare in source tarballs, security risk)
- 06-03: CI mode vs non-CI checksum handling — CI errors on missing checksums, non-CI warns (rationale: reproducibility vs convenience)
- 06-03: Streaming checksum computation — io.MultiWriter computes SHA256 during download (rationale: performance for large tarballs)
- 06-03: Cleanup on extraction error — Remove target directory on any extraction failure (rationale: prevent partial/corrupted states)
- 06-04: Alphabetical order for independent dependencies — No inter-dependencies defaults to alphabetical for reproducibility (rationale: deterministic build order)
- 06-04: graph.StableTopologicalSort with lexical order — Stable sort with a < b comparison for consistent ordering (rationale: reproducible builds across environments)
- 06-04: Fail-fast fetch on first error — FetchAll stops immediately on error (rationale: matches fail-fast principle from Phase 1)
- 06-04: Cache-first fetch strategy — Manager checks cache.Has() before fetching (rationale: avoid unnecessary network operations)
- 06-05: DepBuilder in build package — Placed in internal/build to avoid import cycle (build→config→deps) (rationale: clean package hierarchy)
- 06-05: Collapsed dependency build output — Single line per dependency, expands on error (rationale: clean progress without noise)
- 06-05: Include path auto-detection — inline config > include/ directory > source root (rationale: matches C++ library conventions)
- 06-05: Dependencies inherit variant and platform — DepBuildOptions passes variant and platform from main build (rationale: ABI compatibility)
- 06-05: Dependency artifacts in .build/variant/deps/ — Separate from main project artifacts (rationale: clear ownership, easy to clean)
- 06-06: Command handlers accept dependency map — RunList/RunFetch/RunClean take map[string]Dependency instead of *config.Config (rationale: avoids import cycle, config imports deps)
- 06-07: Graph builder distinguishes target vs external dependencies — BuildGraphFromConfig checks both cfg.Targets and cfg.Dependencies, only adds target-to-target edges to build graph (rationale: external dependencies handled separately by builder, allows depends field to reference both types)

### Pending Todos

None yet.

### Blockers/Concerns

- **Network isolation:** Environment has no external network access. Used `go test -mod=mod` to work with locally cached modules in phase 3. CUE stubs work for testing but full validation requires `go mod tidy` with network.
- **Variant application bug:** ApplyVariant() in variants.go unifies entire config with variant definition, causing conflicts. Validation works for configs without variants. Fix needed for variant-based builds.
- **--rebuild-all output issue:** The --rebuild-all flag completes successfully but produces no output and may not actually force recompilation. Needs investigation in future plan.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 001 | Adjust default build directory to .build | 2026-01-23 | c3d5a8c | [001-adjust-default-build-dir](./quick/001-adjust-default-build-dir/) |
| 002 | Fix testdata CUE files to use idiomatic syntax | 2026-01-23 | 1e30e6f | [002-fix-testdata-cue-idiomatic](./quick/002-fix-testdata-cue-idiomatic/) |
| 003 | Target object folder structure | 2026-01-23 | b16ea53 | [003-target-obj-folder-structure](./quick/003-target-obj-folder-structure/) |

## Session Continuity

Last session: 2026-01-23 19:11 UTC
Stopped at: Completed 07-02-PLAN.md (compile_commands.json generation)
Resume file: None
Next step: Continue Phase 7 with 07-04-PLAN.md (CLI integration)
