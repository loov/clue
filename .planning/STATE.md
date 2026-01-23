# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time — not during.

**Current focus:** Phase 4 - Parallel Execution (in progress)

## Current Position

Phase: 4 of 8 (Parallel Execution)
Plan: 1 of 3 in current phase
Status: In progress
Last activity: 2026-01-23 — Completed 04-01-PLAN.md (Parallel Compilation Infrastructure)

Progress: [███████████████] 100% (23/23 plans complete across all phases)

## Performance Metrics

**Velocity:**
- Total plans completed: 23
- Average duration: 5.2min
- Total execution time: 2.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 8 | 40min | 5.0min |
| 02-core-compilation | 9 | 39min | 4.3min |
| 03-incremental-builds | 5 | 37min | 7.4min |
| 04-parallel-execution | 1 | 3.5min | 3.5min |

**Recent Trend:**
- Last 5 plans: 04-01 (3.5min), 03-05 (4.8min), 03-04 (7.4min), 03-03 (8.4min), 03-02 (3min)
- Trend: Phase 4 started, parallel compilation infrastructure complete

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

Last session: 2026-01-23
Stopped at: Completed 04-01-PLAN.md (Parallel Compilation Infrastructure)
Resume file: None
Next step: Execute 04-02-PLAN.md (Signal Handling)
