---
task: 002-fix-testdata-cue-idiomatic
type: quick
subsystem: testdata
tags: [cue, formatting, code-style]

key-files:
  modified:
    - testdata/multi-target/clue.cue
    - testdata/syslibs-test/clue.cue
    - testdata/sample/clue.cue

duration: 3min
completed: 2026-01-23
---

# Quick Task 002: Fix Testdata CUE Files Summary

**Converted all testdata CUE files from JSON syntax to idiomatic CUE syntax with unquoted field names and proper formatting**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-23
- **Completed:** 2026-01-23
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Converted testdata/multi-target/clue.cue from single-line JSON to formatted CUE
- Converted testdata/syslibs-test/clue.cue from single-line JSON to formatted CUE
- Converted testdata/sample/clue.cue from JSON-style (quoted keys) to idiomatic CUE
- All files remain valid CUE and parse correctly by the config loader

## Task Commits

1. **Task 1: Convert all testdata CUE files to idiomatic syntax** - `1e30e6f` (style)
2. **Task 2: Run full test suite to verify no regressions** - verification only, no commit

## Files Modified

- `testdata/multi-target/clue.cue` - Multi-target project config (library + executable with dependencies)
- `testdata/syslibs-test/clue.cue` - System library linking test config
- `testdata/sample/clue.cue` - Sample project with variants and env vars

## Changes Applied

**Before (JSON syntax):**
```json
{"name": "syslibs-test", "version": "1.0.0", ...}
```

**After (idiomatic CUE):**
```cue
name: "syslibs-test"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    std:      "c++17"
}
```

Key transformations:
- Removed quotes from field names
- Added multi-line formatting with tab indentation
- Aligned colons for readability
- Preserved all values exactly

## Decisions Made

None - followed plan as specified

## Deviations from Plan

None - plan executed exactly as written

## Issues Encountered

- `cue vet` CLI not available in environment (command not found)
- **Resolution:** Used Go config loader tests to verify CUE validity - all pass
- Pre-existing cmd/clue test failures (variant application bug) confirmed unrelated to this change

## Verification

- All internal package tests pass (config, build, graph, errors)
- Config loader parses all three converted files successfully
- Pre-existing variant application bug in cmd/clue is documented in STATE.md and unaffected by this change

---
*Quick Task: 002-fix-testdata-cue-idiomatic*
*Completed: 2026-01-23*
