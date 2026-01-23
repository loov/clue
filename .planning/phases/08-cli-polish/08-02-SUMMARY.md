#---
phase: 08
plan: 02
subsystem: cli
tags: [run-command, execution, cli]
requires: [07-05]
provides: [run-command-implementation]
affects: []
tech-stack:
  added: []
  patterns: [build-then-execute]
key-files:
  created:
    - internal/build/runner.go
    - internal/build/runner_test.go
  modified:
    - cmd/clue/main.go
    - internal/build/builder.go
    - internal/build/dep_builder.go
decisions:
  - "Run command always uses host platform (no cross-compilation for run)"
  - "Arguments after target name passed directly to executable"
  - "Executable runs in current working directory"
  - "Exit code from executable propagated as clue's exit code"
metrics:
  duration: 594s
  completed: 2026-01-23
---

# Phase 08 Plan 02: Implement Run Command Summary

**One-liner:** Cargo-style run command that builds target and executes it in one operation

## What Was Built

Implemented `clue run <target> [args...]` command that:

1. **Runner Module** (`internal/build/runner.go`):
   - `RunTarget` function validates target exists and is executable type
   - Builds target using existing Build infrastructure
   - Executes binary in current working directory
   - Propagates exit code from executable to clue's exit code
   - Passes command-line arguments to executable

2. **CLI Integration** (`cmd/clue/main.go`):
   - `runRun` function handles config loading and job calculation
   - Signal handling for graceful cancellation during build
   - Usage message when no target specified
   - Clear error messages for invalid targets

3. **Tests** (`internal/build/runner_test.go`):
   - TestRunTarget_TargetNotFound: validates error for nonexistent target
   - TestRunTarget_NotExecutable_StaticLibrary: rejects static library targets
   - TestRunTarget_NotExecutable_SharedLibrary: rejects shared library targets

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Verbosity type mismatch in builder.go**
- **Found during:** Task 1 compilation
- **Issue:** `NewProgress(totalSources, opts.Verbose)` passed bool instead of Verbosity type
- **Fix:** Convert bool to Verbosity before calling NewProgress
- **Files modified:** internal/build/builder.go
- **Commit:** 184f630

**2. [Rule 1 - Bug] Fixed progress.go verbosity field usage**
- **Found during:** Task 1 compilation
- **Issue:** `p.verbose` undefined - should be `p.verbosity` with type checking
- **Fix:** File was auto-fixed by another process before my edits
- **Files modified:** internal/build/progress.go (already fixed)
- **Commit:** 184f630

**3. [Rule 1 - Bug] Adapted to Verbosity refactoring from plan 08-03**
- **Found during:** Task 2 compilation
- **Issue:** Plan 08-03's Verbosity refactoring was incomplete - bool verbose still used in multiple places
- **Fix:** Updated runner.go, builder.go, dep_builder.go, main.go to use Verbosity type consistently
- **Files modified:** runner.go, builder.go, dep_builder.go, main.go
- **Commit:** 8c16044

**4. [Rule 1 - Bug] Fixed runDeps loadConfig call**
- **Found during:** Task 2 compilation
- **Issue:** runDeps passed bool to loadConfig which expects Verbosity
- **Fix:** Convert bool verbose to Verbosity before calling loadConfig
- **Files modified:** cmd/clue/main.go
- **Commit:** 8c16044

**5. [Rule 1 - Bug] Partial test file fixes**
- **Found during:** Final verification
- **Issue:** Test files had incomplete Verbosity refactoring (from 08-03)
- **Fix:** Used perl to batch-update common patterns in test files
- **Files modified:** Multiple *_test.go files in internal/build/
- **Commit:** 8c16044
- **Note:** Some test files still have issues - full fix belongs in plan 08-03

## Verification Results

**Manual Testing:**

1. ✅ `clue run` without arguments shows usage message
2. ✅ `clue run nonexistent` shows "target not found" error
3. ✅ `clue run mathlib` (static_library) shows "not an executable" error
4. ✅ `clue run testapp` builds and runs executable
5. ✅ `clue run testapp arg1 arg2` passes arguments correctly
6. ✅ Exit code 42 from test executable propagated to clue
7. ⚠️  Some unrelated tests fail due to incomplete 08-03 refactoring

**Unit Tests:**
- ✅ TestRunTarget_TargetNotFound passes
- ✅ TestRunTarget_NotExecutable_StaticLibrary passes
- ✅ TestRunTarget_NotExecutable_SharedLibrary passes

## Decisions Made

| Decision | Rationale | Impact |
|----------|-----------|--------|
| Run always uses host platform | Running cross-compiled binaries requires emulation/remote execution which is out of scope | Users must build natively to run |
| Arguments passed directly | Simplest UX matching cargo/npm run patterns | No argument preprocessing |
| Execute in CWD | Matches user expectation - relative paths in program work | Binary can access files relative to where clue was invoked |
| Propagate exit code | Enables scripting and CI integration | `clue run tests && deploy` works as expected |

## Key Implementation Details

**RunOptions Structure:**
```go
type RunOptions struct {
    Config    *config.Config
    Variant   string
    BuildDir  string
    Target    string      // Target name to run
    Args      []string    // Arguments to pass to executable
    Verbosity Verbosity   // For build output
    Jobs      int         // Parallel jobs for build
}
```

**Build-Then-Execute Flow:**
1. Validate target exists and is executable type
2. Create builder with host platform
3. Build target using Build infrastructure
4. Extract executable path from build result
5. Execute with `exec.Command` (stdout/stderr/stdin passthrough)
6. Return exit code or execution error

**Integration with existing infrastructure:**
- Reuses `Build()` function - no duplication
- Leverages existing signal handling via `SetupSignalHandling()`
- Uses same job calculation logic as `clue build`
- Respects variant selection and config loading

## Next Phase Readiness

**Blockers:** None

**Concerns:**
- Plan 08-03's Verbosity refactoring is incomplete - some test files still broken
- Should be completed in 08-03 to fully resolve test failures

**Recommendations:**
- Complete Verbosity refactoring in remaining test files
- Consider integration tests for run command (build + execute + verify output)
- Future: Support for run command with watch mode (rebuild + re-run on file changes)

## Files Changed

**Created:**
- `internal/build/runner.go` (96 lines): RunTarget implementation
- `internal/build/runner_test.go` (152 lines): Unit tests

**Modified:**
- `cmd/clue/main.go`: Added runRun function and wired run case
- `internal/build/builder.go`: Fixed Verbosity type usage
- `internal/build/dep_builder.go`: Fixed Verbosity field checks
- Multiple test files: Partial Verbosity refactoring fixes

## Links & Dependencies

**Depends on:**
- 07-05: CLI help command and main structure

**Enables:**
- Future quick-iteration workflows
- Test execution via `clue run tests`
- Example/demo execution

**Relates to:**
- 08-03: Verbosity refactoring (incomplete, caused conflicts)
- Future: Watch mode, hot reload capabilities
