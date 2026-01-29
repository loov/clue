---
phase: quick
plan: 024
subsystem: build
tags: [dependencies, headers, include-paths, cue, go]

# Dependency graph
requires:
  - phase: quick-022
    provides: vendor clue.cue configs moved to parent inline build blocks
provides:
  - headers field in #InlineBuildConfig schema for declaring public header files
  - Automatic parent directory inclusion for dependencies with headers
  - Clean alternative to includes workarounds for header-based dependencies
affects: [dependency-management, build-configuration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "headers field semantic: declares public headers, sets IncludePath to parent dir"
    - "Include path priority: headers > includes > include/ dir > sourcePath"

key-files:
  created: []
  modified:
    - internal/config/schema.cue
    - internal/deps/types.go
    - internal/config/loader.go
    - internal/build/dep_builder.go
    - internal/build/dep_builder_test.go
    - testdata/multi-deps-example/clue.cue

key-decisions:
  - "headers field returns parent directory for #include \"depname/header.h\" pattern"
  - "Maintain fallback chain: headers > includes > include/ dir > sourcePath"

patterns-established:
  - "Dependencies with public headers use headers field instead of includes workarounds"

# Metrics
duration: 5min
completed: 2026-01-29
---

# Quick Task 024: Add headers Field to InlineBuildConfig Summary

**Added headers field to inline build config enabling semantic header declaration with automatic parent directory inclusion**

## Performance

- **Duration:** 5 min
- **Started:** 2026-01-29T18:51:27Z
- **Completed:** 2026-01-29T18:57:24Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Added optional headers field to #InlineBuildConfig CUE schema
- Extended InlineConfig Go struct with Headers []string field
- Updated config loader to extract headers from CUE configurations
- Modified determineIncludePath to check headers first and return parent directory
- Added comprehensive unit tests for headers-based include path logic
- Updated multi-deps-example to use headers instead of includes workaround

## Task Commits

Each task was committed atomically:

1. **Task 1: Add headers field to schema, types, and loader** - `8f684a0` (feat)
2. **Task 2: Update determineIncludePath and add test** - `6554b95` (feat)

## Files Created/Modified
- `internal/config/schema.cue` - Added headers?: [...string] to #InlineBuildConfig
- `internal/deps/types.go` - Added Headers []string field to InlineConfig struct
- `internal/config/loader.go` - Extract headers in extractInlineConfig
- `internal/build/dep_builder.go` - Check headers field first in determineIncludePath
- `internal/build/dep_builder_test.go` - Added TestDepBuilder_HeadersIncludePath with 4 test cases
- `testdata/multi-deps-example/clue.cue` - Replaced includes: [".."] with headers: ["math.h"]

## Decisions Made

**Priority chain for include path determination:**
- Check headers first: if set, return parent directory of sourcePath
- Fallback to includes: if set, return sourcePath + includes[0]
- Fallback to include/ directory check
- Final fallback: sourcePath itself

**Semantic meaning:**
- headers field declares "this dependency has public headers"
- Setting headers causes IncludePath = filepath.Dir(sourcePath)
- This enables `#include "depname/header.h"` pattern without path-escape hacks

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test expectation for filepath.Join behavior**
- **Found during:** Task 2 (test execution)
- **Issue:** Test expected filepath.Join(path, "..") to return "path/.." but filepath.Join cleans paths automatically
- **Fix:** Updated test expectation from "/tmp/test/vendor/simplemath/.." to "/tmp/test/vendor"
- **Files modified:** internal/build/dep_builder_test.go
- **Verification:** All tests pass
- **Committed in:** 6554b95 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Bug fix corrected incorrect test expectation. No scope creep.

## Issues Encountered

**Filepath.Join path cleaning:**
- Initially expected filepath.Join to preserve ".." in path strings
- Discovered filepath.Join automatically cleans paths, so Join(a, "..") returns parent
- This is correct behavior - updated test expectations accordingly

## Next Phase Readiness

**Ready for use:**
- Dependencies can now declare public headers via headers field
- Include paths automatically set to parent directory for header-based deps
- Replaces fragile includes: [".."] workaround pattern
- All existing tests pass, no regressions

**No blockers or concerns**

---
*Phase: quick*
*Completed: 2026-01-29*
