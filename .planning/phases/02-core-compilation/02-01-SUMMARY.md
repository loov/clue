---
phase: 02-core-compilation
plan: 01
subsystem: build
tags: [gcc, clang, compiler-flags, static-library, semantic-config]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: CUE configuration schema and loader infrastructure
provides:
  - Semantic flag abstraction (optimize, warnings, debug levels)
  - Compiler flag mapping for GCC/Clang
  - System library linking configuration
  - Build directory configuration in CUE schema
affects: [02-02, 02-03, 02-04, 02-05, 02-06]

# Tech tracking
tech-stack:
  added: []
  patterns: [semantic-flag-mapping, flag-table-pattern]

key-files:
  created:
    - internal/build/flags.go
    - internal/build/flags_test.go
  modified:
    - internal/config/schema.cue

key-decisions:
  - "Semantic flags map to GCC/Clang compatible options: none→-O0, size→-Os, fast→-O2, aggressive→-O3"
  - "Warnings as errors enabled by default (warningsAsErrors: true) for early error detection"
  - "Debug flag levels: none (no debug), minimal (-g1 line tables), full (-g complete debug info)"
  - "Raw compiler/linker flags remain as escape hatch alongside semantic flags"

patterns-established:
  - "Flag mapping tables: Map semantic names to compiler-specific flags"
  - "BuildConfig struct: Central configuration for semantic build options"
  - "Graceful unknown handling: Missing map keys don't panic, just skip the flag"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 2 Plan 1: Semantic Flag Mapping Summary

**Human-friendly semantic flags (optimize: "fast") map to GCC/Clang compiler options with tested mappings for optimization, warnings, debug info, and system library linking**

## Performance

- **Duration:** 2 min 6 sec
- **Started:** 2026-01-23T06:26:24Z
- **Completed:** 2026-01-23T06:28:30Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Semantic flag fields added to CUE schema (#Target) for user-friendly configuration
- BuildCompilerFlags() and BuildLinkerFlags() functions map semantic options to compiler arguments
- Comprehensive test coverage (16 tests) verifying all flag mappings and combinations

## Task Commits

Each task was committed atomically:

1. **Task 1: Update CUE schema with semantic flag definitions** - `04e916a` (feat)
2. **Task 2: Create semantic flag mapping implementation** - `d785a71` (feat)
3. **Task 3: Add flag mapping tests** - `4797f15` (test)

## Files Created/Modified

- `internal/config/schema.cue` - Added semantic flag fields (optimize, warnings, warningsAsErrors, debug, sysLibs) to #Target and buildDir to #Config
- `internal/build/flags.go` - BuildCompilerFlags() and BuildLinkerFlags() with semantic-to-flag mapping tables
- `internal/build/flags_test.go` - 16 tests covering optimization, warnings, debug, system libraries, and combined scenarios

## Decisions Made

**Semantic flag mapping:**
- **Optimization:** none→-O0 (fast compile), size→-Os (optimize size), fast→-O2 (balanced), aggressive→-O3 (max speed)
- **Warnings:** off→[] (none), default→-Wall, strict→-Wall -Wextra, pedantic→-Wall -Wextra -Wpedantic
- **Debug:** none→"" (no debug), minimal→-g1 (line tables only), full→-g (complete debug info)
- **Rationale:** GCC/Clang compatible, matches RESEARCH.md recommendations, provides clear semantic meaning

**Warnings as errors default:**
- warningsAsErrors defaults to `true` in CUE schema
- Rationale: Fail-fast principle, catches potential issues early in development

**Raw flags as escape hatch:**
- RawCompiler and RawLinker fields in BuildConfig
- Flags field in CUE schema remains alongside semantic options
- Rationale: Users can override/extend semantic flags for edge cases without blocking

**System library linking:**
- sysLibs field accepts library names without -l prefix (e.g., "pthread" → "-lpthread")
- BuildLinkerFlags() adds -l prefix automatically
- Rationale: Cleaner configuration syntax, consistent with semantic flag philosophy

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all tasks completed successfully without problems.

## Next Phase Readiness

**Ready for 02-02 (Compiler Invocation):**
- Semantic flag mapping infrastructure complete
- BuildCompilerFlags() ready to provide flags for compiler execution
- BuildLinkerFlags() ready for linking and system library handling

**Ready for future plans:**
- CUE schema supports buildDir configuration (default: "build")
- Flag mapping tested and verified for all semantic options
- Raw flag escape hatch available for advanced use cases

**No blockers:**
- All verification checks pass
- Full test coverage ensures correctness
- No external dependencies needed

---
*Phase: 02-core-compilation*
*Completed: 2026-01-23*
