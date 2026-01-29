---
phase: 12-watch-mode
plan: 02
subsystem: build
tags: [watch, fsnotify, cli, signal-handling, context-cancellation]

# Dependency graph
requires:
  - phase: 12-01
    provides: Watcher type with fsnotify integration and debouncing
provides:
  - clue watch command for continuous rebuilds
  - Cancel-and-restart on rapid changes
  - Config reload on .cue file changes
  - Graceful shutdown on Ctrl+C
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cancel-and-restart: in-progress builds cancelled via context when new changes detected"
    - "Signal handling: Ctrl+C captured via os/signal for graceful shutdown"

key-files:
  created: []
  modified:
    - main.go
    - internal/build/watcher.go

key-decisions:
  - "Use context.WithCancel for build cancellation on rapid changes"
  - "Screen clear uses ANSI escape sequence (\\033[H\\033[2J)"
  - "WatchCount method added for cleaner API"

patterns-established:
  - "Watch command: initial build -> start watcher -> wait for signal"
  - "Rebuild callback: cancel old -> clear screen -> reload if config -> build"

# Metrics
duration: 3min
completed: 2026-01-29
---

# Phase 12 Plan 02: Watch Command Summary

**CLI watch command with cancel-and-restart builds, config reloading, and graceful Ctrl+C shutdown**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-29T02:49:33Z
- **Completed:** 2026-01-29T02:52:33Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added `clue watch` command to CLI with full feature set
- Initial full build runs before watch loop starts
- Screen clears and shows timestamp/trigger on each rebuild
- In-progress builds cancelled when new changes detected
- Config changes (.cue files) trigger full config reload
- Ctrl+C gracefully stops watch mode
- WatchCount method provides directory count for status line

## Task Commits

Each task was committed atomically:

1. **Task 1: Add watch command to main.go** - `496481a` (feat)
2. **Task 2: Add WatchCount method and status line** - `522bec5` (feat)

## Files Created/Modified
- `main.go` - Added runWatch function, collectSourceDirs helper, watch command case
- `internal/build/watcher.go` - Added WatchCount() method

## Decisions Made
- **Context cancellation for builds:** Using context.WithCancel allows in-progress builds to be interrupted cleanly when new changes are detected, avoiding duplicate work
- **ANSI screen clear:** Using `\033[H\033[2J` for screen clearing works on all terminal emulators without external dependencies
- **WatchCount method:** Added explicit method rather than exposing config internals for cleaner API

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Watch mode feature complete with all must-haves verified
- Phase 12 (Watch Mode) is complete pending 12-01 execution
- Ready for v0.2.0 release

---
*Phase: 12-watch-mode*
*Completed: 2026-01-29*
