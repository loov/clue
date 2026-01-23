---
phase: 08-cli-polish
plan: 01
subsystem: cli
tags: [verbosity, flags, cli-ux, output-control, timing]

# Dependency graph
requires:
  - phase: 02-core-compilation
    provides: Progress struct for build output
  - phase: 04-parallel-execution
    provides: Builder with parallel compilation
provides:
  - Verbosity type (Quiet/Normal/Verbose) for output control
  - --quiet and -v flags with mutual exclusion validation
  - FormatDuration helper for human-readable timing
  - Verbosity-aware progress output throughout build system
affects: [08-cli-polish (remaining plans), future CLI features]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Verbosity enum instead of bool flags for three-level output control
    - FormatDuration for consistent human-readable timing (450ms, 2.3s, 1m 12s)
    - Mutual exclusion validation for CLI flags

key-files:
  created:
    - internal/build/verbosity.go
    - internal/build/timing.go
    - internal/build/verbosity_test.go
    - internal/build/timing_test.go
  modified:
    - internal/build/progress.go
    - internal/build/builder.go
    - cmd/clue/main.go
    - internal/build/dep_builder.go
    - internal/build/cache_manager.go

key-decisions:
  - "Verbosity enum (Quiet/Normal/Verbose) over multiple bool flags for clarity and extensibility"
  - "ValidateVerbosityFlags enforces mutual exclusion at CLI entry point"
  - "FormatDuration provides human-readable timing across all build output"
  - "Progress methods check verbosity before output (except Error which always outputs)"

patterns-established:
  - "Verbosity parameter threaded through build system (BuildOptions, DepBuildOptions, CacheManager)"
  - "Quiet mode suppresses all non-error output (zero stdout on success)"
  - "Verbose mode shows full compiler commands and per-file timing"
  - "Normal mode shows progress counts and summaries"

# Metrics
duration: 15min
completed: 2026-01-23
---

# Phase 08 Plan 01: Verbosity Control Summary

**--quiet and -v flags with three-level verbosity control (Quiet/Normal/Verbose) for build output, plus human-readable timing formatter**

## Performance

- **Duration:** 15 min
- **Started:** 2026-01-23T22:26:52Z
- **Completed:** 2026-01-23T22:41:24Z
- **Tasks:** 3
- **Files modified:** 18 (5 new, 13 modified)

## Accomplishments
- Three-level verbosity control (Quiet/Normal/Verbose) throughout build system
- --quiet flag suppresses all non-error output for CI pipelines
- -v flag shows full compiler commands and per-file timing for debugging
- Mutual exclusion validation prevents conflicting flags
- Human-readable duration formatting (450ms, 2.3s, 1m 12s)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Verbosity and Timing Infrastructure** - `ccfbbc2` (feat)
   - Verbosity type with Quiet/Normal/Verbose constants
   - ValidateVerbosityFlags for mutual exclusion checking
   - FormatDuration for human-readable timing
   - Comprehensive unit tests

2. **Task 2: Update Progress with Verbosity Support** - `189de3b` (feat)
   - Changed Progress.verbose from bool to Verbosity type
   - Updated NewProgress signature to accept Verbosity
   - Added CompilingTimed method for verbose mode with timing
   - All output methods respect verbosity levels
   - Error() always outputs regardless of verbosity

3. **Task 3: Wire Verbosity Flags to CLI** - `a15556a` (feat)
   - Added --quiet flag to main.go
   - Added ValidateVerbosityFlags call at CLI entry point
   - Updated BuildOptions, DepBuildOptions, CacheManager to use Verbosity
   - Updated all command handlers (build, validate, clean) to respect verbosity
   - Updated NewBuilder and NewDepBuilder signatures
   - Fixed all test files to use VerbosityNormal instead of false

## Files Created/Modified
- `internal/build/verbosity.go` - Verbosity enum and validation
- `internal/build/timing.go` - FormatDuration helper
- `internal/build/verbosity_test.go` - Verbosity validation tests
- `internal/build/timing_test.go` - Duration formatting tests
- `internal/build/progress.go` - Verbosity-aware progress output
- `internal/build/builder.go` - BuildOptions with Verbosity, platform info respects quiet mode
- `internal/build/dep_builder.go` - DepBuildOptions with Verbosity
- `internal/build/cache_manager.go` - CacheManager with Verbosity parameter
- `cmd/clue/main.go` - --quiet flag, validation, verbosity threading to all commands
- `internal/build/*_test.go` - Updated 13 test files for Verbosity parameter changes

## Decisions Made

**Verbosity enum over multiple bool flags:**
- Chose Verbosity type (Quiet=0, Normal=1, Verbose=2) instead of separate --quiet and --verbose bool flags
- Rationale: Three distinct levels are clearer than boolean combinations, extensible if we add more levels later

**Mutual exclusion at CLI entry point:**
- ValidateVerbosityFlags called immediately after flag.Parse()
- Rationale: Fail-fast with clear error message, prevents undefined behavior from conflicting flags

**FormatDuration formatting rules:**
- <1s: milliseconds (450ms)
- 1-60s: decimal seconds (2.3s)
- ≥60s: minutes and seconds (1m 12s)
- Rationale: Human-friendly at all scales, consistent across build system

**Error output never suppressed:**
- Progress.Error() always outputs even in quiet mode
- Rationale: Errors must be visible for debugging, quiet mode is about progress not failures

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

**Test file updates:**
- BuildOptions changed from Verbose (bool) to Verbosity (Verbosity type)
- ExecutorConfig still uses Verbose (bool) - not changed
- RunOptions uses Verbosity (Verbosity type)
- Had to carefully distinguish between these in test files
- Resolution: Used sed to update test files, then manually fixed ExecutorConfig instances

**Verbosity threading:**
- Many functions needed signature updates to pass Verbosity through
- NewBuilder, NewDepBuilder, NewCacheManager, loadConfig all updated
- Resolution: Systematic update of all call sites and test files

## Next Phase Readiness

Ready for Phase 08 Plan 02 (any remaining CLI polish features). Verbosity infrastructure is complete and tested.

**Foundation established:**
- Three-level verbosity control works throughout build system
- All output methods respect verbosity levels
- Human-readable timing available via FormatDuration
- Test suite updated and passing

**No blockers or concerns.**

---
*Phase: 08-cli-polish*
*Completed: 2026-01-23*
