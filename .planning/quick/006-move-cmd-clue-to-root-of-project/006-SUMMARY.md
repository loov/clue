---
phase: quick
plan: 006
subsystem: cli
tags: [go, go-install, project-structure]

# Dependency graph
requires:
  - phase: all prior phases
    provides: cmd/clue CLI entry point
provides:
  - main.go at project root for simpler go install
  - simplified go install github.com/loov/clue@latest
affects: [user-experience, installation]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - main.go (moved from cmd/clue/main.go)
    - main_test.go (moved from cmd/clue/main_test.go)

key-decisions:
  - "Use git mv to preserve file history through the move"
  - "Update testdata paths from ../../testdata to testdata"

patterns-established:
  - "CLI entry point at project root for simpler go install experience"

# Metrics
duration: 4min
completed: 2026-01-24
---

# Quick Task 006: Move cmd/clue to Project Root Summary

**Relocated CLI entry point to project root enabling `go install github.com/loov/clue@latest` without /cmd/clue suffix**

## Performance

- **Duration:** 4 min
- **Started:** 2026-01-24T06:10:00Z
- **Completed:** 2026-01-24T06:14:00Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Moved main.go and main_test.go from cmd/clue/ to project root
- Updated testdata paths in tests (from ../../testdata to testdata)
- Removed empty cmd/clue and cmd directories
- Preserved git history through git mv

## Task Commits

Each task was committed atomically:

1. **Task 1: Move files and update test paths** - `f1caee3` (refactor)

## Files Created/Modified
- `main.go` - CLI entry point (moved from cmd/clue/main.go)
- `main_test.go` - CLI integration tests (moved from cmd/clue/main_test.go, updated testdata paths)

## Decisions Made
- Used `git mv` for file moves to preserve commit history
- Updated all 6 testdata path references from `"..", "..", "testdata"` to `"testdata"`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Users can now install with `go install github.com/loov/clue@latest`
- All 14 CLI tests pass
- Binary builds successfully from project root

---
*Quick Task: 006*
*Completed: 2026-01-24*
