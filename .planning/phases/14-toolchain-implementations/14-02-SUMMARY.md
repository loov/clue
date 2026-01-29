---
phase: 14-toolchain-implementations
plan: 02
subsystem: toolchain
tags: [gcc, clang, sanitizers, coverage, compiler-flags, embedding]

# Dependency graph
requires:
  - phase: 14-toolchain-implementations
    plan: 01
    provides: gccish.Toolchain embeddable struct, SanitizerFlags helper
provides:
  - GCC toolchain package with Toolchain type implementing toolchain.Toolchain
  - Clang toolchain package with Toolchain type implementing toolchain.Toolchain
  - GCC-specific memory sanitizer skip with warning
  - GCC gcov-based coverage flags (-fprofile-arcs, -ftest-coverage)
  - Clang source-based coverage flags (-fprofile-instr-generate, -fcoverage-mapping)
affects: [14-03 msvc implementation, factory pattern]

# Tech tracking
tech-stack:
  added: []
  patterns: [struct embedding for shared behavior, method override for toolchain-specific flags]

key-files:
  created:
    - internal/toolchain/gcc/gcc.go
    - internal/toolchain/gcc/gcc_test.go
    - internal/toolchain/clang/clang.go
    - internal/toolchain/clang/clang_test.go
  modified: []

key-decisions:
  - "GCC overrides CompilerFlags and LinkerFlags to add sanitizers and coverage"
  - "Clang overrides CompilerFlags and LinkerFlags to add sanitizers and coverage"
  - "Struct embedding delegates all other methods to gccish.Toolchain"

patterns-established:
  - "Toolchain-specific packages embed gccish.Toolchain and override only flag methods"
  - "Compile-time interface check: var _ toolchain.Toolchain = (*Toolchain)(nil)"

# Metrics
duration: 2min
completed: 2026-01-29
---

# Phase 14 Plan 02: GCC and Clang Toolchain Packages Summary

**GCC and Clang toolchain packages using struct embedding for shared behavior with toolchain-specific sanitizer and coverage flag handling**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-29T08:00:00Z
- **Completed:** 2026-01-29T08:02:00Z
- **Tasks:** 2
- **Files modified:** 4 created, 1 removed

## Accomplishments
- Created internal/toolchain/gcc package embedding gccish.Toolchain
- Created internal/toolchain/clang package embedding gccish.Toolchain
- GCC skips memory sanitizer with warning, uses gcov coverage flags
- Clang supports all sanitizers, uses source-based coverage with link-time flag
- Comprehensive test coverage: 6 tests for GCC, 7 tests for Clang

## Task Commits

Each task was committed atomically:

1. **Task 1: Create GCC toolchain package** - `c5cb301` (feat)
2. **Task 2: Create Clang toolchain package** - `5b71725` (feat)

## Files Created/Modified
- `internal/toolchain/gcc/gcc.go` - GCC toolchain implementation (56 lines)
- `internal/toolchain/gcc/gcc_test.go` - GCC-specific tests (162 lines)
- `internal/toolchain/clang/clang.go` - Clang toolchain implementation (62 lines)
- `internal/toolchain/clang/clang_test.go` - Clang-specific tests (168 lines)
- `internal/toolchain/clang/.gitkeep` - Removed placeholder

## Decisions Made
- **Struct embedding delegates non-overridden methods:** CC(), CXX(), AR(), Name(), IsCrossCompiler(), String(), Identity() all inherited from gccish.Toolchain
- **Override only CompilerFlags and LinkerFlags:** These are the only methods that differ between GCC and Clang
- **Use compile-time interface check:** `var _ toolchain.Toolchain = (*Toolchain)(nil)` ensures interface compliance

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- GCC and Clang toolchain packages ready for use by factory pattern
- Both implement toolchain.Toolchain interface via embedding + override
- Ready for Plan 03: MSVC implementation

---
*Phase: 14-toolchain-implementations*
*Completed: 2026-01-29*
