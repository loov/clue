---
phase: 06-external-dependencies
plan: 06
subsystem: cli
tags: [cli, dependencies, commands, user-interface]

# Dependency graph
requires:
  - phase: 06-external-dependencies
    provides: Manager for fetching and building dependencies
provides:
  - clue deps list - shows dependency status table
  - clue deps fetch - downloads dependencies
  - clue deps clean - removes cache
  - User-friendly CLI commands for dependency management
affects: [07-advanced-features]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Command handler pattern - separate business logic from CLI wiring"
    - "Import cycle avoidance - pass dependency map instead of config struct"

key-files:
  created:
    - internal/deps/commands.go
    - internal/deps/commands_test.go
  modified:
    - cmd/clue/main.go

key-decisions:
  - "Command handlers accept dependency map directly to avoid import cycle"
  - "Verbose flag shows additional info (ref, url) for dependencies"
  - "Update subcommand is placeholder for Phase 6"

patterns-established:
  - "Command handler pattern: RunList, RunFetch, RunClean separate from CLI"
  - "FetchOptions struct for configurable operations"

# Metrics
duration: 3.5min
completed: 2026-01-23
---

# Phase 6 Plan 6: CLI Commands Summary

**User-facing CLI commands for dependency management with clear status tables and feedback**

## Performance

- **Duration:** 3.5 min
- **Started:** 2026-01-23T17:29:51Z
- **Completed:** 2026-01-23T17:33:18Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Implemented `clue deps list` showing status table with name, type, status, location
- Implemented `clue deps fetch` for downloading all or specific dependencies
- Implemented `clue deps clean` for removing cache (all or specific)
- Added comprehensive tests for all command handlers
- Integrated commands into main CLI with proper help messages

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement deps command handlers** - `379a924` (feat)
2. **Task 2: Wire deps commands to CLI** - `8d9604e` (feat)
3. **Task 3: Add command tests** - `f8045d5` (test)

## Files Created/Modified
- `internal/deps/commands.go` - Command handlers (RunList, RunFetch, RunClean, RunUpdate)
- `internal/deps/commands_test.go` - Comprehensive tests for command handlers
- `cmd/clue/main.go` - Added deps subcommand and runDeps handler

## Decisions Made

**1. Import cycle avoidance pattern**
- Command handlers accept `map[string]Dependency` instead of `*config.Config`
- Rationale: config package imports deps, so deps cannot import config
- CLI layer extracts cfg.Dependencies before calling handlers

**2. Verbose flag shows ref/url info**
- list command shows git ref, tarball url, or vendored path in verbose mode
- Rationale: helps users understand dependency configuration

**3. Update subcommand placeholder**
- RunUpdate prints "not yet implemented" message
- Rationale: Phase 6 focused on core functionality, updates are future enhancement

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

**Import cycle detection**
- Initial implementation imported config package in commands.go
- Resolved by passing dependency map directly instead of config struct
- Solution aligns with Go best practices for avoiding import cycles

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Dependency management CLI complete
- Users can list, fetch, and clean dependencies via commands
- Ready for Phase 7 advanced features or final integration
- Update checking functionality can be added in future if needed

---
*Phase: 06-external-dependencies*
*Completed: 2026-01-23*
