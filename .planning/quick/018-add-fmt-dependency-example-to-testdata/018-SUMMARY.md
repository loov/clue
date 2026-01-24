---
phase: quick
plan: 018
subsystem: testdata
tags: [git-dependency, fmtlib, fmt, config-parsing, testdata]

# Dependency graph
requires: []
provides:
  - testdata/git-dep-project example for git dependency configuration
  - TestGitDepProject_ConfigParsing integration test
affects: [documentation, user-onboarding]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created:
    - testdata/git-dep-project/clue.cue
    - testdata/git-dep-project/main.cpp
  modified:
    - internal/deps/integration_test.go

key-decisions:
  - "Used fmtlib/fmt 10.2.1 (stable release) instead of main branch"
  - "Full library mode (not header-only) to demonstrate build config"

patterns-established: []

# Metrics
duration: 2min
completed: 2026-01-24
---

# Quick Task 018: Add fmt Dependency Example to Testdata Summary

**Git dependency testdata for fmtlib/fmt with config parsing integration test**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-24T12:56:05Z
- **Completed:** 2026-01-24T12:58:23Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added testdata/git-dep-project demonstrating realistic git dependency for fmtlib/fmt
- Created main.cpp showing fmt::print usage pattern
- Added TestGitDepProject_ConfigParsing validating config parsing for all git dependency fields

## Task Commits

Each task was committed atomically:

1. **Task 1: Create git-dep-project testdata with fmtlib/fmt configuration** - `55a803e` (testdata)
2. **Task 2: Add integration test for git-dep-project config parsing** - `552ad3e` (internal/deps)

## Files Created/Modified
- `testdata/git-dep-project/clue.cue` - Git dependency configuration for fmtlib/fmt 10.2.1
- `testdata/git-dep-project/main.cpp` - Example C++ code using fmt::print
- `internal/deps/integration_test.go` - Added TestGitDepProject_ConfigParsing test

## Decisions Made
- Used ref "10.2.1" (stable release tag) instead of "main" for reproducibility
- Used full library mode (src/format.cc, src/os.cc) to demonstrate build config, not header-only
- Test validates all git dependency fields: repo, ref, build sources, includes, targetType

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- testdata/git-dep-project available for user reference
- Config parsing validated via integration test
- Ready for use in documentation or user guides

---
*Phase: quick*
*Completed: 2026-01-24*
