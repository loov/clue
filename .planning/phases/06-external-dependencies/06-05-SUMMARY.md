---
phase: 06-external-dependencies
plan: 05
subsystem: build-system
tags: [dependencies, static-libraries, compilation, linking]

# Dependency graph
requires:
  - phase: 06-04
    provides: Dependency resolver and manager for fetching
  - phase: 02-core-compilation
    provides: Compiler and linker infrastructure
provides:
  - Dependency builder that compiles external deps to static libraries
  - Integration with main build pipeline
  - Automatic include path and library linking
affects: [06-06, 06-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Dependency building before main targets
    - Collapsed build output for dependencies
    - Include path auto-detection (prefer include/ directory)

key-files:
  created:
    - internal/build/dep_builder.go
    - internal/build/dep_builder_test.go
  modified:
    - internal/build/builder.go

key-decisions:
  - "DepBuilder in build package to avoid import cycles"
  - "Collapsed output for dependency builds (expanded on error)"
  - "Include path auto-detection: inline config > include/ dir > source root"
  - "Dependencies inherit variant and platform from main build"
  - "Dependency libraries go to .build/variant/deps/{name}/lib/"

patterns-established:
  - "Dependency build artifacts in .build/variant/deps/"
  - "Single-line progress output per dependency"
  - "Include paths added to both compilation and linking"

# Metrics
duration: 6.2min
completed: 2026-01-23
---

# Phase 06 Plan 05: Dependency Building Integration Summary

**External dependencies automatically compiled to static libraries before main targets, with proper include paths and linking**

## Performance

- **Duration:** 6.2 min
- **Started:** 2026-01-23T17:21:07Z
- **Completed:** 2026-01-23T17:27:18Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- DepBuilder compiles dependency sources to static libraries
- Main Builder automatically builds dependencies before targets
- Dependency include paths and libraries passed to dependent targets
- Comprehensive test coverage for all dependency build scenarios

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement dependency builder** - `606c502` (feat)
2. **Task 2: Integrate dependency building into main builder** - `9fc18b9` (feat)
3. **Task 3: Add dependency building tests** - `76d54db` (test)

## Files Created/Modified
- `internal/build/dep_builder.go` - Builds individual dependencies to static libraries
- `internal/build/dep_builder_test.go` - Tests for inline config, clue.cue, globs, include paths
- `internal/build/builder.go` - Extended with dependency building support

## Decisions Made

**1. DepBuilder in build package to avoid import cycles**
- **Rationale:** build imports config which imports deps, so deps cannot import build
- **Impact:** Cleaner package structure, avoids circular dependencies

**2. Collapsed output for dependency builds**
- **Rationale:** Clean progress output, expands to show compiler errors on failure
- **Format:** "Building libfoo [4 files]" instead of per-file output

**3. Include path auto-detection**
- **Order:** inline config includes > sourcePath/include directory > source root
- **Rationale:** Matches common C++ library conventions (header-only or include/)

**4. Dependencies inherit variant and platform from main build**
- **Rationale:** Ensures ABI compatibility between dependencies and main targets
- **Implementation:** DepBuildOptions passes variant and platform

**5. Dependency artifacts in dedicated directory**
- **Path:** .build/variant/deps/{name}/lib/lib{name}.a
- **Rationale:** Separation from main project artifacts, clear ownership

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed include path not added during compilation**
- **Found during:** Task 3 (TestDepBuilder_IncludePath)
- **Issue:** Test failing because include/ directory not added to compilation includes
- **Fix:** determineIncludePath called before compilation loop, added to compilationIncludes
- **Files modified:** internal/build/dep_builder.go
- **Verification:** Test passes, header found during compilation
- **Committed in:** 76d54db (part of Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Fix necessary for include path feature to work correctly. No scope creep.

## Issues Encountered

**Import cycle prevention:**
- Initial attempt placed DepBuilder in internal/deps package
- Import cycle: build → config → deps → build
- Solution: Moved DepBuilder to internal/build package
- Result: Clean package hierarchy maintained

**Resolver access:**
- Manager.resolver is unexported
- Cannot call mgr.resolver.BuildOrder() from builder
- Solution: Simple alphabetical sort in buildDependencies (dependencies don't have interdependencies yet)
- Note: When interdependencies are added in future phase, will need Manager.BuildOrder() method

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for:**
- CLI commands for dependency management (clue deps fetch, clue deps build, clue deps clean)
- Full project builds with external dependencies
- Cross-compilation of dependencies

**Notes:**
- Dependency interdependencies not yet supported (alphabetical build order used)
- Manager.BuildOrder() method needed if deps depend on other deps
- All core infrastructure in place for full dependency workflow

---
*Phase: 06-external-dependencies*
*Completed: 2026-01-23*
