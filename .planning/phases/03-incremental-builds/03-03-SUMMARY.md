---
phase: 03-incremental-builds
plan: 03
subsystem: build
tags: [cache, incremental-builds, xxhash, json, atomic-writes]

# Dependency graph
requires:
  - phase: 03-01
    provides: Cache key computation (ComputeFileHash, ComputeCacheKey, NormalizeFlags) and dependency parsing (ParseDepFile)
provides:
  - CacheManager for determining rebuild necessity
  - NeedsRebuild decision engine with granular rebuild reasons
  - StoreResult for caching compilation results with header hashes
  - Atomic manifest writes preventing cache corruption
  - Comprehensive test coverage for cache invalidation scenarios
affects: [03-04, 03-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Atomic file writes using temp file + rename pattern
    - JSON manifest for human-debuggable cache state
    - Content-addressable caching via source hash keys
    - Granular rebuild reasons for user feedback

key-files:
  created:
    - internal/build/cache_manager.go
    - internal/build/cache_manager_test.go
  modified:
    - internal/build/cache.go (added HeaderHashes field and JSON tags)

key-decisions:
  - "Manifest keyed by source hash for fast lookup"
  - "Atomic writes prevent corruption on crash/interrupt"
  - "Absolute path normalization for reliable source/header comparison"
  - "Header hashes stored per-file for granular invalidation"
  - "Rebuild reasons provide user feedback on why recompilation needed"

patterns-established:
  - "atomicWrite pattern: temp file + rename for crash-safe persistence"
  - "Absolute path normalization: filepath.Abs for consistent comparisons"
  - "Explicit rebuild reasons: ReasonNotCached, ReasonForced, ReasonSourceChanged, etc."

# Metrics
duration: 8min
completed: 2026-01-23
---

# Phase 03 Plan 03: Cache Manager Summary

**CacheManager with granular rebuild detection for source, headers, flags, and compiler changes using atomic manifest persistence**

## Performance

- **Duration:** 8 min 24 sec
- **Started:** 2026-01-23T10:50:08Z
- **Completed:** 2026-01-23T10:58:32Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- CacheManager determines rebuild necessity with 8 distinct reasons
- Header-level invalidation via per-file hash tracking
- Atomic manifest writes prevent corruption on crash/interrupt
- Comprehensive test suite covering all invalidation scenarios
- Absolute path normalization for reliable comparisons

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement CacheManager type** - `a7dc042` (feat)
2. **Task 2: Add CacheManager tests** - `0fdbe57` (test)

## Files Created/Modified
- `internal/build/cache_manager.go` - CacheManager with NeedsRebuild, StoreResult, GetCached methods
- `internal/build/cache_manager_test.go` - Comprehensive tests for all rebuild scenarios
- `internal/build/cache.go` - Added HeaderHashes field to CacheKey, added JSON tags for serialization

## Decisions Made

**Manifest keyed by source hash:**
- Use source file hash as manifest key for O(1) lookup
- Rationale: Fast cache checks without scanning entire manifest

**Atomic writes for crash safety:**
- atomicWrite helper uses temp file + rename pattern
- Rationale: Prevents corrupted manifest on crash or interrupt, follows POSIX atomic rename guarantee

**Absolute path normalization:**
- Convert source/header paths to absolute before comparison
- Rationale: Dep files may contain relative paths while source is absolute (or vice versa), normalization ensures reliable comparison

**Header-level granularity:**
- Store individual header hashes in HeaderHashes map
- Rationale: More precise than combined DepsHash, enables reporting which specific header changed

**Explicit rebuild reasons:**
- NeedsRebuild returns specific reason (ReasonNotCached, ReasonSourceChanged, etc.)
- Rationale: Provides actionable feedback to users on why rebuild occurred

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed slicesEqual to stringSlicesEqual**
- **Found during:** Task 2 (test compilation)
- **Issue:** Name collision with slicesEqual in flags_test.go causing build failure
- **Fix:** Renamed helper function to stringSlicesEqual to avoid conflict
- **Files modified:** internal/build/cache_manager.go
- **Verification:** go build ./internal/build/... succeeds
- **Committed in:** 0fdbe57 (Task 2 commit)

**2. [Rule 1 - Bug] Fixed absolute path comparison for source file detection**
- **Found during:** Task 2 (test failures)
- **Issue:** Dep files contain relative "test.cpp" while srcPath is absolute, causing source file to be treated as changed header
- **Fix:** Added filepath.Abs normalization in both NeedsRebuild and StoreResult for consistent comparison
- **Files modified:** internal/build/cache_manager.go
- **Verification:** TestNeedsRebuild_HeaderChanged and TestManifestPersistence pass
- **Committed in:** 0fdbe57 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered

**Test failures due to path normalization:**
- Initial tests failed because dep file contained relative path "test.cpp" but source was absolute
- Root cause: Dep files from real compilers may use either relative or absolute paths
- Solution: Normalize both source and dependency paths to absolute before comparison
- Impact: More robust cache manager that handles real compiler output correctly

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for integration:**
- CacheManager can be integrated into build pipeline
- NeedsRebuild provides decision logic for incremental builds
- StoreResult caches successful compilations
- Manifest persists across build invocations

**Next steps:**
- Integrate CacheManager into Builder.BuildTarget
- Wire up cache checks before compilation
- Store results after successful compilation
- Add --rebuild-all flag support

**No blockers:** All cache infrastructure complete and tested.

---
*Phase: 03-incremental-builds*
*Completed: 2026-01-23*
