---
phase: 06-external-dependencies
plan: 02
subsystem: build-system
tags: [dependencies, go-git, cache, git, vendored]

# Dependency graph
requires:
  - phase: 06-01
    provides: Dependency types (GitDependency, TarballDependency, VendoredDependency)
provides:
  - Cache manager for tracking fetched dependencies
  - Git fetcher using go-git with shallow clone optimization
  - Vendored fetcher for local dependency validation
affects: [06-03-dependency-building, 06-04-dependency-integration]

# Tech tracking
tech-stack:
  added: [github.com/go-git/go-git/v5]
  patterns: [Fetcher interface for dependency retrieval, .deps directory for caching]

key-files:
  created:
    - internal/deps/cache.go
    - internal/deps/fetcher.go
    - internal/deps/git_fetcher.go
    - internal/deps/vendored_fetcher.go

key-decisions:
  - "Shallow clone by default (Depth: 1) for git fetches - fallback to full clone for pinned commits"
  - "Tag detection based on ref format (starts with v or contains dots)"
  - ".clue-dep marker file tracks fetch metadata with JSON"
  - "Vendored dependencies validated in place - no caching needed"

patterns-established:
  - "Fetcher interface: all dependency types implement Fetch(ctx, dep, targetPath)"
  - "Cache.Has() checks for .git directory presence for git deps"
  - "Verbose mode provides detailed progress output for all operations"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 6 Plan 2: Dependency Fetching Summary

**Git cloning with shallow optimization and vendored dependency validation using go-git and filesystem checks**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-23T17:10:55Z
- **Completed:** 2026-01-23T17:12:50Z
- **Tasks:** 3
- **Files modified:** 4 created, 2 modified (go.mod, go.sum)

## Accomplishments
- Cache manager tracks fetched dependencies with Has/Path/Clean operations
- Git fetcher clones repositories using go-git with shallow clone by default
- Vendored fetcher validates local paths and checks for build configuration
- Context cancellation support for clean abort on Ctrl+C

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement dependency cache manager** - `41d6054` (feat)
2. **Task 2: Implement git fetcher with go-git** - `730bf4b` (feat)
3. **Task 3: Implement vendored fetcher** - `9498533` (feat)

## Files Created/Modified
- `internal/deps/cache.go` - Cache manager with Has/Path/MarkFetched/Clean/CleanDep operations
- `internal/deps/fetcher.go` - Fetcher interface defining Fetch method
- `internal/deps/git_fetcher.go` - Git repository cloning using go-git library
- `internal/deps/vendored_fetcher.go` - Local path validation for vendored dependencies
- `go.mod` - Added github.com/go-git/go-git/v5 dependency
- `go.sum` - Dependency checksums

## Decisions Made

**Shallow clone optimization:** Git fetcher uses Depth: 1 by default for speed, with automatic fallback to full clone when shallow fails (e.g., for pinned commits that aren't branch heads).

**Tag detection heuristic:** Refs starting with "v" or containing dots are treated as tags and use NewTagReferenceName, others use NewBranchReferenceName. This handles common versioning patterns (v1.0.0, 1.2.3) without requiring explicit type specification.

**Cache directory structure:** Git deps in `.deps/git/{name}-{ref}`, tarball deps in `.deps/tarball/{name}-{checksum}`. Vendored deps remain in place at their original paths.

**Marker file for integrity:** `.clue-dep` JSON file created in cache directories tracks fetch metadata (name, type, fetched_at, ref/url/path) for future verification and debugging.

**In-place validation for vendored deps:** Vendored fetcher doesn't copy files - it validates the path exists and warns if no build configuration (clue.cue or inline config) is present.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all implementations completed successfully with expected behavior.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for 06-03:** Dependency building infrastructure can now use fetchers to retrieve dependencies before building them.

**Blockers:** None. Environment has no external network access, so actual git clones will fail in testing, but infrastructure is in place for when network is available.

**Concerns:**
- CleanDep() string matching logic may need refinement - currently checks if directory name starts with sanitized name followed by dash
- No network means git fetch testing is limited to unit tests without actual clone operations

---
*Phase: 06-external-dependencies*
*Completed: 2026-01-23*
