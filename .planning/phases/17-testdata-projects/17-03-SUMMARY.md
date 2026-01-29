---
phase: 17-testdata-projects
plan: 03
subsystem: testing
tags: [testdata, dependencies, transitive-deps, vendored-libs, c++]

# Dependency graph
requires:
  - phase: 17-testdata-projects
    provides: Understanding of testdata project patterns
provides:
  - Multi-dependency testdata project with transitive dependencies (main -> stringutils -> simplemath)
  - Demonstrates chained vendored library dependencies
  - Example for testing dependency resolution and link order
affects: [17-testdata-projects-04, integration-testing, build-verification]

# Tech tracking
tech-stack:
  added: []
  patterns: [transitive-vendored-deps, chained-libraries]

key-files:
  created:
    - testdata/multi-deps-example/clue.cue
    - testdata/multi-deps-example/main.cpp
    - testdata/multi-deps-example/vendor/simplemath/clue.cue
    - testdata/multi-deps-example/vendor/simplemath/math.h
    - testdata/multi-deps-example/vendor/simplemath/math.cpp
    - testdata/multi-deps-example/vendor/stringutils/clue.cue
    - testdata/multi-deps-example/vendor/stringutils/utils.h
    - testdata/multi-deps-example/vendor/stringutils/utils.cpp
  modified: []

key-decisions:
  - "Used chained dependency pattern: stringutils depends on simplemath, main depends on both"
  - "Designed output with exact strings for test validation"

patterns-established:
  - "Transitive vendored dependency pattern (A -> B -> C)"
  - "Multiple dependency declaration in same project"

# Metrics
duration: 1min
completed: 2026-01-29
---

# Phase 17 Plan 03: Multi-Deps Example Summary

**Created testdata project demonstrating transitive vendored dependencies with chained libraries (stringutils -> simplemath)**

## Performance

- **Duration:** 1 min
- **Started:** 2026-01-29T14:07:28Z
- **Completed:** 2026-01-29T14:08:31Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Created simplemath vendored library with basic math operations
- Created stringutils vendored library that depends on simplemath
- Created main project that uses both libraries, testing transitive dependency resolution
- All libraries properly configured with CUE build definitions

## Task Commits

Each task was committed atomically:

1. **Task 1: Create simplemath vendored library** - `3b8f62a` (testdata)
2. **Task 2: Create stringutils vendored library (depends on simplemath)** - `90e582d` (testdata)
3. **Task 3: Create main project using both libraries** - `1b5f23d` (testdata)

## Files Created/Modified
- `testdata/multi-deps-example/clue.cue` - Main project config with both dependencies
- `testdata/multi-deps-example/main.cpp` - Executable demonstrating both direct and chained usage
- `testdata/multi-deps-example/vendor/simplemath/clue.cue` - Base library config (no deps)
- `testdata/multi-deps-example/vendor/simplemath/math.h` - Math function declarations
- `testdata/multi-deps-example/vendor/simplemath/math.cpp` - Math function implementations
- `testdata/multi-deps-example/vendor/stringutils/clue.cue` - Library config depending on simplemath
- `testdata/multi-deps-example/vendor/stringutils/utils.h` - String utility declarations
- `testdata/multi-deps-example/vendor/stringutils/utils.cpp` - String utilities using simplemath

## Decisions Made
- Used chained dependency pattern where stringutils depends on simplemath to test transitive resolution
- Main project declares both dependencies for clarity, though only stringutils is directly depended on
- Designed output with exact strings ("10 + 5 = 15", "7 * 8 = 56", etc.) for test validation in Plan 04

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- testdata/multi-deps-example/ complete and ready for integration testing
- Dependency chain: main -> stringutils -> simplemath demonstrates transitive dependency resolution
- Output strings designed for automated test validation
- Ready for Plan 04 (integration testing of all testdata projects)

---
*Phase: 17-testdata-projects*
*Completed: 2026-01-29*
