---
phase: quick
plan: 022
subsystem: testdata
tags: [cue, vendored-deps, inline-config]
completed: 2026-01-29
duration: "56s"
depends_on: []
key-files:
  modified:
    - testdata/multi-deps-example/clue.cue
  deleted:
    - testdata/multi-deps-example/vendor/simplemath/clue.cue
    - testdata/multi-deps-example/vendor/stringutils/clue.cue
---

# Quick Task 022: Move Vendor CUE to Parent Config Summary

Moved vendor dependency build configs into parent clue.cue inline build blocks, eliminating separate vendor clue.cue files and the unrealistic `../simplemath` relative path hack.

## What Changed

The `testdata/multi-deps-example` project previously had separate `clue.cue` files in each vendored dependency directory (`vendor/simplemath/clue.cue` and `vendor/stringutils/clue.cue`). These have been replaced with inline `build` blocks in the parent `clue.cue`, which is the recommended pattern for vendored dependencies.

### Parent clue.cue updates

- `simplemath` dependency: Added `build: { sources: ["math.cpp"] }`
- `stringutils` dependency: Added `build: { sources: ["utils.cpp"], includes: [".."] }`
  - The `includes: [".."]` resolves to `vendor/stringutils/..` = `vendor/`, enabling `#include "simplemath/math.h"` in `utils.cpp`

### Deleted files

- `testdata/multi-deps-example/vendor/simplemath/clue.cue` - config moved to parent inline build block
- `testdata/multi-deps-example/vendor/stringutils/clue.cue` - config moved to parent inline build block; eliminated the `path: "../simplemath"` relative dependency hack

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 4dd9185 | Move vendor clue.cue configs to parent inline build blocks |

## Verification

- All tests pass (`make test`), including `TestMultiDepsExample_Integration`
- `go vet` passes
- Vendor clue.cue files confirmed deleted

## Deviations from Plan

None - plan executed exactly as written.
