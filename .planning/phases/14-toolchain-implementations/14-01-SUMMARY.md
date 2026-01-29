---
phase: 14-toolchain-implementations
plan: 01
subsystem: toolchain
tags: [gcc, clang, compiler-flags, cross-compilation, sanitizers]

# Dependency graph
requires:
  - phase: 13-toolchain-interface
    provides: Toolchain interface, Config, Platform, flag helpers
provides:
  - Embeddable gccish.Toolchain for shared GCC/Clang behavior
  - SanitizerFlags helper for toolchain-specific sanitizer handling
affects: [14-02 gcc implementation, 14-03 clang implementation]

# Tech tracking
tech-stack:
  added: []
  patterns: [composition via embedding, helper functions for toolchain-specific behavior]

key-files:
  created:
    - internal/toolchain/gccish/gccish.go
    - internal/toolchain/gccish/gccish_test.go
  modified: []

key-decisions:
  - "Keep sanitizers and coverage out of base CompilerFlags/LinkerFlags - these differ between GCC and Clang"
  - "SanitizerFlags as standalone function rather than method - allows flexibility for callers"
  - "IsCrossCompiler detection via string patterns (-linux-, -darwin-) in compiler path"

patterns-established:
  - "Shared toolchain behavior via embeddable struct with New() constructor"
  - "CompilerFlags/LinkerFlags return base flags, callers append toolchain-specific ones"

# Metrics
duration: 2min
completed: 2026-01-29
---

# Phase 14 Plan 01: Shared GCC/Clang Toolchain Summary

**Embeddable gccish.Toolchain providing 9 interface methods for shared GCC/Clang behavior including flag generation, cross-compiler detection, and sanitizer helpers**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-29T07:54:50Z
- **Completed:** 2026-01-29T07:56:37Z
- **Tasks:** 2
- **Files modified:** 2 created, 1 removed

## Accomplishments
- Created gccish package with Toolchain struct implementing all 9 Toolchain interface methods
- Implemented shared CompilerFlags and LinkerFlags generation (optimization, warnings, debug, LTO, PIC)
- Added SanitizerFlags helper that supports GCC's memory sanitizer skip with warning
- Comprehensive test coverage with 18 test cases covering all methods

## Task Commits

Each task was committed atomically:

1. **Task 1: Create gccish package with shared toolchain implementation** - `0ba6f33` (feat)
2. **Task 2: Add unit tests for gccish package** - `bd09047` (test)

## Files Created/Modified
- `internal/toolchain/gccish/gccish.go` - Shared GCC/Clang toolchain implementation (186 lines)
- `internal/toolchain/gccish/gccish_test.go` - Comprehensive unit tests (344 lines)
- `internal/toolchain/gcc/.gitkeep` - Removed placeholder

## Decisions Made
- **Sanitizers excluded from base flags:** CompilerFlags and LinkerFlags do not include sanitizer handling because GCC and Clang handle sanitizers differently (GCC skips memory sanitizer). Callers add sanitizer flags using the SanitizerFlags helper.
- **SanitizerFlags as function not method:** Allows GCC to call `SanitizerFlags(sanitizers, true)` to skip memory and Clang to call `SanitizerFlags(sanitizers, false)` to include all.
- **Coverage excluded from base flags:** GCC uses `-fprofile-arcs -ftest-coverage` while Clang uses `-fprofile-instr-generate -fcoverage-mapping`. Left to specific implementations.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- gccish package ready for embedding by gcc and clang packages in Plan 02
- All 9 Toolchain interface methods implemented and tested
- SanitizerFlags helper available for toolchain-specific sanitizer handling

---
*Phase: 14-toolchain-implementations*
*Completed: 2026-01-29*
