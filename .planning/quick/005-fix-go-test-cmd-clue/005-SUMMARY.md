# Quick Task 005: Fix go test ./cmd/clue Summary

**One-liner:** Fixed 3 failing tests by resolving variant name conflicts and correcting flag positioning

## Changes Made

### Task 1: Fix variant name conflicts (7c071fc)

**Problem:** Tests TestValidateCommand and TestValidateWithVariant failed with "name: conflicting values" errors when applying variants.

**Root Cause:** Two issues combined:
1. testdata/sample/clue.cue variants had explicit `name` fields that conflicted with project name during CUE unification
2. ApplyVariant() tried to unify variant object with entire root config, causing field conflicts (variant fields like `optimization` weren't allowed at root level)

**Fix:**
- Removed `name` fields from all variant definitions in testdata/sample/clue.cue
- Made `#Variant.name` optional in schema.cue (name derived from map key)
- Rewrote ApplyVariant() to extract variant details into ActiveVariant struct without modifying CUE value (avoids problematic unification)

**Files modified:**
- internal/config/schema.cue - make Variant.name optional
- internal/config/variants.go - simplified ApplyVariant to not unify
- testdata/sample/clue.cue - removed name fields from variants

### Task 2: Fix flag positioning (0793bc5)

**Problem:** TestClean_AfterBuild failed with "Expected build dir to be empty" because `--all` flag wasn't parsed.

**Root Cause:** Tests used `clean --all` but Go's flag package requires flags BEFORE commands (decision 01-05).

**Fix:** Changed all instances of `clean --all` to `--all clean`:
- Line 210: TestBuild_MultiTarget cleanup
- Line 275: TestBuild_Verbose cleanup
- Line 332: TestClean_AfterBuild test assertion
- Line 366: TestBuild_SysLibs cleanup

**Files modified:**
- cmd/clue/main_test.go - 4 flag position corrections

### Task 3: Verify all tests pass

All 14 tests in cmd/clue package now pass:
- TestValidateCommand
- TestValidateWithVariant
- TestValidateInvalidConfig
- TestVersionFlag
- TestValidateNoConfigError
- TestCycleDetectionError
- TestBuild_MultiTarget
- TestBuild_Verbose
- TestClean_AfterBuild
- TestBuild_SysLibs
- TestTargetFlag_Empty
- TestTargetFlag_Valid
- TestTargetFlag_Invalid
- TestTargetFlag_UnsupportedPlatform

## Deviations from Plan

### Rule 1 - Bug Fix: ApplyVariant unification bug

**Found during:** Task 1

**Issue:** The plan assumed removing `name` from variants would fix the unification conflict. However, the deeper issue was that ApplyVariant() unified the entire variant object (with fields like `optimization`, `debug_info`) with the root config, causing "field not allowed" errors.

**Fix:** Rewrote ApplyVariant() to only extract variant details into the ActiveVariant struct, without modifying the CUE value. This resolves the STATE.md blocker "Variant application bug."

**Files modified:** internal/config/variants.go

## Impact

- Resolves blocker documented in STATE.md: "Variant application bug: ApplyVariant() in variants.go unifies entire config with variant definition"
- All cmd/clue tests now pass
- Variant-based builds and validation work correctly

## Metrics

- Duration: ~3 minutes
- Commits: 2
- Files modified: 4
