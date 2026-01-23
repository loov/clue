# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time — not during.

**Current focus:** Phase 2 - Core Compilation (next up)

## Current Position

Phase: 2 of 8 (Core Compilation)
Plan: 4 of 7 in current phase
Status: In progress
Last activity: 2026-01-23 — Completed 02-04-PLAN.md (Linker & Archiver)

Progress: [████░░░░░░] ~36% (4/7 plans in Phase 2 complete)

## Performance Metrics

**Velocity:**
- Total plans completed: 12
- Average duration: 4.0min
- Total execution time: 0.80 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 8 | 40min | 5.0min |
| 02-core-compilation | 4 | 8min | 2.0min |

**Recent Trend:**
- Last 5 plans: 02-04 (2min), 02-03 (2min), 02-02 (2min), 02-01 (2min), 01-08 (2min)
- Trend: Sustained 2min velocity in Phase 2, foundation infrastructure enabling rapid execution

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

### Pending Todos

None yet.

### Blockers/Concerns

- **Network isolation:** Environment has no external network access. CUE stubs work for testing but full validation requires `go mod tidy` with network.

## Session Continuity

Last session: 2026-01-23T06:33:33Z
Stopped at: Completed 02-04-PLAN.md (Linker & Archiver)
Resume file: None
Next step: Continue Phase 2 - Plan 02-05 (Build Orchestrator)
