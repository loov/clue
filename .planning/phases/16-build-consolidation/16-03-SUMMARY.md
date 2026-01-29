---
phase: 16-build-consolidation
plan: 03
subsystem: testing
tags: [test-helpers, code-organization, DRY]
completed: 2026-01-29
duration: 168s

# Dependency Graph
requires:
  - phase: 16
    plan: 02
    reason: "Build package structure established"
provides:
  - artifact: internal/testclue package
    exports: [SkipIfNoClang, SkipIfNoClangPP, CreateTestProject, CreateLargeTestProject]
    purpose: Shared test utilities for integration tests
affects:
  - subsystem: testing
    impact: Future tests can use testclue helpers instead of duplicating code

# Technical Stack
tech-stack:
  added:
    - internal/testclue: Shared test utilities package
  patterns:
    - "testing.TB interface for Test/Benchmark compatibility"
    - "t.Helper() for proper error reporting in test helpers"

# Key Files
key-files:
  created:
    - path: internal/testclue/helpers.go
      purpose: Toolchain availability skip helpers
      exports: [SkipIfNoClang, SkipIfNoClangPP]
      loc: 23
    - path: internal/testclue/fixtures.go
      purpose: Test project creation helpers
      exports: [CreateTestProject, CreateLargeTestProject, formatCueArray]
      loc: 171
  modified:
    - path: internal/build/incremental_test.go
      change: Use testclue.SkipIfNoClang and testclue.CreateTestProject
      removed_loc: 93
    - path: internal/build/parallel_integration_test.go
      change: Use testclue.SkipIfNoClangPP and testclue.CreateLargeTestProject
      removed_loc: 114
    - path: internal/build/integration_output_test.go
      change: Use testclue.SkipIfNoClangPP
      removed_loc: 5

# Decisions
decisions:
  - what: Use testing.TB interface instead of *testing.T
    why: Allows helpers to work with both Test and Benchmark functions
    alternatives: [Use *testing.T only]
    rationale: Standard Go practice for test helpers that work in any test context

  - what: Mark all helpers with t.Helper()
    why: Ensures test failures report the correct line number in calling test
    alternatives: [Omit t.Helper()]
    rationale: Standard Go practice for test helper functions

  - what: Keep formatCueArray as internal helper in fixtures.go
    why: Only used by CreateLargeTestProject, not general purpose
    alternatives: [Export it, Move to separate utility package]
    rationale: Smallest scope principle - keep helper private until multiple callers need it
---

# Phase 16 Plan 03: Test Helper Extraction Summary

**One-liner:** Extracted shared test helpers (skip functions, project creation) into internal/testclue package, eliminating 197 lines of duplication.

## Objective

Create internal/testclue package with shared test helpers and update test files to use them. Per CONTEXT.md, shared test helpers go in `internal/testclue` (not testutil/testutils).

## What Was Built

### Task 1: Created internal/testclue Package

Created two files with shared test utilities:

**internal/testclue/helpers.go:**
- `SkipIfNoClang(t testing.TB)` - Skips test if clang not available
- `SkipIfNoClangPP(t testing.TB)` - Skips test if clang++ not available

**internal/testclue/fixtures.go:**
- `CreateTestProject(t testing.TB, tmpDir string)` - Creates minimal 2-file C++ project for incremental build testing
- `CreateLargeTestProject(t testing.TB)` - Creates 20-file C++ project for parallel build testing
- `formatCueArray(items []string)` - Internal helper for CUE array formatting

All helpers use `testing.TB` interface and `t.Helper()` for proper test error reporting.

### Task 2: Updated Test Files

Refactored three test files to use shared helpers:

1. **internal/build/incremental_test.go**
   - Replaced local `skipIfNoClang` with `testclue.SkipIfNoClang`
   - Replaced local `createTestProject` with `testclue.CreateTestProject`
   - Removed 93 lines of duplicate code

2. **internal/build/parallel_integration_test.go**
   - Replaced local `skipIfNoClangPP` with `testclue.SkipIfNoClangPP`
   - Replaced local `createLargeTestProject` with `testclue.CreateLargeTestProject`
   - Removed 114 lines of duplicate code

3. **internal/build/integration_output_test.go**
   - Added import for testclue
   - Replaced `skipIfNoClangPP` calls with `testclue.SkipIfNoClangPP`

## Technical Implementation

### Test Helper Best Practices

1. **testing.TB Interface**: Used instead of `*testing.T` to support both Test and Benchmark functions
2. **t.Helper() Marking**: All helpers marked with `t.Helper()` so test failures report correct line numbers
3. **Absolute Paths**: Test fixtures use absolute paths for CUE config sources (Go test requirement)
4. **TempDir Cleanup**: CreateLargeTestProject returns no-op cleanup (t.TempDir() auto-cleans)

### Code Elimination

Removed duplicate implementations across three test files:
- **skipIfNoClang**: Duplicated in incremental_test.go
- **skipIfNoClangPP**: Duplicated in parallel_integration_test.go and integration_output_test.go
- **createTestProject**: Duplicated in incremental_test.go
- **createLargeTestProject**: Duplicated in parallel_integration_test.go
- **formatCueArray**: Duplicated in parallel_integration_test.go

Total duplication eliminated: **197 lines**

## Verification

### Build Verification
```bash
go build ./internal/testclue/...
# ✓ Package builds without errors
```

### Duplication Check
```bash
grep -c "func skipIfNoClang\|func createTestProject\|func createLargeTestProject" internal/build/*_test.go
# Result: 0 (no duplicate helpers remain)
```

### Test Suite
```bash
go test ./...
# ✓ All 116+ tests pass
# ✓ internal/build tests: ok (9.189s)
```

## Deviations from Plan

None - plan executed exactly as written.

## Metrics

- **Files created**: 2
- **Files modified**: 3
- **Lines added**: 194
- **Lines removed**: 197
- **Net change**: -3 lines
- **Tests affected**: 9 test functions across 3 files
- **All tests passing**: ✓

## Integration Points

### Exports Used By

- `internal/build/incremental_test.go`: SkipIfNoClang, CreateTestProject
- `internal/build/parallel_integration_test.go`: SkipIfNoClangPP, CreateLargeTestProject
- `internal/build/integration_output_test.go`: SkipIfNoClangPP

### Import Pattern

```go
import "github.com/loov/clue/internal/testclue"

func TestSomething(t *testing.T) {
    testclue.SkipIfNoClang(t)
    projectDir, cfg := testclue.CreateTestProject(t, t.TempDir())
    // ... test implementation
}
```

## Decisions Made

| Decision | Rationale | Status |
|----------|-----------|--------|
| Use testing.TB interface | Allows helpers to work with both Test and Benchmark functions | Good |
| Mark all helpers with t.Helper() | Ensures test failures report correct line number in calling test | Good |
| Keep formatCueArray internal | Only used by CreateLargeTestProject, not general purpose | Good |

## Lessons Learned

### What Worked Well

1. **Extract-then-refactor pattern**: Creating package first, then updating callers, made changes atomic and verifiable
2. **testing.TB interface**: Standard practice for test helpers that need to work in multiple contexts
3. **t.Helper() discipline**: Proper error reporting makes test failures easier to debug

### Best Practices Established

1. **Test helpers go in internal/testclue**: Named per CONTEXT.md convention, not testutil/testutils
2. **Skip helpers for toolchain requirements**: Consistent pattern for tests requiring specific compilers
3. **Fixture builders return cleanup functions**: Even if no-op, enables future cleanup logic without API change

## Next Phase Readiness

### Blockers

None.

### Concerns

None.

### Recommendations

1. **Future test helpers**: Add to internal/testclue package following established patterns
2. **Other test files**: Consider extracting more helpers as duplication emerges
3. **Testing documentation**: Could document test helper usage in CONTRIBUTING.md

## Related Work

- Phase 16 Plan 02: Established build package structure and toolchain abstractions
- Future phases: Any new integration tests should use testclue helpers instead of duplicating code

## Performance Impact

No runtime performance impact. Slightly improved compile times (less code to parse/compile).

## Success Criteria Met

- [x] internal/testclue package exists with helpers.go and fixtures.go
- [x] SkipIfNoClang, SkipIfNoClangPP, CreateTestProject, CreateLargeTestProject exported
- [x] All functions use t.Helper() for proper error reporting
- [x] Test files import testclue and use shared helpers
- [x] No duplicate helper functions remaining in test files
- [x] All 116+ tests pass

## Commits

1. **b62869b**: `feat(16-03): create internal/testclue package with shared test helpers`
   - Created helpers.go with SkipIfNoClang, SkipIfNoClangPP
   - Created fixtures.go with CreateTestProject, CreateLargeTestProject
   - Used testing.TB interface and t.Helper() for proper error reporting

2. **0dcada6**: `refactor(16-03): update test files to use testclue package`
   - Updated incremental_test.go to use testclue helpers
   - Updated parallel_integration_test.go to use testclue helpers
   - Updated integration_output_test.go to use testclue helpers
   - Removed 197 lines of duplicate helper implementations
   - All 116+ tests pass with shared helpers
