# Quick Task 004: Fix Orphaned Integration Tests (Verbosity Enum Update)

**One-liner:** Updated integration tests in deps and generate packages to use Verbosity enum instead of deprecated Verbose bool API.

## Completed Tasks

| Task | Name | Commit | Files Modified |
|------|------|--------|----------------|
| 1 | Update deps integration tests | 5bc24ef | internal/deps/integration_test.go |
| 2 | Update generate integration tests | de8cb80 | internal/generate/integration_test.go |
| 3 | Verify all tests pass | (verification only) | - |

## Changes Made

### Task 1: deps/integration_test.go
- Updated 4 `build.NewBuilder()` calls from `false`/`true` third parameter to `build.VerbosityNormal`/`build.VerbosityVerbose`
- Updated 4 `build.BuildOptions` structs from `Verbose: false/true` to `Verbosity: build.VerbosityNormal/VerbosityVerbose`
- Affected test functions:
  - TestSuccessCriteria1_VendoredDependency
  - TestSuccessCriteria3_OfflineBuild (2 builder instances)
  - TestSuccessCriteria4_DependencyBuildOutput

### Task 2: generate/integration_test.go
- Updated 1 `build.NewBuilder()` call from `false` third parameter to `build.VerbosityNormal`
- Updated 1 `build.BuildOptions` struct from `Verbose: false` to `Verbosity: build.VerbosityNormal`
- Affected test function: TestNinjaIdenticalOutput

## Verification Results

- `go build ./internal/deps/...` - passed
- `go build ./internal/generate/...` - passed
- `go test ./internal/deps/...` - passed (0.699s)
- `go test ./internal/generate/...` - passed (0.006s)
- No remaining `Verbose:` references in either file

## Root Cause

Phase 08-01 refactored `build.BuildOptions` from using a simple `Verbose bool` field to a three-level `Verbosity build.Verbosity` enum (Quiet=0, Normal=1, Verbose=2). The integration tests in `internal/deps` and `internal/generate` packages were not updated as part of that refactoring, causing compilation failures:
- `NewBuilder` third parameter changed from `bool` to `Verbosity`
- `BuildOptions.Verbose bool` became `BuildOptions.Verbosity Verbosity`

## Deviations from Plan

None - plan executed exactly as written.

## Duration

Start: 2026-01-24T05:24:21Z
End: 2026-01-24T05:25:38Z
Duration: ~1.3 minutes
