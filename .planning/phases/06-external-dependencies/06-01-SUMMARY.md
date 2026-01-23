---
phase: 06-external-dependencies
plan: 01
subsystem: configuration
tags: [cue, dependencies, git, tarball, vendored, config-schema]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: CUE schema and loader infrastructure
provides:
  - Dependency type definitions (Git, Tarball, Vendored)
  - CUE schema for dependency declarations
  - Config loader integration for dependencies
affects: [06-02, 06-03, 06-04, 06-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dependency interface with Name(), Type(), CachePath(), Validate() methods"
    - "CUE union types for dependency discrimination"
    - "InlineConfig for dependencies without clue.cue"
    - "Type-based dependency extraction in loader"

key-files:
  created:
    - internal/deps/types.go
    - internal/config/loader_deps_test.go
  modified:
    - internal/config/schema.cue
    - internal/config/loader.go

key-decisions:
  - "Dependency name from map key - CUE uses dependencies map keys as dependency names, not separate name field"
  - "Cache path structure - .deps/{type}/{sanitized-name}-{short-ref} for predictable locations"
  - "Vendored deps return original path - no caching needed for local source tree dependencies"
  - "InlineConfig for deps without clue.cue - enables building third-party libraries without configuration files"
  - "Validation at load time - Validate() called during config extraction for fail-fast"

patterns-established:
  - "Dependency interface pattern: common methods across all dependency types"
  - "Union type discrimination: use type field to select concrete type in loader"
  - "Constructor functions: NewGitDependency, NewTarballDependency, NewVendoredDependency with defaults"

# Metrics
duration: 4min
completed: 2026-01-23
---

# Phase 06 Plan 01: Dependency Schema and Loader Summary

**CUE schema and Go types for git, tarball, and vendored dependencies with validation and cache path generation**

## Performance

- **Duration:** 4 minutes
- **Started:** 2026-01-23T17:03:33Z
- **Completed:** 2026-01-23T17:07:35Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Created dependency type system with Dependency interface and three implementations
- Extended CUE schema with #Dependency union type and validation constraints
- Integrated dependency extraction into config loader with validation
- Established cache path generation for each dependency type

## Task Commits

Each task was committed atomically:

1. **Task 1: Create dependency types package** - `4ea8195` (feat)
2. **Task 2: Extend CUE schema with dependency definitions** - `b286ea6` (feat)
3. **Task 3: Extend config loader to extract dependencies** - `3a8179c` (feat)

## Files Created/Modified
- `internal/deps/types.go` - Dependency interface and implementations (GitDependency, TarballDependency, VendoredDependency, InlineConfig)
- `internal/config/schema.cue` - Added #Dependency union type, #GitDependency, #TarballDependency, #VendoredDependency, #InlineBuildConfig
- `internal/config/loader.go` - Added extractDependencies, extractGitDependency, extractTarballDependency, extractVendoredDependency methods
- `internal/config/loader_deps_test.go` - Comprehensive tests for dependency extraction and validation

## Decisions Made
- **Dependency name from map key:** The name comes from the CUE map key in the dependencies block, not from a field within the dependency definition. This matches CUE's structural approach and avoids redundancy.
- **Cache path structure:** Git deps use `.deps/git/{name}-{ref}`, tarball deps use `.deps/tarball/{name}-{checksum-prefix}`, vendored deps return original path. This provides predictable locations for future fetch operations.
- **Validation at load time:** Each dependency's Validate() method is called during config extraction, ensuring fail-fast behavior with clear error messages.
- **InlineConfig for third-party libs:** Dependencies without their own clue.cue can specify build configuration inline, enabling integration of libraries that don't use clue.
- **CUE regex validation:** Git URLs validated with `=~"^(https?://|git@)"`, tarball URLs with `=~"^https?://"`, SHA256 checksums with `=~"^[a-f0-9]{64}$"` - catches errors at parse time.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all tasks completed without issues.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for 06-02 (dependency fetching). The dependency types, schema, and loader integration are complete and tested. Future plans can:
- Use `deps.Dependency` interface for fetching operations
- Read dependency definitions from `config.Dependencies` map
- Use `CachePath()` to determine where to store fetched dependencies
- Rely on validation having occurred at load time

No blockers or concerns.

---
*Phase: 06-external-dependencies*
*Completed: 2026-01-23*
