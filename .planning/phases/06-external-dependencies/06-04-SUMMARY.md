---
phase: 06-external-dependencies
plan: 04
subsystem: dependencies
tags: [graph, topological-sort, dominikbraun, dependency-resolution]

# Dependency graph
requires:
  - phase: 06-01
    provides: Dependency types, schema, and loader
  - phase: 06-02
    provides: Cache manager and git/vendored fetchers
  - phase: 06-03
    provides: Tarball fetcher with secure extraction
provides:
  - Dependency resolver with topological sort
  - Manager for coordinated fetch operations
  - Build order determination with cycle detection
  - Status tracking for cached/missing dependencies
affects: [06-05-cli, 06-06-integration]

# Tech tracking
tech-stack:
  added: [dominikbraun/graph]
  patterns: [topological-sort, fail-fast-fetching, cache-first-strategy]

key-files:
  created:
    - internal/deps/resolver.go
    - internal/deps/manager.go
    - internal/deps/resolver_test.go
    - internal/deps/manager_test.go
  modified: []

key-decisions:
  - "Alphabetical order for independent dependencies (reproducibility)"
  - "graph.StableTopologicalSort with lexical order for determinism"
  - "Fail-fast on first fetch error (matches fail-fast principle)"
  - "Progress format [N/M] with type and ref info"

patterns-established:
  - "Resolver separates ordering logic from fetch operations"
  - "Manager coordinates all fetchers through single interface"
  - "Cache.Has() check before fetch for optimization"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 6 Plan 4: Dependency Resolution and Manager Summary

**Topological sort for dependency build order with graph library, coordinated fetch manager with cache-first strategy and progress reporting**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-23T17:15:55Z
- **Completed:** 2026-01-23T17:18:44Z
- **Tasks:** 3
- **Files modified:** 4 created

## Accomplishments
- Resolver determines build order using dominikbraun/graph with cycle prevention
- Manager coordinates fetch operations across all fetcher types with caching
- Comprehensive test suite with 16 tests verifying ordering and fetch logic
- Status tracking reports cached/missing state for all dependencies

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement dependency resolver** - `4b404bf` (feat)
2. **Task 2: Implement dependency manager** - `356c2e9` (feat)
3. **Task 3: Add resolver and manager tests** - `0413508` (test)

## Files Created/Modified
- `internal/deps/resolver.go` - Topological sort with cycle detection using graph library
- `internal/deps/manager.go` - Coordinates fetch/build operations across fetcher types
- `internal/deps/resolver_test.go` - Tests for build order, cycle detection, reference validation
- `internal/deps/manager_test.go` - Tests for status, caching, fetch coordination

## Decisions Made

**1. Alphabetical order for independent dependencies**
- Rationale: Most dependencies don't depend on each other; alphabetical order ensures reproducible builds

**2. graph.PreventCycles() for cycle detection**
- Rationale: Fail-fast at edge insertion rather than sort-time detection

**3. Fail-fast fetch on first error**
- Rationale: Matches project's fail-fast principle from Phase 1

**4. Progress format [N/M] with details**
- Rationale: Clear user feedback showing progress and what's being fetched

**5. Cache-first fetch strategy**
- Rationale: Skip network operations for cached dependencies

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded smoothly with existing fetchers and graph library.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for:**
- CLI commands (deps fetch, deps status, deps clean)
- Integration with build system
- Config file parsing for dependencies section

**Notes:**
- Phase 6 doesn't implement recursive dependency resolution (deps of deps)
- Dependencies can only depend on other configured dependencies
- Future: Parse clue.cue from dependency cache to extract interdependencies

---
*Phase: 06-external-dependencies*
*Completed: 2026-01-23*
